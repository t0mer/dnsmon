package v1

import (
	"net/http"
	"sort"

	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/dnsclient"
)

type recordTypeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RecordTypes handles GET /api/v1/record-types.
func RecordTypes() http.HandlerFunc {
	types := buildRecordTypes()
	return func(w http.ResponseWriter, r *http.Request) {
		apierr.WriteJSON(w, http.StatusOK, types)
	}
}

func buildRecordTypes() []recordTypeInfo {
	names := dnsclient.SupportedTypeNames()
	sort.Strings(names)
	out := make([]recordTypeInfo, 0, len(names))
	for _, name := range names {
		desc := dnsclient.TypeDescriptions[name]
		out = append(out, recordTypeInfo{Name: name, Description: desc})
	}
	return out
}
