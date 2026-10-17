package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

// lockPlanDiagnosisContext reads the adopted native aggregate, never legacy AI
// analyses. Lock order is owner -> BodyState -> Diagnosis -> linked sources.
// The head lock fences concurrent adoption/CAS. SHARE source locks keep source
// availability stable until the plan decision commits. An empty R0 is legal;
// initializing its stable head creates neither a diagnosis nor a revision.
func (r *IndependentTrainingPlanRepository) lockPlanDiagnosisContext(ctx context.Context, owner uuid.UUID, ids []uuid.UUID, bodyRevision int64) (json.RawMessage, error) {
	db := r.dbFor(ctx)
	sources := NewDiagnosisSourceRepository(db)
	if _, err := NewDiagnosisRepository(db).EnsureAggregate(ctx, owner); err != nil {
		return nil, err
	}
	var head model.Diagnosis
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=?", owner).Take(&head).Error; err != nil {
		return nil, err
	}
	conditions := []model.ConditionEntry{}
	evidence := []model.ConditionEntryEvidence{}
	if len(ids) > 0 {
		if err := db.Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id IN ?", owner, ids).Order("id").Find(&conditions).Error; err != nil {
			return nil, err
		}
		if len(conditions) != len(ids) {
			return nil, ErrIndependentPlanNotFound
		}
		for _, entry := range conditions {
			if entry.CommittedRevision > head.CurrentRevision || entry.Disposition != model.ConditionAdopted {
				return nil, fmt.Errorf("associated condition is not currently adopted")
			}
			if len(entry.SourceClaimRefs) == 0 && entry.Assessment == nil {
				return nil, fmt.Errorf("associated condition has no readable source")
			}
			for _, claim := range entry.SourceClaimRefs {
				if err := sources.Claim(ctx, owner, claim); err != nil {
					return nil, err
				}
			}
			if entry.Assessment != nil {
				if err := sources.Analysis(ctx, owner, entry.Assessment.BasedOnRunID); err != nil {
					return nil, err
				}
			}
		}
		if err := db.Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND condition_id IN ?", owner, ids).Order("condition_id,evidence_type,evidence_id,evidence_version,role").Limit(1001).Find(&evidence).Error; err != nil {
			return nil, err
		}
		if len(evidence) > 1000 {
			return nil, fmt.Errorf("condition evidence exceeds bounded read")
		}
		for _, link := range evidence {
			if err := sources.Evidence(ctx, owner, model.DiagnosisEvidenceRef{EvidenceType: link.EvidenceType, EvidenceID: link.EvidenceID, Version: strconv.FormatInt(link.EvidenceVersion, 10)}, bodyRevision); err != nil {
				return nil, err
			}
		}
	}
	return json.Marshal(struct {
		CurrentRevision  int64
		Conditions       []model.ConditionEntry
		Evidence         []model.ConditionEntryEvidence
		SourcesAvailable bool
	}{head.CurrentRevision, conditions, evidence, true})
}
