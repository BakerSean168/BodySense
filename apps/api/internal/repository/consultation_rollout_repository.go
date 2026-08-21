package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"gorm.io/gorm"
)

type ConsultationRolloutRepository struct {
	db *gorm.DB
}

func NewConsultationRolloutRepository(db *gorm.DB) *ConsultationRolloutRepository {
	return &ConsultationRolloutRepository{db: db}
}

func (r *ConsultationRolloutRepository) Create(
	ctx context.Context,
	observation *model.ConsultationRolloutObservation,
) error {
	return r.db.WithContext(ctx).Create(observation).Error
}

func (r *ConsultationRolloutRepository) ListByChallenger(
	ctx context.Context,
	challengerConfigurationID string,
	stage string,
	limit int,
) ([]model.ConsultationRolloutObservation, error) {
	if limit <= 0 {
		limit = 100
	}
	var observations []model.ConsultationRolloutObservation
	err := r.db.WithContext(ctx).
		Where("challenger_configuration_id = ? AND stage = ?", challengerConfigurationID, stage).
		Order("created_at DESC").
		Limit(limit).
		Find(&observations).Error
	return observations, err
}

func (r *ConsultationRolloutRepository) CountByChallenger(
	ctx context.Context,
	challengerConfigurationID string,
	stage string,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ConsultationRolloutObservation{}).
		Where("challenger_configuration_id = ? AND stage = ?", challengerConfigurationID, stage).
		Count(&count).Error
	return count, err
}
