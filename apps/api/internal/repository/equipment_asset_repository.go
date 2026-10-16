package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// maxListedEquipmentAssets bounds the unpaginated inventory read.
const maxListedEquipmentAssets = 200

// EquipmentAssetRepository owns the owner-scoped equipment inventory (PC-D2).
// Every query is filtered by the authenticated owner; a foreign id is
// indistinguishable from a missing one.
type EquipmentAssetRepository struct {
	db *gorm.DB
}

func NewEquipmentAssetRepository(db *gorm.DB) *EquipmentAssetRepository {
	return &EquipmentAssetRepository{db: db}
}

// lockEquipmentOwner serialises inventory writes per owner so the idempotency
// replay and the "one current asset per name" rule are checked atomically.
// It takes a transaction-scoped advisory lock and never locks users rows.
func lockEquipmentOwner(tx *gorm.DB, owner uuid.UUID) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "equipment_assets:"+owner.String()).Error
}

func equipmentRequestDigest(fields model.EquipmentAssetFields) (string, error) {
	encoded, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func equipmentNameInUse(tx *gorm.DB, owner uuid.UUID, name string, except *uuid.UUID) (bool, error) {
	query := tx.Model(&model.EquipmentAsset{}).
		Where("user_id = ? AND availability <> ? AND lower(display_name) = lower(?)", owner, model.EquipmentRetired, name)
	if except != nil {
		query = query.Where("id <> ?", *except)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create stores a new asset. An exact retry with the same idempotency key
// returns the original row; the same key with different fields is a conflict.
func (r *EquipmentAssetRepository) Create(ctx context.Context, owner uuid.UUID, command model.CreateEquipmentAssetCommand) (*model.EquipmentAsset, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidEquipmentAsset)
	}
	command.Normalize()
	if err := command.Validate(); err != nil {
		return nil, err
	}
	digest, err := equipmentRequestDigest(command.EquipmentAssetFields)
	if err != nil {
		return nil, err
	}
	var output model.EquipmentAsset
	err = database.FromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockEquipmentOwner(tx, owner); err != nil {
			return err
		}
		var prior model.EquipmentAsset
		err := tx.Where("user_id = ? AND create_idempotency_key = ?", owner, command.IdempotencyKey).Take(&prior).Error
		if err == nil {
			if prior.CreateRequestDigest != digest {
				return model.ErrEquipmentAssetIdempotencyConflict
			}
			output = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		inUse, err := equipmentNameInUse(tx, owner, command.DisplayName, nil)
		if err != nil {
			return err
		}
		if inUse {
			return model.ErrEquipmentAssetDuplicateName
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		output = model.EquipmentAsset{
			ID: uuid.New(), UserID: owner, DisplayName: command.DisplayName, Notes: command.Notes,
			AccessKind: command.AccessKind, Availability: command.Availability,
			ValidFrom: command.ValidFrom, ValidUntil: command.ValidUntil, Revision: 1,
			CreateIdempotencyKey: command.IdempotencyKey, CreateRequestDigest: digest,
			CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&output).Error
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

// Update replaces the editable fields under optimistic concurrency. Retired is
// terminal: a retired asset keeps its history and cannot be revived (the user
// adds a new one instead). An exact replay of the update that produced the
// current revision returns the current row instead of a conflict.
func (r *EquipmentAssetRepository) Update(ctx context.Context, owner, id uuid.UUID, command model.UpdateEquipmentAssetCommand) (*model.EquipmentAsset, error) {
	if owner == uuid.Nil || id == uuid.Nil {
		return nil, model.ErrEquipmentAssetNotFound
	}
	command.Normalize()
	if err := command.Validate(); err != nil {
		return nil, err
	}
	var output model.EquipmentAsset
	err := database.FromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockEquipmentOwner(tx, owner); err != nil {
			return err
		}
		var current model.EquipmentAsset
		if err := tx.Where("user_id = ? AND id = ?", owner, id).Take(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrEquipmentAssetNotFound
			}
			return err
		}
		if current.Revision != command.ExpectedRevision {
			if current.Revision == command.ExpectedRevision+1 && current.SameFields(command.EquipmentAssetFields) {
				output = current
				return nil
			}
			return fmt.Errorf("%w: expected revision %d, current %d", model.ErrEquipmentAssetRevisionConflict, command.ExpectedRevision, current.Revision)
		}
		if current.Availability == model.EquipmentRetired {
			return model.ErrEquipmentAssetRetired
		}
		if command.Availability != model.EquipmentRetired {
			inUse, err := equipmentNameInUse(tx, owner, command.DisplayName, &id)
			if err != nil {
				return err
			}
			if inUse {
				return model.ErrEquipmentAssetDuplicateName
			}
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		result := tx.Model(&model.EquipmentAsset{}).
			Where("user_id = ? AND id = ? AND revision = ?", owner, id, current.Revision).
			Updates(map[string]any{
				"display_name": command.DisplayName, "notes": command.Notes,
				"access_kind": command.AccessKind, "availability": command.Availability,
				"valid_from": command.ValidFrom, "valid_until": command.ValidUntil,
				"revision": current.Revision + 1, "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrEquipmentAssetRevisionConflict
		}
		return tx.Where("user_id = ? AND id = ?", owner, id).Take(&output).Error
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

// List returns the owner's current (not retired) assets, newest first.
func (r *EquipmentAssetRepository) List(ctx context.Context, owner uuid.UUID) ([]model.EquipmentAsset, error) {
	items := []model.EquipmentAsset{}
	if owner == uuid.Nil {
		return items, nil
	}
	err := database.FromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND availability <> ?", owner, model.EquipmentRetired).
		Order("created_at DESC, id DESC").Limit(maxListedEquipmentAssets).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list equipment assets: %w", err)
	}
	return items, nil
}

// ListUsable returns the assets usable now (available and inside their
// validity window), oldest first so a frozen plan input is stable.
func (r *EquipmentAssetRepository) ListUsable(ctx context.Context, owner uuid.UUID, now time.Time) ([]model.EquipmentAsset, error) {
	items, err := r.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	usable := make([]model.EquipmentAsset, 0, len(items))
	for index := len(items) - 1; index >= 0; index-- {
		if items[index].UsableAt(now) {
			usable = append(usable, items[index])
		}
	}
	return usable, nil
}
