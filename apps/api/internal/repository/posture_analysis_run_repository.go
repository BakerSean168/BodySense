package repository

import (
	"context"
	"encoding/json"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PostureAnalysisRunRepository persists the derived posture-analysis run of a
// posture photo asset. Exactly one run exists per (user, asset); the
// lifecycle is compare-and-set pending -> processing -> completed | failed so
// stale workers cannot reopen a terminal analysis.
type PostureAnalysisRunRepository struct{ db *gorm.DB }

func NewPostureAnalysisRunRepository(db *gorm.DB) *PostureAnalysisRunRepository {
	return &PostureAnalysisRunRepository{db: db}
}

func (r *PostureAnalysisRunRepository) Create(ctx context.Context, run *model.PostureAnalysisRun) error {
	return database.FromContext(ctx, r.db).Create(run).Error
}

// GetOwnedByAsset returns the single posture run of one asset in the caller's
// owner scope, or nil when it does not exist.
func (r *PostureAnalysisRunRepository) GetOwnedByAsset(
	ctx context.Context,
	assetID, userID uuid.UUID,
) (*model.PostureAnalysisRun, error) {
	var run model.PostureAnalysisRun
	err := database.FromContext(ctx, r.db).
		Where("asset_id = ? AND user_id = ?", assetID, userID).
		First(&run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListCompletedByUser returns the caller's completed posture runs, newest
// first. The service collapses them to the newest completed run per view.
func (r *PostureAnalysisRunRepository) ListCompletedByUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]model.PostureAnalysisRun, error) {
	var runs []model.PostureAnalysisRun
	err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND status = ?", userID, model.DerivedRunCompleted).
		Order("created_at DESC").
		Find(&runs).Error
	return runs, err
}

// ListByUser returns every posture run of one owner, newest first.
func (r *PostureAnalysisRunRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]model.PostureAnalysisRun, error) {
	var runs []model.PostureAnalysisRun
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&runs).Error
	return runs, err
}

// BeginProcessing performs exactly pending -> processing and pins the job.
func (r *PostureAnalysisRunRepository) BeginProcessing(
	ctx context.Context,
	runID, userID, jobID uuid.UUID,
) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.PostureAnalysisRun{}).
		Where("id = ? AND user_id = ? AND status = ?", runID, userID, model.DerivedRunPending).
		Updates(map[string]any{
			"status":     model.DerivedRunProcessing,
			"job_id":     jobID,
			"updated_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// CompleteRun performs exactly processing -> completed and persists the
// validated analysis payload plus the immutable Agent configuration identity
// that produced it.
func (r *PostureAnalysisRunRepository) CompleteRun(
	ctx context.Context,
	runID, userID uuid.UUID,
	agentConfigurationID string,
	payload json.RawMessage,
) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.PostureAnalysisRun{}).
		Where("id = ? AND user_id = ? AND status = ?", runID, userID, model.DerivedRunProcessing).
		Updates(map[string]any{
			"status":                 model.DerivedRunCompleted,
			"result_payload":         datatypes.JSON(payload),
			"agent_configuration_id": agentConfigurationID,
			"updated_at":             gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// FailRun moves a non-terminal run to failed with the error payload. Terminal
// results are monotonic and cannot be overwritten.
func (r *PostureAnalysisRunRepository) FailRun(
	ctx context.Context,
	runID, userID uuid.UUID,
	payload json.RawMessage,
) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.PostureAnalysisRun{}).
		Where("id = ? AND user_id = ? AND status IN ?", runID, userID,
			[]model.DerivedRunStatus{model.DerivedRunPending, model.DerivedRunProcessing}).
		Updates(map[string]any{
			"status":         model.DerivedRunFailed,
			"result_payload": datatypes.JSON(payload),
			"updated_at":     gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}
