package repository

import (
	"context"
	"encoding/json"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UploadRepository handles database operations for user uploads.
type UploadRepository struct {
	db *gorm.DB
}

// NewUploadRepository creates a new UploadRepository.
func NewUploadRepository(db *gorm.DB) *UploadRepository {
	return &UploadRepository{db: db}
}

// Create creates a new upload record.
func (r *UploadRepository) Create(ctx context.Context, upload *model.UserUpload) error {
	return r.db.WithContext(ctx).Create(upload).Error
}

// GetByID retrieves an upload by ID.
// Returns nil if not found (not an error).
func (r *UploadRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.UserUpload, error) {
	var upload model.UserUpload
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&upload).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &upload, nil
}

// GetByUserID retrieves all uploads for a user, ordered by creation time descending.
func (r *UploadRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error) {
	var uploads []model.UserUpload
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&uploads).Error
	if err != nil {
		return nil, err
	}
	return uploads, nil
}

// Delete removes an upload record by ID with ownership check.
func (r *UploadRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.UserUpload{}).Error
}

// UpdateOCRResult updates the OCR result and status for an upload with ownership check.
func (r *UploadRepository) UpdateOCRResult(ctx context.Context, id, userID uuid.UUID, status string, result json.RawMessage) error {
	return r.db.WithContext(ctx).
		Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"ocr_status": status,
			"ocr_result": result,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

// UpdateOCRStatus updates only the OCR status for an upload with ownership check.
func (r *UploadRepository) UpdateOCRStatus(ctx context.Context, id, userID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"ocr_status": status,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

// UpdateAnalysisStatus updates only the posture-analysis status for an upload
// with ownership check.
func (r *UploadRepository) UpdateAnalysisStatus(ctx context.Context, id, userID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"analysis_status": status,
			"updated_at":      gorm.Expr("NOW()"),
		}).Error
}

// UpdateAnalysisResult updates the analysis result + status for an upload
// with ownership check.
func (r *UploadRepository) UpdateAnalysisResult(ctx context.Context, id, userID uuid.UUID, status string, result json.RawMessage) error {
	return r.db.WithContext(ctx).
		Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"analysis_status": status,
			"analysis_result": result,
			"updated_at":      gorm.Expr("NOW()"),
		}).Error
}

// UpdateAgentConfiguration persists the immutable Agent configuration used
// for the analysis of this upload.
func (r *UploadRepository) UpdateAgentConfiguration(ctx context.Context, id uuid.UUID, configurationID string) error {
	return r.db.WithContext(ctx).
		Model(&model.UserUpload{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"agent_configuration_id": configurationID,
			"updated_at":             gorm.Expr("NOW()"),
		}).Error
}

// GetLatestPostureAnalyses returns the user's completed three-view posture
// analyses (front/side/back), newest first. Used by the consultation Agent
// tool and profile summary. Returns only the caller's own rows.
func (r *UploadRepository) GetLatestPostureAnalyses(ctx context.Context, userID uuid.UUID) ([]model.UserUpload, error) {
	var uploads []model.UserUpload
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND file_type IN ? AND analysis_status = ?",
			userID, []string{"photo_front", "photo_side", "photo_back"}, "completed").
		Order("created_at DESC").
		Find(&uploads).Error
	if err != nil {
		return nil, err
	}
	return uploads, nil
}
