package main

import "testing"

func TestApplyPortOverride(t *testing.T) {
	tests := []struct {
		name   string
		listen string
		port   int
		want   string
	}{
		{"port-only listen", ":8080", 9000, ":9000"},
		{"host and port listen", "0.0.0.0:8080", 9000, "0.0.0.0:9000"},
		{"named host listen", "localhost:8080", 1234, "localhost:1234"},
		{"empty listen", "", 8080, ":8080"},
		{"zero port is no-op", ":8080", 0, ":8080"},
		{"negative port is no-op", "0.0.0.0:8080", -1, "0.0.0.0:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyPortOverride(tt.listen, tt.port); got != tt.want {
				t.Errorf("applyPortOverride(%q, %d) = %q, want %q", tt.listen, tt.port, got, tt.want)
			}
		})
	}
}

func TestServiceArguments(t *testing.T) {
	got := serviceArguments("", "", ":8080", 9000, "debug")
	want := []string{"--listen", ":8080", "--port", "9000", "--log-level", "debug"}
	if len(got) != len(want) {
		t.Fatalf("serviceArguments length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("serviceArguments[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
