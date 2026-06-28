package repository

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

// KnowledgeRepository handles knowledge base database operations.
type KnowledgeRepository struct {
	db *gorm.DB
}

// NewKnowledgeRepository creates a new KnowledgeRepository.
func NewKnowledgeRepository(db *gorm.DB) *KnowledgeRepository {
	return &KnowledgeRepository{db: db}
}

// Create creates a new knowledge entry.
func (r *KnowledgeRepository) Create(ctx context.Context, entry *model.KnowledgeEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// CreateBatch creates multiple knowledge entries in a single transaction.
func (r *KnowledgeRepository) CreateBatch(ctx context.Context, entries []*model.KnowledgeEntry) error {
	return r.db.WithContext(ctx).CreateInBatches(entries, 100).Error
}

// FindByID finds a knowledge entry by ID.
func (r *KnowledgeRepository) FindByID(ctx context.Context, id int64) (*model.KnowledgeEntry, error) {
	var entry model.KnowledgeEntry
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// FindByCategory finds knowledge entries by category.
func (r *KnowledgeRepository) FindByCategory(ctx context.Context, category string, limit int) ([]*model.KnowledgeEntry, error) {
	var entries []*model.KnowledgeEntry
	err := r.db.WithContext(ctx).
		Where("category = ?", category).
		Order("created_at DESC").
		Limit(limit).
		Find(&entries).Error
	return entries, err
}

// SearchByEmbedding performs cosine similarity search using pgvector.
func (r *KnowledgeRepository) SearchByEmbedding(ctx context.Context, embedding pgvector.Vector, topK int) ([]*model.KnowledgeEntry, []float64, error) {
	type result struct {
		model.KnowledgeEntry
		Similarity float64 `gorm:"column:similarity"`
	}

	var results []result
	err := r.db.WithContext(ctx).
		Table("knowledge_entries").
		Select("*, 1 - (embedding <=> ?) as similarity", embedding).
		Where("embedding IS NOT NULL").
		Order(fmt.Sprintf("embedding <=> '%s'", embedding.String())).
		Limit(topK).
		Find(&results).Error

	if err != nil {
		return nil, nil, err
	}

	entries := make([]*model.KnowledgeEntry, len(results))
	similarities := make([]float64, len(results))
	for i, res := range results {
		entries[i] = &res.KnowledgeEntry
		similarities[i] = res.Similarity
	}

	return entries, similarities, nil
}

// UpdateEmbedding updates the embedding for a knowledge entry.
func (r *KnowledgeRepository) UpdateEmbedding(ctx context.Context, id int64, embedding pgvector.Vector) error {
	return r.db.WithContext(ctx).
		Model(&model.KnowledgeEntry{}).
		Where("id = ?", id).
		Update("embedding", embedding).Error
}

// Delete deletes a knowledge entry by ID.
func (r *KnowledgeRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.KnowledgeEntry{}, id).Error
}

// Count returns the total number of knowledge entries.
func (r *KnowledgeRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.KnowledgeEntry{}).Count(&count).Error
	return count, err
}

// CountByCategory returns the number of knowledge entries in a category.
func (r *KnowledgeRepository) CountByCategory(ctx context.Context, category string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.KnowledgeEntry{}).
		Where("category = ?", category).
		Count(&count).Error
	return count, err
}
