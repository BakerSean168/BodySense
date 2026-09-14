package repository

import (
	"context"
	"encoding/json"

	"github.com/bodysense/api/internal/database"
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
	return database.FromContext(ctx, r.db).Create(upload).Error
}

// GetByID retrieves an upload by ID.
// Returns nil if not found (not an error).
func (r *UploadRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.UserUpload, error) {
	var upload model.UserUpload
	err := database.FromContext(ctx, r.db).
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
	err := database.FromContext(ctx, r.db).
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
	return database.FromContext(ctx, r.db).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.UserUpload{}).Error
}

// BeginOCR performs exactly pending -> processing. Duplicate/stale workers
// cannot reopen a terminal upload.
func (r *UploadRepository) BeginOCR(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ? AND ocr_status = ?", id, userID, model.UploadOCRPending).
		Updates(map[string]any{
			"ocr_status": model.UploadOCRProcessing,
			"updated_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// CompleteOCR performs exactly processing -> completed and persists the result.
func (r *UploadRepository) CompleteOCR(ctx context.Context, id, userID uuid.UUID, resultJSON json.RawMessage) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ? AND ocr_status = ?", id, userID, model.UploadOCRProcessing).
		Updates(map[string]any{
			"ocr_status": model.UploadOCRCompleted,
			"ocr_result": resultJSON,
			"updated_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// FailOCR moves only a non-terminal OCR state to failed. Once completed or
// failed, late workers/timeouts cannot replace the terminal result.
func (r *UploadRepository) FailOCR(ctx context.Context, id, userID uuid.UUID, resultJSON json.RawMessage) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ? AND ocr_status IN ?", id, userID, []model.UploadOCRStatus{model.UploadOCRPending, model.UploadOCRProcessing}).
		Updates(map[string]any{
			"ocr_status": model.UploadOCRFailed,
			"ocr_result": resultJSON,
			"updated_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// BeginAnalysis performs exactly pending -> processing. The `none` state is a
// non-photo leaf and is intentionally not enterable from this worker path.
func (r *UploadRepository) BeginAnalysis(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ? AND analysis_status = ?", id, userID, model.UploadAnalysisPending).
		Updates(map[string]any{
			"analysis_status": model.UploadAnalysisProcessing,
			"updated_at":      gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// CompleteAnalysis performs exactly processing -> completed.
func (r *UploadRepository) CompleteAnalysis(ctx context.Context, id, userID uuid.UUID, resultJSON json.RawMessage) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ? AND analysis_status = ?", id, userID, model.UploadAnalysisProcessing).
		Updates(map[string]any{
			"analysis_status": model.UploadAnalysisCompleted,
			"analysis_result": resultJSON,
			"updated_at":      gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// FailAnalysis moves only pending/processing to failed. Terminal results are
// monotonic and cannot be overwritten by stale worker completion or timeout.
func (r *UploadRepository) FailAnalysis(ctx context.Context, id, userID uuid.UUID, resultJSON json.RawMessage) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ? AND analysis_status IN ?", id, userID, []model.UploadAnalysisStatus{model.UploadAnalysisPending, model.UploadAnalysisProcessing}).
		Updates(map[string]any{
			"analysis_status": model.UploadAnalysisFailed,
			"analysis_result": resultJSON,
			"updated_at":      gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// UpdateAgentConfiguration persists the immutable Agent configuration used
// for the analysis of this upload.
func (r *UploadRepository) UpdateAgentConfiguration(ctx context.Context, id uuid.UUID, configurationID string) error {
	return database.FromContext(ctx, r.db).
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
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND file_type IN ? AND analysis_status = ?",
			userID, []string{"photo_front", "photo_side", "photo_back"}, model.UploadAnalysisCompleted).
		Order("created_at DESC").
		Find(&uploads).Error
	if err != nil {
		return nil, err
	}
	return uploads, nil
}

// ListByStorageBackend returns a stable batch for one upload storage backend.
func (r *UploadRepository) ListByStorageBackend(ctx context.Context, backend string, after *uuid.UUID, limit int) ([]model.UserUpload, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := database.FromContext(ctx, r.db).Where("storage_backend = ?", backend)
	if after != nil {
		query = query.Where("id > ?", *after)
	}
	var uploads []model.UserUpload
	if err := query.Order("id ASC").Limit(limit).Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

// CompareAndSwapStorageBackend advances one manifest only when its source
// identity still matches the object that was copied and verified.
func (r *UploadRepository) CompareAndSwapStorageBackend(
	ctx context.Context, id, userID uuid.UUID, fromBackend, storageKey, toBackend string,
) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.UserUpload{}).
		Where("id = ? AND user_id = ? AND storage_backend = ? AND storage_key = ?", id, userID, fromBackend, storageKey).
		Updates(map[string]any{"storage_backend": toBackend, "updated_at": gorm.Expr("NOW()")})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
