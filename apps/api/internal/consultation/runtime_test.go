package consultation

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/stream"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeRuntimeEventRepo struct {
	events []model.RuntimeEvent
}

func (r *fakeRuntimeEventRepo) Create(ctx context.Context, event *model.RuntimeEvent) error {
	r.events = append(r.events, *event)
	return nil
}

func (r *fakeRuntimeEventRepo) CreateBatch(ctx context.Context, events []*model.RuntimeEvent) error {
	for _, event := range events {
		if err := r.Create(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (r *fakeRuntimeEventRepo) ListByRunID(ctx context.Context, conversationID, runID uuid.UUID, afterSeq, limit int) ([]model.RuntimeEvent, bool, error) {
	out := make([]model.RuntimeEvent, 0, len(r.events))
	for _, event := range r.events {
		if event.ConversationID == conversationID && event.RunID == runID && event.Seq > afterSeq {
			out = append(out, event)
		}
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

func (r *fakeRuntimeEventRepo) ListByConversationID(ctx context.Context, conversationID uuid.UUID) ([]model.RuntimeEvent, error) {
	out := make([]model.RuntimeEvent, 0, len(r.events))
	for _, event := range r.events {
		if event.ConversationID == conversationID {
			out = append(out, event)
		}
	}
	return out, nil
}

func TestReplayCompletedRunReplaysRuntimeEventsWithoutSyntheticError(t *testing.T) {
	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()
	ids := datatypes.JSON([]byte(`{"conversation_id":"` + conversationID.String() + `","run_id":"` + runID.String() + `","turn_id":"` + turnID.String() + `"}`))

	runtime := &Runtime{
		runtimeEventService: service.NewRuntimeEventService(&fakeRuntimeEventRepo{events: []model.RuntimeEvent{
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 3, Channel: "message", Type: "message.completed", IDs: ids, Payload: datatypes.JSON([]byte(`{"status":"completed"}`))},
			{ConversationID: conversationID, RunID: runID, TurnID: &turnID, Seq: 4, Channel: "run", Type: "run.completed", IDs: ids, Payload: datatypes.JSON([]byte(`{"status":"completed"}`))},
		}}),
		streamRuntime: stream.NewRuntime(),
	}

	recorder := httptest.NewRecorder()
	runtime.replayCompletedRun(context.Background(), recorder, &model.Run{
		ID:             runID,
		ConversationID: conversationID,
		TurnID:         turnID,
		Status:         "completed",
	})

	body := recorder.Body.String()
	if strings.Contains(body, "stream.error") {
		t.Fatalf("replay should not emit stream.error: %s", body)
	}
	if !strings.Contains(body, "event: message.completed") || !strings.Contains(body, "event: run.completed") {
		t.Fatalf("replay did not include stored events: %s", body)
	}
	if !strings.Contains(body, `"type":"stream.done"`) || !strings.Contains(body, `"seq":5`) {
		t.Fatalf("replay done should follow stored seqs: %s", body)
	}
}
