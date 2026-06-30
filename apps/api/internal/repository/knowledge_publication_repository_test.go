package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupKnowledgePublicationRepo(t *testing.T) (*KnowledgePublicationRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewKnowledgePublicationRepository(gormDB), mock, func() { _ = sqlDB.Close() }
}

func knowledgePublicationRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id",
		"knowledge_unit_id",
		"publication_key",
		"title",
		"published_version",
		"published_at",
		"published_by",
		"created_by",
		"status",
		"metadata",
		"created_at",
		"updated_at",
	}).AddRow(
		uuid.New(),
		int64(42),
		"unit-42-v1",
		"Test publication",
		1,
		now,
		"reviewer",
		"author",
		"published",
		[]byte(`{}`),
		now,
		now,
	)
}

func TestKnowledgePublicationRepositoryGetByID(t *testing.T) {
	repo, mock, cleanup := setupKnowledgePublicationRepo(t)
	defer cleanup()

	id := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "knowledge_publications" WHERE id = $1 ORDER BY "knowledge_publications"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(knowledgePublicationRows())

	pub, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if pub == nil {
		t.Fatal("expected publication, got nil")
	}
	if pub.KnowledgeUnitID == nil || *pub.KnowledgeUnitID != 42 || pub.PublicationKey != "unit-42-v1" {
		t.Errorf("unexpected publication: %+v", pub)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestKnowledgePublicationRepositoryGetByIDNotFound(t *testing.T) {
	repo, mock, cleanup := setupKnowledgePublicationRepo(t)
	defer cleanup()

	id := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "knowledge_publications" WHERE id = $1 ORDER BY "knowledge_publications"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	pub, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if pub != nil {
		t.Fatalf("expected nil publication, got %+v", pub)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestKnowledgePublicationRepositoryListByStatus(t *testing.T) {
	repo, mock, cleanup := setupKnowledgePublicationRepo(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "knowledge_publications" WHERE status = $1 ORDER BY created_at DESC`)).
		WithArgs("published").
		WillReturnRows(knowledgePublicationRows())

	pubs, err := repo.ListByStatus(context.Background(), "published")
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if len(pubs) != 1 {
		t.Fatalf("expected 1 publication, got %d", len(pubs))
	}
	if pubs[0].Status != "published" {
		t.Errorf("status = %q, want published", pubs[0].Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
