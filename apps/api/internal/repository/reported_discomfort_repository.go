package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReportedDiscomfortRepository stores the onboarding discomfort self-report
// (one row per user). It never touches the BodyState ledger.
type ReportedDiscomfortRepository struct {
	db *gorm.DB
}

func NewReportedDiscomfortRepository(db *gorm.DB) *ReportedDiscomfortRepository {
	return &ReportedDiscomfortRepository{db: db}
}

// Replace stores the user's current answer, replacing any earlier one. It
// joins the caller's transaction when one is in the context.
func (r *ReportedDiscomfortRepository) Replace(ctx context.Context, userID uuid.UUID, codes []string, reportedAt time.Time) error {
	encoded, err := json.Marshal(codes)
	if err != nil {
		return fmt.Errorf("encode reported discomfort areas: %w", err)
	}
	row := model.ReportedDiscomfortAreas{
		UserID: userID, AreaCodes: encoded,
		Source: model.ReportedDiscomfortSourceOnboarding, ReportedAt: reportedAt.UTC(),
	}
	return database.FromContext(ctx, r.db).WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"area_codes", "source", "reported_at"}),
		}).
		Create(&row).Error
}

// Get returns the user's answer, or nil when they never answered the step.
func (r *ReportedDiscomfortRepository) Get(ctx context.Context, userID uuid.UUID) (*model.ReportedDiscomfortAreas, error) {
	var row model.ReportedDiscomfortAreas
	err := database.FromContext(ctx, r.db).WithContext(ctx).Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
