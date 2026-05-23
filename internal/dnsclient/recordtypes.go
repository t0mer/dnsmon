package dnsclient

import (
	"sort"

	"github.com/miekg/dns"
)

// SupportedTypes maps DNS record type names to their numeric identifiers.
var SupportedTypes = map[string]uint16{
	"A":      dns.TypeA,
	"AAAA":   dns.TypeAAAA,
	"CNAME":  dns.TypeCNAME,
	"MX":     dns.TypeMX,
	"NS":     dns.TypeNS,
	"PTR":    dns.TypePTR,
	"SOA":    dns.TypeSOA,
	"TXT":    dns.TypeTXT,
	"CAA":    dns.TypeCAA,
	"SRV":    dns.TypeSRV,
	"DS":     dns.TypeDS,
	"DNSKEY": dns.TypeDNSKEY,
	"NAPTR":  dns.TypeNAPTR,
	"TLSA":   dns.TypeTLSA,
	"SVCB":   dns.TypeSVCB,
	"HTTPS":  dns.TypeHTTPS,
}

// TypeDescriptions maps record type names to human-readable descriptions.
var TypeDescriptions = map[string]string{
	"A":      "IPv4 address record",
	"AAAA":   "IPv6 address record",
	"CNAME":  "Canonical name (alias) record",
	"MX":     "Mail exchange record",
	"NS":     "Name server record",
	"PTR":    "Reverse DNS pointer record",
	"SOA":    "Start of authority record",
	"TXT":    "Text record",
	"CAA":    "Certification Authority Authorization record",
	"SRV":    "Service locator record",
	"DS":     "Delegation signer record (DNSSEC)",
	"DNSKEY": "DNS public key record (DNSSEC)",
	"NAPTR":  "Naming authority pointer record",
	"TLSA":   "TLS authentication record",
	"SVCB":   "Service binding record",
	"HTTPS":  "HTTPS service binding record",
}

// ParseType returns the numeric DNS type for the given name, and whether it is supported.
func ParseType(name string) (uint16, bool) {
	t, ok := SupportedTypes[name]
	return t, ok
}

// TypeName returns the string name for a numeric DNS type, or the numeric value as a string if unknown.
func TypeName(t uint16) string {
	return dns.TypeToString[t]
}

// SupportedTypeNames returns a sorted slice of all supported record type names.
func SupportedTypeNames() []string {
	names := make([]string, 0, len(SupportedTypes))
	for name := range SupportedTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
