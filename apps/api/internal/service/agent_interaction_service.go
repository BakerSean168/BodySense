package service

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrInteractionNotFound  = errors.New("interaction not found")
	ErrInteractionConflict  = errors.New("interaction answer conflicts with existing answer")
	ErrInteractionClosed    = errors.New("interaction is not pending")
	ErrInteractionExpired   = errors.New("interaction has expired")
	ErrConversationNotFound = errors.New("conversation not found or access denied")
)

// DefaultInteractionTTL is how long a pending ask_user waits before auto-expiry.
const DefaultInteractionTTL = 24 * time.Hour

// AgentInteractionService handles user interaction persistence and resume.
type AgentInteractionService struct {
	repo             agentInteractionRepo
	runLifecycle     interactionRunLifecycle
	conversationRepo conversationOwnershipChecker
	transactions     interactionTransactionManager
	lifecycleEvents  interactionLifecycleEventStore
	settle           interactionSettleTrigger
}

// interactionSettleTrigger is the chat retention settle trigger (PC-RET-001).
// An Ask User that closes without an answer (expired or cancelled) is what a
// pending interaction was holding back, so closing it lets retention move.
type interactionSettleTrigger interface {
	OnConversationSettled(ctx context.Context, conversationID uuid.UUID)
}

type agentInteractionRepo interface {
	CreatePending(ctx context.Context, interaction *model.AgentInteraction) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentInteraction, error)
	GetByRunAndToolCall(ctx context.Context, runID uuid.UUID, toolCallID string) (*model.AgentInteraction, error)
	MarkAnswered(ctx context.Context, id uuid.UUID, answer any) (bool, error)
	CancelPending(ctx context.Context, id uuid.UUID) (bool, error)
	ExpirePending(ctx context.Context, id uuid.UUID) (bool, error)
	ListPendingByConversation(ctx context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error)
	ListExpiredPending(ctx context.Context, now time.Time, limit int) ([]model.AgentInteraction, error)
	AggregateInteractionMetrics(ctx context.Context, userID uuid.UUID, conversationID *uuid.UUID) (answered, expired, pending int, avgWaitSeconds float64, err error)
}

type interactionRunLifecycle interface {
	MarkWaitingUser(ctx context.Context, id uuid.UUID) error
	FailWaitingUser(ctx context.Context, id uuid.UUID, errJSON any) (bool, error)
}

type interactionLifecycleEventStore interface {
	Flush(ctx context.Context) error
	PersistPreparedMilestone(ctx context.Context, conversationID, runID uuid.UUID, turnID *uuid.UUID, event dto.StreamEvent) error
	PersistOutOfBandMilestone(ctx context.Context, conversationID, runID uuid.UUID, turnID *uuid.UUID, channel, eventType string, ids dto.StreamEventIDs, payload any) (dto.StreamEvent, error)
}

type interactionPreparedEventsBuilder func(interaction *model.AgentInteraction) ([]dto.StreamEvent, error)

type interactionTransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

// NewAgentInteractionService creates a new AgentInteractionService.
// conversationOwnershipChecker proves that a conversation belongs to a user before
// interaction data (including metrics) is exposed.
func NewAgentInteractionService(
	repo agentInteractionRepo,
	runLifecycle interactionRunLifecycle,
	conversationRepo conversationOwnershipChecker,
	transactions interactionTransactionManager,
) *AgentInteractionService {
	return &AgentInteractionService{
		repo:             repo,
		runLifecycle:     runLifecycle,
		conversationRepo: conversationRepo,
		transactions:     transactions,
	}
}

func (s *AgentInteractionService) WithLifecycleEvents(events interactionLifecycleEventStore) *AgentInteractionService {
	s.lifecycleEvents = events
	return s
}

// WithRetentionSettle wires the chat retention settle trigger. It fires after
// an expiry or cancellation committed, never inside its transaction.
func (s *AgentInteractionService) WithRetentionSettle(trigger interactionSettleTrigger) *AgentInteractionService {
	s.settle = trigger
	return s
}

// settled tells retention that an interaction stopped waiting for the user.
func (s *AgentInteractionService) settled(ctx context.Context, interaction model.AgentInteraction) {
	if s.settle != nil {
		s.settle.OnConversationSettled(ctx, interaction.ConversationID)
	}
}

// GetPendingInteractions returns pending interactions for a conversation.
func (s *AgentInteractionService) GetPendingInteractions(
	ctx context.Context,
	conversationID uuid.UUID,
) ([]model.AgentInteraction, error) {
	return s.repo.ListPendingByConversation(ctx, conversationID)
}

// GetInteractionByID returns an interaction by its ID.
func (s *AgentInteractionService) GetInteractionByID(
	ctx context.Context,
	interactionID uuid.UUID,
) (*model.AgentInteraction, error) {
	return s.repo.GetByID(ctx, interactionID)
}

func jsonEqual(a, b datatypes.JSON) bool {
	a = bytes.TrimSpace(a)
	b = bytes.TrimSpace(b)
	if len(a) == 0 {
		a = []byte("null")
	}
	if len(b) == 0 {
		b = []byte("null")
	}
	return bytes.Equal(a, b)
}
