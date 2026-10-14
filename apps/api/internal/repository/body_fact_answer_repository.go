package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
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
