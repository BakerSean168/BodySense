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
