package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AIOutputReviewRepository handles database operations for AI output reviews.
type AIOutputReviewRepository struct {
	db *gorm.DB
}

// NewAIOutputReviewRepository creates a new AIOutputReviewRepository.
func NewAIOutputReviewRepository(db *gorm.DB) *AIOutputReviewRepository {
	return &AIOutputReviewRepository{db: db}
}

// Create creates a new review record.
func (r *AIOutputReviewRepository) Create(ctx context.Context, review *model.AIOutputReview) error {
	return r.db.WithContext(ctx).Create(review).Error
}

// ListByRunID retrieves all reviews for a run.
func (r *AIOutputReviewRepository) ListByRunID(ctx context.Context, runID uuid.UUID) ([]model.AIOutputReview, error) {
	var reviews []model.AIOutputReview
	err := r.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&reviews).Error
	return reviews, err
}

// ListByConversationID retrieves all reviews for a conversation.
func (r *AIOutputReviewRepository) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.AIOutputReview, error) {
	var reviews []model.AIOutputReview
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&reviews).Error
	return reviews, err
}
