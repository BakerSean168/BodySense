package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentIndicatorReviewRepository handles append-only persistence of
// document indicator reviews. It exposes no update or delete mutation so
// review history cannot be rewritten after the fact.
type DocumentIndicatorReviewRepository struct{ db *gorm.DB }

// NewDocumentIndicatorReviewRepository creates a new repository.
func NewDocumentIndicatorReviewRepository(db *gorm.DB) *DocumentIndicatorReviewRepository {
	return &DocumentIndicatorReviewRepository{db: db}
}

// Create appends a new review row. The idempotency unique index is the only
// guard against duplicate submission of the same request.
func (r *DocumentIndicatorReviewRepository) Create(ctx context.Context, review *model.DocumentIndicatorReview) error {
	return r.db.WithContext(ctx).Create(review).Error
}

// ByOwnerScope returns any review rows for the owner's run+indicator scope
// with the given idempotency key, used by the service to detect duplicate
// re-submission of the same request.
func (r *DocumentIndicatorReviewRepository) ByOwnerScope(
	ctx context.Context,
	userID uuid.UUID,
	extractionRunID uuid.UUID,
	indicatorIndex int,
	idempotencyKey string,
) ([]model.DocumentIndicatorReview, error) {
	var rows []model.DocumentIndicatorReview
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND extraction_run_id = ? AND indicator_index = ? AND idempotency_key = ?",
			userID, extractionRunID, indicatorIndex, idempotencyKey).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

// ListByUpload returns all review rows for a given upload owned by userId,
// ordered ascending (append-only replay order).
func (r *DocumentIndicatorReviewRepository) ListByUpload(
	ctx context.Context,
	uploadID uuid.UUID,
	userID uuid.UUID,
) ([]model.DocumentIndicatorReview, error) {
	var rows []model.DocumentIndicatorReview
	err := r.db.WithContext(ctx).
		Where("upload_id = ? AND user_id = ?", uploadID, userID).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

// ListByExtractionRun returns all review rows for a single extraction run
// ordered ascending, used to project the effective latest review per candidate.
func (r *DocumentIndicatorReviewRepository) ListByExtractionRun(
	ctx context.Context,
	extractionRunID uuid.UUID,
) ([]model.DocumentIndicatorReview, error) {
	var rows []model.DocumentIndicatorReview
	err := r.db.WithContext(ctx).
		Where("extraction_run_id = ?", extractionRunID).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}
