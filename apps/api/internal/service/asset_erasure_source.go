package service

import (
	"context"
	"errors"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// EraseSourceCopies is the source_objects target of an asset erasure. It runs
// the source-removal command (asset unavailable, every tracked replica
// deleted) and then verifies independently that no tracked replica remains
// and the primary object is absent from its backend. It never touches
// derived records; the erasure service owns those targets.
func (s *UploadService) EraseSourceCopies(ctx context.Context, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error) {
	if s.storage == nil {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: 1, FailureCode: model.AssetErasureCodeStorageUnavailable}, nil
	}
	asset, err := s.loadOwnedAsset(ctx, userID, assetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	present, err := s.presentSourceReplicas(ctx, asset)
	if err != nil {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: 1, FailureCode: model.AssetErasureCodeStorageUnavailable}, nil
	}
	if _, removeErr := s.RemoveSource(ctx, userID, assetID); removeErr != nil {
		if errors.Is(removeErr, ErrUploadNotFound) {
			return model.AssetErasureOutcome{}, removeErr
		}
		remaining, _ := s.presentSourceReplicas(ctx, asset)
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: max(remaining, 1), FailureCode: model.AssetErasureCodeObjectDeleteFailed}, nil
	}
	tracked, err := s.assets.ListStorageReplicas(ctx, assetID, userID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	remaining, err := s.presentSourceReplicas(ctx, asset)
	if err != nil {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: 1, FailureCode: model.AssetErasureCodeStorageUnavailable}, nil
	}
	if len(tracked) > 0 || remaining > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: max(len(tracked), remaining), FailureCode: model.AssetErasureCodeObjectStillPresent}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: present}, nil
}

// presentSourceReplicas counts objects that exist among the asset's primary
// source and every tracked replica.
func (s *UploadService) presentSourceReplicas(ctx context.Context, asset *model.UserAsset) (int, error) {
	tracked, err := s.assets.ListStorageReplicas(ctx, asset.ID, asset.UserID)
	if err != nil {
		return 0, err
	}
	candidates := map[model.UserAssetStorageCleanup]struct{}{sourceReplica(asset, asset.StorageBackend): {}}
	for _, replica := range tracked {
		candidates[replica] = struct{}{}
	}
	present := 0
	for replica := range candidates {
		store, err := s.storage.Store(replica.StorageBackend)
		if err != nil {
			return 0, err
		}
		exists, err := store.Exists(ctx, replica.StorageKey)
		if err != nil {
			return 0, err
		}
		if exists {
			present++
		}
	}
	return present, nil
}
