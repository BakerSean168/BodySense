package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDiagnosisAggregatePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()
	// Concurrent initialization is idempotent and never creates a revision.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { _, err := repo.EnsureAggregate(ctx, owner); errs <- err })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertBodyFactCount(t, db, "diagnoses", owner, 1)
	assertBodyFactCount(t, db, "diagnosis_revisions", owner, 0)
	head, err := repo.GetHead(ctx, owner)
	if err != nil || head.CurrentRevision != 0 {
		t.Fatalf("head=%#v err=%v", head, err)
	}
	requireDiagnosisSQLState(t, db.Exec("INSERT INTO diagnoses(user_id) VALUES (?)", owner).Error, "23505")
	requireDiagnosisSQLState(t, db.Exec("UPDATE diagnoses SET current_revision=1 WHERE user_id=?", owner).Error, "23503")
	requireDiagnosisSQLState(t, db.Exec("UPDATE diagnoses SET current_revision=-1 WHERE user_id=?", owner).Error, "23514")
	// Caller rollback must include repository initialization.
	fresh := seedBodyStateContextUser(t, db)
	abort := errors.New("caller abort")
	err = database.NewTransactionManager(db).WithinTransaction(ctx, func(txctx context.Context) error {
		if _, err := repo.EnsureAggregate(txctx, fresh); err != nil {
			return err
		}
		if _, err := repo.GetHead(txctx, fresh); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	if _, err := repo.GetHead(ctx, fresh); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatal(err)
	}
	if _, err := repo.EnsureAggregate(ctx, uuid.Nil); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatal(err)
	}
}

type diagnosisFixture struct {
	owner       uuid.UUID
	run         model.ConditionAnalysisRun
	proposal    model.DiagnosisProposal
	decision    model.DiagnosisDecision
	revision    model.DiagnosisRevision
	entry       model.ConditionEntry
	fact        model.BodyFact
	observation model.BodyObservation
	episode     uuid.UUID
}

// Synthetic explicit user request and authorized history, inserted as test data.
// This does not implement or exercise the future decision application service.
func seedDiagnosisFixture(t *testing.T, db *gorm.DB) diagnosisFixture {
	t.Helper()
	ctx := context.Background()
	f := diagnosisFixture{owner: seedBodyStateContextUser(t, db), episode: uuid.New()}
	repo := NewDiagnosisRepository(db)
	if _, err := repo.EnsureAggregate(ctx, f.owner); err != nil {
		t.Fatal(err)
	}
	body := NewBodyStateRepository(db)
	if _, err := body.RecordDirectFacts(ctx, f.owner, bodyFactTestCommand("dx-facts", 0)); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=? AND kind='symptom'", f.owner).Take(&f.fact).Error; err != nil {
		t.Fatal(err)
	}
	obs, err := body.RecordDirectObservation(ctx, f.owner, observationDirectCommand(1, "dx-observation", "My measured weight is 67.5 kg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=? AND id=?", f.owner, obs.ObservationID).Take(&f.observation).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := db.Exec(`INSERT INTO symptom_episodes(id,user_id,symptom_type,body_region_ids,laterality,created_revision,created_at)
 VALUES (?,?,'shoulder discomfort','[]','unknown',2,?)`, f.episode, f.owner, now).Error; err != nil {
		t.Fatal(err)
	}
	source := seedUserMessageFactEnvelope(t, db, f.owner, datatypes.JSON(`[{"type":"text","text":"Please formally analyze my shoulder evidence"}]`), "user")
	var chat model.Run
	if err := db.Take(&chat, "id=?", source.RunID).Error; err != nil {
		t.Fatal(err)
	}
	evidence := model.StatementEvidence{ID: uuid.New(), UserID: f.owner, SourceKind: "user_message", SourceVersion: 1,
		SourceConversationID: &chat.ConversationID, SourceRunID: &source.RunID, SourceMessageID: &source.MessageID,
		Excerpt: "Please formally analyze my shoulder evidence", Availability: "available", RecordedAt: now}
	if err := db.Create(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	refs := []model.DiagnosisEvidenceRef{{EvidenceType: model.DiagnosisBodyFact, EvidenceID: f.fact.ID, Version: "1"}}
	f.run = model.ConditionAnalysisRun{UserID: f.owner, ID: uuid.New(), RequestMessageID: source.MessageID,
		RequestEvidenceID: evidence.ID, RequestSourceVersion: 1, Trigger: "new_analysis",
		Scope:                    model.ConditionAnalysisScope{ConditionIDs: []uuid.UUID{}, EvidenceRefs: refs, Description: "shoulder"},
		BasedOnBodyStateRevision: 2, InputEvidenceRefs: refs, AgentHypotheses: []model.ConditionAgentHypothesis{},
		ReplayInput: datatypes.JSON(`{"snapshot":"frozen"}`), ExecutionProvenance: datatypes.JSON(`{"provider":"synthetic"}`),
		IdempotencyKey: "dx-run", RequestFingerprint: strings.Repeat("a", 64), StartedAt: now}
	if err := db.Create(&f.run).Error; err != nil {
		t.Fatal(err)
	}
	f.proposal = model.DiagnosisProposal{UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, ChangeKind: model.DiagnosisCreate,
		Content: datatypes.JSON(`{"proposedEntry":{"medicalName":"Synthetic named syndrome"}}`), ContentFingerprint: strings.Repeat("b", 64),
		EvidenceRefs: refs, CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: now}
	if err := db.Create(&f.proposal).Error; err != nil {
		t.Fatal(err)
	}
	// An AI proposal is not an adopted condition or aggregate mutation.
	assertBodyFactCount(t, db, "condition_entries", f.owner, 0)
	f.decision = model.DiagnosisDecision{UserID: f.owner, ID: uuid.New(), ProposalID: f.proposal.ID, RunID: f.run.ID,
		ContentFingerprint: f.proposal.ContentFingerprint, DecidedEvidenceRefs: refs, Decision: model.DiagnosisAccept,
		ToolCallID: "exact-tool", DecidedAt: now, IdempotencyKey: "dx-decision", RequestFingerprint: strings.Repeat("c", 64), ReceiptID: uuid.New()}
	if err := db.Create(&f.decision).Error; err != nil {
		t.Fatal(err)
	}
	assertBodyFactCount(t, db, "condition_entries", f.owner, 0)
	assertBodyFactCount(t, db, "diagnosis_revisions", f.owner, 0)
	head, err := repo.GetHead(ctx, f.owner)
	if err != nil || head.CurrentRevision != 0 {
		t.Fatalf("decision changed head: %#v %v", head, err)
	}
	f.revision = model.DiagnosisRevision{UserID: f.owner, ID: uuid.New(), Revision: 1, RunID: &f.run.ID,
		Patch: datatypes.JSON(`[{"op":"create","medicalName":"Synthetic named syndrome"}]`), DecisionIDs: []uuid.UUID{f.decision.ID},
		BodyStateRevision: 2, EvidenceRefs: refs, ReplayInput: f.run.ReplayInput, ExecutionProvenance: f.run.ExecutionProvenance,
		IdempotencyKey: "dx-commit", RequestFingerprint: strings.Repeat("d", 64), ReceiptID: uuid.New(), CommittedAt: now}
	f.entry = model.ConditionEntry{UserID: f.owner, ID: uuid.New(), MedicalName: "Synthetic named syndrome",
		Assessment: &model.ConditionAssessment{Level: model.DiagnosisSuspected, Rationale: "Synthetic analysis", InformationGaps: []string{},
			DifferentialNotes: []string{}, BasedOnRunID: f.run.ID},
		SourceClaimRefs: []model.DiagnosisSourceClaimRef{{ClaimID: "synthetic-claim", SourceKind: "user_suspicion", SourceID: evidence.ID.String(),
			SourceVersion: "1", Statement: "Could my shoulder have a named syndrome?", RecordedAt: now, ValidityAtAssessment: "current"}},
		CauseAnalysis: model.ConditionCauseAnalysis{Summary: "Unknown", Rationale: "Insufficient causal evidence"}, InformationGaps: []string{},
		HandlingPlan: model.ConditionHandlingPlan{Summary: "Seek assessment", Actions: []string{}, CautionNotes: []string{}},
		Disposition:  model.ConditionAdopted, LatestDecisionID: f.decision.ID, LastAnalysisRunID: &f.run.ID, CommittedRevision: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&f.revision).Error; err != nil {
			return err
		}
		if err := tx.Create(&f.entry).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE diagnoses SET current_revision=1 WHERE user_id=?", f.owner).Error
	}); err != nil {
		t.Fatal(err)
	}
	return f
}

func requireDiagnosisSQLState(t *testing.T, err error, code string) {
	t.Helper()
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != code {
		t.Fatalf("want SQLSTATE %s, got %v", code, err)
	}
}

func TestDiagnosisOwnerMaskedReadsAndFrozenReplayPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	f := seedDiagnosisFixture(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()
	entry, err := repo.GetCondition(ctx, f.owner, f.entry.ID)
	if err != nil || entry.Assessment == nil || entry.Assessment.Level != model.DiagnosisSuspected || entry.Disposition != model.ConditionAdopted {
		t.Fatalf("entry=%#v err=%v", entry, err)
	}
	if rows, err := repo.ListConditions(ctx, foreign); err != nil || len(rows) != 0 {
		t.Fatalf("foreign list=%#v %v", rows, err)
	}
	for _, id := range []uuid.UUID{f.entry.ID, uuid.New()} {
		if _, err := repo.GetCondition(ctx, foreign, id); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatalf("owner-masked 404 seam: %v", err)
		}
		if _, err := repo.ListEvidence(ctx, foreign, id); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
	}
	for _, owner := range []uuid.UUID{foreign, uuid.Nil} {
		if _, err := repo.GetAnalysisRun(ctx, owner, f.run.ID); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
		if _, err := repo.GetProposal(ctx, owner, f.proposal.ID); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
		if _, err := repo.GetDecisionReceipt(ctx, owner, "dx-decision"); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
		if _, err := repo.GetRevision(ctx, owner, 1); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatal(err)
		}
	}
	proposal, err := repo.GetProposal(ctx, f.owner, f.proposal.ID)
	if err != nil || proposal.ContentFingerprint != f.proposal.ContentFingerprint || proposal.EvidenceRefs[0].Version != "1" {
		t.Fatalf("proposal=%#v %v", proposal, err)
	}
	for range 2 {
		receipt, err := repo.GetDecisionReceipt(ctx, f.owner, "dx-decision")
		if err != nil || receipt.ID != f.decision.ID || receipt.ReceiptID != f.decision.ReceiptID || receipt.ToolCallID != "exact-tool" {
			t.Fatalf("receipt=%#v %v", receipt, err)
		}
	}
	// Source changes must not rewrite an already frozen replay base.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.fact.SourceEvidenceID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&model.Message{}, "id=?", f.run.RequestMessageID).Error; err != nil {
		t.Fatal(err)
	}
	run, err := repo.GetAnalysisRun(ctx, f.owner, f.run.ID)
	if err != nil || !jsonEqual(run.ReplayInput, f.run.ReplayInput) || !jsonEqual(run.ExecutionProvenance, f.run.ExecutionProvenance) {
		t.Fatalf("replay changed: %#v %v", run, err)
	}
	rev, err := repo.GetRevision(ctx, f.owner, 1)
	if err != nil || rev.ReceiptID != f.revision.ReceiptID || !jsonEqual(rev.ReplayInput, f.run.ReplayInput) {
		t.Fatalf("revision=%#v %v", rev, err)
	}
	for _, table := range []string{"condition_analysis_runs", "diagnosis_proposals", "diagnosis_decisions", "diagnosis_revisions"} {
		requireDiagnosisSQLState(t, db.Exec("UPDATE "+table+" SET id=id WHERE user_id=?", f.owner).Error, "23514")
	}
	counts, err := NewPrivacyErasureRepository(db).CountUserData(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"diagnoses", "condition_analysis_runs", "condition_entries", "diagnosis_proposals", "diagnosis_decisions", "diagnosis_revisions"} {
		found := false
		for _, count := range counts {
			if count.Name == table && count.Count == 1 {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing privacy inventory: %s", table)
		}
	}
	if err := db.Exec("DELETE FROM users WHERE id=?", f.owner).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"diagnoses", "condition_analysis_runs", "condition_entries", "condition_entry_evidence", "diagnosis_proposals", "diagnosis_decisions", "diagnosis_revisions"} {
		assertBodyFactCount(t, db, table, f.owner, 0)
	}
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return string(ax) == string(by)
}
