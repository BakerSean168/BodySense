package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const supabaseJWKSCacheTTL = 8 * time.Minute

// SupabaseConfig contains only the project endpoint and two separate scopes
// of credentials: a public project URL for verifying JWKS and an optional
// server secret for permanent account deletion. The browser publishable key
// is never required by Go; never send the admin secret to the browser.
type SupabaseConfig struct {
	URL       string
	SecretKey string
}

func SupabaseConfigFromEnv() (SupabaseConfig, error) {
	cfg := SupabaseConfig{
		URL:       strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_URL")), "/"),
		SecretKey: strings.TrimSpace(os.Getenv("SUPABASE_SECRET_KEY")),
	}
	if _, err := checkSupabaseURL(cfg.URL); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func checkSupabaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return nil, fmt.Errorf("SUPABASE_URL must be an origin (without a path or query)")
	}
	if u.Scheme != "https" {
		// A deterministic, *loopback-only* Auth double is required for the
		// existing isolated browser E2E checks. Never allow cleartext hosted
		// identity traffic on a production or ordinary development server.
		ip := net.ParseIP(u.Hostname())
		loopback := ip != nil && ip.IsLoopback()
		if u.Scheme != "http" || !loopback || os.Getenv("APP_ENV") != "test" ||
			os.Getenv("BODYSENSE_E2E_STUB_AUTH") != "1" {
			return nil, fmt.Errorf("SUPABASE_URL must use https (test-only loopback stub excluded)")
		}
	}
	return u, nil
}

// SupabaseClaims are the signed identity attributes that Go is allowed to
// consume. Product authorization remains in the local users table.
type SupabaseClaims struct {
	jwt.RegisteredClaims
	Email       string `json:"email"`
	Role        string `json:"role"`
	IsAnonymous bool   `json:"is_anonymous"`
}

type jwkRecord struct {
	KeyID     string `json:"kid"`
	KeyType   string `json:"kty"`
	Algorithm string `json:"alg"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
	Use       string `json:"use"`
}

type SupabaseVerifier struct {
	issuer              string
	jwksURL             string
	client              *http.Client
	mu                  sync.Mutex
	keys                map[string]*ecdsa.PublicKey
	loadedAt            time.Time
	lastUnknownKeyCheck time.Time
}

func NewSupabaseVerifier(baseURL string, client *http.Client) (*SupabaseVerifier, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if _, err := checkSupabaseURL(baseURL); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &SupabaseVerifier{
		issuer:  baseURL + "/auth/v1",
		jwksURL: baseURL + "/auth/v1/.well-known/jwks.json",
		client:  client,
	}, nil
}

func (v *SupabaseVerifier) Verify(ctx context.Context, bearer string) (*SupabaseClaims, error) {
	if len(bearer) == 0 || len(bearer) > 8192 {
		return nil, errors.New("missing or oversized Supabase access token")
	}
	claims := new(SupabaseClaims)
	token, err := jwt.ParseWithClaims(bearer, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodES256.Alg() {
			return nil, errors.New("unsupported Supabase JWT signing algorithm")
		}
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 256 {
			return nil, errors.New("missing Supabase signing key id")
		}
		return v.key(ctx, kid)
	}, jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("invalid Supabase access token: %w", err)
	}
	if token == nil || !token.Valid {
		return nil, errors.New("invalid Supabase access token")
	}
	subject, err := uuid.Parse(claims.Subject)
	if err != nil || subject == uuid.Nil || claims.Role != "authenticated" ||
		claims.IsAnonymous || strings.TrimSpace(claims.Email) == "" {
		return nil, errors.New("Supabase token is not an authenticated email user")
	}
	return claims, nil
}

func (v *SupabaseVerifier) key(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	if now.Sub(v.loadedAt) < supabaseJWKSCacheTTL {
		if k, ok := v.keys[kid]; ok {
			return k, nil
		}
		// A forged kid must not force one outbound request per attempt.
		if now.Sub(v.lastUnknownKeyCheck) < 30*time.Second {
			return nil, errors.New("unknown Supabase signing key")
		}
	}
	v.lastUnknownKeyCheck = now
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Supabase JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Supabase JWKS returned %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, 65537)
	data, err := io.ReadAll(limited)
	if err != nil || len(data) > 65536 {
		return nil, errors.New("invalid Supabase JWKS response size")
	}
	var document struct {
		Keys []jwkRecord `json:"keys"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode Supabase JWKS: %w", err)
	}
	fresh := make(map[string]*ecdsa.PublicKey, len(document.Keys))
	for _, item := range document.Keys {
		if item.KeyType != "EC" || item.Curve != "P-256" || item.Algorithm != "ES256" ||
			item.KeyID == "" || (item.Use != "" && item.Use != "sig") {
			continue
		}
		x, xErr := base64.RawURLEncoding.DecodeString(item.X)
		y, yErr := base64.RawURLEncoding.DecodeString(item.Y)
		if xErr != nil || yErr != nil || len(x) != 32 || len(y) != 32 {
			continue
		}
		key := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}
		if key.Curve.IsOnCurve(key.X, key.Y) {
			fresh[item.KeyID] = key
		}
	}
	if len(fresh) == 0 {
		return nil, errors.New("Supabase JWKS has no acceptable ES256 signing key")
	}
	v.keys = fresh
	v.loadedAt = now
	if key, ok := fresh[kid]; ok {
		return key, nil
	}
	return nil, errors.New("Supabase signing key was not found")
}
