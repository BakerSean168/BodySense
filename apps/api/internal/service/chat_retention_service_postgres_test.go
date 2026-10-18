package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeCheckpointService stands in for the ai-service cleanup endpoint.
type fakeCheckpointService struct {
	mu       sync.Mutex
	fail     bool
	requests []checkpointCleanupRequest
	paths    []string
}

func (f *fakeCheckpointService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req checkpointCleanupRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.paths = append(f.paths, r.URL.Path)
	fail := f.fail
	f.mu.Unlock()
	if fail {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"detail":{"code":"checkpoint_cleanup_failed"}}`))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "cleaned", "thread_id": req.ConversationID, "pruned_run_ids": req.PrunedRunIDs,
		"checkpoint_id": "checkpoint-1", "removed_messages": 2 * len(req.PrunedRunIDs),
		"retained_messages": 2, "deleted_checkpoints": 5, "remaining_run_ids": []string{},
	})
}

func (f *fakeCheckpointService) setFail(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = fail
}

func (f *fakeCheckpointService) last() (checkpointCleanupRequest, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1], f.paths[len(f.paths)-1]
}

func (f *fakeCheckpointService) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

type pruneJobFixture struct {
	t            *testing.T
	db           *gorm.DB
	owner        uuid.UUID
	conversation uuid.UUID
	seq          int
	repo         *repository.ChatRetentionRepository
	jobs         *JobRuntime
	ai           *fakeCheckpointService
	svc          *ChatRetentionService
}

func newPruneJobFixture(t *testing.T) *pruneJobFixture {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run chat retention job integration tests")
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
	owner := uuid.New()
	if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES (?,?,?)", owner, "retention-"+owner.String()+"@example.test", "synthetic-hash").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id = ?", owner).Error })
	conversation, _, err := repository.NewConversationRepository(db).GetOrCreateForUser(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	ai := &fakeCheckpointService{}
	server := httptest.NewServer(ai)
	t.Cleanup(server.Close)
	f := &pruneJobFixture{
		t: t, db: db, owner: owner, conversation: conversation.ID,
		repo: repository.NewChatRetentionRepository(db),
		jobs: NewJobRuntime(repository.NewJobRepository(db)), ai: ai,
	}
	f.svc = NewChatRetentionService(f.repo, &AIClient{httpClient: server.Client(), baseURL: server.URL}, f.jobs)
	return f
}

func (f *pruneJobFixture) exec(query string, args ...any) {
	f.t.Helper()
	if err := f.db.Exec(query, args...).Error; err != nil {
		f.t.Fatal(err)
	}
}

// addTurn appends a user+assistant interaction under one Run.
func (f *pruneJobFixture) addTurn(status string) uuid.UUID {
	f.t.Helper()
	run, turn := uuid.New(), uuid.New()
	f.exec(`INSERT INTO runs(id,conversation_id,turn_id,request_id,user_id,status,model) VALUES (?,?,?,?,?,?,'synthetic')`,
		run, f.conversation, turn, "req-"+run.String(), f.owner, status)
	for _, role := range []string{"user", "assistant"} {
		f.seq++
		f.exec(`INSERT INTO messages(conversation_id,turn_id,run_id,role,status,seq,content_text) VALUES (?,?,?,?,'completed',?,'synthetic')`,
			f.conversation, turn, run, role, f.seq)
	}
	return run
}

func (f *pruneJobFixture) advance(processed, contextStart int) {
	f.t.Helper()
	ctx := context.Background()
	c, _ := f.repo.Get(ctx, f.owner)
	if _, err := f.repo.AdvanceProcessed(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: c.RetentionRevision, ThroughSeq: processed}); err != nil {
		f.t.Fatal(err)
	}
	c, _ = f.repo.Get(ctx, f.owner)
	if _, err := f.repo.AdvanceContextStart(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: c.RetentionRevision, ThroughSeq: contextStart}); err != nil {
		f.t.Fatal(err)
	}
}

// runJob claims and executes exactly this job, like the worker does.
func (f *pruneJobFixture) runJob(id uuid.UUID) *model.Job {
	f.t.Helper()
	ctx := context.Background()
	claimed, ok, err := f.jobs.ClaimPending(ctx, id)
	if err != nil || !ok {
		f.t.Fatalf("claim %s: ok=%v %v", id, ok, err)
	}
	if err := f.svc.execute(ctx, claimed); err != nil {
		f.t.Fatal(err)
	}
	job, err := f.jobs.GetJob(ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return job
}

func (f *pruneJobFixture) pruned() int {
	f.t.Helper()
	c, err := f.repo.Get(context.Background(), f.owner)
	if err != nil {
		f.t.Fatal(err)
	}
	return c.PrunedThroughSeq
}

func (f *pruneJobFixture) messages() int64 {
	var n int64
	f.db.Raw("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", f.conversation).Scan(&n)
	return n
}

func TestChatRetentionPruneJobRetriesFailedCleanupPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	first, second := f.addTurn("completed"), f.addTurn("completed")
	f.addTurn("completed")
	f.advance(6, 5)

	job, err := f.svc.EnqueuePrune(ctx, f.owner, "run:"+second.String())
	if err != nil {
		t.Fatal(err)
	}
	if again, err := f.svc.EnqueuePrune(ctx, f.owner, "run:"+second.String()); err != nil || again.ID != job.ID {
		t.Fatalf("same trigger enqueued twice: %v", err)
	}

	// The ai-service cannot clean: the job goes back to the queue, R stays.
	f.ai.setFail(true)
	for attempt := 1; attempt < ChatRetentionPruneMaxAttempts; attempt++ {
		got := f.runJob(job.ID)
		if got.Status != model.JobStatusPending || got.Attempts != attempt || len(got.Error) == 0 {
			t.Fatalf("attempt %d: job=%s attempts=%d", attempt, got.Status, got.Attempts)
		}
		if f.pruned() != 0 || f.messages() != 6 {
			t.Fatal("failed cleanup moved R or deleted messages")
		}
	}
	if got := f.runJob(job.ID); got.Status != model.JobStatusFailed {
		t.Fatalf("exhausted job not failed: %s", got.Status)
	}
	if f.ai.calls() != ChatRetentionPruneMaxAttempts || f.pruned() != 0 {
		t.Fatal("retry budget not honoured")
	}

	// A later trigger succeeds once the cleanup does.
	f.ai.setFail(false)
	retry, err := f.svc.EnqueuePrune(ctx, f.owner, "manual")
	if err != nil || retry.ID == job.ID {
		t.Fatalf("new trigger not enqueued: %v", err)
	}
	done := f.runJob(retry.ID)
	if done.Status != model.JobStatusCompleted {
		t.Fatalf("job=%s", done.Status)
	}
	var receipt model.ChatRetentionPruneReceipt
	if err := json.Unmarshal(done.Result, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.JobID != retry.ID || receipt.PrunedThroughSeq != 4 || receipt.DeletedMessages != 4 ||
		receipt.CheckpointCleanup != "verified" || receipt.CheckpointsDeleted != 5 || receipt.CheckpointID != "checkpoint-1" {
		t.Fatalf("receipt=%#v", receipt)
	}
	if f.pruned() != 4 || f.messages() != 2 {
		t.Fatal("verified pass did not apply")
	}
	last, path := f.ai.last()
	want := []string{first.String(), second.String()}
	slices.Sort(want)
	if path != "/runtime/threads/"+f.conversation.String()+"/checkpoints/cleanup" ||
		last.ConversationID != f.conversation.String() || last.UserID != f.owner.String() || !slices.Equal(last.PrunedRunIDs, want) {
		t.Fatalf("cleanup request=%#v path=%s", last, path)
	}
	// The worker path finds nothing left to do.
	extra, _ := f.svc.EnqueuePrune(ctx, f.owner, "after")
	if got := f.runJob(extra.ID); got.Status != model.JobStatusCompleted || f.ai.calls() != ChatRetentionPruneMaxAttempts+1 {
		t.Fatalf("no-op pass called the ai-service or did not complete: %s", got.Status)
	}
}

func TestChatRetentionPruneJobWaitsForAnInterruptedRunPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(4, 3)
	waiting := f.addTurn("waiting_user")
	f.exec(`INSERT INTO agent_interactions(run_id,conversation_id,tool_call_id,question,status) VALUES (?,?,'ask-1','{"q":"where"}','pending')`,
		waiting, f.conversation)

	job, err := f.svc.EnqueuePrune(ctx, f.owner, "run:"+waiting.String())
	if err != nil {
		t.Fatal(err)
	}
	got := f.runJob(job.ID)
	var result map[string]any
	_ = json.Unmarshal(got.Result, &result)
	if got.Status != model.JobStatusCompleted || result["outcome"] != "deferred" {
		t.Fatalf("job=%s result=%v", got.Status, result)
	}
	// The interrupted thread was never touched, so its resume cannot land on
	// a rewritten checkpoint.
	if f.ai.calls() != 0 || f.pruned() != 0 || f.messages() != 6 {
		t.Fatal("pass ran beside an interrupted Run")
	}

	// The answer settles the Run; the next trigger prunes.
	f.exec("UPDATE runs SET status = 'completed' WHERE id = ?", waiting)
	f.exec("UPDATE agent_interactions SET status = 'answered' WHERE run_id = ?", waiting)
	next, _ := f.svc.EnqueuePrune(ctx, f.owner, "settled:"+waiting.String())
	if got := f.runJob(next.ID); got.Status != model.JobStatusCompleted || f.pruned() != 2 || f.ai.calls() != 1 {
		t.Fatalf("job=%s pruned=%d calls=%d", got.Status, f.pruned(), f.ai.calls())
	}
}

func TestChatRetentionStalePruneJobIsRecoveredPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(4, 3)
	job, err := f.svc.EnqueuePrune(ctx, f.owner, "stale")
	if err != nil {
		t.Fatal(err)
	}
	// A worker claimed it and died.
	if _, ok, err := f.jobs.ClaimPending(ctx, job.ID); err != nil || !ok {
		t.Fatal(err)
	}
	// A negative staleness window treats every running prune job as stale.
	if _, err := f.svc.RunPendingPrunes(ctx, 100, -time.Second); err != nil {
		t.Fatal(err)
	}
	got, _ := f.jobs.GetJob(ctx, job.ID)
	if got.Status != model.JobStatusCompleted || got.Attempts != 2 || f.pruned() != 2 {
		t.Fatalf("job=%s attempts=%d pruned=%d", got.Status, got.Attempts, f.pruned())
	}
}

// addSizedTurn appends a user+assistant interaction whose two messages are
// each ~500 estimated tokens (1012 a turn), so two turns overflow the minimum
// 1024-token context budget.
func (f *pruneJobFixture) addSizedTurn(status string) uuid.UUID {
	f.t.Helper()
	run, turn := uuid.New(), uuid.New()
	f.exec(`INSERT INTO runs(id,conversation_id,turn_id,request_id,user_id,status,model) VALUES (?,?,?,?,?,?,'synthetic')`,
		run, f.conversation, turn, "req-"+run.String(), f.owner, status)
	for _, role := range []string{"user", "assistant"} {
		f.seq++
		f.exec(`INSERT INTO messages(conversation_id,turn_id,run_id,role,status,seq,content_text) VALUES (?,?,?,?,'completed',?,repeat('x', 1500))`,
			f.conversation, turn, run, role, f.seq)
	}
	return run
}

func (f *pruneJobFixture) pruneJobs() int64 {
	var n int64
	f.db.Raw("SELECT COUNT(*) FROM jobs WHERE user_id = ? AND job_type = ?", f.owner, model.ChatRetentionPruneJobType).Scan(&n)
	return n
}

func TestChatRetentionSettleEnqueuesOnePrunePerWindowPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.svc.WithContextTokenBudget(model.MinChatContextTokenBudget)
	for range 3 {
		f.addSizedTurn("completed")
	}

	cursor, job, err := f.svc.ReconcileWindow(ctx, f.owner)
	if err != nil || job == nil {
		t.Fatalf("reconcile: job=%v %v", job, err)
	}
	if cursor.ProcessedThroughSeq != 6 || cursor.ContextStartSeq != 5 {
		t.Fatalf("cursor %+v, want P=6 C=5", cursor)
	}
	// Settling again with the same window is the same job.
	f.svc.OnRunSettled(ctx, f.owner)
	_, again, err := f.svc.ReconcileWindow(ctx, f.owner)
	if err != nil || again == nil || again.ID != job.ID || f.pruneJobs() != 1 {
		t.Fatalf("repeat enqueue: job=%v jobs=%d %v", again, f.pruneJobs(), err)
	}
	if got := f.runJob(job.ID); got.Status != model.JobStatusCompleted || f.pruned() != 4 || f.messages() != 2 {
		t.Fatalf("job=%s pruned=%d messages=%d", got.Status, f.pruned(), f.messages())
	}
	// C - 1 is no longer past R: nothing more to enqueue.
	if _, next, err := f.svc.ReconcileWindow(ctx, f.owner); err != nil || next != nil || f.pruneJobs() != 1 {
		t.Fatalf("after prune: job=%v jobs=%d %v", next, f.pruneJobs(), err)
	}
}

func TestChatRetentionUnansweredAskUserIsNeverPrunedPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.svc.WithContextTokenBudget(model.MinChatContextTokenBudget)
	f.addSizedTurn("completed")
	waiting := f.addSizedTurn("waiting_user")
	f.exec(`INSERT INTO agent_interactions(run_id,conversation_id,tool_call_id,question,status) VALUES (?,?,'ask-1','{"q":"where"}','pending')`,
		waiting, f.conversation)

	cursor, job, err := f.svc.ReconcileWindow(ctx, f.owner)
	if err != nil || job == nil {
		t.Fatalf("reconcile: job=%v %v", job, err)
	}
	if cursor.ProcessedThroughSeq != 2 || cursor.ContextStartSeq != 3 {
		t.Fatalf("cursor %+v, want P=2 C=3", cursor)
	}
	got := f.runJob(job.ID)
	var result map[string]any
	_ = json.Unmarshal(got.Result, &result)
	if result["outcome"] != "deferred" || f.pruned() != 0 || f.messages() != 4 || f.ai.calls() != 0 {
		t.Fatalf("pass beside an unanswered Ask User: result=%v pruned=%d messages=%d", result, f.pruned(), f.messages())
	}

	// Answering settles the Run; the settle trigger processes it, keeps it
	// in context (it fits the budget) and prunes only the turn before it.
	f.exec("UPDATE runs SET status = 'completed' WHERE id = ?", waiting)
	f.exec("UPDATE agent_interactions SET status = 'answered' WHERE run_id = ?", waiting)
	cursor, next, err := f.svc.ReconcileWindow(ctx, f.owner)
	if err != nil || next == nil || next.ID == job.ID {
		t.Fatalf("settle: job=%v %v", next, err)
	}
	if cursor.ProcessedThroughSeq != 4 || cursor.ContextStartSeq != 3 {
		t.Fatalf("cursor %+v, want P=4 C=3", cursor)
	}
	if got := f.runJob(next.ID); got.Status != model.JobStatusCompleted || f.pruned() != 2 || f.messages() != 2 {
		t.Fatalf("job=%s pruned=%d messages=%d", got.Status, f.pruned(), f.messages())
	}
	var kept int64
	f.db.Raw("SELECT COUNT(*) FROM messages WHERE run_id = ?", waiting).Scan(&kept)
	if kept != 2 {
		t.Fatalf("answered Ask User run lost %d messages", 2-kept)
	}
}

// An Ask User that expires was the only thing keeping its Run unsettled. The
// expiry worker fires the settle trigger, so the window moves and the turn
// before it is pruned without waiting for another Run to settle.
func TestChatRetentionAskUserExpiryTriggersSettlePostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.svc.WithContextTokenBudget(model.MinChatContextTokenBudget)
	f.addSizedTurn("completed")
	waiting := f.addSizedTurn("waiting_user")
	f.exec(`INSERT INTO agent_interactions(run_id,conversation_id,tool_call_id,question,status,expires_at)
VALUES (?,?,'ask-expiring','{"q":"where"}','pending',now() - interval '1 minute')`, waiting, f.conversation)

	cursor, job, err := f.svc.ReconcileWindow(ctx, f.owner)
	if err != nil || job == nil || cursor.ProcessedThroughSeq != 2 || cursor.ContextStartSeq != 3 {
		t.Fatalf("before expiry: cursor=%+v job=%v %v", cursor, job, err)
	}
	if got := f.runJob(job.ID); f.pruned() != 0 || got.Status != model.JobStatusCompleted {
		t.Fatalf("pass beside a pending Ask User pruned %d (job %s)", f.pruned(), got.Status)
	}

	interactions := NewAgentInteractionService(
		repository.NewAgentInteractionRepository(f.db),
		NewRunService(repository.NewRunRepository(f.db)),
		repository.NewConversationRepository(f.db),
		database.NewTransactionManager(f.db),
	).WithLifecycleEvents(&fakeInteractionLifecycleEvents{}).WithRetentionSettle(f.svc)
	expired, err := interactions.ExpireExpiredInteractions(ctx, 100)
	if err != nil || !slices.ContainsFunc(expired, func(i model.AgentInteraction) bool { return i.RunID == waiting }) {
		t.Fatalf("expiry sweep: %d expired, %v", len(expired), err)
	}
	var status string
	f.db.Raw("SELECT status FROM runs WHERE id = ?", waiting).Scan(&status)
	if status != string(model.RunStatusFailed) {
		t.Fatalf("expired Ask User left its Run %q", status)
	}

	// No other Run settled: only the expiry trigger can have moved P.
	c, err := f.repo.Get(ctx, f.owner)
	if err != nil || c.ProcessedThroughSeq != 4 || c.ContextStartSeq != 3 {
		t.Fatalf("after expiry cursor=%+v %v, want P=4 C=3", c, err)
	}
	var next model.Job
	key := repository.PruneJobKey(f.conversation, 0, "window:4:3")
	if err := f.db.Where("user_id = ? AND idempotency_key = ?", f.owner, key).First(&next).Error; err != nil {
		t.Fatalf("expiry enqueued no prune job %s: %v", key, err)
	}
	if got := f.runJob(next.ID); got.Status != model.JobStatusCompleted || f.pruned() != 2 || f.messages() != 2 {
		t.Fatalf("job=%s pruned=%d messages=%d", got.Status, f.pruned(), f.messages())
	}
	// A second sweep finds nothing and triggers nothing new.
	if again, err := interactions.ExpireExpiredInteractions(ctx, 100); err != nil || slices.ContainsFunc(again, func(i model.AgentInteraction) bool { return i.RunID == waiting }) {
		t.Fatalf("second sweep: %v %v", again, err)
	}
}
