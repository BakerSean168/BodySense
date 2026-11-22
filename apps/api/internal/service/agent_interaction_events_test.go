package service

import (
	"context"
	"errors"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"testing"
	"time"
)

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

type recordingSettle struct{ conversations []uuid.UUID }

func (r *recordingSettle) OnConversationSettled(_ context.Context, conversationID uuid.UUID) {
	r.conversations = append(r.conversations, conversationID)
}

// PC-RET-001: an Ask User that closes without an answer settles its Run, so
// every expiry and cancellation path fires the retention settle trigger once,
// after the transition committed.
func TestAgentInteractionServiceClosingWithoutAnswerFiresRetentionSettle(t *testing.T) {
	pastDue := func(repo *fakeInteractionRepo, id uuid.UUID) {
		past := time.Now().UTC().Add(-time.Minute)
		repo.byID[id].ExpiresAt = &past
	}
	type setup struct {
		repo   *fakeInteractionRepo
		settle *recordingSettle
		svc    *AgentInteractionService
		item   *model.AgentInteraction
	}
	newSetup := func(t *testing.T, events bool, tx fakeInteractionTransactionManager) setup {
		repo, settle := newFakeInteractionRepo(), &recordingSettle{}
		svc := NewAgentInteractionService(repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(), tx).WithRetentionSettle(settle)
		if events {
			svc.WithLifecycleEvents(&fakeInteractionLifecycleEvents{})
		}
		item, err := svc.CreatePendingInteraction(context.Background(), uuid.New(), uuid.New(), "call-settle", datatypes.JSON(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		return setup{repo, settle, svc, item}
	}
	wantOnce := func(t *testing.T, s setup) {
		t.Helper()
		if len(s.settle.conversations) != 1 || s.settle.conversations[0] != s.item.ConversationID {
			t.Fatalf("settle calls=%v, want once for %s", s.settle.conversations, s.item.ConversationID)
		}
	}
	ctx := context.Background()

	t.Run("expiry sweep with lifecycle events", func(t *testing.T) {
		s := newSetup(t, true, fakeInteractionTransactionManager{})
		pastDue(s.repo, s.item.ID)
		if _, err := s.svc.ExpireExpiredInteractions(ctx, 10); err != nil {
			t.Fatal(err)
		}
		wantOnce(t, s)
		if _, err := s.svc.ExpireExpiredInteractions(ctx, 10); err != nil {
			t.Fatal(err)
		}
		wantOnce(t, s)
	})
	t.Run("repository-only expiry sweep", func(t *testing.T) {
		s := newSetup(t, false, fakeInteractionTransactionManager{})
		pastDue(s.repo, s.item.ID)
		if _, err := s.svc.ExpireExpiredInteractions(ctx, 10); err != nil {
			t.Fatal(err)
		}
		wantOnce(t, s)
	})
	t.Run("late answer soft-expires", func(t *testing.T) {
		for _, events := range []bool{false, true} {
			s := newSetup(t, events, fakeInteractionTransactionManager{})
			pastDue(s.repo, s.item.ID)
			resume := s.svc.ResumeInteraction
			if events {
				resume = s.svc.ResumeInteractionWithEvent
			}
			if err := resume(ctx, s.item.ID, datatypes.JSON(`{"text":"late"}`)); !errors.Is(err, ErrInteractionExpired) {
				t.Fatalf("events=%v: %v", events, err)
			}
			wantOnce(t, s)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		s := newSetup(t, true, fakeInteractionTransactionManager{})
		if err := s.svc.CancelInteraction(ctx, s.item.ID); err != nil {
			t.Fatal(err)
		}
		wantOnce(t, s)
	})
	t.Run("rolled back expiry does not settle", func(t *testing.T) {
		s := newSetup(t, true, fakeInteractionTransactionManager{})
		pastDue(s.repo, s.item.ID)
		failing := NewAgentInteractionService(s.repo, &fakeRunStatusRepo{}, newFakeConversationOwnership(),
			fakeInteractionTransactionManager{err: errors.New("synthetic rollback")}).
			WithLifecycleEvents(&fakeInteractionLifecycleEvents{}).WithRetentionSettle(s.settle)
		if _, err := failing.ExpireExpiredInteractions(ctx, 10); err != nil {
			t.Fatal(err)
		}
		if len(s.settle.conversations) != 0 {
			t.Fatalf("settled after a rolled back expiry: %v", s.settle.conversations)
		}
	})
}
