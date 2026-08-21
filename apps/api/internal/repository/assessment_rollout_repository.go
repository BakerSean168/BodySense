package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"gorm.io/gorm"
)

type AssessmentRolloutRepository struct {
	db *gorm.DB
}

func NewAssessmentRolloutRepository(db *gorm.DB) *AssessmentRolloutRepository {
	return &AssessmentRolloutRepository{db: db}
}

func (r *AssessmentRolloutRepository) Create(ctx context.Context, observation *model.AssessmentRolloutObservation) error {
	return r.db.WithContext(ctx).Create(observation).Error
}

func (r *AssessmentRolloutRepository) ListRecent(
	ctx context.Context,
	championID, challengerID, stage string,
	canaryBPS, limit int,
) ([]model.AssessmentRolloutObservation, error) {
	query := r.db.WithContext(ctx).
		Where("champion_configuration_id = ?", championID).
		Where("challenger_configuration_id = ?", challengerID).
		Where("stage = ?", stage)
	if canaryBPS > 0 {
		query = query.Where("canary_bps = ?", canaryBPS)
	}
	if limit <= 0 {
		limit = 100
	}
	var rows []model.AssessmentRolloutObservation
	err := query.Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
