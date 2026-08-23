package repository

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openRunLeaseIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run lease integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	return db
}

func seedLeaseRun(t *testing.T, db *gorm.DB, status string, leaseExpiry time.Time) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO users(id,email,password_hash) VALUES (?,?,?)",
		userID, "lease-"+userID.String()+"@example.test", "synthetic-hash",
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id = ?", userID).Error })
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO conversations(id,user_id,title,active_run_id) VALUES (?,?,?,NULL)",
		conversationID, userID, "lease test",
	).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	if err := db.WithContext(ctx).Exec(`
INSERT INTO runs(id,conversation_id,turn_id,request_id,user_id,status,model,lease_owner,lease_expires_at,lease_heartbeat_at)
VALUES (?,?,?,?,?,?,?,?,?,?)`,
		runID, conversationID, turnID, "lease-"+runID.String(), userID, status, "synthetic", "worker-a", leaseExpiry, leaseExpiry.Add(-time.Minute),
	).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if err := db.WithContext(ctx).Exec("UPDATE conversations SET active_run_id = ? WHERE id = ?", runID, conversationID).Error; err != nil {
		t.Fatalf("set active run: %v", err)
	}
	return userID, runID
}

func TestRunLeaseConcurrentReconciliationPostgres(t *testing.T) {
	db := openRunLeaseIntegrationDB(t)
	ctx := context.Background()
	_, runID := seedLeaseRun(t, db, "running", time.Now().UTC().Add(-time.Minute))

	const reconcilers = 2
	start := make(chan struct{})
	results := make(chan int, reconcilers)
	errs := make(chan error, reconcilers)
	var wg sync.WaitGroup
	for i := 0; i < reconcilers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			runs, err := NewRunRepository(db).ReclaimExpiredRuns(ctx, time.Now().UTC(), 10)
			results <- len(runs)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("reconciler error: %v", err)
		}
	}
	total := 0
	for count := range results {
		total += count
	}
	if total != 1 {
		t.Fatalf("total reclaimed=%d, want exactly one", total)
	}
	var run model.Run
	if err := db.First(&run, "id = ?", runID).Error; err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Status != "failed" || run.CompletedAt == nil || run.LeaseExpiresAt != nil || run.LeaseOwner != "" {
		t.Fatalf("reclaimed run=%+v", run)
	}
}

func TestRunLeaseCompletionVsReconcilerSingleTerminalWinnerPostgres(t *testing.T) {
	db := openRunLeaseIntegrationDB(t)
	ctx := context.Background()
	userID, runID := seedLeaseRun(t, db, "running", time.Now().UTC().Add(-time.Minute))
	repo := NewRunRepository(db)

	start := make(chan struct{})
	var completed bool
	var reclaimed int
	var completeErr, reclaimErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		completed, completeErr = repo.TryCompleteRun(ctx, runID, userID, nil, "synthetic-response")
	}()
	go func() {
		defer wg.Done()
		<-start
		var runs []model.Run
		runs, reclaimErr = repo.ReclaimExpiredRuns(ctx, time.Now().UTC(), 10)
		reclaimed = len(runs)
	}()
	close(start)
	wg.Wait()

	if completeErr != nil || reclaimErr != nil {
		t.Fatalf("race errors: complete=%v reclaim=%v", completeErr, reclaimErr)
	}
	winnerCount := reclaimed
	if completed {
		winnerCount++
	}
	if winnerCount != 1 {
		t.Fatalf("terminal winners=%d (completed=%v reclaimed=%d), want one", winnerCount, completed, reclaimed)
	}
	var run model.Run
	if err := db.First(&run, "id = ?", runID).Error; err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Status != "completed" && run.Status != "failed" {
		t.Fatalf("final status=%q, want completed or failed", run.Status)
	}
}

func TestRunLeaseWaitingUserIsNeverReclaimedPostgres(t *testing.T) {
	db := openRunLeaseIntegrationDB(t)
	ctx := context.Background()
	_, runID := seedLeaseRun(t, db, "waiting_user", time.Now().UTC().Add(-time.Hour))

	runs, err := NewRunRepository(db).ReclaimExpiredRuns(ctx, time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("ReclaimExpiredRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("reclaimed waiting_user runs=%d, want 0", len(runs))
	}
	var run model.Run
	if err := db.First(&run, "id = ?", runID).Error; err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Status != "waiting_user" {
		t.Fatalf("waiting_user became %q", run.Status)
	}
}
