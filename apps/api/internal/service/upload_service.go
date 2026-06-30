package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

const (
	MaxFileSize = 10 << 20 // 10MB
	UploadDir   = "uploads"
)

// Allowed file types
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/webp":      true,
	"application/pdf": true,
}

// Allowed file type values
var allowedFileTypes = map[string]bool{
	"photo_front": true,
	"photo_side":  true,
	"photo_back":  true,
	"report":      true,
}

// UploadService handles upload business logic.
type UploadService struct {
	uploadRepo   *repository.UploadRepository
	jobRuntime   *JobRuntime
	aiServiceURL string
}

// NewUploadService creates a new UploadService.
func NewUploadService(uploadRepo *repository.UploadRepository, jobRuntime *JobRuntime) *UploadService {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8100"
	}
	return &UploadService{
		uploadRepo:   uploadRepo,
		jobRuntime:   jobRuntime,
		aiServiceURL: aiServiceURL,
	}
}

// UploadFile handles file upload: validates, saves to disk, creates DB record, and triggers OCR.
func (s *UploadService) UploadFile(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader, fileType string) (*model.UserUpload, error) {
	// Validate file type
	if !allowedFileTypes[fileType] {
		return nil, errors.New("invalid file type: must be photo_front, photo_side, photo_back, or report")
	}

	// Validate file size
	if file.Size > MaxFileSize {
		return nil, errors.New("file size exceeds 10MB limit")
	}

	// Validate MIME type from header
	mimeType := file.Header.Get("Content-Type")
	if !allowedMimeTypes[mimeType] {
		return nil, errors.New("invalid file type: only JPEG, PNG, WebP, and PDF are allowed")
	}

	// Create upload directory for user
	userDir := filepath.Join(UploadDir, userID.String())
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	fileName := uuid.New().String() + ext
	filePath := filepath.Join(userDir, fileName)

	// Save file to disk and verify content type
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Read first 512 bytes to detect actual content type
	buf := make([]byte, 512)
	n, err := src.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	detectedType := http.DetectContentType(buf[:n])
	// Normalize: http.DetectContentType may return "image/jpeg; charset=utf-8" etc.
	if idx := len(detectedType); idx > 0 {
		for i, c := range detectedType {
			if c == ';' {
				detectedType = detectedType[:i]
				break
			}
		}
	}
	if !allowedMimeTypes[detectedType] {
		return nil, errors.New("file content does not match an allowed type")
	}

	// Seek back to beginning for the copy
	if seeker, ok := src.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek file: %w", err)
		}
	}

	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Create database record
	upload := &model.UserUpload{
		ID:           uuid.New(),
		UserID:       userID,
		FileType:     fileType,
		OriginalName: file.Filename,
		FilePath:     filePath,
		FileSize:     file.Size,
		MimeType:     mimeType,
		OCRStatus:    "pending",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.uploadRepo.Create(ctx, upload); err != nil {
		// Clean up file on DB error
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save upload record: %w", err)
	}

	// Trigger OCR via JobRuntime for reports
	if fileType == "report" {
		go s.processOCRWithJob(upload.ID, userID, filePath, mimeType)
	}

	return upload, nil
}

// GetUploads retrieves all uploads for a user.
func (s *UploadService) GetUploads(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error) {
	return s.uploadRepo.GetByUserID(ctx, userID)
}

// GetUpload retrieves a single upload by ID, verifying ownership.
func (s *UploadService) GetUpload(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) (*model.UserUpload, error) {
	upload, err := s.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if upload == nil {
		return nil, errors.New("upload not found")
	}
	if upload.UserID != userID {
		return nil, errors.New("unauthorized")
	}
	return upload, nil
}

// DeleteUpload deletes an upload and its file from disk.
func (s *UploadService) DeleteUpload(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) error {
	upload, err := s.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return err
	}
	if upload == nil {
		return errors.New("upload not found")
	}
	if upload.UserID != userID {
		return errors.New("unauthorized")
	}

	// Delete file from disk
	if err := os.Remove(upload.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete database record
	return s.uploadRepo.Delete(ctx, uploadID, userID)
}

// executeOCRCall sends a file to the AI service OCR endpoint and returns the response body.
// This is the shared implementation used by both processOCR and processOCRWithJob.
func (s *UploadService) executeOCRCall(filePath, mimeType string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(filePath))},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create form part: %w", err)
	}

	if _, err = io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}
	writer.Close()

	resp, err := http.Post(
		s.aiServiceURL+"/api/ocr/extract",
		writer.FormDataContentType(),
		body,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI service: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCR response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OCR service returned status %d", resp.StatusCode)
	}

	return respBody, nil
}

// processOCRWithJob executes OCR through JobRuntime, creating a durable job with idempotency.
func (s *UploadService) processOCRWithJob(uploadID, userID uuid.UUID, filePath string, mimeType string) {
	ctx := context.Background()

	// Idempotency: check if OCR is already in progress or completed for this upload
	upload, err := s.uploadRepo.GetByID(ctx, uploadID)
	if err == nil && upload != nil && (upload.OCRStatus == "processing" || upload.OCRStatus == "completed") {
		log.Printf("OCR already %s for upload %s, skipping", upload.OCRStatus, uploadID)
		return
	}

	// Create a durable OCR job
	inputJSON, _ := json.Marshal(map[string]any{
		"upload_id": uploadID.String(),
		"file_path": filePath,
		"mime_type": mimeType,
	})
	job, err := s.jobRuntime.CreateJob(ctx, userID, "ocr", inputJSON, nil, nil)
	if err != nil {
		log.Printf("failed to create OCR job for upload %s: %v", uploadID, err)
		return
	}

	// Transition to running
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, "running", nil, nil); err != nil {
		log.Printf("failed to start OCR job %s: %v", job.ID, err)
		return
	}

	// Update upload status
	_ = s.uploadRepo.UpdateOCRStatus(ctx, uploadID, userID, "processing")

	// Execute OCR using shared implementation
	respBody, err := s.executeOCRCall(filePath, mimeType)
	if err != nil {
		s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, userID, "failed",
			json.RawMessage(fmt.Sprintf(`{"error": "%s"}`, err.Error())))
		return
	}

	// Success: update both job and upload
	s.jobRuntime.TransitionTo(ctx, job.ID, "succeeded", json.RawMessage(respBody), nil)
	_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, userID, "completed", respBody)
}
