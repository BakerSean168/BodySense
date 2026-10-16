package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This test runs only against an explicitly isolated synthetic PG18 fixture.
// Never set BODYSENSE_INTEGRATION_DATABASE_URL to a staging/production DB.
func TestConversationSingletonPreflightNativePostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires explicit isolated BODYSENSE_INTEGRATION_DATABASE_URL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open synthetic PostgreSQL 18: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	owner, stranger := uuid.New(), uuid.New()
	for _, userID := range []uuid.UUID{owner, stranger} {
		if err := db.WithContext(ctx).Exec(
			"INSERT INTO users(id,email,password_hash) VALUES (?,?,?)",
			userID, "singleton-"+userID.String()+"@example.invalid", "synthetic-only",
		).Error; err != nil {
			t.Fatalf("seed synthetic owner: %v", err)
		}
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM users WHERE id IN (?,?)", owner, stranger).Error
	})
	repo := NewConversationRepository(db)
	lookup := func(userID uuid.UUID) (*uuid.UUID, error) {
		t.Helper()
		conversation, err := repo.GetSoleLiveByUserID(ctx, userID)
		if conversation == nil {
			return nil, err
		}
		return &conversation.ID, err
	}
	requireID := func(userID, expected uuid.UUID) {
		t.Helper()
		actual, err := lookup(userID)
		if err != nil || actual == nil || *actual != expected {
			t.Fatalf("owner lookup got=%v err=%v, expected non-deleted conversation", actual, err)
		}
	}
	requireNone := func(userID uuid.UUID) {
		t.Helper()
		actual, err := lookup(userID)
		if actual != nil || err != nil {
			t.Fatalf("owner lookup got=%v err=%v, expected no live conversation", actual, err)
		}
	}
	requireNone(owner)

	first, second, foreign := uuid.New(), uuid.New(), uuid.New()
	add := func(conversationID, userID uuid.UUID, status string) {
		t.Helper()
		if err := db.WithContext(ctx).Exec(
			"INSERT INTO conversations(id,user_id,status) VALUES (?,?,?)",
			conversationID, userID, status,
		).Error; err != nil {
			t.Fatalf("seed synthetic Conversation: %v", err)
		}
	}

	add(first, owner, "active")
	requireID(owner, first)

	// Distinct non-deleted statuses do not hide historical ambiguity.
	add(second, owner, "archived")
	actual, err := lookup(owner)
	if actual != nil || !errors.Is(err, ErrLegacyMultipleLiveConversations) {
		t.Fatalf("ambiguous legacy owner returned=%v err=%v", actual, err)
	}
	for _, legacyID := range []uuid.UUID{first, second} {
		if err != nil && strings.Contains(err.Error(), legacyID.String()) {
			t.Fatal("legacy Conversation ID leaked through ambiguity error")
		}
	}

	// Soft-deleted history still exists, but is not an active selector.
	if err := db.WithContext(ctx).Exec(
		"UPDATE conversations SET deleted_at = ? WHERE id = ?", time.Now().UTC(), first,
	).Error; err != nil {
		t.Fatal(err)
	}
	actual, err = lookup(owner)
	if actual != nil || !errors.Is(err, ErrLegacyConversationHistory) {
		t.Fatalf("mixed live/deleted history got=%v err=%v; must reconcile before creation", actual, err)
	}

	// The reader must be tenant-scoped even while another owner has rows.
	add(foreign, stranger, "active")
	actual, err = lookup(owner)
	if actual != nil || !errors.Is(err, ErrLegacyConversationHistory) {
		t.Fatalf("foreign owner cannot hide old identity: got=%v err=%v", actual, err)
	}
	requireID(stranger, foreign)

	if err := db.WithContext(ctx).Exec(
		"UPDATE conversations SET deleted_at = ? WHERE id = ?", time.Now().UTC(), second,
	).Error; err != nil {
		t.Fatal(err)
	}
	actual, err = lookup(owner)
	if actual != nil || !errors.Is(err, ErrLegacyConversationHistory) {
		t.Fatalf("soft-deleted-only owner got=%v err=%v; should remain blocked", actual, err)
	}
	requireID(stranger, foreign)

	var total int64
	if err := db.Table("conversations").Where("user_id = ?", owner).Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("read-only singleton lookup unexpectedly modified history: got=%d, want 2", total)
	}

	found, err := repo.GetSoleLiveByUserID(ctx, uuid.Nil)
	if found != nil || !errors.Is(err, ErrInvalidConversationOwner) {
		t.Fatalf("invalid owner did not fail closed: found=%v err=%v", found, err)
	}
}
