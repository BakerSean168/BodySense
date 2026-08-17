package database

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTransactionManagerRollsBackCallbackFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE outcomes SET body_state_revision = $1 WHERE id = $2")).
		WithArgs(int64(9), "outcome-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	manager := NewTransactionManager(db)
	expected := errors.New("body state projection failed")
	err = manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		if execErr := FromContext(ctx, db).Exec(
			"UPDATE outcomes SET body_state_revision = ? WHERE id = ?",
			int64(9), "outcome-1",
		).Error; execErr != nil {
			return execErr
		}
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
