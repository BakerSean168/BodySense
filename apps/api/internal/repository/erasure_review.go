package repository

import (
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// erasureBodyRefs are BodyState records derived from a source being erased.
type erasureBodyRefs struct {
	Facts        []uuid.UUID
	Observations []uuid.UUID
	Episodes     []uuid.UUID
}

func (r erasureBodyRefs) empty() bool {
	return len(r.Facts) == 0 && len(r.Observations) == 0 && len(r.Episodes) == 0
}

// nonEmptyIDs keeps an IN list valid when there is nothing to match.
func nonEmptyIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return []uuid.UUID{uuid.Nil}
	}
	return ids
}

// dependentReviews is the review signal of an erasure: the given adopted
// conditions, then every active or paused Training plan whose current
// revision rests on one of them or on a derived BodyState record. It only
// reads; nothing here changes a Diagnosis or a plan.
func dependentReviews(tx *gorm.DB, userID uuid.UUID, conditions []uuid.UUID, refs erasureBodyRefs) ([]model.ErasureReview, error) {
	reviews := make([]model.ErasureReview, 0, len(conditions))
	for _, id := range conditions {
		reviews = append(reviews, model.ErasureReview{Subject: model.ErasureReviewDiagnosisCondition, SubjectID: id})
	}
	if len(conditions) == 0 && refs.empty() {
		return reviews, nil
	}
	plans, err := dependentPlans(tx, userID, conditions, refs)
	if err != nil {
		return nil, err
	}
	return append(reviews, plans...), nil
}

func dependentPlans(tx *gorm.DB, userID uuid.UUID, conditions []uuid.UUID, refs erasureBodyRefs) ([]model.ErasureReview, error) {
	var rows []struct {
		PlanID     uuid.UUID
		RevisionID uuid.UUID
	}
	err := tx.Raw(`SELECT p.id AS plan_id, p.current_accepted_revision_id AS revision_id FROM training_plans p
WHERE p.user_id = @user AND p.status IN ('active','paused') AND p.current_accepted_revision_id IS NOT NULL AND (
  EXISTS (SELECT 1 FROM training_plan_revision_conditions rc
    WHERE rc.user_id = p.user_id AND rc.revision_id = p.current_accepted_revision_id AND rc.condition_id IN @conditions)
  OR EXISTS (SELECT 1 FROM training_plan_revision_body_refs b
    WHERE b.user_id = p.user_id AND b.revision_id = p.current_accepted_revision_id
      AND (b.body_fact_id IN @facts OR b.body_observation_id IN @observations OR b.symptom_episode_id IN @episodes)))
ORDER BY p.id`, map[string]any{
		"user": userID, "conditions": nonEmptyIDs(conditions), "facts": nonEmptyIDs(refs.Facts),
		"observations": nonEmptyIDs(refs.Observations), "episodes": nonEmptyIDs(refs.Episodes),
	}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	reviews := make([]model.ErasureReview, 0, len(rows))
	for _, row := range rows {
		revision := row.RevisionID
		reviews = append(reviews, model.ErasureReview{Subject: model.ErasureReviewTrainingPlan, SubjectID: row.PlanID, RevisionID: &revision})
	}
	return reviews, nil
}
