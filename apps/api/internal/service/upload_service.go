package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	aiServiceURL             string
	healthDocumentServiceURL string
	deployment               *AgentDeploymentPolicy
	healthDocumentDeployment *HealthDocumentDeploymentPolicy
	storage                  *uploadstorage.Registry
}

// NewUploadService creates a new UploadService.
func NewUploadService(
	assets userAssetRepository,
	extractionRuns documentExtractionRunRepository,
	postureRuns postureAnalysisRunRepository,
	jobRuntime *JobRuntime,
	outputReviewService *OutputReviewService,
	storage *uploadstorage.Registry,
) *UploadService {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8100"
	}
	healthDocumentServiceURL := os.Getenv("HEALTH_DOCUMENT_SERVICE_URL")
	if healthDocumentServiceURL == "" {
		healthDocumentServiceURL = aiServiceURL
	}
	return &UploadService{
		assets:                   assets,
		extractionRuns:           extractionRuns,
		postureRuns:              postureRuns,
		jobRuntime:               jobRuntime,
		outputReviewService:      outputReviewService,
		aiServiceURL:             aiServiceURL,
		healthDocumentServiceURL: healthDocumentServiceURL,
		storage:                  storage,
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
		log.Printf("failed to create derived run rows for asset %s: %v", asset.ID, err)
	}

	switch fileType {
	case "report":
		if _, _, err := s.enqueueOCRJob(ctx, asset.ID, userID); err != nil {
			log.Printf("failed to enqueue OCR job for asset %s: %v", asset.ID, err)
		}
	case "photo_front", "photo_side", "photo_back":
		if _, _, err := s.enqueuePostureJob(ctx, asset.ID, userID, fileType); err != nil {
			log.Printf("failed to enqueue posture job for asset %s: %v", asset.ID, err)
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

// GetUploads assembles the caller's upload views from the asset manifests and
// the latest derived run state of each asset.
func (s *UploadService) GetUploads(ctx context.Context, userID uuid.UUID) ([]model.UserUploadView, error) {
	assets, err := s.assets.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	extractionRuns, err := s.extractionRuns.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	postureRuns, err := s.postureRuns.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	latestExtractionByAsset := make(map[uuid.UUID]model.DocumentExtractionRun)
	for _, run := range extractionRuns {
		if existing, ok := latestExtractionByAsset[run.AssetID]; !ok || !extractionRunNewer(run, existing) {
			latestExtractionByAsset[run.AssetID] = run
		}
	}
	postureRunByAsset := make(map[uuid.UUID]model.PostureAnalysisRun)
	for _, run := range postureRuns {
		postureRunByAsset[run.AssetID] = run
	}
	views := make([]model.UserUploadView, 0, len(assets))
	for i := range assets {
		view := assetToUploadView(&assets[i])
		if run, ok := latestExtractionByAsset[assets[i].ID]; ok {
			applyExtractionToView(&view, &run)
		}
		if run, ok := postureRunByAsset[assets[i].ID]; ok {
			applyPostureToView(&view, &run)
		}
		views = append(views, view)
	}
	return views, nil
}

func extractionRunNewer(candidate, current model.DocumentExtractionRun) bool {
	if candidate.CreatedAt.After(current.CreatedAt) {
		return true
	}
	if candidate.CreatedAt.Before(current.CreatedAt) {
		return false
	}
	return candidate.ID.String() > current.ID.String()
}

// GetUpload assembles one owned upload view. Foreign and missing assets are
// the same owner-masked absence.
func (s *UploadService) GetUpload(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (*model.UserUploadView, error) {
	asset, err := s.loadOwnedAsset(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	return s.projectAssetView(ctx, asset)
}

// loadOwnedAsset returns the caller's asset or ErrUploadNotFound. A foreign
// asset is deliberately indistinguishable from a missing one.
func (s *UploadService) loadOwnedAsset(ctx context.Context, userID, assetID uuid.UUID) (*model.UserAsset, error) {
	asset, err := s.assets.GetOwnedByID(ctx, assetID, userID)
	if err != nil {
		return nil, err
	}
	if asset == nil || asset.UserID != userID {
		return nil, ErrUploadNotFound
	}
	return asset, nil
}

// projectAssetView assembles the public upload projection of one asset with
// its latest derived run state.
func (s *UploadService) projectAssetView(ctx context.Context, asset *model.UserAsset) (*model.UserUploadView, error) {
	view := assetToUploadView(asset)
	if asset.Kind == model.UserAssetKindReport {
		run, err := s.extractionRuns.GetLatestOwnedByAsset(ctx, asset.ID, asset.UserID)
		if err != nil {
			return nil, err
		}
		applyExtractionToView(&view, run)
	}
	if asset.IsPosturePhoto() {
		run, err := s.postureRuns.GetOwnedByAsset(ctx, asset.ID, asset.UserID)
		if err != nil {
			return nil, err
		}
		applyPostureToView(&view, run)
	}
	return &view, nil
}

func assetToUploadView(asset *model.UserAsset) model.UserUploadView {
	return model.UserUploadView{
		ID:             asset.ID,
		UserID:         asset.UserID,
		FileType:       asset.FileType(),
		OriginalName:   asset.OriginalName,
		FileSize:       asset.ByteSize,
		MimeType:       asset.MimeType,
		OCRStatus:      "pending",
		AnalysisStatus: "none",
		Availability:   asset.Availability,
		CreatedAt:      asset.CreatedAt,
		UpdatedAt:      asset.UpdatedAt,
	}
}

func applyExtractionToView(view *model.UserUploadView, run *model.DocumentExtractionRun) {
	if run == nil {
		return
	}
	view.OCRStatus = string(run.Status)
	if len(run.ResultPayload) > 0 {
		view.OCRResult = json.RawMessage(run.ResultPayload)
	}
}

func applyPostureToView(view *model.UserUploadView, run *model.PostureAnalysisRun) {
	if run == nil {
		return
	}
	view.AnalysisStatus = string(run.Status)
	if len(run.ResultPayload) > 0 {
		view.AnalysisResult = json.RawMessage(run.ResultPayload)
	}
}

// ReadImageDataURL loads an owned, still-available image through
// UploadStorage and returns a data URL suitable for multimodal LLM input.
// The object manifest size is checked before any bytes cross the model
// boundary.
func (s *UploadService) ReadImageDataURL(ctx context.Context, userID, assetID uuid.UUID) (string, string, error) {
	asset, err := s.loadOwnedAsset(ctx, userID, assetID)
	if err != nil {
		return "", "", err
	}
	if asset.Availability != model.UserAssetAvailable {
		return "", "", ErrUploadSourceUnavailable
	}
	mime := asset.MimeType
	if !strings.HasPrefix(mime, "image/") {
		return "", "", errors.New("upload is not an image")
	}
	const maxBytes = 8 << 20 // 8 MiB
	if asset.ByteSize > maxBytes {
		return "", "", errors.New("image too large for consultation multimodal input")
	}
	data, err := readUploadObject(ctx, s.storage, asset, maxBytes)
	if err != nil {
		return "", "", fmt.Errorf("read upload object: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), mime, nil
}

// OpenOwnedAssetSource streams the private original of one owned asset after
// the availability check. Source-removed and erased assets fail closed: the
// historical extraction evidence stays readable, but nobody may present the
// original as currently verifiable.
func (s *UploadService) OpenOwnedAssetSource(ctx context.Context, userID, assetID uuid.UUID) (io.ReadCloser, *model.UserAsset, error) {
	asset, err := s.loadOwnedAsset(ctx, userID, assetID)
	if err != nil {
		return nil, nil, err
	}
	if asset.Availability != model.UserAssetAvailable {
		return nil, nil, ErrUploadSourceUnavailable
	}
	reader, _, err := openUploadObject(ctx, s.storage, asset)
	if err != nil {
		return nil, nil, fmt.Errorf("open upload source: %w", err)
	}
	return reader, asset, nil
}

// RemoveSource executes the PC-D7 source-removal command: it removes the
// private original object and marks the asset source_removed while keeping
// the asset row, every derived run and every adopted indicator review, so
// historical evidence stays readable with an explicit source-unavailable
// marker. It is not erasure (AS-002 owns that lifecycle) and it never
// revokes historical admission. Repeating the command is idempotent: an
// already source-removed asset succeeds again and only retries the object
// deletion if the first attempt failed.
func (s *UploadService) RemoveSource(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (*model.UserAsset, error) {
	asset, err := s.loadOwnedAsset(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	if asset.Availability == model.UserAssetAvailable {
		transitioned, err := s.assets.MarkSourceRemoved(ctx, assetID, userID)
		if err != nil {
			return nil, err
		}
		if !transitioned {
			// A concurrent caller moved the lifecycle first; reload to observe
			// the surviving state instead of assuming anything.
			asset, err = s.loadOwnedAsset(ctx, userID, assetID)
			if err != nil {
				return nil, err
			}
		}
	}
	if asset.Availability == model.UserAssetErased {
		// The erasure lifecycle (AS-002) already removed the source; nothing
		// further to remove and nothing to resurrect here.
		return asset, nil
	}
	if s.storage == nil {
		return nil, errors.New("upload storage is not configured")
	}
	store, err := s.storage.Store(asset.StorageBackend)
	if err != nil {
		return nil, err
	}
	if err := store.Delete(ctx, asset.StorageKey); err != nil {
		return nil, fmt.Errorf("failed to delete upload object: %w", err)
	}
	return asset, nil
}
