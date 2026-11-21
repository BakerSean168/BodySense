package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
		if err := tx.Exec(`UPDATE user_assets SET availability = 'erased', erased_at = ?, original_name = '', content_sha256 = NULL, updated_at = ?
WHERE user_id = ? AND id = ?`, now, now, request.UserID, request.AssetID).Error; err != nil {
			return err
		}
		// A Profile avatar is a pointer to the asset identity, not a copy: it is
		// cleared with the manifest so no profile keeps naming an erased asset.
		if err := tx.Exec(`UPDATE user_profiles SET avatar_asset_id = NULL, revision = revision + 1, updated_at = ?
WHERE user_id = ? AND avatar_asset_id = ?`, now, request.UserID, request.AssetID).Error; err != nil {
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
// Runtime checkpoints live outside this database: their target verifies on
// the ai-service receipt, and no new copy can appear once the source is
// unavailable (images are resolved from it) and the posture runs are gone.
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
		{model.AssetErasureDiagnosisRecords, countAssetPinningConditions},
		{model.AssetErasureDocumentExtractions, countAssetDocumentCopies},
		{model.AssetErasureAssessmentReports, countAssetAssessmentRefs},
		{model.AssetErasureEpisodicNotes, countAssetEpisodicNotes},
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
