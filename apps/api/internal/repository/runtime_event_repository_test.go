package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupRuntimeEventRepo(t *testing.T) (*RuntimeEventRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewRuntimeEventRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func runtimeEventRows(runID, conversationID uuid.UUID, seqs ...int) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{
		"id",
		"conversation_id",
		"run_id",
		"turn_id",
		"seq",
		"channel",
		"type",
		"ids",
		"payload",
		"source",
		"replayable",
		"created_at",
	})
	now := time.Now()
	for _, seq := range seqs {
		rows.AddRow(
			uuid.New(),
			conversationID,
			runID,
			uuid.New(),
			seq,
			"message",
			"message.completed",
			[]byte(`{"run_id":"`+runID.String()+`"}`),
			[]byte(`{"status":"completed"}`),
			"go",
			true,
			now,
		)
	}
	return rows
}

func TestRuntimeEventRepositoryListByRunID(t *testing.T) {
	repo, mock, cleanup := setupRuntimeEventRepo(t)
	defer cleanup()

	conversationID := uuid.New()
	runID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "runtime_events" WHERE conversation_id = $1 AND run_id = $2 AND seq > $3 ORDER BY seq ASC LIMIT $4`)).
		WithArgs(conversationID, runID, 3, 3).
		WillReturnRows(runtimeEventRows(runID, conversationID, 4, 5))

	events, hasMore, err := repo.ListByRunID(context.Background(), conversationID, runID, 3, 2)
	if err != nil {
		t.Fatalf("ListByRunID failed: %v", err)
	}
	if hasMore {
		t.Fatal("expected hasMore to be false")
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Seq != 4 || events[1].Seq != 5 {
		t.Fatalf("unexpected seqs: %+v", events)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRuntimeEventRepositoryListByRunIDHasMore(t *testing.T) {
	repo, mock, cleanup := setupRuntimeEventRepo(t)
	defer cleanup()

	conversationID := uuid.New()
	runID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "runtime_events" WHERE conversation_id = $1 AND run_id = $2 AND seq > $3 ORDER BY seq ASC LIMIT $4`)).
		WithArgs(conversationID, runID, 0, 3).
		WillReturnRows(runtimeEventRows(runID, conversationID, 1, 2, 3))

	events, hasMore, err := repo.ListByRunID(context.Background(), conversationID, runID, 0, 2)
	if err != nil {
		t.Fatalf("ListByRunID failed: %v", err)
	}
	if !hasMore {
		t.Fatal("expected hasMore to be true")
	}
	if len(events) != 2 {
		t.Fatalf("expected trimmed 2 events, got %d", len(events))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRuntimeEventRepositoryListByConversationID(t *testing.T) {
	repo, mock, cleanup := setupRuntimeEventRepo(t)
	defer cleanup()

	conversationID := uuid.New()
	runID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "runtime_events" WHERE conversation_id = $1 ORDER BY created_at ASC, seq ASC`)).
		WithArgs(conversationID).
		WillReturnRows(runtimeEventRows(runID, conversationID, 1, 2))

	events, err := repo.ListByConversationID(context.Background(), conversationID)
	if err != nil {
		t.Fatalf("ListByConversationID failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("unexpected seqs: %+v", events)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
