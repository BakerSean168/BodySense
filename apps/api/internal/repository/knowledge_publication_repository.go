package repository

import (
	"context"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// KnowledgePublicationRepository handles database operations for knowledge publications.
type KnowledgePublicationRepository struct {
	db *gorm.DB
}

// NewKnowledgePublicationRepository creates a new KnowledgePublicationRepository.
func NewKnowledgePublicationRepository(db *gorm.DB) *KnowledgePublicationRepository {
	return &KnowledgePublicationRepository{db: db}
}

// Create creates a new publication record.
func (r *KnowledgePublicationRepository) Create(ctx context.Context, pub *model.KnowledgePublication) error {
	return database.FromContext(ctx, r.db).Create(pub).Error
}

// GetByID retrieves a publication by ID.
func (r *KnowledgePublicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.KnowledgePublication, error) {
	var pub model.KnowledgePublication
	err := database.FromContext(ctx, r.db).Where("id = ?", id).First(&pub).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pub, nil
}

// GetByKey retrieves a publication by immutable publication key.
func (r *KnowledgePublicationRepository) GetByKey(
	ctx context.Context,
	publicationKey string,
) (*model.KnowledgePublication, error) {
	var pub model.KnowledgePublication
	err := r.db.WithContext(ctx).Where("publication_key = ?", publicationKey).First(&pub).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pub, nil
}

// ListByStatus retrieves publications by status.
func (r *KnowledgePublicationRepository) ListByStatus(ctx context.Context, status string) ([]model.KnowledgePublication, error) {
	var pubs []model.KnowledgePublication
	err := database.FromContext(ctx, r.db).
		Where("status = ?", status).
		Order("created_at DESC").
		Find(&pubs).Error
	return pubs, err
}
