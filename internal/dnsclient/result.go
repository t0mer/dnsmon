package dnsclient

import "time"

// TraceStep represents one delegation hop in an authoritative trace.
type TraceStep struct {
	Zone          string   `json:"zone"`
	Nameserver    string   `json:"nameserver"`
	IP            string   `json:"ip"`
	QueryType     string   `json:"query_type"`
	Answers       []Answer `json:"answers,omitempty"`
	Authority     []Answer `json:"authority,omitempty"`
	Additional    []Answer `json:"additional,omitempty"`
	Authoritative bool     `json:"authoritative"`
	DurationMS    int64    `json:"duration_ms"`
	Status        string   `json:"status"`
	Error         string   `json:"error,omitempty"`
}

// TraceResult is the full delegation chain for a name/type pair.
type TraceResult struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Steps []TraceStep `json:"steps"`
}

const (
	StatusOK       = "ok"
	StatusNXDomain = "nxdomain"
	StatusTimeout  = "timeout"
	StatusServFail = "servfail"
	StatusRefused  = "refused"
	StatusError    = "error"
)

// Resolver represents a DNS resolver with location metadata.
type Resolver struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	IP       string  `json:"ip"`
	Port     int     `json:"port"`
	Protocol string  `json:"protocol"`
	Country  string  `json:"country"`
	City     string  `json:"city"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	ASN      uint32  `json:"asn,omitempty"`
	ISP      string  `json:"isp,omitempty"`
}

// Answer is a single DNS resource record answer.
type Answer struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	TTL   uint32 `json:"ttl"`
	Value string `json:"value"`
}

// ResolverResult is the response from one resolver.
type ResolverResult struct {
	Resolver   Resolver `json:"resolver"`
	Status     string   `json:"status"`
	Answers    []Answer `json:"answers"`
	Authority  []Answer `json:"authority,omitempty"`
	Additional []Answer `json:"additional,omitempty"`
	DNSSEC     *bool    `json:"dnssec,omitempty"`
	DurationMS int64    `json:"duration_ms"`
	Error      string   `json:"error,omitempty"`
	QueriedAt  time.Time `json:"queried_at"`
}

// Check is a complete propagation check result.
type Check struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Type      string           `json:"type"`
	CreatedAt time.Time        `json:"created_at"`
	Results   []ResolverResult `json:"results"`
	Summary   CheckSummary     `json:"summary"`
}

// CheckSummary aggregates the results of a Check.
type CheckSummary struct {
	TotalResolvers   int            `json:"total_resolvers"`
	Responded        int            `json:"responded"`
	NXDomain         int            `json:"nxdomain"`
	Timeouts         int            `json:"timeouts"`
	Errors           int            `json:"errors"`
	UniqueAnswerSets int            `json:"unique_answer_sets"`
	Consensus        map[string]int `json:"consensus"`
	// ConvergedPct is the percentage of responding resolvers serving the majority answer.
	ConvergedPct int `json:"converged_pct"`
	// PropagationETA is the estimated seconds until full propagation, based on the maximum
	// TTL observed across resolvers that diverge from the majority answer. Zero means
	// already fully propagated; nil means not applicable (no majority / no responses).
	PropagationETA *int64 `json:"propagation_eta,omitempty"`
}
