package repository

import (
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// statementDecisionTables hold the immutable Diagnosis and plan decision
// trail. Their frozen replay inputs and audit content may embed a statement
// or a record derived from it; they are never rewritten (decision_records is
// retained_by_policy) and account erasure removes them.
var statementDecisionTables = []string{
	"condition_analysis_runs", "condition_analysis_events", "diagnosis_proposals",
	"diagnosis_decisions", "diagnosis_settlements", "diagnosis_revisions",
	"condition_entries", "condition_entry_evidence", "condition_relations",
	"condition_disposition_changes", "condition_direct_edits",
	"plan_proposals", "training_plan_revisions",
}

// mentioningClause matches an owner row whose JSON text names any needle.
func mentioningClause(userID uuid.UUID, needles []string) (string, []any) {
	clauses := make([]string, 0, len(needles))
	args := []any{userID}
	for _, needle := range needles {
		clauses = append(clauses, "strpos(to_jsonb(t)::text, ?) > 0")
		args = append(args, needle)
	}
	return fmt.Sprintf("t.user_id = ? AND (%s)", strings.Join(clauses, " OR ")), args
}

// eraseStatementAssessmentReports deletes the assessment reports naming the
// statement or a derived record once nothing pins them; a frozen report is
// deleted whole, never rewritten.
func eraseStatementAssessmentReports(tx *gorm.DB, userID uuid.UUID, pins statementPins) (model.AssetErasureOutcome, error) {
	needles := pins.needles()
	held, err := countRowsMentioning(tx, "assessment_reports", userID, needles)
	if err != nil || held == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, err
	}
	if len(pins.conditions) > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: held, FailureCode: model.AssetErasureCodeAssessmentFrozen}, nil
	}
	where, args := mentioningClause(userID, needles)
	deleted := tx.Exec(`DELETE FROM assessment_reports t WHERE `+where, args...)
	if deleted.Error != nil {
		return model.AssetErasureOutcome{}, deleted.Error
	}
	left, err := countRowsMentioning(tx, "assessment_reports", userID, needles)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if left > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: left, FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: int(deleted.RowsAffected)}, nil
}

// inventoryStatementDecisions never mutates: decision-trail rows naming the
// statement or a derived record are frozen replay inputs, reported as a
// terminal retained_by_policy outcome rather than a failure to retry.
func inventoryStatementDecisions(tx *gorm.DB, userID uuid.UUID, pins statementPins) (model.AssetErasureOutcome, error) {
	n, err := countStatementDecisions(tx, userID, pins.needles())
	if err != nil || n == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, err
	}
	return model.AssetErasureOutcome{State: model.ErasureTargetRetainedByPolicy, Outstanding: n, RetentionReason: model.ErasureRetentionReplayInputFrozen}, nil
}

func countStatementDecisions(tx *gorm.DB, userID uuid.UUID, needles []string) (int, error) {
	total := 0
	for _, table := range statementDecisionTables {
		n, err := countRowsMentioning(tx, table, userID, needles)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}
