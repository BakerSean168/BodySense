package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DiagnosisSourceRepository qualifies current owner-bound sources, not medical
// conclusions. Both diagnosis decisions and plan admission use this authority.
// SHARE locks keep source availability stable until their transaction commits.
type DiagnosisSourceRepository struct{ db *gorm.DB }

func NewDiagnosisSourceRepository(db *gorm.DB) *DiagnosisSourceRepository {
	return &DiagnosisSourceRepository{db: db}
}
func (r *DiagnosisSourceRepository) dbFor(ctx context.Context) *gorm.DB {
	return database.FromContext(ctx, r.db)
}
func (r *DiagnosisSourceRepository) Statement(ctx context.Context, owner, id uuid.UUID, version int64) error {
	var source model.StatementEvidence
	if err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id=?", owner, id).Take(&source).Error; err != nil {
		return err
	}
	if source.Availability != "available" || (version > 0 && source.SourceVersion != version) {
		return fmt.Errorf("statement source unavailable or changed")
	}
	return nil
}
func (r *DiagnosisSourceRepository) Analysis(ctx context.Context, owner, id uuid.UUID) error {
	var run model.ConditionAnalysisRun
	if err := r.dbFor(ctx).Where("user_id=? AND id=?", owner, id).Take(&run).Error; err != nil {
		return err
	}
	return r.Statement(ctx, owner, run.RequestEvidenceID, run.RequestSourceVersion)
}
func (r *DiagnosisSourceRepository) Claim(ctx context.Context, owner uuid.UUID, claim model.DiagnosisSourceClaimRef) error {
	id, err := uuid.Parse(claim.SourceID)
	if err != nil || id == uuid.Nil || claim.SourceVersion == "" {
		return fmt.Errorf("condition claim source is not resolvable")
	}
	switch claim.SourceKind {
	case model.DiagnosisUserSuspicion, model.DiagnosisUserReportedClinical:
		v, err := strconv.ParseInt(claim.SourceVersion, 10, 64)
		if err != nil || v <= 0 {
			return fmt.Errorf("invalid statement source version")
		}
		return r.Statement(ctx, owner, id, v)
	case model.DiagnosisAgentAnalysis:
		return r.Analysis(ctx, owner, id)
	case model.DiagnosisClinicalDocument:
		admission, err := uuid.Parse(claim.EvidenceAdmissionID)
		if err != nil || claim.SourceVersion != "1" {
			return fmt.Errorf("unreadable document admission")
		}
		return r.Document(ctx, owner, admission, &id)
	default:
		return fmt.Errorf("unknown condition source kind")
	}
}
func (r *DiagnosisSourceRepository) Document(ctx context.Context, owner, reviewID uuid.UUID, assetID *uuid.UUID) error {
	var review model.DocumentIndicatorReview
	if err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id=?", owner, reviewID).Take(&review).Error; err != nil {
		return err
	}
	if (review.Action != "confirm" && review.Action != "correct") || (assetID != nil && *assetID != review.AssetID) {
		return fmt.Errorf("document source admission unavailable")
	}
	var asset model.UserAsset
	if err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id=?", owner, review.AssetID).Take(&asset).Error; err != nil {
		return err
	}
	if asset.Kind != model.UserAssetKindReport || asset.Availability != model.UserAssetAvailable {
		return fmt.Errorf("document source unavailable")
	}
	return nil
}
func (r *DiagnosisSourceRepository) Evidence(ctx context.Context, owner uuid.UUID, ref model.DiagnosisEvidenceRef, bodyRevision int64) error {
	v, err := strconv.ParseInt(ref.Version, 10, 64)
	if err != nil || v <= 0 || strconv.FormatInt(v, 10) != ref.Version || ref.EvidenceID == uuid.Nil {
		return fmt.Errorf("invalid exact evidence version")
	}
	db := r.dbFor(ctx).Clauses(clause.Locking{Strength: "SHARE"})
	switch ref.EvidenceType {
	case model.DiagnosisBodyFact:
		var fact model.BodyFact
		if err := db.Where("user_id=? AND id=? AND created_revision=?", owner, ref.EvidenceID, v).Take(&fact).Error; err != nil {
			return err
		}
		if fact.AssertionStatus != "asserted" {
			return fmt.Errorf("condition fact is no longer asserted")
		}
		return r.Statement(ctx, owner, fact.SourceEvidenceID, 0)
	case model.DiagnosisBodyObservation:
		var observation model.BodyObservation
		if err := db.Where("user_id=? AND id=? AND created_revision=?", owner, ref.EvidenceID, v).Take(&observation).Error; err != nil {
			return err
		}
		if observation.AssertionStatus != "asserted" {
			return fmt.Errorf("condition observation is no longer asserted")
		}
		return r.Statement(ctx, owner, observation.SourceEvidenceID, 0)
	case model.DiagnosisSymptomEpisode:
		var episode model.SymptomEpisode
		if err := db.Where("user_id=? AND id=?", owner, ref.EvidenceID).Take(&episode).Error; err != nil {
			return err
		}
		if episode.IdentityStatus != "current" || v < episode.CreatedRevision || v > bodyRevision {
			return fmt.Errorf("condition episode source is not current")
		}
		var links []model.EpisodeFactLink
		if err := db.Where("user_id=? AND episode_id=? AND created_revision<=?", owner, episode.ID, v).Order("fact_id").Limit(1001).Find(&links).Error; err != nil {
			return err
		}
		if len(links) == 0 || len(links) > 1000 {
			return fmt.Errorf("episode source evidence unavailable")
		}
		for _, link := range links {
			if err := r.Statement(ctx, owner, link.BasisEvidenceID, 0); err != nil {
				return err
			}
		}
		return nil
	case model.DiagnosisDocumentEvidence:
		if v != 1 {
			return fmt.Errorf("invalid document evidence version")
		}
		return r.Document(ctx, owner, ref.EvidenceID, nil)
	default:
		return fmt.Errorf("unknown condition evidence type")
	}
}

// AnalysisRequest proves that the formal run is grounded in its owner's user
// message. A stored request pointer or an assistant message is not authorization.
func (r *DiagnosisSourceRepository) AnalysisRequest(ctx context.Context, owner uuid.UUID, run model.ConditionAnalysisRun) error {
	return r.UserRequest(ctx, owner, run.RequestEvidenceID, run.RequestMessageID, run.RequestSourceVersion)
}

// UserRequest proves a Diagnosis command is grounded in the owner's own,
// still-available chat message at the exact version. Formal analysis and
// direct edits share it, so neither an assistant message nor another owner's
// statement can authorize a Diagnosis change.
func (r *DiagnosisSourceRepository) UserRequest(ctx context.Context, owner, evidence, messageID uuid.UUID, version int64) error {
	var source model.StatementEvidence
	db := r.dbFor(ctx)
	if err := db.Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id=? AND id=?", owner, evidence).Take(&source).Error; err != nil {
		return err
	}
	if source.Availability != "available" || source.SourceKind != "user_message" || source.SourceVersion != version || source.SourceMessageID == nil || *source.SourceMessageID != messageID || source.SourceConversationID == nil {
		return fmt.Errorf("diagnosis request source unavailable")
	}
	var message model.Message
	return db.Table("messages AS m").Select("m.*").Joins("JOIN conversations AS c ON c.id=m.conversation_id").
		Where("c.user_id=? AND m.id=? AND m.conversation_id=? AND m.role='user'", owner, messageID, *source.SourceConversationID).
		Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "m"}}).Take(&message).Error
}
