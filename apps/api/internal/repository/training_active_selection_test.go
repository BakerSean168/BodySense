package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupActiveTrainingSelectionMock(t *testing.T) (*TrainingRepository, sqlmock.Sqlmock) {
	t.Helper()
	dbConn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: dbConn}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return NewTrainingRepository(db), mock
}

func expectActiveTrainingSelection(
	mock sqlmock.Sqlmock, owner uuid.UUID, rows *sqlmock.Rows,
) {
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM \"training_plans\" WHERE user_id = $1 AND status = $2 ORDER BY created_at DESC,id DESC LIMIT $3",
	)).WithArgs(owner, "active", 2).WillReturnRows(rows)
}

func activeTrainingRows(owner uuid.UUID, ids ...uuid.UUID) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "user_id", "status", "goal", "created_at"})
	for _, id := range ids {
		rows.AddRow(id, owner, "active", "synthetic-only", time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	}
	return rows
}

func TestTrainingActiveSelectionFailClosedOnMissingOwner(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	got, err := repo.GetActivePlanByUserID(context.Background(), uuid.Nil)
	if got != nil || err == nil {
		t.Fatalf("missing owner must not query or return active plans: %#v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTrainingActiveSelectionEmptyAndSingle(t *testing.T) {
	for _, count := range []int{0, 1} {
		t.Run(map[int]string{0: "none", 1: "one"}[count], func(t *testing.T) {
			repo, mock := setupActiveTrainingSelectionMock(t)
			owner := uuid.New()
			id := uuid.New()
			rows := activeTrainingRows(owner)
			if count == 1 {
				rows = activeTrainingRows(owner, id)
			}
			expectActiveTrainingSelection(mock, owner, rows)
			got, err := repo.GetActivePlanByUserID(context.Background(), owner)
			if err != nil {
				t.Fatal(err)
			}
			if count == 0 && got != nil {
				t.Fatalf("expected nil, got %#v", got)
			}
			if count == 1 && (got == nil || got.ID != id) {
				t.Fatalf("expected sole active plan %s, got %#v", id, got)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTrainingActiveSelectionRejectsTwoWithoutLeakingIdentity(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	owner := uuid.New()
	first, second := uuid.New(), uuid.New()
	expectActiveTrainingSelection(mock, owner, activeTrainingRows(owner, first, second))
	got, err := repo.GetActivePlanByUserID(context.Background(), owner)
	if got != nil || !errors.Is(err, ErrMultipleActiveTrainingPlans) {
		t.Fatalf("multiple active plans must fail closed: %#v %v", got, err)
	}
	for _, id := range []uuid.UUID{owner, first, second} {
		if strings.Contains(err.Error(), id.String()) {
			t.Fatal("ambiguous active plan IDs leaked")
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTrainingActiveSelectionPropagatesQueryError(t *testing.T) {
	repo, mock := setupActiveTrainingSelectionMock(t)
	owner := uuid.New()
	sentinel := errors.New("synthetic plan query failed")
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM \"training_plans\" WHERE user_id = $1 AND status = $2 ORDER BY created_at DESC,id DESC LIMIT $3",
	)).WithArgs(owner, "active", 2).WillReturnError(sentinel)
	got, err := repo.GetActivePlanByUserID(context.Background(), owner)
	if got != nil || !errors.Is(err, sentinel) {
		t.Fatalf("query failure incorrectly masked: %#v %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
