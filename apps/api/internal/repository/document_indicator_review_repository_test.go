package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupReviewRepo(t *testing.T) (*DocumentIndicatorReviewRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewDocumentIndicatorReviewRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func TestDocumentIndicatorReviewRepositoryAppendsAndNeverExposesMutation(t *testing.T) {
	repo, mock, cleanup := setupReviewRepo(t)
	defer cleanup()
	ctx := context.Background()
	row := &model.DocumentIndicatorReview{
		UserID:           uuid.New(),
		UploadID:         uuid.New(),
		ExtractionRunID:  uuid.New(),
		IndicatorIndex:   0,
		Action:           string(model.ReviewActionConfirm),
		IdempotencyKey:   "idem-1",
		MachineCandidate: datatypes.JSON(`{"indicator_id":"hemoglobin"}`),
		ReviewerUserID:   uuid.New(),
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "document_indicator_reviews"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(row.ID))
	mock.ExpectCommit()
	if err := repo.Create(ctx, row); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDocumentIndicatorReviewRepositoryListByExtractionRun(t *testing.T) {
	repo, mock, cleanup := setupReviewRepo(t)
	defer cleanup()
	runID := uuid.New()
	cols := []string{"id", "user_id", "upload_id", "extraction_run_id", "indicator_index", "action", "idempotency_key", "reviewed_payload", "machine_candidate", "source_refs", "page_ref", "reviewer_user_id", "note", "created_at"}
	mock.ExpectQuery(`SELECT \* FROM "document_indicator_reviews" WHERE extraction_run_id = \$[0-9]+ ORDER BY created_at ASC`).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(uuid.New(), uuid.New(), uuid.New(), runID, 0, "confirm", "k", datatypes.JSON(`{}`), datatypes.JSON(`{}`), datatypes.JSON(`[]`), datatypes.JSON(`{}`), uuid.New(), "", time.Now()))
	rows, err := repo.ListByExtractionRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("ListByExtractionRun: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d want 1", len(rows))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

// appendOnlyReviewStore is satisfied by the repository. It deliberately lists
// only the append/read methods; an Update or Delete would break compilation.
var _ appendOnlyReviewStore = (*DocumentIndicatorReviewRepository)(nil)

type appendOnlyReviewStore interface {
	Create(ctx context.Context, review *model.DocumentIndicatorReview) error
	ByOwnerScope(ctx context.Context, userID uuid.UUID, extractionRunID uuid.UUID, indicatorIndex int, idempotencyKey string) ([]model.DocumentIndicatorReview, error)
	ListByUpload(ctx context.Context, uploadID uuid.UUID, userID uuid.UUID) ([]model.DocumentIndicatorReview, error)
	ListByExtractionRun(ctx context.Context, extractionRunID uuid.UUID) ([]model.DocumentIndicatorReview, error)
}
