package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	MaxFileSize    = 10 << 20 // 10MB
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
	"consultation_photo": true, // ad-hoc chat attachment (no auto posture job)
	"photo_front":        true,
	"photo_side":         true,
	"photo_back":         true,
	"report":             true,
}

type uploadRepository interface {
	Create(ctx context.Context, upload *model.UserUpload) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.UserUpload, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	UpdateOCRResult(ctx context.Context, id, userID uuid.UUID, status string, result json.RawMessage) error
	UpdateOCRStatus(ctx context.Context, id, userID uuid.UUID, status string) error
	UpdateAnalysisStatus(ctx context.Context, id, userID uuid.UUID, status string) error
	UpdateAnalysisResult(ctx context.Context, id, userID uuid.UUID, status string, result json.RawMessage) error
	UpdateAgentConfiguration(ctx context.Context, id uuid.UUID, configurationID string) error
	GetLatestPostureAnalyses(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error)
}

// UploadService handles upload business logic.
type UploadService struct {
	uploadRepo          uploadRepository
	jobRuntime          *JobRuntime
	outputReviewService *OutputReviewService
	aiServiceURL        string
	deployment          *AgentDeploymentPolicy
	storage             *uploadstorage.Registry
}

// NewUploadService creates a new UploadService.
func NewUploadService(
	uploadRepo uploadRepository,
	jobRuntime *JobRuntime,
	outputReviewService *OutputReviewService,
	storage *uploadstorage.Registry,
) *UploadService {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8100"
	}
	return &UploadService{
		uploadRepo:          uploadRepo,
		jobRuntime:          jobRuntime,
		outputReviewService: outputReviewService,
		aiServiceURL:        aiServiceURL,
		storage:             storage,
	}
}

// WithDeployment attaches the Go-owned deployment policy so posture analysis
// resolves its champion Agent configuration through the North-Star control plane.
func (s *UploadService) WithDeployment(deployment *AgentDeploymentPolicy) *UploadService {
	s.deployment = deployment
	return s
}

// UploadFile validates an owned upload, commits the immutable storage object,
// creates its DB manifest, and only then enqueues derived AI work. Storage is
// written before the row so the database never points at an object that was not
// durably committed; a DB failure attempts an idempotent object rollback.
func (s *UploadService) UploadFile(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader, fileType string) (*model.UserUpload, error) {
	if s.storage == nil {
		return nil, errors.New("upload storage is not configured")
	}
	if !allowedFileTypes[fileType] {
		return nil, errors.New("invalid file type: must be consultation_photo, photo_front, photo_side, photo_back, or report")
	}
	if file.Size > MaxFileSize {
		return nil, errors.New("file size exceeds 10MB limit")
	}
	mimeType := file.Header.Get("Content-Type")
	if !allowedMimeTypes[mimeType] {
		return nil, errors.New("invalid file type: only JPEG, PNG, WebP, and PDF are allowed")
	}

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()
	buf := make([]byte, 512)
	n, err := src.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	detectedType := http.DetectContentType(buf[:n])
	if separator := strings.IndexByte(detectedType, ';'); separator >= 0 {
		detectedType = detectedType[:separator]
	}
	if !allowedMimeTypes[detectedType] {
		return nil, errors.New("file content does not match an allowed type")
	}
	if mimeType != detectedType {
		return nil, fmt.Errorf("declared MIME type %s does not match detected content %s", mimeType, detectedType)
	}

	uploadID := uuid.New()
	storageKey, err := uploadstorage.BuildObjectKey(userID, uploadID, detectedType)
	if err != nil {
		return nil, err
	}
	store := s.storage.DefaultStore()
	body := io.MultiReader(bytes.NewReader(buf[:n]), src)
	if err := store.Put(ctx, storageKey, body, file.Size, detectedType); err != nil {
		return nil, fmt.Errorf("failed to store upload object: %w", err)
	}

	analysisStatus := "none"
	switch fileType {
	case "photo_front", "photo_side", "photo_back":
		analysisStatus = "pending"
	}
	now := time.Now()
	upload := &model.UserUpload{
		ID:             uploadID,
		UserID:         userID,
		FileType:       fileType,
		OriginalName:   file.Filename,
		StorageBackend: store.Backend(),
		StorageKey:     storageKey,
		FileSize:       file.Size,
		MimeType:       mimeType,
		OCRStatus:      "pending",
		AnalysisStatus: analysisStatus,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.uploadRepo.Create(ctx, upload); err != nil {
		if cleanupErr := store.Delete(ctx, storageKey); cleanupErr != nil {
			return nil, fmt.Errorf("failed to save upload record: %w; rollback upload object: %v", err, cleanupErr)
		}
		return nil, fmt.Errorf("failed to save upload record: %w", err)
	}

	switch fileType {
	case "report":
		if _, _, err := s.enqueueOCRJob(ctx, upload.ID, userID); err != nil {
			log.Printf("failed to enqueue OCR job for upload %s: %v", upload.ID, err)
		}
	case "photo_front", "photo_side", "photo_back":
		if _, _, err := s.enqueuePostureJob(ctx, upload.ID, userID, fileType); err != nil {
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

// ReadImageDataURL loads an owned image through UploadStorage and returns a
// data URL suitable for multimodal LLM input. The object manifest size is
// checked before any bytes cross the model boundary.
func (s *UploadService) ReadImageDataURL(ctx context.Context, userID, uploadID uuid.UUID) (string, string, error) {
	upload, err := s.GetUpload(ctx, userID, uploadID)
	if err != nil {
		return "", "", err
	}
	mime := upload.MimeType
	if !strings.HasPrefix(mime, "image/") {
		return "", "", errors.New("upload is not an image")
	}
	const maxBytes = 8 << 20 // 8 MiB
	if upload.FileSize > maxBytes {
		return "", "", errors.New("image too large for consultation multimodal input")
	}
	data, err := readUploadObject(ctx, s.storage, upload, maxBytes)
	if err != nil {
		return "", "", fmt.Errorf("read upload object: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), mime, nil
}

// DeleteUpload removes the private storage object before deleting its DB
// manifest. Storage deletion is idempotent, so a DB failure remains retryable.
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
	if s.storage == nil {
		return errors.New("upload storage is not configured")
	}
	store, err := s.storage.Store(upload.StorageBackend)
	if err != nil {
		return err
	}
	if err := store.Delete(ctx, upload.StorageKey); err != nil {
		return fmt.Errorf("failed to delete upload object: %w", err)
	}
	return s.uploadRepo.Delete(ctx, uploadID, userID)
}

// executeOCRCall streams an already-authorized upload to the AI service. Go
// remains the blob authority; Python receives only the request body, never OSS
// credentials or an object-store location.
func (s *UploadService) executeOCRCall(reader io.Reader, mimeType string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="upload"`},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create form part: %w", err)
	}
	if _, err = io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize OCR multipart body: %w", err)
	}

	resp, err := http.Post(s.aiServiceURL+"/api/ocr/extract", writer.FormDataContentType(), body)
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

func (s *UploadService) enqueueOCRJob(ctx context.Context, uploadID, userID uuid.UUID) (*model.Job, bool, error) {
	idempotencyKey := fmt.Sprintf("upload_ocr:%s", uploadID.String())
	inputJSON, _ := json.Marshal(ocrJobInput{UploadID: uploadID.String()})
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
	upload, err := s.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("load OCR upload: %w", err)
	}
	if upload == nil || upload.UserID != job.UserID {
		err := errors.New("OCR upload is missing or not owned by job user")
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		return err
	}

	if err := s.jobRuntime.TransitionTo(ctx, job.ID, "running", nil, nil); err != nil {
		return fmt.Errorf("start OCR job: %w", err)
	}
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "ocr_processing", "percent": 10})
	_ = s.uploadRepo.UpdateOCRStatus(ctx, uploadID, job.UserID, "processing")

	reader, _, err := openUploadObject(ctx, s.storage, upload)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, job.UserID, "failed", json.RawMessage(`{"error":"upload object unavailable"}`))
		return err
	}
	respBody, callErr := s.executeOCRCall(reader, upload.MimeType)
	closeErr := reader.Close()
	if callErr == nil && closeErr != nil {
		callErr = closeErr
	}
	if callErr != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": callErr.Error()})
		errPayload, _ := json.Marshal(map[string]string{"error": callErr.Error()})
		_ = s.uploadRepo.UpdateOCRResult(ctx, uploadID, job.UserID, "failed", errPayload)
		return callErr
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
	if input.UploadID == "" {
		return input, fmt.Errorf("OCR job input missing upload_id")
	}
	return input, nil
}

// ---------------------------------------------------------------------------
// Posture analysis pipeline (mirrors the OCR pipeline above)
// ---------------------------------------------------------------------------

type postureJobInput struct {
	UploadID        string `json:"upload_id"`
	View            string `json:"view"` // "front" | "side" | "back"
	ConfigurationID string `json:"configuration_id,omitempty"`
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
func (s *UploadService) executePostureCall(reader io.Reader, mimeType, view, configurationID string) ([]byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("view", view); err != nil {
		return nil, fmt.Errorf("failed to write view field: %w", err)
	}
	if strings.TrimSpace(configurationID) == "" {
		return nil, fmt.Errorf("posture configuration id is required")
	}
	if err := writer.WriteField("configuration_id", configurationID); err != nil {
		return nil, fmt.Errorf("failed to write configuration_id field: %w", err)
	}
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="upload"`},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create form part: %w", err)
	}
	if _, err = io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize posture multipart body: %w", err)
	}
	resp, err := http.Post(s.aiServiceURL+"/api/posture/analyze", writer.FormDataContentType(), body)
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

func (s *UploadService) enqueuePostureJob(ctx context.Context, uploadID, userID uuid.UUID, fileType string) (*model.Job, bool, error) {
	view, ok := photoTypeToView[fileType]
	if !ok {
		return nil, false, fmt.Errorf("unsupported photo file_type: %s", fileType)
	}
	if s.deployment == nil {
		return nil, false, errors.New("posture deployment policy is not configured")
	}
	idempotencyKey := fmt.Sprintf("posture_analyze:%s", uploadID.String())
	inputJSON, _ := json.Marshal(postureJobInput{
		UploadID:        uploadID.String(),
		View:            view,
		ConfigurationID: s.deployment.PostureConfigurationID(),
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
	upload, err := s.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("load posture upload: %w", err)
	}
	if upload == nil || upload.UserID != job.UserID {
		err := errors.New("posture upload is missing or not owned by job user")
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		return err
	}
	if err := s.jobRuntime.TransitionTo(ctx, job.ID, "running", nil, nil); err != nil {
		return fmt.Errorf("start posture job: %w", err)
	}
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "posture_analyzing", "percent": 10})
	_ = s.uploadRepo.UpdateAnalysisStatus(ctx, uploadID, job.UserID, "processing")

	reader, _, err := openUploadObject(ctx, s.storage, upload)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		errPayload, _ := json.Marshal(map[string]string{"error": "upload object unavailable"})
		_ = s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "failed", errPayload)
		return err
	}
	respBody, callErr := s.executePostureCall(reader, upload.MimeType, input.View, input.ConfigurationID)
	closeErr := reader.Close()
	if callErr == nil && closeErr != nil {
		callErr = closeErr
	}
	if callErr != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": callErr.Error()})
		errPayload, _ := json.Marshal(map[string]string{"error": callErr.Error()})
		_ = s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "failed", errPayload)
		return callErr
	}

	analysisPayload, err := validatePostureAgentResponse(respBody, input.ConfigurationID)
	if err != nil {
		_ = s.jobRuntime.TransitionTo(ctx, job.ID, "failed", nil, map[string]string{"error": err.Error()})
		errPayload, _ := json.Marshal(map[string]string{"error": "posture agent identity validation failed"})
		_ = s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "failed", errPayload)
		return err
	}
	s.recordPostureGovernance(ctx, job, analysisPayload)
	_ = s.jobRuntime.UpdateProgress(ctx, job.ID, map[string]any{"stage": "posture_completed", "percent": 100})
	_ = s.jobRuntime.TransitionTo(ctx, job.ID, "completed", json.RawMessage(respBody), nil)
	_ = s.uploadRepo.UpdateAnalysisResult(ctx, uploadID, job.UserID, "completed", analysisPayload)
	if input.ConfigurationID != "" {
		_ = s.uploadRepo.UpdateAgentConfiguration(ctx, uploadID, input.ConfigurationID)
	}
	return nil
}

func validatePostureAgentResponse(respBody []byte, expectedConfigurationID string) ([]byte, error) {
	registration, ok := knownPostureConfigurations[strings.TrimSpace(expectedConfigurationID)]
	if !ok {
		return nil, fmt.Errorf("unknown Posture Agent configuration id %q", expectedConfigurationID)
	}
	var envelope struct {
		Status string         `json:"status"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode posture agent response: %w", err)
	}
	if envelope.Status != "completed" || envelope.Result == nil {
		return nil, fmt.Errorf("posture agent returned incomplete response")
	}
	configuration, ok := envelope.Result["agent_configuration"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("posture agent response missing agent_configuration")
	}
	if id, _ := configuration["id"].(string); id != expectedConfigurationID {
		return nil, fmt.Errorf("posture agent configuration mismatch: got %q want %q", id, expectedConfigurationID)
	}
	if role, _ := configuration["role"].(string); role != "posture" {
		return nil, fmt.Errorf("posture agent role mismatch: %q", role)
	}
	if policy, _ := configuration["decision_policy_revision"].(string); policy != registration.DecisionPolicyRevision {
		return nil, fmt.Errorf("posture decision policy mismatch: %q", policy)
	}
	if logicalModel, _ := configuration["logical_model"].(string); logicalModel != registration.LogicalModel {
		return nil, fmt.Errorf("posture logical model mismatch: %q", logicalModel)
	}
	if registration.MechanismRevision != "" {
		if revision, _ := configuration["geometry_mechanism_revision"].(string); revision != registration.MechanismRevision {
			return nil, fmt.Errorf("posture configuration mechanism mismatch: %q", revision)
		}
	}
	execution, ok := envelope.Result["execution_provenance"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("posture agent response missing execution_provenance")
	}
	if logicalModel, _ := execution["logical_model"].(string); logicalModel != registration.LogicalModel {
		return nil, fmt.Errorf("posture execution logical model mismatch: %q", logicalModel)
	}
	if err := validatePostureMechanismProvenance(envelope.Result, registration); err != nil {
		return nil, err
	}
	trace := map[string]any{
		"decision":                 "persist",
		"authority":                "go",
		"agent_configuration_id":   expectedConfigurationID,
		"decision_policy_revision": registration.DecisionPolicyRevision,
		"logical_model":            registration.LogicalModel,
	}
	if registration.MechanismRevision != "" {
		trace["mechanism_revision"] = registration.MechanismRevision
		trace["model_sha256"] = registration.ModelSHA256
		trace["threshold_revision"] = registration.ThresholdRevision
		trace["threshold_sha256"] = registration.ThresholdSHA256
	}
	envelope.Result["generation_decision_trace"] = trace
	return json.Marshal(envelope.Result)
}

func validatePostureMechanismProvenance(
	result map[string]any,
	registration postureConfigurationRegistration,
) error {
	if registration.MechanismRevision == "" {
		return nil
	}
	mechanism, ok := result["mechanism_provenance"].(map[string]any)
	if !ok {
		return errors.New("posture agent response missing mechanism_provenance")
	}
	if status, _ := mechanism["status"].(string); status != "verified" {
		return fmt.Errorf("posture mechanism status mismatch: %q", status)
	}
	expected := map[string]string{
		"mechanism_revision": registration.MechanismRevision,
		"engine":             registration.Engine,
		"engine_version":     registration.EngineVersion,
		"model_uri":          registration.ModelURI,
		"model_sha256":       registration.ModelSHA256,
		"threshold_revision": registration.ThresholdRevision,
		"threshold_sha256":   registration.ThresholdSHA256,
	}
	for field, want := range expected {
		got, _ := mechanism[field].(string)
		if got != want {
			return fmt.Errorf("posture mechanism %s mismatch: got %q want %q", field, got, want)
		}
	}
	return nil
}

// recordPostureGovernance audits the P2 gate result for posture analysis jobs.
func (s *UploadService) recordPostureGovernance(ctx context.Context, job model.Job, respBody []byte) {
	if s.outputReviewService == nil || len(respBody) == 0 {
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return
	}

	verdict := "unknown"
	issues := datatypes.JSON("[]")
	var validated datatypes.JSON

	if gov, ok := parsed["governance"].(map[string]any); ok {
		if v, ok := gov["verdict"].(string); ok && v != "" {
			verdict = v
		}
		if rawIssues, ok := gov["issues"]; ok {
			if b, err := json.Marshal(rawIssues); err == nil {
				issues = datatypes.JSON(b)
			}
		}
	}

	if verdict == "accepted" || verdict == "degraded" {
		safe := make(map[string]any, len(parsed))
		for k, v := range parsed {
			if k == "governance" || k == "safety_fallback" {
				continue
			}
			safe[k] = v
		}
		if b, err := json.Marshal(safe); err == nil {
			validated = datatypes.JSON(b)
		}
	}

	jobID := job.ID
	userID := job.UserID
	s.outputReviewService.RecordReview(
		ctx,
		"posture",
		verdict,
		&userID,
		nil,
		&jobID,
		nil,
		issues,
		validated,
		datatypes.JSON(respBody),
	)
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
	if input.UploadID == "" || input.View == "" || input.ConfigurationID == "" {
		return input, fmt.Errorf("posture job input missing required fields")
	}
	return input, nil
}

// GetPostureAnalyses returns the user's completed three-view posture analyses.
// Used by the profile summary and (Phase 3-B1) the consultation Agent tool.
func (s *UploadService) GetPostureAnalyses(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error) {
	return s.uploadRepo.GetLatestPostureAnalyses(ctx, userID)
}

// PostureAnalysisView is one completed single-view analysis, stripped of raw
// file metadata so consultation tools and assessment can consume it safely.
type PostureAnalysisView struct {
	UploadID       string          `json:"upload_id"`
	View           string          `json:"view"`
	FileType       string          `json:"file_type"`
	AnalysisStatus string          `json:"analysis_status"`
	Analysis       json.RawMessage `json:"analysis"`
	CreatedAt      time.Time       `json:"created_at"`
}

// PostureAnalysisSummary is the Agent/assessment-facing aggregate of the
// user's latest completed three-view posture analyses.
type PostureAnalysisSummary struct {
	HasAnalysis bool                  `json:"has_analysis"`
	Views       []PostureAnalysisView `json:"views"`
	// Findings is a flattened list of finding objects across views for quick use.
	Findings []any `json:"findings"`
	// Summaries is per-view summary_markdown text when present.
	Summaries []string `json:"summaries"`
}

// viewFromFileType maps upload file_type to the posture view id.
func viewFromFileType(fileType string) string {
	switch fileType {
	case "photo_front":
		return "front"
	case "photo_side":
		return "side"
	case "photo_back":
		return "back"
	default:
		return fileType
	}
}

// BuildPostureAnalysisSummary collapses completed posture uploads into the
// compact shape shared by the read-only HTTP endpoint, consultation business
// context, and assessment reuse. Prefer the newest completed row per view.
func BuildPostureAnalysisSummary(uploads []model.UserUpload) PostureAnalysisSummary {
	summary := PostureAnalysisSummary{
		HasAnalysis: false,
		Views:       []PostureAnalysisView{},
		Findings:    []any{},
		Summaries:   []string{},
	}
	if len(uploads) == 0 {
		return summary
	}

	seenView := map[string]bool{}
	for _, upload := range uploads {
		view := viewFromFileType(upload.FileType)
		if seenView[view] {
			continue
		}
		if len(upload.AnalysisResult) == 0 {
			continue
		}
		seenView[view] = true
		summary.HasAnalysis = true
		summary.Views = append(summary.Views, PostureAnalysisView{
			UploadID:       upload.ID.String(),
			View:           view,
			FileType:       upload.FileType,
			AnalysisStatus: upload.AnalysisStatus,
			Analysis:       upload.AnalysisResult,
			CreatedAt:      upload.CreatedAt,
		})

		var parsed map[string]any
		if err := json.Unmarshal(upload.AnalysisResult, &parsed); err == nil {
			if findings, ok := parsed["findings"].([]any); ok {
				summary.Findings = append(summary.Findings, findings...)
			}
			if text, ok := parsed["summary_markdown"].(string); ok && text != "" {
				summary.Summaries = append(summary.Summaries, text)
			}
		}
	}
	return summary
}

// GetPostureAnalysisSummary loads and collapses the caller's completed posture
// analyses. Empty result (no completed analysis) is not an error.
func (s *UploadService) GetPostureAnalysisSummary(ctx context.Context, userID uuid.UUID) (PostureAnalysisSummary, error) {
	uploads, err := s.GetPostureAnalyses(ctx, userID)
	if err != nil {
		return PostureAnalysisSummary{}, err
	}
	return BuildPostureAnalysisSummary(uploads), nil
}
