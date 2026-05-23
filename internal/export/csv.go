package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/t0mer/dnsmon/internal/dnsclient"
)

// CSV writes a Check's results as CSV to w.
func CSV(w io.Writer, check *dnsclient.Check) error {
	cw := csv.NewWriter(w)

	if err := cw.Write([]string{
		"Resolver", "IP", "Country", "City",
		"Status", "Answer", "TTL", "Duration_ms", "Queried_At",
	}); err != nil {
		return fmt.Errorf("writing csv header: %w", err)
	}

	for _, r := range check.Results {
		answer := ""
		ttl := ""
		if len(r.Answers) > 0 {
			vals := make([]string, len(r.Answers))
			for i, a := range r.Answers {
				vals[i] = a.Value
			}
			answer = strings.Join(vals, "; ")
			ttl = fmt.Sprintf("%d", r.Answers[0].TTL)
		}

		record := []string{
			r.Resolver.Name,
			r.Resolver.IP,
			r.Resolver.Country,
			r.Resolver.City,
			r.Status,
			answer,
			ttl,
			fmt.Sprintf("%d", r.DurationMS),
			r.QueriedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if err := cw.Write(record); err != nil {
			return fmt.Errorf("writing csv record: %w", err)
		}
	}

	cw.Flush()
	return cw.Error()
}
