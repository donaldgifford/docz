package oauth

import "testing"

func TestMatchRedirect(t *testing.T) {
	t.Parallel()

	registered := []string{"https://agent.example/cb", "http://127.0.0.1:9999/cb", "http://localhost/callback", "http://[::1]:7/cb"}
	tests := []struct {
		name      string
		requested string
		want      string
		ok        bool
	}{
		{"exact https", "https://agent.example/cb", "https://agent.example/cb", true},
		{"exact loopback", "http://127.0.0.1:9999/cb", "http://127.0.0.1:9999/cb", true},
		{"loopback other port", "http://127.0.0.1:54321/cb", "http://127.0.0.1:54321/cb", true},
		{"localhost gains a port", "http://localhost:8080/callback", "http://localhost:8080/callback", true},
		{"ipv6 loopback", "http://[::1]:65535/cb", "http://[::1]:65535/cb", true},
		{"loopback other path", "http://127.0.0.1:54321/other", "", false},
		{"loopback other query", "http://127.0.0.1:54321/cb?x=1", "", false},
		{"loopback with fragment", "http://127.0.0.1:54321/cb#f", "", false},
		{"loopback no port", "http://127.0.0.1/cb", "", false},
		{"port zero", "http://127.0.0.1:0/cb", "", false},
		{"port out of range", "http://127.0.0.1:70000/cb", "", false},
		{"https other port", "https://agent.example:8443/cb", "", false},
		{"https on a loopback path", "https://127.0.0.1:54321/cb", "", false},
		{"other host", "http://127.0.0.2:54321/cb", "", false},
		{"not a URL", "::nope", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := matchRedirect(registered, tt.requested)
			if got != tt.want || ok != tt.ok {
				t.Errorf("matchRedirect(%q) = %q, %v, want %q, %v", tt.requested, got, ok, tt.want, tt.ok)
			}
		})
	}
}
