package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func execStatementTarget(t *testing.T, repo *StatementErasureRepository, request *model.StatementErasureRequest, target model.StatementErasureTargetName) model.AssetErasureOutcome {
	t.Helper()
	outcome, err := repo.ExecuteDatabaseTarget(context.Background(), request, target)
	if err != nil {
		t.Fatalf("%s: %v", target, err)
	}
	if err := repo.RecordOutcome(context.Background(), request, target, outcome); err != nil {
		t.Fatal(err)
	}
	return outcome
}

func wantStatementOutcome(t *testing.T, target model.StatementErasureTargetName, got model.AssetErasureOutcome, state model.AssetErasureTargetState, code string, outstanding int) {
	t.Helper()
	if got.State != state || got.FailureCode != code || got.Outstanding != outstanding {
		t.Fatalf("%s: got %+v want state=%s code=%q outstanding=%d", target, got, state, code, outstanding)
	}
}

// decisionTrail is the frozen Diagnosis decision trail of one owner as JSON.
func decisionTrail(t *testing.T, db *gorm.DB, owner uuid.UUID) string {
	t.Helper()
	var out string
	if err := db.Raw(`SELECT jsonb_build_object(
 'runs',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM condition_analysis_runs x WHERE user_id=?),
 'proposals',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM diagnosis_proposals x WHERE user_id=?),
 'decisions',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM diagnosis_decisions x WHERE user_id=?),
 'revisions',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY id),'[]') FROM diagnosis_revisions x WHERE user_id=?),
 'evidence',(SELECT coalesce(jsonb_agg(to_jsonb(x) ORDER BY condition_id, evidence_id),'[]') FROM condition_entry_evidence x WHERE user_id=?)
)::text`, owner, owner, owner, owner, owner).Scan(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

func statementReceiptTarget(t *testing.T, receipt *model.StatementErasureReceipt, name model.StatementErasureTargetName) model.StatementErasureTarget {
	t.Helper()
	for _, target := range receipt.Targets {
		if target.Target == name {
			return target
		}
	}
	t.Fatalf("target %s missing", name)
	return model.StatementErasureTarget{}
}

// A statement whose derived fact an adopted condition cites stays pinned:
// the body record must be withdrawn and the condition withdrawn (or the
// Diagnosis revised) first. Then the fact's text becomes a tombstone, the
// assessment copy is deleted, the frozen decision trail is retained by policy
// unchanged, and the request ends erased_with_retention, never erased.
func TestBodyFactStatementErasurePinnedWithdrawTombstonePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisFixture(t, db)
	foreign := seedBodyStateContextUser(t, db)
	statement := f.fact.SourceEvidenceID
	link := model.ConditionEntryEvidence{UserID: f.owner, ConditionID: f.entry.ID, EvidenceType: model.DiagnosisBodyFact,
		EvidenceID: f.fact.ID, EvidenceVersion: f.fact.CreatedRevision, Role: model.DiagnosisSupporting,
		Rationale: "Synthetic relevance", AnalyzedByRunID: f.run.ID}
	if err := db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO assessment_reports(user_id,status,observations) VALUES (?,'completed',?::jsonb)`,
		f.owner, `[{"ref":"body_fact:`+f.fact.ID.String()+`"}]`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewStatementErasureRepository(db)
	if _, err := repo.CreateOrGet(ctx, foreign, statement, "foreign"); !errors.Is(err, model.ErrStatementErasureNotFound) {
		t.Fatalf("foreign statement: %v", err)
	}
	receipt, err := repo.CreateOrGet(ctx, f.owner, statement, "erase-statement")
	if err != nil || receipt.Request.Status != model.StatementErasurePending || len(receipt.Targets) != len(model.StatementErasureTargets) {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if _, err := repo.CreateOrGet(ctx, f.owner, f.observation.SourceEvidenceID, "erase-statement"); !errors.Is(err, model.ErrStatementErasureIdempotencyUsed) {
		t.Fatalf("key reuse: %v", err)
	}
	if again, err := repo.CreateOrGet(ctx, f.owner, statement, "another-key"); err != nil || again.Request.ID != receipt.Request.ID {
		t.Fatalf("repeat: %+v %v", again, err)
	}
	reviews := map[model.ErasureReviewSubject]uuid.UUID{}
	for _, review := range receipt.Reviews {
		reviews[review.Subject] = review.SubjectID
	}
	if len(receipt.Reviews) != 2 || reviews[model.ErasureReviewBodyFact] != f.fact.ID || reviews[model.ErasureReviewDiagnosisCondition] != f.entry.ID {
		t.Fatalf("reviews=%+v", receipt.Reviews)
	}
	request := &receipt.Request
	wantStatementOutcome(t, "diagnosis", execStatementTarget(t, repo, request, model.StatementErasureDiagnosisRecords),
		model.AssetErasureTargetBlocked, model.AssetErasureCodeDiagnosisReview, 1)
	wantStatementOutcome(t, "plans", execStatementTarget(t, repo, request, model.StatementErasureTrainingPlans), model.AssetErasureTargetVerified, "", 0)
	wantStatementOutcome(t, "body", execStatementTarget(t, repo, request, model.StatementErasureBodyRecords),
		model.AssetErasureTargetBlocked, model.ErasureCodeBodyRecordActive, 1)
	wantStatementOutcome(t, "assessment", execStatementTarget(t, repo, request, model.StatementErasureAssessmentReports),
		model.AssetErasureTargetBlocked, model.AssetErasureCodeAssessmentFrozen, 1)
	decisions := execStatementTarget(t, repo, request, model.StatementErasureDecisionRecords)
	if decisions.State != model.ErasureTargetRetainedByPolicy || decisions.RetentionReason != model.ErasureRetentionReplayInputFrozen || decisions.Outstanding < 1 {
		t.Fatalf("decisions: %+v", decisions)
	}
	if terminal, err := repo.Finalize(ctx, request); err != nil || terminal {
		t.Fatalf("finalize while pinned: %v %v", terminal, err)
	}

	// The user withdraws the derived fact; the adopted condition still pins its text.
	if err := db.Exec(`UPDATE body_facts SET assertion_status='withdrawn' WHERE user_id=? AND id=?`, f.owner, f.fact.ID).Error; err != nil {
		t.Fatal(err)
	}
	wantStatementOutcome(t, "body", execStatementTarget(t, repo, request, model.StatementErasureBodyRecords),
		model.AssetErasureTargetBlocked, model.AssetErasureCodeDiagnosisPinned, 1)
	// Withdrawing the condition releases the pin; the trail itself is frozen.
	if err := db.Exec(`UPDATE condition_entries SET disposition='inactive' WHERE user_id=? AND id=?`, f.owner, f.entry.ID).Error; err != nil {
		t.Fatal(err)
	}
	trail := decisionTrail(t, db, f.owner)
	wantStatementOutcome(t, "diagnosis", execStatementTarget(t, repo, request, model.StatementErasureDiagnosisRecords), model.AssetErasureTargetVerified, "", 0)
	if got := execStatementTarget(t, repo, request, model.StatementErasureBodyRecords); got.State != model.AssetErasureTargetVerified || got.Erased != 1 {
		t.Fatalf("body after review: %+v", got)
	}
	if got := execStatementTarget(t, repo, request, model.StatementErasureAssessmentReports); got.State != model.AssetErasureTargetVerified || got.Erased != 1 {
		t.Fatalf("assessment after review: %+v", got)
	}
	execStatementTarget(t, repo, request, model.StatementErasureDecisionRecords)
	scope, err := repo.RuntimeScope(ctx, f.owner, statement)
	if err != nil || len(scope.ThreadIDs) != 1 || len(scope.AttachedThreadIDs) != 0 || len(scope.RecordIDs) != 1 || scope.RecordIDs[0] != f.fact.ID {
		t.Fatalf("runtime scope=%+v err=%v", scope, err)
	}
	if _, err := repo.ExecuteDatabaseTarget(ctx, request, model.StatementErasureRuntimeCheckpoints); err == nil {
		t.Fatal("runtime checkpoints are not a database target")
	}
	if err := repo.RecordOutcome(ctx, request, model.StatementErasureRuntimeCheckpoints, model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}); err != nil {
		t.Fatal(err)
	}
	if terminal, err := repo.Finalize(ctx, request); err != nil || !terminal {
		t.Fatalf("finalize: %v %v", terminal, err)
	}
	receipt, err = repo.GetByStatement(ctx, f.owner, statement)
	if err != nil || receipt.Request.Status != model.StatementErasureErasedWithRetention || receipt.Request.ErasedAt == nil || len(receipt.Reviews) != 0 {
		t.Fatalf("terminal receipt=%+v err=%v", receipt, err)
	}
	retained := statementReceiptTarget(t, receipt, model.StatementErasureDecisionRecords)
	if retained.State != model.ErasureTargetRetainedByPolicy || retained.RetentionReason == nil || *retained.RetentionReason != model.ErasureRetentionReplayInputFrozen || retained.FailureCode != nil {
		t.Fatalf("retained target=%+v", retained)
	}
	if got := decisionTrail(t, db, f.owner); got != trail {
		t.Fatal("frozen decision trail was rewritten")
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM statement_evidence WHERE id=? AND availability='erased' AND excerpt='' AND question_excerpt IS NULL`, statement); n != 1 {
		t.Fatalf("statement not erased: %d", n)
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM body_facts WHERE id=? AND erased_at IS NOT NULL AND statement='' AND details='{}'`, f.fact.ID); n != 1 {
		t.Fatalf("fact tombstone: %d", n)
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM body_facts WHERE user_id=? AND id<>? AND erased_at IS NULL AND statement<>''`, f.owner, f.fact.ID); n != 1 {
		t.Fatalf("unrelated fact changed: %d", n)
	}
	if err := db.Exec(`UPDATE body_facts SET statement='back' WHERE id=?`, f.fact.ID).Error; err == nil {
		t.Fatal("fact tombstone revived")
	}
	if err := db.Exec(`UPDATE statement_erasure_requests SET status='outstanding', erased_at=NULL WHERE id=?`, request.ID).Error; err == nil {
		t.Fatal("terminal request reopened")
	}
	if terminal, err := repo.Finalize(ctx, request); err != nil || !terminal {
		t.Fatalf("repeat finalize: %v %v", terminal, err)
	}
	if ids, err := repo.ListRecoverable(ctx, 0, 10); err != nil || len(ids) != 0 {
		t.Fatalf("terminal request recoverable: %v %v", ids, err)
	}
}

// A statement nothing rests on is erased fully: every target verifies and
// the status is erased. The SQL guards refuse an early or dishonest terminal
// state and a retention outside decision_records.
func TestBodyFactStatementErasureCleanAndGuardsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	ctx := context.Background()
	owner := seedBodyStateContextUser(t, db)
	if _, err := NewBodyStateRepository(db).RecordDirectFacts(ctx, owner, bodyFactTestCommand("clean-facts", 0)); err != nil {
		t.Fatal(err)
	}
	var facts []model.BodyFact
	if err := db.Where("user_id=?", owner).Order("kind").Find(&facts).Error; err != nil || len(facts) != 2 {
		t.Fatalf("facts=%d err=%v", len(facts), err)
	}
	erase, keep := facts[0], facts[1]
	if err := db.Exec(`UPDATE body_facts SET assertion_status='withdrawn' WHERE id=?`, erase.ID).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewStatementErasureRepository(db)
	receipt, err := repo.CreateOrGet(ctx, owner, erase.SourceEvidenceID, "clean")
	if err != nil {
		t.Fatal(err)
	}
	request := &receipt.Request
	if len(receipt.Reviews) != 0 {
		t.Fatalf("reviews=%+v", receipt.Reviews)
	}
	for _, target := range model.StatementErasureTargets {
		switch target {
		case model.StatementErasureManifest:
		case model.StatementErasureRuntimeCheckpoints:
			if scope, err := repo.RuntimeScope(ctx, owner, erase.SourceEvidenceID); err != nil || len(scope.ThreadIDs) != 0 {
				t.Fatalf("scope=%+v err=%v", scope, err)
			}
			if err := repo.RecordOutcome(ctx, request, target, model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}); err != nil {
				t.Fatal(err)
			}
		default:
			if got := execStatementTarget(t, repo, request, target); got.State != model.AssetErasureTargetVerified {
				t.Fatalf("%s: %+v", target, got)
			}
		}
	}
	if terminal, err := repo.Finalize(ctx, request); err != nil || !terminal {
		t.Fatalf("finalize: %v %v", terminal, err)
	}
	receipt, _ = repo.GetByStatement(ctx, owner, erase.SourceEvidenceID)
	if receipt.Request.Status != model.StatementErasureErased {
		t.Fatalf("status=%s", receipt.Request.Status)
	}
	for _, target := range receipt.Targets {
		if target.State != model.AssetErasureTargetVerified {
			t.Fatalf("target %+v", target)
		}
	}
	if n := erasureRows(t, db, `SELECT count(*) FROM body_facts WHERE id=? AND erased_at IS NULL AND statement<>''`, keep.ID); n != 1 {
		t.Fatalf("kept fact changed: %d", n)
	}

	// Guards on a fresh request for the other statement.
	other, err := repo.CreateOrGet(ctx, owner, keep.SourceEvidenceID, "guard")
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"erased", "erased_with_retention"} {
		if err := db.Exec(`UPDATE statement_erasure_requests SET status=?, erased_at=now() WHERE id=?`, status, other.Request.ID).Error; err == nil {
			t.Fatalf("%s reported early", status)
		}
	}
	if err := db.Exec(`UPDATE statement_erasure_requests SET statement_id=? WHERE id=?`, erase.SourceEvidenceID, other.Request.ID).Error; err == nil {
		t.Fatal("request identity changed")
	}
	if err := db.Exec(`UPDATE statement_erasure_targets SET state='retained_by_policy', retention_reason='replay_input_frozen'
 WHERE request_id=? AND target='body_records'`, other.Request.ID).Error; err == nil {
		t.Fatal("retention outside decision_records")
	}
	if err := db.Exec(`UPDATE statement_erasure_targets SET state='retained_by_policy'
 WHERE request_id=? AND target='decision_records'`, other.Request.ID).Error; err == nil {
		t.Fatal("retention without a reason")
	}
	// An asserted record blocks; its text is never cleared while asserted.
	if err := db.Exec(`UPDATE body_facts SET erased_at=now(), statement='', details='{}', body_region_ids='[]', temporal='{"precision": "unknown"}' WHERE id=?`, keep.ID).Error; err == nil {
		t.Fatal("asserted fact tombstoned")
	}
}

// Every table that references a statement (FK or evidence id column) is in
// the statement inventory or documented as an id-only reference.
func TestBodyFactStatementErasureInventoryCoversSchemaPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	var tables []string
	if err := db.Raw(`SELECT DISTINCT table_name::text FROM information_schema.columns
 WHERE table_schema='public' AND column_name IN ('source_evidence_id','request_evidence_id','basis_evidence_id','statement_id')
 UNION
 SELECT DISTINCT c.conrelid::regclass::text FROM pg_constraint c
 WHERE c.contype='f' AND c.confrelid='public.statement_evidence'::regclass
 ORDER BY 1`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	inventoried := map[string]bool{
		"body_facts":                 true, // body_records (tombstoned text)
		"body_observations":          true, // body_records
		"body_hypotheses":            true, // body_records
		"body_fact_commits":          true, // id-only commit receipt
		"body_observation_commits":   true, // id-only commit receipt
		"body_hypothesis_commits":    true, // id-only commit receipt
		"episode_fact_links":         true, // id-only link to a tombstoned fact
		"episode_state_evidence":     true, // id-only link (claim is an enum)
		"episode_relations":          true, // id-only link to a tombstoned fact
		"safety_resolutions":         true, // id-only lift basis (no statement text)
		"condition_analysis_runs":    true, // decision_records (frozen replay input)
		"condition_direct_edits":     true, // decision_records
		"diagnosis_proposals":        true, // decision_records
		"statement_erasure_requests": true, // the receipt itself
	}
	for _, table := range tables {
		if !inventoried[strings.TrimPrefix(table, "public.")] {
			t.Errorf("table %s references statements but is not in the erasure inventory", table)
		}
	}
}
