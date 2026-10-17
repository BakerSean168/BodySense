package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupLifecycleRepos(t *testing.T) (*DocumentExtractionRunRepository, *PostureAnalysisRunRepository, *UserAssetRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewDocumentExtractionRunRepository(gormDB),
		NewPostureAnalysisRunRepository(gormDB),
		NewUserAssetRepository(gormDB),
		mock, func() { _ = sqlDB.Close() }
}

func TestExtractionRunBeginProcessingRequiresPending(t *testing.T) {
	extractionRuns, _, _, mock, cleanup := setupLifecycleRepos(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "document_extraction_runs" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	started, err := extractionRuns.BeginProcessing(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if err != nil || !started {
		t.Fatalf("BeginProcessing started=%v err=%v", started, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractionRunLateFailureCannotOverwriteTerminal(t *testing.T) {
	extractionRuns, _, _, mock, cleanup := setupLifecycleRepos(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "document_extraction_runs" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND status IN \(\$[0-9]+,\$[0-9]+\)`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	updated, err := extractionRuns.FailRun(context.Background(), uuid.New(), uuid.New(), json.RawMessage(`{"error":"late"}`))
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("late extraction failure must not overwrite a terminal run")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func strPtr(value string) *string { return &value }

func TestExtractionRunCompleteRequiresProcessingAndPersistsAudit(t *testing.T) {
	extractionRuns, _, _, mock, cleanup := setupLifecycleRepos(t)
	defer cleanup()
	run := &model.DocumentExtractionRun{
		ID:                  uuid.New(),
		UserID:              uuid.New(),
		MechanismRevision:   strPtr("tesseract-baseline-v3"),
		DocumentSHA256:      strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		ResultSHA256:        strPtr("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		RawTextSHA256:       strPtr("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
		ResultPayload:       datatypes.JSON(`{"status":"completed","result":{}}`),
		IndicatorSnapshot:   datatypes.JSON(`[]`),
		SourceSummary:       datatypes.JSON(`{}`),
		MechanismProvenance: datatypes.JSON(`{}`),
	}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "document_extraction_runs" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	completed, err := extractionRuns.CompleteRun(context.Background(), run)
	if err != nil || !completed {
		t.Fatalf("CompleteRun completed=%v err=%v", completed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostureRunLifecycleCompareAndSet(t *testing.T) {
	_, postureRuns, _, mock, cleanup := setupLifecycleRepos(t)
	defer cleanup()
	runID, userID := uuid.New(), uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "posture_analysis_runs" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	started, err := postureRuns.BeginProcessing(context.Background(), runID, userID, uuid.New())
	if err != nil || !started {
		t.Fatalf("BeginProcessing started=%v err=%v", started, err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "posture_analysis_runs" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND status = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	completed, err := postureRuns.CompleteRun(context.Background(), runID, userID, "posture-config", json.RawMessage(`{"view":"front"}`))
	if err != nil || !completed {
		t.Fatalf("CompleteRun completed=%v err=%v", completed, err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "posture_analysis_runs" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND status IN \(\$[0-9]+,\$[0-9]+\)`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	failed, err := postureRuns.FailRun(context.Background(), runID, userID, json.RawMessage(`{"error":"late"}`))
	if err != nil {
		t.Fatal(err)
	}
	if failed {
		t.Fatal("late posture failure must not overwrite a terminal run")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUserAssetMarkSourceRemovedOnlyFromAvailable(t *testing.T) {
	_, _, assets, mock, cleanup := setupLifecycleRepos(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "user_assets" SET .* WHERE id = \$[0-9]+ AND user_id = \$[0-9]+ AND availability = \$[0-9]+`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	removed, err := assets.MarkSourceRemoved(context.Background(), uuid.New(), uuid.New())
	if err != nil || !removed {
		t.Fatalf("MarkSourceRemoved removed=%v err=%v", removed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUserAssetReadRequiresOwnerInSQL(t *testing.T) {
	_, _, assets, mock, cleanup := setupLifecycleRepos(t)
	defer cleanup()
	id, foreign := uuid.New(), uuid.New()
	mock.ExpectQuery(`SELECT .* FROM "user_assets" WHERE id = \$1 AND user_id = \$2 .*`).
		WithArgs(id, foreign, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}))
	asset, err := assets.GetOwnedByID(context.Background(), id, foreign)
	if err != nil || asset != nil {
		t.Fatalf("foreign lookup asset=%v err=%v", asset, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
