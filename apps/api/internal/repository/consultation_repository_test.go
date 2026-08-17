package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupConsultationRepo(t *testing.T) (*ConsultationRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewConsultationRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func TestConsultationRepositoryCreateRunEnvelopeUsesSingleTransaction(t *testing.T) {
	repo, mock, cleanup := setupConsultationRepo(t)
	defer cleanup()

	userID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE id = \$1.*FOR UPDATE`).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	// ADR 0004: an omitted conversation id now looks for the user's canonical
	// long-lived consultation, not merely an empty conversation.
	mock.ExpectQuery(`SELECT .* FROM "conversations" JOIN consultation_sessions ON consultation_sessions\.conversation_id = conversations\.id WHERE conversations\.user_id = \$1 AND conversations\.status = \$2 AND conversations\.deleted_at IS NULL ORDER BY COALESCE\(conversations\.last_message_at, conversations\.created_at\) DESC.*LIMIT \$3`).
		WithArgs(userID, "active", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "conversations"`)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "consultation_sessions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at", "ended_at"}).AddRow(now, now, nil))
	mock.ExpectQuery(`SELECT .* FROM "conversations" WHERE id = \$1 AND user_id = \$2 AND deleted_at IS NULL.*LIMIT \$3 FOR UPDATE`).
		WithArgs(sqlmock.AnyArg(), userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "active_run_id", "active_stream_id"}).
			AddRow(uuid.New(), userID, nil, ""))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(MAX(seq), 0) + 1 FROM "messages" WHERE conversation_id = $1`)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "runs"`)).
		WillReturnRows(sqlmock.NewRows([]string{"started_at", "completed_at"}).AddRow(now, nil))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "messages"`)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "messages"`)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "conversations" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, _, _, _, _, existed, err := repo.CreateRunEnvelope(
		context.Background(),
		userID,
		nil,
		"req-1",
		datatypes.JSON(`[{"type":"text","text":"hello"}]`),
		datatypes.JSON(`{}`),
		"consultation-thread",
	)
	if err != nil {
		t.Fatalf("CreateRunEnvelope returned error: %v", err)
	}
	if existed {
		t.Fatal("expected a new run envelope")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnsureConversationRunAvailableRejectsRunningRun(t *testing.T) {
	repo, mock, cleanup := setupConsultationRepo(t)
	defer cleanup()
	conversationID := uuid.New()
	runID := uuid.New()
	conversation := &model.Conversation{ID: conversationID, ActiveRunID: &runID}

	mock.ExpectQuery(`SELECT .* FROM "runs" WHERE .*id = \$1 AND conversation_id = \$2.*LIMIT \$3`).
		WithArgs(runID, conversationID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "status"}).
			AddRow(runID, conversationID, "running"))

	err := repo.ensureConversationRunAvailable(context.Background(), repo.db, conversation)
	if !errors.Is(err, model.ErrConversationRunInProgress) {
		t.Fatalf("expected active-run conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnsureConversationRunAvailableRejectsPendingInteraction(t *testing.T) {
	repo, mock, cleanup := setupConsultationRepo(t)
	defer cleanup()
	conversationID := uuid.New()
	conversation := &model.Conversation{ID: conversationID}

	mock.ExpectQuery(`SELECT count\(\*\) FROM "agent_interactions" WHERE conversation_id = \$1 AND status = \$2`).
		WithArgs(conversationID, "pending").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	err := repo.ensureConversationRunAvailable(context.Background(), repo.db, conversation)
	if !errors.Is(err, model.ErrConversationRunInProgress) {
		t.Fatalf("expected pending-interaction conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestEnsureConversationRunAvailableClearsTerminalPointer(t *testing.T) {
	repo, mock, cleanup := setupConsultationRepo(t)
	defer cleanup()
	conversationID := uuid.New()
	runID := uuid.New()
	conversation := &model.Conversation{ID: conversationID, ActiveRunID: &runID, ActiveStreamID: runID.String()}

	mock.ExpectQuery(`SELECT .* FROM "runs" WHERE .*id = \$1 AND conversation_id = \$2.*LIMIT \$3`).
		WithArgs(runID, conversationID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "status"}).
			AddRow(runID, conversationID, "completed"))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "conversations" SET .*active_run_id.*active_stream_id.* WHERE id = \$[0-9]+`).
		WithArgs(nil, "", sqlmock.AnyArg(), conversationID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT count\(\*\) FROM "agent_interactions" WHERE conversation_id = \$1 AND status = \$2`).
		WithArgs(conversationID, "pending").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	if err := repo.ensureConversationRunAvailable(context.Background(), repo.db, conversation); err != nil {
		t.Fatalf("terminal pointer should be cleared: %v", err)
	}
	if conversation.ActiveRunID != nil || conversation.ActiveStreamID != "" {
		t.Fatalf("stale active pointer was not cleared: %#v", conversation)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
