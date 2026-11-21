package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	activeAssetJobStatuses = []string{
		string(model.JobStatusPending), string(model.JobStatusRunning), string(model.JobStatusWaitingUser),
	}
	activeDerivedRunStatuses = []string{string(model.DerivedRunPending), string(model.DerivedRunProcessing)}
)

// RecordOutcome stores one target's result. A verified target keeps its
// cumulative erased count; any other state reports what is still outstanding.
func (r *AssetErasureRepository) RecordOutcome(ctx context.Context, request *model.AssetErasureRequest, target model.AssetErasureTargetName, outcome model.AssetErasureOutcome) error {
	return recordAssetErasureOutcome(r.conn(ctx), r.now(), request.UserID, request.ID, target, outcome)
}

func recordAssetErasureOutcome(tx *gorm.DB, now time.Time, userID, requestID uuid.UUID, target model.AssetErasureTargetName, outcome model.AssetErasureOutcome) error {
	var code any
	var verifiedAt any
	outstanding := outcome.Outstanding
	switch outcome.State {
	case model.AssetErasureTargetVerified:
		verifiedAt, outstanding = now, 0
	case model.AssetErasureTargetPending:
	default:
		code = outcome.FailureCode
	}
	result := tx.Exec(`UPDATE asset_erasure_targets
SET state = ?, erased_count = erased_count + ?, outstanding_count = ?, failure_code = ?,
    attempt_count = attempt_count + 1, verified_at = ?, updated_at = ?
WHERE user_id = ? AND request_id = ? AND target = ?`,
		outcome.State, outcome.Erased, outstanding, code, verifiedAt, now, userID, requestID, target)
	if result.Error != nil {
		return fmt.Errorf("record asset erasure target %s: %w", target, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("record asset erasure target %s: target row missing", target)
	}
	return nil
}

// ExecuteDatabaseTarget erases (or, for families that need a decision or a
// later handler, inventories) one database-resident target of the asset and
// verifies it inside the same transaction.
func (r *AssetErasureRepository) ExecuteDatabaseTarget(ctx context.Context, request *model.AssetErasureRequest, target model.AssetErasureTargetName) (model.AssetErasureOutcome, error) {
	var outcome model.AssetErasureOutcome
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		switch target {
		case model.AssetErasureDerivedJobs:
			outcome, err = eraseAssetJobs(tx, request.UserID, request.AssetID)
		case model.AssetErasurePostureAnalyses:
			outcome, err = eraseAssetPostureRuns(tx, request.UserID, request.AssetID)
		case model.AssetErasureDiagnosisRecords:
			outcome, err = inventoryOnly(tx, countAssetPinningConditions, request, model.AssetErasureTargetBlocked, model.AssetErasureCodeDiagnosisReview)
		case model.AssetErasureDocumentExtractions:
			outcome, err = eraseAssetDocumentExtractions(tx, request.UserID, request.AssetID)
		case model.AssetErasureAssessmentReports:
			outcome, err = eraseAssetAssessmentReports(tx, request.UserID, request.AssetID)
		case model.AssetErasureEpisodicNotes:
			// Serialize the note source gate's FOR SHARE admissions with our
			// source row before deleting notes. Finalize locks it again.
			var locked []uuid.UUID
			if err = tx.Raw(`SELECT id FROM user_assets WHERE user_id = ? AND id = ? FOR UPDATE`,
				request.UserID, request.AssetID).Scan(&locked).Error; err != nil {
				return err
			}
			if len(locked) == 0 {
				return fmt.Errorf("asset erasure source not found")
			}
			outcome, err = eraseAssetEpisodicNotes(tx, request.UserID, request.AssetID)
		default:
			err = fmt.Errorf("target %s is not a database target", target)
		}
		return err
	})
	return outcome, err
}

// inventoryOnly verifies a family this slice may not mutate: absent copies
// verify; present copies stay outstanding in the given state.
func inventoryOnly(tx *gorm.DB, count assetCopyCounter, request *model.AssetErasureRequest, state model.AssetErasureTargetState, code string) (model.AssetErasureOutcome, error) {
	n, err := count(tx, request.UserID, request.AssetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if n == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, nil
	}
	return model.AssetErasureOutcome{State: state, Outstanding: n, FailureCode: code}, nil
}

func assetJobIDs(tx *gorm.DB, userID, assetID uuid.UUID) (all []uuid.UUID, active int, err error) {
	var rows []struct {
		ID     uuid.UUID
		Status string
	}
	if err = tx.Raw(`SELECT id, status FROM jobs WHERE user_id = ? AND input->>'asset_id' = ? FOR UPDATE`,
		userID, assetID.String()).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		all = append(all, row.ID)
		for _, s := range activeAssetJobStatuses {
			if row.Status == s {
				active++
			}
		}
	}
	return all, active, nil
}

// eraseAssetJobs deletes the asset's terminal jobs (results hold the full model
// output), their events (cascade) and AI output reviews. Live work is retried.
func eraseAssetJobs(tx *gorm.DB, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error) {
	ids, active, err := assetJobIDs(tx, userID, assetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if active > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: active, FailureCode: model.AssetErasureCodeDerivedWorkActive}, nil
	}
	if len(ids) == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, nil
	}
	reviews := tx.Exec(`DELETE FROM ai_output_reviews WHERE job_id IN ?`, ids)
	if reviews.Error != nil {
		return model.AssetErasureOutcome{}, reviews.Error
	}
	jobs := tx.Exec(`DELETE FROM jobs WHERE user_id = ? AND id IN ?`, userID, ids)
	if jobs.Error != nil {
		return model.AssetErasureOutcome{}, jobs.Error
	}
	remaining, _, err := assetJobIDs(tx, userID, assetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if len(remaining) > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: len(remaining), FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: int(reviews.RowsAffected + jobs.RowsAffected)}, nil
}

func eraseAssetPostureRuns(tx *gorm.DB, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error) {
	active, err := countRows(tx, `SELECT count(*) FROM posture_analysis_runs WHERE user_id = ? AND asset_id = ? AND status IN ?`, userID, assetID, activeDerivedRunStatuses)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if active > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: active, FailureCode: model.AssetErasureCodeDerivedWorkActive}, nil
	}
	deleted := tx.Exec(`DELETE FROM posture_analysis_runs WHERE user_id = ? AND asset_id = ?`, userID, assetID)
	if deleted.Error != nil {
		return model.AssetErasureOutcome{}, deleted.Error
	}
	return verifyCount(tx, int(deleted.RowsAffected), `SELECT count(*) FROM posture_analysis_runs WHERE user_id = ? AND asset_id = ?`, userID, assetID)
}

// eraseAssetDocumentExtractions erases extraction runs and indicator reviews
// once no adopted condition cites the asset (pinned evidence waits for the
// Diagnosis to be revised or the condition withdrawn). Rows nothing links to
// are deleted; rows a withdrawn condition still links to become tombstones
// (identity kept, content cleared), so no Diagnosis row is ever changed.
func eraseAssetDocumentExtractions(tx *gorm.DB, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error) {
	pinned, err := lockAssetForPin(tx, userID, assetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if pinned > 0 {
		held, err := countAssetDocumentCopies(tx, userID, assetID)
		if err != nil || held == 0 {
			return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, err
		}
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: held, FailureCode: model.AssetErasureCodeDiagnosisPinned}, nil
	}
	active, err := countRows(tx, `SELECT count(*) FROM document_extraction_runs WHERE user_id = ? AND asset_id = ? AND status IN ?`, userID, assetID, activeDerivedRunStatuses)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if active > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: active, FailureCode: model.AssetErasureCodeDerivedWorkActive}, nil
	}
	const linked = `EXISTS (SELECT 1 FROM condition_entry_evidence e
    WHERE e.user_id = r.user_id AND e.document_review_id = r.id)`
	tombstonedReviews := tx.Exec(`UPDATE document_indicator_reviews r
SET erased_at = now(), reviewed_payload = '{}', machine_candidate = '{}', source_refs = '[]', page_ref = '{}', note = ''
WHERE r.user_id = ? AND r.asset_id = ? AND r.erased_at IS NULL AND `+linked, userID, assetID)
	if tombstonedReviews.Error != nil {
		return model.AssetErasureOutcome{}, tombstonedReviews.Error
	}
	reviews := tx.Exec(`DELETE FROM document_indicator_reviews r WHERE r.user_id = ? AND r.asset_id = ? AND NOT `+linked, userID, assetID)
	if reviews.Error != nil {
		return model.AssetErasureOutcome{}, reviews.Error
	}
	const kept = `EXISTS (SELECT 1 FROM document_indicator_reviews r WHERE r.extraction_run_id = x.id)`
	tombstonedRuns := tx.Exec(`UPDATE document_extraction_runs x
SET erased_at = now(), result_payload = NULL, document_sha256 = NULL, result_sha256 = NULL, raw_text_sha256 = NULL,
    indicator_snapshot = '[]', source_summary = '{}', updated_at = now()
WHERE x.user_id = ? AND x.asset_id = ? AND x.erased_at IS NULL AND `+kept, userID, assetID)
	if tombstonedRuns.Error != nil {
		return model.AssetErasureOutcome{}, tombstonedRuns.Error
	}
	runs := tx.Exec(`DELETE FROM document_extraction_runs x WHERE x.user_id = ? AND x.asset_id = ? AND NOT `+kept, userID, assetID)
	if runs.Error != nil {
		return model.AssetErasureOutcome{}, runs.Error
	}
	held, err := countAssetDocumentCopies(tx, userID, assetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if held > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: held, FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	erased := tombstonedReviews.RowsAffected + reviews.RowsAffected + tombstonedRuns.RowsAffected + runs.RowsAffected
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: int(erased)}, nil
}

// eraseAssetAssessmentReports deletes the assessment reports whose frozen
// input or output names the asset, once no adopted condition cites it. A
// frozen replay input is never rewritten: the whole derived report goes.
func eraseAssetAssessmentReports(tx *gorm.DB, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error) {
	pinned, err := lockAssetForPin(tx, userID, assetID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	held, err := countAssetAssessmentRefs(tx, userID, assetID)
	if err != nil || held == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, err
	}
	if pinned > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: held, FailureCode: model.AssetErasureCodeAssessmentFrozen}, nil
	}
	deleted := tx.Exec(`DELETE FROM assessment_reports t WHERE t.user_id = ? AND strpos(to_jsonb(t)::text, ?) > 0`, userID, assetID.String())
	if deleted.Error != nil {
		return model.AssetErasureOutcome{}, deleted.Error
	}
	return verifyCount(tx, int(deleted.RowsAffected), `SELECT count(*) FROM assessment_reports t WHERE t.user_id = ? AND strpos(to_jsonb(t)::text, ?) > 0`, userID, assetID.String())
}

// eraseAssetEpisodicNotes deletes the full shadow note when even one
// explicitly cited upload is erased. This is under the asset FOR UPDATE lock
// in ExecuteDatabaseTarget, and verified before committing the receipt.
func eraseAssetEpisodicNotes(tx *gorm.DB, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error) {
	deleted := tx.Exec(`DELETE FROM episodic_notes
WHERE user_id = ? AND source_asset_ids @> ARRAY[?::uuid]`, userID, assetID)
	if deleted.Error != nil {
		return model.AssetErasureOutcome{}, deleted.Error
	}
	return verifyCount(tx, int(deleted.RowsAffected),
		`SELECT count(*) FROM episodic_notes WHERE user_id = ?
AND source_asset_ids @> ARRAY[?::uuid]`, userID, assetID)
}
