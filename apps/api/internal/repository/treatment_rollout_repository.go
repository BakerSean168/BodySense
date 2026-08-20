package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"gorm.io/gorm"
)

type TreatmentRolloutRepository struct{ db *gorm.DB }

func NewTreatmentRolloutRepository(db *gorm.DB) *TreatmentRolloutRepository {
	return &TreatmentRolloutRepository{db: db}
}

func (r *TreatmentRolloutRepository) Create(ctx context.Context, observation *model.TreatmentRolloutObservation) error {
	return r.db.WithContext(ctx).Create(observation).Error
}

func (r *TreatmentRolloutRepository) ListRecent(
	ctx context.Context,
	championID string,
	challengerID string,
	stage string,
	canaryBPS int,
	limit int,
) ([]model.TreatmentRolloutObservation, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	var items []model.TreatmentRolloutObservation
	err := r.db.WithContext(ctx).
		Where("champion_configuration_id = ? AND challenger_configuration_id = ? AND stage = ? AND canary_bps = ?", championID, challengerID, stage, canaryBPS).
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}
