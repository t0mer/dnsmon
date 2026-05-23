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

func TestResolveListen(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		listenFlag string
		portFlag   int
		portEnv    string
		want       string
		wantErr    bool
	}{
		{"nothing set keeps config", ":8080", "", 0, "", ":8080", false},
		{"PORT env overrides config port", ":8080", "", 0, "9000", ":9000", false},
		{"PORT env preserves host", "127.0.0.1:8080", "", 0, "9000", "127.0.0.1:9000", false},
		{"--listen overrides PORT env", ":8080", "0.0.0.0:7000", 0, "9000", "0.0.0.0:7000", false},
		{"--port overrides everything", ":8080", "0.0.0.0:7000", 6000, "9000", "0.0.0.0:6000", false},
		{"--port overrides PORT env", ":8080", "", 6000, "9000", ":6000", false},
		{"invalid PORT errors", ":8080", "", 0, "abc", "", true},
		{"zero PORT errors", ":8080", "", 0, "0", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveListen(tt.configured, tt.listenFlag, tt.portFlag, tt.portEnv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveListen(%q,%q,%d,%q) expected error, got %q",
						tt.configured, tt.listenFlag, tt.portFlag, tt.portEnv, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveListen returned unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveListen(%q,%q,%d,%q) = %q, want %q",
					tt.configured, tt.listenFlag, tt.portFlag, tt.portEnv, got, tt.want)
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
