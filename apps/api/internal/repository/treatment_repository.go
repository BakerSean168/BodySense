package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TreatmentRepository persists one current aggregate plus immutable plan
// revisions, executable interventions, and longitudinal outcomes.
type TreatmentRepository struct {
	db *gorm.DB
}

func NewTreatmentRepository(db *gorm.DB) *TreatmentRepository {
	return &TreatmentRepository{db: db}
}

func (r *TreatmentRepository) ensureLocked(
	ctx context.Context,
	tx *gorm.DB,
	userID uuid.UUID,
) (*model.Treatment, error) {
	var treatment model.Treatment
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&treatment).Error
	if err == nil {
		return &treatment, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	treatment = model.Treatment{
		ID: uuid.New(), UserID: userID, CurrentRevision: 0,
		Status:        model.TreatmentStatusReviewRecommended,
		StatusReasons: datatypes.JSON(`[]`),
	}
	if err := tx.WithContext(ctx).Create(&treatment).Error; err != nil {
		// Concurrent first creation is resolved by reloading the unique user row.
		if reloadErr := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&treatment).Error; reloadErr != nil {
			return nil, err
		}
	}
	return &treatment, nil
}

func (r *TreatmentRepository) CreateProposal(
	ctx context.Context,
	userID uuid.UUID,
	revision model.TreatmentRevision,
	interventions []model.Intervention,
) (*model.Treatment, *model.TreatmentRevision, error) {
	var treatment *model.Treatment
	var stored model.TreatmentRevision
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		locked, err := r.ensureLocked(ctx, tx, userID)
		if err != nil {
			return err
		}
		treatment = locked
		var next int
		if err := tx.WithContext(ctx).Model(&model.TreatmentRevision{}).
			Where("treatment_id = ?", locked.ID).
			Select("COALESCE(MAX(revision), 0) + 1").
			Scan(&next).Error; err != nil {
			return err
		}
		if next <= 0 {
			next = 1
		}
		revision.ID = uuid.New()
		revision.TreatmentID = locked.ID
		revision.Revision = next
		revision.AcceptanceState = model.TreatmentAcceptanceProposed
		revision.LifecycleState = model.TreatmentStatusReviewRecommended
		if revision.CreatedAt.IsZero() {
			revision.CreatedAt = time.Now().UTC()
		}
		if err := tx.WithContext(ctx).Create(&revision).Error; err != nil {
			return err
		}
		stored = revision
		for index := range interventions {
			interventions[index].ID = uuid.New()
			interventions[index].UserID = userID
			interventions[index].TreatmentID = locked.ID
			interventions[index].TreatmentRevisionID = revision.ID
			interventions[index].Position = index
			interventions[index].Status = "proposed"
			if len(interventions[index].Prescription) == 0 {
				interventions[index].Prescription = datatypes.JSON(`{}`)
			}
		}
		if len(interventions) > 0 {
			if err := tx.WithContext(ctx).Create(&interventions).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	stored.Interventions = interventions
	return treatment, &stored, nil
}

func (r *TreatmentRepository) AcceptRevision(
	ctx context.Context,
	userID, revisionID uuid.UUID,
	expectedBodyStateRevision int64,
	acceptanceDecisionTrace datatypes.JSON,
) (*model.Treatment, *model.TreatmentRevision, bool, error) {
	var treatment model.Treatment
	var revision model.TreatmentRevision
	bodyStateMatched := true
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Joins("JOIN treatments ON treatments.id = treatment_revisions.treatment_id").
			Where("treatment_revisions.id = ? AND treatments.user_id = ?", revisionID, userID).
			First(&revision).Error; err != nil {
			return err
		}
		var bodyState model.BodyState
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&bodyState).Error; err != nil {
			return err
		}
		if bodyState.CurrentRevision != expectedBodyStateRevision {
			bodyStateMatched = false
			return nil
		}
		locked, err := r.ensureLocked(ctx, tx, userID)
		if err != nil {
			return err
		}
		if locked.ID != revision.TreatmentID {
			return errors.New("treatment revision ownership mismatch")
		}
		if revision.AcceptanceState != model.TreatmentAcceptanceProposed {
			return errors.New("only proposed treatment revisions can be accepted")
		}
		now := time.Now().UTC()
		// Previous accepted revisions and interventions remain historical but cease
		// to be the current executable strategy.
		if err := tx.WithContext(ctx).Model(&model.TreatmentRevision{}).
			Where("treatment_id = ? AND acceptance_state = ? AND id <> ?", locked.ID, model.TreatmentAcceptanceAccepted, revision.ID).
			Update("lifecycle_state", model.TreatmentStatusSuperseded).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&model.Intervention{}).
			Where("treatment_id = ? AND treatment_revision_id <> ? AND status IN ?", locked.ID, revision.ID, []string{"active", "paused", "proposed"}).
			Updates(map[string]any{"status": "superseded", "ended_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&model.TreatmentRevision{}).
			Where("id = ? AND treatment_id = ?", revision.ID, locked.ID).
			Updates(map[string]any{
				"acceptance_state":          model.TreatmentAcceptanceAccepted,
				"lifecycle_state":           model.TreatmentStatusActive,
				"accepted_at":               now,
				"acceptance_decision_trace": acceptanceDecisionTrace,
			}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&model.Intervention{}).
			Where("treatment_revision_id = ?", revision.ID).
			Updates(map[string]any{"status": "active", "started_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&model.Treatment{}).
			Where("id = ? AND user_id = ?", locked.ID, userID).
			Updates(map[string]any{
				"current_revision":             revision.Revision,
				"status":                       model.TreatmentStatusActive,
				"source_body_state_revision":   revision.SourceBodyStateRevision,
				"source_diagnosis_analysis_id": revision.SourceDiagnosisAnalysisID,
				"status_reasons":               datatypes.JSON(`[]`),
				"updated_at":                   now,
			}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", locked.ID).First(&treatment).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", revision.ID).First(&revision).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).
			Where("treatment_revision_id = ?", revision.ID).
			Order("position ASC").
			Find(&revision.Interventions).Error
	})
	if err != nil {
		return nil, nil, false, err
	}
	if !bodyStateMatched {
		return nil, nil, false, nil
	}
	treatment.Current = &revision
	return &treatment, &revision, true, nil
}

func (r *TreatmentRepository) RejectRevision(
	ctx context.Context,
	userID, revisionID uuid.UUID,
) error {
	return database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var revision model.TreatmentRevision
		if err := tx.WithContext(ctx).
			Joins("JOIN treatments ON treatments.id = treatment_revisions.treatment_id").
			Where("treatment_revisions.id = ? AND treatments.user_id = ?", revisionID, userID).
			First(&revision).Error; err != nil {
			return err
		}
		if revision.AcceptanceState == model.TreatmentAcceptanceAccepted {
			return errors.New("accepted treatment revision cannot be rejected")
		}
		if err := tx.WithContext(ctx).Model(&model.TreatmentRevision{}).
			Where("id = ?", revisionID).
			Update("acceptance_state", model.TreatmentAcceptanceRejected).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Model(&model.Intervention{}).
			Where("treatment_revision_id = ? AND status = ?", revisionID, "proposed").
			Update("status", "cancelled").Error
	})
}

func (r *TreatmentRepository) SetStatus(
	ctx context.Context,
	userID uuid.UUID,
	status string,
	reasons datatypes.JSON,
) (*model.Treatment, error) {
	if len(reasons) == 0 {
		reasons = datatypes.JSON(`[]`)
	}
	var treatment model.Treatment
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		locked, err := r.ensureLocked(ctx, tx, userID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.WithContext(ctx).Model(&model.Treatment{}).
			Where("id = ?", locked.ID).
			Updates(map[string]any{"status": status, "status_reasons": reasons, "updated_at": now}).Error; err != nil {
			return err
		}
		interventionStatus := "active"
		if status == model.TreatmentStatusPaused {
			interventionStatus = "paused"
		}
		if status == model.TreatmentStatusCompleted || status == model.TreatmentStatusSuperseded {
			interventionStatus = status
		}
		if status != model.TreatmentStatusReviewRecommended {
			if err := tx.WithContext(ctx).Model(&model.Intervention{}).
				Where("treatment_id = ? AND status IN ?", locked.ID, []string{"active", "paused"}).
				Updates(map[string]any{"status": interventionStatus, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		return tx.WithContext(ctx).Where("id = ?", locked.ID).First(&treatment).Error
	})
	if err != nil {
		return nil, err
	}
	return &treatment, nil
}

func (r *TreatmentRepository) GetCurrent(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Treatment, error) {
	var treatment model.Treatment
	err := database.FromContext(ctx, r.db).Where("user_id = ?", userID).First(&treatment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if treatment.CurrentRevision > 0 {
		var revision model.TreatmentRevision
		if err := database.FromContext(ctx, r.db).
			Where("treatment_id = ? AND revision = ?", treatment.ID, treatment.CurrentRevision).
			First(&revision).Error; err != nil {
			return nil, err
		}
		if err := database.FromContext(ctx, r.db).
			Where("treatment_revision_id = ?", revision.ID).
			Order("position ASC").
			Find(&revision.Interventions).Error; err != nil {
			return nil, err
		}
		treatment.Current = &revision
	}
	return &treatment, nil
}

func (r *TreatmentRepository) GetRevision(
	ctx context.Context,
	userID, revisionID uuid.UUID,
) (*model.TreatmentRevision, error) {
	var revision model.TreatmentRevision
	err := database.FromContext(ctx, r.db).
		Joins("JOIN treatments ON treatments.id = treatment_revisions.treatment_id").
		Where("treatment_revisions.id = ? AND treatments.user_id = ?", revisionID, userID).
		First(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := database.FromContext(ctx, r.db).
		Where("treatment_revision_id = ?", revision.ID).
		Order("position ASC").
		Find(&revision.Interventions).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

func (r *TreatmentRepository) ListRevisions(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.TreatmentRevision, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var treatment model.Treatment
	if err := database.FromContext(ctx, r.db).Where("user_id = ?", userID).First(&treatment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []model.TreatmentRevision{}, nil
		}
		return nil, err
	}
	var revisions []model.TreatmentRevision
	if err := database.FromContext(ctx, r.db).
		Where("treatment_id = ?", treatment.ID).
		Order("revision DESC").
		Limit(limit).
		Find(&revisions).Error; err != nil {
		return nil, err
	}
	for index := range revisions {
		if err := database.FromContext(ctx, r.db).
			Where("treatment_revision_id = ?", revisions[index].ID).
			Order("position ASC").
			Find(&revisions[index].Interventions).Error; err != nil {
			return nil, err
		}
	}
	return revisions, nil
}

func (r *TreatmentRepository) GetIntervention(
	ctx context.Context,
	userID, interventionID uuid.UUID,
) (*model.Intervention, error) {
	var item model.Intervention
	err := database.FromContext(ctx, r.db).
		Where("id = ? AND user_id = ?", interventionID, userID).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *TreatmentRepository) CreateOutcome(
	ctx context.Context,
	outcome *model.Outcome,
) (*model.Outcome, bool, error) {
	if outcome.ID == uuid.Nil {
		outcome.ID = uuid.New()
	}
	if outcome.CausalityLevel == "" {
		outcome.CausalityLevel = "association_only"
	}
	if outcome.OccurredAt.IsZero() {
		outcome.OccurredAt = time.Now().UTC()
	}
	if len(outcome.Value) == 0 {
		outcome.Value = datatypes.JSON(`{}`)
	}
	if len(outcome.Provenance) == 0 {
		outcome.Provenance = datatypes.JSON(`{}`)
	}
	result := database.FromContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "source_type"}, {Name: "source_key"}},
		DoNothing: true,
	}).Create(outcome)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return outcome, true, nil
	}
	var stored model.Outcome
	if err := database.FromContext(ctx, r.db).
		Where("user_id = ? AND source_type = ? AND source_key = ?", outcome.UserID, outcome.SourceType, outcome.SourceKey).
		First(&stored).Error; err != nil {
		return nil, false, err
	}
	return &stored, false, nil
}

func (r *TreatmentRepository) UpdateOutcomeBodyStateRevision(
	ctx context.Context,
	outcomeID, userID uuid.UUID,
	revision int64,
) error {
	return database.FromContext(ctx, r.db).Model(&model.Outcome{}).
		Where("id = ? AND user_id = ?", outcomeID, userID).
		Update("body_state_revision", revision).Error
}

func (r *TreatmentRepository) ListOutcomes(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]model.Outcome, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var items []model.Outcome
	err := database.FromContext(ctx, r.db).
		Where("user_id = ?", userID).
		Order("occurred_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func (r *TreatmentRepository) RequireCurrentRevision(
	ctx context.Context,
	userID uuid.UUID,
) (*model.Treatment, *model.TreatmentRevision, error) {
	treatment, err := r.GetCurrent(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if treatment == nil || treatment.Current == nil {
		return nil, nil, fmt.Errorf("no accepted current treatment")
	}
	return treatment, treatment.Current, nil
}
