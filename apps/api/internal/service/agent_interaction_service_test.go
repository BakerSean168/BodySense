package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeInteractionRepo struct {
	byID map[uuid.UUID]*model.AgentInteraction
}

func newFakeInteractionRepo() *fakeInteractionRepo {
	return &fakeInteractionRepo{byID: map[uuid.UUID]*model.AgentInteraction{}}
}

func (r *fakeInteractionRepo) CreatePending(_ context.Context, interaction *model.AgentInteraction) error {
	for _, existing := range r.byID {
		if existing.RunID == interaction.RunID && existing.ToolCallID == interaction.ToolCallID {
			return nil
		}
	}
	if interaction.ID == uuid.Nil {
		interaction.ID = uuid.New()
	}
	copied := *interaction
	r.byID[interaction.ID] = &copied
	return nil
}

func (r *fakeInteractionRepo) GetByID(_ context.Context, id uuid.UUID) (*model.AgentInteraction, error) {
	if interaction, ok := r.byID[id]; ok {
		copied := *interaction
		return &copied, nil
	}
	return nil, nil
}

func (r *fakeInteractionRepo) GetByRunAndToolCall(_ context.Context, runID uuid.UUID, toolCallID string) (*model.AgentInteraction, error) {
	for _, interaction := range r.byID {
		if interaction.RunID == runID && interaction.ToolCallID == toolCallID {
			copied := *interaction
			return &copied, nil
		}
	}
	return nil, nil
}

func (r *fakeInteractionRepo) MarkAnswered(_ context.Context, id uuid.UUID, answer any) (bool, error) {
	interaction, ok := r.byID[id]
	if !ok || interaction.Status != model.AgentInteractionPending {
		return false, nil
	}
	interaction.Status = model.AgentInteractionAnswered
	interaction.Answer = answer.(datatypes.JSON)
	return true, nil
}

func (r *fakeInteractionRepo) CancelPending(_ context.Context, id uuid.UUID) (bool, error) {
	interaction, ok := r.byID[id]
	if !ok || interaction.Status != model.AgentInteractionPending {
		return false, nil
	}
	interaction.Status = model.AgentInteractionCancelled
	return true, nil
}

func (r *fakeInteractionRepo) ExpirePending(_ context.Context, id uuid.UUID) (bool, error) {
	item, ok := r.byID[id]
	if !ok || item.Status != model.AgentInteractionPending {
		return false, nil
	}
	item.Status = model.AgentInteractionExpired
	return true, nil
}

func (r *fakeInteractionRepo) ListExpiredPending(_ context.Context, now time.Time, limit int) ([]model.AgentInteraction, error) {
	var out []model.AgentInteraction
	for _, item := range r.byID {
		if item.Status == "pending" && item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
			out = append(out, *item)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *fakeInteractionRepo) ListPendingByConversation(_ context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error) {
	var interactions []model.AgentInteraction
	for _, interaction := range r.byID {
		if interaction.ConversationID == conversationID && interaction.Status == model.AgentInteractionPending {
			interactions = append(interactions, *interaction)
		}
	}
	return interactions, nil
}

func (r *fakeInteractionRepo) AggregateInteractionMetrics(_ context.Context, _ uuid.UUID, conversationID *uuid.UUID) (answered, expired, pending int, avgWaitSeconds float64, err error) {
	for _, item := range r.byID {
		if conversationID != nil && item.ConversationID != *conversationID {
			continue
		}
		switch item.Status {
		case model.AgentInteractionAnswered:
			answered++
		case model.AgentInteractionExpired:
			expired++
		case model.AgentInteractionPending:
			pending++
		}
	}
	return answered, expired, pending, 0, nil
}

type fakeConversationOwnership struct {
	byID map[uuid.UUID]*model.Conversation
}

func newFakeConversationOwnership() *fakeConversationOwnership {
	return &fakeConversationOwnership{byID: map[uuid.UUID]*model.Conversation{}}
}

func (o *fakeConversationOwnership) Create(_ context.Context, conversation *model.Conversation) error {
	o.byID[conversation.ID] = conversation
	return nil
}

func (o *fakeConversationOwnership) GetByID(_ context.Context, id, userID uuid.UUID) (*model.Conversation, error) {
	conversation, ok := o.byID[id]
	if !ok || conversation.UserID != userID {
		return nil, nil
	}
	copied := *conversation
	return &copied, nil
}

func (o *fakeConversationOwnership) SoftDelete(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (o *fakeConversationOwnership) GetLastEmptyConversation(_ context.Context, _ uuid.UUID) (*model.Conversation, error) {
	return nil, nil
}

type fakeRunStatusRepo struct {
	lastRunID uuid.UUID
	last      model.RunStatus
	err       error
}

func (r *fakeRunStatusRepo) MarkWaitingUser(_ context.Context, id uuid.UUID) error {
	r.lastRunID = id
	r.last = model.RunStatusWaitingUser
	return r.err
}
func (r *fakeRunStatusRepo) FailWaitingUser(_ context.Context, id uuid.UUID, _ any) (bool, error) {
	r.lastRunID = id
	r.last = model.RunStatusFailed
	return true, r.err
}

type fakeInteractionTransactionManager struct {
	err error
}

func (m fakeInteractionTransactionManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(ctx)
}

func TestAgentInteractionServiceCreatePendingReturnsDurableInteraction(t *testing.T) {
	repo := newFakeInteractionRepo()
	runRepo := &fakeRunStatusRepo{}
	svc := NewAgentInteractionService(repo, runRepo, newFakeConversationOwnership(), fakeInteractionTransactionManager{})
	runID := uuid.New()
	conversationID := uuid.New()

	interaction, err := svc.CreatePendingInteraction(context.Background(), runID, conversationID, "call-1", datatypes.JSON(`{"question":"疼吗？"}`))
	if err != nil {
		t.Fatalf("CreatePendingInteraction: %v", err)
	}
	if interaction.ID == uuid.Nil {
		t.Fatal("expected durable interaction ID")
	}
	if interaction.ToolCallID != "call-1" {
		t.Errorf("tool call id = %q, want call-1", interaction.ToolCallID)
	}
	if runRepo.last != "waiting_user" || runRepo.lastRunID != runID {
		t.Errorf("run status = %q for %s, want waiting_user for %s", runRepo.last, runRepo.lastRunID, runID)
	}
}

func TestAgentInteractionServiceResumeIsIdempotentForSameAnswer(t *testing.T) {
	repo := newFakeInteractionRepo()
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(), fakeInteractionTransactionManager{})
	interaction, err := svc.CreatePendingInteraction(context.Background(), uuid.New(), uuid.New(), "call-1", datatypes.JSON(`{}`))
	if err != nil {
		t.Fatalf("CreatePendingInteraction: %v", err)
	}
	answer := datatypes.JSON(`{"text":"可以"}`)

	if err := svc.ResumeInteraction(context.Background(), interaction.ID, answer); err != nil {
		t.Fatalf("first resume: %v", err)
	}
	if err := svc.ResumeInteraction(context.Background(), interaction.ID, answer); err != nil {
		t.Fatalf("second resume with same answer: %v", err)
	}
}

func TestAgentInteractionServiceResumeRejectsDifferentAnswer(t *testing.T) {
	repo := newFakeInteractionRepo()
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(), fakeInteractionTransactionManager{})
	interaction, err := svc.CreatePendingInteraction(context.Background(), uuid.New(), uuid.New(), "call-1", datatypes.JSON(`{}`))
	if err != nil {
		t.Fatalf("CreatePendingInteraction: %v", err)
	}

	if err := svc.ResumeInteraction(context.Background(), interaction.ID, datatypes.JSON(`{"text":"A"}`)); err != nil {
		t.Fatalf("first resume: %v", err)
	}
	err = svc.ResumeInteraction(context.Background(), interaction.ID, datatypes.JSON(`{"text":"B"}`))
	if !errors.Is(err, ErrInteractionConflict) {
		t.Fatalf("expected ErrInteractionConflict, got %v", err)
	}
}

func TestAgentInteractionServiceCancelPending(t *testing.T) {
	repo := newFakeInteractionRepo()
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(), fakeInteractionTransactionManager{})
	interaction, err := svc.CreatePendingInteraction(context.Background(), uuid.New(), uuid.New(), "call-1", datatypes.JSON(`{}`))
	if err != nil {
		t.Fatalf("CreatePendingInteraction: %v", err)
	}

	if err := svc.CancelInteraction(context.Background(), interaction.ID); err != nil {
		t.Fatalf("CancelInteraction: %v", err)
	}
	cancelled, err := repo.GetByID(context.Background(), interaction.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if cancelled.Status != model.AgentInteractionCancelled {
		t.Errorf("status = %q, want cancelled", cancelled.Status)
	}
}

func TestAgentInteractionServiceResumeRejectsExpired(t *testing.T) {
	repo := newFakeInteractionRepo()
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(), fakeInteractionTransactionManager{})
	interaction, err := svc.CreatePendingInteraction(context.Background(), uuid.New(), uuid.New(), "call-exp", datatypes.JSON(`{}`))
	if err != nil {
		t.Fatalf("CreatePendingInteraction: %v", err)
	}
	past := time.Now().UTC().Add(-time.Hour)
	interaction.ExpiresAt = &past
	repo.byID[interaction.ID].ExpiresAt = &past

	err = svc.ResumeInteraction(context.Background(), interaction.ID, datatypes.JSON(`{"text":"late"}`))
	if !errors.Is(err, ErrInteractionExpired) {
		t.Fatalf("expected ErrInteractionExpired, got %v", err)
	}
}

func TestAgentInteractionServiceExpireSweep(t *testing.T) {
	repo := newFakeInteractionRepo()
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(), fakeInteractionTransactionManager{})
	interaction, err := svc.CreatePendingInteraction(context.Background(), uuid.New(), uuid.New(), "call-sweep", datatypes.JSON(`{}`))
	if err != nil {
		t.Fatalf("CreatePendingInteraction: %v", err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	repo.byID[interaction.ID].ExpiresAt = &past

	expired, err := svc.ExpireExpiredInteractions(context.Background(), 10)
	if err != nil {
		t.Fatalf("ExpireExpiredInteractions: %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired, got %d", len(expired))
	}
	if repo.byID[interaction.ID].Status != model.AgentInteractionExpired {
		t.Fatalf("expected status expired, got %s", repo.byID[interaction.ID].Status)
	}
}

func TestAgentInteractionServiceGetInteractionMetricsDeniedForForeignConversation(t *testing.T) {
	repo := newFakeInteractionRepo()
	owner := newFakeConversationOwnership()
	userID := uuid.New()
	otherUserID := uuid.New()
	conversationID := uuid.New()
	owner.byID[conversationID] = &model.Conversation{ID: conversationID, UserID: otherUserID}
	if err := repo.CreatePending(context.Background(), &model.AgentInteraction{ID: uuid.New(), RunID: uuid.New(), ConversationID: conversationID, ToolCallID: "call-1", Status: model.AgentInteractionAnswered}); err != nil {
		t.Fatalf("seed interaction: %v", err)
	}

	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, owner, fakeInteractionTransactionManager{})
	_, err := svc.GetInteractionMetrics(context.Background(), userID, &conversationID)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected ErrConversationNotFound, got %v", err)
	}
}

func TestAgentInteractionServiceGetInteractionMetricsOwned(t *testing.T) {
	repo := newFakeInteractionRepo()
	owner := newFakeConversationOwnership()
	userID := uuid.New()
	conversationID := uuid.New()
	owner.byID[conversationID] = &model.Conversation{ID: conversationID, UserID: userID}
	for _, status := range []string{"answered", "expired", "pending"} {
		if err := repo.CreatePending(context.Background(), &model.AgentInteraction{ID: uuid.New(), RunID: uuid.New(), ConversationID: conversationID, ToolCallID: "call-" + status, Status: model.AgentInteractionStatus(status)}); err != nil {
			t.Fatalf("seed interaction %s: %v", status, err)
		}
	}

	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, owner, fakeInteractionTransactionManager{})
	metrics, err := svc.GetInteractionMetrics(context.Background(), userID, &conversationID)
	if err != nil {
		t.Fatalf("GetInteractionMetrics: %v", err)
	}
	if metrics.Answered != 1 || metrics.Expired != 1 || metrics.Pending != 1 {
		t.Fatalf("metrics = %+v, want 1 answered, 1 expired, 1 pending", metrics)
	}
}

func TestAgentInteractionServiceGetInteractionMetricsRejectsMissingConversation(t *testing.T) {
	repo := newFakeInteractionRepo()
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(), fakeInteractionTransactionManager{})
	conversationID := uuid.New()
	_, err := svc.GetInteractionMetrics(context.Background(), uuid.New(), &conversationID)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected ErrConversationNotFound, got %v", err)
	}
}

type fakeInteractionLifecycleEvents struct {
	events []dto.StreamEvent
}

func (f *fakeInteractionLifecycleEvents) Flush(context.Context) error { return nil }

func (f *fakeInteractionLifecycleEvents) PersistPreparedMilestone(
	_ context.Context,
	_, _ uuid.UUID,
	_ *uuid.UUID,
	event dto.StreamEvent,
) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeInteractionLifecycleEvents) PersistOutOfBandMilestone(
	_ context.Context,
	conversationID, runID uuid.UUID,
	_ *uuid.UUID,
	channel, eventType string,
	ids dto.StreamEventIDs,
	payload any,
) (dto.StreamEvent, error) {
	event, err := dto.NewStreamEvent(len(f.events)+1, channel, eventType, ids, payload)
	if err != nil {
		return dto.StreamEvent{}, err
	}
	if event.IDs.ConversationID == "" {
		event.IDs.ConversationID = conversationID.String()
	}
	if event.IDs.RunID == "" {
		event.IDs.RunID = runID.String()
	}
	f.events = append(f.events, event)
	return event, nil
}

func TestCreatePendingInteractionWithEventsPersistsRequiredAndInterrupted(t *testing.T) {
	repo := newFakeInteractionRepo()
	runRepo := &fakeRunStatusRepo{}
	events := &fakeInteractionLifecycleEvents{}
	svc := NewAgentInteractionService(repo, runRepo, newFakeConversationOwnership(), fakeInteractionTransactionManager{}).
		WithLifecycleEvents(events)
	runID := uuid.New()
	conversationID := uuid.New()
	turnID := uuid.New()

	interaction, prepared, err := svc.CreatePendingInteractionWithEvents(
		context.Background(), runID, conversationID, "call-required", datatypes.JSON(`{"prompt":"where?"}`),
		func(interaction *model.AgentInteraction) ([]dto.StreamEvent, error) {
			required, err := dto.NewStreamEvent(5, "state", "state.interaction.required", dto.StreamEventIDs{
				ConversationID: conversationID.String(), RunID: runID.String(), TurnID: turnID.String(), InteractionID: interaction.ID.String(), ToolCallID: interaction.ToolCallID,
			}, map[string]any{"interaction_id": interaction.ID.String(), "question": map[string]any{"prompt": "where?"}, "created_at": interaction.CreatedAt.Format(time.RFC3339Nano)})
			if err != nil {
				return nil, err
			}
			interrupted, err := dto.NewStreamEvent(6, "run", "run.interrupted", dto.StreamEventIDs{
				ConversationID: conversationID.String(), RunID: runID.String(), TurnID: turnID.String(), InteractionID: interaction.ID.String(),
			}, map[string]any{"status": "waiting_user", "interaction_id": interaction.ID.String()})
			return []dto.StreamEvent{required, interrupted}, err
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if interaction == nil || interaction.Status != model.AgentInteractionPending {
		t.Fatalf("interaction=%+v, want pending", interaction)
	}
	if runRepo.last != model.RunStatusWaitingUser {
		t.Fatalf("run status=%q, want waiting_user", runRepo.last)
	}
	if len(prepared) != 2 || len(events.events) != 2 {
		t.Fatalf("prepared=%d durable=%d, want 2/2", len(prepared), len(events.events))
	}
	if events.events[0].Type != "state.interaction.required" || events.events[1].Type != "run.interrupted" {
		t.Fatalf("unexpected event order: %q, %q", events.events[0].Type, events.events[1].Type)
	}
}

func TestResumeInteractionWithEventPersistsAnsweredOnSourceRun(t *testing.T) {
	repo := newFakeInteractionRepo()
	runRepo := &fakeRunStatusRepo{}
	events := &fakeInteractionLifecycleEvents{}
	svc := NewAgentInteractionService(repo, runRepo, newFakeConversationOwnership(), fakeInteractionTransactionManager{}).
		WithLifecycleEvents(events)
	runID := uuid.New()
	conversationID := uuid.New()
	interaction, err := svc.CreatePendingInteraction(context.Background(), runID, conversationID, "call-answer", datatypes.JSON(`{"prompt":"pain?"}`))
	if err != nil {
		t.Fatal(err)
	}
	answer := datatypes.JSON(`{"text":"yes"}`)
	if err := svc.ResumeInteractionWithEvent(context.Background(), interaction.ID, answer); err != nil {
		t.Fatal(err)
	}
	stored, _ := repo.GetByID(context.Background(), interaction.ID)
	if stored == nil || stored.Status != model.AgentInteractionAnswered {
		t.Fatalf("stored=%+v, want answered", stored)
	}
	if len(events.events) != 1 || events.events[0].Type != "state.interaction.answered" {
		t.Fatalf("events=%+v, want one interaction.answered", events.events)
	}
	if events.events[0].IDs.RunID != runID.String() {
		t.Fatalf("answered event run=%q, want source run %s", events.events[0].IDs.RunID, runID)
	}
}

func TestExpireInteractionWithEventsFailsWaitingRunAndPersistsBothMilestones(t *testing.T) {
	repo := newFakeInteractionRepo()
	runRepo := &fakeRunStatusRepo{}
	events := &fakeInteractionLifecycleEvents{}
	svc := NewAgentInteractionService(repo, runRepo, newFakeConversationOwnership(), fakeInteractionTransactionManager{}).
		WithLifecycleEvents(events)
	runID := uuid.New()
	conversationID := uuid.New()
	interaction, err := svc.CreatePendingInteraction(context.Background(), runID, conversationID, "call-expire", datatypes.JSON(`{"prompt":"later?"}`))
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	repo.byID[interaction.ID].ExpiresAt = &past

	expired, err := svc.ExpireExpiredInteractions(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || runRepo.last != model.RunStatusFailed {
		t.Fatalf("expired=%d run=%q, want 1/failed", len(expired), runRepo.last)
	}
	if len(events.events) != 2 || events.events[0].Type != "state.interaction.expired" || events.events[1].Type != "run.failed" {
		t.Fatalf("unexpected expiry events: %+v", events.events)
	}
}
