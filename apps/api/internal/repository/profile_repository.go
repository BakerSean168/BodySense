package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProfileRepository handles database operations for user profiles.
type ProfileRepository struct {
	db *gorm.DB
}

// NewProfileRepository creates a new ProfileRepository.
func NewProfileRepository(db *gorm.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// GetByUserID retrieves a user profile by user ID.
// Returns nil if no profile exists (not an error).
func (r *ProfileRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error) {
	var profile model.UserProfile
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&profile).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// CreateOrUpdate creates a new profile or updates an existing one.
// Uses PostgreSQL UPSERT (ON CONFLICT) for atomic operation.
func (r *ProfileRepository) CreateOrUpdate(ctx context.Context, profile *model.UserProfile) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"gender",
				"age",
				"height_cm",
				"weight_kg",
				"bmi",
				"occupation",
				"sleep_time",
				"wake_time",
				"exercise_type",
				"exercise_frequency",
				"injury_history",
				"self_description",
				"updated_at",
			}),
		}).
		Create(profile).Error
}
