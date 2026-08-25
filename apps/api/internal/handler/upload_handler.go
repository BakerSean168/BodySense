package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// uploadResponse preserves the existing v1 JSON shape while storage_backend and
// storage_key remain server-private authority. file_path is now a deprecated
// compatibility projection of the opaque storage key, not a host filesystem path.
type uploadResponse struct {
	model.UserUpload
	FilePath string `json:"file_path"`
}

func newUploadResponse(upload model.UserUpload) uploadResponse {
	return uploadResponse{UserUpload: upload, FilePath: upload.StorageKey}
}

func newUploadResponses(uploads []model.UserUpload) []uploadResponse {
	responses := make([]uploadResponse, 0, len(uploads))
	for _, upload := range uploads {
		responses = append(responses, newUploadResponse(upload))
	}
	return responses
}

// UploadHandler handles upload HTTP requests.
type UploadHandler struct {
	uploadService *service.UploadService
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(uploadService *service.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: uploadService}
}

// Upload handles POST /api/v1/uploads
// Accepts multipart form with "file" and "file_type" fields.
func (h *UploadHandler) Upload(c *gin.Context) {
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

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// Get file type from form
	fileType := c.PostForm("file_type")
	if fileType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_type is required"})
		return
	}

	upload, err := h.uploadService.UploadFile(c.Request.Context(), uid, file, fileType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, newUploadResponse(*upload))
}

// GetUploads handles GET /api/v1/uploads
// Returns all uploads for the current user.
func (h *UploadHandler) GetUploads(c *gin.Context) {
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

	uploads, err := h.uploadService.GetUploads(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get uploads"})
		return
	}

	// Return empty array instead of null.
	c.JSON(http.StatusOK, newUploadResponses(uploads))
}

// GetUpload handles GET /api/v1/uploads/:id
// Returns a single upload by ID.
func (h *UploadHandler) GetUpload(c *gin.Context) {
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

	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload id"})
		return
	}

	upload, err := h.uploadService.GetUpload(c.Request.Context(), uid, uploadID)
	if err != nil {
		if err.Error() == "upload not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
			return
		}
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get upload"})
		return
	}

	c.JSON(http.StatusOK, newUploadResponse(*upload))
}

// GetPostureAnalysis handles GET /api/v1/uploads/posture-analysis
// Returns the caller's latest completed three-view posture analyses (or an
// empty summary when none exist). Used by the consultation Agent tool and
// profile surfaces so analysis_result is no longer a write-only island.
func (h *UploadHandler) GetPostureAnalysis(c *gin.Context) {
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

	summary, err := h.uploadService.GetPostureAnalysisSummary(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get posture analysis"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// DeleteUpload handles DELETE /api/v1/uploads/:id
// Deletes an upload and its private storage object.
func (h *UploadHandler) DeleteUpload(c *gin.Context) {
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

	uploadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload id"})
		return
	}

	if err := h.uploadService.DeleteUpload(c.Request.Context(), uid, uploadID); err != nil {
		if err.Error() == "upload not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "upload not found"})
			return
		}
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete upload"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "upload deleted successfully"})
}
