package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type privacyErasureService interface {
	Plan(ctx context.Context, userID uuid.UUID) (*service.PrivacyErasurePlan, error)
	Request(ctx context.Context, userID uuid.UUID, confirmation string) (*model.PrivacyErasureRequest, error)
}

type PrivacyHandler struct {
	service privacyErasureService
	auth    *AuthHandler
}

func NewPrivacyHandler(service privacyErasureService, authHandler *AuthHandler) *PrivacyHandler {
	return &PrivacyHandler{service: service, auth: authHandler}
}

type privacyErasureRequest struct {
	Confirmation string `json:"confirmation" binding:"required"`
}

func (h *PrivacyHandler) PlanErasure(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	plan, err := h.service.Plan(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PRIVACY_PLAN_FAILED", "unable to prepare data deletion plan")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, plan)
}

func (h *PrivacyHandler) RequestErasure(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req privacyErasureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "confirmation phrase is required")
		return
	}
	request, err := h.service.Request(c.Request.Context(), userID, req.Confirmation)
	if err != nil {
		if errors.Is(err, service.ErrPrivacyErasureConfirmation) {
			respondError(c, http.StatusBadRequest, "CONFIRMATION_MISMATCH", "confirmation phrase does not match")
			return
		}
		respondError(c, http.StatusInternalServerError, "PRIVACY_ERASURE_FAILED", "unable to persist data deletion request")
		return
	}

	// The request is irreversible once persisted. Forget the browser credential
	// immediately; durable recovery continues server-side if a later stage retries.
	h.auth.ClearRefreshCookie(c)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusAccepted, gin.H{
		"request_id": request.ID.String(),
		"status":     request.Status,
		"message":    "data deletion has been accepted and will continue even if this browser disconnects",
	})
}

func authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return uuid.Nil, false
	}
	value, ok := raw.(string)
	if !ok {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "invalid authenticated user")
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(value)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "invalid authenticated user")
		return uuid.Nil, false
	}
	return userID, true
}
