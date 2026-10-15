package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordAnswerFacts binds a tool proposal to the exact Go-accepted answer.
// Ordinary turn recording cannot silently relabel a reply as a new message.
func (r *BodyStateRepository) RecordAnswerFacts(ctx context.Context, userID uuid.UUID, source model.AnswerFactSource, command model.RecordBodyFactsCommand) (*model.BodyFactCommit, error) {
	if source.RunID == uuid.Nil || source.InteractionID == uuid.Nil || source.MessageID == uuid.Nil {
		return nil, fmt.Errorf("%w: accepted answer identity required", model.ErrInvalidBodyFact)
	}
	return r.recordFacts(ctx, userID, command, &model.MessageFactSource{RunID: source.RunID, MessageID: source.MessageID}, &source)
}

func verifyBodyFactAnswer(tx *gorm.DB, owner uuid.UUID, source model.AnswerFactSource, message model.Message) (*model.AgentInteraction, string, error) {
	invalid := func() (*model.AgentInteraction, string, error) {
		return nil, "", fmt.Errorf("%w: exact accepted answer source unavailable", model.ErrInvalidBodyFact)
	}
	var resume model.InteractionResume
	lock := clause.Locking{Strength: "SHARE"}
	if err := tx.Clauses(lock).Where("interaction_id = ? AND user_id = ? AND run_id = ? AND user_message_id = ? AND conversation_id = ?", source.InteractionID, owner, source.RunID, source.MessageID, message.ConversationID).First(&resume).Error; err != nil {
		return invalid()
	}
	var interaction model.AgentInteraction
	if err := tx.Clauses(lock).Where("id = ? AND run_id = ? AND conversation_id = ? AND status = ?", source.InteractionID, source.RunID, message.ConversationID, model.AgentInteractionAnswered).First(&interaction).Error; err != nil {
		return invalid()
	}
	if message.RunID == nil || *message.RunID != source.RunID || interaction.ToolName != "ask_user" || strings.TrimSpace(interaction.ToolCallID) == "" || len(interaction.ToolCallID) > 256 {
		return invalid()
	}
	answer, text, err := model.ValidateInteractionAnswer(interaction.Question, interaction.Answer)
	if err != nil || text != messageTextFromParts(message.Parts) {
		return invalid()
	}
	answerHash, err := model.InteractionJSONFingerprint(answer)
	if err != nil || answerHash != resume.AnswerFingerprint {
		return invalid()
	}
	questionHash, err := model.InteractionJSONFingerprint(interaction.Question)
	if err != nil || questionHash != resume.QuestionFingerprint {
		return invalid()
	}
	var q struct {
		Question string `json:"question"`
	}
	if json.Unmarshal(interaction.Question, &q) != nil || strings.TrimSpace(q.Question) == "" {
		return invalid()
	}
	questionRunes := []rune(q.Question)
	if len(questionRunes) > 2000 {
		questionRunes = questionRunes[:2000]
	}
	return &interaction, string(questionRunes), nil
}

// RecordSafetyScreen commits the user's answer to a Go-bound symptom intake as
// one safety screen in its own BodyState revision. The bound fact must be the
// owner's asserted, non-absent V2 symptom statement committed in the same Run;
// the screen references it instead of inserting a duplicate symptom fact.
func (r *BodyStateRepository) RecordSafetyScreen(ctx context.Context, userID uuid.UUID, source model.AnswerFactSource, command model.SafetyScreenCommand) (*model.SafetyScreen, error) {
	if userID == uuid.Nil || source.RunID == uuid.Nil || source.InteractionID == uuid.Nil || source.MessageID == uuid.Nil || command.BodyFactID == uuid.Nil {
		return nil, fmt.Errorf("%w: accepted answer and bound fact identities required", model.ErrInvalidBodyFact)
	}
	if len(command.CaptureID) != 24 || strings.Trim(command.CaptureID, "0123456789abcdef") != "" {
		return nil, fmt.Errorf("%w: invalid structured intake capture", model.ErrInvalidBodyFact)
	}
	var screen *model.SafetyScreen
	err := database.FromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		state, next, err := bodyStateLockNextRevision(ctx, tx, userID, nil)
		if err != nil {
			return err
		}
		var messages []model.Message
		if err := tx.Table("public.messages AS m").Select("m.*").
			Joins("JOIN public.runs AS r ON r.turn_id = m.turn_id AND r.conversation_id = m.conversation_id").
			Joins("JOIN public.conversations AS c ON c.id = m.conversation_id").
			Where("m.id = ? AND r.id = ? AND r.user_id = ? AND c.user_id = ? AND c.deleted_at IS NULL AND r.status = ? AND m.role = ? AND m.status = ?", source.MessageID, source.RunID, userID, userID, "running", "user", "completed").
			Clauses(clause.Locking{Strength: "SHARE"}).Limit(2).Find(&messages).Error; err != nil {
			return err
		}
		if len(messages) != 1 {
			return fmt.Errorf("%w: exactly one live accepted answer is required for this run", model.ErrInvalidBodyFact)
		}
		interaction, _, err := verifyBodyFactAnswer(tx, userID, source, messages[0])
		if err != nil {
			return err
		}
		if interaction.ToolCallID != "intake-"+command.CaptureID {
			return fmt.Errorf("%w: structured intake is not bound to its runtime question", model.ErrInvalidBodyFact)
		}
		// The bound symptom must still be the owner's asserted, non-absent
		// statement and must originate from this Run's own user message.
		var fact model.BodyFact
		if err := tx.Table("public.body_facts AS f").Select("f.*").
			Joins("JOIN public.statement_evidence AS e ON e.user_id = f.user_id AND e.id = f.source_evidence_id").
			Where("f.id = ? AND f.user_id = ? AND f.kind = ? AND f.assertion_status = ? AND e.source_run_id = ?", command.BodyFactID, userID, "symptom", "asserted", source.RunID).
			Clauses(clause.Locking{Strength: "SHARE"}).Take(&fact).Error; err != nil {
			return fmt.Errorf("%w: bound symptom fact is unavailable", model.ErrInvalidBodyFact)
		}
		var details model.SymptomFactDetails
		if json.Unmarshal(fact.Details, &details) != nil || details.Presence == "absent" {
			return fmt.Errorf("%w: an absent symptom needs no safety screen", model.ErrInvalidBodyFact)
		}
		now := time.Now().UTC()
		answers := command.Answers
		screen = &model.SafetyScreen{
			ID: uuid.New(), UserID: userID, CaptureID: command.CaptureID, BodyFactID: fact.ID,
			SourceConversationID: messages[0].ConversationID, SourceRunID: source.RunID, SourceMessageID: messages[0].ID,
			SourceInteractionID: interaction.ID, SourceToolCallID: interaction.ToolCallID,
			Trauma: answers.Trauma, RadiatingPain: answers.RadiatingPain, Numbness: answers.Numbness,
			Weakness: answers.Weakness, Dizziness: answers.Dizziness, BodyStateRevision: next, RecordedAt: now,
		}
		// Revision and outbox carry identifiers only.
		metadata := map[string]any{"safety_screen_id": screen.ID, "body_fact_id": fact.ID}
		if _, err := bodyStatePersistRevision(ctx, tx, state, next, "safety.screened", "structured_answer", metadata); err != nil {
			return err
		}
		if err := tx.Create(screen).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		return tx.Create(&model.BodyStateOutbox{ID: uuid.New(), UserID: userID, Revision: next, EventType: "body_state.safety_screened", Payload: datatypes.JSON(payload), CreatedAt: now}).Error
	})
	if err != nil {
		return nil, err
	}
	return screen, nil
}

// ListSafetyScreens returns every safety screen of the owner, newest first.
// It is deliberately unbounded: an unresolved positive concept in an old
// screen must keep blocking, so truncation would fail open.
func (r *BodyStateRepository) ListSafetyScreens(ctx context.Context, userID uuid.UUID) ([]model.SafetyScreen, error) {
	screens := []model.SafetyScreen{}
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidBodyFact)
	}
	err := database.FromContext(ctx, r.db).Where("user_id = ?", userID).
		Order("body_state_revision DESC, id ASC").Find(&screens).Error
	if err != nil {
		return nil, err
	}
	return screens, nil
}
