package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bodysense/api/internal/config"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

const (
	MaxFileSize    = 10 << 20 // 10MB
	ocrJobType     = "upload.ocr_extract"
	postureJobType = "upload.posture_analyze"
)

// Allowed file types
var allowedFileTypes = map[string]bool{
	"consultation_photo": true, // ad-hoc chat attachment (no auto posture job)
	"photo_front":        true,
	"photo_side":         true,
	"photo_back":         true,
	"report":             true,
}

// Allowed MIME types
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/webp":      true,
	"application/pdf": true,
}

// ErrUploadNotFound is the owner-masked absence error: a missing asset and a
// foreign asset are indistinguishable to the caller.
var ErrUploadNotFound = errors.New("upload not found")

// ErrUploadSourceUnavailable means the asset exists but its private original
// is no longer readable (source removal or erasure); no caller may treat the
// historical extraction as a currently verifiable original.
var ErrUploadSourceUnavailable = errors.New("upload source is unavailable")

// userAssetRepository is the file-identity port the upload domain needs.
type userAssetRepository interface {
	Create(ctx context.Context, asset *model.UserAsset) error
	GetOwnedByID(ctx context.Context, id, userID uuid.UUID) (*model.UserAsset, error)
	uploadSourceReplicaRepository
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserAsset, error)
	MarkSourceRemoved(ctx context.Context, id, userID uuid.UUID) (bool, error)
}

// documentExtractionRunRepository is the derived OCR-run port: run creation,
// lifecycle transitions and owner-scoped reads.
type documentExtractionRunRepository interface {
	Create(ctx context.Context, run *model.DocumentExtractionRun) error
	GetOwnedByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.DocumentExtractionRun, error)
	GetLatestOwnedByAsset(ctx context.Context, assetID uuid.UUID, userID uuid.UUID) (*model.DocumentExtractionRun, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.DocumentExtractionRun, error)
	BeginProcessing(ctx context.Context, runID, userID, jobID uuid.UUID) (bool, error)
	CompleteRun(ctx context.Context, run *model.DocumentExtractionRun) (bool, error)
	FailRun(ctx context.Context, runID, userID uuid.UUID, payload json.RawMessage) (bool, error)
}

// postureAnalysisRunRepository is the derived posture-run port.
type postureAnalysisRunRepository interface {
	Create(ctx context.Context, run *model.PostureAnalysisRun) error
	GetOwnedByAsset(ctx context.Context, assetID, userID uuid.UUID) (*model.PostureAnalysisRun, error)
	ListCompletedByUser(ctx context.Context, userID uuid.UUID) ([]model.PostureAnalysisRun, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.PostureAnalysisRun, error)
	BeginProcessing(ctx context.Context, runID, userID, jobID uuid.UUID) (bool, error)
	CompleteRun(ctx context.Context, runID, userID uuid.UUID, agentConfigurationID string, payload json.RawMessage) (bool, error)
	FailRun(ctx context.Context, runID, userID uuid.UUID, payload json.RawMessage) (bool, error)
}

// UploadService owns the asset file identity, its derived AI runs and the
// source-removal command. It never mutates extraction runs after completion
// and never deletes adopted evidence.
type UploadService struct {
	assets                   userAssetRepository
	extractionRuns           documentExtractionRunRepository
	postureRuns              postureAnalysisRunRepository
	jobRuntime               *JobRuntime
	outputReviewService      *OutputReviewService
	aiClient                 *AIClient
	healthDocumentClient     *AIClient
	deployment               *AgentDeploymentPolicy
	healthDocumentDeployment *HealthDocumentDeploymentPolicy
	storage                  *uploadstorage.Registry
}

// NewUploadService creates a new UploadService. The AI and health-document
// endpoints arrive resolved from the composition root; see config.Upload.
func NewUploadService(
	assets userAssetRepository,
	extractionRuns documentExtractionRunRepository,
	postureRuns postureAnalysisRunRepository,
	jobRuntime *JobRuntime,
	outputReviewService *OutputReviewService,
	storage *uploadstorage.Registry,
	cfg config.Upload,
) *UploadService {
	return &UploadService{
		assets:               assets,
		extractionRuns:       extractionRuns,
		postureRuns:          postureRuns,
		jobRuntime:           jobRuntime,
		outputReviewService:  outputReviewService,
		aiClient:             newAIClient(cfg.AIServiceURL, uploadJobTimeout),
		healthDocumentClient: newAIClient(cfg.HealthDocumentServiceURL, uploadJobTimeout),
		storage:              storage,
	}
}

// WithDeployment attaches the Go-owned deployment policy so posture analysis
// resolves its champion Agent configuration through the North-Star control plane.
func (s *UploadService) WithDeployment(deployment *AgentDeploymentPolicy) *UploadService {
	s.deployment = deployment
	return s
}

func (s *UploadService) WithHealthDocumentDeployment(deployment *HealthDocumentDeploymentPolicy) *UploadService {
	s.healthDocumentDeployment = deployment
	return s
}

func (s *UploadService) resolveHealthDocumentConfigurationID() string {
	if s.healthDocumentDeployment != nil {
		return s.healthDocumentDeployment.ConfigurationID()
	}
	return defaultHealthDocumentChampionID
}

// UploadFile validates an owned upload, commits the immutable storage object
// (hashing the exact bytes it stores), creates the user_assets manifest plus
// the derived pending run rows, and only then enqueues the AI work. Storage
// is written before any row so the database never points at an object that
// was not durably committed; a DB failure attempts an idempotent object
// rollback.
func (s *UploadService) UploadFile(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader, fileType string) (*model.UserUploadView, error) {
	if s.storage == nil {
		return nil, errors.New("upload storage is not configured")
	}
	kind, photoView, known := model.UserAssetKindFromFileType(fileType)
	if !known {
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

	assetID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate asset identity: %w", err)
	}
	storageKey, err := uploadstorage.BuildObjectKey(userID, assetID, detectedType)
	if err != nil {
		return nil, err
	}
	store := s.storage.DefaultStore()
	contentHasher := sha256.New()
	body := io.TeeReader(io.MultiReader(bytes.NewReader(buf[:n]), src), contentHasher)
	if err := store.Put(ctx, storageKey, body, file.Size, detectedType); err != nil {
		return nil, fmt.Errorf("failed to store upload object: %w", err)
	}

	now := time.Now()
	asset := &model.UserAsset{
		ID:             assetID,
		UserID:         userID,
		Kind:           kind,
		PhotoView:      photoViewPtr(photoView),
		OriginalName:   file.Filename,
		MimeType:       mimeType,
		ByteSize:       file.Size,
		ContentSHA256:  hex.EncodeToString(contentHasher.Sum(nil)),
		StorageBackend: store.Backend(),
		StorageKey:     storageKey,
		Availability:   model.UserAssetAvailable,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.assets.Create(ctx, asset); err != nil {
		if cleanupErr := store.Delete(ctx, storageKey); cleanupErr != nil {
			return nil, fmt.Errorf("failed to save upload record: %w; rollback upload object: %v", err, cleanupErr)
		}
		return nil, fmt.Errorf("failed to save upload record: %w", err)
	}

	if err := s.createDerivedRuns(ctx, asset); err != nil {
		slog.Default().Error("create_derived_run_rows_failed", "asset_id", asset.ID, "error", err)
	}

	switch fileType {
	case "report":
		if _, _, err := s.enqueueOCRJob(ctx, asset.ID, userID); err != nil {
			slog.Default().Error("enqueue_ocr_job_failed", "asset_id", asset.ID, "error", err)
		}
	case "photo_front", "photo_side", "photo_back":
		if _, _, err := s.enqueuePostureJob(ctx, asset.ID, userID, fileType); err != nil {
			slog.Default().Error("enqueue_posture_job_failed", "asset_id", asset.ID, "error", err)
		}
	}
	return s.projectAssetView(ctx, asset)
}

// photoViewPtr converts the empty concrete-view value into a NULL column.
func photoViewPtr(view model.PostureView) *model.PostureView {
	if view == "" {
		return nil
	}
	return &view
}

// createDerivedRuns persists the pending derived-run manifests of one asset:
// one document-extraction run per report, one posture-analysis run per
// posture photo. Their rows exist before any AI work is enqueued so the run
// lifecycle is always observable and never implicit in job state.
func (s *UploadService) createDerivedRuns(ctx context.Context, asset *model.UserAsset) error {
	switch {
	case asset.Kind == model.UserAssetKindReport:
		run := &model.DocumentExtractionRun{
			UserID:          asset.UserID,
			AssetID:         asset.ID,
			Status:          model.DerivedRunPending,
			ConfigurationID: s.resolveHealthDocumentConfigurationID(),
			CreatedAt:       asset.CreatedAt,
			UpdatedAt:       asset.UpdatedAt,
		}
		return s.extractionRuns.Create(ctx, run)
	case asset.IsPosturePhoto():
		run := &model.PostureAnalysisRun{
			UserID:    asset.UserID,
			AssetID:   asset.ID,
			View:      string(*asset.PhotoView),
			Status:    model.DerivedRunPending,
			CreatedAt: asset.CreatedAt,
			UpdatedAt: asset.UpdatedAt,
		}
		return s.postureRuns.Create(ctx, run)
	default:
		return nil
	}
}
