package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrInteractionNotFound = errors.New("interaction not found")
	ErrInteractionConflict = errors.New("interaction answer conflicts with existing answer")
	ErrInteractionClosed   = errors.New("interaction is not pending")
	ErrInteractionExpired  = errors.New("interaction has expired")
	ErrConversationNotFound = errors.New("conversation not found or access denied")
)

// DefaultInteractionTTL is how long a pending ask_user waits before auto-expiry.
const DefaultInteractionTTL = 24 * time.Hour

// AgentInteractionService handles user interaction persistence and resume.
type AgentInteractionService struct {
	repo            agentInteractionRepo
	runRepo         runStatusRepo
	conversationRepo conversationOwnershipChecker
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

type runStatusRepo interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// NewAgentInteractionService creates a new AgentInteractionService.
// conversationOwnershipChecker proves that a conversation belongs to a user before
// interaction data (including metrics) is exposed.
func NewAgentInteractionService(
	repo agentInteractionRepo,
	runRepo runStatusRepo,
	conversationRepo conversationOwnershipChecker,
) *AgentInteractionService {
	return &AgentInteractionService{
		repo:            repo,
		runRepo:         runRepo,
		conversationRepo: conversationRepo,
	}
}

// CreatePendingInteraction creates a pending interaction from an ask_user event.
func (s *AgentInteractionService) CreatePendingInteraction(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
) (*model.AgentInteraction, error) {
	expires := time.Now().UTC().Add(DefaultInteractionTTL)
	interaction := &model.AgentInteraction{
		RunID:          runID,
		ConversationID: conversationID,
		ToolCallID:     toolCallID,
		ToolName:       "ask_user",
		Question:       question,
		Status:         "pending",
		ExpiresAt:      &expires,
	}
	if err := s.repo.CreatePending(ctx, interaction); err != nil {
		return nil, fmt.Errorf("create pending interaction: %w", err)
	}
	created, err := s.repo.GetByRunAndToolCall(ctx, runID, toolCallID)
	if err != nil {
		return nil, fmt.Errorf("load pending interaction: %w", err)
	}
	if created == nil {
		return nil, fmt.Errorf("load pending interaction: %w", ErrInteractionNotFound)
	}

	if err := s.runRepo.UpdateStatus(ctx, runID, "waiting_user"); err != nil {
		return nil, fmt.Errorf("mark run waiting_user: %w", err)
	}

	return created, nil
}

// ResumeInteraction marks an interaction as answered. It is idempotent for
// repeated submissions with the same answer and rejects conflicting answers.
func (s *AgentInteractionService) ResumeInteraction(
	ctx context.Context,
	interactionID uuid.UUID,
	answer datatypes.JSON,
) error {
	interaction, err := s.repo.GetByID(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("get interaction: %w", err)
	}
	if interaction == nil {
		return ErrInteractionNotFound
	}
	if interaction.Status == "answered" {
		if jsonEqual(interaction.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}
	if interaction.Status == "expired" {
		return ErrInteractionExpired
	}
	if interaction.Status != "pending" {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}
	// Soft-expire if the TTL elapsed but the sweeper has not yet run.
	if interaction.ExpiresAt != nil && !interaction.ExpiresAt.After(time.Now().UTC()) {
		if _, expErr := s.repo.ExpirePending(ctx, interactionID); expErr != nil {
			log.Printf("failed to expire interaction %s: %v", interactionID, expErr)
		}
		return ErrInteractionExpired
	}

	updated, err := s.repo.MarkAnswered(ctx, interactionID, answer)
	if err != nil {
		return fmt.Errorf("mark answered: %w", err)
	}
	if !updated {
		latest, latestErr := s.repo.GetByID(ctx, interactionID)
		if latestErr != nil {
			return fmt.Errorf("reload interaction after answer race: %w", latestErr)
		}
		if latest != nil && latest.Status == "answered" && jsonEqual(latest.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}

	return nil
}

// CancelInteraction marks a pending interaction as cancelled.
func (s *AgentInteractionService) CancelInteraction(ctx context.Context, interactionID uuid.UUID) error {
	interaction, err := s.repo.GetByID(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("get interaction: %w", err)
	}
	if interaction == nil {
		return ErrInteractionNotFound
	}
	if interaction.Status == "cancelled" {
		return nil
	}
	if interaction.Status != "pending" {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}
	updated, err := s.repo.CancelPending(ctx, interactionID)
	if err != nil {
		return fmt.Errorf("cancel interaction: %w", err)
	}
	if !updated {
		return ErrInteractionClosed
	}
	return nil
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

// ExpireExpiredInteractions marks due pending interactions as expired.
// Returns the interactions that transitioned to expired (for event emission).
func (s *AgentInteractionService) ExpireExpiredInteractions(
	ctx context.Context,
	limit int,
) ([]model.AgentInteraction, error) {
	due, err := s.repo.ListExpiredPending(ctx, time.Now().UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list expired interactions: %w", err)
	}
	expired := make([]model.AgentInteraction, 0, len(due))
	for _, item := range due {
		updated, expErr := s.repo.ExpirePending(ctx, item.ID)
		if expErr != nil {
			log.Printf("expire interaction %s: %v", item.ID, expErr)
			continue
		}
		if updated {
			item.Status = "expired"
			expired = append(expired, item)
		}
	}
	return expired, nil
}

// InteractionExpiredHandler is called after an interaction is marked expired.
// Typically records state.interaction.expired into the durable event log.
type InteractionExpiredHandler func(ctx context.Context, interaction model.AgentInteraction)

// StartInteractionExpiryWorker periodically sweeps expired pending interactions.
// onExpired is optional; when set it receives each newly expired interaction.
func (s *AgentInteractionService) StartInteractionExpiryWorker(
	ctx context.Context,
	interval time.Duration,
	onExpired InteractionExpiredHandler,
) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				expired, err := s.ExpireExpiredInteractions(ctx, 100)
				if err != nil {
					log.Printf("interaction expiry sweep failed: %v", err)
					continue
				}
				if len(expired) == 0 {
					continue
				}
				log.Printf("interaction expiry sweep: expired %d pending interaction(s)", len(expired))
				if onExpired == nil {
					continue
				}
				for _, item := range expired {
					onExpired(ctx, item)
				}
			}
		}
	}()
}

// InteractionMetrics is a lightweight projection over agent_interactions
// (T0-1 Phase C). Computed on read from the source table — no separate metrics table.
type InteractionMetrics struct {
	Total          int     `json:"total"`
	Answered       int     `json:"answered"`
	Expired        int     `json:"expired"`
	Pending        int     `json:"pending"`
	AnswerRate     float64 `json:"answer_rate"`
	ExpireRate     float64 `json:"expire_rate"`
	AvgWaitSeconds float64 `json:"avg_wait_seconds"`
}

// GetInteractionMetrics returns answer/expire rates and average wait time.
// conversationID nil => all of the user's conversations; otherwise scoped to one
// conversation. Ownership of the conversation is proven before any data is read.
func (s *AgentInteractionService) GetInteractionMetrics(
	ctx context.Context,
	userID uuid.UUID,
	conversationID *uuid.UUID,
) (InteractionMetrics, error) {
	if conversationID != nil && *conversationID != uuid.Nil {
		conversation, err := s.conversationRepo.GetByID(ctx, *conversationID, userID)
		if err != nil {
			return InteractionMetrics{}, fmt.Errorf("verify interaction metrics ownership: %w", err)
		}
		if conversation == nil {
			return InteractionMetrics{}, ErrConversationNotFound
		}
	}
	answered, expired, pending, avgWait, err := s.repo.AggregateInteractionMetrics(ctx, userID, conversationID)
	if err != nil {
		return InteractionMetrics{}, err
	}
	total := answered + expired + pending
	m := InteractionMetrics{
		Total:          total,
		Answered:       answered,
		Expired:        expired,
		Pending:        pending,
		AvgWaitSeconds: avgWait,
	}
	closed := answered + expired
	if closed > 0 {
		m.AnswerRate = float64(answered) / float64(closed)
		m.ExpireRate = float64(expired) / float64(closed)
	}
	return m, nil
}
