package resolvers

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/t0mer/dnsmon/internal/dnsclient"
)

//go:embed resolvers.json
var builtinJSON []byte

// LoadBuiltin parses the embedded resolvers.json and returns all resolvers.
func LoadBuiltin() ([]dnsclient.Resolver, error) {
	var resolvers []dnsclient.Resolver
	if err := json.Unmarshal(builtinJSON, &resolvers); err != nil {
		return nil, fmt.Errorf("parsing builtin resolvers: %w", err)
	}
	return resolvers, nil
}
