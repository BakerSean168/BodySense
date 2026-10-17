package repository

import (
	"context"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserAssetRepository persists the sole owner-scoped file identity. Derived
// OCR/posture state lives in the run repositories; this repository never
// touches run or review data.
type UserAssetRepository struct {
	db *gorm.DB
}

// NewUserAssetRepository creates a new UserAssetRepository.
func NewUserAssetRepository(db *gorm.DB) *UserAssetRepository {
	return &UserAssetRepository{db: db}
}

// Create persists a new user asset manifest.
func (r *UserAssetRepository) Create(ctx context.Context, asset *model.UserAsset) error {
	return database.FromContext(ctx, r.db).Create(asset).Error
}

// GetOwnedByID retrieves one asset enforced by owner scope, or nil when the
// asset does not exist for this owner.
func (r *UserAssetRepository) GetOwnedByID(ctx context.Context, id, userID uuid.UUID) (*model.UserAsset, error) {
	var asset model.UserAsset
	err := database.FromContext(ctx, r.db).
		Where("id = ? AND user_id = ?", id, userID).
		First(&asset).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

// ListByUserID retrieves all assets for a user, newest first.
func (r *UserAssetRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserAsset, error) {
	var assets []model.UserAsset
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&assets).Error
	if err != nil {
		return nil, err
	}
	return assets, nil
}

// MarkSourceRemoved performs exactly available -> source_removed. It is the
// only availability transition this repository owns: erasure transitions
// (AS-002) are a separate lifecycle and never pass through here. A zero
// row count means the asset is absent, foreign, or already past available;
// callers re-read the asset to distinguish the idempotent case.
func (r *UserAssetRepository) MarkSourceRemoved(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserAsset{}).
		Where("id = ? AND user_id = ? AND availability = ?", id, userID, model.UserAssetAvailable).
		Updates(map[string]any{
			"availability":      model.UserAssetSourceRemoved,
			"source_removed_at": gorm.Expr("NOW()"),
			"updated_at":        gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ListByStorageBackend returns a stable batch for one storage backend.
func (r *UserAssetRepository) ListByStorageBackend(ctx context.Context, backend string, after *uuid.UUID, limit int) ([]model.UserAsset, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := database.FromContext(ctx, r.db).Where("storage_backend = ?", backend)
	if after != nil {
		query = query.Where("id > ?", *after)
	}
	var assets []model.UserAsset
	if err := query.Order("id ASC").Limit(limit).Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}

// CompareAndSwapStorageBackend advances one manifest only when its source
// identity still matches the object that was copied and verified.
func (r *UserAssetRepository) CompareAndSwapStorageBackend(
	ctx context.Context, id, userID uuid.UUID, fromBackend, storageKey, toBackend string,
) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserAsset{}).
		Where("id = ? AND user_id = ? AND storage_backend = ? AND storage_key = ?", id, userID, fromBackend, storageKey).
		Updates(map[string]any{"storage_backend": toBackend, "updated_at": gorm.Expr("NOW()")})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
