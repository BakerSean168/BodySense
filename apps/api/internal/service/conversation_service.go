package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type conversationRepo interface {
	Create(ctx context.Context, conversation *model.Conversation) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, cursor *time.Time, limit int) ([]model.Conversation, bool, error)
	Update(ctx context.Context, conversation *model.Conversation) error
	SoftDelete(ctx context.Context, id, userID uuid.UUID) error
	UpdatePinned(ctx context.Context, id, userID uuid.UUID, pinned bool) error
	UpdateTitle(ctx context.Context, id, userID uuid.UUID, title string) error
	UpdateTitleStatus(ctx context.Context, id, userID uuid.UUID, status string) error
	UpdateLastMessageAt(ctx context.Context, id, userID uuid.UUID) error
	UpdateActiveRunID(ctx context.Context, id, userID uuid.UUID, runID *uuid.UUID, streamID string) error
	UpdateStatus(ctx context.Context, id, userID uuid.UUID, status string) error
}

type messageRepo interface {
	Create(ctx context.Context, message *model.Message) error
	GetByID(ctx context.Context, id, conversationID uuid.UUID) (*model.Message, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Message, error)
	UpdateStatus(ctx context.Context, id, conversationID uuid.UUID, status string) error
	UpdateParts(ctx context.Context, id, conversationID uuid.UUID, parts any) error
	UpdateCompleted(ctx context.Context, id, conversationID uuid.UUID, parts any, usage map[string]any, providerInfo map[string]any) error
	GetNextSeq(ctx context.Context, conversationID uuid.UUID) (int, error)
}

type runRepo interface {
	Create(ctx context.Context, run *model.Run) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Run, error)
	GetByRequestID(ctx context.Context, userID uuid.UUID, requestID string) (*model.Run, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Run, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error
	FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) error
}

type shareRepo interface {
	Create(ctx context.Context, share *model.ConversationShare) error
	GetByToken(ctx context.Context, token string) (*model.ConversationShare, error)
	GetByConversationID(ctx context.Context, conversationID uuid.UUID) (*model.ConversationShare, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByConversationID(ctx context.Context, conversationID uuid.UUID) error
}

// ConversationService handles conversation business logic.
type ConversationService struct {
	conversationRepo conversationRepo
	messageRepo      messageRepo
	runRepo          runRepo
	shareRepo        shareRepo
	aiClient         *AIClient
}

// NewConversationService creates a new ConversationService.
func NewConversationService(
	conversationRepo conversationRepo,
	messageRepo messageRepo,
	runRepo runRepo,
	shareRepo shareRepo,
	aiClient *AIClient,
) *ConversationService {
	return &ConversationService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		runRepo:          runRepo,
		shareRepo:        shareRepo,
		aiClient:         aiClient,
	}
}

// CreateConversation creates a new conversation for a user with the given model.
func (s *ConversationService) CreateConversation(ctx context.Context, userID uuid.UUID, modelStr string) (*model.Conversation, error) {
	conversation := &model.Conversation{
		ID:           uuid.New(),
		UserID:       userID,
		DefaultModel: modelStr,
		Status:       "active",
		TitleStatus:  "pending",
	}
	if err := s.conversationRepo.Create(ctx, conversation); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return conversation, nil
}

// GetConversation retrieves a conversation by ID with ownership check, including its messages.
func (s *ConversationService) GetConversation(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, []model.Message, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get conversation: %w", err)
	}
	if conversation == nil {
		return nil, nil, nil
	}

	messages, err := s.messageRepo.ListByConversationID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("list messages: %w", err)
	}

	return conversation, messages, nil
}

// GetConversationByID retrieves a conversation by ID with ownership check (without messages).
func (s *ConversationService) GetConversationByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return conversation, nil
}

// ListConversations retrieves conversations for a user with cursor-based pagination.
func (s *ConversationService) ListConversations(ctx context.Context, userID uuid.UUID, cursor *time.Time, limit int) ([]model.Conversation, bool, error) {
	conversations, hasMore, err := s.conversationRepo.ListByUserID(ctx, userID, cursor, limit)
	if err != nil {
		return nil, false, fmt.Errorf("list conversations: %w", err)
	}
	return conversations, hasMore, nil
}

// DeleteConversation soft-deletes a conversation with ownership check.
func (s *ConversationService) DeleteConversation(ctx context.Context, id, userID uuid.UUID) error {
	// Verify ownership
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("get conversation for delete: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found: %s", id)
	}

	if err := s.conversationRepo.SoftDelete(ctx, id, userID); err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

// PinConversation pins or unpins a conversation with ownership check.
func (s *ConversationService) PinConversation(ctx context.Context, id, userID uuid.UUID, pinned bool) error {
	// Verify ownership
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("get conversation for pin: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found: %s", id)
	}

	if err := s.conversationRepo.UpdatePinned(ctx, id, userID, pinned); err != nil {
		return fmt.Errorf("pin conversation: %w", err)
	}
	return nil
}

// RenameTitle directly updates the title of a conversation and sets status to 'generated'.
func (s *ConversationService) RenameTitle(ctx context.Context, id, userID uuid.UUID, title string) error {
	// Verify ownership
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("get conversation for rename: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found: %s", id)
	}

	if err := s.conversationRepo.UpdateTitle(ctx, id, userID, title); err != nil {
		return fmt.Errorf("update title: %w", err)
	}
	if err := s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "generated"); err != nil {
		return fmt.Errorf("update title status: %w", err)
	}
	return nil
}

// GenerateTitle asynchronously generates a title for the conversation.
// Sets title_status to 'generating' and launches a goroutine that calls the AI service.
// On completion, updates title and title_status to 'generated' (or 'failed' on error).
func (s *ConversationService) GenerateTitle(ctx context.Context, id, userID uuid.UUID) error {
	// Verify ownership
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("get conversation for title generation: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found: %s", id)
	}

	// Set status to generating
	if err := s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "generating"); err != nil {
		return fmt.Errorf("update title status: %w", err)
	}

	// Async title generation (placeholder — will be replaced with queue)
	go s.generateTitleAsync(id, userID)

	return nil
}

// UpdateLastMessageAt updates the last_message_at timestamp for a conversation.
func (s *ConversationService) UpdateLastMessageAt(ctx context.Context, id, userID uuid.UUID) error {
	if err := s.conversationRepo.UpdateLastMessageAt(ctx, id, userID); err != nil {
		return fmt.Errorf("update last_message_at: %w", err)
	}
	return nil
}

// UpdateActiveRunID sets or clears the active_run_id and active_stream_id on a conversation.
func (s *ConversationService) UpdateActiveRunID(ctx context.Context, id, userID uuid.UUID, runID *uuid.UUID, streamID string) error {
	if err := s.conversationRepo.UpdateActiveRunID(ctx, id, userID, runID, streamID); err != nil {
		return fmt.Errorf("update active_run_id: %w", err)
	}
	return nil
}

// UpdateConversationStatus updates the status of a conversation (active/archived/deleted).
func (s *ConversationService) UpdateConversationStatus(ctx context.Context, id, userID uuid.UUID, status string) error {
	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("get conversation for update: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found: %s", id)
	}

	if err := s.conversationRepo.UpdateStatus(ctx, id, userID, status); err != nil {
		return fmt.Errorf("update conversation status: %w", err)
	}
	return nil
}

// ListRuns retrieves all runs for a conversation with ownership check.
func (s *ConversationService) ListRuns(ctx context.Context, conversationID, userID uuid.UUID) ([]model.Run, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversation for runs: %w", err)
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found: %s", conversationID)
	}

	runs, err := s.runRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return runs, nil
}

// generateTitleAsync calls the AI service to generate a title for the conversation.
func (s *ConversationService) generateTitleAsync(id, userID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conversation, err := s.conversationRepo.GetByID(ctx, id, userID)
	if err != nil || conversation == nil {
		log.Printf("generateTitleAsync: conversation not found %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}

	// Fetch messages to send to AI service
	messages, err := s.messageRepo.ListByConversationID(ctx, id)
	if err != nil {
		log.Printf("generateTitleAsync: failed to list messages for %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}

	// Build message payload for the AI service
	msgPayload := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		m := map[string]any{
			"role": msg.Role,
		}
		if msg.Parts != nil {
			m["parts"] = msg.Parts
		}
		msgPayload = append(msgPayload, m)
	}

	// Call AI service to generate title
	title, err := s.aiClient.GenerateTitle(ctx, msgPayload)
	if err != nil {
		log.Printf("generateTitleAsync: AI service call failed for %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}

	if title == "" {
		title = "新对话"
	}

	if err := s.conversationRepo.UpdateTitle(ctx, id, userID, title); err != nil {
		log.Printf("generateTitleAsync: failed to update title for %s: %v", id, err)
		_ = s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "failed")
		return
	}
	if err := s.conversationRepo.UpdateTitleStatus(ctx, id, userID, "generated"); err != nil {
		log.Printf("generateTitleAsync: failed to update title status for %s: %v", id, err)
	}
}
