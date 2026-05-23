package dnsclient

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/tomerklein/gdns/internal/config"
)

// Client performs DNS queries.
type Client interface {
	Query(ctx context.Context, resolver Resolver, name string, qtype uint16) (*ResolverResult, error)
}

type client struct {
	cfg *config.Config
}

// New returns a new DNS client.
func New(cfg *config.Config) Client {
	return &client{cfg: cfg}
}

func (c *client) Query(ctx context.Context, resolver Resolver, name string, qtype uint16) (*ResolverResult, error) {
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	msg := new(dns.Msg)
	msg.SetQuestion(name, qtype)
	msg.RecursionDesired = true
	msg.SetEdns0(4096, false)

	port := resolver.Port
	if port == 0 {
		port = 53
	}

	proto := resolver.Protocol
	if proto == "" {
		proto = c.cfg.DNS.DefaultProtocol
	}
	if proto == "dot" {
		proto = "tcp-tls"
	}

	server := net.JoinHostPort(resolver.IP, fmt.Sprintf("%d", port))
	dc := &dns.Client{
		Net:     proto,
		Timeout: c.cfg.DNS.QueryTimeout,
	}

	maxAttempts := 1 + c.cfg.DNS.Retry
	var (
		resp *dns.Msg
		rtt  time.Duration
		err  error
	)

	start := time.Now()
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return &ResolverResult{
				Resolver:   resolver,
				Status:     StatusTimeout,
				DurationMS: time.Since(start).Milliseconds(),
				Error:      ctx.Err().Error(),
				QueriedAt:  start,
			}, nil
		default:
		}

		resp, rtt, err = dc.ExchangeContext(ctx, msg, server)
		if err == nil {
			break
		}

		if isTimeout(err) && attempt < maxAttempts-1 {
			continue
		}
		break
	}

	queriedAt := start
	durationMS := rtt.Milliseconds()

	if err != nil {
		status := StatusError
		if isTimeout(err) {
			status = StatusTimeout
		}
		return &ResolverResult{
			Resolver:   resolver,
			Status:     status,
			DurationMS: durationMS,
			Error:      err.Error(),
			QueriedAt:  queriedAt,
		}, nil
	}

	result := &ResolverResult{
		Resolver:   resolver,
		DurationMS: durationMS,
		QueriedAt:  queriedAt,
	}

	result.Status = rcodeToStatus(resp.Rcode)

	if resp.AuthenticatedData {
		t := true
		result.DNSSEC = &t
	}

	for _, rr := range resp.Answer {
		result.Answers = append(result.Answers, rrToAnswer(rr))
	}
	for _, rr := range resp.Ns {
		result.Authority = append(result.Authority, rrToAnswer(rr))
	}
	for _, rr := range resp.Extra {
		if rr.Header().Rrtype == dns.TypeOPT {
			continue
		}
		result.Additional = append(result.Additional, rrToAnswer(rr))
	}

	return result, nil
}

func rcodeToStatus(rcode int) string {
	switch rcode {
	case dns.RcodeSuccess:
		return StatusOK
	case dns.RcodeNameError:
		return StatusNXDomain
	case dns.RcodeServerFailure:
		return StatusServFail
	case dns.RcodeRefused:
		return StatusRefused
	default:
		return StatusError
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}

func rrToAnswer(rr dns.RR) Answer {
	hdr := rr.Header()
	a := Answer{
		Name: hdr.Name,
		Type: dns.TypeToString[hdr.Rrtype],
		TTL:  hdr.Ttl,
	}

	switch v := rr.(type) {
	case *dns.A:
		a.Value = v.A.String()
	case *dns.AAAA:
		a.Value = v.AAAA.String()
	case *dns.CNAME:
		a.Value = v.Target
	case *dns.MX:
		a.Value = fmt.Sprintf("%d %s", v.Preference, v.Mx)
	case *dns.NS:
		a.Value = v.Ns
	case *dns.PTR:
		a.Value = v.Ptr
	case *dns.SOA:
		a.Value = fmt.Sprintf("%s %s %d %d %d %d %d",
			v.Ns, v.Mbox, v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl)
	case *dns.TXT:
		a.Value = strings.Join(v.Txt, " ")
	case *dns.CAA:
		a.Value = fmt.Sprintf("%d %s %q", v.Flag, v.Tag, v.Value)
	case *dns.SRV:
		a.Value = fmt.Sprintf("%d %d %d %s", v.Priority, v.Weight, v.Port, v.Target)
	case *dns.DS:
		a.Value = fmt.Sprintf("%d %d %d %s", v.KeyTag, v.Algorithm, v.DigestType, v.Digest)
	case *dns.DNSKEY:
		a.Value = fmt.Sprintf("%d %d %d %s", v.Flags, v.Protocol, v.Algorithm, v.PublicKey)
	case *dns.NAPTR:
		a.Value = fmt.Sprintf("%d %d %q %q %q %s", v.Order, v.Preference, v.Flags, v.Service, v.Regexp, v.Replacement)
	case *dns.TLSA:
		a.Value = fmt.Sprintf("%d %d %d %s", v.Usage, v.Selector, v.MatchingType, v.Certificate)
	case *dns.SVCB:
		a.Value = rr.String()
	case *dns.HTTPS:
		a.Value = rr.String()
	default:
		a.Value = rr.String()
	}

	return a
}
