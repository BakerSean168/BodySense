package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BodyStateHandler exposes explicit user-owned mutations for the right-side
// workbench. Consultation/tool producers use BodyStateService internally rather
// than calling these HTTP endpoints.
type BodyStateHandler struct {
	service *service.BodyStateService
}

func NewBodyStateHandler(bodyStateService *service.BodyStateService) *BodyStateHandler {
	return &BodyStateHandler{service: bodyStateService}
}

// GetCurrent handles GET /api/v1/body-state.
func (h *BodyStateHandler) GetCurrent(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	snapshot, err := h.service.GetSnapshot(c.Request.Context(), uid, 50)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load body state")
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

// UpsertFact handles POST /api/v1/body-state/facts.
func (h *BodyStateHandler) UpsertFact(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var req dto.UpsertBodyStateFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	fact, revision, err := h.service.UpsertFact(c.Request.Context(), uid, req.ExpectedRevision, bodyStateFactFromInput(req.Fact))
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"fact": fact, "revision": revision})
}

// CorrectFact handles POST /api/v1/body-state/facts/:id/correct.
// This endpoint deliberately means "the previous record was wrong"; callers
// must use the temporal endpoint when the old state used to be true.
func (h *BodyStateHandler) CorrectFact(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	factID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid fact id")
		return
	}
	var req dto.CorrectBodyStateFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	fact, revision, err := h.service.CorrectFact(c.Request.Context(), uid, req.ExpectedRevision, factID, bodyStateFactFromInput(req.Replacement))
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"fact": fact, "revision": revision})
}

// ReviewFact handles PATCH /api/v1/body-state/facts/:id/review.
func (h *BodyStateHandler) ReviewFact(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	factID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid fact id")
		return
	}
	var req dto.ReviewBodyStateFactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	fact, revision, err := h.service.ReviewFact(
		c.Request.Context(), uid, req.ExpectedRevision, factID, req.ReviewState,
	)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"fact": fact, "revision": revision})
}

// UpdateFactTemporal handles PATCH /api/v1/body-state/facts/:id/temporal.
func (h *BodyStateHandler) UpdateFactTemporal(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	factID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid fact id")
		return
	}
	var req dto.UpdateBodyStateFactTemporalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	fact, revision, err := h.service.UpdateFactTemporal(
		c.Request.Context(), uid, req.ExpectedRevision, factID, req.LifecycleState, req.Trend, req.ValidUntil,
	)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"fact": fact, "revision": revision})
}

// AddObservation handles POST /api/v1/body-state/observations.
func (h *BodyStateHandler) AddObservation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var req dto.BodyStateObservationInput
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	observation := model.BodyStateObservation{
		ConcernKey: req.ConcernKey, Kind: req.Kind, BodyRegion: req.BodyRegion, BodyRegionID: req.BodyRegionID, Method: req.Method,
		Value: bodyStateRawJSON(req.Value, `{}`), Condition: bodyStateRawJSON(req.Condition, `{}`),
		SourceKey: req.SourceKey, Provenance: bodyStateRawJSON(req.Provenance, `{}`),
		ObservedAt: req.ObservedAt, LifecycleState: req.LifecycleState,
	}
	stored, revision, err := h.service.AddObservation(c.Request.Context(), uid, req.ExpectedRevision, observation)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"observation": stored, "revision": revision})
}

// ReviewObservation handles PATCH /api/v1/body-state/observations/:id/review.
func (h *BodyStateHandler) ReviewObservation(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	observationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid observation id")
		return
	}
	var req dto.ReviewBodyStateObservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	observation, revision, err := h.service.ReviewObservation(
		c.Request.Context(), uid, req.ExpectedRevision, observationID, req.ReviewState,
	)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"observation": observation, "revision": revision})
}

func bodyStateFactFromInput(input dto.BodyStateFactInput) model.BodyStateFact {
	return model.BodyStateFact{
		ConcernKey: input.ConcernKey, Kind: input.Kind, BodyRegion: input.BodyRegion, BodyRegionID: input.BodyRegionID, Value: input.Value,
		Details: bodyStateRawJSON(input.Details, `{}`), Origin: input.Origin, ReviewState: input.ReviewState,
		LifecycleState: input.LifecycleState, Trend: input.Trend, SourceKey: input.SourceKey,
		Provenance: bodyStateRawJSON(input.Provenance, `{}`), ObservedAt: input.ObservedAt,
		ValidFrom: input.ValidFrom, ValidUntil: input.ValidUntil,
	}
}

func bodyStateRawJSON(raw json.RawMessage, fallback string) datatypes.JSON {
	if len(raw) == 0 {
		return datatypes.JSON(fallback)
	}
	return datatypes.JSON(raw)
}

func bodyStateHandleMutationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, repository.ErrBodyStateRevisionConflict):
		respondError(c, http.StatusConflict, "BODY_STATE_REVISION_CONFLICT", err.Error())
	case errors.Is(err, service.ErrUnknownBodyRegionID):
		respondError(c, http.StatusBadRequest, "INVALID_BODY_REGION_ID", err.Error())
	case errors.Is(err, service.ErrBodyRegionIDValidationUnavailable):
		respondError(c, http.StatusServiceUnavailable, "BODY_REGION_VALIDATION_UNAVAILABLE", err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "body state item not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update body state")
	}
	return true
}

// AddHypothesis handles POST /api/v1/body-state/hypotheses.
func (h *BodyStateHandler) AddHypothesis(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var req dto.BodyStateHypothesisInput
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	item, revision, err := h.service.AddHypothesis(c.Request.Context(), uid, req.ExpectedRevision, model.BodyStateHypothesis{
		ConcernKey: req.ConcernKey, Statement: req.Statement, LifecycleState: req.LifecycleState,
		Confidence:               req.Confidence,
		SupportingFactIDs:        datatypes.JSON(bodyStateRawJSON(req.SupportingFactIDs, `[]`)),
		SupportingObservationIDs: datatypes.JSON(bodyStateRawJSON(req.SupportingObservationIDs, `[]`)),
		SupportingEvidenceIDs:    datatypes.JSON(bodyStateRawJSON(req.SupportingEvidenceIDs, `[]`)),
		CounterevidenceIDs:       datatypes.JSON(bodyStateRawJSON(req.CounterevidenceIDs, `[]`)),
		Provenance:               datatypes.JSON(bodyStateRawJSON(req.Provenance, `{}`)),
	})
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"hypothesis": item, "revision": revision})
}

// UpdateHypothesisLifecycle handles PATCH /api/v1/body-state/hypotheses/:id/lifecycle.
func (h *BodyStateHandler) UpdateHypothesisLifecycle(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid hypothesis id")
		return
	}
	var req dto.UpdateBodyStateHypothesisLifecycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	item, revision, err := h.service.UpdateHypothesisLifecycle(
		c.Request.Context(), uid, req.ExpectedRevision, itemID, req.LifecycleState, req.CounterevidenceIDs,
	)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"hypothesis": item, "revision": revision})
}

func (h *BodyStateHandler) ListEvidence(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	items, err := h.service.ListEvidence(c.Request.Context(), uid, 100)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load evidence")
		return
	}
	c.JSON(http.StatusOK, gin.H{"evidence": items})
}

func (h *BodyStateHandler) ResolveSafety(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var req dto.ResolveBodyStateSafetyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	revision, err := h.service.ResolveSafetyState(c.Request.Context(), uid, req.Resolution, req.Note)
	if bodyStateHandleMutationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"revision": revision})
}
