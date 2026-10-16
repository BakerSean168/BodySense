package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResolveSafetyRestriction is only for an authenticated user's explicit command.
// It is not exposed as an AI tool. The aggregate lock serializes screen recording
// with exact-target lifts; retries return the original receipt before CAS checks.
func (r *BodyStateRepository) ResolveSafetyRestriction(ctx context.Context, owner, target uuid.UUID, command model.ResolveSafetyRestrictionCommand) (*model.SafetyResolution, error) {
	if owner == uuid.Nil || target == uuid.Nil {
		return nil, model.ErrInvalidSafetyResolution
	}
	if err := command.Validate(); err != nil {
		return nil, err
	}
	fingerprint, err := bodyFactFingerprintValue(struct {
		Target  uuid.UUID
		Command model.ResolveSafetyRestrictionCommand
	}{target, command})
	if err != nil {
		return nil, err
	}
	var output model.SafetyResolution
	err = database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, owner, nil)
		if err != nil {
			return err
		}
		var prior model.SafetyResolution
		err = tx.Where("user_id=? AND idempotency_key=?", owner, command.IdempotencyKey).Take(&prior).Error
		if err == nil {
			if prior.UserID != owner {
				return fmt.Errorf("safety resolution owner mismatch")
			}
			if prior.Fingerprint != fingerprint {
				return model.ErrSafetyResolutionIdempotencyConflict
			}
			output = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var restriction model.SafetyRestriction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=? AND id=?", owner, target).Take(&restriction).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrSafetyRestrictionNotFound
			}
			return err
		}
		if restriction.UserID != owner {
			return fmt.Errorf("safety restriction owner mismatch")
		}
		if restriction.PolicyRevision != model.SafetyScreenRestrictionPolicyV1 {
			return model.ErrInvalidSafetyResolution
		}
		if err := tx.Model(&model.SafetyRestrictionExercise{}).Where("user_id=? AND restriction_id=?", owner, target).Order("exercise_id").Pluck("exercise_id", &restriction.ExerciseIDs).Error; err != nil {
			return err
		}
		policy := command.BasisKind
		if policy == "expiry" {
			policy = "on_expiry"
		}
		if restriction.LiftPolicy != policy {
			return model.ErrInvalidSafetyResolution
		}
		var existing int64
		if err := tx.Model(&model.SafetyResolution{}).Where("user_id=? AND restriction_id=?", owner, target).Count(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return model.ErrSafetyRestrictionAlreadyResolved
		}
		now := r.safetyNow()
		evidence, err := safetyResolutionEvidence(tx, owner, restriction, command, now, true)
		if err != nil {
			return err
		}
		if restriction.CreatedRevision != command.ExpectedRestrictionVersion {
			return fmt.Errorf("%w: stale restriction version", ErrBodyStateRevisionConflict)
		}
		if evidence != nil {
			var uses int64
			if err := tx.Model(&model.SafetyResolution{}).Where("user_id=? AND source_message_id=?", owner, command.SourceMessageID).Count(&uses).Error; err != nil {
				return err
			}
			if uses != 0 {
				return fmt.Errorf("%w: confirmation already used", model.ErrInvalidSafetyResolution)
			}
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: stale safety revision", ErrBodyStateRevisionConflict)
		}
		output = model.SafetyResolution{ID: uuid.New(), UserID: owner, RestrictionID: target, RestrictionVersion: restriction.CreatedRevision, SourceVersion: command.SourceVersion, BasisKind: command.BasisKind, SourceLevel: "user_reported", ScopeKind: restriction.ScopeKind, PolicyRevision: model.SafetyResolutionPolicyV1, IdempotencyKey: command.IdempotencyKey, Fingerprint: fingerprint, Revision: next, EventID: uuid.New(), Actor: "user", LiftedAt: now}
		if evidence != nil {
			output.SourceEvidenceID, output.SourceMessageID = &evidence.ID, evidence.SourceMessageID
			output.CheckInteractionID = evidence.SourceInteractionID
		} else {
			output.SourceLevel, output.Actor = "go_policy", "system"
		}
		metadata := map[string]any{"resolution_id": output.ID, "restriction_id": target, "source_evidence_id": output.SourceEvidenceID, "policy_revision": output.PolicyRevision}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "safety.restriction_resolved", output.BasisKind, metadata)
		if err != nil {
			return err
		}
		output.RevisionID = revision.ID
		if evidence != nil {
			if err := tx.Create(evidence).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&output).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		return tx.Create(&model.BodyStateOutbox{ID: output.EventID, UserID: owner, Revision: next, EventType: "body_state.safety_restriction_resolved", Payload: datatypes.JSON(payload), CreatedAt: now}).Error
	})
	if err != nil {
		return nil, err
	}
	return &output, nil
}

// ListSafetyRestrictions reads every restriction, including historical lifts,
// in one database snapshot. Truncation would silently remove old active risks.
func (r *BodyStateRepository) ListSafetyRestrictions(ctx context.Context, owner uuid.UUID) (*model.SafetyRestrictionList, error) {
	if owner == uuid.Nil {
		return nil, model.ErrInvalidSafetyResolution
	}
	var output *model.SafetyRestrictionList
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		head, err := NewBodyStateRepository(tx).GetBodyStateHead(ctx, owner)
		if err != nil {
			return err
		}
		var rows []model.SafetyRestriction
		if err := tx.Where("user_id=?", owner).Order("created_revision, id").Find(&rows).Error; err != nil {
			return err
		}
		var exercises []model.SafetyRestrictionExercise
		if err := tx.Where("user_id=?", owner).Order("restriction_id, exercise_id").Find(&exercises).Error; err != nil {
			return err
		}
		scoped := map[uuid.UUID][]string{}
		for _, exercise := range exercises {
			if exercise.UserID != owner {
				return fmt.Errorf("safety restriction exercise owner mismatch")
			}
			scoped[exercise.RestrictionID] = append(scoped[exercise.RestrictionID], exercise.ExerciseID)
		}
		var resolutions []model.SafetyResolution
		if err := tx.Where("user_id=?", owner).Find(&resolutions).Error; err != nil {
			return err
		}
		resolved := make(map[uuid.UUID]*model.SafetyResolution, len(resolutions))

		for i := range resolutions {
			resolution := &resolutions[i]
			if resolution.UserID != owner || resolution.Revision > head.CurrentRevision {
				return fmt.Errorf("safety resolution owner/revision mismatch")
			}
			resolved[resolution.RestrictionID] = resolution
		}
		output = &model.SafetyRestrictionList{UserID: owner, CurrentRevision: head.CurrentRevision, Restrictions: []model.SafetyRestrictionRecord{}}
		for _, row := range rows {
			if row.UserID != owner || row.CreatedRevision > head.CurrentRevision {
				return fmt.Errorf("safety restriction owner/revision mismatch")
			}
			if row.LiftConditions == nil {
				row.LiftConditions = []string{}
			}
			row.ExerciseIDs = scoped[row.ID]
			delete(scoped, row.ID)
			if row.ExerciseIDs == nil {
				row.ExerciseIDs = []string{}
			}
			if (row.ScopeKind == "exercise") != (len(row.ExerciseIDs) > 0) {
				return fmt.Errorf("safety restriction exercise scope mismatch")
			}
			resolution := resolved[row.ID]
			if resolution != nil && resolution.Revision <= row.CreatedRevision {
				return fmt.Errorf("invalid safety resolution revision")
			}
			valid := false
			if resolution != nil {
				valid, err = safetyResolutionAvailable(tx, owner, row, *resolution, r.safetyNow())
				if err != nil {
					return err
				}
			}
			output.Restrictions = append(output.Restrictions, model.SafetyRestrictionRecord{SafetyRestriction: row, Resolution: resolution, ClearanceAvailable: valid, Active: !valid})
		}
		if len(scoped) > 0 {
			return fmt.Errorf("orphan safety restriction exercise scope")
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return output, nil
}
