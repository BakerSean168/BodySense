package repository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func seedThreadProjectionFixture(t *testing.T, db *gorm.DB, owner uuid.UUID) (*model.ThreadProjection, []model.ThreadProjectionMessage, []model.ThreadProjectionToolCall) {
	t.Helper()
	source := seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(`[{"type":"text","text":"synthetic statement"}]`), "user")
	var message model.Message
	if err := db.First(&message, "id = ?", source.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	projection := &model.ThreadProjection{ConversationID: message.ConversationID, UserID: owner, Title: "before", TitleStatus: "generated", Status: "active", Phase: model.ConsultationPhaseCollecting, Metadata: datatypes.JSON(`{}`), ExtractedInfo: datatypes.JSON(`[]`), PendingInteractions: datatypes.JSON(`[]`), InteractionHistory: datatypes.JSON(`[]`), ConversationCreatedAt: now, ConversationUpdatedAt: now, SessionCreatedAt: now, SessionUpdatedAt: now, RefreshedAt: now}
	messages := []model.ThreadProjectionMessage{{MessageID: message.ID, ConversationID: message.ConversationID, TurnID: message.TurnID, Seq: 1, Role: "user", Status: "completed", Parts: message.Parts, ContentText: "before", Metadata: datatypes.JSON(`{}`), CreatedAt: now, UpdatedAt: now}}
	tools := []model.ThreadProjectionToolCall{{ToolCallID: "provider-call-1", ConversationID: message.ConversationID, RunID: source.RunID, MessageID: &message.ID, ToolName: "search_knowledge", Arguments: datatypes.JSON(`{}`), Status: "succeeded", Result: datatypes.JSON(`{"marker":"before"}`), Metadata: datatypes.JSON(`{}`), CreatedAt: now, StartedAt: now, FinishedAt: &now}}
	return projection, messages, tools
}

func TestThreadProjectionRepeatedProviderIDsAreRunAndOwnerScopedPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	repo := NewThreadProjectionRepository(db)
	ctx := context.Background()
	first, messages, tools := seedThreadProjectionFixture(t, db, owner)
	secondRun := model.Run{ID: uuid.New(), UserID: owner, ConversationID: first.ConversationID, TurnID: uuid.New(), RequestID: uuid.NewString(), Model: "synthetic", Status: model.RunStatusCompleted}
	if err := db.Create(&secondRun).Error; err != nil {
		t.Fatal(err)
	}
	secondTool := tools[0]
	secondTool.RunID = secondRun.ID
	secondTool.Status = "failed"
	secondTool.Result = nil
	secondTool.Error = datatypes.JSON(`{"error":"second only"}`)
	tools = append(tools, secondTool)
	if err := repo.UpsertSnapshot(ctx, first, messages, tools); err != nil {
		t.Fatalf("same provider ID across runs rejected: %v", err)
	}
	otherProjection, otherMessages, otherTools := seedThreadProjectionFixture(t, db, other)
	if err := repo.UpsertSnapshot(ctx, otherProjection, otherMessages, otherTools); err != nil {
		t.Fatalf("same provider ID across owners rejected: %v", err)
	}
	_, _, read, err := repo.GetByConversationID(ctx, first.ConversationID, owner)
	if err != nil || len(read) != 2 {
		t.Fatalf("tools=%#v err=%v", read, err)
	}
	statuses := map[uuid.UUID]string{}
	for _, tool := range read {
		statuses[tool.RunID] = tool.Status
		if tool.ToolCallID != "provider-call-1" {
			t.Fatal("raw provider ID rewritten")
		}
	}
	if statuses[tools[0].RunID] != "succeeded" || statuses[secondRun.ID] != "failed" {
		t.Fatal("tool status crossed run boundaries")
	}
	first.Title = "owner refresh"
	if err := repo.UpsertSnapshot(ctx, first, messages, tools[:1]); err != nil {
		t.Fatal(err)
	}
	untouched, _, otherRead, err := repo.GetByConversationID(ctx, otherProjection.ConversationID, other)
	if err != nil || untouched.Title != "before" || len(otherRead) != 1 {
		t.Fatal("refresh mutated another user's tool")
	}
	if projection, _, _, err := repo.GetByConversationID(ctx, first.ConversationID, other); err != nil || projection != nil {
		t.Fatal("foreign owner could read thread")
	}
}

func TestThreadProjectionFailedSnapshotWriteRollsBackAllPartsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewThreadProjectionRepository(db)
	ctx := context.Background()
	projection, messages, tools := seedThreadProjectionFixture(t, db, owner)
	if err := repo.UpsertSnapshot(ctx, projection, messages, tools); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("synthetic tool projection write failure")
	const callback = "thread_projection_test_fail_last_write"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "thread_projection_tool_calls" {
			_ = tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	projection.Title = "failed update"
	messages[0].ContentText = "failed update"
	tools[0].Result = datatypes.JSON(`{"marker":"failed update"}`)
	if err := repo.UpsertSnapshot(ctx, projection, messages, tools); !errors.Is(err, failure) {
		t.Fatalf("expected injected failure, got %v", err)
	}
	stored, storedMessages, storedTools, err := repo.GetByConversationID(ctx, projection.ConversationID, owner)
	if err != nil || stored.Title != "before" || len(storedMessages) != 1 || storedMessages[0].ContentText != "before" || len(storedTools) != 1 || !strings.Contains(string(storedTools[0].Result), "before") {
		t.Fatalf("snapshot only partially rolled back: %v", err)
	}
}

type threadSnapshotBarrierKey struct{}

func TestThreadProjectionReadsOneSnapshotDuringConcurrentRefreshPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewThreadProjectionRepository(db)
	ctx := context.Background()
	projection, messages, tools := seedThreadProjectionFixture(t, db, owner)
	if err := repo.UpsertSnapshot(ctx, projection, messages, tools); err != nil {
		t.Fatal(err)
	}
	captured, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	const callback = "thread_projection_test_snapshot_barrier"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "thread_projections" && tx.Statement.Context.Value(threadSnapshotBarrierKey{}) == true {
			once.Do(func() { close(captured); <-release })
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	type result struct {
		projection *model.ThreadProjection
		messages   []model.ThreadProjectionMessage
		tools      []model.ThreadProjectionToolCall
		err        error
	}
	done := make(chan result, 1)
	readCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		p, m, tc, e := repo.GetByConversationID(context.WithValue(readCtx, threadSnapshotBarrierKey{}, true), projection.ConversationID, owner)
		done <- result{p, m, tc, e}
	}()
	select {
	case <-captured:
	case <-readCtx.Done():
		close(release)
		t.Fatal("snapshot barrier timed out")
	}
	projection.Title = "after"
	messages[0].ContentText = "after"
	tools[0].Result = datatypes.JSON(`{"marker":"after"}`)
	err := repo.UpsertSnapshot(ctx, projection, messages, tools)
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	old := <-done
	if old.err != nil || old.projection == nil || old.projection.Title != "before" || len(old.messages) != 1 || old.messages[0].ContentText != "before" || len(old.tools) != 1 || !strings.Contains(string(old.tools[0].Result), "before") {
		t.Fatalf("mixed old metadata and new thread rows: %#v", old)
	}
	fresh, freshMessages, freshTools, err := repo.GetByConversationID(ctx, projection.ConversationID, owner)
	if err != nil || fresh.Title != "after" || freshMessages[0].ContentText != "after" || !strings.Contains(string(freshTools[0].Result), "after") {
		t.Fatal("new read did not see the refreshed snapshot")
	}
}

func TestThreadProjectionDowngradeRefusesLossOfDistinctRunHistoryPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewThreadProjectionRepository(db)
	first, messages, tools := seedThreadProjectionFixture(t, db, owner)
	second, messages2, tools2 := seedThreadProjectionFixture(t, db, owner)
	if err := repo.UpsertSnapshot(context.Background(), first, messages, tools); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertSnapshot(context.Background(), second, messages2, tools2); err != nil {
		t.Fatal(err)
	}
}
