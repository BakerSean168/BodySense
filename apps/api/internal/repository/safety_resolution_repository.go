package repository

import (
	"context"
	"database/sql"
	"encoding/json"
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
		if restriction.PolicyRevision != model.SafetyScreenRestrictionPolicyV1 || restriction.LiftPolicy != "professional_clearance" || restriction.ScopeKind != "all_training" || restriction.RiskLevel != "restricted" {
			return model.ErrInvalidSafetyResolution
		}
		var existing int64
		if err := tx.Model(&model.SafetyResolution{}).Where("user_id=? AND restriction_id=?", owner, target).Count(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return model.ErrSafetyRestrictionAlreadyResolved
		}
		// Resolve ownership before revision checks. Foreign or missing ids reveal
		// neither the owner's current revision nor the other person's source content.
		var messages []struct {
			model.Message
			SourceRunID uuid.UUID
		}
		if err := tx.Table("public.messages AS m").Select("m.*, r.id AS source_run_id").
			Joins("JOIN public.conversations AS c ON c.id=m.conversation_id").
			Joins("JOIN public.runs AS r ON r.turn_id=m.turn_id AND r.conversation_id=m.conversation_id").
			Where("m.id=? AND c.user_id=? AND r.user_id=? AND c.deleted_at IS NULL AND m.role='user' AND m.status='completed'", command.SourceMessageID, owner, owner).
			Clauses(clause.Locking{Strength: "SHARE"}).Limit(2).Find(&messages).Error; err != nil {
			return err
		}
		if len(messages) != 1 {
			return model.ErrSafetyResolutionSourceNotFound
		}
		source := messages[0]
		if source.ID != command.SourceMessageID || source.SourceRunID == uuid.Nil {
			return fmt.Errorf("safety clearance source identity mismatch")
		}
		// Require the whole message to be this explicit confirmation, allowing only
		// surrounding whitespace/punctuation. Quoted, negated or hypothetical phrases
		// inside unrelated prose cannot be mistaken for a clearance declaration.
		original := messageTextFromParts(source.Parts)
		if source.CreatedAt.Before(restriction.ValidFrom) || (original != command.Statement && original != command.Statement+"。") {
			return model.ErrInvalidSafetyResolution
		}
		var sourceUses int64
		if err := tx.Model(&model.SafetyResolution{}).Where("user_id=? AND source_message_id=?", owner, source.ID).Count(&sourceUses).Error; err != nil {
			return err
		}
		if sourceUses != 0 {
			return fmt.Errorf("%w: confirmation already bound to another restriction", model.ErrInvalidSafetyResolution)
		}
		if state.CurrentRevision != command.ExpectedRevision {
			return fmt.Errorf("%w: stale safety revision", ErrBodyStateRevisionConflict)
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		evidence := model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: 1, SourceConversationID: &source.ConversationID, SourceRunID: &source.SourceRunID, SourceMessageID: &source.ID, Excerpt: command.Statement, Availability: "available", RecordedAt: now}
		output = model.SafetyResolution{ID: uuid.New(), UserID: owner, RestrictionID: target, SourceEvidenceID: evidence.ID, SourceMessageID: source.ID, SourceVersion: 1, BasisKind: "professional_clearance", SourceLevel: "user_reported", ScopeKind: "all_training", PolicyRevision: model.SafetyResolutionPolicyV1, IdempotencyKey: command.IdempotencyKey, Fingerprint: fingerprint, Revision: next, EventID: uuid.New(), Actor: "user", LiftedAt: now}
		metadata := map[string]any{"resolution_id": output.ID, "restriction_id": target, "source_evidence_id": evidence.ID, "policy_revision": output.PolicyRevision}
		revision, err := bodyStatePersistRevision(ctx, tx, state, next, "safety.restriction_resolved", "user_message", metadata)
		if err != nil {
			return err
		}
		output.RevisionID = revision.ID
		if err := tx.Create(&evidence).Error; err != nil {
			return err
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
		available := map[uuid.UUID]bool{}
		for i := range resolutions {
			resolution := &resolutions[i]
			if resolution.UserID != owner || resolution.Revision > head.CurrentRevision {
				return fmt.Errorf("safety resolution owner/revision mismatch")
			}
			var evidence model.StatementEvidence
			if err := tx.Where("user_id=? AND id=?", owner, resolution.SourceEvidenceID).Take(&evidence).Error; err != nil {
				return err
			}
			if evidence.UserID != owner {
				return fmt.Errorf("safety clearance evidence owner mismatch")
			}
			resolved[resolution.RestrictionID] = resolution
			available[resolution.RestrictionID] = evidence.Availability == "available" && evidence.SourceKind == "user_message" && evidence.SourceVersion == resolution.SourceVersion && evidence.SourceMessageID != nil && *evidence.SourceMessageID == resolution.SourceMessageID && evidence.Excerpt == model.ProfessionalClearanceStatement
		}
		output = &model.SafetyRestrictionList{UserID: owner, CurrentRevision: head.CurrentRevision, Restrictions: []model.SafetyRestrictionRecord{}}
		for _, row := range rows {
			if row.UserID != owner || row.CreatedRevision > head.CurrentRevision {
				return fmt.Errorf("safety restriction owner/revision mismatch")
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
			output.Restrictions = append(output.Restrictions, model.SafetyRestrictionRecord{SafetyRestriction: row, Resolution: resolution, ClearanceAvailable: available[row.ID], Active: resolution == nil || !available[row.ID]})
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
