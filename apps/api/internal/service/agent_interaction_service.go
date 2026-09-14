package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// CreatePendingInteraction creates a pending interaction from an ask_user event.
// Tests and non-stream callers may use this state-only form; the production
// consultation stream uses CreatePendingInteractionWithEvents so the public
// lifecycle milestones share the same transaction.
func (s *AgentInteractionService) CreatePendingInteraction(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
) (*model.AgentInteraction, error) {
	interaction, _, err := s.createPendingInteraction(ctx, runID, conversationID, toolCallID, question, nil)
	return interaction, err
}

// CreatePendingInteractionWithEvents commits pending interaction state,
// running->waiting_user and the prepared interaction.required/run.interrupted
// milestones as one unit. Returned events are already durable and must only be
// written to SSE after commit.
func (s *AgentInteractionService) CreatePendingInteractionWithEvents(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
	prepare interactionPreparedEventsBuilder,
) (*model.AgentInteraction, []dto.StreamEvent, error) {
	if prepare == nil {
		return nil, nil, errors.New("create pending interaction: prepared event builder is required")
	}
	return s.createPendingInteraction(ctx, runID, conversationID, toolCallID, question, prepare)
}

func (s *AgentInteractionService) createPendingInteraction(
	ctx context.Context,
	runID, conversationID uuid.UUID,
	toolCallID string,
	question datatypes.JSON,
	prepare interactionPreparedEventsBuilder,
) (*model.AgentInteraction, []dto.StreamEvent, error) {
	if s.transactions == nil {
		return nil, nil, errors.New("create pending interaction: transaction manager is not configured")
	}
	if prepare != nil {
		if s.lifecycleEvents == nil {
			return nil, nil, errors.New("create pending interaction: lifecycle event store is not configured")
		}
		if err := s.lifecycleEvents.Flush(ctx); err != nil {
			return nil, nil, fmt.Errorf("flush runtime events before interaction required: %w", err)
		}
	}
	now := time.Now().UTC()
	expires := now.Add(DefaultInteractionTTL)
	interaction := &model.AgentInteraction{
		ID:             uuid.New(),
		RunID:          runID,
		ConversationID: conversationID,
		ToolCallID:     toolCallID,
		ToolName:       "ask_user",
		Question:       question,
		Status:         model.AgentInteractionPending,
		CreatedAt:      now,
		ExpiresAt:      &expires,
	}

	var created *model.AgentInteraction
	var prepared []dto.StreamEvent
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreatePending(txCtx, interaction); err != nil {
			return fmt.Errorf("create pending interaction: %w", err)
		}
		loaded, err := s.repo.GetByRunAndToolCall(txCtx, runID, toolCallID)
		if err != nil {
			return fmt.Errorf("load pending interaction: %w", err)
		}
		if loaded == nil {
			return fmt.Errorf("load pending interaction: %w", ErrInteractionNotFound)
		}
		created = loaded
		if err := s.runLifecycle.MarkWaitingUser(txCtx, runID); err != nil {
			return fmt.Errorf("mark run waiting_user: %w", err)
		}
		if prepare == nil {
			return nil
		}
		prepared, err = prepare(created)
		if err != nil {
			return fmt.Errorf("prepare interaction lifecycle events: %w", err)
		}
		for _, event := range prepared {
			var turnID *uuid.UUID
			if event.IDs.TurnID != "" {
				parsed, parseErr := uuid.Parse(event.IDs.TurnID)
				if parseErr != nil {
					return fmt.Errorf("parse interaction lifecycle turn id: %w", parseErr)
				}
				turnID = &parsed
			}
			if err := s.lifecycleEvents.PersistPreparedMilestone(txCtx, conversationID, runID, turnID, event); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return created, prepared, nil
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
	if interaction.Status == model.AgentInteractionAnswered {
		if jsonEqual(interaction.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}
	if interaction.Status == model.AgentInteractionExpired {
		return ErrInteractionExpired
	}
	if interaction.Status != model.AgentInteractionPending {
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
		if latest != nil && latest.Status == model.AgentInteractionAnswered && jsonEqual(latest.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}

	return nil
}

// ResumeInteractionWithEvent is the production answer transition. The
// pending->answered CAS and state.interaction.answered milestone share one
// transaction on the original interrupted run.
func (s *AgentInteractionService) ResumeInteractionWithEvent(
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
	if interaction.Status == model.AgentInteractionAnswered {
		if jsonEqual(interaction.Answer, answer) {
			return nil
		}
		return ErrInteractionConflict
	}
	if interaction.Status == model.AgentInteractionExpired {
		return ErrInteractionExpired
	}
	if interaction.Status != model.AgentInteractionPending {
		return fmt.Errorf("%w: %s", ErrInteractionClosed, interaction.Status)
	}
	if interaction.ExpiresAt != nil && !interaction.ExpiresAt.After(time.Now().UTC()) {
		if _, expErr := s.expireInteractionWithEvent(ctx, *interaction); expErr != nil {
			return fmt.Errorf("expire overdue interaction: %w", expErr)
		}
		return ErrInteractionExpired
	}
	if s.transactions == nil || s.lifecycleEvents == nil {
		return errors.New("answer interaction: lifecycle coordinator is not configured")
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return fmt.Errorf("flush runtime events before interaction answer: %w", err)
	}

	updated := false
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.repo.MarkAnswered(txCtx, interactionID, answer)
		if err != nil {
			return fmt.Errorf("mark answered: %w", err)
		}
		if !won {
			return nil
		}
		updated = true
		_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
			txCtx,
			interaction.ConversationID,
			interaction.RunID,
			nil,
			"state",
			"state.interaction.answered",
			dto.StreamEventIDs{
				ConversationID: interaction.ConversationID.String(),
				RunID:          interaction.RunID.String(),
				InteractionID:  interaction.ID.String(),
				ToolCallID:     interaction.ToolCallID,
			},
			map[string]any{
				"interaction_id": interaction.ID.String(),
				"answer":         json.RawMessage(answer),
			},
		)
		return err
	})
	if err != nil {
		return fmt.Errorf("answer interaction with lifecycle event: %w", err)
	}
	if updated {
		return nil
	}
	latest, latestErr := s.repo.GetByID(ctx, interactionID)
	if latestErr != nil {
		return fmt.Errorf("reload interaction after answer race: %w", latestErr)
	}
	if latest != nil && latest.Status == model.AgentInteractionAnswered && jsonEqual(latest.Answer, answer) {
		return nil
	}
	return ErrInteractionConflict
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
	if interaction.Status == model.AgentInteractionCancelled {
		return nil
	}
	if interaction.Status != model.AgentInteractionPending {
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

// ExpireExpiredInteractions closes due pending interactions. When lifecycle
// events are configured (production), interaction.expired and the waiting Run's
// run.failed transition are committed atomically. Tests may still exercise the
// repository-only form by leaving the event store unset.
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
		var updated bool
		var expErr error
		if s.lifecycleEvents != nil {
			updated, expErr = s.expireInteractionWithEvent(ctx, item)
		} else {
			updated, expErr = s.repo.ExpirePending(ctx, item.ID)
		}
		if expErr != nil {
			log.Printf("expire interaction %s: %v", item.ID, expErr)
			continue
		}
		if updated {
			item.Status = model.AgentInteractionExpired
			expired = append(expired, item)
		}
	}
	return expired, nil
}

func (s *AgentInteractionService) expireInteractionWithEvent(ctx context.Context, interaction model.AgentInteraction) (bool, error) {
	if s.transactions == nil || s.lifecycleEvents == nil {
		return false, errors.New("expire interaction: lifecycle coordinator is not configured")
	}
	if err := s.lifecycleEvents.Flush(ctx); err != nil {
		return false, fmt.Errorf("flush runtime events before interaction expiry: %w", err)
	}
	expiredAt := time.Now().UTC()
	updated := false
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		won, err := s.repo.ExpirePending(txCtx, interaction.ID)
		if err != nil {
			return err
		}
		if !won {
			return nil
		}
		updated = true
		runFailed, err := s.runLifecycle.FailWaitingUser(
			txCtx,
			interaction.RunID,
			datatypes.JSON(`{"message":"interaction expired"}`),
		)
		if err != nil {
			return err
		}
		_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
			txCtx,
			interaction.ConversationID,
			interaction.RunID,
			nil,
			"state",
			"state.interaction.expired",
			dto.StreamEventIDs{
				ConversationID: interaction.ConversationID.String(),
				RunID:          interaction.RunID.String(),
				InteractionID:  interaction.ID.String(),
				ToolCallID:     interaction.ToolCallID,
			},
			map[string]any{
				"interaction_id": interaction.ID.String(),
				"expired_at":     expiredAt.Format(time.RFC3339Nano),
				"reason":         "ttl_elapsed",
			},
		)
		if err != nil {
			return err
		}
		if runFailed {
			_, err = s.lifecycleEvents.PersistOutOfBandMilestone(
				txCtx,
				interaction.ConversationID,
				interaction.RunID,
				nil,
				"run",
				"run.failed",
				dto.StreamEventIDs{
					ConversationID: interaction.ConversationID.String(),
					RunID:          interaction.RunID.String(),
					InteractionID:  interaction.ID.String(),
				},
				map[string]any{"status": "failed", "reason": "interaction_expired"},
			)
		}
		return err
	})
	if err != nil {
		return false, fmt.Errorf("expire interaction with lifecycle events: %w", err)
	}
	return updated, nil
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
