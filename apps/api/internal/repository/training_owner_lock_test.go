package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/database"
	"github.com/google/uuid"
)

func TestTrainingOwnerLockRejectsMissingOwnerWithoutSQL(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	err := repo.LockOwnerForPlanActivation(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrTrainingPlanOwnerNotFound) {
		t.Fatalf("missing owner must not acquire lock: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTrainingOwnerLockRejectsOutsideTransactionWithoutSQL(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	err := repo.LockOwnerForPlanActivation(context.Background(), uuid.New())
	if !errors.Is(err, ErrTrainingPlanOwnerLockRequiresTransaction) {
		t.Fatalf("lock outside transaction falsely claimed serialization: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTrainingOwnerLockUsesScopedUserRowForNoKeyUpdate(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	userID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT \"id\" FROM \"users\" WHERE id = $1 LIMIT $2 FOR NO KEY UPDATE",
	)).WithArgs(userID, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	mock.ExpectCommit()
	manager := database.NewTransactionManager(repo.db)
	err := manager.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		return repo.LockOwnerForPlanActivation(txCtx, userID)
	})
	if err != nil {
		t.Fatalf("owner locking should succeed inside transaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTrainingOwnerLockFailsClosedForMissingOwnerInTransaction(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	owner := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT \"id\" FROM \"users\" WHERE id = $1 LIMIT $2 FOR NO KEY UPDATE",
	)).WithArgs(owner, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	err := database.NewTransactionManager(repo.db).WithinTransaction(
		context.Background(), func(txCtx context.Context) error {
			return repo.LockOwnerForPlanActivation(txCtx, owner)
		},
	)
	if !errors.Is(err, ErrTrainingPlanOwnerNotFound) {
		t.Fatalf("missing owner was silently admitted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTrainingOwnerLockPropagatesDatabaseFailures(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	owner := uuid.New()
	sentinel := errors.New("synthetic owner lock database failure")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT \"id\" FROM \"users\" WHERE id = $1 LIMIT $2 FOR NO KEY UPDATE",
	)).WithArgs(owner, 1).WillReturnError(sentinel)
	mock.ExpectRollback()
	err := database.NewTransactionManager(repo.db).WithinTransaction(
		context.Background(), func(txCtx context.Context) error {
			return repo.LockOwnerForPlanActivation(txCtx, owner)
		},
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("failed owner lock did not preserve database error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
