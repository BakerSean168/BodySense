package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// ShareService handles conversation sharing business logic.
type ShareService struct {
	conversationRepo conversationRepo
	messageRepo      messageRepo
	shareRepo        shareRepo
}

// NewShareService creates a new ShareService.
func NewShareService(
	conversationRepo conversationRepo,
	messageRepo messageRepo,
	shareRepo shareRepo,
) *ShareService {
	return &ShareService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		shareRepo:        shareRepo,
	}
}

// ShareConversation creates a public share snapshot of a conversation.
// It loads the conversation messages, creates a snapshot, generates a share_token,
// and returns the share record and the share URL.
func (s *ShareService) ShareConversation(ctx context.Context, conversationID, userID uuid.UUID) (*model.ConversationShare, string, error) {
	// Verify ownership
	conversation, err := s.conversationRepo.GetByID(ctx, conversationID, userID)
	if err != nil {
		return nil, "", fmt.Errorf("get conversation for share: %w", err)
	}
	if conversation == nil {
		return nil, "", fmt.Errorf("conversation not found: %s", conversationID)
	}

	// Load messages
	messages, err := s.messageRepo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, "", fmt.Errorf("load messages for share: %w", err)
	}

	// Create snapshot JSON
	snapshotMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, "", fmt.Errorf("marshal snapshot messages: %w", err)
	}

	// Generate share token
	token, err := generateShareToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate share token: %w", err)
	}

	share := &model.ConversationShare{
		ID:               uuid.New(),
		ConversationID:   conversationID,
		ShareToken:       token,
		SnapshotMessages: snapshotMessages,
		SnapshotTitle:    conversation.Title,
	}

	if err := s.shareRepo.Create(ctx, share); err != nil {
		return nil, "", fmt.Errorf("create share: %w", err)
	}

	shareURL := fmt.Sprintf("/consultation/share/%s", token)
	return share, shareURL, nil
}

// UnshareConversation removes the share for a conversation.
func (s *ShareService) UnshareConversation(ctx context.Context, conversationID, userID uuid.UUID) error {
	// Verify ownership
	conversation, err := s.conversationRepo.GetByID(ctx, conversationID, userID)
	if err != nil {
		return fmt.Errorf("get conversation for unshare: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found: %s", conversationID)
	}

	if err := s.shareRepo.DeleteByConversationID(ctx, conversationID); err != nil {
		return fmt.Errorf("delete share: %w", err)
	}
	return nil
}

// GetSharedConversation retrieves a shared conversation by its token.
func (s *ShareService) GetSharedConversation(ctx context.Context, token string) (*model.ConversationShare, error) {
	share, err := s.shareRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("get share by token: %w", err)
	}
	if share == nil {
		return nil, nil
	}
	return share, nil
}

// generateShareToken generates a cryptographically secure 32-character hex string.
func generateShareToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
