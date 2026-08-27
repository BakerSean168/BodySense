package handler

import (
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

type OnboardingContextHandler struct {
	service *service.OnboardingContextService
}

func NewOnboardingContextHandler(onboardingService *service.OnboardingContextService) *OnboardingContextHandler {
	return &OnboardingContextHandler{service: onboardingService}
}

func (h *OnboardingContextHandler) Submit(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var request dto.OnboardingContextRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.service.Submit(c.Request.Context(), uid, request)
	if errors.Is(err, service.ErrInvalidOnboardingContext) {
		respondError(c, http.StatusBadRequest, "INVALID_ONBOARDING_CONTEXT", err.Error())
		return
	}
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, result)
}
