package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupJobRepo(t *testing.T) (*JobRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewJobRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func TestJobRepositoryClaimPendingIsConditionalAndIncrementsAttempt(t *testing.T) {
	repo, mock, cleanup := setupJobRepo(t)
	defer cleanup()
	jobID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .*"attempts"=attempts \+ 1.* WHERE id = \$[0-9]+ AND status = \$[0-9]+ AND attempts < max_attempts`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	claimed, err := repo.ClaimPending(context.Background(), jobID)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("expected pending job to be claimed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestJobRepositoryClaimPendingReturnsFalseWhenAnotherWorkerWon(t *testing.T) {
	repo, mock, cleanup := setupJobRepo(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .* WHERE id = \$[0-9]+ AND status = \$[0-9]+ AND attempts < max_attempts`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	claimed, err := repo.ClaimPending(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("expected claim to lose when conditional update affects zero rows")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
