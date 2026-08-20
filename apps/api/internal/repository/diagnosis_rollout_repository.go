package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"gorm.io/gorm"
)

type DiagnosisRolloutRepository struct{ db *gorm.DB }

func NewDiagnosisRolloutRepository(db *gorm.DB) *DiagnosisRolloutRepository {
	return &DiagnosisRolloutRepository{db: db}
}

func (r *DiagnosisRolloutRepository) Create(ctx context.Context, observation *model.DiagnosisRolloutObservation) error {
	return r.db.WithContext(ctx).Create(observation).Error
}

func (r *DiagnosisRolloutRepository) ListRecent(
	ctx context.Context,
	championID string,
	challengerID string,
	stage string,
	canaryBPS int,
	limit int,
) ([]model.DiagnosisRolloutObservation, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	var items []model.DiagnosisRolloutObservation
	err := r.db.WithContext(ctx).
		Where("champion_configuration_id = ? AND challenger_configuration_id = ? AND stage = ? AND canary_bps = ?", championID, challengerID, stage, canaryBPS).
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}
