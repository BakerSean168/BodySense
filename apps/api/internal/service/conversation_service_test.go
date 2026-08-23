package service

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newServiceTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return gormDB, mock, func() { _ = sqlDB.Close() }
}

// TestConversationServiceDeleteConversationRevokesSharesTransactionally proves
// that delete + share revocation run inside one transaction and share the tx DB
// via FromContext. The share revocation must be atomic with the soft delete.
func TestConversationServiceDeleteConversationRevokesSharesTransactionally(t *testing.T) {
	gormDB, mock, cleanup := newServiceTestDB(t)
	defer cleanup()

	userID := uuid.New()
	conversationID := uuid.New()

	conversationRepo := repository.NewConversationRepository(gormDB)
	shareRepo := repository.NewConversationShareRepository(gormDB)

	svc := NewConversationService(
		conversationRepo,
		nil,
		nil,
		shareRepo,
		nil,
		database.NewTransactionManager(gormDB),
	)

	// Ownership lookup + share revocation + soft delete all inside the tx.
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations" WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL ORDER BY "conversations"."id" LIMIT $3`)).
		WithArgs(conversationID, userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(conversationID, userID))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "conversation_shares" WHERE conversation_id = $1`)).
		WithArgs(conversationID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "conversations" SET "deleted_at"=$1,"updated_at"=$2 WHERE id = $3 AND user_id = $4 AND deleted_at IS NULL`)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), conversationID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.DeleteConversation(context.Background(), conversationID, userID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestConversationServiceDeleteConversationRollsBackOnShareRevoke proves the
// operation is atomic: a failed share revocation rolls back and does not soft-delete.
func TestConversationServiceDeleteConversationRollsBackOnShareRevoke(t *testing.T) {
	gormDB, mock, cleanup := newServiceTestDB(t)
	defer cleanup()

	userID := uuid.New()
	conversationID := uuid.New()

	conversationRepo := repository.NewConversationRepository(gormDB)
	shareRepo := repository.NewConversationShareRepository(gormDB)

	svc := NewConversationService(
		conversationRepo,
		nil,
		nil,
		shareRepo,
		nil,
		database.NewTransactionManager(gormDB),
	)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "conversations" WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL ORDER BY "conversations"."id" LIMIT $3`)).
		WithArgs(conversationID, userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(conversationID, userID))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "conversation_shares" WHERE conversation_id = $1`)).
		WithArgs(conversationID).
		WillReturnError(gorm.ErrInvalidData)
	mock.ExpectRollback()

	err := svc.DeleteConversation(context.Background(), conversationID, userID)
	if err == nil {
		t.Fatal("expected DeleteConversation to fail when share revocation fails")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}