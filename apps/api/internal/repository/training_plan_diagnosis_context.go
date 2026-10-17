package repository

import (
	"context"
	"encoding/json"
	"errors"
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
				if err := r.planClaimSource(ctx, owner, claim); err != nil {
					return nil, err
				}
			}
			if entry.Assessment != nil {
				if err := r.planAnalysisSource(ctx, owner, entry.Assessment.BasedOnRunID); err != nil {
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
			if err := r.planEvidenceSource(ctx, owner, link, bodyRevision); err != nil {
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
func (r *IndependentTrainingPlanRepository) planStatementSource(ctx context.Context, owner, id uuid.UUID, version int64) error {
	var source model.StatementEvidence
	err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id=?", owner, id).Take(&source).Error
	if err != nil {
		return fmt.Errorf("read condition statement source: %w", err)
	}
	if source.Availability != "available" || (version > 0 && source.SourceVersion != version) {
		return fmt.Errorf("condition statement source unavailable or changed")
	}
	return nil
}
func (r *IndependentTrainingPlanRepository) planAnalysisSource(ctx context.Context, owner, id uuid.UUID) error {
	var run model.ConditionAnalysisRun
	if err := r.dbFor(ctx).Where("user_id=? AND id=?", owner, id).Take(&run).Error; err != nil {
		return err
	}
	return r.planStatementSource(ctx, owner, run.RequestEvidenceID, run.RequestSourceVersion)
}
func (r *IndependentTrainingPlanRepository) planClaimSource(ctx context.Context, owner uuid.UUID, claim model.DiagnosisSourceClaimRef) error {
	id, err := uuid.Parse(claim.SourceID)
	if err != nil || id == uuid.Nil || claim.SourceVersion == "" {
		return fmt.Errorf("condition claim source is not resolvable")
	}
	// The assessment-time validity is not a current source authority. Read the
	// owner-bound source instead; never infer clinical eligibility from its text.
	switch claim.SourceKind {
	case model.DiagnosisUserSuspicion, model.DiagnosisUserReportedClinical:
		version, err := strconv.ParseInt(claim.SourceVersion, 10, 64)
		if err != nil || version <= 0 {
			return fmt.Errorf("invalid statement source version")
		}
		return r.planStatementSource(ctx, owner, id, version)
	case model.DiagnosisAgentAnalysis:
		return r.planAnalysisSource(ctx, owner, id)
	case model.DiagnosisClinicalDocument:
		admission, err := uuid.Parse(claim.EvidenceAdmissionID)
		if err != nil || claim.SourceVersion != "1" {
			return fmt.Errorf("unreadable document admission")
		}
		return r.planDocumentSource(ctx, owner, admission, &id)
	default:
		return fmt.Errorf("unknown condition source kind")
	}
}
func (r *IndependentTrainingPlanRepository) planDocumentSource(ctx context.Context, owner, reviewID uuid.UUID, uploadID *uuid.UUID) error {
	var review model.DocumentIndicatorReview
	if err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id=?", owner, reviewID).Take(&review).Error; err != nil {
		return err
	}
	if (review.Action != "confirm" && review.Action != "correct") || (uploadID != nil && *uploadID != review.UploadID) {
		return fmt.Errorf("document source admission unavailable")
	}
	// This checks original source availability only; a review is not proof of a
	// clinical diagnosis and never creates or reclassifies an adopted condition.
	var upload model.UserUpload
	return r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id=?", owner, review.UploadID).Take(&upload).Error
}
func (r *IndependentTrainingPlanRepository) planEvidenceSource(ctx context.Context, owner uuid.UUID, link model.ConditionEntryEvidence, bodyRevision int64) error {
	db := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"})
	switch link.EvidenceType {
	case model.DiagnosisBodyFact:
		var fact model.BodyFact
		if err := db.Where("user_id=? AND id=? AND created_revision=?", owner, link.EvidenceID, link.EvidenceVersion).Take(&fact).Error; err != nil {
			return err
		}
		if fact.AssertionStatus != "asserted" {
			return fmt.Errorf("condition fact is no longer asserted")
		}
		return r.planStatementSource(ctx, owner, fact.SourceEvidenceID, 0)
	case model.DiagnosisBodyObservation:
		var observation model.BodyObservation
		if err := db.Where("user_id=? AND id=? AND created_revision=?", owner, link.EvidenceID, link.EvidenceVersion).Take(&observation).Error; err != nil {
			return err
		}
		if observation.AssertionStatus != "asserted" {
			return fmt.Errorf("condition observation is no longer asserted")
		}
		return r.planStatementSource(ctx, owner, observation.SourceEvidenceID, 0)
	case model.DiagnosisSymptomEpisode:
		var episode model.SymptomEpisode
		if err := db.Where("user_id=? AND id=?", owner, link.EvidenceID).Take(&episode).Error; err != nil {
			return err
		}
		if episode.IdentityStatus != "current" || link.EvidenceVersion < episode.CreatedRevision || link.EvidenceVersion > bodyRevision {
			return fmt.Errorf("condition episode source is not current")
		}
		var links []model.EpisodeFactLink
		if err := db.Where("user_id=? AND episode_id=? AND created_revision<=?", owner, episode.ID, link.EvidenceVersion).Order("fact_id").Limit(1001).Find(&links).Error; err != nil {
			return err
		}
		if len(links) == 0 || len(links) > 1000 {
			return fmt.Errorf("episode source evidence unavailable")
		}
		for _, source := range links {
			if err := r.planStatementSource(ctx, owner, source.BasisEvidenceID, 0); err != nil {
				return err
			}
		}
		return nil
	case model.DiagnosisDocumentEvidence:
		if link.EvidenceVersion != 1 {
			return fmt.Errorf("invalid document evidence version")
		}
		return r.planDocumentSource(ctx, owner, link.EvidenceID, nil)
	default:
		return errors.New("unknown condition evidence type")
	}
}
