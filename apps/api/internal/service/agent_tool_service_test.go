package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// mockAgentToolCallRepo implements AgentToolCallRepo interface.
type mockAgentToolCallRepo struct {
	upserted   []*model.AgentToolCall
	lastStatus model.AgentToolCallStatus
	lastResult datatypes.JSON
}

var _ AgentToolCallRepo = (*mockAgentToolCallRepo)(nil)

func (m *mockAgentToolCallRepo) UpsertStarted(_ context.Context, tc *model.AgentToolCall) error {
	m.upserted = append(m.upserted, tc)
	m.lastStatus = tc.Status
	return nil
}

func (m *mockAgentToolCallRepo) MarkSucceeded(_ context.Context, _ uuid.UUID, _ string, result any) (bool, error) {
	m.lastStatus = model.AgentToolCallSucceeded
	if r, ok := result.(datatypes.JSON); ok {
		m.lastResult = r
	}
	return true, nil
}

func (m *mockAgentToolCallRepo) MarkFailed(_ context.Context, _ uuid.UUID, _ string, result any) (bool, error) {
	m.lastStatus = model.AgentToolCallFailed
	if r, ok := result.(datatypes.JSON); ok {
		m.lastResult = r
	}
	return true, nil
}

func TestRecordToolCall_PersistsAsRunning(t *testing.T) {
	repo := &mockAgentToolCallRepo{}
	svc := NewAgentToolService(repo)

	runID := uuid.New()
	convID := uuid.New()
	msgID := uuid.New()
	svc.RecordToolCall(context.Background(), runID, convID, &msgID, "tc-1", "search_knowledge", datatypes.JSON(`{"query":"test"}`))

	if len(repo.upserted) != 1 {
		t.Fatalf("expected 1 upsert, got %d", len(repo.upserted))
	}
	if repo.upserted[0].Status != model.AgentToolCallRunning {
		t.Errorf("expected status 'running', got %q", repo.upserted[0].Status)
	}
	if repo.upserted[0].ToolCallID != "tc-1" {
		t.Errorf("expected tool_call_id 'tc-1', got %q", repo.upserted[0].ToolCallID)
	}
	if repo.upserted[0].ToolName != "search_knowledge" {
		t.Errorf("expected tool_name 'search_knowledge', got %q", repo.upserted[0].ToolName)
	}
}

func TestRecordToolCall_SkipsEmptyToolCallID(t *testing.T) {
	repo := &mockAgentToolCallRepo{}
	svc := NewAgentToolService(repo)

	svc.RecordToolCall(context.Background(), uuid.New(), uuid.New(), nil, "", "test", datatypes.JSON(`{}`))

	if len(repo.upserted) != 0 {
		t.Errorf("expected 0 upserts for empty tool_call_id, got %d", len(repo.upserted))
	}
}

func TestRecordToolResult_MarksSucceeded(t *testing.T) {
	repo := &mockAgentToolCallRepo{}
	svc := NewAgentToolService(repo)

	svc.RecordToolResult(context.Background(), uuid.New(), "tc-1", datatypes.JSON(`{"result":"ok"}`), false)

	if repo.lastStatus != model.AgentToolCallSucceeded {
		t.Errorf("expected status 'succeeded', got %q", repo.lastStatus)
	}
}

func TestRecordToolResult_MarksFailed(t *testing.T) {
	repo := &mockAgentToolCallRepo{}
	svc := NewAgentToolService(repo)

	svc.RecordToolResult(context.Background(), uuid.New(), "tc-1", datatypes.JSON(`{"error":"timeout"}`), true)

	if repo.lastStatus != model.AgentToolCallFailed {
		t.Errorf("expected status 'failed', got %q", repo.lastStatus)
	}
}

func TestRecordToolResult_SkipsEmptyToolCallID(t *testing.T) {
	repo := &mockAgentToolCallRepo{}
	svc := NewAgentToolService(repo)

	// Should not panic or call repo
	svc.RecordToolResult(context.Background(), uuid.New(), "", datatypes.JSON(`{}`), false)

	if repo.lastStatus != model.AgentToolCallStatus("") {
		t.Errorf("expected no repo call for empty tool_call_id, got status %q", repo.lastStatus)
	}
}
