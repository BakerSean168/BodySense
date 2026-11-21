package repository

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExecuteDatabaseTarget erases (or, for families that need a review first or
// are kept by policy, inventories) one database-resident target and verifies
// it inside the same transaction, under the statement row lock.
func (r *StatementErasureRepository) ExecuteDatabaseTarget(ctx context.Context, request *model.StatementErasureRequest, target model.StatementErasureTargetName) (model.AssetErasureOutcome, error) {
	var outcome model.AssetErasureOutcome
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		if target == model.StatementErasureSourceMessage {
			// Chat rows are guarded by the Conversation lock, not the pins.
			var err error
			outcome, err = eraseStatementSourceMessage(tx, request.UserID, request.StatementID)
			return err
		}
		pins, err := lockStatementPins(tx, request.UserID, request.StatementID)
		if err != nil {
			return err
		}
		switch target {
		case model.StatementErasureDiagnosisRecords:
			outcome = blockedWhile(len(pins.conditions), model.AssetErasureCodeDiagnosisReview)
		case model.StatementErasureTrainingPlans:
			outcome = blockedWhile(len(pins.plans), model.ErasureCodePlanReview)
		case model.StatementErasureBodyRecords:
			outcome, err = eraseStatementBodyRecords(tx, request.UserID, request.StatementID, pins)
		case model.StatementErasureAssessmentReports:
			outcome, err = eraseStatementAssessmentReports(tx, request.UserID, pins)
		case model.StatementErasureDecisionRecords:
			outcome, err = inventoryStatementDecisions(tx, request.UserID, pins)
		case model.StatementErasureEpisodicNotes:
			outcome, err = eraseStatementEpisodicNotes(tx, request.UserID, request.StatementID)
		default:
			err = fmt.Errorf("target %s is not a database target", target)
		}
		return err
	})
	return outcome, err
}

func blockedWhile(n int, code string) model.AssetErasureOutcome {
	if n == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: n, FailureCode: code}
}

// RuntimeScope lists what the runtime_checkpoints target hands the
// ai-service: every Conversation of the owner, the one the statement was said
// in, and the derived record ids. The user's own chat message is a
// conversation record (chat retention), not a copy.
func (r *StatementErasureRepository) RuntimeScope(ctx context.Context, userID, statementID uuid.UUID) (model.StatementRuntimeScope, error) {
	db := r.conn(ctx)
	scope := model.StatementRuntimeScope{ThreadIDs: []uuid.UUID{}, AttachedThreadIDs: []uuid.UUID{}}
	var rows []struct {
		ID       uuid.UUID
		Attached bool
	}
	if err := db.Raw(`SELECT c.id, EXISTS (SELECT 1 FROM statement_evidence e
    WHERE e.user_id = c.user_id AND e.id = ? AND e.source_conversation_id = c.id) AS attached
 FROM conversations c WHERE c.user_id = ? ORDER BY c.id`, statementID, userID).Scan(&rows).Error; err != nil {
		return scope, fmt.Errorf("statement runtime scope: %w", err)
	}
	for _, row := range rows {
		scope.ThreadIDs = append(scope.ThreadIDs, row.ID)
		if row.Attached {
			scope.AttachedThreadIDs = append(scope.AttachedThreadIDs, row.ID)
		}
	}
	derived, err := statementDerivedRecords(db, userID, statementID, false)
	if err != nil {
		return scope, fmt.Errorf("statement runtime scope: %w", err)
	}
	scope.RecordIDs = derived.ids()
	return scope, nil
}
