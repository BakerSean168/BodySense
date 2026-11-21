package repository

import (
	"context"

	"github.com/bodysense/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Finalize erases the statement and ends the request in one transaction,
// only when every other target verified (decision_records and source_message
// may be retained by policy) and a re-check of every database family still holds. A copy that
// reappeared reopens its target instead. It returns whether the request is
// now terminal.
func (r *StatementErasureRepository) Finalize(ctx context.Context, request *model.StatementErasureRequest) (bool, error) {
	terminal := false
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.StatementErasureRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", request.UserID, request.ID).Take(&current).Error; err != nil {
			return err
		}
		if current.Status.Terminal() {
			terminal = true
			return nil
		}
		pins, err := lockStatementPins(tx, request.UserID, request.StatementID)
		if err != nil {
			return err
		}
		now := r.now()
		blockManifest := func() error {
			return recordStatementErasureOutcome(tx, now, request.UserID, request.ID, model.StatementErasureManifest,
				model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: 1, FailureCode: model.AssetErasureCodeTargetsOutstanding})
		}
		var unverified int64
		if err := tx.Model(&model.StatementErasureTarget{}).
			Where("user_id = ? AND request_id = ? AND target <> ? AND state NOT IN ?", request.UserID, request.ID, model.StatementErasureManifest,
				[]model.AssetErasureTargetState{model.AssetErasureTargetVerified, model.ErasureTargetRetainedByPolicy}).
			Count(&unverified).Error; err != nil {
			return err
		}
		if unverified > 0 {
			return blockManifest()
		}
		reappeared := map[model.StatementErasureTargetName]int{}
		if n := len(pins.conditions); n > 0 {
			reappeared[model.StatementErasureDiagnosisRecords] = n
		}
		if n := len(pins.plans); n > 0 {
			reappeared[model.StatementErasureTrainingPlans] = n
		}
		if n := pins.derived.unerased(); n > 0 {
			reappeared[model.StatementErasureBodyRecords] = n
		}
		if n, err := countRowsMentioning(tx, "assessment_reports", request.UserID, pins.needles()); err != nil {
			return err
		} else if n > 0 {
			reappeared[model.StatementErasureAssessmentReports] = n
		}
		if n, _, err := episodicNotesForStatement(tx, request.UserID, request.StatementID); err != nil {
			return err
		} else if n > 0 {
			reappeared[model.StatementErasureEpisodicNotes] = n
		}
		if len(reappeared) > 0 {
			for target, n := range reappeared {
				if err := recordStatementErasureOutcome(tx, now, request.UserID, request.ID, target,
					model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: n, FailureCode: model.AssetErasureCodeCopyReappeared}); err != nil {
					return err
				}
			}
			return blockManifest()
		}
		// A decision made since the decision_records pass is retained too;
		// the statement row lock keeps the answer stable until commit.
		decisions, err := inventoryStatementDecisions(tx, request.UserID, pins)
		if err != nil {
			return err
		}
		if err := recordStatementErasureOutcome(tx, now, request.UserID, request.ID, model.StatementErasureDecisionRecords, decisions); err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE statement_evidence SET availability = 'erased', excerpt = '', question_excerpt = NULL
WHERE user_id = ? AND id = ?`, request.UserID, request.StatementID).Error; err != nil {
			return err
		}
		if err := recordStatementErasureOutcome(tx, now, request.UserID, request.ID, model.StatementErasureManifest,
			model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: 1}); err != nil {
			return err
		}
		var retained int64
		if err := tx.Model(&model.StatementErasureTarget{}).
			Where("user_id = ? AND request_id = ? AND state = ?", request.UserID, request.ID, model.ErasureTargetRetainedByPolicy).
			Count(&retained).Error; err != nil {
			return err
		}
		status := model.StatementErasureErased
		if retained > 0 {
			status = model.StatementErasureErasedWithRetention
		}
		if err := tx.Exec(`UPDATE statement_erasure_requests
SET status = ?, erased_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE user_id = ? AND id = ?`, status, now, now, request.UserID, request.ID).Error; err != nil {
			return err
		}
		terminal = true
		return nil
	})
	return terminal, err
}
