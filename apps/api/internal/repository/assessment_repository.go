package repository

import (
	"context"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssessmentRepository handles database operations for assessment reports.
type AssessmentRepository struct {
	db *gorm.DB
}

// NewAssessmentRepository creates a new AssessmentRepository.
func NewAssessmentRepository(db *gorm.DB) *AssessmentRepository {
	return &AssessmentRepository{db: db}
}

// Create creates a new assessment report.
func (r *AssessmentRepository) Create(ctx context.Context, report *model.AssessmentReport) error {
	return database.FromContext(ctx, r.db).Create(report).Error
}

// GetByID retrieves an assessment report by ID and user ID.
func (r *AssessmentRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.AssessmentReport, error) {
	var report model.AssessmentReport
	err := database.FromContext(ctx, r.db).
		Where("id = ? AND user_id = ?", id, userID).
		First(&report).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// ListByUserID retrieves assessment reports for a user, ordered by creation time desc.
func (r *AssessmentRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.AssessmentReport, int64, error) {
	var reports []model.AssessmentReport
	var total int64

	query := database.FromContext(ctx, r.db).Where("user_id = ?", userID)

	// Count total
	if err := query.Model(&model.AssessmentReport{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch with pagination
	err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&reports).Error

	if err != nil {
		return nil, 0, err
	}
	return reports, total, nil
}
