package handler

import (
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LifestyleHandler struct {
	service *service.LifestyleService
}

func NewLifestyleHandler(lifestyleService *service.LifestyleService) *LifestyleHandler {
	return &LifestyleHandler{service: lifestyleService}
}

func (h *LifestyleHandler) Get(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	result, err := h.service.Get(c.Request.Context(), uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load lifestyle context")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *LifestyleHandler) Update(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var request dto.UpdateLifestyleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.service.Update(c.Request.Context(), uid, request)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *LifestyleHandler) AcceptCandidate(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	candidateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_CANDIDATE_ID", "candidate id must be a valid UUID")
		return
	}
	var request dto.ReviewLifestyleCandidateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.service.AcceptCandidate(
		c.Request.Context(), uid, request.ExpectedRevision, candidateID,
	)
	if errors.Is(err, service.ErrInvalidLifestyleCandidate) {
		respondError(c, http.StatusNotFound, "LIFESTYLE_CANDIDATE_NOT_FOUND", "lifestyle candidate is not reviewable")
		return
	}
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *LifestyleHandler) RejectCandidate(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	candidateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_CANDIDATE_ID", "candidate id must be a valid UUID")
		return
	}
	var request dto.ReviewLifestyleCandidateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.service.RejectCandidate(
		c.Request.Context(), uid, request.ExpectedRevision, candidateID,
	)
	if errors.Is(err, service.ErrInvalidLifestyleCandidate) {
		respondError(c, http.StatusNotFound, "LIFESTYLE_CANDIDATE_NOT_FOUND", "lifestyle candidate is not reviewable")
		return
	}
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, result)
}
