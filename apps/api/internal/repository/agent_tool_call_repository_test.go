package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupAgentToolCallRepo(t *testing.T) (*AgentToolCallRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewAgentToolCallRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func TestAgentToolCallRepositoryMarkSucceededRequiresRunning(t *testing.T) {
	repo, mock, cleanup := setupAgentToolCallRepo(t)
	defer cleanup()
	runID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "agent_tool_calls" SET .* WHERE run_id = \$[0-9]+ AND tool_call_id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := repo.MarkSucceeded(context.Background(), runID, "tc-1", datatypes.JSON(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected running tool call to transition to succeeded")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentToolCallRepositoryTerminalRaceDoesNotOverwrite(t *testing.T) {
	repo, mock, cleanup := setupAgentToolCallRepo(t)
	defer cleanup()
	runID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "agent_tool_calls" SET .* WHERE run_id = \$[0-9]+ AND tool_call_id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	updated, err := repo.MarkFailed(context.Background(), runID, "tc-1", datatypes.JSON(`{"error":"late"}`))
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("late terminal result must not overwrite an existing terminal winner")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentToolCallRepositoryDuplicateStartOnlyRefreshesRunningRow(t *testing.T) {
	repo, mock, cleanup := setupAgentToolCallRepo(t)
	defer cleanup()
	runID := uuid.New()
	conversationID := uuid.New()
	tc := &model.AgentToolCall{
		ID: uuid.New(), RunID: runID, ConversationID: conversationID,
		ToolCallID: "tc-1", ToolName: "search", Arguments: datatypes.JSON(`{"q":"x"}`),
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "agent_tool_calls" .* ON CONFLICT .* DO NOTHING RETURNING`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "agent_tool_calls" SET .* WHERE run_id = \$[0-9]+ AND tool_call_id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := repo.UpsertStarted(context.Background(), tc); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
