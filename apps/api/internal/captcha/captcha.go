// Package captcha owns the ALTCHA human-verification gate that replaced
// Cloudflare Turnstile. The API issues proof-of-work challenges, and the
// Supabase Before User Created hook has this package verify the solved
// proof locally before any account exists. Verification is fail-closed:
// a missing, forged, expired or replayed proof — or any missing
// configuration or store failure — rejects the signup.
package captcha

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	altcha "github.com/altcha-org/altcha-lib-go/v2"
	"github.com/bodysense/api/internal/config"
	"github.com/redis/go-redis/v9"
	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"
)

const (
	// challengeAlgorithm is the only KDF this deployment issues and
	// accepts; a payload naming anything else was not issued here.
	challengeAlgorithm = "PBKDF2/SHA-256"
	// challengeCost and challengeKeyLength follow the ALTCHA library's
	// documented PBKDF2/SHA-256 example values.
	challengeCost      = 5000
	challengeKeyLength = 32
	// challengeTTL bounds how long an issued challenge stays solvable;
	// it also caps how long a burned salt must be remembered.
	challengeTTL       = 5 * time.Minute
	replayKeyPrefix    = "captcha:altcha:used:"
	deliveryKeyPrefix  = "captcha:hook:delivery:"
	deliveryTTL        = 10 * time.Minute
	rateLimitKeyPrefix = "captcha:altcha:challenge:ip:"
	defaultRateLimit   = 60
	defaultRateWindow  = 10 * time.Minute
)

var (
	ErrNotConfigured    = errors.New("captcha is not configured")
	ErrProofMissing     = errors.New("captcha proof is missing")
	ErrProofInvalid     = errors.New("captcha proof is invalid")
	ErrProofExpired     = errors.New("captcha proof has expired")
	ErrProofReplayed    = errors.New("captcha proof has already been used")
	ErrStoreFailure     = errors.New("captcha proof store failed")
	ErrHookBadSignature = errors.New("auth hook signature is invalid")
)

// Service issues ALTCHA challenges and verifies submitted proofs. The
// Redis client backs both the per-IP challenge brake and the proof
// replay guard; it is the same client the rest of the API uses.
type Service struct {
	hmacKey   string
	verifier  *standardwebhooks.Webhook
	redis     *redis.Client
	rateLimit int
	rateWin   time.Duration
}

// NewService resolves the hook verifier eagerly so a malformed secret is
// visible from the first hook call instead of failing open; while either
// secret is unusable the handlers fail closed.
func NewService(cfg config.Captcha, redisClient *redis.Client) *Service {
	s := &Service{
		hmacKey:   cfg.HMACKey,
		redis:     redisClient,
		rateLimit: defaultRateLimit,
		rateWin:   defaultRateWindow,
	}
	if secret := normalizeHookSecret(cfg.HookSecret); secret != "" {
		if verifier, err := standardwebhooks.NewWebhook(secret); err == nil {
			s.verifier = verifier
		}
	}
	return s
}

// normalizeHookSecret accepts the secret as the Supabase dashboard
// displays it (v1,whsec_<base64>), as the Standard Webhooks libraries
// take it (whsec_<base64>) or as the bare base64 key, and returns the
// whsec_ form the Go library expects.
func normalizeHookSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	secret = strings.TrimPrefix(secret, "v1,")
	if secret == "" || strings.HasPrefix(secret, "whsec_") {
		return secret
	}
	return "whsec_" + secret
}

// Configured reports whether challenge issuance can run at all. There is
// deliberately no fallback key: an unconfigured deployment must say so
// (the challenge endpoint answers 503) instead of signing challenges
// with a guessable secret.
func (s *Service) Configured() bool { return s.hmacKey != "" }

// HookConfigured reports whether hook calls can be authenticated and
// proofs verified; the hook handler fails closed unless this holds.
func (s *Service) HookConfigured() bool { return s.hmacKey != "" && s.verifier != nil }

// CreateChallenge issues a fresh signed challenge for the widget.
func (s *Service) CreateChallenge() (altcha.Challenge, error) {
	if !s.Configured() {
		return altcha.Challenge{}, ErrNotConfigured
	}
	expiresAt := time.Now().Add(challengeTTL)
	return altcha.CreateChallenge(altcha.CreateChallengeOptions{
		Algorithm:           challengeAlgorithm,
		DeriveKey:           altcha.DeriveKeyPBKDF2(),
		HMACSignatureSecret: s.hmacKey,
		Cost:                challengeCost,
		KeyLength:           challengeKeyLength,
		ExpiresAt:           &expiresAt,
	})
}

// AllowChallenge is the per-IP brake on challenge issuance: a fixed
// window counter in Redis. The proof-of-work cost is the real defence;
// this only keeps one source from minting challenges without bound. A
// Redis failure is reported but does not block issuance — the hook-side
// replay guard is the fail-closed part of the design.
func (s *Service) AllowChallenge(ctx context.Context, ip string) (bool, error) {
	if s.redis == nil || ip == "" {
		return true, nil
	}
	key := rateLimitKeyPrefix + ip
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return true, err
	}
	if count == 1 {
		_ = s.redis.Expire(ctx, key, s.rateWin).Err()
	}
	return count <= int64(s.rateLimit), nil
}

// VerifyHookSignature authenticates one Supabase hook delivery against
// the Standard Webhooks headers it arrived with.
func (s *Service) VerifyHookSignature(body []byte, headers http.Header) error {
	if s.verifier == nil {
		return ErrNotConfigured
	}
	if err := s.verifier.Verify(body, headers); err != nil {
		return fmt.Errorf("%w: %v", ErrHookBadSignature, err)
	}
	return nil
}

// DeliveryAllowed reports whether this hook delivery was already
// verified and allowed once. Supabase retries a delivery whose response
// it never saw; without this record the retry would burn against the
// replay guard and reject a legitimate signup.
func (s *Service) DeliveryAllowed(ctx context.Context, deliveryID string) bool {
	if s.redis == nil || deliveryID == "" {
		return false
	}
	seen, err := s.redis.Exists(ctx, deliveryKeyPrefix+deliveryID).Result()
	return err == nil && seen > 0
}

// MarkDeliveryAllowed records an allowed delivery for the retry window.
func (s *Service) MarkDeliveryAllowed(ctx context.Context, deliveryID string) {
	if s.redis == nil || deliveryID == "" {
		return
	}
	_ = s.redis.Set(ctx, deliveryKeyPrefix+deliveryID, "allowed", deliveryTTL).Err()
}

// VerifyProof checks one base64 ALTCHA payload end to end: structure,
// expiry, replay (the challenge salt is burned before the expensive
// re-derivation, so a captured proof can never be spent twice) and the
// library's own signature + solution verification.
func (s *Service) VerifyProof(ctx context.Context, encoded string) error {
	if !s.Configured() {
		return ErrNotConfigured
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return ErrProofMissing
	}
	raw, err := decodePayload(encoded)
	if err != nil {
		return ErrProofInvalid
	}
	var payload altcha.Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ErrProofInvalid
	}
	params := payload.Challenge.Parameters
	if params.Algorithm != challengeAlgorithm {
		return ErrProofInvalid
	}
	now := time.Now()
	if params.ExpiresAt != 0 && !time.Unix(params.ExpiresAt, 0).After(now) {
		return ErrProofExpired
	}
	if err := s.burnSalt(ctx, params.Salt, params.ExpiresAt, now); err != nil {
		return err
	}
	result, err := altcha.VerifySolution(altcha.VerifySolutionOptions{
		Challenge:           payload.Challenge,
		Solution:            payload.Solution,
		DeriveKey:           altcha.DeriveKeyPBKDF2(),
		HMACSignatureSecret: s.hmacKey,
	})
	if err != nil {
		return ErrProofInvalid
	}
	if result.Expired {
		return ErrProofExpired
	}
	if !result.Verified {
		return ErrProofInvalid
	}
	return nil
}

// burnSalt atomically claims the challenge salt until the challenge
// expires. The replay guard is part of the gate, so a missing store is
// a failure, not a skip.
func (s *Service) burnSalt(ctx context.Context, salt string, expiresAt int64, now time.Time) error {
	if s.redis == nil {
		return ErrStoreFailure
	}
	ttl := challengeTTL
	if expiresAt != 0 {
		ttl = time.Unix(expiresAt, 0).Sub(now)
		if ttl <= 0 {
			return ErrProofExpired
		}
	}
	ok, err := s.redis.SetNX(ctx, replayKeyPrefix+salt, "1", ttl).Result()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStoreFailure, err)
	}
	if !ok {
		return ErrProofReplayed
	}
	return nil
}

// decodePayload accepts the base64 alphabets a widget payload may
// arrive in; the payload itself is JSON either way.
func decodePayload(encoded string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if raw, err := enc.DecodeString(encoded); err == nil {
			return raw, nil
		}
	}
	return nil, errors.New("payload is not base64")
}
