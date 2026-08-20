package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// TreatmentHandler exposes proposal/acceptance/history/outcome operations for the
// revisioned Treatment aggregate.
type TreatmentHandler struct {
	treatments *service.TreatmentService
	training   *service.TrainingService
	replay     *service.TreatmentReplayService
}

func NewTreatmentHandler(
	treatments *service.TreatmentService,
	training *service.TrainingService,
	replay *service.TreatmentReplayService,
) *TreatmentHandler {
	return &TreatmentHandler{treatments: treatments, training: training, replay: replay}
}

func (h *TreatmentHandler) GenerateProposal(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var req struct {
		DiagnosisAnalysisID string         `json:"diagnosis_analysis_id"`
		UserConstraints     map[string]any `json:"user_constraints"`
		ChangeReason        string         `json:"change_reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var proposal *model.TreatmentRevision
	var err error
	if req.DiagnosisAnalysisID == "" {
		proposal, err = h.treatments.GenerateProposalForLatest(c.Request.Context(), uid, req.UserConstraints, req.ChangeReason)
	} else {
		analysisID, parseErr := uuid.Parse(req.DiagnosisAnalysisID)
		if parseErr != nil {
			respondError(c, http.StatusBadRequest, "INVALID_DIAGNOSIS_ANALYSIS_ID", "invalid diagnosis analysis id")
			return
		}
		proposal, err = h.treatments.GenerateProposal(c.Request.Context(), uid, service.TreatmentProposalInput{
			DiagnosisAnalysisID: analysisID, UserConstraints: req.UserConstraints, ChangeReason: req.ChangeReason,
		})
	}
	if treatmentHandlerError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"proposal": proposal})
}

func (h *TreatmentHandler) AcceptRevision(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	revisionID, err := uuid.Parse(c.Param("revisionId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid treatment revision id")
		return
	}
	var req struct {
		ConsultationID *uuid.UUID `json:"consultation_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if h.training == nil {
		respondError(c, http.StatusServiceUnavailable, "TRAINING_DOMAIN_UNAVAILABLE", "training execution service is not configured")
		return
	}
	treatment, trainingPlan, err := h.training.AcceptTreatmentAndEnsurePlan(
		c.Request.Context(), uid, revisionID, req.ConsultationID,
	)
	if treatmentHandlerError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"treatment": treatment, "training_plan": trainingPlan})
}

func (h *TreatmentHandler) RejectRevision(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	revisionID, err := uuid.Parse(c.Param("revisionId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid treatment revision id")
		return
	}
	if treatmentHandlerError(c, h.treatments.RejectProposal(c.Request.Context(), uid, revisionID)) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TreatmentHandler) GetCurrent(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	current, err := h.treatments.PreviewCurrentReview(c.Request.Context(), uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load current treatment")
		return
	}
	c.JSON(http.StatusOK, gin.H{"treatment": current})
}

func (h *TreatmentHandler) ReviewCurrent(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	current, err := h.treatments.EvaluateCurrentReview(c.Request.Context(), uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to evaluate treatment review state")
		return
	}
	c.JSON(http.StatusOK, gin.H{"treatment": current})
}

func (h *TreatmentHandler) ListRevisions(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.treatments.ListRevisions(c.Request.Context(), uid, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load treatment history")
		return
	}
	c.JSON(http.StatusOK, gin.H{"revisions": items})
}

func (h *TreatmentHandler) GetRevision(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	revisionID, err := uuid.Parse(c.Param("revisionId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid treatment revision id")
		return
	}
	item, err := h.treatments.GetRevision(c.Request.Context(), uid, revisionID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load treatment revision")
		return
	}
	if item == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "treatment revision not found")
		return
	}
	c.JSON(http.StatusOK, item)
}

// ReplayRevision is read-only: it never creates, accepts or rejects Treatment.
func (h *TreatmentHandler) ReplayRevision(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	if h.replay == nil {
		respondError(c, http.StatusServiceUnavailable, "TREATMENT_REPLAY_UNAVAILABLE", "treatment replay is not configured")
		return
	}
	revisionID, err := uuid.Parse(c.Param("revisionId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid treatment revision id")
		return
	}
	var req struct {
		Mode            string `json:"mode" binding:"required"`
		ConfigurationID string `json:"configuration_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var report *service.TreatmentReplayReport
	switch strings.TrimSpace(req.Mode) {
	case "historical":
		report, err = h.replay.HistoricalReplay(c.Request.Context(), uid, revisionID)
	case "counterfactual":
		if strings.TrimSpace(req.ConfigurationID) == "" {
			respondError(c, http.StatusBadRequest, "CONFIGURATION_REQUIRED", "counterfactual replay requires configuration_id")
			return
		}
		report, err = h.replay.CounterfactualReplay(c.Request.Context(), uid, revisionID, req.ConfigurationID)
	default:
		respondError(c, http.StatusBadRequest, "INVALID_REPLAY_MODE", "replay mode must be historical or counterfactual")
		return
	}
	if err != nil {
		h.respondTreatmentReplayError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *TreatmentHandler) ExportRegressionCase(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	if h.replay == nil {
		respondError(c, http.StatusServiceUnavailable, "TREATMENT_REPLAY_UNAVAILABLE", "treatment replay is not configured")
		return
	}
	revisionID, err := uuid.Parse(c.Param("revisionId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "invalid treatment revision id")
		return
	}
	exported, err := h.replay.ExportRegressionCase(c.Request.Context(), uid, revisionID)
	if err != nil {
		h.respondTreatmentReplayError(c, err)
		return
	}
	c.JSON(http.StatusOK, exported)
}

func (h *TreatmentHandler) respondTreatmentReplayError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTreatmentReplayNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "treatment revision not found")
	case errors.Is(err, service.ErrTreatmentReplayUnavailable):
		respondError(c, http.StatusConflict, "TREATMENT_REPLAY_INPUT_UNAVAILABLE", "this historical Treatment revision predates frozen replay input")
	case strings.Contains(err.Error(), "unknown Treatment Agent configuration id"):
		respondError(c, http.StatusUnprocessableEntity, "UNKNOWN_AGENT_CONFIGURATION", err.Error())
	default:
		respondError(c, http.StatusBadGateway, "TREATMENT_REPLAY_FAILED", "treatment replay failed")
	}
}

func (h *TreatmentHandler) RecordOutcome(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	var req struct {
		TreatmentID          *uuid.UUID      `json:"treatment_id"`
		TreatmentRevisionID  *uuid.UUID      `json:"treatment_revision_id"`
		InterventionID       *uuid.UUID      `json:"intervention_id"`
		SourceType           string          `json:"source_type" binding:"required"`
		SourceKey            string          `json:"source_key" binding:"required"`
		Kind                 string          `json:"kind" binding:"required"`
		ConcernKey           string          `json:"concern_key"`
		BodyRegion           string          `json:"body_region"`
		Value                json.RawMessage `json:"value"`
		Notes                string          `json:"notes"`
		AssociationStatement string          `json:"association_statement"`
		CausalityLevel       string          `json:"causality_level"`
		OccurredAt           *time.Time      `json:"occurred_at"`
		Provenance           json.RawMessage `json:"provenance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil {
		occurredAt = req.OccurredAt.UTC()
	}
	value := req.Value
	if len(value) == 0 {
		value = json.RawMessage(`{}`)
	}
	provenance := req.Provenance
	if len(provenance) == 0 {
		provenance = json.RawMessage(`{}`)
	}
	outcome, created, err := h.treatments.RecordOutcome(c.Request.Context(), uid, model.Outcome{
		TreatmentID: req.TreatmentID, TreatmentRevisionID: req.TreatmentRevisionID, InterventionID: req.InterventionID,
		SourceType: req.SourceType, SourceKey: req.SourceKey, Kind: req.Kind,
		ConcernKey: req.ConcernKey, BodyRegion: req.BodyRegion,
		Value: datatypes.JSON(value), Notes: req.Notes,
		AssociationStatement: req.AssociationStatement, CausalityLevel: req.CausalityLevel,
		OccurredAt: occurredAt, Provenance: datatypes.JSON(provenance),
	})
	if treatmentHandlerError(c, err) {
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"outcome": outcome, "created": created})
}

func (h *TreatmentHandler) ListOutcomes(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := h.treatments.ListOutcomes(c.Request.Context(), uid, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load outcomes")
		return
	}
	c.JSON(http.StatusOK, gin.H{"outcomes": items})
}

func treatmentHandlerError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrTreatmentDiagnosisNotReady):
		respondError(c, http.StatusConflict, "DIAGNOSIS_NOT_READY", err.Error())
	case errors.Is(err, service.ErrTreatmentSafetyBlocked):
		respondError(c, http.StatusConflict, "TREATMENT_SAFETY_BLOCKED", err.Error())
	case errors.Is(err, service.ErrTreatmentAnalysisStale):
		respondError(c, http.StatusConflict, "DIAGNOSIS_STALE", err.Error())
	case errors.Is(err, service.ErrTreatmentCandidateAssessmentRequired):
		respondError(c, http.StatusConflict, "DIAGNOSIS_ASSESSMENT_REQUIRED", err.Error())
	case errors.Is(err, service.ErrTreatmentProposalOutdated):
		respondError(c, http.StatusConflict, "TREATMENT_PROPOSAL_OUTDATED", err.Error())
	case errors.Is(err, service.ErrTrainingProjectionFailed):
		respondError(c, http.StatusInternalServerError, "TRAINING_PROJECTION_FAILED", err.Error())
	default:
		respondError(c, http.StatusBadRequest, "TREATMENT_OPERATION_FAILED", err.Error())
	}
	return true
}
