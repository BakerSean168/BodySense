package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupUploadLifecycleRepo(t *testing.T) (*UploadRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewUploadRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func TestUploadRepositoryBeginOCRRequiresPending(t *testing.T) {
	repo, mock, cleanup := setupUploadLifecycleRepo(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "user_uploads" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND ocr_status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := repo.BeginOCR(context.Background(), uuid.New(), uuid.New())
	if err != nil || !updated {
		t.Fatalf("BeginOCR updated=%v err=%v", updated, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRepositoryLateOCRFailureCannotOverwriteTerminal(t *testing.T) {
	repo, mock, cleanup := setupUploadLifecycleRepo(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "user_uploads" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND ocr_status IN \(\$[0-9]+,\$[0-9]+\)`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	updated, err := repo.FailOCR(context.Background(), uuid.New(), uuid.New(), json.RawMessage(`{"error":"late"}`))
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("late OCR failure must not overwrite a terminal state")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRepositoryCompleteAnalysisRequiresProcessing(t *testing.T) {
	repo, mock, cleanup := setupUploadLifecycleRepo(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "user_uploads" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND analysis_status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := repo.CompleteAnalysis(context.Background(), uuid.New(), uuid.New(), json.RawMessage(`{"ok":true}`))
	if err != nil || !updated {
		t.Fatalf("CompleteAnalysis updated=%v err=%v", updated, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRepositoryLateAnalysisFailureCannotOverwriteTerminal(t *testing.T) {
	repo, mock, cleanup := setupUploadLifecycleRepo(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "user_uploads" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND analysis_status IN \(\$[0-9]+,\$[0-9]+\)`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	updated, err := repo.FailAnalysis(context.Background(), uuid.New(), uuid.New(), json.RawMessage(`{"error":"late"}`))
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("late posture failure must not overwrite a terminal state")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
