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
	MaxFileSize    = 10 << 20 // 10MB
	UploadDir      = "uploads"
	ocrJobType     = "upload.ocr_extract"
	postureJobType = "upload.posture_analyze"
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

	// Create database record. Photo uploads start their posture analysis in a
	// 'pending' state (an async job will pick it up); reports/other types keep
	// the default 'none'.
	analysisStatus := "none"
	switch fileType {
	case "photo_front", "photo_side", "photo_back":
		analysisStatus = "pending"
	}

	upload := &model.UserUpload{
		ID:             uuid.New(),
		UserID:         userID,
		FileType:       fileType,
		OriginalName:   file.Filename,
		FilePath:       filePath,
		FileSize:       file.Size,
		MimeType:       mimeType,
		OCRStatus:      "pending",
		AnalysisStatus: analysisStatus,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.uploadRepo.Create(ctx, upload); err != nil {
		// Clean up file on DB error
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save upload record: %w", err)
	}

	// Trigger async AI processing based on file type. Reports go through OCR;
	// posture photos go through the vision analysis pipeline. Both reuse the
	// recoverable, idempotent JobRuntime pattern.
	switch fileType {
	case "report":
		if _, _, err := s.enqueueOCRJob(ctx, upload.ID, userID, filePath, mimeType); err != nil {
			log.Printf("failed to enqueue OCR job for upload %s: %v", upload.ID, err)
		}
	case "photo_front", "photo_side", "photo_back":
		if _, _, err := s.enqueuePostureJob(ctx, upload.ID, userID, filePath, mimeType, fileType); err != nil {
			log.Printf("failed to enqueue posture job for upload %s: %v", upload.ID, err)
		}
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

type ocrJobInput struct {
	UploadID string `json:"upload_id"`
	FilePath string `json:"file_path"`
	MimeType string `json:"mime_type"`
}

// StartUploadWorker starts a background worker that recovers and processes
// both OCR and posture-analysis jobs. It replaces the OCR-only worker so a
// single loop drives every upload-derived AI job type.
func (s *UploadService) StartUploadWorker(ctx context.Context, pollInterval, staleRunningAfter time.Duration) {
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			if _, err := s.RecoverUploadJobs(ctx, 10, staleRunningAfter); err != nil {
				log.Printf("upload job recovery failed: %v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// uploadJobType describes how a recoverable upload job is processed and timed
// out. Adding a new upload-derived AI job type is a matter of registering one
// entry here.
type uploadJobType struct {
	name    string
	process func(ctx context.Context, job model.Job) error
	timeout func(ctx context.Context, job model.Job) error
}

func (s *UploadService) uploadJobTypes() []uploadJobType {
	return []uploadJobType{
		{name: ocrJobType, process: s.processOCRJob, timeout: s.timeoutOCRJob},
		{name: postureJobType, process: s.processPostureJob, timeout: s.timeoutPostureJob},
	}
}

// RecoverUploadJobs processes pending jobs and times out stale running jobs
// across every registered upload job type.
func (s *UploadService) RecoverUploadJobs(ctx context.Context, limit int, staleRunningAfter time.Duration) (int, error) {
	if s.jobRuntime == nil {
		return 0, nil
	}

	processed := 0
	for _, jt := range s.uploadJobTypes() {
		jobs, err := s.jobRuntime.ListRecoverable(ctx, jt.name, staleRunningAfter, limit)
		if err != nil {
			return processed, err
		}
		for _, job := range jobs {
			switch job.Status {
			case "pending":
				if err := jt.process(ctx, job); err != nil {
					log.Printf("failed to process %s job %s: %v", jt.name, job.ID, err)
				}
				processed++
			case "running":
				if err := jt.timeout(ctx, job); err != nil {
					log.Printf("failed to timeout stale %s job %s: %v", jt.name, job.ID, err)
				}
				processed++
			}
		}
	}
	return processed, nil
}

func (s *UploadService) enqueueOCRJob(ctx context.Context, uploadID, userID uuid.UUID, filePath string, mimeType string) (*model.Job, bool, error) {
	idempotencyKey := fmt.Sprintf("upload_ocr:%s", uploadID.String())
	inputJSON, _ := json.Marshal(ocrJobInput{
		UploadID: uploadID.String(),
		FilePath: filePath,
		MimeType: mimeType,
	})

	job, existed, err := s.jobRuntime.CreateJobWithIdempotency(ctx, userID, ocrJobType, inputJSON, idempotencyKey, nil, nil)
	if err != nil {
		return nil, false, err
	}
	return job, existed, nil
}

func (s *UploadService) processOCRJob(ctx context.Context, job model.Job) error {
	input, err := parseOCRJobInput(job)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		return err
	}

	uploadID, err := uuid.Parse(input.UploadID)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": "invalid upload_id"})
		return fmt.Errorf("invalid upload_id: %w", err)
	}

	if err := s.jobRuntime.TransitionTo(ctx, job.ID, "running", nil, nil); err != nil {
		return fmt.Errorf("start OCR job: %w", err)
	}
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "ocr_processing", "percent": 10})
	_ = s.uploadRepo.UpdateOCRStatus(ctx, uploadID, job.UserID, "processing")

	respBody, err := s.executeOCRCall(input.FilePath, input.MimeType)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, job.UserID, "failed",
			json.RawMessage(fmt.Sprintf(`{"error": "%s"}`, err.Error())))
		return err
	}

	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "ocr_completed", "percent": 100})
	_ = s.jobRuntime.TransitionTo(ctx, job.ID, "completed", json.RawMessage(respBody), nil)
	_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, job.UserID, "completed", respBody)
	return nil
}

func (s *UploadService) timeoutOCRJob(ctx context.Context, job model.Job) error {
	input, err := parseOCRJobInput(job)
	if err != nil {
		return s.jobRuntime.TransitionTo(ctx, job.ID, "timed_out", nil, map[string]string{"error": err.Error()})
	}
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, "timed_out", nil, map[string]string{"error": "stale OCR job timed out"}); err != nil {
		return err
	}
	if uploadID, parseErr := uuid.Parse(input.UploadID); parseErr == nil {
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, job.UserID, "failed", json.RawMessage(`{"error":"stale OCR job timed out"}`))
	}
	return nil
}

func parseOCRJobInput(job model.Job) (ocrJobInput, error) {
	var input ocrJobInput
	if err := json.Unmarshal(job.Input, &input); err != nil {
		return input, fmt.Errorf("parse OCR job input: %w", err)
	}
	if input.UploadID == "" || input.FilePath == "" || input.MimeType == "" {
		return input, fmt.Errorf("OCR job input missing required fields")
	}
	return input, nil
}

// ---------------------------------------------------------------------------
// Posture analysis pipeline (mirrors the OCR pipeline above)
// ---------------------------------------------------------------------------

type postureJobInput struct {
	UploadID string `json:"upload_id"`
	FilePath string `json:"file_path"`
	MimeType string `json:"mime_type"`
	View     string `json:"view"` // "front" | "side" | "back"
}

// photoTypeToView maps the upload file_type to the analysis view sent to the
// AI service.
var photoTypeToView = map[string]string{
	"photo_front": "front",
	"photo_side":  "side",
	"photo_back":  "back",
}

// executePostureCall sends a photo to the AI service posture endpoint together
// with its view and returns the raw response body.
func (s *UploadService) executePostureCall(filePath, mimeType, view string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("view", view); err != nil {
		return nil, fmt.Errorf("failed to write view field: %w", err)
	}

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
		s.aiServiceURL+"/api/posture/analyze",
		writer.FormDataContentType(),
		body,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI service: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read posture response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("posture service returned status %d", resp.StatusCode)
	}

	return respBody, nil
}

func (s *UploadService) enqueuePostureJob(ctx context.Context, uploadID, userID uuid.UUID, filePath, mimeType, fileType string) (*model.Job, bool, error) {
	view, ok := photoTypeToView[fileType]
	if !ok {
		return nil, false, fmt.Errorf("unsupported photo file_type: %s", fileType)
	}

	idempotencyKey := fmt.Sprintf("posture_analyze:%s", uploadID.String())
	inputJSON, _ := json.Marshal(postureJobInput{
		UploadID: uploadID.String(),
		FilePath: filePath,
		MimeType: mimeType,
		View:     view,
	})

	job, existed, err := s.jobRuntime.CreateJobWithIdempotency(ctx, userID, postureJobType, inputJSON, idempotencyKey, nil, nil)
	if err != nil {
		return nil, false, err
	}
	return job, existed, nil
}

func (s *UploadService) processPostureJob(ctx context.Context, job model.Job) error {
	input, err := parsePostureJobInput(job)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		return err
	}

	uploadID, err := uuid.Parse(input.UploadID)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": "invalid upload_id"})
		return fmt.Errorf("invalid upload_id: %w", err)
	}

	if err := s.jobRuntime.TransitionTo(ctx, job.ID, "running", nil, nil); err != nil {
		return fmt.Errorf("start posture job: %w", err)
	}
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "posture_analyzing", "percent": 10})
	_ = s.uploadRepo.UpdateAnalysisStatus(ctx, uploadID, job.UserID, "processing")

	respBody, err := s.executePostureCall(input.FilePath, input.MimeType, input.View)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		_ = s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "failed", errPayload)
		return err
	}

	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "posture_completed", "percent": 100})
	_ = s.jobRuntime.TransitionTo(ctx, job.ID, "completed", json.RawMessage(respBody), nil)
	_ = s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "completed", respBody)
	return nil
}

func (s *UploadService) timeoutPostureJob(ctx context.Context, job model.Job) error {
	input, err := parsePostureJobInput(job)
	if err != nil {
		return s.jobRuntime.TransitionTo(ctx, job.ID, "timed_out", nil, map[string]string{"error": err.Error()})
	}
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, "timed_out", nil, map[string]string{"error": "stale posture job timed out"}); err != nil {
		return err
	}
	if uploadID, parseErr := uuid.Parse(input.UploadID); parseErr == nil {
		_ = s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "failed", json.RawMessage(`{"error":"stale posture job timed out"}`))
	}
	return nil
}

func parsePostureJobInput(job model.Job) (postureJobInput, error) {
	var input postureJobInput
	if err := json.Unmarshal(job.Input, &input); err != nil {
		return input, fmt.Errorf("parse posture job input: %w", err)
	}
	if input.UploadID == "" || input.FilePath == "" || input.MimeType == "" || input.View == "" {
		return input, fmt.Errorf("posture job input missing required fields")
	}
	return input, nil
}

// GetPostureAnalyses returns the user's completed three-view posture analyses.
// Used by the profile summary and (Phase 3-B1) the consultation Agent tool.
func (s *UploadService) GetPostureAnalyses(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error) {
	return s.uploadRepo.GetLatestPostureAnalyses(ctx, userID)
}
