package repository

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OutcomeRepository persists the user's longitudinal Outcomes. An Outcome is
// idempotent per (user, source_type, source_key) and is not tied to a plan.
type OutcomeRepository struct {
	db *gorm.DB
}

func NewOutcomeRepository(db *gorm.DB) *OutcomeRepository {
	return &OutcomeRepository{db: db}
}

func (r *OutcomeRepository) CreateOutcome(
	ctx context.Context,
	outcome *model.Outcome,
) (*model.Outcome, bool, error) {
	if outcome.ID == uuid.Nil {
		outcome.ID = uuid.New()
	}
	if outcome.CausalityLevel == "" {
		outcome.CausalityLevel = "association_only"
	}
	if outcome.OccurredAt.IsZero() {
		outcome.OccurredAt = time.Now().UTC()
	}
	if len(outcome.Value) == 0 {
		outcome.Value = datatypes.JSON(`{}`)
	}
	if len(outcome.Provenance) == 0 {
		outcome.Provenance = datatypes.JSON(`{}`)
	}
	result := database.FromContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "source_type"}, {Name: "source_key"}},
		DoNothing: true,
	}).Create(outcome)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return outcome, true, nil
	}
	var stored model.Outcome
	if err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND source_type = ? AND source_key = ?", outcome.UserID, outcome.SourceType, outcome.SourceKey).
		First(&stored).Error; err != nil {
		return nil, false, err
	}
	return &stored, false, nil
}

func (r *OutcomeRepository) UpdateOutcomeBodyStateRevision(
	ctx context.Context,
	outcomeID, userID uuid.UUID,
	revision int64,
) error {
	return database.FromContext(ctx, r.db).Model(&model.Outcome{}).
		Where("id = ? AND user_id = ?", outcomeID, userID).
		Update("body_state_revision", revision).Error
}

func (r *OutcomeRepository) ListOutcomes(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.Outcome, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var items []model.Outcome
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("occurred_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}
