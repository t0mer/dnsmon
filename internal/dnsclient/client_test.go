package dnsclient_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomerklein/gdns/internal/config"
	"github.com/tomerklein/gdns/internal/dnsclient"
)

func newTestConfig(timeout time.Duration) *config.Config {
	return &config.Config{
		DNS: config.DNSConfig{
			QueryTimeout:           timeout,
			PerResolverConcurrency: 4,
			DefaultProtocol:        "udp",
			Retry:                  0,
		},
	}
}

func startTestServer(t *testing.T, handler dns.HandlerFunc) (string, func()) {
	t.Helper()

	mux := dns.NewServeMux()
	mux.HandleFunc(".", handler)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &dns.Server{
		PacketConn: pc,
		Net:        "udp",
		Handler:    mux,
	}

	started := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(started) }

	go func() {
		_ = srv.ActivateAndServe()
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("test DNS server did not start in time")
	}

	addr := pc.LocalAddr().String()
	return addr, func() { _ = srv.Shutdown() }
}

func resolverAt(addr string) dnsclient.Resolver {
	host, portStr, _ := net.SplitHostPort(addr)
	port := 53
	if portStr != "" {
		_, _ = net.LookupPort("udp", portStr)
		p, err := net.LookupPort("udp", portStr)
		if err == nil {
			port = p
		}
	}
	return dnsclient.Resolver{
		ID:       "test",
		Name:     "Test",
		IP:       host,
		Port:     port,
		Protocol: "udp",
	}
}

func TestQuery_Success(t *testing.T) {
	addr, stop := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		if len(r.Question) > 0 && r.Question[0].Qtype == dns.TypeA {
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   r.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				A: net.ParseIP("1.2.3.4"),
			})
		}
		_ = w.WriteMsg(m)
	})
	defer stop()

	c := dnsclient.New(newTestConfig(3 * time.Second))
	result, err := c.Query(context.Background(), resolverAt(addr), "test.example.", dns.TypeA)

	require.NoError(t, err)
	assert.Equal(t, dnsclient.StatusOK, result.Status)
	require.Len(t, result.Answers, 1)
	assert.Equal(t, "A", result.Answers[0].Type)
	assert.Equal(t, "1.2.3.4", result.Answers[0].Value)
	assert.Equal(t, uint32(300), result.Answers[0].TTL)
	assert.GreaterOrEqual(t, result.DurationMS, int64(0))
	assert.False(t, result.QueriedAt.IsZero())
}

func TestQuery_NXDOMAIN(t *testing.T) {
	addr, stop := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeNameError
		_ = w.WriteMsg(m)
	})
	defer stop()

	c := dnsclient.New(newTestConfig(3 * time.Second))
	result, err := c.Query(context.Background(), resolverAt(addr), "nonexistent.example.", dns.TypeA)

	require.NoError(t, err)
	assert.Equal(t, dnsclient.StatusNXDomain, result.Status)
	assert.Empty(t, result.Answers)
}

func TestQuery_ContextCancel(t *testing.T) {
	addr, stop := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		time.Sleep(500 * time.Millisecond)
		m := new(dns.Msg)
		m.SetReply(r)
		_ = w.WriteMsg(m)
	})
	defer stop()

	ctx, cancel := context.WithCancel(context.Background())

	c := dnsclient.New(newTestConfig(3 * time.Second))

	cancel()

	result, err := c.Query(ctx, resolverAt(addr), "test.example.", dns.TypeA)

	require.NoError(t, err)
	assert.Equal(t, dnsclient.StatusTimeout, result.Status)
}

func TestQuery_Timeout(t *testing.T) {
	addr, stop := startTestServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		time.Sleep(200 * time.Millisecond)
	})
	defer stop()

	c := dnsclient.New(newTestConfig(50 * time.Millisecond))
	result, err := c.Query(context.Background(), resolverAt(addr), "test.example.", dns.TypeA)

	require.NoError(t, err)
	assert.Equal(t, dnsclient.StatusTimeout, result.Status)
}
