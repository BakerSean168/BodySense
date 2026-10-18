package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// assetErasureDiagnosisTables are the Diagnosis audit and projection tables an
// asset or one of its indicator reviews can be cited from (FK or JSON ref).
var assetErasureDiagnosisTables = []string{
	"condition_analysis_runs", "condition_analysis_events", "diagnosis_proposals",
	"diagnosis_decisions", "diagnosis_settlements", "diagnosis_revisions",
	"condition_entries", "condition_entry_evidence", "condition_relations",
	"condition_disposition_changes", "condition_direct_edits",
}

var (
	activeAssetJobStatuses = []string{
		string(model.JobStatusPending), string(model.JobStatusRunning), string(model.JobStatusWaitingUser),
	}
	activeDerivedRunStatuses = []string{string(model.DerivedRunPending), string(model.DerivedRunProcessing)}
)

// AssetErasureRepository owns the per-asset erasure request, its target
// receipts and the database-resident controlled copies of one asset. Object
// storage and runtime stores are executed by the service through their ports.
type AssetErasureRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewAssetErasureRepository(db *gorm.DB) *AssetErasureRepository {
	return &AssetErasureRepository{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (r *AssetErasureRepository) conn(ctx context.Context) *gorm.DB {
	return database.FromContext(ctx, r.db)
}

// CreateOrGet admits one erasure per owned asset. A repeated command for the
// same asset returns the existing request whatever its key; a key already
// used for another asset is rejected. Missing and foreign assets are absent.
func (r *AssetErasureRepository) CreateOrGet(ctx context.Context, userID, assetID uuid.UUID, idempotencyKey string) (*model.AssetErasureReceipt, error) {
	var requestID uuid.UUID
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var asset model.UserAsset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, assetID).Take(&asset).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrAssetErasureNotFound
			}
			return err
		}
		var byKey model.AssetErasureRequest
		err := tx.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).Take(&byKey).Error
		if err == nil && byKey.AssetID != assetID {
			return model.ErrAssetErasureIdempotencyUsed
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var existing model.AssetErasureRequest
		err = tx.Where("user_id = ? AND asset_id = ?", userID, assetID).Take(&existing).Error
		if err == nil {
			requestID = existing.ID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := r.now()
		request := model.AssetErasureRequest{
			ID: uuid.New(), UserID: userID, AssetID: assetID, IdempotencyKey: idempotencyKey,
			InventoryRevision: model.AssetErasureInventoryRevision, Status: model.AssetErasurePending,
			RequestedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		targets := make([]model.AssetErasureTarget, 0, len(model.AssetErasureTargets))
		for _, name := range model.AssetErasureTargets {
			targets = append(targets, model.AssetErasureTarget{
				UserID: userID, RequestID: request.ID, Target: name,
				State: model.AssetErasureTargetPending, UpdatedAt: now,
			})
		}
		if err := tx.Create(&targets).Error; err != nil {
			return err
		}
		requestID = request.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.Receipt(ctx, userID, requestID)
}

// GetByAsset returns the owner's receipt for one asset.
func (r *AssetErasureRepository) GetByAsset(ctx context.Context, userID, assetID uuid.UUID) (*model.AssetErasureReceipt, error) {
	var request model.AssetErasureRequest
	if err := r.conn(ctx).Where("user_id = ? AND asset_id = ?", userID, assetID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrAssetErasureNotFound
		}
		return nil, err
	}
	return r.Receipt(ctx, userID, request.ID)
}

// Receipt loads one request with its targets in inventory order.
func (r *AssetErasureRepository) Receipt(ctx context.Context, userID, requestID uuid.UUID) (*model.AssetErasureReceipt, error) {
	var request model.AssetErasureRequest
	if err := r.conn(ctx).Where("user_id = ? AND id = ?", userID, requestID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrAssetErasureNotFound
		}
		return nil, err
	}
	var rows []model.AssetErasureTarget
	if err := r.conn(ctx).Where("user_id = ? AND request_id = ?", userID, requestID).Find(&rows).Error; err != nil {
		return nil, err
	}
	byName := make(map[model.AssetErasureTargetName]model.AssetErasureTarget, len(rows))
	for _, row := range rows {
		byName[row.Target] = row
	}
	receipt := &model.AssetErasureReceipt{Request: request, Targets: make([]model.AssetErasureTarget, 0, len(rows))}
	for _, name := range model.AssetErasureTargets {
		if row, ok := byName[name]; ok {
			receipt.Targets = append(receipt.Targets, row)
		}
	}
	return receipt, nil
}

// Claim leases a request for one execution pass. Erased requests and live
// leases are never claimed, so two workers never execute the same request.
func (r *AssetErasureRepository) Claim(ctx context.Context, requestID uuid.UUID, owner string, lease time.Duration) (*model.AssetErasureRequest, bool, error) {
	now := r.now()
	var request model.AssetErasureRequest
	result := r.conn(ctx).Raw(`
UPDATE asset_erasure_requests
SET status = 'running', attempt_count = attempt_count + 1,
    lease_owner = ?, lease_expires_at = ?, updated_at = ?
WHERE id = ?
  AND (status IN ('pending','outstanding') OR (status = 'running' AND lease_expires_at < ?))
RETURNING *`, owner, now.Add(lease), now, requestID, now).Scan(&request)
	if result.Error != nil {
		return nil, false, fmt.Errorf("claim asset erasure: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &request, true, nil
}

// Release ends a pass that did not erase everything.
func (r *AssetErasureRepository) Release(ctx context.Context, request *model.AssetErasureRequest) error {
	return r.conn(ctx).Exec(`UPDATE asset_erasure_requests
SET status = 'outstanding', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE user_id = ? AND id = ? AND status = 'running' AND lease_owner = ?`,
		r.now(), request.UserID, request.ID, *request.LeaseOwner).Error
}

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
			outcome, err = inventoryOnly(tx, countAssetDiagnosisRefs, request, model.AssetErasureTargetBlocked, model.AssetErasureCodeDiagnosisReview)
		case model.AssetErasureDocumentExtractions:
			outcome, err = eraseAssetDocumentExtractions(tx, request.UserID, request.AssetID)
		case model.AssetErasureAssessmentReports:
			outcome, err = inventoryOnly(tx, countAssetAssessmentRefs, request, model.AssetErasureTargetBlocked, model.AssetErasureCodeAssessmentFrozen)
		case model.AssetErasureRuntimeCheckpoints:
			outcome, err = inventoryOnly(tx, countAssetRuntimeRefs, request, model.AssetErasureTargetUnsupported, model.AssetErasureCodeRuntimeHookPending)
		default:
			err = fmt.Errorf("target %s is not a database target", target)
		}
		return err
	})
	return outcome, err
}

type assetCopyCounter func(tx *gorm.DB, userID, assetID uuid.UUID) (int, error)

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

func countRows(tx *gorm.DB, query string, args ...any) (int, error) {
	var n int64
	if err := tx.Raw(query, args...).Scan(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
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

// eraseAssetDocumentExtractions deletes extraction runs and indicator reviews
// only when no Diagnosis cites the asset: cited evidence waits for review.
func eraseAssetDocumentExtractions(tx *gorm.DB, userID, assetID uuid.UUID) (model.AssetErasureOutcome, error) {
	if err := tx.Exec(`SELECT 1 FROM user_assets WHERE user_id = ? AND id = ? FOR UPDATE`, userID, assetID).Error; err != nil {
		return model.AssetErasureOutcome{}, err
	}
	pinned, err := countAssetDiagnosisRefs(tx, userID, assetID)
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
	reviews := tx.Exec(`DELETE FROM document_indicator_reviews WHERE user_id = ? AND asset_id = ?`, userID, assetID)
	if reviews.Error != nil {
		return model.AssetErasureOutcome{}, reviews.Error
	}
	runs := tx.Exec(`DELETE FROM document_extraction_runs WHERE user_id = ? AND asset_id = ?`, userID, assetID)
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
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: int(reviews.RowsAffected + runs.RowsAffected)}, nil
}

func verifyCount(tx *gorm.DB, erased int, query string, args ...any) (model.AssetErasureOutcome, error) {
	held, err := countRows(tx, query, args...)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if held > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: held, FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: erased}, nil
}

func countAssetDocumentCopies(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	return countRows(tx, `SELECT (SELECT count(*) FROM document_extraction_runs WHERE user_id = ? AND asset_id = ?)
 + (SELECT count(*) FROM document_indicator_reviews WHERE user_id = ? AND asset_id = ?)`, userID, assetID, userID, assetID)
}

// countAssetDiagnosisRefs counts Diagnosis rows that cite the asset or one of
// its indicator reviews, by FK column or anywhere inside their JSON.
func countAssetDiagnosisRefs(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	var reviewIDs []string
	if err := tx.Raw(`SELECT id::text FROM document_indicator_reviews WHERE user_id = ? AND asset_id = ?`, userID, assetID).Scan(&reviewIDs).Error; err != nil {
		return 0, err
	}
	needles := append([]string{assetID.String()}, reviewIDs...)
	total := 0
	for _, table := range assetErasureDiagnosisTables {
		n, err := countRowsMentioning(tx, table, userID, needles)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func countAssetAssessmentRefs(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	return countRowsMentioning(tx, "assessment_reports", userID, []string{assetID.String()})
}

// countAssetRuntimeRefs counts conversations a message attached the asset to.
// Their runtime checkpoints may hold the inline image the runtime received.
func countAssetRuntimeRefs(tx *gorm.DB, userID, assetID uuid.UUID) (int, error) {
	return countRows(tx, `SELECT count(DISTINCT m.conversation_id) FROM messages m
 JOIN conversations c ON c.id = m.conversation_id
 WHERE c.user_id = ? AND m.parts @> jsonb_build_array(jsonb_build_object('upload_id', ?::text))`, userID, assetID.String())
}

// countRowsMentioning matches the whole owner-scoped row as JSON text, so a
// reference nested anywhere (FK column, evidence ref, frozen replay input) counts.
func countRowsMentioning(tx *gorm.DB, table string, userID uuid.UUID, needles []string) (int, error) {
	if len(needles) == 0 {
		return 0, nil
	}
	clauses := make([]string, 0, len(needles))
	args := []any{userID}
	for _, needle := range needles {
		clauses = append(clauses, "strpos(to_jsonb(t)::text, ?) > 0")
		args = append(args, needle)
	}
	query := fmt.Sprintf(`SELECT count(*) FROM %s t WHERE t.user_id = ? AND (%s)`, table, strings.Join(clauses, " OR "))
	return countRows(tx, query, args...)
}

// Finalize tombstones the asset and marks the request erased in one
// transaction, only when every other target verified and a re-check of every
// database family still finds no copy. A copy that reappeared reopens its
// target instead. It returns whether the request is erased.
func (r *AssetErasureRepository) Finalize(ctx context.Context, request *model.AssetErasureRequest) (bool, error) {
	erased := false
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.AssetErasureRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", request.UserID, request.ID).Take(&current).Error; err != nil {
			return err
		}
		if current.Status == model.AssetErasureErased {
			erased = true
			return nil
		}
		var asset model.UserAsset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", request.UserID, request.AssetID).Take(&asset).Error; err != nil {
			return err
		}
		var unverified int64
		if err := tx.Model(&model.AssetErasureTarget{}).
			Where("user_id = ? AND request_id = ? AND target <> ? AND state <> ?", request.UserID, request.ID, model.AssetErasureManifest, model.AssetErasureTargetVerified).
			Count(&unverified).Error; err != nil {
			return err
		}
		if unverified > 0 || asset.Availability == model.UserAssetAvailable {
			return recordAssetErasureOutcome(tx, r.now(), request.UserID, request.ID, model.AssetErasureManifest,
				model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: 1, FailureCode: model.AssetErasureCodeTargetsOutstanding})
		}
		reappeared, err := recheckAssetCopies(tx, request.UserID, request.AssetID)
		if err != nil {
			return err
		}
		now := r.now()
		if len(reappeared) > 0 {
			for target, n := range reappeared {
				if err := recordAssetErasureOutcome(tx, now, request.UserID, request.ID, target,
					model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: n, FailureCode: model.AssetErasureCodeCopyReappeared}); err != nil {
					return err
				}
			}
			return recordAssetErasureOutcome(tx, now, request.UserID, request.ID, model.AssetErasureManifest,
				model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: 1, FailureCode: model.AssetErasureCodeTargetsOutstanding})
		}
		if err := tx.Exec(`UPDATE user_assets SET availability = 'erased', erased_at = ?, original_name = '', updated_at = ?
WHERE user_id = ? AND id = ?`, now, now, request.UserID, request.AssetID).Error; err != nil {
			return err
		}
		if err := recordAssetErasureOutcome(tx, now, request.UserID, request.ID, model.AssetErasureManifest,
			model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: 1}); err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE asset_erasure_requests
SET status = 'erased', erased_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE user_id = ? AND id = ?`, now, now, request.UserID, request.ID).Error; err != nil {
			return err
		}
		erased = true
		return nil
	})
	return erased, err
}

// recheckAssetCopies re-counts every database family under the asset row
// lock; composite-FK inserts against the asset wait for this transaction.
func recheckAssetCopies(tx *gorm.DB, userID, assetID uuid.UUID) (map[model.AssetErasureTargetName]int, error) {
	found := map[model.AssetErasureTargetName]int{}
	jobs, _, err := assetJobIDs(tx, userID, assetID)
	if err != nil {
		return nil, err
	}
	if len(jobs) > 0 {
		found[model.AssetErasureDerivedJobs] = len(jobs)
	}
	checks := []struct {
		target model.AssetErasureTargetName
		count  assetCopyCounter
	}{
		{model.AssetErasureSourceObjects, func(tx *gorm.DB, u, a uuid.UUID) (int, error) {
			return countRows(tx, `SELECT count(*) FROM user_asset_storage_cleanups WHERE user_id = ? AND asset_id = ?`, u, a)
		}},
		{model.AssetErasurePostureAnalyses, func(tx *gorm.DB, u, a uuid.UUID) (int, error) {
			return countRows(tx, `SELECT count(*) FROM posture_analysis_runs WHERE user_id = ? AND asset_id = ?`, u, a)
		}},
		{model.AssetErasureDiagnosisRecords, countAssetDiagnosisRefs},
		{model.AssetErasureDocumentExtractions, countAssetDocumentCopies},
		{model.AssetErasureAssessmentReports, countAssetAssessmentRefs},
		{model.AssetErasureRuntimeCheckpoints, countAssetRuntimeRefs},
	}
	for _, check := range checks {
		n, err := check.count(tx, userID, assetID)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			found[check.target] = n
		}
	}
	return found, nil
}

// ListRecoverable returns requests a worker should run: new ones, expired
// leases, and outstanding ones with a transient failure older than retryAfter.
// Blocked and unsupported targets wait for a decision, a handler or an
// explicit repeat of the command.
func (r *AssetErasureRepository) ListRecoverable(ctx context.Context, retryAfter time.Duration, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 10
	}
	now := r.now()
	var ids []uuid.UUID
	err := r.conn(ctx).Raw(`SELECT q.id FROM asset_erasure_requests q
WHERE q.status = 'pending'
   OR (q.status = 'running' AND q.lease_expires_at < ?)
   OR (q.status = 'outstanding' AND q.updated_at < ? AND EXISTS (
        SELECT 1 FROM asset_erasure_targets t
        WHERE t.user_id = q.user_id AND t.request_id = q.id AND t.state IN ('pending','failed')))
ORDER BY q.updated_at ASC
LIMIT ?`, now, now.Add(-retryAfter), limit).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("list recoverable asset erasures: %w", err)
	}
	return ids, nil
}

// GetRequest loads a request by id for the worker.
func (r *AssetErasureRepository) GetRequest(ctx context.Context, requestID uuid.UUID) (*model.AssetErasureRequest, error) {
	var request model.AssetErasureRequest
	if err := r.conn(ctx).Where("id = ?", requestID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrAssetErasureNotFound
		}
		return nil, err
	}
	return &request, nil
}
