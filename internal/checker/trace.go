package checker

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/t0mer/dnsmon/internal/dnsclient"
)

// rootServers are the anycast addresses of IANA root nameservers used as
// trace entry points. We hard-code IPv4 addresses to avoid a bootstrap
// dependency on a recursive resolver.
var rootServers = []string{
	"198.41.0.4",   // a.root-servers.net
	"199.9.14.201", // b.root-servers.net
	"192.33.4.12",  // c.root-servers.net
	"199.7.91.13",  // d.root-servers.net
}

const maxTraceSteps = 24

// Trace walks the DNS delegation chain from the root nameservers down to the
// authoritative server(s) for name, performing non-recursive queries at each
// hop — equivalent to `dig +trace`.
func (c *Checker) Trace(ctx context.Context, name, qtype string) (*dnsclient.TraceResult, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if _, ok := dnsclient.ParseType(qtype); !ok {
		return nil, fmt.Errorf("unsupported record type: %s", qtype)
	}
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	qtypeNum, _ := dnsclient.ParseType(qtype)
	result := &dnsclient.TraceResult{
		Name: strings.TrimSuffix(name, "."),
		Type: qtype,
	}

	servers := make([]string, len(rootServers))
	copy(servers, rootServers)

	for step := 0; step < maxTraceSteps; step++ {
		if len(servers) == 0 {
			break
		}
		ts, next, done, err := c.traceStep(ctx, servers[0], name, qtypeNum, qtype)
		result.Steps = append(result.Steps, ts)
		if err != nil || done {
			break
		}
		if len(next) == 0 {
			break
		}
		servers = next
	}

	return result, nil
}

// traceStep sends one non-recursive query to serverIP and returns:
//   - the populated TraceStep
//   - the list of server IPs to query next (nil if authoritative or failed)
//   - done=true when we have an authoritative answer
//   - any hard error
func (c *Checker) traceStep(ctx context.Context, serverIP, name string, qtypeNum uint16, qtype string) (dnsclient.TraceStep, []string, bool, error) {
	ts := dnsclient.TraceStep{
		IP:        serverIP,
		QueryType: qtype,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(name, qtypeNum)
	msg.RecursionDesired = false
	msg.SetEdns0(4096, false)

	dc := &dns.Client{
		Net:     "udp",
		Timeout: c.cfg.DNS.QueryTimeout,
	}

	start := time.Now()
	resp, _, err := dc.ExchangeContext(ctx, msg, net.JoinHostPort(serverIP, "53"))
	ts.DurationMS = time.Since(start).Milliseconds()

	if err != nil {
		// Retry over TCP on truncation or timeout.
		dc.Net = "tcp"
		resp, _, err = dc.ExchangeContext(ctx, msg, net.JoinHostPort(serverIP, "53"))
		ts.DurationMS = time.Since(start).Milliseconds()
	}

	if err != nil {
		ts.Status = dnsclient.StatusError
		ts.Error = err.Error()
		return ts, nil, false, err
	}

	// Determine the zone label from the first NS record in the authority section.
	if len(resp.Ns) > 0 {
		ts.Zone = strings.TrimSuffix(resp.Ns[0].Header().Name, ".")
		if ns, ok := resp.Ns[0].(*dns.NS); ok {
			ts.Nameserver = strings.TrimSuffix(ns.Ns, ".")
		}
	} else if len(resp.Answer) > 0 {
		ts.Zone = strings.TrimSuffix(resp.Answer[0].Header().Name, ".")
		ts.Nameserver = serverIP
	} else {
		ts.Zone = "."
		ts.Nameserver = serverIP
	}

	ts.Authoritative = resp.Authoritative
	ts.Status = rcodeToStatus(resp.Rcode)

	for _, rr := range resp.Answer {
		ts.Answers = append(ts.Answers, dnsclient.RRToAnswer(rr))
	}
	for _, rr := range resp.Ns {
		ts.Authority = append(ts.Authority, dnsclient.RRToAnswer(rr))
	}
	for _, rr := range resp.Extra {
		if rr.Header().Rrtype == dns.TypeOPT {
			continue
		}
		ts.Additional = append(ts.Additional, dnsclient.RRToAnswer(rr))
	}

	if resp.Authoritative {
		return ts, nil, true, nil
	}

	// Gather glue records from the additional section. Prefer IPv4 over IPv6
	// so the trace works in environments without IPv6 connectivity.
	glueV4 := make(map[string]string)
	glueV6 := make(map[string]string)
	for _, rr := range resp.Extra {
		switch a := rr.(type) {
		case *dns.A:
			key := strings.ToLower(strings.TrimSuffix(a.Hdr.Name, "."))
			if !isPrivateOrLocal(a.A.String()) {
				glueV4[key] = a.A.String()
			}
		case *dns.AAAA:
			key := strings.ToLower(strings.TrimSuffix(a.Hdr.Name, "."))
			if !isPrivateOrLocal(a.AAAA.String()) {
				glueV6[key] = a.AAAA.String()
			}
		}
	}
	// Merge: IPv4 takes precedence.
	glue := make(map[string]string)
	for k, v := range glueV6 {
		glue[k] = v
	}
	for k, v := range glueV4 {
		glue[k] = v
	}

	// Build next-hop server list from NS records. We strongly prefer IPv4 so
	// that the trace works in environments without IPv6 connectivity.
	var next []string
	seen := make(map[string]bool)
	for _, rr := range resp.Ns {
		ns, ok := rr.(*dns.NS)
		if !ok {
			continue
		}
		nsName := strings.ToLower(strings.TrimSuffix(ns.Ns, "."))

		// 1. Use IPv4 glue first.
		if ip, ok := glueV4[nsName]; ok && !seen[ip] {
			seen[ip] = true
			next = append(next, ip)
			continue
		}

		// 2. Fall back to resolving IPv4 via the system resolver.
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, nsName)
		if err == nil {
			for _, a := range addrs {
				if a.IP.To4() != nil && !isPrivateOrLocal(a.IP.String()) && !seen[a.IP.String()] {
					seen[a.IP.String()] = true
					next = append(next, a.IP.String())
					break
				}
			}
			if len(next) > 0 {
				continue
			}
		}

		// 3. Last resort: use IPv6 glue.
		if ip, ok := glueV6[nsName]; ok && !seen[ip] {
			seen[ip] = true
			next = append(next, ip)
		}
	}

	return ts, next, false, nil
}

// rcodeToStatus maps a DNS RCODE integer to a status string.
// Duplicated from dnsclient/client.go to keep the checker package self-contained.
func rcodeToStatus(rcode int) string {
	switch rcode {
	case dns.RcodeSuccess:
		return dnsclient.StatusOK
	case dns.RcodeNameError:
		return dnsclient.StatusNXDomain
	case dns.RcodeServerFailure:
		return dnsclient.StatusServFail
	case dns.RcodeRefused:
		return dnsclient.StatusRefused
	default:
		return dnsclient.StatusError
	}
}
