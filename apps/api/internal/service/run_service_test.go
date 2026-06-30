package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
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
	if run, ok := m.runs[id]; ok {
		run.Status = "completed"
		m.lastStatus = "completed"
	}
	return nil
}

func (m *mockRunRepo) FailRun(ctx context.Context, id, userID uuid.UUID, errJSON any) error {
	if run, ok := m.runs[id]; ok {
		run.Status = "failed"
		m.lastStatus = "failed"
	}
	return nil
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
