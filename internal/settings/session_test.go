package settings

import (
	"testing"
	"time"
)

func TestSignParseSession(t *testing.T) {
	secret, _ := GenerateToken()

	token := SignSession(secret, "admin", time.Hour)
	name, ok := ParseSession(secret, token)
	if !ok || name != "admin" {
		t.Fatalf("ParseSession = (%q, %v), want (admin, true)", name, ok)
	}
}

func TestParseSessionRejects(t *testing.T) {
	secret, _ := GenerateToken()
	token := SignSession(secret, "admin", time.Hour)

	// Wrong secret.
	if _, ok := ParseSession("other-secret", token); ok {
		t.Error("expected failure with wrong secret")
	}
	// Tampered payload (valid token with its payload swapped).
	if _, ok := ParseSession(secret, "AAAA."+token); ok {
		t.Error("expected failure with tampered payload")
	}
	// Garbage.
	for _, bad := range []string{"", "nodot", "a.b.c"} {
		if _, ok := ParseSession(secret, bad); ok {
			t.Errorf("ParseSession(%q) = true, want false", bad)
		}
	}
	// Expired.
	expired := SignSession(secret, "admin", -time.Minute)
	if _, ok := ParseSession(secret, expired); ok {
		t.Error("expected failure for expired token")
	}
}
