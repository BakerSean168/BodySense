package repository

import (
	"context"
	"errors"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"strings"
	"sync"
	"testing"
)

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

// bigText makes each message of a turn ~500 estimated tokens, so a turn is
// 1012 tokens and two turns overflow the minimum 1024-token budget.
var bigText = strings.Repeat("x", 1500)

func TestChatRetentionWindowReconcileByTokenBudgetPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	budget := model.MinChatContextTokenBudget
	f.addTurn("completed", bigText)
	f.addTurn("completed", bigText)

	cursor, moved, err := f.repo.ReconcileWindow(ctx, f.owner, budget)
	if err != nil || !moved {
		t.Fatalf("reconcile: moved=%v %v", moved, err)
	}
	if cursor.ProcessedThroughSeq != 4 || cursor.ContextStartSeq != 3 || cursor.PrunedThroughSeq != 0 {
		t.Fatalf("cursor %+v, want P=4 C=3 R=0", cursor)
	}
	// Reconciling an unchanged transcript writes nothing.
	again, moved, err := f.repo.ReconcileWindow(ctx, f.owner, budget)
	if err != nil || moved || again.RetentionRevision != cursor.RetentionRevision {
		t.Fatalf("second reconcile moved=%v rev %d -> %d: %v", moved, cursor.RetentionRevision, again.RetentionRevision, err)
	}

	// An unanswered Ask User adds an unsettled Run: P stops in front of it
	// and C may only reach it, never pass it.
	waiting := f.addTurn("waiting_user", bigText)
	f.exec(`INSERT INTO agent_interactions(run_id,conversation_id,tool_call_id,question,status) VALUES (?,?,'ask-1','{"q":"where"}','pending')`,
		*waiting.run, f.conversation)
	cursor, _, err = f.repo.ReconcileWindow(ctx, f.owner, budget)
	if err != nil {
		t.Fatal(err)
	}
	if cursor.ProcessedThroughSeq != 4 || cursor.ContextStartSeq != 5 {
		t.Fatalf("cursor %+v, want P=4 C=5", cursor)
	}
	window, err := f.repo.ContextWindow(ctx, f.owner, f.conversation)
	if err != nil {
		t.Fatal(err)
	}
	if window.ContextStartSeq != 5 || len(window.RunIDs) != 1 || window.RunIDs[0] != *waiting.run {
		t.Fatalf("window %+v, want only the waiting run", window)
	}
	// The pass refuses while the Ask User waits; nothing of it is deleted.
	if _, err := f.prune(ctx, f.owner); !errors.Is(err, model.ErrRetentionBusy) {
		t.Fatalf("prune beside an unanswered Ask User: %v", err)
	}
	if !f.messageExists(waiting.user) || !f.messageExists(waiting.reply) {
		t.Fatal("unanswered Ask User messages were deleted")
	}

	// A foreign owner cannot read the window.
	other := newRetentionFixture(t, db)
	if _, err := f.repo.ContextWindow(ctx, other.owner, f.conversation); !errors.Is(err, model.ErrRetentionNotFound) {
		t.Fatalf("foreign window read: %v", err)
	}
	if _, _, err := f.repo.ReconcileWindow(ctx, f.owner, budget-1); !errors.Is(err, model.ErrRetentionCursorInvalid) {
		t.Fatalf("budget below minimum: %v", err)
	}
}

func TestChatRetentionContextCursorCannotSplitARunPostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	first := f.addTurn("completed", "first")
	// A second turn of the same Run (as an Ask User answer appended to it).
	second := f.addTurn("", "second")
	f.exec("UPDATE messages SET run_id = ? WHERE id IN (?, ?)", *first.run, second.user, second.reply)
	f.addTurn("completed", "third")
	c := f.cursor()
	if _, err := f.repo.AdvanceProcessed(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: c.RetentionRevision, ThroughSeq: 6}); err != nil {
		t.Fatal(err)
	}
	c = f.cursor()
	_, err := f.repo.AdvanceContextStart(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: c.RetentionRevision, ThroughSeq: 3})
	if !errors.Is(err, model.ErrRetentionCursorInvalid) {
		t.Fatalf("context cursor split a run: %v", err)
	}
	if _, err := f.repo.AdvanceContextStart(ctx, f.owner, model.AdvanceRetentionCursorCommand{ExpectedRevision: c.RetentionRevision, ThroughSeq: 5}); err != nil {
		t.Fatal(err)
	}
}
