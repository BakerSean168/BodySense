package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

type HealthHistoryHandler struct {
	service *service.HealthHistoryService
}

func NewHealthHistoryHandler(healthHistoryService *service.HealthHistoryService) *HealthHistoryHandler {
	return &HealthHistoryHandler{service: healthHistoryService}
}

func (h *HealthHistoryHandler) GetInjuryHistory(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	result, err := h.service.GetInjuryHistory(c.Request.Context(), uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load injury history")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *HealthHistoryHandler) UpdateInjuryHistory(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var request dto.UpdateInjuryHistoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.service.UpdateInjuryHistory(c.Request.Context(), uid, request)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, result)
}
