package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PC-RET-001 against the real baseline: three retention cursors and the
// controlled prune pass (plan, checkpoint cleanup receipt, commit). Every test
// owns fresh synthetic users.

type retentionFixture struct {
	t            *testing.T
	db           *gorm.DB
	repo         *ChatRetentionRepository
	owner        uuid.UUID
	conversation uuid.UUID
	seq          int
}

type retentionTurn struct {
	run      *uuid.UUID
	turn     uuid.UUID
	user     uuid.UUID
	reply    uuid.UUID
	userSeq  int
	replySeq int
}

func newRetentionFixture(t *testing.T, db *gorm.DB) *retentionFixture {
	t.Helper()
	owner := createSingletonTestUser(t, db)
	conversation, _, err := NewConversationRepository(db).GetOrCreateForUser(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	return &retentionFixture{t: t, db: db, repo: NewChatRetentionRepository(db), owner: owner, conversation: conversation.ID}
}

func (f *retentionFixture) exec(query string, args ...any) {
	f.t.Helper()
	if err := f.db.Exec(query, args...).Error; err != nil {
		f.t.Fatalf("%s: %v", strings.Fields(query)[0:3], err)
	}
}

// addTurn appends a user+assistant interaction with a Run of the given status
// (empty = no Run), two runtime events and one tool call.
func (f *retentionFixture) addTurn(runStatus string, text string) retentionTurn {
	f.t.Helper()
	turn := retentionTurn{turn: uuid.New(), user: uuid.New(), reply: uuid.New()}
	if runStatus != "" {
		id := uuid.New()
		turn.run = &id
		f.exec(`INSERT INTO runs(id,conversation_id,turn_id,request_id,user_id,status,model) VALUES (?,?,?,?,?,?,'synthetic')`,
			id, f.conversation, turn.turn, "req-"+id.String(), f.owner, runStatus)
		f.exec(`INSERT INTO runtime_events(conversation_id,run_id,turn_id,seq,channel,type) VALUES (?,?,?,1,'stream','stream.start'),(?,?,?,2,'stream','stream.done')`,
			f.conversation, id, turn.turn, f.conversation, id, turn.turn)
		f.exec(`INSERT INTO agent_tool_calls(run_id,conversation_id,tool_call_id,tool_name,arguments) VALUES (?,?,?,'lookup','{"q":"synthetic"}')`,
			id, f.conversation, "tool-"+id.String())
	}
	f.seq++
	turn.userSeq = f.seq
	f.exec(`INSERT INTO messages(id,conversation_id,turn_id,run_id,role,status,seq,content_text) VALUES (?,?,?,?,'user','completed',?,?)`,
		turn.user, f.conversation, turn.turn, turn.run, f.seq, text)
	f.seq++
	turn.replySeq = f.seq
	f.exec(`INSERT INTO messages(id,conversation_id,turn_id,run_id,role,status,seq,content_text) VALUES (?,?,?,?,'assistant','completed',?,?)`,
		turn.reply, f.conversation, turn.turn, turn.run, f.seq, "reply about "+text)
	return turn
}

func (f *retentionFixture) addEvidence(owner uuid.UUID, turn retentionTurn) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO statement_evidence(id,user_id,source_kind,source_version,excerpt,availability,recorded_at,source_conversation_id,source_run_id,source_message_id)
VALUES (?,?,'user_message',1,'synthetic excerpt','available',now(),?,?,?)`, id, owner, f.conversation, turn.run, turn.user)
	return id
}

func (f *retentionFixture) count(query string, args ...any) int64 {
	f.t.Helper()
	var n int64
	if err := f.db.Raw(query, args...).Scan(&n).Error; err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *retentionFixture) messageExists(id uuid.UUID) bool {
	return f.count("SELECT COUNT(*) FROM messages WHERE id = ?", id) == 1
}

func (f *retentionFixture) runEvents(run *uuid.UUID) int64 {
	return f.count("SELECT COUNT(*) FROM runtime_events WHERE run_id = ?", *run)
}

func (f *retentionFixture) cursor() *model.ChatRetentionCursor {
	f.t.Helper()
	c, err := f.repo.Get(context.Background(), f.owner)
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

func (f *retentionFixture) advance(processed, contextStart int) {
	f.t.Helper()
	ctx := context.Background()
	if _, err := f.repo.AdvanceProcessed(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: f.cursor().RetentionRevision, ThroughSeq: processed}); err != nil {
		f.t.Fatalf("advance processed %d: %v", processed, err)
	}
	if _, err := f.repo.AdvanceContextStart(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: f.cursor().RetentionRevision, ThroughSeq: contextStart}); err != nil {
		f.t.Fatalf("advance context %d: %v", contextStart, err)
	}
}

// cleanedReceipt is what a successful ai-service cleanup returns for plan.
func cleanedReceipt(plan *model.ChatRetentionPrunePlan) *model.CheckpointCleanupReceipt {
	ids := make([]string, len(plan.RunIDs))
	for i, id := range plan.RunIDs {
		ids[i] = id.String()
	}
	checkpoint := "checkpoint-" + plan.ConversationID.String()
	return &model.CheckpointCleanupReceipt{
		Status: "cleaned", ThreadID: plan.ConversationID.String(), PrunedRunIDs: ids,
		CheckpointID: &checkpoint, RemovedMessages: 2 * len(ids), DeletedCheckpoints: 3,
	}
}

// prune runs one pass with a cleanup that always succeeds.
func (f *retentionFixture) prune(ctx context.Context, owner uuid.UUID) (*model.ChatRetentionPruneReceipt, error) {
	plan, replay, err := f.repo.PlanPrune(ctx, owner)
	if err != nil || replay != nil {
		return replay, err
	}
	return f.repo.CommitPrune(ctx, owner, plan, cleanedReceipt(plan), uuid.Nil)
}

func TestChatRetentionCursorsAreOrderedAndMonotonicPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	for i := 0; i < 3; i++ {
		f.addTurn("completed", "knee")
	}
	ctx := context.Background()
	c := f.cursor()
	if c.ProcessedThroughSeq != 0 || c.ContextStartSeq != 1 || c.PrunedThroughSeq != 0 || c.RetentionRevision != 0 {
		t.Fatalf("defaults=%#v", c)
	}
	cmd := func(rev int64, seq int) model.AdvanceRetentionCursorCommand {
		return model.AdvanceRetentionCursorCommand{ExpectedRevision: rev, ThroughSeq: seq}
	}
	if _, err := f.repo.AdvanceContextStart(ctx, f.owner, cmd(0, 3)); !errors.Is(err, model.ErrRetentionCursorInvalid) {
		t.Fatalf("context skipped unprocessed messages: %v", err)
	}
	if _, err := f.repo.AdvanceProcessed(ctx, f.owner, cmd(0, 3)); !errors.Is(err, model.ErrRetentionCursorInvalid) {
		t.Fatalf("processed split an interaction: %v", err)
	}
	if _, err := f.repo.AdvanceProcessed(ctx, f.owner, cmd(0, 7)); !errors.Is(err, model.ErrRetentionCursorInvalid) {
		t.Fatalf("processed past the transcript: %v", err)
	}
	if c, err := f.repo.AdvanceProcessed(ctx, f.owner, cmd(0, 4)); err != nil || c.ProcessedThroughSeq != 4 || c.RetentionRevision != 1 {
		t.Fatalf("advance=%#v %v", c, err)
	}
	// An exact retry (stale revision) is an idempotent no-op.
	if c, err := f.repo.AdvanceProcessed(ctx, f.owner, cmd(0, 4)); err != nil || c.RetentionRevision != 1 {
		t.Fatalf("retry=%#v %v", c, err)
	}
	if _, err := f.repo.AdvanceProcessed(ctx, f.owner, cmd(1, 2)); !errors.Is(err, model.ErrRetentionCursorRegression) {
		t.Fatalf("processed moved backwards: %v", err)
	}
	if _, err := f.repo.AdvanceProcessed(ctx, f.owner, cmd(0, 6)); !errors.Is(err, model.ErrRetentionRevisionConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if _, err := f.repo.AdvanceContextStart(ctx, f.owner, cmd(1, 4)); !errors.Is(err, model.ErrRetentionCursorInvalid) {
		t.Fatalf("context split an interaction: %v", err)
	}
	if c, err := f.repo.AdvanceContextStart(ctx, f.owner, cmd(1, 5)); err != nil || c.ContextStartSeq != 5 || c.RetentionRevision != 2 {
		t.Fatalf("context=%#v %v", c, err)
	}
	if _, err := f.repo.AdvanceContextStart(ctx, f.owner, cmd(2, 3)); !errors.Is(err, model.ErrRetentionCursorRegression) {
		t.Fatalf("context moved backwards: %v", err)
	}
	// The database refuses an out-of-order cursor even from a raw writer.
	for _, raw := range []string{
		"UPDATE conversations SET pruned_through_seq = 5 WHERE id = ?",
		"UPDATE conversations SET context_start_seq = 6 WHERE id = ?",
		"UPDATE conversations SET pruned_through_seq = -1 WHERE id = ?",
	} {
		if err := db.Exec(raw, f.conversation).Error; err == nil || !strings.Contains(err.Error(), "conversations_retention_cursor_order_check") {
			t.Fatalf("%s accepted: %v", raw, err)
		}
	}
}

func TestChatRetentionConcurrentAdvanceHasOneWinnerPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	for i := 0; i < 3; i++ {
		f.addTurn("completed", "knee")
	}
	targets := []int{2, 4, 6, 2, 4, 6}
	errs := make([]error, len(targets))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i, target int) {
			defer wg.Done()
			<-start
			_, errs[i] = f.repo.AdvanceProcessed(context.Background(), f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: 0, ThroughSeq: target})
		}(i, target)
	}
	close(start)
	wg.Wait()
	c := f.cursor()
	if c.RetentionRevision != 1 {
		t.Fatalf("revision=%d, want exactly one applied move", c.RetentionRevision)
	}
	for i, err := range errs {
		switch {
		case err == nil:
			if targets[i] != c.ProcessedThroughSeq {
				t.Fatalf("loser target %d reported success over %d", targets[i], c.ProcessedThroughSeq)
			}
		case errors.Is(err, model.ErrRetentionRevisionConflict), errors.Is(err, model.ErrRetentionCursorRegression):
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestChatRetentionContextReadStartsAtContextCursorPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	for i := 0; i < 3; i++ {
		f.addTurn("completed", "knee")
	}
	read := NewMessageContextRepository(db)
	all, err := read.SearchRelevant(context.Background(), f.owner, f.conversation, []string{"knee"}, 0, 20)
	if err != nil || len(all) != 6 {
		t.Fatalf("before=%d %v", len(all), err)
	}
	f.advance(4, 5)
	window, err := read.SearchRelevant(context.Background(), f.owner, f.conversation, []string{"knee"}, 0, 20)
	if err != nil || len(window) != 2 {
		t.Fatalf("after=%d %v", len(window), err)
	}
	for _, m := range window {
		if m.Seq < 5 {
			t.Fatalf("context read returned seq %d before context_start", m.Seq)
		}
	}
}

func TestChatRetentionPruneRetainsReferencedEvidencePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	plain := f.addTurn("completed", "plain")
	fact := f.addTurn("completed", "BodyFact source")
	dx := f.addTurn("completed", "Diagnosis request")
	ask := f.addTurn("completed", "Ask User answer")
	tail := f.addTurn("completed", "tail")

	f.addEvidence(f.owner, fact)
	dxEvidence := f.addEvidence(f.owner, dx)
	f.exec("INSERT INTO diagnoses(user_id) VALUES (?)", f.owner)
	dxRun := uuid.New()
	f.exec(`INSERT INTO condition_analysis_runs(user_id,id,request_message_id,request_evidence_id,request_source_version,trigger,scope,
based_on_body_state_revision,based_on_diagnosis_revision,input_evidence_refs,agent_hypotheses,replay_input,execution_provenance,
idempotency_key,request_fingerprint,started_at) VALUES (?,?,?,?,1,'new_analysis','{}',0,0,'[]','[]','{}','{}',?,?,now())`,
		f.owner, dxRun, dx.user, dxEvidence, "dx-"+dxRun.String(), strings.Repeat("a", 64))
	interaction := uuid.New()
	f.exec(`INSERT INTO agent_interactions(id,run_id,conversation_id,tool_call_id,question,answer,status,answered_at)
VALUES (?,?,?,'ask-1','{"q":"where"}','{"a":"knee"}','answered',now())`, interaction, *ask.run, f.conversation)
	f.exec(`INSERT INTO interaction_resumes(interaction_id,user_id,conversation_id,run_id,request_id,answer_fingerprint,question_fingerprint,
user_message_id,assistant_message_id,claimed_at) VALUES (?,?,?,?,'resume-1',?,?,?,?,now())`,
		interaction, f.owner, f.conversation, *ask.run, strings.Repeat("b", 64), strings.Repeat("c", 64), ask.user, ask.reply)

	f.advance(10, 11)
	receipt, err := f.prune(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	// min(C-1, last-1) = 9 splits the tail interaction, so the pass stops at 8.
	if receipt.Replayed || receipt.FromPrunedSeq != 0 || receipt.PrunedThroughSeq != 8 || receipt.DeletedMessages != 4 ||
		receipt.RetainedMessages != 4 || receipt.PrunedRuns != 1 || receipt.DeletedRuntimeEvents != 2 ||
		receipt.DeletedToolCalls != 1 || receipt.CheckpointCleanup != "verified" || receipt.JobID == uuid.Nil ||
		receipt.CheckpointsDeleted != 3 || receipt.CheckpointID == "" {
		t.Fatalf("receipt=%#v", receipt)
	}
	for _, kept := range []uuid.UUID{fact.user, dx.user, ask.user, ask.reply, tail.user, tail.reply} {
		if !f.messageExists(kept) {
			t.Fatalf("referenced or live message %s pruned", kept)
		}
	}
	for _, gone := range []uuid.UUID{plain.user, plain.reply, fact.reply} {
		if f.messageExists(gone) {
			t.Fatalf("message %s survived", gone)
		}
	}
	if f.runEvents(plain.run) != 0 || f.runEvents(fact.run) != 2 || f.runEvents(ask.run) != 2 || f.runEvents(tail.run) != 2 {
		t.Fatal("run event retention mismatch")
	}
	if f.count("SELECT COUNT(*) FROM agent_tool_calls WHERE run_id = ?", *plain.run) != 0 {
		t.Fatal("pruned run kept its tool-call copy")
	}
	if f.count("SELECT COUNT(*) FROM interaction_resumes WHERE interaction_id = ?", interaction) != 1 ||
		f.count("SELECT COUNT(*) FROM statement_evidence WHERE user_id = ?", f.owner) != 2 ||
		f.count("SELECT COUNT(*) FROM condition_analysis_runs WHERE user_id = ?", f.owner) != 1 {
		t.Fatal("durable evidence lost")
	}
	if dx.replySeq != 6 || f.messageExists(dx.reply) {
		t.Fatal("unreferenced Diagnosis reply survived")
	}
	c := f.cursor()
	if c.PrunedThroughSeq != 8 || c.ProcessedThroughSeq != 10 || c.ContextStartSeq != 11 {
		t.Fatalf("cursor=%#v", c)
	}
	// Seq allocation stays above the pruned cursor: the newest message is kept.
	if f.count("SELECT COALESCE(MAX(seq),0) FROM messages WHERE conversation_id = ?", f.conversation) <= int64(c.PrunedThroughSeq) {
		t.Fatal("next seq could reuse a pruned position")
	}
	again, err := f.prune(ctx, f.owner)
	if err != nil || !again.Replayed || again.JobID != receipt.JobID || again.PrunedThroughSeq != 8 {
		t.Fatalf("replay=%#v %v", again, err)
	}
	if f.count("SELECT COUNT(*) FROM jobs WHERE user_id = ? AND job_type = ? AND status = 'completed'", f.owner, model.ChatRetentionPruneJobType) != 1 {
		t.Fatal("prune job not recorded exactly once")
	}
	if _, err := f.repo.AdvanceContextStart(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: c.RetentionRevision, ThroughSeq: 9}); !errors.Is(err, model.ErrRetentionCursorRegression) {
		t.Fatalf("context cursor moved backwards after prune: %v", err)
	}
}

func TestChatRetentionUnsettledRunBlocksThePassPostgres(t *testing.T) {
	for _, status := range []string{"running", "waiting_user"} {
		t.Run(status, func(t *testing.T) {
			db := openSingletonTestDB(t)
			f := newRetentionFixture(t, db)
			done := f.addTurn("completed", "done")
			active := f.addTurn(status, "active")
			if status == "waiting_user" {
				f.exec(`INSERT INTO agent_interactions(run_id,conversation_id,tool_call_id,question,status) VALUES (?,?,'ask-1','{"q":"where"}','pending')`,
					*active.run, f.conversation)
			}
			note := f.addTurn("", "run-less note")
			ctx := context.Background()
			if _, err := f.repo.AdvanceProcessed(ctx, f.owner, model.AdvanceRetentionCursorCommand{ThroughSeq: 4}); !errors.Is(err, model.ErrRetentionCursorInvalid) {
				t.Fatalf("processed passed an active Run: %v", err)
			}
			// Even with cursors forced past it by a raw writer, an unsettled
			// Run's LangGraph thread is mid-step or interrupted: the whole
			// pass waits, so its checkpoint (and a pending Ask User resume)
			// is never rewritten under it.
			f.exec("UPDATE conversations SET processed_through_seq = 6, context_start_seq = 7 WHERE id = ?", f.conversation)
			if _, err := f.prune(ctx, f.owner); !errors.Is(err, model.ErrRetentionBusy) {
				t.Fatalf("pass ran beside an unsettled Run: %v", err)
			}
			if !f.messageExists(done.user) || !f.messageExists(active.user) || !f.messageExists(note.user) ||
				f.runEvents(done.run) != 2 || f.cursor().PrunedThroughSeq != 0 {
				t.Fatal("busy pass deleted data or moved R")
			}
			// Once the Run settles the pass proceeds.
			f.exec("UPDATE runs SET status = 'completed' WHERE id = ?", *active.run)
			f.exec("UPDATE agent_interactions SET status = 'answered' WHERE run_id = ?", *active.run)
			receipt, err := f.prune(ctx, f.owner)
			if err != nil || receipt.PrunedThroughSeq != 4 || receipt.DeletedMessages != 4 {
				t.Fatalf("receipt=%#v %v", receipt, err)
			}
			if f.messageExists(done.user) || f.messageExists(active.reply) || !f.messageExists(note.user) || !f.messageExists(note.reply) {
				t.Fatal("settled material kept or tail pruned")
			}
		})
	}
}

func TestChatRetentionCommitRequiresAVerifiedCleanupPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	first := f.addTurn("completed", "first")
	second := f.addTurn("completed", "second")
	f.addTurn("completed", "tail")
	f.advance(6, 5)
	plan, replay, err := f.repo.PlanPrune(ctx, f.owner)
	if err != nil || replay != nil || plan.TargetSeq != 4 || len(plan.RunIDs) != 2 {
		t.Fatalf("plan=%#v replay=%#v %v", plan, replay, err)
	}
	if !slices.Contains(plan.RunIDs, *first.run) || !slices.Contains(plan.RunIDs, *second.run) {
		t.Fatal("plan does not name the Runs behind the boundary")
	}
	wrongThread := cleanedReceipt(plan)
	wrongThread.ThreadID = uuid.NewString()
	missingRun := cleanedReceipt(plan)
	missingRun.PrunedRunIDs = missingRun.PrunedRunIDs[:1]
	survivor := cleanedReceipt(plan)
	survivor.RemainingRunIDs = []string{first.run.String()}
	notCleaned := cleanedReceipt(plan)
	notCleaned.Status = "thread_busy"
	for name, receipt := range map[string]*model.CheckpointCleanupReceipt{
		"none": nil, "wrong thread": wrongThread, "missing run": missingRun, "survivor": survivor, "not cleaned": notCleaned,
	} {
		if _, err := f.repo.CommitPrune(ctx, f.owner, plan, receipt, uuid.Nil); !errors.Is(err, model.ErrRetentionCleanupUnverified) {
			t.Fatalf("%s: unverified cleanup accepted: %v", name, err)
		}
	}
	if f.cursor().PrunedThroughSeq != 0 || !f.messageExists(first.user) ||
		f.count("SELECT COUNT(*) FROM jobs WHERE user_id = ?", f.owner) != 0 {
		t.Fatal("an unverified cleanup moved R or deleted data")
	}
	receipt, err := f.repo.CommitPrune(ctx, f.owner, plan, cleanedReceipt(plan), uuid.Nil)
	if err != nil || receipt.PrunedThroughSeq != 4 || receipt.CheckpointCleanup != "verified" {
		t.Fatalf("receipt=%#v %v", receipt, err)
	}
}

func TestChatRetentionNewMessageBetweenPlanAndCommitIsNeverLostPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	f.addTurn("completed", "old")
	kept := f.addTurn("completed", "kept")
	f.advance(4, 3)
	plan, _, err := f.repo.PlanPrune(ctx, f.owner)
	if err != nil || plan == nil || plan.TargetSeq != 2 {
		t.Fatalf("plan=%#v %v", plan, err)
	}
	// A turn lands while the ai-service is cleaning: its Run is still active.
	arriving := f.addTurn("running", "arriving")
	if _, err := f.repo.CommitPrune(ctx, f.owner, plan, cleanedReceipt(plan), uuid.Nil); !errors.Is(err, model.ErrRetentionBusy) {
		t.Fatalf("commit raced an active turn: %v", err)
	}
	// It settles before the retry: the stale plan is still refused.
	f.exec("UPDATE runs SET status = 'completed' WHERE id = ?", *arriving.run)
	if _, err := f.repo.CommitPrune(ctx, f.owner, plan, cleanedReceipt(plan), uuid.Nil); !errors.Is(err, model.ErrRetentionPlanStale) {
		t.Fatalf("stale plan committed: %v", err)
	}
	if f.cursor().PrunedThroughSeq != 0 || f.count("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", f.conversation) != 6 {
		t.Fatal("stale commit deleted data")
	}
	receipt, err := f.prune(ctx, f.owner)
	if err != nil || receipt.PrunedThroughSeq != 2 {
		t.Fatalf("receipt=%#v %v", receipt, err)
	}
	for _, id := range []uuid.UUID{kept.user, kept.reply, arriving.user, arriving.reply} {
		if !f.messageExists(id) {
			t.Fatalf("message %s lost", id)
		}
	}
}

func TestChatRetentionConcurrentPruneRunsOncePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	for i := 0; i < 3; i++ {
		f.addTurn("completed", "knee")
	}
	f.advance(6, 7)
	const workers = 8
	receipts := make([]*model.ChatRetentionPruneReceipt, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			receipts[i], errs[i] = f.prune(context.Background(), f.owner)
		}(i)
	}
	close(start)
	wg.Wait()
	fresh := 0
	for i := range receipts {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if !receipts[i].Replayed {
			fresh++
		}
		if receipts[i].JobID != receipts[0].JobID || receipts[i].PrunedThroughSeq != 4 || receipts[i].DeletedMessages != 4 {
			t.Fatalf("divergent receipt %#v", receipts[i])
		}
	}
	if fresh != 1 {
		t.Fatalf("prune executed %d times", fresh)
	}
	if f.count("SELECT COUNT(*) FROM jobs WHERE user_id = ? AND job_type = ?", f.owner, model.ChatRetentionPruneJobType) != 1 ||
		f.count("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", f.conversation) != 2 ||
		f.cursor().RetentionRevision != 3 {
		t.Fatal("concurrent prune was not applied exactly once")
	}
}

func TestChatRetentionOwnerIsolationPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	a := newRetentionFixture(t, db)
	b := newRetentionFixture(t, db)
	ctx := context.Background()
	turnA := a.addTurn("completed", "a")
	a.addTurn("completed", "a")
	b.addTurn("completed", "b")
	b.addTurn("completed", "b")
	a.advance(4, 5)
	b.advance(4, 5)

	// No Conversation, no cursor: masked as not found, never created.
	stranger := createSingletonTestUser(t, db)
	if _, err := a.repo.Get(ctx, stranger); !errors.Is(err, model.ErrRetentionNotFound) {
		t.Fatal(err)
	}
	if _, err := a.prune(ctx, stranger); !errors.Is(err, model.ErrRetentionNotFound) {
		t.Fatal(err)
	}
	if _, err := a.repo.AdvanceProcessed(ctx, stranger, model.AdvanceRetentionCursorCommand{ThroughSeq: 2}); !errors.Is(err, model.ErrRetentionNotFound) {
		t.Fatal(err)
	}
	if _, err := a.prune(ctx, uuid.Nil); !errors.Is(err, model.ErrRetentionNotFound) {
		t.Fatal(err)
	}

	// Another owner's evidence pointing at A's message is an anomaly: A's pass
	// fails closed and deletes nothing.
	a.addEvidence(b.owner, turnA)
	if _, err := a.prune(ctx, a.owner); !errors.Is(err, model.ErrRetentionOwnerAnomaly) {
		t.Fatalf("cross-owner reference not rejected: %v", err)
	}
	if a.count("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", a.conversation) != 4 || a.cursor().PrunedThroughSeq != 0 {
		t.Fatal("anomalous pass deleted data")
	}

	// B's pass touches only B's Conversation.
	receipt, err := b.prune(ctx, b.owner)
	if err != nil || receipt.ConversationID != b.conversation || receipt.DeletedMessages != 2 {
		t.Fatalf("receipt=%#v %v", receipt, err)
	}
	if a.count("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", a.conversation) != 4 {
		t.Fatal("B's prune touched A")
	}

	// A Run of another owner inside A's Conversation is an anomaly too.
	foreign := uuid.New()
	turn := uuid.New()
	a.exec(`INSERT INTO runs(id,conversation_id,turn_id,request_id,user_id,status,model) VALUES (?,?,?,'foreign',?,'completed','synthetic')`,
		foreign, a.conversation, turn, b.owner)
	a.exec(`INSERT INTO messages(conversation_id,turn_id,run_id,role,status,seq,content_text) VALUES (?,?,?,'user','completed',5,'x')`,
		a.conversation, turn, foreign)
	if _, err := a.repo.AdvanceProcessed(ctx, a.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: a.cursor().RetentionRevision, ThroughSeq: 5}); !errors.Is(err, model.ErrRetentionOwnerAnomaly) {
		t.Fatalf("foreign run not rejected: %v", err)
	}
	a.exec("DELETE FROM messages WHERE run_id = ?", foreign)
	a.exec("DELETE FROM runs WHERE id = ?", foreign)
}
