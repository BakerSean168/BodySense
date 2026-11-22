package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"io"
	"strings"
)

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
	var asset *model.UserAsset
	err := s.assets.WithStorageLock(ctx, assetID, func(ctx context.Context) error {
		var err error
		asset, err = s.loadOwnedAsset(ctx, userID, assetID)
		if err != nil {
			return err
		}
		if err := s.assets.TrackStorageReplica(ctx, sourceReplica(asset, asset.StorageBackend)); err != nil {
			return err
		}
		if asset.Availability == model.UserAssetAvailable {
			if _, err := s.assets.MarkSourceRemoved(ctx, assetID, userID); err != nil {
				return err
			}
		}
		// Always refresh after CAS: both the response and cleanup use durable state.
		asset, err = s.loadOwnedAsset(ctx, userID, assetID)
		if err != nil {
			return err
		}
		if asset.Availability != model.UserAssetSourceRemoved && asset.Availability != model.UserAssetErased {
			return ErrUploadSourceUnavailable
		}
		replicas, err := s.assets.ListStorageReplicas(ctx, assetID, userID)
		if err != nil {
			return err
		}
		var cleanupErr error
		for _, replica := range replicas {
			cleanupErr = errors.Join(cleanupErr, deleteSourceReplica(ctx, s.storage, s.assets, replica))
		}
		return cleanupErr
	})
	if err != nil {
		return nil, err
	}
	return asset, nil
}
