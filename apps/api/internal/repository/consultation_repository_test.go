package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations" WHERE user_id = $1 AND status = $2 AND last_message_at IS NULL AND deleted_at IS NULL ORDER BY created_at DESC,"conversations"."id" LIMIT $3`)).
		WithArgs(userID, "active", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "conversations"`)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "consultation_sessions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at", "ended_at"}).AddRow(now, now, nil))
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
