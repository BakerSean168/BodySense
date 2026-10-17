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

// DocumentExtractionRunRepository persists the derived OCR/extraction run of
// one report asset, including the full pending -> processing ->
// completed | failed lifecycle. Transitions are compare-and-set so a stale or
// competing worker can never reopen a terminal run.
type DocumentExtractionRunRepository struct{ db *gorm.DB }

func NewDocumentExtractionRunRepository(db *gorm.DB) *DocumentExtractionRunRepository {
	return &DocumentExtractionRunRepository{db: db}
}

func (r *DocumentExtractionRunRepository) Create(ctx context.Context, run *model.DocumentExtractionRun) error {
	return database.FromContext(ctx, r.db).Create(run).Error
}

// GetLatestOwnedByAsset resolves the newest extraction run of one asset in
// the caller's owner scope. Newest-created wins so a future reprocess run can
// supersede an older one without rewriting it.
func (r *DocumentExtractionRunRepository) GetLatestOwnedByAsset(
	ctx context.Context,
	assetID uuid.UUID,
	userID uuid.UUID,
) (*model.DocumentExtractionRun, error) {
	var run model.DocumentExtractionRun
	err := database.FromContext(ctx, r.db).
		Where("asset_id = ? AND user_id = ?", assetID, userID).
		Order("created_at DESC").
		Order("id DESC").
		First(&run).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListByUser returns every extraction run of one owner, newest first.
func (r *DocumentExtractionRunRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]model.DocumentExtractionRun, error) {
	var runs []model.DocumentExtractionRun
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Order("id DESC").
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
	err := database.FromContext(ctx, r.db).
		Where("id = ? AND user_id = ?", id, userID).
		First(&run).Error
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// BeginProcessing performs exactly pending -> processing and pins the job
// that owns the execution. Duplicate or stale workers cannot restart a run.
func (r *DocumentExtractionRunRepository) BeginProcessing(
	ctx context.Context,
	runID, userID, jobID uuid.UUID,
) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.DocumentExtractionRun{}).
		Where("id = ? AND user_id = ? AND status = ?", runID, userID, model.DerivedRunPending).
		Updates(map[string]any{
			"status":     model.DerivedRunProcessing,
			"job_id":     jobID,
			"updated_at": gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// CompleteRun performs exactly processing -> completed and persists the full
// extraction audit: document/result/raw-text hashes, mechanism provenance,
// the durable indicator snapshot and source summary, and the validated
// response payload of this run.
func (r *DocumentExtractionRunRepository) CompleteRun(
	ctx context.Context,
	run *model.DocumentExtractionRun,
) (bool, error) {
	if run == nil || run.ID == uuid.Nil {
		return false, nil
	}
	result := database.FromContext(ctx, r.db).Model(&model.DocumentExtractionRun{}).
		Where("id = ? AND user_id = ? AND status = ?", run.ID, run.UserID, model.DerivedRunProcessing).
		Updates(map[string]any{
			"status":               model.DerivedRunCompleted,
			"mechanism_revision":   run.MechanismRevision,
			"document_sha256":      run.DocumentSHA256,
			"result_sha256":        run.ResultSHA256,
			"raw_text_sha256":      run.RawTextSHA256,
			"result_payload":       run.ResultPayload,
			"indicator_snapshot":   run.IndicatorSnapshot,
			"source_summary":       run.SourceSummary,
			"mechanism_provenance": run.MechanismProvenance,
			"updated_at":           gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}

// FailRun moves a non-terminal run to failed with the error payload. Once
// completed, late workers or timeouts cannot replace the terminal result.
func (r *DocumentExtractionRunRepository) FailRun(
	ctx context.Context,
	runID, userID uuid.UUID,
	payload json.RawMessage,
) (bool, error) {
	result := database.FromContext(ctx, r.db).Model(&model.DocumentExtractionRun{}).
		Where("id = ? AND user_id = ? AND status IN ?", runID, userID,
			[]model.DerivedRunStatus{model.DerivedRunPending, model.DerivedRunProcessing}).
		Updates(map[string]any{
			"status":         model.DerivedRunFailed,
			"result_payload": datatypes.JSON(payload),
			"updated_at":     gorm.Expr("NOW()"),
		})
	return result.RowsAffected == 1, result.Error
}
