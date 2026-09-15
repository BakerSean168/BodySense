package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTreatmentLifecycleRepo(t *testing.T) (*TreatmentRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewTreatmentRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func TestTreatmentRepositoryRejectProposalUsesCompareAndSet(t *testing.T) {
	repo, mock, cleanup := setupTreatmentLifecycleRepo(t)
	defer cleanup()

	userID := uuid.New()
	revisionID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "treatment_revisions" SET .* WHERE id = \$[0-9]+ AND acceptance_state = \$[0-9]+ AND treatment_id IN \(SELECT id FROM treatments WHERE user_id = \$[0-9]+\)`).
		WithArgs(model.TreatmentAcceptanceRejected, revisionID, model.TreatmentAcceptanceProposed, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "interventions" SET .* WHERE treatment_revision_id = \$[0-9]+ AND status = \$[0-9]+`).
		WithArgs("cancelled", sqlmock.AnyArg(), revisionID, "proposed").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.RejectRevision(context.Background(), userID, revisionID); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTreatmentRepositoryLateRejectCannotOverwriteAcceptedRevision(t *testing.T) {
	repo, mock, cleanup := setupTreatmentLifecycleRepo(t)
	defer cleanup()

	userID := uuid.New()
	treatmentID := uuid.New()
	revisionID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "treatment_revisions" SET .* WHERE id = \$[0-9]+ AND acceptance_state = \$[0-9]+ AND treatment_id IN \(SELECT id FROM treatments WHERE user_id = \$[0-9]+\)`).
		WithArgs(model.TreatmentAcceptanceRejected, revisionID, model.TreatmentAcceptanceProposed, userID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT .* FROM "treatment_revisions" JOIN treatments ON treatments.id = treatment_revisions.treatment_id WHERE .*`).
		WithArgs(revisionID, userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "treatment_id", "acceptance_state"}).
			AddRow(revisionID, treatmentID, model.TreatmentAcceptanceAccepted))
	mock.ExpectRollback()

	err := repo.RejectRevision(context.Background(), userID, revisionID)
	if err == nil || !strings.Contains(err.Error(), "accepted treatment revision cannot be rejected") {
		t.Fatalf("RejectRevision err=%v, want accepted-revision rejection", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
