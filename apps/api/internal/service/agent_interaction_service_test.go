package service

import (
	"context"
	"errors"
	"testing"

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

func (r *fakeInteractionRepo) ListPendingByConversation(_ context.Context, conversationID uuid.UUID) ([]model.AgentInteraction, error) {
	var interactions []model.AgentInteraction
	for _, interaction := range r.byID {
		if interaction.ConversationID == conversationID && interaction.Status == "pending" {
			interactions = append(interactions, *interaction)
		}
	}
	return interactions, nil
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
	svc := NewAgentInteractionService(repo, runRepo)
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{})
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{})
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
	svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{})
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
