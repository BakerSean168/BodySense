package service

import (
	"context"
	"errors"
	"testing"
	"time"

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
	if !ok || interaction.Status != "pending" {
		return false, nil
	}
	interaction.Status = "answered"
	interaction.Answer = answer.(datatypes.JSON)
	return true, nil
}

func (r *fakeInteractionRepo) CancelPending(_ context.Context, id uuid.UUID) (bool, error) {
	interaction, ok := r.byID[id]
	if !ok || interaction.Status != "pending" {
		return false, nil
	}
	interaction.Status = "cancelled"
	return true, nil
}

func (r *fakeInteractionRepo) ExpirePending(_ context.Context, id uuid.UUID) (bool, error) {
	item, ok := r.byID[id]
	if !ok || item.Status != "pending" {
		return false, nil
	}
	item.Status = "expired"
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
		if interaction.ConversationID == conversationID && interaction.Status == "pending" {
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
		case "answered":
			answered++
		case "expired":
			expired++
		case "pending":
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
	last      string
}

func (r *fakeRunStatusRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	r.lastRunID = id
	r.last = status
	return nil
}

func TestAgentInteractionServiceCreatePendingReturnsDurableInteraction(t *testing.T) {
	repo := newFakeInteractionRepo()
	runRepo := &fakeRunStatusRepo{}
	svc := NewAgentInteractionService(repo, runRepo, newFakeConversationOwnership())
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership())
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership())
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership())
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
	if cancelled.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", cancelled.Status)
	}
}

func TestAgentInteractionServiceResumeRejectsExpired(t *testing.T) {
	repo := newFakeInteractionRepo()
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership())
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership())
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
	if repo.byID[interaction.ID].Status != "expired" {
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
	if err := repo.CreatePending(context.Background(), &model.AgentInteraction{ID: uuid.New(), RunID: uuid.New(), ConversationID: conversationID, ToolCallID: "call-1", Status: "answered"}); err != nil {
		t.Fatalf("seed interaction: %v", err)
	}

	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, owner)
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
		if err := repo.CreatePending(context.Background(), &model.AgentInteraction{ID: uuid.New(), RunID: uuid.New(), ConversationID: conversationID, ToolCallID: "call-" + status, Status: status}); err != nil {
			t.Fatalf("seed interaction %s: %v", status, err)
		}
	}

	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, owner)
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership())
	conversationID := uuid.New()
	_, err := svc.GetInteractionMetrics(context.Background(), uuid.New(), &conversationID)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected ErrConversationNotFound, got %v", err)
	}
}
