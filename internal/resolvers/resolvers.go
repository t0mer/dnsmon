package resolvers

import (
	"fmt"

	"github.com/t0mer/dnsmon/internal/dnsclient"
)

// DefaultPort is the standard DNS port.
const DefaultPort = 53

// Addr returns the "ip:port" address for dialing the resolver.
func Addr(r dnsclient.Resolver) string {
	port := r.Port
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("%s:%d", r.IP, port)
}

// IsValid reports whether the resolver has the minimum required fields.
func IsValid(r dnsclient.Resolver) bool {
	return r.IP != ""
}
