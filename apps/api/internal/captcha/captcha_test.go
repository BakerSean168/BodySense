package captcha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	altcha "github.com/altcha-org/altcha-lib-go/v2"
	"github.com/bodysense/api/internal/config"
	"github.com/redis/go-redis/v9"
)

const testHMACKey = "test-altcha-hmac-key"

// testHookSecretValue is one fixed 32-byte key in the exact display form
// the Supabase dashboard issues, so secret normalization is exercised by
// every test that builds a service or signs a delivery.
var testHookSecretValue = func() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return "v1,whsec_" + base64.StdEncoding.EncodeToString(key)
}()

func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(config.Captcha{
		HMACKey:    testHMACKey,
		HookSecret: testHookSecretValue,
	}, newTestRedis(t))
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func encodePayload(t *testing.T, payload altcha.Payload) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// solvedPayload issues a real challenge through the service and solves
// it with the library, exactly as the browser widget does.
func solvedPayload(t *testing.T, svc *Service) string {
	t.Helper()
	challenge, err := svc.CreateChallenge()
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	return solveChallenge(t, challenge)
}

// solvedPayloadWith issues a low-cost challenge directly through the
// library so negative-path tests stay fast; VerifyProof re-derives from
// the payload's own parameters, so the verdict path is identical.
func solvedPayloadWith(t *testing.T, mutate func(*altcha.CreateChallengeOptions)) string {
	t.Helper()
	opts := altcha.CreateChallengeOptions{
		Algorithm:           challengeAlgorithm,
		DeriveKey:           altcha.DeriveKeyPBKDF2(),
		HMACSignatureSecret: testHMACKey,
		Cost:                10,
		KeyLength:           32,
	}
	if mutate != nil {
		mutate(&opts)
	}
	challenge, err := altcha.CreateChallenge(opts)
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	return solveChallenge(t, challenge)
}

func solveChallenge(t *testing.T, challenge altcha.Challenge) string {
	t.Helper()
	solution, err := altcha.SolveChallenge(altcha.SolveChallengeOptions{
		Challenge: challenge,
		DeriveKey: altcha.DeriveKeyPBKDF2(),
	})
	if err != nil {
		t.Fatalf("solve challenge: %v", err)
	}
	if solution == nil {
		t.Fatal("solve challenge: no solution found")
	}
	return encodePayload(t, altcha.Payload{Challenge: challenge, Solution: *solution})
}

func TestVerifyProofAcceptsSolvedChallenge(t *testing.T) {
	svc := newTestService(t)
	proof := solvedPayload(t, svc)
	if err := svc.VerifyProof(context.Background(), proof); err != nil {
		t.Fatalf("verify solved proof: %v", err)
	}
}

func TestVerifyProofRejectsMissing(t *testing.T) {
	svc := newTestService(t)
	for _, proof := range []string{"", "   "} {
		if err := svc.VerifyProof(context.Background(), proof); !errors.Is(err, ErrProofMissing) {
			t.Fatalf("proof %q: got %v, want ErrProofMissing", proof, err)
		}
	}
}

func TestVerifyProofRejectsForged(t *testing.T) {
	svc := newTestService(t)

	// A payload whose challenge signature was tampered with after issue.
	proof := solvedPayloadWith(t, nil)
	raw, err := base64.StdEncoding.DecodeString(proof)
	if err != nil {
		t.Fatalf("decode proof: %v", err)
	}
	var payload altcha.Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal proof: %v", err)
	}
	sig := []byte(payload.Challenge.Signature)
	if sig[len(sig)-1] == '0' {
		sig[len(sig)-1] = '1'
	} else {
		sig[len(sig)-1] = '0'
	}
	payload.Challenge.Signature = string(sig)
	if err := svc.VerifyProof(context.Background(), encodePayload(t, payload)); !errors.Is(err, ErrProofInvalid) {
		t.Fatalf("tampered signature: got %v, want ErrProofInvalid", err)
	}

	// A genuine proof issued under a different HMAC key.
	foreign := solvedPayloadWith(t, func(opts *altcha.CreateChallengeOptions) {
		opts.HMACSignatureSecret = "another-key"
	})
	if err := svc.VerifyProof(context.Background(), foreign); !errors.Is(err, ErrProofInvalid) {
		t.Fatalf("foreign-key proof: got %v, want ErrProofInvalid", err)
	}

	// Not a payload at all.
	if err := svc.VerifyProof(context.Background(), "!!!not-base64!!!"); !errors.Is(err, ErrProofInvalid) {
		t.Fatalf("garbage proof: got %v, want ErrProofInvalid", err)
	}
	empty := base64.StdEncoding.EncodeToString([]byte("{}"))
	if err := svc.VerifyProof(context.Background(), empty); !errors.Is(err, ErrProofInvalid) {
		t.Fatalf("empty payload: got %v, want ErrProofInvalid", err)
	}
}

func TestVerifyProofRejectsExpired(t *testing.T) {
	svc := newTestService(t)
	past := time.Now().Add(-time.Minute)
	proof := solvedPayloadWith(t, func(opts *altcha.CreateChallengeOptions) {
		opts.ExpiresAt = &past
	})
	if err := svc.VerifyProof(context.Background(), proof); !errors.Is(err, ErrProofExpired) {
		t.Fatalf("expired proof: got %v, want ErrProofExpired", err)
	}
}

func TestVerifyProofRejectsReplay(t *testing.T) {
	svc := newTestService(t)
	proof := solvedPayloadWith(t, nil)
	if err := svc.VerifyProof(context.Background(), proof); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if err := svc.VerifyProof(context.Background(), proof); !errors.Is(err, ErrProofReplayed) {
		t.Fatalf("replayed proof: got %v, want ErrProofReplayed", err)
	}
}

func TestUnconfiguredServiceFailsClosed(t *testing.T) {
	svc := NewService(config.Captcha{}, nil)
	if _, err := svc.CreateChallenge(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("create challenge: got %v, want ErrNotConfigured", err)
	}
	if err := svc.VerifyProof(context.Background(), "anything"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("verify proof: got %v, want ErrNotConfigured", err)
	}
	if svc.Configured() || svc.HookConfigured() {
		t.Fatal("unconfigured service must report not configured")
	}
}

func TestNormalizeHookSecret(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("read random: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(key)
	for _, in := range []string{"v1,whsec_" + b64, "whsec_" + b64, b64, "  v1,whsec_" + b64 + " "} {
		if got := normalizeHookSecret(in); got != "whsec_"+b64 {
			t.Fatalf("normalize %q: got %q", in, got)
		}
	}
	if got := normalizeHookSecret(""); got != "" {
		t.Fatalf("normalize empty: got %q", got)
	}
}
