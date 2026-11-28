package captcha

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// maxHookBodyBytes bounds one hook delivery; Supabase caps hook payloads
// at 20KB, so this leaves generous headroom without inviting abuse.
const maxHookBodyBytes = 64 * 1024

// RegisterRoutes mounts the two public captcha endpoints. Both are
// public by design: the challenge endpoint is how an anonymous browser
// starts a proof, and the hook endpoint authenticates its caller with
// the Standard Webhooks signature instead of a session.
func RegisterRoutes(r *gin.Engine, svc *Service) {
	r.GET("/api/v1/captcha/challenge", svc.handleChallenge)
	r.POST("/api/v1/auth-hooks/before-user-created", svc.handleBeforeUserCreated)
}

func (s *Service) handleChallenge(c *gin.Context) {
	if !s.Configured() {
		slog.Default().Warn("captcha_challenge_requested_while_unconfigured")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "captcha_not_configured"})
		return
	}
	allowed, err := s.AllowChallenge(c.Request.Context(), c.ClientIP())
	if err != nil {
		slog.Default().Warn("captcha_challenge_rate_limit_unavailable", "error", err)
	} else if !allowed {
		c.Header("Retry-After", "60")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too_many_challenge_requests"})
		return
	}
	challenge, err := s.CreateChallenge()
	if err != nil {
		slog.Default().Error("captcha_challenge_issue_failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "challenge_issue_failed"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, challenge)
}

// hookRequest is the slice of the Supabase Before User Created payload
// this gate reads: the ALTCHA proof travels in user_metadata, where
// signUp's options.data lands it.
type hookRequest struct {
	User struct {
		UserMetadata map[string]json.RawMessage `json:"user_metadata"`
	} `json:"user"`
}

func (s *Service) handleBeforeUserCreated(c *gin.Context) {
	log := slog.Default()
	if !s.HookConfigured() {
		// Fail closed: while the gate cannot run, Supabase must not
		// create accounts, so the hook errors instead of allowing.
		log.Error("auth_hook_called_while_unconfigured")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hook_not_configured"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxHookBodyBytes))
	if err != nil {
		hookReject(c, http.StatusBadRequest, "人机验证请求无效，请重试")
		return
	}
	if err := s.VerifyHookSignature(body, c.Request.Header); err != nil {
		log.Warn("auth_hook_signature_rejected", "error", err)
		hookReject(c, http.StatusUnauthorized, "人机验证请求签名无效")
		return
	}
	ctx := c.Request.Context()
	deliveryID := c.GetHeader("webhook-id")
	if s.DeliveryAllowed(ctx, deliveryID) {
		// A redelivery of an already-allowed hook call: the proof was
		// verified and burned for this delivery, so the recorded
		// decision stands instead of reading as a proof replay.
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	var req hookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		hookReject(c, http.StatusBadRequest, "人机验证请求无效，请重试")
		return
	}
	proof := ""
	if raw, ok := req.User.UserMetadata["altcha"]; ok {
		_ = json.Unmarshal(raw, &proof)
	}
	switch err := s.VerifyProof(ctx, proof); {
	case err == nil:
		s.MarkDeliveryAllowed(ctx, deliveryID)
		c.JSON(http.StatusOK, gin.H{})
	case errors.Is(err, ErrProofMissing):
		hookReject(c, http.StatusBadRequest, "缺少人机验证证明，请完成验证后再注册")
	case errors.Is(err, ErrProofExpired):
		hookReject(c, http.StatusBadRequest, "人机验证已过期，请重新验证后再注册")
	case errors.Is(err, ErrProofReplayed):
		hookReject(c, http.StatusBadRequest, "人机验证证明已被使用，请重新验证后再注册")
	case errors.Is(err, ErrStoreFailure), errors.Is(err, ErrNotConfigured):
		log.Error("auth_hook_verification_unavailable", "error", err)
		hookReject(c, http.StatusServiceUnavailable, "注册服务暂时不可用，请稍后重试")
	default:
		hookReject(c, http.StatusBadRequest, "人机验证未通过，请重新验证后再注册")
	}
}

// hookReject answers in the Supabase hook protocol: the status code plus
// an error object whose message propagates to the signup caller.
func hookReject(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{"http_code": status, "message": message},
	})
}
