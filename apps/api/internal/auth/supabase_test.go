package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newSupabaseJWTFixture(t *testing.T) (*SupabaseVerifier, *ecdsa.PrivateKey, string, *atomic.Int32) {
	t.Helper()
	signingKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/auth/v1/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		x := signingKey.PublicKey.X.FillBytes(make([]byte, 32))
		y := signingKey.PublicKey.Y.FillBytes(make([]byte, 32))
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "supabase-kid-test", "kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig",
			"x": base64.RawURLEncoding.EncodeToString(x),
			"y": base64.RawURLEncoding.EncodeToString(y),
		}}})
	}))
	t.Cleanup(server.Close)
	verifier, err := NewSupabaseVerifier(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return verifier, signingKey, server.URL + "/auth/v1", &calls
}

func signSupabaseTestToken(t *testing.T, key *ecdsa.PrivateKey, c SupabaseClaims, kid string) string {
	t.Helper()
	signed := jwt.NewWithClaims(jwt.SigningMethodES256, c)
	signed.Header["kid"] = kid
	str, err := signed.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return str
}

func validSupabaseClaims(issuer string) SupabaseClaims {
	now := time.Now().UTC()
	return SupabaseClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{"authenticated"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
		Email: "check@example.test",
		Role:  "authenticated",
	}
}

func TestSupabaseVerifierES256SignatureIssuerAudienceAndRole(t *testing.T) {
	verifier, key, issuer, calls := newSupabaseJWTFixture(t)
	claims := validSupabaseClaims(issuer)
	value := signSupabaseTestToken(t, key, claims, "supabase-kid-test")
	got, err := verifier.Verify(context.Background(), value)
	if err != nil || got.Subject != claims.Subject {
		t.Fatalf("valid Supabase token rejected: got=%+v err=%v", got, err)
	}
	if _, err := verifier.Verify(context.Background(), value); err != nil {
		t.Fatalf("cached public key must remain valid: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("JWKS must be cached: calls=%d", calls.Load())
	}

	tests := map[string]func(*SupabaseClaims){
		"wrong issuer":     func(c *SupabaseClaims) { c.Issuer = "https://other.supabase.co/auth/v1" },
		"wrong audience":   func(c *SupabaseClaims) { c.Audience = jwt.ClaimStrings{"anon"} },
		"expired":          func(c *SupabaseClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)) },
		"wrong role":       func(c *SupabaseClaims) { c.Role = "service_role" },
		"anonymous":        func(c *SupabaseClaims) { c.IsAnonymous = true },
		"missing email":    func(c *SupabaseClaims) { c.Email = "" },
		"invalid subject":  func(c *SupabaseClaims) { c.Subject = "not-a-uuid" },
		"future issued-at": func(c *SupabaseClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(10 * time.Minute)) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			bad := validSupabaseClaims(issuer)
			mutate(&bad)
			token := signSupabaseTestToken(t, key, bad, "supabase-kid-test")
			if _, err := verifier.Verify(context.Background(), token); err == nil {
				t.Fatal("bad token was accepted")
			}
		})
	}
}

func TestSupabaseVerifierRejectsForgeryAndUnknownSigningKey(t *testing.T) {
	verifier, key, issuer, _ := newSupabaseJWTFixture(t)
	good := validSupabaseClaims(issuer)
	token := signSupabaseTestToken(t, key, good, "supabase-kid-test")
	if _, err := verifier.Verify(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	alt, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	forged := signSupabaseTestToken(t, alt, good, "supabase-kid-test")
	if _, err := verifier.Verify(context.Background(), forged); err == nil {
		t.Fatal("forged signature accepted")
	}
	unknown := signSupabaseTestToken(t, key, good, "unknown-kid")
	if _, err := verifier.Verify(context.Background(), unknown); err == nil {
		t.Fatal("unknown signing key accepted")
	}

	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, good)
	hs.Header["kid"] = "supabase-kid-test"
	hsToken, err := hs.SignedString([]byte("untrusted-client-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), hsToken); err == nil {
		t.Fatal("HS256 algorithm confusion accepted")
	}
	if _, err := verifier.Verify(context.Background(), strings.Repeat("a", 8193)); err == nil {
		t.Fatal("oversized token accepted")
	}
}

func TestSupabaseVerifierFailsClosedWhenJWKSUnavailable(t *testing.T) {
	v, key, issuer, _ := newSupabaseJWTFixture(t)
	token := signSupabaseTestToken(t, key, validSupabaseClaims(issuer), "supabase-kid-test")
	v.client = &http.Client{Transport: failingSupabaseRoundTripper{}}
	if _, err := v.Verify(context.Background(), token); err == nil {
		t.Fatal("unavailable JWKS must deny first token")
	}
}

type failingSupabaseRoundTripper struct{}

func (failingSupabaseRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

func TestSupabaseConfigRejectsUnsafeHosts(t *testing.T) {
	for _, raw := range []string{"", "http://localhost:9999", "https://example.com/auth/v1", "https://example.com/?key=1", "https://user@exam.ple"} {
		if _, err := NewSupabaseVerifier(raw, nil); err == nil {
			t.Fatalf("accepted invalid Supabase origin %q", raw)
		}
	}
	t.Setenv("SUPABASE_URL", "https://example.supabase.co")
	if _, err := SupabaseConfigFromEnv(); err != nil {
		t.Fatalf("valid hosted project URL rejected: %v", err)
	}
}

func TestSupabaseLoopbackAuthDoubleIsStrictlyTestOnly(t *testing.T) {
	local := "http://127.0.0.1:18120"
	if _, err := NewSupabaseVerifier(local, nil); err == nil {
		t.Fatal("cleartext identity accepted in ordinary runtime")
	}
	t.Setenv("APP_ENV", "test")
	t.Setenv("BODYSENSE_E2E_STUB_AUTH", "1")
	if _, err := NewSupabaseVerifier(local, nil); err != nil {
		t.Fatalf("isolated test-only loopback origin rejected: %v", err)
	}
	for _, remote := range []string{
		"http://example.supabase.co", "http://192.0.2.1:18120", "http://localhost:18120",
	} {
		if _, err := NewSupabaseVerifier(remote, nil); err == nil {
			t.Fatalf("non-literal-loopback or remote cleartext origin accepted %q", remote)
		}
	}
	t.Setenv("APP_ENV", "production")
	if _, err := NewSupabaseVerifier(local, nil); err == nil {
		t.Fatal("production must reject cleartext Supabase URL, even with stub flag")
	}
}

func TestSupabaseAdminRevokerFailClosedAndIdempotent(t *testing.T) {
	id := uuid.New()
	missing := NewSupabaseAdminRevoker(SupabaseConfig{URL: "https://example.supabase.co"}, nil)
	if missing.Ready() == nil {
		t.Fatal("deletion must not work without secret")
	}
	if err := missing.RevokeUser(context.Background(), id); err == nil {
		t.Fatal("empty secret allowed")
	}
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodDelete || r.URL.Path != "/auth/v1/admin/users/"+id.String() ||
			r.Header.Get("apikey") != "test-admin-secret" ||
			r.Header.Get("Authorization") != "Bearer test-admin-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	revoker := NewSupabaseAdminRevoker(SupabaseConfig{URL: server.URL, SecretKey: "test-admin-secret"}, server.Client())
	if err := revoker.RevokeUser(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("admin call missing")
	}
}
