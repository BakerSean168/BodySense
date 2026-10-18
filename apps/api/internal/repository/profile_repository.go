package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProfileRepository persists the minimal optional Profile (PC-D1) only.
type ProfileRepository struct {
	db *gorm.DB
}

func NewProfileRepository(db *gorm.DB) *ProfileRepository { return &ProfileRepository{db: db} }

// GetByUserID returns the owner's profile, or nil before one exists. An avatar
// whose asset is no longer an available photo of this owner (source removed,
// erased) is projected as absent; the stored pointer is left untouched.
func (r *ProfileRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error) {
	var profile model.UserProfile
	tx := database.FromContext(ctx, r.db).WithContext(ctx)
	err := tx.Where("user_id = ?", userID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if profile.AvatarAssetID != nil {
		usable, err := profileAvatarUsable(tx, userID, *profile.AvatarAssetID)
		if err != nil {
			return nil, err
		}
		if !usable {
			profile.AvatarAssetID = nil
		}
	}
	profile.ProjectPreferences()
	return &profile, nil
}

// CreateOrUpdate records the onboarding identity answers (gender, birth date)
// only. Display name, avatar and preferences are preserved, and the revision
// advances so a concurrently open editor sees the change as a conflict.
func (r *ProfileRepository) CreateOrUpdate(ctx context.Context, profile *model.UserProfile) error {
	now := time.Now().UTC()
	return database.FromContext(ctx, r.db).WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Set{
				{Column: clause.Column{Name: "gender"}, Value: profile.Gender},
				{Column: clause.Column{Name: "birth_date"}, Value: profile.BirthDate},
				{Column: clause.Column{Name: "revision"}, Value: gorm.Expr("user_profiles.revision + 1")},
				{Column: clause.Column{Name: "updated_at"}, Value: now},
			},
		}).
		Select("user_id", "gender", "birth_date", "revision", "created_at", "updated_at").
		Create(&model.UserProfile{
			UserID: profile.UserID, Gender: profile.Gender, BirthDate: profile.BirthDate,
			Revision: 1, CreatedAt: now, UpdatedAt: now,
		}).Error
}

// Replace replaces every editable field under the owner's row lock. Expected
// revision 0 creates the profile; an exact replay of the update that produced
// the current revision returns the stored profile unchanged.
func (r *ProfileRepository) Replace(ctx context.Context, userID uuid.UUID, command model.ReplaceProfileCommand) (*model.UserProfile, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: owner is required", model.ErrInvalidProfile)
	}
	err := database.FromContext(ctx, r.db).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize first creation as well as updates for this owner.
		if err := tx.Exec("SELECT id FROM users WHERE id = ? FOR UPDATE", userID).Error; err != nil {
			return err
		}
		var current model.UserProfile
		found := true
		if err := tx.Where("user_id = ?", userID).Take(&current).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			found = false
		}
		currentRevision := int64(0)
		if found {
			currentRevision = current.Revision
		}
		if currentRevision != command.ExpectedRevision {
			if found && currentRevision == command.ExpectedRevision+1 && current.SameFields(command.ProfileFields) {
				return nil
			}
			return fmt.Errorf("%w: expected revision %d, current %d",
				model.ErrProfileRevisionConflict, command.ExpectedRevision, currentRevision)
		}
		// Setting or keeping an avatar always re-checks ownership and
		// availability, so a removed source cannot be re-attached.
		if command.AvatarAssetID != nil {
			usable, err := profileAvatarUsable(tx, userID, *command.AvatarAssetID)
			if err != nil {
				return err
			}
			if !usable {
				return model.ErrProfileAvatarUnavailable
			}
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		values := map[string]any{
			"display_name": command.DisplayName, "avatar_asset_id": command.AvatarAssetID,
			"gender": command.Gender, "birth_date": command.BirthDate,
			"locale": command.Locale, "time_zone": command.TimeZone,
			"unit_preference": command.UnitPreference,
			"revision":        currentRevision + 1, "updated_at": now,
		}
		if !found {
			values["user_id"] = userID
			values["created_at"] = now
			return tx.Model(&model.UserProfile{}).Create(values).Error
		}
		result := tx.Model(&model.UserProfile{}).
			Where("user_id = ? AND revision = ?", userID, currentRevision).
			Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrProfileRevisionConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, userID)
}

func profileAvatarUsable(tx *gorm.DB, userID, assetID uuid.UUID) (bool, error) {
	var count int64
	err := tx.Table("user_assets").
		Where("user_id = ? AND id = ? AND kind = ? AND availability = ?", userID, assetID, "photo", "available").
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check profile avatar asset: %w", err)
	}
	return count == 1, nil
}
