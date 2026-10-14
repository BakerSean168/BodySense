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

func setupSingletonConversationMock(t *testing.T) (*ConversationRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return NewConversationRepository(db), mock
}

func singletonConversationRows(ids ...uuid.UUID) *sqlmock.Rows {
	return singletonConversationRowsWithTombstones(ids, make([]bool, len(ids)))
}

func singletonConversationRowsWithTombstones(ids []uuid.UUID, tombstones []bool) *sqlmock.Rows {
	if len(ids) != len(tombstones) {
		panic("synthetic singleton fixture flags must align")
	}
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "status", "created_at", "updated_at", "metadata", "deleted_at",
	})
	for i, id := range ids {
		var deletedAt any
		if tombstones[i] {
			deletedAt = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		}
		rows.AddRow(id,
			uuid.MustParse("10000000-0000-4000-8000-000000000001"),
			"archived", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), []byte("{}"), deletedAt)
	}
	return rows
}

func expectSoleConversationLookup(mock sqlmock.Sqlmock, userID uuid.UUID, rows *sqlmock.Rows) {
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM \"conversations\" WHERE user_id = $1 ORDER BY deleted_at ASC NULLS FIRST,created_at ASC,id ASC LIMIT $2",
	)).WithArgs(userID, 2).WillReturnRows(rows)
}

func TestConversationSingletonPreflightRejectsEmptyOwnerWithoutSQL(t *testing.T) {
	repo, mock := setupSingletonConversationMock(t)
	found, err := repo.GetSoleLiveByUserID(context.Background(), uuid.Nil)
	if found != nil || !errors.Is(err, ErrInvalidConversationOwner) {
		t.Fatalf("found=%v err=%v; expected rejected unbound owner", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConversationSingletonPreflightAllowsLazyCreation(t *testing.T) {
	repo, mock := setupSingletonConversationMock(t)
	owner := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	expectSoleConversationLookup(mock, owner, singletonConversationRows())
	found, err := repo.GetSoleLiveByUserID(context.Background(), owner)
	if found != nil || err != nil {
		t.Fatalf("found=%v err=%v; expected nil,nil", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConversationSingletonPreflightPreservesNonDeletedArchivedRow(t *testing.T) {
	repo, mock := setupSingletonConversationMock(t)
	owner := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	id := uuid.MustParse("20000000-0000-4000-8000-000000000001")
	expectSoleConversationLookup(mock, owner, singletonConversationRows(id))
	found, err := repo.GetSoleLiveByUserID(context.Background(), owner)
	if err != nil || found == nil || found.ID != id || found.Status != "archived" {
		t.Fatalf("found=%v err=%v; expected original archived row", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestConversationSingletonPreflightFailsClosedOnTwoLegacyRows(t *testing.T) {
	repo, mock := setupSingletonConversationMock(t)
	owner := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	first := uuid.MustParse("20000000-0000-4000-8000-000000000001")
	second := uuid.MustParse("20000000-0000-4000-8000-000000000002")
	expectSoleConversationLookup(mock, owner, singletonConversationRows(first, second))
	found, err := repo.GetSoleLiveByUserID(context.Background(), owner)
	if found != nil || !errors.Is(err, ErrLegacyMultipleLiveConversations) {
		t.Fatalf("found=%v err=%v; wanted ambiguous history without selection", found, err)
	}
	for _, id := range []uuid.UUID{owner, first, second} {
		if strings.Contains(err.Error(), id.String()) {
			t.Fatal("legacy identifiers leaked from error")
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Deleted-only and mixed history must NOT look like a first-time user;
// the approved singleton identity may only be created after reconciliation.
func TestConversationSingletonPreflightBlocksHistoricalTombstones(t *testing.T) {
	owner := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	live := uuid.MustParse("20000000-0000-4000-8000-000000000001")
	deleted := uuid.MustParse("20000000-0000-4000-8000-000000000002")
	cases := []struct {
		name       string
		ids        []uuid.UUID
		tombstones []bool
	}{
		{name: "deleted-only", ids: []uuid.UUID{deleted}, tombstones: []bool{true}},
		{name: "mixed", ids: []uuid.UUID{live, deleted}, tombstones: []bool{false, true}},
		{name: "all-deleted", ids: []uuid.UUID{live, deleted}, tombstones: []bool{true, true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := setupSingletonConversationMock(t)
			expectSoleConversationLookup(mock, owner,
				singletonConversationRowsWithTombstones(tc.ids, tc.tombstones))
			found, err := repo.GetSoleLiveByUserID(context.Background(), owner)
			if found != nil || !errors.Is(err, ErrLegacyConversationHistory) {
				t.Fatalf("found=%v err=%v; expected history reconciliation", found, err)
			}
			for _, id := range tc.ids {
				if strings.Contains(err.Error(), id.String()) {
					t.Fatal("historical IDs leaked to caller")
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConversationSingletonPreflightPropagatesDatabaseFailure(t *testing.T) {
	repo, mock := setupSingletonConversationMock(t)
	owner := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	sentinel := errors.New("synthetic database read failure")
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM \"conversations\" WHERE user_id = $1 ORDER BY deleted_at ASC NULLS FIRST,created_at ASC,id ASC LIMIT $2",
	)).WithArgs(owner, 2).WillReturnError(sentinel)
	found, err := repo.GetSoleLiveByUserID(context.Background(), owner)
	if found != nil || !errors.Is(err, sentinel) {
		t.Fatalf("found=%v err=%v; must propagate read failure", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
