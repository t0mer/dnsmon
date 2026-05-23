package v1

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/tomerklein/gdns/internal/api/apierr"
	"github.com/tomerklein/gdns/internal/checker"
)

// ReverseRequest is the request body for POST /api/v1/reverse.
type ReverseRequest struct {
	IP        string   `json:"ip" validate:"required,ip"`
	Resolvers []string `json:"resolvers"`
}

// PostReverse handles POST /api/v1/reverse.
func PostReverse(chkr *checker.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ReverseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				"Invalid request body.", nil)
			return
		}

		if err := validate.Struct(req); err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				err.Error(), nil)
			return
		}

		ptrName, err := toPTRName(req.IP)
		if err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				fmt.Sprintf("Cannot convert IP to PTR name: %s", err), nil)
			return
		}

		check, err := chkr.Check(r.Context(), checker.CheckRequest{
			Name:        ptrName,
			Type:        "PTR",
			ResolverIDs: req.Resolvers,
			Save:        false,
			AllowPrivate: true,
		})
		if err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				err.Error(), nil)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, check)
	}
}

// toPTRName converts an IPv4 or IPv6 address to its PTR query name.
func toPTRName(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipStr)
	}

	if v4 := ip.To4(); v4 != nil {
		// Reverse the octets and append .in-addr.arpa.
		parts := strings.Split(v4.String(), ".")
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		return strings.Join(parts, ".") + ".in-addr.arpa.", nil
	}

	// IPv6: expand to full form, reverse nibbles, append .ip6.arpa.
	v6 := ip.To16()
	if v6 == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipStr)
	}
	hex := fmt.Sprintf("%x", []byte(v6))
	nibbles := make([]string, len(hex))
	for i, c := range hex {
		nibbles[i] = string(c)
	}
	for i, j := 0, len(nibbles)-1; i < j; i, j = i+1, j-1 {
		nibbles[i], nibbles[j] = nibbles[j], nibbles[i]
	}
	return strings.Join(nibbles, ".") + ".ip6.arpa.", nil
}
