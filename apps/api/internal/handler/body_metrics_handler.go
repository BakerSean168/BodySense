package handler

import (
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

type BodyMetricsHandler struct {
	service *service.BodyMetricsService
}

func NewBodyMetricsHandler(bodyMetricsService *service.BodyMetricsService) *BodyMetricsHandler {
	return &BodyMetricsHandler{service: bodyMetricsService}
}

func (h *BodyMetricsHandler) Get(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load body metrics")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *BodyMetricsHandler) Update(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var request dto.UpdateBodyMetricsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.service.Update(c.Request.Context(), uid, request)
	if errors.Is(err, service.ErrInvalidBodyMetric) {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "height or weight is outside the supported range")
		return
	}
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, result)
}
