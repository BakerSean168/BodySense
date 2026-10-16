package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// These tests exercise PC-D4/PC-D5 against the real baseline schema: one
// Conversation per user and one active Run per user, enforced in PostgreSQL.

func openSingletonTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run singleton Conversation integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createSingletonTestUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	owner := uuid.New()
	if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES (?,?,?)", owner, "singleton-"+owner.String()+"@example.test", "synthetic-hash").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DELETE FROM users WHERE id = ?", owner).Error; err != nil {
			t.Error(err)
		}
	})
	return owner
}

func TestConversationSingletonConcurrentGetOrCreateConvergesOnOneIdentity(t *testing.T) {
	db := openSingletonTestDB(t)
	owner := createSingletonTestUser(t, db)
	repo := NewConversationRepository(db)

	const devices = 8
	ids := make([]uuid.UUID, devices)
	created := make([]bool, devices)
	errs := make([]error, devices)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < devices; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			conversation, wasCreated, err := repo.GetOrCreateForUser(context.Background(), owner)
			errs[i], created[i] = err, wasCreated
			if conversation != nil {
				ids[i] = conversation.ID
			}
		}(i)
	}
	close(start)
	wg.Wait()

	createdCount := 0
	for i := 0; i < devices; i++ {
		if errs[i] != nil {
			t.Fatalf("device %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("devices converged on different identities: %s vs %s", ids[i], ids[0])
		}
		if created[i] {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("exactly one device must create the Conversation, got %d", createdCount)
	}
	var rows, sessions int64
	db.Model(&model.Conversation{}).Where("user_id = ?", owner).Count(&rows)
	db.Model(&model.ConsultationSession{}).Where("conversation_id = ?", ids[0]).Count(&sessions)
	if rows != 1 || sessions != 1 {
		t.Fatalf("want one conversation and one session, got %d / %d", rows, sessions)
	}

	again, wasCreated, err := repo.GetOrCreateForUser(context.Background(), owner)
	if err != nil || wasCreated || again.ID != ids[0] {
		t.Fatalf("second call must return the same identity: id=%v created=%v err=%v", again, wasCreated, err)
	}
	if _, _, err := repo.GetOrCreateForUser(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidConversationOwner) {
		t.Fatalf("nil owner must be rejected before any write, got %v", err)
	}
}

func TestConversationSingletonDatabaseRejectsSecondConversation(t *testing.T) {
	db := openSingletonTestDB(t)
	owner := createSingletonTestUser(t, db)
	if _, _, err := NewConversationRepository(db).GetOrCreateForUser(context.Background(), owner); err != nil {
		t.Fatal(err)
	}
	err := db.Exec("INSERT INTO conversations(user_id) VALUES (?)", owner).Error
	if err == nil || !strings.Contains(err.Error(), "conversations_user_id_key") {
		t.Fatalf("a second Conversation for one user must violate conversations_user_id_key, got %v", err)
	}
}

func TestConversationSingletonDatabaseRejectsSecondActiveRunPerUser(t *testing.T) {
	db := openSingletonTestDB(t)
	owner := createSingletonTestUser(t, db)
	conversation, _, err := NewConversationRepository(db).GetOrCreateForUser(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	insertRun := func(status string) error {
		return db.Exec(
			"INSERT INTO runs(conversation_id,turn_id,request_id,user_id,status,model) VALUES (?,?,?,?,?,?)",
			conversation.ID, uuid.New(), uuid.NewString(), owner, status, "synthetic",
		).Error
	}
	if err := insertRun(string(model.RunStatusWaitingUser)); err != nil {
		t.Fatal(err)
	}
	if err := insertRun(string(model.RunStatusRunning)); err == nil || !strings.Contains(err.Error(), "idx_runs_one_active_per_user") {
		t.Fatalf("a second active Run for one user must violate idx_runs_one_active_per_user, got %v", err)
	}
	if err := insertRun("completed"); err != nil {
		t.Fatalf("terminal Runs are unrestricted: %v", err)
	}
	if !isActiveRunPerUserViolation(errors.New(`ERROR: duplicate key value violates unique constraint "idx_runs_one_active_per_user" (SQLSTATE 23505)`)) {
		t.Fatal("unique violation of the per-user index must map to run-in-progress")
	}
}

func createSingletonTestEnvelope(repo *ConsultationRepository, owner uuid.UUID, conversationID *uuid.UUID) (*model.Run, error) {
	_, run, _, _, _, _, err := repo.CreateRunEnvelope(
		context.Background(), owner, conversationID, uuid.NewString(),
		datatypes.JSON(`[{"type":"text","text":"synthetic"}]`), datatypes.JSON(`{}`), "synthetic",
	)
	return run, err
}

func TestConversationSingletonRunAdmissionIsSerialPerUser(t *testing.T) {
	db := openSingletonTestDB(t)
	owner := createSingletonTestUser(t, db)
	repo := NewConsultationRepository(db, "singleton-test-owner")

	// Two devices start a Run at the same time: exactly one is admitted.
	const devices = 2
	errs := make([]error, devices)
	runs := make([]*model.Run, devices)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < devices; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			runs[i], errs[i] = createSingletonTestEnvelope(repo, owner, nil)
		}(i)
	}
	close(start)
	wg.Wait()
	admitted, busy := 0, 0
	var active *model.Run
	for i := 0; i < devices; i++ {
		switch {
		case errs[i] == nil:
			admitted++
			active = runs[i]
		case errors.Is(errs[i], model.ErrConversationRunInProgress):
			busy++
		default:
			t.Fatalf("device %d: unexpected error %v", i, errs[i])
		}
	}
	if admitted != 1 || busy != 1 {
		t.Fatalf("want one admitted and one busy device, got %d / %d", admitted, busy)
	}

	// A Run blocked on Ask User keeps the user busy; it is never replaced.
	if err := db.Model(&model.Run{}).Where("id = ?", active.ID).Updates(map[string]any{"status": model.RunStatusWaitingUser, "lease_expires_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := createSingletonTestEnvelope(repo, owner, &active.ConversationID); !errors.Is(err, model.ErrConversationRunInProgress) {
		t.Fatalf("waiting_user Run must block a new Run, got %v", err)
	}
	var waiting model.Run
	if err := db.First(&waiting, "id = ?", active.ID).Error; err != nil || waiting.Status != model.RunStatusWaitingUser {
		t.Fatalf("original waiting_user Run must be preserved: %+v err=%v", waiting, err)
	}

	// An expired running lease is reclaimed and the next Run reuses the same Conversation.
	expired := time.Now().Add(-time.Minute)
	if err := db.Model(&model.Run{}).Where("id = ?", active.ID).Updates(map[string]any{"status": model.RunStatusRunning, "lease_expires_at": expired}).Error; err != nil {
		t.Fatal(err)
	}
	next, err := createSingletonTestEnvelope(repo, owner, nil)
	if err != nil {
		t.Fatalf("expired lease must be reclaimed: %v", err)
	}
	if next.ConversationID != active.ConversationID {
		t.Fatalf("new Run must stay in the singleton Conversation: %s vs %s", next.ConversationID, active.ConversationID)
	}
	var reclaimed model.Run
	if err := db.First(&reclaimed, "id = ?", active.ID).Error; err != nil || reclaimed.Status != "failed" {
		t.Fatalf("expired Run must be reclaimed as failed: %+v err=%v", reclaimed, err)
	}
}

func TestConversationSingletonRunRejectsForeignConversationID(t *testing.T) {
	db := openSingletonTestDB(t)
	owner := createSingletonTestUser(t, db)
	other := createSingletonTestUser(t, db)
	foreign, _, err := NewConversationRepository(db).GetOrCreateForUser(context.Background(), other)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewConsultationRepository(db, "singleton-test-owner")
	if _, err := createSingletonTestEnvelope(repo, owner, &foreign.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("another owner's Conversation must be owner-masked as not found, got %v", err)
	}
	unknown := uuid.New()
	if _, err := createSingletonTestEnvelope(repo, owner, &unknown); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("an unknown Conversation ID must be not found, got %v", err)
	}
	var foreignRuns int64
	db.Model(&model.Run{}).Where("conversation_id = ?", foreign.ID).Count(&foreignRuns)
	if foreignRuns != 0 {
		t.Fatalf("no Run may be written into another owner's Conversation, got %d", foreignRuns)
	}
}

func TestConversationSingletonPendingAskUserBlocksNewRunAndStalePointerIsCleared(t *testing.T) {
	db := openSingletonTestDB(t)
	owner := createSingletonTestUser(t, db)
	repo := NewConsultationRepository(db, "singleton-test-owner")
	first, err := createSingletonTestEnvelope(repo, owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The first Run finished but left a pending Ask User interaction.
	if err := db.Model(&model.Run{}).Where("id = ?", first.ID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	interaction := &model.AgentInteraction{
		ID: uuid.New(), RunID: first.ID, ConversationID: first.ConversationID, ToolCallID: "ask-singleton",
		ToolName: "ask_user", Question: datatypes.JSON(`{"question":"synthetic?","answer_type":"text"}`),
		Status: model.AgentInteractionPending, Metadata: datatypes.JSON(`{}`),
	}
	if err := db.Create(interaction).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := createSingletonTestEnvelope(repo, owner, nil); !errors.Is(err, model.ErrConversationRunInProgress) {
		t.Fatalf("pending Ask User must block a new Run, got %v", err)
	}
	if err := db.Model(interaction).Update("status", "cancelled").Error; err != nil {
		t.Fatal(err)
	}
	// The completed Run's pointer is stale; the next Run clears and replaces it.
	second, err := createSingletonTestEnvelope(repo, owner, nil)
	if err != nil {
		t.Fatalf("a terminal Run must not block the user: %v", err)
	}
	var conversation model.Conversation
	if err := db.First(&conversation, "id = ?", first.ConversationID).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.ActiveRunID == nil || *conversation.ActiveRunID != second.ID {
		t.Fatalf("active pointer must name the new Run, got %v", conversation.ActiveRunID)
	}
}
