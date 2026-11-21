package repository

import (
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// statementDerived lists the BodyState records derived from one statement.
type statementDerived struct {
	Facts, Observations, Hypotheses []statementDerivedRecord
}

type statementDerivedRecord struct {
	ID              uuid.UUID
	AssertionStatus string
	Erased          bool
}

func (d statementDerived) all() [][]statementDerivedRecord {
	return [][]statementDerivedRecord{d.Facts, d.Observations, d.Hypotheses}
}

func derivedIDs(records []statementDerivedRecord) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func (d statementDerived) bodyRefs() erasureBodyRefs {
	return erasureBodyRefs{Facts: derivedIDs(d.Facts), Observations: derivedIDs(d.Observations)}
}

// ids is every derived record id (runtime and replay needles).
func (d statementDerived) ids() []uuid.UUID {
	out := []uuid.UUID{}
	for _, records := range d.all() {
		out = append(out, derivedIDs(records)...)
	}
	return out
}

// needles is the statement id and every derived record id as text.
func (d statementDerived) needles(statementID uuid.UUID) []string {
	out := []string{statementID.String()}
	for _, id := range d.ids() {
		out = append(out, id.String())
	}
	return out
}

func (d statementDerived) active() int {
	n := 0
	for _, records := range d.all() {
		for _, record := range records {
			if record.AssertionStatus == "asserted" {
				n++
			}
		}
	}
	return n
}

// unerased counts derived records whose text is still present.
func (d statementDerived) unerased() int {
	n := 0
	for _, records := range d.all() {
		for _, record := range records {
			if !record.Erased {
				n++
			}
		}
	}
	return n
}

func (d statementDerived) activeReviews() []model.ErasureReview {
	reviews := []model.ErasureReview{}
	subjects := []model.ErasureReviewSubject{model.ErasureReviewBodyFact, model.ErasureReviewBodyObservation, model.ErasureReviewBodyHypothesis}
	for i, records := range d.all() {
		for _, record := range records {
			if record.AssertionStatus == "asserted" {
				reviews = append(reviews, model.ErasureReview{Subject: subjects[i], SubjectID: record.ID})
			}
		}
	}
	return reviews
}

// statementDerivedRecords reads (and with lock, row-locks) the records
// derived from the statement.
func statementDerivedRecords(tx *gorm.DB, userID, statementID uuid.UUID, lock bool) (statementDerived, error) {
	var out statementDerived
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	for _, family := range []struct {
		table string
		into  *[]statementDerivedRecord
	}{{"body_facts", &out.Facts}, {"body_observations", &out.Observations}, {"body_hypotheses", &out.Hypotheses}} {
		query := fmt.Sprintf(`SELECT id, assertion_status, erased_at IS NOT NULL AS erased FROM %s
WHERE user_id = ? AND source_evidence_id = ? ORDER BY id%s`, family.table, suffix)
		if err := tx.Raw(query, userID, statementID).Scan(family.into).Error; err != nil {
			return out, err
		}
	}
	return out, nil
}
