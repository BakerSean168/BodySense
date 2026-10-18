package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func freshnessByCondition(t *testing.T, view *NativeDiagnosisView) map[uuid.UUID]*DiagnosisFreshness {
	t.Helper()
	out := map[uuid.UUID]*DiagnosisFreshness{}
	for _, condition := range view.ConditionEntries {
		if condition.Freshness == nil {
			t.Fatalf("condition %s presented without freshness", condition.ConditionID)
		}
		out[condition.ConditionID] = condition.Freshness
	}
	return out
}

func hasFreshnessReason(f *DiagnosisFreshness, code string) *DiagnosisFreshnessReason {
	for i := range f.Reasons {
		if f.Reasons[i].Code == code {
			return &f.Reasons[i]
		}
	}
	return nil
}

func freshnessAuditCounts(t *testing.T, db *gorm.DB, owner uuid.UUID) map[string]int64 {
	t.Helper()
	counts := map[string]int64{}
	for _, table := range []string{"condition_entries", "condition_entry_evidence", "diagnosis_revisions", "condition_analysis_events", "condition_direct_edits", "diagnosis_settlements"} {
		var n int64
		if err := db.Table(table).Where("user_id=?", owner).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		counts[table] = n
	}
	return counts
}

func TestDiagnosisFreshnessConditionAndRunPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f, entries := settleFixture(t, db)
	a, b := entries[0], entries[1]
	freshness := NewDiagnosisFreshnessService(db)
	plain := NewNativeDiagnosisReadService(repository.NewDiagnosisRepository(db))
	reviewed := NewReviewedNativeDiagnosisReader(plain, freshness)
	runs := NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(db)).WithFreshness(freshness)
	read := func() map[uuid.UUID]*DiagnosisFreshness {
		t.Helper()
		view, err := reviewed.Read(ctx, f.owner)
		if err != nil {
			t.Fatal(err)
		}
		return freshnessByCondition(t, view)
	}

	// Freshly adopted Conditions are current, based on their run's BodyState.
	for id, got := range read() {
		if got.State != DiagnosisFreshnessCurrent || got.BasedOnBodyStateRevision != f.run.BasedOnBodyStateRevision ||
			got.EvaluatedDiagnosisRevision != 1 || len(got.Reasons) != 0 {
			t.Fatalf("condition %s: %+v", id, got)
		}
	}
	// The Agent-facing read never carries the derived signal.
	if view, err := plain.Read(ctx, f.owner); err != nil || view.ConditionEntries[0].Freshness != nil {
		t.Fatalf("plain read must stay the committed projection: %+v %v", view, err)
	}

	// An open run on head 1 is current until the Diagnosis moves under it.
	open := nextSettlementRun(t, db, f, []uuid.UUID{a.ID},
		settlementProposalSpec{model.DiagnosisUpdate, &a.ID, map[string]any{"proposedEntry": map[string]any{"causeAnalysis": map[string]string{"summary": "Run explanation", "rationale": "No causal certainty claimed"}}}})
	projection, err := runs.Read(ctx, f.owner, open.run.ID)
	if err != nil || projection.Freshness == nil || projection.Freshness.State != DiagnosisFreshnessCurrent {
		t.Fatalf("open run freshness %+v %v", projection, err)
	}
	if terminal, err := runs.Read(ctx, f.owner, f.run.ID); err != nil || terminal.Freshness != nil {
		t.Fatalf("a committed run is history and has no freshness: %+v %v", terminal, err)
	}

	// A ConditionEntry change (direct withdrawal of b) makes the open run
	// visibly stale and names the changed Condition, before settlement 409s.
	if _, err := NewDiagnosisDirectEditService(db).Apply(ctx, f.owner, directEditCommand(f, b.ID, model.ConditionWithdraw, 1)); err != nil {
		t.Fatal(err)
	}
	projection, err = runs.Read(ctx, f.owner, open.run.ID)
	if err != nil || projection.Freshness == nil || projection.Freshness.State != DiagnosisFreshnessStale || projection.Freshness.EvaluatedDiagnosisRevision != 2 {
		t.Fatalf("run after direct edit %+v %v", projection, err)
	}
	if r := hasFreshnessReason(projection.Freshness, freshnessDiagnosisRevisionChanged); r == nil || *r.DiagnosisRevision != 2 {
		t.Fatalf("missing diagnosis revision reason: %+v", projection.Freshness)
	}
	if r := hasFreshnessReason(projection.Freshness, freshnessConditionChanged); r == nil || *r.ConditionID != b.ID || *r.DiagnosisRevision != 2 {
		t.Fatalf("missing changed condition: %+v", projection.Freshness)
	}
	if _, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(open)); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("stale run settled: %v", err)
	}
	// A direct edit is not a new analysis: a's basis and freshness are unchanged.
	if got := read()[a.ID]; got.State != DiagnosisFreshnessCurrent || got.EvaluatedDiagnosisRevision != 2 {
		t.Fatalf("a after unrelated edit: %+v", got)
	}

	// New related evidence recommends review; nothing is written.
	before := freshnessAuditCounts(t, db, f.owner)
	commit, err := repository.NewBodyStateRepository(db).RecordDirectFacts(ctx, f.owner, model.RecordBodyFactsCommand{
		ExpectedRevision: 1, IdempotencyKey: uuid.NewString(),
		Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "Synthetic user reports the knee discomfort returned", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var newFacts []uuid.UUID
	if err := json.Unmarshal(commit.FactIDs, &newFacts); err != nil || len(newFacts) != 1 {
		t.Fatalf("new fact ids %v %v", newFacts, err)
	}
	got := read()[a.ID]
	if r := hasFreshnessReason(got, freshnessRelatedFactRecorded); got.State != DiagnosisFreshnessReviewRecommended || r == nil ||
		*r.BodyStateRevision != 2 || *r.EvidenceID != newFacts[0] || got.EvaluatedBodyStateRevision != 2 {
		t.Fatalf("related fact: %+v", got)
	}

	// Withdrawing the cited fact makes the adopted Condition stale, with the
	// exact BodyState revision as its auditable cause.
	if _, err := repository.NewBodyStateRepository(db).WithdrawDirectFact(ctx, f.owner, f.fact.ID, model.AmendBodyFactCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 2}); err != nil {
		t.Fatal(err)
	}
	got = read()[a.ID]
	if r := hasFreshnessReason(got, freshnessCitedEvidenceWithdrawn); got.State != DiagnosisFreshnessStale || r == nil ||
		*r.BodyStateRevision != 3 || r.ChangeType != "facts.withdrawn" || *r.EvidenceID != f.fact.ID || *r.EvidenceType != model.DiagnosisBodyFact {
		t.Fatalf("withdrawn cited fact: %+v", got)
	}
	if hasFreshnessReason(got, freshnessCitedEvidenceUnavailable) != nil {
		t.Fatalf("revision-attributed loss repeated as unattributed: %+v", got)
	}
	projection, err = runs.Read(ctx, f.owner, open.run.ID)
	if err != nil || hasFreshnessReason(projection.Freshness, freshnessCitedEvidenceWithdrawn) == nil {
		t.Fatalf("open run must also see its cited fact withdrawn: %+v %v", projection, err)
	}

	// The projection never edits the Diagnosis or its audit.
	if after := freshnessAuditCounts(t, db, f.owner); len(after) != len(before) {
		t.Fatal("audit tables changed")
	} else {
		for table, n := range before {
			if after[table] != n {
				t.Fatalf("freshness read wrote %s: %d -> %d", table, n, after[table])
			}
		}
	}
	if entry := directEditEntry(t, db, f.owner, a.ID); entry.Disposition != model.ConditionAdopted || entry.CommittedRevision != 1 ||
		entry.LatestDecisionID != a.LatestDecisionID || directEditHead(t, db, f.owner) != 2 {
		t.Fatalf("stale Condition was silently edited: %+v", entry)
	}

	// TrainingPlan generation refuses the stale adopted Condition (DIAGNOSIS_STALE).
	current, err := plain.Read(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	gate := &TreatmentService{}
	gate.AttachDiagnosisFreshness(freshness)
	if err := gate.requireFreshDiagnosis(ctx, f.owner, adoptedTreatmentDiagnosis(current)); !errors.Is(err, ErrTreatmentDiagnosisStale) {
		t.Fatalf("stale adopted Condition passed the generation gate: %v", err)
	}

	// An unavailable source claim or request statement is stale too.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.request.ID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	got = read()[a.ID]
	if hasFreshnessReason(got, freshnessSourceClaimUnavailable) == nil || hasFreshnessReason(got, freshnessAssessmentUnavailable) == nil {
		t.Fatalf("blocked analysis request: %+v", got)
	}
	projection, err = runs.Read(ctx, f.owner, open.run.ID)
	if err != nil || hasFreshnessReason(projection.Freshness, freshnessRequestUnavailable) == nil {
		t.Fatalf("open run request source: %+v %v", projection, err)
	}

	// Freshness is owner-scoped: another owner's view cannot be evaluated here.
	foreign := &NativeDiagnosisView{UserID: uuid.New(), ConditionEntries: current.ConditionEntries}
	if _, err := freshness.EvaluateConditions(ctx, f.owner, foreign); err == nil {
		t.Fatal("foreign view evaluated")
	}
}
