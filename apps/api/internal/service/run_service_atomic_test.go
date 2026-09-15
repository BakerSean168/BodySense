package service

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTryCompleteRunWithEventRollsBackWhenEventInsertFails(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	runRepo := repository.NewRunRepository(gormDB)
	eventRepo := repository.NewRuntimeEventRepository(gormDB)
	svc := NewRunService(runRepo, "test-owner").WithLifecycleEvents(
		NewRuntimeEventService(eventRepo),
		database.NewTransactionManager(gormDB),
	)

	userID := uuid.New()
	conversationID := uuid.New()
	turnID := uuid.New()
	run := &model.Run{ID: uuid.New(), UserID: userID, ConversationID: conversationID, TurnID: turnID, Status: model.RunStatusRunning}
	event, err := dto.NewStreamEvent(7, "run", "run.completed", dto.StreamEventIDs{
		ConversationID: conversationID.String(), RunID: run.ID.String(), TurnID: turnID.String(),
	}, map[string]any{"status": "completed"})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "runs" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND status IN \(\$[0-9]+,\$[0-9]+\)`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "runtime_events" .* RETURNING "id","ids","payload","created_at"`).
		WillReturnError(errors.New("runtime event insert failed"))
	mock.ExpectRollback()

	completed, err := svc.TryCompleteRunWithEvent(context.Background(), run, userID, nil, "", event)
	if err == nil {
		t.Fatal("expected lifecycle event insertion failure")
	}
	if completed {
		t.Fatal("run must not report completed when lifecycle transaction rolls back")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
