package repository

import (
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// statementPinningConditionsSQL selects the adopted condition entries citing
// the statement: a current evidence link to a derived fact or observation, or
// a user claim naming the statement. Withdrawn (inactive) conditions do not pin.
const statementPinningConditionsSQL = `SELECT c.id FROM condition_entries c
WHERE c.user_id = @user AND c.disposition = 'adopted' AND (
  EXISTS (SELECT 1 FROM condition_entry_evidence e
    WHERE e.user_id = c.user_id AND e.condition_id = c.id
      AND (e.body_fact_id IN @facts OR e.body_observation_id IN @observations))
  OR EXISTS (SELECT 1 FROM jsonb_array_elements(c.source_claim_refs) ref
    WHERE ref->>'sourceKind' IN ('user_suspicion','user_reported_clinical') AND ref->>'sourceId' = @statement))
ORDER BY c.id`

func statementPinningConditions(tx *gorm.DB, userID, statementID uuid.UUID, derived statementDerived) ([]uuid.UUID, error) {
	refs := derived.bodyRefs()
	var ids []uuid.UUID
	err := tx.Raw(statementPinningConditionsSQL, map[string]any{
		"user": userID, "statement": statementID.String(),
		"facts": nonEmptyIDs(refs.Facts), "observations": nonEmptyIDs(refs.Observations),
	}).Scan(&ids).Error
	return ids, err
}

// statementPins is what holds a statement's copies in place right now.
type statementPins struct {
	statementID uuid.UUID
	derived     statementDerived
	conditions  []uuid.UUID
	plans       []model.ErasureReview
}

func (p statementPins) needles() []string { return p.derived.needles(p.statementID) }

// lockStatementPins takes the statement row lock (Diagnosis and plan
// admission qualify a statement under a share lock) and the derived record
// locks, then reads the conditions and plans that pin the statement.
func lockStatementPins(tx *gorm.DB, userID, statementID uuid.UUID) (statementPins, error) {
	pins := statementPins{statementID: statementID}
	if err := tx.Exec(`SELECT 1 FROM statement_evidence WHERE user_id = ? AND id = ? FOR UPDATE`, userID, statementID).Error; err != nil {
		return pins, err
	}
	var err error
	if pins.derived, err = statementDerivedRecords(tx, userID, statementID, true); err != nil {
		return pins, err
	}
	if pins.conditions, err = statementPinningConditions(tx, userID, statementID, pins.derived); err != nil {
		return pins, err
	}
	refs := pins.derived.bodyRefs()
	if len(pins.conditions) > 0 || !refs.empty() {
		if pins.plans, err = dependentPlans(tx, userID, pins.conditions, refs); err != nil {
			return pins, err
		}
	}
	return pins, nil
}
