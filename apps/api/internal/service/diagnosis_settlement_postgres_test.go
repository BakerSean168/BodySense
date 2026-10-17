package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func diagnosisSettlementEntry(f diagnosisDecisionFixture, name string) map[string]any {
	return map[string]any{
		"medicalName":     name,
		"conceptRef":      map[string]any{"system": "urn:synthetic:medical-concepts", "code": name},
		"assessment":      map[string]any{"level": "suspected", "rationale": "Synthetic formal proposal, not a clinical validation", "basedOnRunId": f.run.ID, "informationGaps": []string{}, "differentialNotes": []string{}},
		"sourceClaimRefs": []map[string]any{{"claimId": "synthetic-run-claim", "validityAtAssessment": "current", "sourceKind": "agent_analysis", "sourceId": f.run.ID.String(), "sourceVersion": "1", "statement": "Synthetic formal analysis hypothesis", "recordedAt": f.run.StartedAt}},
		"causeAnalysis":   map[string]any{"summary": "Causality not established", "rationale": "Causal inference not established"},
		"informationGaps": []string{},
		"handlingPlan":    map[string]any{"summary": "Seek human clinical review", "actions": []string{}, "cautionNotes": []string{}},
		"evidenceLinks":   []map[string]any{{"evidenceType": "body_fact", "evidenceId": f.fact.ID, "evidenceVersion": "1", "role": "supporting", "rationale": "Synthetic source qualified in this formal analysis", "analyzedByRunId": f.run.ID}},
	}
}
func seedDiagnosisSettlementFixture(t *testing.T, db *gorm.DB) diagnosisDecisionFixture {
	t.Helper()
	f := seedDiagnosisDecisionFixture(t, db)
	for i, old := range f.proposals {
		p := old
		p.ID = uuid.New()
		p.SupersedesProposalID = &old.ID
		content, err := json.Marshal(map[string]any{"proposedEntry": diagnosisSettlementEntry(f, []string{"Synthetic A syndrome", "Synthetic B syndrome"}[i])})
		if err != nil {
			t.Fatal(err)
		}
		p.Content = datatypes.JSON(content)
		p.ContentFingerprint, err = model.DiagnosisProposalContentFingerprint(&p)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		f.proposals[i] = p
	}
	return f
}
func diagnosisSettleCommand(f diagnosisDecisionFixture) DiagnosisSettlementCommand {
	return DiagnosisSettlementCommand{RunID: f.run.ID, ExpectedRevision: f.run.BasedOnDiagnosisRevision, ToolCallID: "user-close-formal-analysis", IdempotencyKey: uuid.NewString()}
}
func recordSettlementChoice(t *testing.T, db *gorm.DB, f diagnosisDecisionFixture, index int, kind model.DiagnosisDecisionKind) *model.DiagnosisDecision {
	t.Helper()
	c := diagnosisDecisionCommand(f, index)
	c.ExpectedRevision = f.run.BasedOnDiagnosisRevision
	c.Decision = kind
	r, err := NewDiagnosisProposalDecisionService(db).Record(context.Background(), f.owner, c)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestDiagnosisSettlementPartialThenAtomicCommitPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	f := seedDiagnosisSettlementFixture(t, db)
	svc := NewDiagnosisSettlementService(db)
	ctx := context.Background()
	first := recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
	command := diagnosisSettleCommand(f)
	if _, err := svc.Settle(ctx, f.owner, command); !errors.Is(err, ErrDiagnosisChoicesIncomplete) {
		t.Fatalf("partial analysis settled: %v", err)
	}
	assertDiagnosisNotAdopted(t, db, f.owner)
	second := recordSettlementChoice(t, db, f, 1, model.DiagnosisAccept)
	result, err := svc.Settle(ctx, f.owner, command)
	if err != nil || result == nil || result.Outcome != "committed" || result.Revision != 1 || result.DiagnosisRevisionID == nil {
		t.Fatalf("commit %+v: %v", result, err)
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_entries", 2)
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 1)
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_settlements", 1)
	var entries []model.ConditionEntry
	if err := db.Where("user_id=?", f.owner).Find(&entries).Error; err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Disposition != model.ConditionAdopted || entry.CommittedRevision != 1 || (entry.LatestDecisionID != first.ID && entry.LatestDecisionID != second.ID) {
			t.Fatalf("partial provenance: %+v", entry)
		}
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_entry_evidence", 2)
	if _, err := svc.Settle(ctx, f.owner, diagnosisSettleCommand(f)); !errors.Is(err, ErrDiagnosisAnalysisClosed) {
		t.Fatalf("closed run settled twice: %v", err)
	}
	// A lost response returns the same materialized result, even after its sources change.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.fact.SourceEvidenceID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	replayed, err := svc.Settle(ctx, f.owner, command)
	if err != nil || replayed.ID != result.ID || !replayed.SettledAt.Equal(result.SettledAt) {
		t.Fatalf("settlement replay %+v %v", replayed, err)
	}
	changed := command
	changed.CloseRemaining = true
	if _, err := svc.Settle(ctx, f.owner, changed); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("different retry body: %v", err)
	}
}
func TestDiagnosisSettlementRejectAndExplicitClosePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("all rejected does not create empty revision", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		recordSettlementChoice(t, db, f, 0, model.DiagnosisReject)
		recordSettlementChoice(t, db, f, 1, model.DiagnosisReject)
		r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f))
		if err != nil || r.Outcome != "closed_without_change" || r.Revision != 0 || r.DiagnosisRevisionID != nil {
			t.Fatalf("rejected result %+v %v", r, err)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
	})
	t.Run("explicit close expires pending and only applies accepted", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		command := diagnosisSettleCommand(f)
		command.CloseRemaining = true
		r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, command)
		if err != nil || r.Outcome != "committed" || len(r.ConditionIDs) != 1 {
			t.Fatalf("explicit close %+v %v", r, err)
		}
		diagnosisDecisionCount(t, db, f.owner, "condition_entries", 1)
		var count int64
		if err := db.Model(&model.ConditionAnalysisEvent{}).Where("user_id=? AND run_id=? AND proposal_id=? AND kind='proposal_expired'", f.owner, f.run.ID, f.proposals[1].ID).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("pending not expired: %d %v", count, err)
		}
	})
}
func TestDiagnosisSettlementConcurrentAndRollbackPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("one revision across concurrent retries", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		recordSettlementChoice(t, db, f, 1, model.DiagnosisAccept)
		svc := NewDiagnosisSettlementService(db)
		c := diagnosisSettleCommand(f)
		var wg sync.WaitGroup
		out := make(chan *model.DiagnosisSettlement, 6)
		errs := make(chan error, 6)
		for range 6 {
			wg.Go(func() { r, e := svc.Settle(ctx, f.owner, c); out <- r; errs <- e })
		}
		wg.Wait()
		close(out)
		close(errs)
		for e := range errs {
			if e != nil {
				t.Fatal(e)
			}
		}
		id := uuid.Nil
		for r := range out {
			if r == nil {
				t.Fatal("nil result")
			}
			if id == uuid.Nil {
				id = r.ID
			}
			if id != r.ID {
				t.Fatal("duplicate settlement")
			}
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 1)
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_settlements", 1)
	})
	t.Run("failure after first entry rolls back all adopted state", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		recordSettlementChoice(t, db, f, 1, model.DiagnosisAccept)
		callback := "test:diagnosis-settle-rollback:" + uuid.NewString()
		sentinel := errors.New("injected second condition failure")
		seen := 0
		if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if c, ok := tx.Statement.Dest.(*model.ConditionEntry); ok && c.UserID == f.owner {
				seen++
				if seen == 2 {
					tx.AddError(sentinel)
				}
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
		if r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); !errors.Is(err, sentinel) || r != nil {
			t.Fatalf("partial commit %+v %v", r, err)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_settlements", 0)
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 2)
	})
}
func TestDiagnosisSettlementRechecksSafetyAndSourcesPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("source lost after decision cannot be adopted", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		recordSettlementChoice(t, db, f, 1, model.DiagnosisReject)
		if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.fact.SourceEvidenceID).Update("availability", "blocked").Error; err != nil {
			t.Fatal(err)
		}
		if r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); err == nil || r != nil {
			t.Fatalf("lost source adopted: %+v %v", r, err)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 2)
	})
	t.Run("urgent after partial acceptance yields zero adoption", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		svc := NewDiagnosisSettlementService(db)
		svc.projectSafety = func(snapshot *BodyStateReadModel) (SafetyEnvelopeV2, error) {
			e, err := ProjectSafetyEnvelopeV2(snapshot)
			e.ActiveBlockers = []SafetyBlockerV1{{RiskLevel: "urgent"}}
			return e, err
		}
		c := diagnosisSettleCommand(f)
		c.CloseRemaining = true
		r, err := svc.Settle(ctx, f.owner, c)
		if !errors.Is(err, ErrDiagnosisSafetyBlocked) || r == nil || r.Outcome != "safety_blocked" {
			t.Fatalf("urgent outcome %+v %v", r, err)
		}
		replay, replayErr := svc.Settle(ctx, f.owner, c)
		if !errors.Is(replayErr, ErrDiagnosisSafetyBlocked) || replay.ID != r.ID {
			t.Fatalf("urgent replay %+v %v", replay, replayErr)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_decisions", 1)
	})
	t.Run("foreign run remains masked", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		other := seedDiagnosisSettlementFixture(t, db)
		if _, err := NewDiagnosisSettlementService(db).Settle(ctx, other.owner, diagnosisSettleCommand(f)); !errors.Is(err, model.ErrDiagnosisNotFound) {
			t.Fatalf("foreign run %v", err)
		}
	})
}

// Relation/disposition/evidence-addendum cases live in the companion test file.
