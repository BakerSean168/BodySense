package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	aiServiceURL string
}

// NewUploadService creates a new UploadService.
func NewUploadService(uploadRepo *repository.UploadRepository) *UploadService {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8100"
	}
	return &UploadService{
		uploadRepo:   uploadRepo,
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

	// Validate MIME type
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

	// Save file to disk
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

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

	// Trigger OCR asynchronously for reports
	if fileType == "report" {
		go s.processOCR(upload.ID, filePath, mimeType)
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
	return s.uploadRepo.Delete(ctx, uploadID)
}

// processOCR sends the file to the AI service for OCR processing.
func (s *UploadService) processOCR(uploadID uuid.UUID, filePath string, mimeType string) {
	ctx := context.Background()

	// Update status to processing
	_ = s.uploadRepo.UpdateOCRStatus(ctx, uploadID, "processing")

	// Prepare multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, "failed",
			json.RawMessage(`{"error": "failed to open file for OCR"}`))
		return
	}
	defer file.Close()

	// Create form file with proper content type
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(filePath))},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, "failed",
			json.RawMessage(`{"error": "failed to create form file"}`))
		return
	}

	if _, err = io.Copy(part, file); err != nil {
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, "failed",
			json.RawMessage(`{"error": "failed to copy file content"}`))
		return
	}

	writer.Close()

	// Send to AI service
	resp, err := http.Post(
		s.aiServiceURL+"/api/ocr/extract",
		writer.FormDataContentType(),
		body,
	)
	if err != nil {
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, "failed",
			json.RawMessage(fmt.Sprintf(`{"error": "failed to connect to AI service: %s"}`, err.Error())))
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, "failed",
			json.RawMessage(`{"error": "failed to read OCR response"}`))
		return
	}

	if resp.StatusCode != http.StatusOK {
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, "failed",
			json.RawMessage(fmt.Sprintf(`{"error": "OCR service returned status %d"}`, resp.StatusCode)))
		return
	}

	// Update with OCR result
	_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, "completed", respBody)
}
