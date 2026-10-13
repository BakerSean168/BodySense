package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClaimInteractionResume participates in the application transaction that also
// appends lifecycle milestones. Its row locks are held until those events commit.
func (r *ConsultationRepository) ClaimInteractionResume(ctx context.Context, command model.ClaimInteractionResume) (*model.InteractionResumeEnvelope, error) {
	if !database.HasTransaction(ctx) {
		return nil, model.ErrInteractionResumeUnavailable
	}
	if command.UserID == uuid.Nil || command.ConversationID == uuid.Nil || command.InteractionID == uuid.Nil || strings.TrimSpace(command.RequestID) != command.RequestID || len(command.RequestID) == 0 || len(command.RequestID) > 200 {
		return nil, model.ErrInvalidInteractionAnswer
	}
	db := database.FromContext(ctx, r.db)
	lock := clause.Locking{Strength: "UPDATE"}
	// Same owner/conversation serialization as ordinary CreateRunEnvelope.
	var owner model.User
	if err := db.Clauses(lock).First(&owner, "id = ?", command.UserID).Error; err != nil {
		return nil, resumeLookupError(err)
	}
	var conversation model.Conversation
	if err := db.Clauses(lock).Where("id = ? AND user_id = ? AND deleted_at IS NULL", command.ConversationID, command.UserID).First(&conversation).Error; err != nil {
		return nil, resumeLookupError(err)
	}
	var interaction model.AgentInteraction
	if err := db.Clauses(lock).Where("id = ? AND conversation_id = ?", command.InteractionID, conversation.ID).First(&interaction).Error; err != nil {
		return nil, resumeLookupError(err)
	}
	var run model.Run
	if err := db.Clauses(lock).Where("id = ? AND user_id = ? AND conversation_id = ?", interaction.RunID, command.UserID, conversation.ID).First(&run).Error; err != nil {
		return nil, resumeLookupError(err)
	}
	answer, text, err := model.ValidateInteractionAnswer(interaction.Question, command.Answer)
	if err != nil {
		return nil, err
	}
	answerHash, err := model.InteractionJSONFingerprint(answer)
	if err != nil {
		return nil, err
	}
	questionHash, err := model.InteractionJSONFingerprint(interaction.Question)
	if err != nil {
		return nil, err
	}

	var matchingRequest model.InteractionResume
	err = db.Where("user_id = ? AND request_id = ?", command.UserID, command.RequestID).First(&matchingRequest).Error
	if err == nil && matchingRequest.InteractionID != interaction.ID {
		return nil, model.ErrInteractionResumeConflict
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var prior model.InteractionResume
	err = db.First(&prior, "interaction_id = ?", interaction.ID).Error
	if err == nil {
		if prior.UserID != command.UserID || prior.ConversationID != conversation.ID || prior.RunID != run.ID || prior.AnswerFingerprint != answerHash || prior.QuestionFingerprint != questionHash || interaction.Status != model.AgentInteractionAnswered {
			return nil, model.ErrInteractionResumeConflict
		}
		var user, assistant model.Message
		if err := db.First(&user, "id = ? AND conversation_id = ?", prior.UserMessageID, conversation.ID).Error; err != nil {
			return nil, resumeLookupError(err)
		}
		if err := db.First(&assistant, "id = ? AND conversation_id = ?", prior.AssistantMessageID, conversation.ID).Error; err != nil {
			return nil, resumeLookupError(err)
		}
		return &model.InteractionResumeEnvelope{Receipt: &prior, Interaction: &interaction, Run: &run, UserMessage: &user, AssistantMessage: &assistant, Existed: true}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// A resume request cannot borrow an ordinary turn's idempotency identity.
	var requestCount int64
	if err := db.Model(&model.Run{}).Where("user_id = ? AND request_id = ?", command.UserID, command.RequestID).Count(&requestCount).Error; err != nil {
		return nil, err
	}
	if requestCount != 0 {
		return nil, model.ErrInteractionResumeConflict
	}
	if interaction.Status == model.AgentInteractionExpired || (interaction.ExpiresAt != nil && !interaction.ExpiresAt.After(time.Now().UTC())) {
		return nil, model.ErrInteractionResumeExpired
	}
	if interaction.Status != model.AgentInteractionPending || interaction.ToolName != "ask_user" || run.Status != model.RunStatusWaitingUser || (conversation.ActiveRunID != nil && *conversation.ActiveRunID != run.ID) {
		return nil, model.ErrInteractionResumeConflict
	}
	var pendingOthers int64
	if err := db.Model(&model.AgentInteraction{}).Where("conversation_id = ? AND status = ? AND id <> ?", conversation.ID, model.AgentInteractionPending, interaction.ID).Count(&pendingOthers).Error; err != nil {
		return nil, err
	}
	if pendingOthers != 0 {
		return nil, model.ErrInteractionResumeConflict
	}

	now := time.Now().UTC()
	interaction.Answer, interaction.Status, interaction.AnsweredAt = answer, model.AgentInteractionAnswered, &now
	if err := db.Model(&model.AgentInteraction{}).Where("id = ?", interaction.ID).Updates(map[string]any{"answer": answer, "status": interaction.Status, "answered_at": now}).Error; err != nil {
		return nil, err
	}
	var nextSeq int
	if err := db.Model(&model.Message{}).Where("conversation_id = ?", conversation.ID).Select("COALESCE(MAX(seq), 0) + 1").Scan(&nextSeq).Error; err != nil {
		return nil, err
	}
	parts, err := json.Marshal([]map[string]string{{"type": "text", "text": text}})
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(map[string]any{"is_interaction_answer": true, "interaction_id": interaction.ID.String()})
	if err != nil {
		return nil, err
	}
	user := &model.Message{ID: uuid.New(), ConversationID: conversation.ID, TurnID: run.TurnID, RunID: &run.ID, Role: "user", Status: "completed", Seq: nextSeq, Parts: datatypes.JSON(parts), Metadata: datatypes.JSON(metadata)}
	assistant := &model.Message{ID: uuid.New(), ConversationID: conversation.ID, TurnID: run.TurnID, RunID: &run.ID, Role: "assistant", Status: "streaming", Seq: nextSeq + 1, Parts: datatypes.JSON(`[]`), Metadata: datatypes.JSON(`{}`)}
	if err := db.Create(user).Error; err != nil {
		return nil, err
	}
	if err := db.Create(assistant).Error; err != nil {
		return nil, err
	}
	expires := runLeaseExpiry()
	if err := db.Model(&model.Run{}).Where("id = ? AND status = ?", run.ID, model.RunStatusWaitingUser).Updates(map[string]any{"status": model.RunStatusRunning, "lease_owner": r.leaseOwner, "lease_expires_at": expires, "lease_heartbeat_at": now}).Error; err != nil {
		return nil, err
	}
	run.Status, run.LeaseOwner, run.LeaseExpiresAt, run.LeaseHeartbeatAt = model.RunStatusRunning, r.leaseOwner, expires, &now
	if err := db.Model(&model.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{"active_run_id": run.ID, "active_stream_id": run.ID.String(), "last_message_at": now}).Error; err != nil {
		return nil, err
	}
	receipt := &model.InteractionResume{InteractionID: interaction.ID, UserID: command.UserID, ConversationID: conversation.ID, RunID: run.ID, RequestID: command.RequestID, AnswerFingerprint: answerHash, QuestionFingerprint: questionHash, UserMessageID: user.ID, AssistantMessageID: assistant.ID, ClaimedAt: now}
	if err := db.Create(receipt).Error; err != nil {
		return nil, err
	}
	return &model.InteractionResumeEnvelope{Receipt: receipt, Interaction: &interaction, Run: &run, UserMessage: user, AssistantMessage: assistant}, nil
}

func resumeLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrInteractionResumeNotFound
	}
	return err
}
