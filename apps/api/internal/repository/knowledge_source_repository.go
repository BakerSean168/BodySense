package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KnowledgeSourceRepository struct {
	db *gorm.DB
}

func NewKnowledgeSourceRepository(db *gorm.DB) *KnowledgeSourceRepository {
	return &KnowledgeSourceRepository{db: db}
}

func (r *KnowledgeSourceRepository) Register(ctx context.Context, source *model.KnowledgeSource) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(source)
	return result.RowsAffected == 1, result.Error
}

func (r *KnowledgeSourceRepository) FindByKey(ctx context.Context, sourceKey string) (*model.KnowledgeSource, error) {
	var source model.KnowledgeSource
	if err := r.db.WithContext(ctx).Where("source_key = ?", sourceKey).First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *KnowledgeSourceRepository) List(ctx context.Context, limit int) ([]model.KnowledgeSource, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var sources []model.KnowledgeSource
	if err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}
