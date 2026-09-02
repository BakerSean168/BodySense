package handler

import (
	"errors"
	"net/http"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HealthDocumentReviewHandler exposes the authenticated review APIs needed by
// the Web review UI: listing the review projection for one extraction run,
// appending a confirm/correct/reject action, and streaming the private source
// upload bytes for a candidate's source context. It never serializes
// storage_backend or storage_key.
type HealthDocumentReviewHandler struct {
	reviewService *service.HealthDocumentReviewService
	uploadService *service.UploadService
}

// NewHealthDocumentReviewHandler creates a new review handler.
func NewHealthDocumentReviewHandler(
	reviewService *service.HealthDocumentReviewService,
	uploadService *service.UploadService,
) *HealthDocumentReviewHandler {
	return &HealthDocumentReviewHandler{reviewService: reviewService, uploadService: uploadService}
}

func (h *HealthDocumentReviewHandler) authenticatedUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, false
	}
	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return uuid.Nil, false
	}
	return uid, true
}

// ListCandidates handles GET /api/v1/uploads/:id/extractions/:runId/reviews
func (h *HealthDocumentReviewHandler) ListCandidates(c *gin.Context) {
	userID, ok := h.authenticatedUserID(c)
	if !ok {
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid extraction run id"})
		return
	}
	projection, err := h.reviewService.ListCandidates(c.Request.Context(), userID, runID)
	if err != nil {
		if errors.Is(err, service.ErrReviewAccessDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "extraction run not accessible"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load review candidates"})
		return
	}
	if projection == nil {
		projection = []service.DocumentIndicatorReviewProjection{}
	}
	c.JSON(http.StatusOK, gin.H{"review_candidates": projection})
}

// AppendReview handles POST /api/v1/uploads/:id/extractions/:runId/reviews
func (h *HealthDocumentReviewHandler) AppendReview(c *gin.Context) {
	userID, ok := h.authenticatedUserID(c)
	if !ok {
		return
	}
	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload id"})
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid extraction run id"})
		return
	}
	var req service.ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid review request body"})
		return
	}
	// The URL owns the identity; never trust client-supplied run/upload ids.
	req.ExtractionRunID = runID
	if err := h.reviewService.EnsureUploadOwnsRun(c.Request.Context(), userID, uploadID, runID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "review target not accessible"})
		return
	}
	record, err := h.reviewService.ApplyReview(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrReviewAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "review target not accessible"})
		case errors.Is(err, service.ErrReviewCandidateMismatch):
			c.JSON(http.StatusConflict, gin.H{"error": "review candidate is stale; reload the extraction run"})
		case errors.Is(err, service.ErrReviewDuplicateConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "idempotency key reused with different review content"})
		case errors.Is(err, service.ErrReviewValidation):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to append review"})
		}
		return
	}
	c.JSON(http.StatusCreated, record)
}

// SourceContext handles GET /api/v1/uploads/:id/extractions/:runId/source
// It streams the authenticated private upload through the upload storage
// boundary so the reviewer can open the original source document. The response
// carries only the upload content; storage_backend/storage_key never leave the
// server, and upload deletion/privacy semantics are inherited from GetUpload.
func (h *HealthDocumentReviewHandler) SourceContext(c *gin.Context) {
	userID, ok := h.authenticatedUserID(c)
	if !ok {
		return
	}
	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload id"})
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid extraction run id"})
		return
	}
	if err := h.reviewService.EnsureUploadOwnsRun(c.Request.Context(), userID, uploadID, runID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "source not accessible"})
		return
	}
	upload, err := h.uploadService.GetUpload(c.Request.Context(), userID, uploadID)
	if err != nil {
		if err.Error() == "upload not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
			return
		}
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load upload"})
		return
	}
	reader, _, err := h.uploadService.OpenUploadObject(c.Request.Context(), upload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open upload source"})
		return
	}
	defer reader.Close()
	c.Header("Content-Type", upload.MimeType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, upload.FileSize, upload.MimeType, reader, nil)
}
