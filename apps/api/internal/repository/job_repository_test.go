package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/model"
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

func TestJobRepositoryClaimPendingCommitsStateAndEventTogether(t *testing.T) {
	repo, mock, cleanup := setupJobRepo(t)
	defer cleanup()
	jobID := uuid.New()
	event := &model.JobEvent{EventType: "job.running"}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .*"attempts"=attempts \+ 1.* WHERE id = \$[0-9]+ AND status = \$[0-9]+ AND attempts < max_attempts`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "job_events" .* RETURNING "id","payload","created_at"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload", "created_at"}).AddRow(uuid.New(), []byte(`{}`), nil))
	mock.ExpectCommit()

	claimed, err := repo.ClaimPending(context.Background(), jobID, event)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("expected pending job to be claimed")
	}
	if event.JobID != jobID {
		t.Fatalf("event job id = %s, want %s", event.JobID, jobID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestJobRepositoryClaimPendingRollsBackWhenLifecycleEventFails(t *testing.T) {
	repo, mock, cleanup := setupJobRepo(t)
	defer cleanup()
	jobID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .* WHERE id = \$[0-9]+ AND status = \$[0-9]+ AND attempts < max_attempts`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "job_events" .* RETURNING "id","payload","created_at"`).
		WillReturnError(errors.New("event insert failed"))
	mock.ExpectRollback()

	claimed, err := repo.ClaimPending(context.Background(), jobID, &model.JobEvent{EventType: "job.running"})
	if err == nil {
		t.Fatal("expected event insertion failure")
	}
	if claimed {
		t.Fatal("claim must not report success when transaction rolls back")
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

	claimed, err := repo.ClaimPending(context.Background(), uuid.New(), &model.JobEvent{EventType: "job.running"})
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

func TestJobRepositoryTransitionStatusUsesCompareAndSetAndAtomicEvent(t *testing.T) {
	repo, mock, cleanup := setupJobRepo(t)
	defer cleanup()
	jobID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "jobs" SET .* WHERE id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "job_events" .* RETURNING "id","payload","created_at"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload", "created_at"}).AddRow(uuid.New(), []byte(`{}`), nil))
	mock.ExpectCommit()

	ok, err := repo.TransitionStatus(
		context.Background(), jobID,
		model.JobStatusRunning, model.JobStatusCompleted,
		map[string]any{"ok": true}, nil,
		&model.JobEvent{EventType: "job.completed"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected transition to win")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
