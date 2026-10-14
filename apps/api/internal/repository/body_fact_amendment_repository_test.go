package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func amendmentSeed(t *testing.T, db *gorm.DB, owner uuid.UUID) (model.BodyFact, model.RecordBodyFactsCommand) {
	t.Helper()
	command := bodyFactTestCommand("original", 0)
	receipt, err := NewBodyStateRepository(db).RecordDirectFacts(context.Background(), owner, command)
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	if err := json.Unmarshal(receipt.FactIDs, &ids); err != nil {
		t.Fatal(err)
	}
	var original model.BodyFact
	if err := db.First(&original, "id = ?", ids[0]).Error; err != nil {
		t.Fatal(err)
	}
	return original, command
}

func TestBodyFactCorrectionAndWithdrawalPreserveHistoryPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	original, input := amendmentSeed(t, db, owner)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	replacement := input.Facts[0]
	replacement.Statement = "I reported my friend's shoulder problem, not my own history"
	correction := model.AmendBodyFactCommand{IdempotencyKey: "correct-original", ExpectedRevision: 1, Replacement: &replacement}
	saved, err := repo.CorrectDirectFact(ctx, owner, original.ID, correction)
	if err != nil || saved == nil || saved.Revision != 2 {
		t.Fatalf("receipt=%#v err=%v", saved, err)
	}
	page, err := repo.ListFacts(ctx, owner, 0)
	if err != nil || len(page.Facts) != 2 {
		t.Fatalf("page=%#v %v", page, err)
	}
	var corrected model.BodyFact
	if err := db.Where("supersedes_fact_id = ?", original.ID).First(&corrected).Error; err != nil {
		t.Fatal(err)
	}
	if corrected.Statement != replacement.Statement || corrected.SupersedesFactID == nil || *corrected.SupersedesFactID != original.ID {
		t.Fatal("correction lineage missing")
	}
	var history model.BodyFact
	if err := db.First(&history, "id = ?", original.ID).Error; err != nil {
		t.Fatal(err)
	}
	if history.Statement != original.Statement || history.AssertionStatus != "superseded" || history.SourceEvidenceID != original.SourceEvidenceID {
		t.Fatal("original assertion/source silently rewritten")
	}
	retry, err := repo.CorrectDirectFact(ctx, owner, original.ID, correction)
	if err != nil || retry.ID != saved.ID {
		t.Fatalf("correction replay failed: %v", err)
	}
	withdrawal := model.AmendBodyFactCommand{IdempotencyKey: "withdraw-corrected", ExpectedRevision: 2}
	withdrawn, err := repo.WithdrawDirectFact(ctx, owner, corrected.ID, withdrawal)
	if err != nil || withdrawn.Revision != 3 {
		t.Fatalf("withdraw=%#v %v", withdrawn, err)
	}
	replay, err := repo.WithdrawDirectFact(ctx, owner, corrected.ID, withdrawal)
	if err != nil || replay.ID != withdrawn.ID {
		t.Fatal("withdrawal replay generated a different commit")
	}
	page, err = repo.ListFacts(ctx, owner, 0)
	if err != nil || len(page.Facts) != 1 || page.Facts[0].Kind != "lifestyle" {
		t.Fatalf("unrelated fact changed: %#v %v", page, err)
	}
	assertBodyFactCount(t, db, "body_facts", owner, 3)
	assertBodyFactCount(t, db, "statement_evidence", owner, 3)
	assertBodyFactCount(t, db, "body_state_revisions", owner, 3)
	var events []model.BodyStateOutbox
	if err := db.Where("user_id = ?", owner).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	expectedEvents := map[int64]string{1: "body_state.facts_recorded", 2: "body_state.facts_corrected", 3: "body_state.facts_withdrawn"}
	for _, event := range events {
		if event.EventType != expectedEvents[event.Revision] {
			t.Fatalf("event type=%s revision=%d", event.EventType, event.Revision)
		}
		if strings.Contains(string(event.Payload), original.Statement) || strings.Contains(string(event.Payload), replacement.Statement) {
			t.Fatal("health text copied into audit")
		}
	}
	// Withdrawal keeps historical data, but deleting the entire synthetic owner
	// still cascades over the self-referential correction chain.
	if err := db.Exec("DELETE FROM users WHERE id = ?", owner).Error; err != nil {
		t.Fatal(err)
	}
	assertBodyFactCount(t, db, "body_facts", owner, 0)
}

func TestBodyFactAmendmentRejectsForeignStaleNoopAndReboundCommandsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	original, input := amendmentSeed(t, db, owner)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	if receipt, err := repo.WithdrawDirectFact(ctx, other, original.ID, model.AmendBodyFactCommand{IdempotencyKey: "foreign"}); !errors.Is(err, model.ErrBodyFactNotFound) || receipt != nil {
		t.Fatalf("foreign target=%v %v", receipt, err)
	}
	noop := model.AmendBodyFactCommand{IdempotencyKey: "noop", ExpectedRevision: 1, Replacement: &input.Facts[0]}
	if receipt, err := repo.CorrectDirectFact(ctx, owner, original.ID, noop); !errors.Is(err, model.ErrInvalidBodyFact) || receipt != nil {
		t.Fatal("no-op created a revision")
	}
	if _, err := repo.WithdrawDirectFact(ctx, owner, original.ID, model.AmendBodyFactCommand{IdempotencyKey: "stale", ExpectedRevision: 0}); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatal("stale request accepted")
	}
	command := model.AmendBodyFactCommand{IdempotencyKey: "same-key", ExpectedRevision: 1}
	if _, err := repo.WithdrawDirectFact(ctx, owner, original.ID, command); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.WithdrawDirectFact(ctx, owner, uuid.New(), command); !errors.Is(err, model.ErrBodyFactIdempotencyConflict) {
		t.Fatal("same key rebound to another target")
	}
	if _, err := repo.WithdrawDirectFact(ctx, owner, original.ID, model.AmendBodyFactCommand{IdempotencyKey: "second-withdraw", ExpectedRevision: 2}); !errors.Is(err, model.ErrBodyFactNotCurrent) {
		t.Fatal("non-current assertion accepted")
	}
	assertBodyFactCount(t, db, "body_state_revisions", owner, 2)
	assertBodyFactCount(t, db, "body_state_revisions", other, 0)
}

func TestBodyFactAmendmentFinalWriteFailureRollsBackPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	original, input := amendmentSeed(t, db, owner)
	failure := errors.New("synthetic receipt failure")
	const name = "body_fact_amendment_fail_receipt"
	if err := db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "body_fact_commits" {
			_ = tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(name) })
	replacement := input.Facts[0]
	replacement.Statement = "I correct my previous statement"
	receipt, err := NewBodyStateRepository(db).CorrectDirectFact(context.Background(), owner, original.ID, model.AmendBodyFactCommand{IdempotencyKey: "failure", ExpectedRevision: 1, Replacement: &replacement})
	if receipt != nil || !errors.Is(err, failure) {
		t.Fatalf("failed write reported success %v %v", receipt, err)
	}
	var unchanged model.BodyFact
	if err := db.First(&unchanged, "id = ?", original.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.AssertionStatus != "asserted" {
		t.Fatal("old assertion not rolled back")
	}
	assertBodyFactCount(t, db, "body_facts", owner, 2)
	assertBodyFactCount(t, db, "statement_evidence", owner, 2)
	assertBodyFactCount(t, db, "body_state_revisions", owner, 1)
}

type factReadSnapshotKey struct{}

func TestBodyFactReadSnapshotSurvivesConcurrentCorrectionPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	original, input := amendmentSeed(t, db, owner)
	captured, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	const callback = "body_fact_read_snapshot_barrier"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "body_states" && tx.Statement.Context.Value(factReadSnapshotKey{}) == true {
			once.Do(func() { close(captured); <-release })
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type result struct {
		page *model.BodyFactPage
		err  error
	}
	done := make(chan result, 1)
	go func() {
		p, e := NewBodyStateRepository(db).ListFacts(context.WithValue(ctx, factReadSnapshotKey{}, true), owner, 0)
		done <- result{p, e}
	}()
	select {
	case <-captured:
	case <-ctx.Done():
		close(release)
		t.Fatal("snapshot barrier timed out")
	}
	replacement := input.Facts[0]
	replacement.Statement = "I correct the original statement"
	_, err := NewBodyStateRepository(db).CorrectDirectFact(context.Background(), owner, original.ID, model.AmendBodyFactCommand{IdempotencyKey: "during-read", ExpectedRevision: 1, Replacement: &replacement})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	read := <-done
	if read.err != nil || read.page == nil || read.page.CurrentRevision != 1 || len(read.page.Facts) != 2 {
		t.Fatalf("inconsistent snapshot=%#v %v", read.page, read.err)
	}
	for _, f := range read.page.Facts {
		if f.Statement == replacement.Statement {
			t.Fatal("new correction leaked into prior-revision page")
		}
	}
	latest, err := NewBodyStateRepository(db).ListFacts(context.Background(), owner, 0)
	if err != nil || latest.CurrentRevision != 2 {
		t.Fatal("fresh read did not observe correction")
	}
}

func TestBodyFactCorrectionMigrationRefusesHistoryLossPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	original, input := amendmentSeed(t, db, owner)
	replacement := input.Facts[0]
	replacement.Statement = "Corrected original"
	if _, err := NewBodyStateRepository(db).CorrectDirectFact(context.Background(), owner, original.ID, model.AmendBodyFactCommand{IdempotencyKey: "correct", ExpectedRevision: 1, Replacement: &replacement}); err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000004_body_fact_corrections.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec(string(down)).Error; err == nil || !strings.Contains(err.Error(), "Cannot roll back BodyFact corrections") {
		t.Fatalf("rollback discarded live provenance: %v", err)
	}
}
