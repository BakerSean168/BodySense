package main

import "testing"

func TestParseCORSOriginsPrefersPluralEnv(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "http://localhost:5173, https://app.example.com")
	t.Setenv("CORS_ORIGIN", "http://legacy.example.com")

	origins := parseCORSOrigins()
	if len(origins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(origins))
	}
	if origins[0] != "http://localhost:5173" || origins[1] != "https://app.example.com" {
		t.Fatalf("unexpected origins: %#v", origins)
	}
}

func TestResolveCORSOriginMatchesRequestOrigin(t *testing.T) {
	allowed := []string{"http://localhost:5173", "https://app.example.com"}

	got := resolveCORSOrigin("https://app.example.com", allowed)
	if got != "https://app.example.com" {
		t.Fatalf("origin = %q, want app origin", got)
	}
}

func TestResolveCORSOriginWildcardReflectsRequestForCredentialedCORS(t *testing.T) {
	got := resolveCORSOrigin("https://app.example.com", []string{"*"})
	if got != "https://app.example.com" {
		t.Fatalf("origin = %q, want reflected request origin", got)
	}
}

func TestResolveCORSOriginRejectsUnlistedOrigin(t *testing.T) {
	got := resolveCORSOrigin("https://evil.example.com", []string{"https://app.example.com"})
	if got != "" {
		t.Fatalf("origin = %q, want empty for unlisted origin", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1, 172.16.0.0/12")
	got := parseTrustedProxies()
	if len(got) != 2 || got[0] != "127.0.0.1" || got[1] != "172.16.0.0/12" {
		t.Fatalf("trusted proxies = %#v", got)
	}
}

func TestResolveAPIListenAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
		want string
	}{
		{name: "host dev defaults to loopback", want: "127.0.0.1:8080"},
		{name: "explicit docker bind", host: "0.0.0.0", port: "8080", want: "0.0.0.0:8080"},
		{name: "project dev port", host: "127.0.0.1", port: "20101", want: "127.0.0.1:20101"},
		{name: "ipv6 loopback", host: "::1", port: "20101", want: "[::1]:20101"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveAPIListenAddress(tt.host, tt.port); got != tt.want {
				t.Fatalf("resolveAPIListenAddress(%q, %q) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}
