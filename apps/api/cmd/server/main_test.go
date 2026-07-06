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

func TestResolveCORSOriginWildcard(t *testing.T) {
	got := resolveCORSOrigin("https://app.example.com", []string{"*"})
	if got != "*" {
		t.Fatalf("origin = %q, want wildcard", got)
	}
}
