package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type fakeIdentityRepo struct {
	received uuid.UUID
	email    string
	err      error
}

func (r *fakeIdentityRepo) EnsureIdentity(_ context.Context, id uuid.UUID, email string) (*model.User, error) {
	r.received, r.email = id, email
	if r.err != nil {
		return nil, r.err
	}
	return &model.User{ID: id, Email: email, Role: model.UserRoleMember}, nil
}

func TestSupabaseAuthMiddlewareValidatesAndProvisionsOnlySignedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	point, err := key.PublicKey.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "test-key", "kty": "EC", "alg": "ES256", "crv": "P-256",
			"x": base64.RawURLEncoding.EncodeToString(point[1:33]),
			"y": base64.RawURLEncoding.EncodeToString(point[33:65]),
		}}})
	}))
	defer server.Close()
	verifier, err := auth.NewSupabaseVerifier(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeIdentityRepo{}
	router := gin.New()
	router.GET("/test", AuthMiddleware(verifier, repo), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id"), "email": c.GetString("email")})
	})
	call := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	if w := call(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer HTTP %d", w.Code)
	}
	if repo.received != uuid.Nil {
		t.Fatal("missing token provisioned an identity")
	}
	id := uuid.New()
	now := time.Now()
	claims := auth.SupabaseClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: id.String(), Issuer: server.URL + "/auth/v1", Audience: jwt.ClaimStrings{"authenticated"},
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
		Email: "USER@EXAMPLE.TEST", Role: "authenticated",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "test-key"
	value, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	w := call(value)
	if w.Code != http.StatusOK {
		t.Fatalf("valid bearer HTTP %d: %s", w.Code, w.Body.String())
	}
	if repo.received != id || repo.email != "user@example.test" {
		t.Fatalf("signed identity mismatch id=%s email=%q", repo.received, repo.email)
	}
	repo.err = errors.New("erasure tombstone")
	if w := call(value); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed account HTTP %d", w.Code)
	}
}
