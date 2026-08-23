package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KnowledgePublicationObservationRepository struct {
	db *gorm.DB
}

func NewKnowledgePublicationObservationRepository(db *gorm.DB) *KnowledgePublicationObservationRepository {
	return &KnowledgePublicationObservationRepository{db: db}
}

func (r *KnowledgePublicationObservationRepository) Create(
	ctx context.Context,
	observation *model.KnowledgePublicationObservation,
) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "publication_id"}, {Name: "observation_key"}},
			DoNothing: true,
		}).
		Create(observation).Error
}

func (r *KnowledgePublicationObservationRepository) ListByPublication(
	ctx context.Context,
	publicationID uuid.UUID,
	kind string,
	limit int,
) ([]model.KnowledgePublicationObservation, error) {
	if limit <= 0 {
		limit = 200
	}
	var observations []model.KnowledgePublicationObservation
	err := r.db.WithContext(ctx).
		Where("publication_id = ? AND observation_kind = ?", publicationID, kind).
		Order("created_at DESC").
		Limit(limit).
		Find(&observations).Error
	return observations, err
}
