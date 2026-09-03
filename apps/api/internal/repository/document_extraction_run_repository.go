package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DocumentExtractionRunRepository struct{ db *gorm.DB }

func NewDocumentExtractionRunRepository(db *gorm.DB) *DocumentExtractionRunRepository {
	return &DocumentExtractionRunRepository{db: db}
}

func (r *DocumentExtractionRunRepository) Create(ctx context.Context, run *model.DocumentExtractionRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// GetLatestOwnedByUpload resolves the server-owned current extraction run for
// an upload. Extraction runs are append-only and are persisted only after a
// successful validated extraction, so newest-created is the exact run backing
// the current OCR projection. Ownership is part of the query to avoid probing.
func (r *DocumentExtractionRunRepository) GetLatestOwnedByUpload(
	ctx context.Context,
	uploadID uuid.UUID,
	userID uuid.UUID,
) (*model.DocumentExtractionRun, error) {
	var run model.DocumentExtractionRun
	err := r.db.WithContext(ctx).
		Where("upload_id = ? AND user_id = ?", uploadID, userID).
		Order("created_at DESC").
		Order("id DESC").
		First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *DocumentExtractionRunRepository) ListByUpload(
	ctx context.Context,
	uploadID uuid.UUID,
) ([]model.DocumentExtractionRun, error) {
	var runs []model.DocumentExtractionRun
	err := r.db.WithContext(ctx).
		Where("upload_id = ?", uploadID).
		Order("created_at DESC").
		Find(&runs).Error
	return runs, err
}

// GetOwnedByID retrieves a single extraction run while enforcing user
// ownership. Returns gorm.ErrRecordNotFound when the run does not exist or
// belongs to another user so callers cannot probe other users' runs.
func (r *DocumentExtractionRunRepository) GetOwnedByID(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*model.DocumentExtractionRun, error) {
	var run model.DocumentExtractionRun
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}
