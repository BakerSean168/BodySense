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

// mockRunRepo implements the runRepo interface for testing.
type mockRunRepo struct {
	runs       map[uuid.UUID]*model.Run
	byReqID    map[string]*model.Run
	lastStatus string
}

func newMockRunRepo() *mockRunRepo {
	return &mockRunRepo{
		runs:    make(map[uuid.UUID]*model.Run),
		byReqID: make(map[string]*model.Run),
	}
}

func (m *mockRunRepo) Create(ctx context.Context, run *model.Run) error {
	m.runs[run.ID] = run
	m.byReqID[run.RequestID] = run
	return nil
}

func (m *mockRunRepo) CreateWithIdempotency(ctx context.Context, run *model.Run) (*model.Run, bool, error) {
	if existing, ok := m.byReqID[run.RequestID]; ok && existing.UserID == run.UserID {
		return existing, true, nil
	}
	if err := m.Create(ctx, run); err != nil {
		return nil, false, err
	}
	return run, false, nil
}

func (m *mockRunRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Run, error) {
	return m.runs[id], nil
}

func (m *mockRunRepo) GetByRequestID(ctx context.Context, userID uuid.UUID, requestID string) (*model.Run, error) {
	run, ok := m.byReqID[requestID]
	if !ok {
		return nil, nil
	}
	if run.UserID != userID {
		return nil, nil
	}
	return run, nil
}

func (m *mockRunRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.Run, error) {
	var runs []model.Run
	for _, r := range m.runs {
		if r.ConversationID == conversationID {
			runs = append(runs, *r)
		}
	}
	return runs, nil
}

func (m *mockRunRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if run, ok := m.runs[id]; ok {
		run.Status = status
		m.lastStatus = status
	}
	return nil
}

func (m *mockRunRepo) CompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) error {
	_, err := m.TryCompleteRun(ctx, id, userID, usage, providerResponseID)
	return err
}

func (m *mockRunRepo) TryCompleteRun(ctx context.Context, id, userID uuid.UUID, usage any, providerResponseID string) (bool, error) {
	if run, ok := m.runs[id]; ok && run.UserID == userID && (run.Status == "running" || run.Status == "waiting_user") {
		run.Status = "completed"
		m.lastStatus = "completed"
		return true, nil
	}
	return false, nil
}

func (m *mockRunRepo) CancelRun(_ context.Context, id, userID uuid.UUID, reason any) (bool, error) {
	if run, ok := m.runs[id]; ok && run.UserID == userID && (run.Status == "running" || run.Status == "waiting_user") {
		run.Status = "cancelled"
		m.lastStatus = "cancelled"
		return true, nil
	}
	return false, nil
}

func (m *mockRunRepo) FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) error {
	if run, ok := m.runs[id]; ok {
		run.Status = "failed"
		m.lastStatus = "failed"
	}
	return nil
}

func (m *mockRunRepo) UpdateAgentConfiguration(
	ctx context.Context,
	id uuid.UUID,
	configurationID string,
	configuration datatypes.JSON,
	provenance datatypes.JSON,
) error {
	if run, ok := m.runs[id]; ok {
		run.AgentConfigurationID = configurationID
		run.AgentConfiguration = configuration
		run.ExecutionProvenance = provenance
	}
	return nil
}

func (m *mockRunRepo) RenewLease(context.Context, uuid.UUID, uuid.UUID, string, time.Time, time.Time) (bool, error) {
	return true, nil
}

func (m *mockRunRepo) ReclaimExpiredRuns(context.Context, time.Time, int) ([]model.Run, error) {
	return nil, nil
}

func TestMarkWaitingUser(t *testing.T) {
	repo := newMockRunRepo()
	svc := NewRunService(repo)

	ctx := context.Background()
	run, err := svc.CreateRun(ctx, uuid.New(), uuid.New(), "req-1", uuid.New(), "")
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	if err := svc.MarkWaitingUser(ctx, run.ID); err != nil {
		t.Fatalf("MarkWaitingUser failed: %v", err)
	}

	if repo.lastStatus != "waiting_user" {
		t.Errorf("expected status waiting_user, got %s", repo.lastStatus)
	}
}

func TestResumeRunning(t *testing.T) {
	repo := newMockRunRepo()
	svc := NewRunService(repo)

	ctx := context.Background()
	run, err := svc.CreateRun(ctx, uuid.New(), uuid.New(), "req-2", uuid.New(), "")
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// First mark as waiting_user
	if err := svc.MarkWaitingUser(ctx, run.ID); err != nil {
		t.Fatalf("MarkWaitingUser failed: %v", err)
	}

	// Then resume
	if err := svc.ResumeRunning(ctx, run.ID); err != nil {
		t.Fatalf("ResumeRunning failed: %v", err)
	}

	if repo.lastStatus != "running" {
		t.Errorf("expected status running, got %s", repo.lastStatus)
	}
}

func TestCancelRunIsIdempotentAndPreventsLaterCompletion(t *testing.T) {
	repo := newMockRunRepo()
	svc := NewRunService(repo)
	ctx := context.Background()
	userID := uuid.New()
	run, err := svc.CreateRun(ctx, uuid.New(), uuid.New(), "cancel-idempotent", userID, "")
	if err != nil {
		t.Fatal(err)
	}

	cancelled, transitioned, err := svc.CancelRun(ctx, run.ID, userID, "user_requested")
	if err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	if !transitioned || cancelled.Status != "cancelled" {
		t.Fatalf("first cancel should transition: run=%+v transitioned=%v", cancelled, transitioned)
	}

	cancelledAgain, transitionedAgain, err := svc.CancelRun(ctx, run.ID, userID, "user_requested")
	if err != nil {
		t.Fatalf("second cancel must be idempotent: %v", err)
	}
	if transitionedAgain || cancelledAgain.Status != "cancelled" {
		t.Fatalf("second cancel should be no-op: run=%+v transitioned=%v", cancelledAgain, transitionedAgain)
	}

	completed, err := svc.TryCompleteRun(ctx, run.ID, userID, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if completed {
		t.Fatal("completion must not overwrite cancelled terminal state")
	}
}

func TestCompletionWinsTerminalRaceAndLaterCancelRejects(t *testing.T) {
	repo := newMockRunRepo()
	svc := NewRunService(repo)
	ctx := context.Background()
	userID := uuid.New()
	run, err := svc.CreateRun(ctx, uuid.New(), uuid.New(), "complete-wins", userID, "")
	if err != nil {
		t.Fatal(err)
	}

	completed, err := svc.TryCompleteRun(ctx, run.ID, userID, nil, "provider-response")
	if err != nil || !completed {
		t.Fatalf("completion should win: completed=%v err=%v", completed, err)
	}

	_, transitioned, err := svc.CancelRun(ctx, run.ID, userID, "too_late")
	if !errors.Is(err, ErrRunTerminal) || transitioned {
		t.Fatalf("late cancel should reject terminal run: transitioned=%v err=%v", transitioned, err)
	}
}
