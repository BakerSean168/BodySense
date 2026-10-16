package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func bodyFactTestCommand(key string, revision int64) model.RecordBodyFactsCommand {
	return model.RecordBodyFactsCommand{IdempotencyKey: key, ExpectedRevision: revision, Facts: []model.BodyFactInput{
		{Kind: "symptom", Statement: "synthetic private shoulder statement", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present","pattern":"unknown"}`)},
		{Kind: "lifestyle", Statement: "synthetic private activity habit", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"habit_type":"activity"}`)},
	}}
}

func assertBodyFactCount(t *testing.T, db *gorm.DB, table string, userID uuid.UUID, want int64) {
	t.Helper()
	var count int64
	if err := db.Table(table).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s count=%d want=%d", table, count, want)
	}
}

func TestBodyFactFingerprintCanonicalizesObjectOrder(t *testing.T) {
	a := bodyFactTestCommand("same", 0)
	b := bodyFactTestCommand("same", 0)
	b.Facts[0].Details = json.RawMessage(`{ "pattern": "unknown", "presence": "present" }`)
	x, err := bodyFactCommandFingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := bodyFactCommandFingerprint(b)
	if err != nil || x != y {
		t.Fatalf("equivalent JSON changed fingerprint: %s %s %v", x, y, err)
	}
}

func TestBodyFactBatchReceiptAndMinimalAuditPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	command := bodyFactTestCommand("batch", 0)
	receipt, err := repo.RecordDirectFacts(ctx, userID, command)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.Revision != 1 || receipt.EventID == uuid.Nil || receipt.RevisionID == uuid.Nil {
		t.Fatalf("invalid commit receipt: %#v", receipt)
	}
	for table, count := range map[string]int64{"body_facts": 2, "statement_evidence": 2, "body_fact_commits": 1, "body_state_revisions": 1, "body_state_outbox": 1} {
		assertBodyFactCount(t, db, table, userID, count)
	}
	page, err := repo.ListFacts(ctx, userID, 0)
	if err != nil || page.CurrentRevision != 1 || len(page.Facts) != 2 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	for _, fact := range page.Facts {
		if fact.Subject != "self" || fact.AssertionStatus != "asserted" || string(fact.Temporal) != `{"precision": "unknown"}` && string(fact.Temporal) != `{"precision":"unknown"}` {
			t.Fatalf("wrong assertion semantics: %#v", fact)
		}
		var source model.StatementEvidence
		if err := db.First(&source, "id = ?", fact.SourceEvidenceID).Error; err != nil {
			t.Fatal(err)
		}
		if source.UserID != userID || source.SourceKind != "direct_edit" || source.Excerpt != fact.Statement {
			t.Fatal("source ownership/text mismatch")
		}
	}
	var revision model.BodyStateRevision
	var event model.BodyStateOutbox
	if err := db.First(&revision, "id = ?", receipt.RevisionID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&event, "id = ?", receipt.EventID).Error; err != nil {
		t.Fatal(err)
	}
	for _, fact := range command.Facts {
		if strings.Contains(string(revision.Changes), fact.Statement) || strings.Contains(string(event.Payload), fact.Statement) {
			t.Fatal("sensitive text was duplicated into general audit/outbox")
		}
	}
}

func TestBodyFactRetryBeforeRevisionCheckAndMismatchedKeyPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	command := bodyFactTestCommand("original", 0)
	first, err := repo.RecordDirectFacts(ctx, userID, command)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecordDirectFacts(ctx, userID, bodyFactTestCommand("later", 1)); err != nil {
		t.Fatal(err)
	}
	retry, err := repo.RecordDirectFacts(ctx, userID, command)
	if err != nil || retry.ID != first.ID || retry.Revision != 1 || retry.EventID != first.EventID {
		t.Fatalf("retry lost original receipt: %#v %v", retry, err)
	}
	command.Facts[0].Statement = "different content must not replace the original"
	if receipt, err := repo.RecordDirectFacts(ctx, userID, command); !errors.Is(err, model.ErrBodyFactIdempotencyConflict) || receipt != nil {
		t.Fatalf("mismatched key receipt=%v err=%v", receipt, err)
	}
	if receipt, err := repo.RecordDirectFacts(ctx, userID, bodyFactTestCommand("stale", 0)); !errors.Is(err, ErrBodyStateRevisionConflict) || receipt != nil {
		t.Fatalf("stale receipt=%v err=%v", receipt, err)
	}
	assertBodyFactCount(t, db, "body_fact_commits", userID, 2)
	assertBodyFactCount(t, db, "body_facts", userID, 4)
}

func TestBodyFactConcurrentDuplicateCommitsOncePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	command := bodyFactTestCommand("concurrent", 0)
	type result struct {
		receipt *model.BodyFactCommit
		err     error
	}
	results := make(chan result, 12)
	var group sync.WaitGroup
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			receipt, err := repo.RecordDirectFacts(context.Background(), userID, command)
			results <- result{receipt, err}
		}()
	}
	group.Wait()
	close(results)
	var id uuid.UUID
	for result := range results {
		if result.err != nil || result.receipt == nil {
			t.Fatalf("concurrent retry failed: %v", result.err)
		}
		if id == uuid.Nil {
			id = result.receipt.ID
		}
		if result.receipt.ID != id || result.receipt.Revision != 1 {
			t.Fatal("duplicate produced a second commit")
		}
	}
	assertBodyFactCount(t, db, "body_state_revisions", userID, 1)
	assertBodyFactCount(t, db, "body_state_outbox", userID, 1)
	assertBodyFactCount(t, db, "body_facts", userID, 2)
}

func TestBodyFactConcurrentDistinctCommandsConflictPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	results := make(chan error, 2)
	for i := range 2 {
		go func() {
			_, err := repo.RecordDirectFacts(context.Background(), userID, bodyFactTestCommand(fmt.Sprintf("command-%d", i), 0))
			results <- err
		}()
	}
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, ErrBodyStateRevisionConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	assertBodyFactCount(t, db, "body_facts", userID, 2)
}

func TestBodyFactFinalWriteFailureRollsBackAllRowsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	userID := seedBodyStateContextUser(t, db)
	failure := errors.New("injected final receipt write failure")
	const callback = "body_fact_test_fail_receipt"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "body_fact_commits" {
			_ = tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	receipt, err := NewBodyStateRepository(db).RecordDirectFacts(context.Background(), userID, bodyFactTestCommand("rollback", 0))
	if !errors.Is(err, failure) || receipt != nil {
		t.Fatalf("failure returned success-shaped receipt: %v %v", receipt, err)
	}
	for _, table := range []string{"body_states", "body_state_revisions", "statement_evidence", "body_facts", "body_state_outbox", "body_fact_commits"} {
		assertBodyFactCount(t, db, table, userID, 0)
	}
}

func TestBodyFactTenantSourceIsolationAndBlockedReadsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	command := bodyFactTestCommand("same-key-different-owners", 0)
	for _, userID := range []uuid.UUID{owner, other} {
		if _, err := repo.RecordDirectFacts(ctx, userID, command); err != nil {
			t.Fatal(err)
		}
	}
	ownerPage, err := repo.ListFacts(ctx, owner, 0)
	if err != nil {
		t.Fatal(err)
	}
	otherPage, err := repo.ListFacts(ctx, other, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range ownerPage.Facts {
		if fact.UserID != owner {
			t.Fatal("cross-user read")
		}
	}
	if err := db.Model(&model.BodyFact{}).Where("id = ?", ownerPage.Facts[0].ID).Update("source_evidence_id", otherPage.Facts[0].SourceEvidenceID).Error; err == nil {
		t.Fatal("database allowed cross-user source rebinding")
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id = ?", owner).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	blocked, err := repo.ListFacts(ctx, owner, 0)
	if err != nil || len(blocked.Facts) != 0 {
		t.Fatalf("blocked sources still readable: %#v %v", blocked, err)
	}
	if _, err := repo.RecordDirectFacts(ctx, owner, command); err != nil {
		t.Fatal(err)
	}
	assertBodyFactCount(t, db, "statement_evidence", owner, 2)
	stillOther, err := repo.ListFacts(ctx, other, 0)
	if err != nil || len(stillOther.Facts) != 2 {
		t.Fatal("blocking one owner damaged independent sources")
	}
}

func TestBodyFactDatabaseRejectsNullTemporalPrecisionPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	if _, err := NewBodyStateRepository(db).RecordDirectFacts(context.Background(), owner, bodyFactTestCommand("temporal-guard", 0)); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE body_facts SET temporal = ?::jsonb WHERE user_id = ?", `{"precision":null}`, owner).Error; err == nil {
		t.Fatal("database CHECK accepted unknown/null precision through SQL three-valued logic")
	}
}

func TestBodyFactPrivacyInventoryAndAccountCascadePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	for _, userID := range []uuid.UUID{owner, other} {
		if _, err := repo.RecordDirectFacts(ctx, userID, bodyFactTestCommand("erase-synthetic-owner", 0)); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := NewPrivacyErasureRepository(db).CountUserData(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	inventory := map[string]int64{}
	for _, count := range counts {
		inventory[count.Name] = count.Count
	}
	for name, expected := range map[string]int64{"body_facts": 2, "statement_evidence": 2, "body_fact_commits": 1, "body_state_outbox": 1} {
		if inventory[name] != expected {
			t.Fatalf("privacy inventory %s=%d want=%d", name, inventory[name], expected)
		}
	}
	if err := db.Exec("DELETE FROM users WHERE id = ?", owner).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"body_facts", "statement_evidence", "body_fact_commits", "body_state_outbox", "body_state_revisions"} {
		assertBodyFactCount(t, db, table, owner, 0)
	}
	assertBodyFactCount(t, db, "body_facts", other, 2)
}

func TestBodyFactPaginationPreservesWholeBatchesPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	for i := int64(0); i < 21; i++ {
		if _, err := repo.RecordDirectFacts(ctx, userID, bodyFactTestCommand(fmt.Sprintf("page-%d", i), i)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.ListFacts(ctx, userID, 0)
	if err != nil || len(first.Facts) != 40 || !first.HasMore || first.NextBeforeRevision == nil {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	second, err := repo.ListFacts(ctx, userID, *first.NextBeforeRevision)
	if err != nil || len(second.Facts) != 2 || second.HasMore || second.Facts[0].CreatedRevision != 1 {
		t.Fatalf("second=%#v err=%v", second, err)
	}
}
