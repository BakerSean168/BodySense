package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type conversationRepo interface {
	GetOrCreateForUser(ctx context.Context, userID uuid.UUID) (*model.Conversation, bool, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Conversation, error)
	UpdateLastMessageAt(ctx context.Context, id, userID uuid.UUID) error
	UpdateActiveRunID(ctx context.Context, id, userID uuid.UUID, runID *uuid.UUID, streamID string) error
}

type messageRepo interface {
	Create(ctx context.Context, message *model.Message) error
	GetByID(ctx context.Context, id, conversationID uuid.UUID) (*model.Message, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Message, error)
	UpdateStatus(ctx context.Context, id, conversationID uuid.UUID, status string) error
	FailStreamingAssistantByRunID(ctx context.Context, runID, conversationID uuid.UUID, errorPayload any) (*model.Message, error)
	UpdateParts(ctx context.Context, id, conversationID uuid.UUID, parts any) error
	UpdateCompleted(ctx context.Context, id, conversationID uuid.UUID, parts any, usage map[string]any, providerInfo map[string]any) error
	UpdateCompletedWithStatus(ctx context.Context, id, conversationID uuid.UUID, parts any, status string) error
	GetNextSeq(ctx context.Context, conversationID uuid.UUID) (int, error)
}

type runRepo interface {
	Create(ctx context.Context, run *model.Run) error
	CreateWithIdempotency(ctx context.Context, run *model.Run) (*model.Run, bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Run, error)
	GetByRequestID(ctx context.Context, userID uuid.UUID, requestID string) (*model.Run, error)
	ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Run, error)
	MarkWaitingUser(ctx context.Context, id uuid.UUID) (bool, error)
	ResumeRunning(ctx context.Context, id uuid.UUID, owner string, expiresAt time.Time) (bool, error)
	FailWaitingUser(ctx context.Context, id uuid.UUID, errJSON any) (bool, error)
	CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error
	TryCompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) (bool, error)
	CancelRun(ctx context.Context, id, userID uuid.UUID, reason any) (bool, error)
	FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) (bool, error)
	UpdateAgentConfiguration(ctx context.Context, id uuid.UUID, configurationID string, configuration datatypes.JSON, provenance datatypes.JSON) error
	RenewLease(ctx context.Context, id, userID uuid.UUID, owner string, expiresAt, heartbeatAt time.Time) (bool, error)
	ListExpiredRuns(ctx context.Context, now time.Time, limit int) ([]model.Run, error)
	FailExpiredRun(ctx context.Context, id uuid.UUID, now time.Time, errJSON any) (bool, error)
}

// ConversationService handles the user's single Conversation (PC-D4).
type ConversationService struct {
	conversationRepo conversationRepo
	messageRepo      messageRepo
	runRepo          runRepo
	txManager        *database.TransactionManager
}

// NewConversationService creates a new ConversationService.
func NewConversationService(
	conversationRepo conversationRepo,
	messageRepo messageRepo,
	runRepo runRepo,
	txManager *database.TransactionManager,
) *ConversationService {
	return &ConversationService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		runRepo:          runRepo,
		txManager:        txManager,
	}
}

// GetOrCreateConversation returns the user's single Conversation and its
// ordered messages, lazily creating the Conversation on first use. Concurrent
// first calls from several devices converge on the same identity.
func (s *ConversationService) GetOrCreateConversation(ctx context.Context, userID uuid.UUID) (*model.Conversation, []model.Message, error) {
	conversation, _, err := s.conversationRepo.GetOrCreateForUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get or create conversation: %w", err)
	}
	messages, err := s.messageRepo.ListByConversationID(ctx, conversation.ID)
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

// FinalizeExecutionLostProjection atomically converts the durable conversation
// projection for a reclaimed run from "active/streaming" to "failed/inactive".
// ReclaimExpiredRuns decides the terminal winner first; this method makes the
// persisted assistant message and conversation pointer agree before the public
// run.failed event becomes observable to recovering browsers.
func (s *ConversationService) FinalizeExecutionLostProjection(
	ctx context.Context,
	run *model.Run,
) (*model.Message, error) {
	if run == nil {
		return nil, nil
	}
	if s.txManager == nil {
		return nil, errors.New("finalize execution lost: transaction manager is not configured")
	}
	if s.messageRepo == nil || s.conversationRepo == nil {
		return nil, errors.New("finalize execution lost: conversation/message repository is not configured")
	}

	errorPayload := map[string]any{
		"code":    "execution_lost",
		"message": "run execution lost; lease expired",
	}
	var failedMessage *model.Message
	err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		message, err := s.messageRepo.FailStreamingAssistantByRunID(
			txCtx, run.ID, run.ConversationID, errorPayload,
		)
		if err != nil {
			return fmt.Errorf("fail reclaimed assistant message: %w", err)
		}
		failedMessage = message
		if err := s.conversationRepo.UpdateActiveRunID(
			txCtx, run.ConversationID, run.UserID, nil, "",
		); err != nil {
			return fmt.Errorf("clear reclaimed active run: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return failedMessage, nil
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
