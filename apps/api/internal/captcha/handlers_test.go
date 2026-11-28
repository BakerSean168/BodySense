package captcha

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	altcha "github.com/altcha-org/altcha-lib-go/v2"
	"github.com/bodysense/api/internal/config"
	"github.com/gin-gonic/gin"
	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"
)

func newTestEngine(svc *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, svc)
	return r
}

func TestChallengeEndpointIssuesSignedChallenge(t *testing.T) {
	svc := newTestService(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge", nil)
	newTestEngine(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
	var challenge altcha.Challenge
	if err := json.Unmarshal(rec.Body.Bytes(), &challenge); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	if challenge.Parameters.Algorithm != challengeAlgorithm {
		t.Fatalf("algorithm = %q", challenge.Parameters.Algorithm)
	}
	if challenge.Signature == "" || challenge.Parameters.Salt == "" {
		t.Fatalf("challenge is unsigned or saltless: %+v", challenge)
	}
	if challenge.Parameters.ExpiresAt == 0 {
		t.Fatal("challenge must carry an expiry")
	}
}

func TestChallengeEndpointUnconfigured(t *testing.T) {
	svc := NewService(config.Captcha{}, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge", nil)
	newTestEngine(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestChallengeEndpointRateLimit(t *testing.T) {
	svc := newTestService(t)
	svc.rateLimit = 3
	engine := newTestEngine(svc)
	for i := 1; i <= 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge", nil)
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/captcha/challenge", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit status = %d, want 429", rec.Code)
	}
}

// hookBody builds one Before User Created delivery body; an empty proof
// means the signup carried no altcha field at all.
func hookBody(t *testing.T, proof string) []byte {
	t.Helper()
	meta := map[string]any{}
	if proof != "" {
		meta["altcha"] = proof
	}
	body, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"type": "before-user-created"},
		"user": map[string]any{
			"id":            "00000000-0000-0000-0000-000000000001",
			"email":         "new@example.com",
			"user_metadata": meta,
		},
	})
	if err != nil {
		t.Fatalf("marshal hook body: %v", err)
	}
	return body
}

// signedHookRequest signs the body exactly as Supabase would, with the
// given secret and delivery id. The secret is passed in the dashboard
// display form; like Supabase itself, the signer uses the bare key.
func signedHookRequest(t *testing.T, secret, deliveryID string, body []byte) *http.Request {
	t.Helper()
	wh, err := standardwebhooks.NewWebhook(strings.TrimPrefix(secret, "v1,"))
	if err != nil {
		t.Fatalf("new webhook signer: %v", err)
	}
	now := time.Now()
	sig, err := wh.Sign(deliveryID, now, body)
	if err != nil {
		t.Fatalf("sign delivery: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth-hooks/before-user-created", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", deliveryID)
	req.Header.Set("webhook-timestamp", strconv.FormatInt(now.Unix(), 10))
	req.Header.Set("webhook-signature", sig)
	return req
}

type hookErrorBody struct {
	Error struct {
		HTTPCode int    `json:"http_code"`
		Message  string `json:"message"`
	} `json:"error"`
}

func postHook(t *testing.T, engine *gin.Engine, req *http.Request) (*httptest.ResponseRecorder, hookErrorBody) {
	t.Helper()
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var parsed hookErrorBody
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	}
	return rec, parsed
}

func TestHookAllowsValidProof(t *testing.T) {
	svc := newTestService(t)
	engine := newTestEngine(svc)
	body := hookBody(t, solvedPayloadWith(t, nil))
	rec, _ := postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-allow", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "{}" {
		t.Fatalf("allow body = %q, want {}", rec.Body.String())
	}
}

func TestHookRejectsMissingProof(t *testing.T) {
	svc := newTestService(t)
	engine := newTestEngine(svc)
	rec, parsed := postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-missing", hookBody(t, "")))
	if rec.Code != http.StatusBadRequest || parsed.Error.HTTPCode != http.StatusBadRequest {
		t.Fatalf("status = %d, parsed %+v", rec.Code, parsed)
	}
	if !strings.Contains(parsed.Error.Message, "缺少人机验证证明") {
		t.Fatalf("message = %q", parsed.Error.Message)
	}
}

func TestHookRejectsBadSignature(t *testing.T) {
	svc := newTestService(t)
	engine := newTestEngine(svc)
	body := hookBody(t, solvedPayloadWith(t, nil))
	other := "v1,whsec_" + base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	rec, _ := postHook(t, engine, signedHookRequest(t, other, "delivery-badsig", body))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body.String())
	}
}

func TestHookRejectsForgedProof(t *testing.T) {
	svc := newTestService(t)
	engine := newTestEngine(svc)
	forged := base64.StdEncoding.EncodeToString([]byte(
		`{"challenge":{"parameters":{"algorithm":"PBKDF2/SHA-256","salt":"00"}},"solution":{"counter":1,"derivedKey":"00"}}`,
	))
	rec, parsed := postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-forged", hookBody(t, forged)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(parsed.Error.Message, "人机验证未通过") {
		t.Fatalf("message = %q", parsed.Error.Message)
	}
}

func TestHookRejectsExpiredProof(t *testing.T) {
	svc := newTestService(t)
	engine := newTestEngine(svc)
	past := time.Now().Add(-time.Minute)
	proof := solvedPayloadWith(t, func(opts *altcha.CreateChallengeOptions) {
		opts.ExpiresAt = &past
	})
	rec, parsed := postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-expired", hookBody(t, proof)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(parsed.Error.Message, "已过期") {
		t.Fatalf("message = %q", parsed.Error.Message)
	}
}

func TestHookRejectsReplayButHonoursRedelivery(t *testing.T) {
	svc := newTestService(t)
	engine := newTestEngine(svc)
	proof := solvedPayloadWith(t, nil)
	body := hookBody(t, proof)

	// First delivery: allowed.
	rec, _ := postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-1", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("first delivery status = %d, body %s", rec.Code, rec.Body.String())
	}

	// The same delivery arriving again (Supabase never saw our 200):
	// still allowed, because the decision belongs to the delivery.
	rec, _ = postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-1", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("redelivery status = %d, body %s", rec.Code, rec.Body.String())
	}

	// A new delivery carrying the same proof is a replay: rejected.
	rec, parsed := postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-2", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(parsed.Error.Message, "已被使用") {
		t.Fatalf("message = %q", parsed.Error.Message)
	}
}

func TestHookUnconfiguredFailsClosed(t *testing.T) {
	svc := NewService(config.Captcha{}, nil)
	engine := newTestEngine(svc)
	rec, _ := postHook(t, engine, signedHookRequest(t, testHookSecretValue, "delivery-x", hookBody(t, "")))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (fail closed)", rec.Code)
	}
}
