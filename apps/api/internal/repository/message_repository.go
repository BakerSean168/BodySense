package repository

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MessageRepository handles database operations for messages.
type MessageRepository struct {
	db *gorm.DB
}

// NewMessageRepository creates a new MessageRepository.
func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create creates a new message.
func (r *MessageRepository) Create(ctx context.Context, message *model.Message) error {
	if strings.TrimSpace(message.ContentText) == "" {
		message.ContentText = messageTextFromParts(message.Parts)
	}
	return r.db.WithContext(ctx).Create(message).Error
}

// GetByID retrieves a message by ID with conversation ownership check.
func (r *MessageRepository) GetByID(ctx context.Context, id, conversationID uuid.UUID) (*model.Message, error) {
	var message model.Message
	err := r.db.WithContext(ctx).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &message, nil
}

// ListByConversationID retrieves all messages for a conversation ordered by seq.
func (r *MessageRepository) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Message, error) {
	var messages []model.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("seq ASC").
		Find(&messages).Error
	return messages, err
}

// FailStreamingAssistantByRunID terminalizes the persisted assistant projection
// for a reclaimed run. The row is locked and updated inside the caller's
// transaction so a thread refresh can never observe a cleared active_run_id
// while the assistant message is still marked streaming.
func (r *MessageRepository) FailStreamingAssistantByRunID(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	errorPayload any,
) (*model.Message, error) {
	encodedError, err := json.Marshal(errorPayload)
	if err != nil {
		return nil, err
	}

	db := database.FromContext(ctx, r.db)
	var message model.Message
	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(
			"run_id = ? AND conversation_id = ? AND role = ? AND status IN ?",
			runID, conversationID, "assistant", []string{"submitted", "streaming"},
		).
		Order("seq DESC").
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	errorJSON := datatypes.JSON(encodedError)
	result := db.Model(&model.Message{}).
		Where("id = ? AND conversation_id = ? AND status IN ?", message.ID, conversationID, []string{"submitted", "streaming"}).
		Updates(map[string]any{"status": "failed", "error": errorJSON})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, nil
	}
	message.Status = "failed"
	message.Error = errorJSON
	return &message, nil
}

// UpdateStatus updates the status of a message with conversation ownership check.
func (r *MessageRepository) UpdateStatus(ctx context.Context, id, conversationID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		Update("status", status).Error
}

// UpdateParts updates the parts JSONB field of a message with conversation ownership check.
func (r *MessageRepository) UpdateParts(ctx context.Context, id, conversationID uuid.UUID, parts any) error {
	return r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		Updates(map[string]any{"parts": parts, "content_text": messageTextFromParts(parts)}).Error
}

// GetNextSeq returns the next sequence number for a conversation.
// Uses COALESCE to handle the case where no messages exist yet.
func (r *MessageRepository) GetNextSeq(ctx context.Context, conversationID uuid.UUID) (int, error) {
	var seq int
	err := r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("conversation_id = ?", conversationID).
		Select("COALESCE(MAX(seq), 0) + 1").
		Scan(&seq).Error
	return seq, err
}

// UpdateCompletedWithStatus atomically updates a message's parts and status in a single query.
// Used for interaction-required finalization where parts and status must change together.
func (r *MessageRepository) UpdateCompletedWithStatus(ctx context.Context, id, conversationID uuid.UUID, parts any, status string) error {
	return r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		Updates(map[string]any{
			"parts":        parts,
			"content_text": messageTextFromParts(parts),
			"status":       status,
		}).Error
}

// UpdateCompleted updates a message with completion data: parts, usage tokens, and provider info.
func (r *MessageRepository) UpdateCompleted(ctx context.Context, id, conversationID uuid.UUID, parts any, usage map[string]any, providerInfo map[string]any) error {
	updates := map[string]any{
		"status":       "completed",
		"parts":        parts,
		"content_text": messageTextFromParts(parts),
	}
	if v, ok := usage["input_tokens"]; ok {
		updates["input_tokens"] = v
	}
	if v, ok := usage["output_tokens"]; ok {
		updates["output_tokens"] = v
	}
	if v, ok := usage["total_tokens"]; ok {
		updates["total_tokens"] = v
	}
	if v, ok := providerInfo["provider"]; ok {
		updates["provider"] = v
	}
	if v, ok := providerInfo["provider_message_id"]; ok {
		updates["provider_message_id"] = v
	}
	if v, ok := providerInfo["provider_response_id"]; ok {
		updates["provider_response_id"] = v
	}

	return r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		Updates(updates).Error
}

func messageTextFromParts(parts any) string {
	encoded, err := json.Marshal(parts)
	if err != nil {
		return ""
	}
	var values []map[string]any
	if err := json.Unmarshal(encoded, &values); err != nil {
		return ""
	}
	texts := make([]string, 0, len(values))
	for _, value := range values {
		if value["type"] != "text" {
			continue
		}
		text, _ := value["text"].(string)
		if text = strings.TrimSpace(text); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}
