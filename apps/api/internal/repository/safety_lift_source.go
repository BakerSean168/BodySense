package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *BodyStateRepository) safetyNow() time.Time {
	if r.safetyClock != nil {
		return r.safetyClock().UTC().Truncate(time.Microsecond)
	}
	return time.Now().UTC().Truncate(time.Microsecond)
}

func safetyResolutionEvidence(tx *gorm.DB, owner uuid.UUID, restriction model.SafetyRestriction, c model.ResolveSafetyRestrictionCommand, now time.Time, locking bool) (*model.StatementEvidence, error) {
	if now.IsZero() {
		return nil, model.ErrInvalidSafetyResolution
	}
	if c.BasisKind == "expiry" {
		if restriction.LiftPolicy != "on_expiry" || restriction.ValidUntil == nil || !restriction.ValidUntil.After(restriction.ValidFrom) || now.Before(*restriction.ValidUntil) {
			return nil, model.ErrInvalidSafetyResolution
		}
		return nil, nil
	}
	// Owner lookup precedes CAS. Neither foreign messages nor interactions expose
	// the other owner's revision, text or answer.
	var messages []struct {
		model.Message
		SourceRunID uuid.UUID
	}
	query := tx
	if locking {
		query = tx.Clauses(clause.Locking{Strength: "SHARE"}).Session(&gorm.Session{})
	}
	if err := query.Table("public.messages AS m").Select("m.*, r.id AS source_run_id").
		Joins("JOIN public.conversations AS c ON c.id=m.conversation_id").
		Joins("JOIN public.runs AS r ON r.turn_id=m.turn_id AND r.conversation_id=m.conversation_id").
		Where("m.id=? AND c.user_id=? AND r.user_id=? AND m.role='user' AND m.status='completed'", c.SourceMessageID, owner, owner).
		Limit(2).Find(&messages).Error; err != nil {
		return nil, err
	}
	if len(messages) != 1 {
		return nil, model.ErrSafetyResolutionSourceNotFound
	}
	source := messages[0]
	if source.CreatedAt.Before(restriction.ValidFrom) || source.CreatedAt.After(now) || source.SourceRunID == uuid.Nil {
		return nil, model.ErrInvalidSafetyResolution
	}
	evidence := &model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: c.SourceVersion, SourceConversationID: &source.ConversationID, SourceRunID: &source.SourceRunID, SourceMessageID: &source.ID, Excerpt: c.Statement, Availability: "available", RecordedAt: now}
	if c.BasisKind == "professional_clearance" {
		original := messageTextFromParts(source.Parts)
		if c.Statement != model.SafetyProfessionalClearanceStatement(restriction) || (original != c.Statement && original != c.Statement+"。") {
			return nil, model.ErrInvalidSafetyResolution
		}
	} else {
		var interaction model.AgentInteraction
		if err := query.Table("agent_interactions AS i").Select("i.*").
			Joins("JOIN conversations AS c ON c.id=i.conversation_id").
			Joins("JOIN runs AS r ON r.id=i.run_id AND r.conversation_id=c.id").
			Where("i.id=? AND c.user_id=? AND r.user_id=?", c.CheckInteractionID, owner, owner).
			Take(&interaction).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, model.ErrSafetyResolutionSourceNotFound
			}
			return nil, err
		}
		accepted, question, err := verifyBodyFactAnswer(tx, owner, model.AnswerFactSource{RunID: source.SourceRunID, InteractionID: c.CheckInteractionID, MessageID: source.ID}, source.Message, locking)
		if err != nil || accepted == nil || interaction.AnsweredAt == nil || interaction.AnsweredAt.Before(restriction.ValidFrom) || interaction.AnsweredAt.After(now) || (interaction.ExpiresAt != nil && !interaction.AnsweredAt.Before(*interaction.ExpiresAt)) || !model.SafetyReverificationSatisfied(restriction, interaction.Question, interaction.Answer) {
			return nil, model.ErrInvalidSafetyResolution
		}
		evidence.SourceKind = "structured_answer"
		evidence.SourceInteractionID = &interaction.ID
		evidence.SourceToolCallID = &interaction.ToolCallID
		evidence.QuestionExcerpt = &question
		evidence.Excerpt = messageTextFromParts(source.Parts)
	}
	return evidence, nil
}

// Read-side revalidation is deliberately live: invalidated evidence, changed
// accepted answers and unavailable conversations re-block without deleting history.
func safetyResolutionAvailable(tx *gorm.DB, owner uuid.UUID, row model.SafetyRestriction, lift model.SafetyResolution, now time.Time) (bool, error) {
	if lift.RestrictionVersion != row.CreatedRevision || lift.ScopeKind != row.ScopeKind || lift.PolicyRevision != model.SafetyResolutionPolicyV1 {
		return false, nil
	}
	c := model.ResolveSafetyRestrictionCommand{IdempotencyKey: "read", ExpectedRestrictionVersion: row.CreatedRevision, BasisKind: lift.BasisKind, SourceVersion: lift.SourceVersion}
	if lift.BasisKind == "expiry" {
		if lift.SourceEvidenceID != nil || lift.SourceMessageID != nil || lift.CheckInteractionID != nil || lift.SourceLevel != "go_policy" || lift.Actor != "system" || lift.SourceVersion != 0 || row.LiftPolicy != "on_expiry" || row.ValidUntil == nil || lift.LiftedAt.Before(*row.ValidUntil) {
			return false, nil
		}
		_, err := safetyResolutionEvidence(tx, owner, row, c, now, false)
		return err == nil, nil
	}
	if lift.SourceEvidenceID == nil || lift.SourceMessageID == nil || lift.SourceLevel != "user_reported" || lift.Actor != "user" {
		return false, nil
	}
	var evidence model.StatementEvidence
	if err := tx.Where("user_id=? AND id=?", owner, *lift.SourceEvidenceID).Take(&evidence).Error; err != nil {
		return false, err
	}
	if evidence.Availability != "available" || evidence.SourceVersion != lift.SourceVersion || evidence.SourceMessageID == nil || *evidence.SourceMessageID != *lift.SourceMessageID {
		return false, nil
	}
	c.SourceMessageID = *lift.SourceMessageID
	if lift.CheckInteractionID != nil {
		c.CheckInteractionID = *lift.CheckInteractionID
	}
	if lift.BasisKind == "professional_clearance" {
		c.Statement = model.SafetyProfessionalClearanceStatement(row)
	}
	policy := lift.BasisKind
	if policy != row.LiftPolicy || c.Validate() != nil {
		return false, nil
	}
	verified, err := safetyResolutionEvidence(tx, owner, row, c, now, false)
	if errors.Is(err, model.ErrInvalidSafetyResolution) || errors.Is(err, model.ErrSafetyResolutionSourceNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return evidence.SourceKind == verified.SourceKind && evidence.Excerpt == verified.Excerpt && equalUUID(evidence.SourceInteractionID, verified.SourceInteractionID) && equalUUID(evidence.SourceRunID, verified.SourceRunID) && equalUUID(evidence.SourceConversationID, verified.SourceConversationID), nil
}

func equalUUID(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// bindSafetyReverification runs only at the durable AskUser creation boundary.
// Runtime prose cannot supply its own conditions, positive answer or version.
func bindSafetyReverification(ctx context.Context, tx *gorm.DB, interaction *model.AgentInteraction) error {
	var request struct {
		Purpose            string    `json:"purpose"`
		RestrictionID      uuid.UUID `json:"restriction_id"`
		RestrictionVersion int64     `json:"restriction_version"`
	}
	if json.Unmarshal(interaction.Question, &request) != nil || request.Purpose != model.SafetyReverificationPurpose {
		return nil
	}
	var restriction model.SafetyRestriction
	err := tx.WithContext(ctx).Table("safety_restrictions AS s").Select("s.*").
		Joins("JOIN runs AS r ON r.user_id=s.user_id").
		Joins("JOIN conversations AS c ON c.id=r.conversation_id AND c.user_id=s.user_id").
		Where("s.id=? AND r.id=? AND c.id=?", request.RestrictionID, interaction.RunID, interaction.ConversationID).Take(&restriction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrSafetyRestrictionNotFound
	}
	if err != nil {
		return err
	}
	if restriction.CreatedRevision != request.RestrictionVersion {
		return fmt.Errorf("%w: stale reverification target", ErrBodyStateRevisionConflict)
	}
	if err := tx.Model(&model.SafetyRestrictionExercise{}).Where("user_id=? AND restriction_id=?", restriction.UserID, restriction.ID).Order("exercise_id").Pluck("exercise_id", &restriction.ExerciseIDs).Error; err != nil {
		return err
	}
	canonical, err := model.SafetyReverificationQuestion(restriction)
	if err != nil {
		return err
	}
	interaction.Question = canonical
	return nil
}
