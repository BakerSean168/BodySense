package handler

import (
	"net/http"
	"strconv"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AssessmentHandler handles assessment HTTP requests.
type AssessmentHandler struct {
	assessmentService       *service.AssessmentService
	assessmentReplayService *service.AssessmentReplayService
}

// NewAssessmentHandler creates a new AssessmentHandler.
func NewAssessmentHandler(assessmentService *service.AssessmentService) *AssessmentHandler {
	return &AssessmentHandler{assessmentService: assessmentService}
}

// WithAssessmentReplay attaches the replay service (read-only historical /
// counterfactual replay + regression export), mirroring Diagnosis/Treatment.
func (h *AssessmentHandler) WithAssessmentReplay(s *service.AssessmentReplayService) *AssessmentHandler {
	h.assessmentReplayService = s
	return h
}

// GenerateAssessment handles POST /api/v1/assessment/generate
func (h *AssessmentHandler) GenerateAssessment(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	report, err := h.assessmentService.GenerateAssessment(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

// GetReport handles GET /api/v1/assessment/:id
func (h *AssessmentHandler) GetReport(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report id"})
		return
	}

	report, err := h.assessmentService.GetReport(c.Request.Context(), reportID, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get report"})
		return
	}
	if report == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// ListReports handles GET /api/v1/assessment
func (h *AssessmentHandler) ListReports(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Parse pagination params
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	reports, total, err := h.assessmentService.ListReports(c.Request.Context(), uid, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list reports"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// ReplayAssessment handles POST /api/v1/assessment/:id/replay. It is read-only:
// replay never creates a report and never mutates BodyState. `mode` is
// historical (no model call, recompute integrity) or counterfactual (run the
// frozen input against another immutable Assessment configuration, transient).
func (h *AssessmentHandler) ReplayAssessment(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if h.assessmentReplayService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "assessment replay is not configured"})
		return
	}
	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report id"})
		return
	}
	var req struct {
		Mode            string `json:"mode" binding:"required"`
		ConfigurationID string `json:"configuration_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var report *service.AssessmentReplayReport
	switch req.Mode {
	case "historical":
		report, err = h.assessmentReplayService.HistoricalReplay(c.Request.Context(), uid, reportID)
	case "counterfactual":
		if req.ConfigurationID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "counterfactual replay requires configuration_id"})
			return
		}
		report, err = h.assessmentReplayService.CounterfactualReplay(
			c.Request.Context(), uid, reportID, req.ConfigurationID,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "replay mode must be historical or counterfactual"})
		return
	}
	if err != nil {
		h.respondAssessmentReplayError(c, err)
		return
	}
	c.JSON(http.StatusOK, report)
}

// ExportAssessmentRegressionCase exposes a dataset-shaped case envelope without
// mutating the repository, mirroring Diagnosis/Treatment regression export.
func (h *AssessmentHandler) ExportAssessmentRegressionCase(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if h.assessmentReplayService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "assessment replay is not configured"})
		return
	}
	reportID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report id"})
		return
	}
	payload, err := h.assessmentReplayService.ExportRegressionCase(c.Request.Context(), uid, reportID)
	if err != nil {
		h.respondAssessmentReplayError(c, err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *AssessmentHandler) respondAssessmentReplayError(c *gin.Context, err error) {
	switch {
	case err == service.ErrAssessmentReplayNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case err == service.ErrAssessmentReplayUnavailable:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
