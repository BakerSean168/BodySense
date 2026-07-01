package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// MessageService handles message business logic.
type MessageService struct {
	messageRepo messageRepo
}

// NewMessageService creates a new MessageService.
func NewMessageService(messageRepo messageRepo) *MessageService {
	return &MessageService{messageRepo: messageRepo}
}

// CreateMessage creates a new message in a conversation.
func (s *MessageService) CreateMessage(
	ctx context.Context,
	conversationID uuid.UUID,
	turnID uuid.UUID,
	role string,
	parts datatypes.JSON,
	seq int,
	status string,
	metadata datatypes.JSON,
) (*model.Message, error) {
	message := &model.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		TurnID:         turnID,
		Role:           role,
		Status:         status,
		Seq:            seq,
		Parts:          parts,
		Metadata:       metadata,
	}
	if err := s.messageRepo.Create(ctx, message); err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}
	return message, nil
}

// GetMessages retrieves all messages for a conversation ordered by seq.
func (s *MessageService) GetMessages(ctx context.Context, conversationID uuid.UUID) ([]model.Message, error) {
	messages, err := s.messageRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	return messages, nil
}

// UpdateMessageStatus updates the status of a message.
func (s *MessageService) UpdateMessageStatus(ctx context.Context, id, conversationID uuid.UUID, status string) error {
	if err := s.messageRepo.UpdateStatus(ctx, id, conversationID, status); err != nil {
		return fmt.Errorf("update message status: %w", err)
	}
	return nil
}

// UpdateMessageCompleted marks a message as completed with parts, usage, and provider info.
func (s *MessageService) UpdateMessageCompleted(
	ctx context.Context,
	id,
	conversationID uuid.UUID,
	parts datatypes.JSON,
	usage map[string]any,
	providerInfo map[string]any,
) error {
	if err := s.messageRepo.UpdateCompleted(ctx, id, conversationID, parts, usage, providerInfo); err != nil {
		return fmt.Errorf("update message completed: %w", err)
	}
	return nil
}

// GetNextSeq returns the next sequence number for a conversation.
func (s *MessageService) GetNextSeq(ctx context.Context, conversationID uuid.UUID) (int, error) {
	seq, err := s.messageRepo.GetNextSeq(ctx, conversationID)
	if err != nil {
		return 0, fmt.Errorf("get next seq: %w", err)
	}
	return seq, nil
}
