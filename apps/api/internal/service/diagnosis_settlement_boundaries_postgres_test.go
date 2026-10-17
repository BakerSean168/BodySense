package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type settlementProposalSpec struct {
	kind    model.DiagnosisChangeKind
	target  *uuid.UUID
	content any
}

func settleFixture(t *testing.T, db *gorm.DB) (diagnosisDecisionFixture, []model.ConditionEntry) {
	t.Helper()
	f := seedDiagnosisSettlementFixture(t, db)
	for i := range f.proposals {
		recordSettlementChoice(t, db, f, i, model.DiagnosisAccept)
	}
	if _, err := NewDiagnosisSettlementService(db).Settle(context.Background(), f.owner, diagnosisSettleCommand(f)); err != nil {
		t.Fatal(err)
	}
	var entries []model.ConditionEntry
	if err := db.Where("user_id=?", f.owner).Order("medical_name").Find(&entries).Error; err != nil || len(entries) != 2 {
		t.Fatalf("seed committed entries: %v %+v", err, entries)
	}
	return f, entries
}

// Synthetic analysis records represent a fresh explicit request. Production
// ingestion of requests/proposals remains the DX-003 boundary, not test logic.
func nextSettlementRun(t *testing.T, db *gorm.DB, base diagnosisDecisionFixture, scope []uuid.UUID, specs ...settlementProposalSpec) diagnosisDecisionFixture {
	t.Helper()
	f := base
	var head model.Diagnosis
	var body model.BodyState
	if err := db.Where("user_id=?", f.owner).Take(&head).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=?", f.owner).Take(&body).Error; err != nil {
		t.Fatal(err)
	}
	f.run.ID = uuid.New()
	f.run.IdempotencyKey = uuid.NewString()
	f.run.RequestFingerprint = strings.Repeat("c", 64)
	f.run.StartedAt = time.Now().UTC()
	f.run.BasedOnDiagnosisRevision = head.CurrentRevision
	f.run.BasedOnBodyStateRevision = body.CurrentRevision
	f.run.Trigger = model.ConditionPartialUpdate
	f.run.Scope.ConditionIDs = scope
	if err := db.Create(&f.run).Error; err != nil {
		t.Fatal(err)
	}
	f.proposals = []model.DiagnosisProposal{}
	for _, spec := range specs {
		raw, err := json.Marshal(spec.content)
		if err != nil {
			t.Fatal(err)
		}
		p := model.DiagnosisProposal{UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, TargetConditionID: spec.target, ChangeKind: spec.kind, Content: datatypes.JSON(raw), BaseRevision: head.CurrentRevision, EvidenceRefs: append([]model.DiagnosisEvidenceRef{}, f.run.InputEvidenceRefs...), CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: time.Now().UTC()}
		p.ContentFingerprint, err = model.DiagnosisProposalContentFingerprint(&p)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		f.proposals = append(f.proposals, p)
	}
	return f
}
func appendSettlementEvent(t *testing.T, db *gorm.DB, f diagnosisDecisionFixture, kind string, proposal, decision *uuid.UUID, details any) {
	t.Helper()
	raw, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	var body model.BodyState
	if err := db.Where("user_id=?", f.owner).Take(&body).Error; err != nil {
		t.Fatal(err)
	}
	err = database.NewTransactionManager(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		return repository.NewDiagnosisRepository(db).AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: f.owner, RunID: f.run.ID, Kind: kind, ProposalID: proposal, DecisionID: decision, BodyStateRevision: body.CurrentRevision, Details: datatypes.JSON(raw), RecordedAt: time.Now().UTC()})
	})
	if err != nil {
		t.Fatal(err)
	}
}
func addSettlementBodyEvidence(t *testing.T, db *gorm.DB, owner uuid.UUID) model.DiagnosisEvidenceRef {
	t.Helper()
	var body model.BodyState
	if err := db.Where("user_id=?", owner).Take(&body).Error; err != nil {
		t.Fatal(err)
	}
	r, err := repository.NewBodyStateRepository(db).RecordDirectFacts(context.Background(), owner, model.RecordBodyFactsCommand{ExpectedRevision: body.CurrentRevision, IdempotencyKey: uuid.NewString(), Facts: []model.BodyFactInput{{Kind: "lifestyle", Statement: "Synthetic new user-authored daily activity", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	if err := json.Unmarshal(r.FactIDs, &ids); err != nil {
		t.Fatal(err)
	}
	var fact model.BodyFact
	if err := db.Where("user_id=? AND id=?", owner, ids[0]).Take(&fact).Error; err != nil {
		t.Fatal(err)
	}
	return model.DiagnosisEvidenceRef{EvidenceType: model.DiagnosisBodyFact, EvidenceID: fact.ID, Version: strconv.FormatInt(fact.CreatedRevision, 10)}
}

func TestDiagnosisSettlementUpdatesAndDispositionPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	base, entries := settleFixture(t, db)
	a, b := entries[0], entries[1]
	f := nextSettlementRun(t, db, base, []uuid.UUID{a.ID, b.ID},
		settlementProposalSpec{model.DiagnosisUpdate, &a.ID, map[string]any{"proposedEntry": map[string]any{"causeAnalysis": map[string]string{"summary": "New explicitly reviewed explanation", "rationale": "No causal certainty claimed"}}}},
		settlementProposalSpec{model.DiagnosisSetDisposition, &b.ID, map[string]any{"newDisposition": "inactive", "dispositionReason": "user_withdrawal"}})
	for i := range f.proposals {
		recordSettlementChoice(t, db, f, i, model.DiagnosisAccept)
	}
	r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f))
	if err != nil || r.Revision != 2 {
		t.Fatalf("update batch %+v %v", r, err)
	}
	var updatedA, updatedB model.ConditionEntry
	if err := db.Where("user_id=? AND id=?", f.owner, a.ID).Take(&updatedA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=? AND id=?", f.owner, b.ID).Take(&updatedB).Error; err != nil {
		t.Fatal(err)
	}
	if updatedA.CauseAnalysis.Summary != "New explicitly reviewed explanation" || updatedA.Disposition != model.ConditionAdopted || updatedB.Disposition != model.ConditionInactive || updatedB.MedicalName != b.MedicalName {
		t.Fatalf("unrequested mutation: %+v %+v", updatedA, updatedB)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 2)
	diagnosisDecisionCount(t, db, f.owner, "condition_disposition_changes", 1)
	diagnosisDecisionCount(t, db, f.owner, "condition_entry_evidence", 2)
	readopt := nextSettlementRun(t, db, base, []uuid.UUID{b.ID}, settlementProposalSpec{model.DiagnosisSetDisposition, &b.ID, map[string]any{"newDisposition": "adopted", "dispositionReason": "readopted"}})
	recordSettlementChoice(t, db, readopt, 0, model.DiagnosisAccept)
	if r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(readopt)); err != nil || r.Revision != 3 {
		t.Fatalf("readopt %+v %v", r, err)
	}
	if err := db.Where("user_id=? AND id=?", f.owner, b.ID).Take(&updatedB).Error; err != nil || updatedB.Disposition != model.ConditionAdopted {
		t.Fatalf("readopt identity changed: %+v %v", updatedB, err)
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_entries", 2)
	diagnosisDecisionCount(t, db, f.owner, "condition_disposition_changes", 2)
}

func TestDiagnosisSettlementRelationsPreserveUnrelatedStatePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	base, entries := settleFixture(t, db)
	a, b := entries[0], entries[1]
	for i, kind := range []string{"differential_with", "coexisting"} {
		f := nextSettlementRun(t, db, base, []uuid.UUID{a.ID, b.ID}, settlementProposalSpec{model.DiagnosisUpdateRelation, &a.ID, map[string]any{"relationChange": map[string]any{"fromConditionId": a.ID, "toConditionId": b.ID, "kind": kind, "basisEvidenceRefs": base.run.InputEvidenceRefs, "rationale": "Explicitly reviewed synthetic relation"}}})
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		if r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); err != nil || r.Revision != int64(i+2) {
			t.Fatalf("relation %+v %v", r, err)
		}
		diagnosisDecisionCount(t, db, f.owner, "condition_relations", int64(i+1))
	}
	for _, original := range entries {
		var current model.ConditionEntry
		if err := db.Where("user_id=? AND id=?", base.owner, original.ID).Take(&current).Error; err != nil {
			t.Fatal(err)
		}
		if current.Disposition != original.Disposition || current.LatestDecisionID != original.LatestDecisionID || !current.UpdatedAt.Equal(original.UpdatedAt) {
			t.Fatal("relation silently mutated condition")
		}
	}
	// An identical relation choice closes its new run without an empty revision.
	repeat := nextSettlementRun(t, db, base, []uuid.UUID{a.ID, b.ID}, settlementProposalSpec{model.DiagnosisUpdateRelation, &a.ID, map[string]any{"relationChange": map[string]any{"fromConditionId": a.ID, "toConditionId": b.ID, "kind": "coexisting", "basisEvidenceRefs": base.run.InputEvidenceRefs, "rationale": "Explicitly reviewed synthetic relation"}}})
	recordSettlementChoice(t, db, repeat, 0, model.DiagnosisAccept)
	if r, err := NewDiagnosisSettlementService(db).Settle(ctx, base.owner, diagnosisSettleCommand(repeat)); err != nil || r.Outcome != "closed_without_change" || r.Revision != 3 {
		t.Fatalf("empty relation revision %+v %v", r, err)
	}
}

func TestDiagnosisSettlementIdentityAndPatchGuardsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	for _, kind := range []string{"medical_identity", "hidden_disposition", "stale_before", "foreign_target"} {
		t.Run(kind, func(t *testing.T) {
			base, entries := settleFixture(t, db)
			target := entries[0].ID
			content := map[string]any{}
			switch kind {
			case "medical_identity":
				content["proposedEntry"] = map[string]any{"conceptRef": map[string]string{"system": "urn:synthetic:medical-concepts", "code": "entirely-different-concept"}}
			case "hidden_disposition":
				content["proposedEntry"] = map[string]any{"disposition": "inactive"}
			case "stale_before":
				content["changeSet"] = []map[string]any{{"field": "medicalName", "before": "not the current name", "after": entries[0].MedicalName}}
			case "foreign_target":
				_, foreign := settleFixture(t, db)
				target = foreign[0].ID
				content["proposedEntry"] = map[string]any{"medicalName": foreign[0].MedicalName}
			}
			if kind == "foreign_target" {
				f := nextSettlementRun(t, db, base, []uuid.UUID{target})
				raw, err := json.Marshal(content)
				if err != nil {
					t.Fatal(err)
				}
				p := model.DiagnosisProposal{UserID: f.owner, ID: uuid.New(), RunID: f.run.ID, TargetConditionID: &target, ChangeKind: model.DiagnosisUpdate, Content: datatypes.JSON(raw), BaseRevision: f.run.BasedOnDiagnosisRevision, EvidenceRefs: f.run.InputEvidenceRefs, CounterEvidenceRefs: []model.DiagnosisEvidenceRef{}, CreatedAt: time.Now().UTC()}
				p.ContentFingerprint, err = model.DiagnosisProposalContentFingerprint(&p)
				if err != nil {
					t.Fatal(err)
				}
				err = db.Create(&p).Error
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != "23503" || pgerr.ConstraintName != "diagnosis_proposal_target_fk" {
					t.Fatalf("foreign target escaped composite owner FK: %v", err)
				}
				diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 1)
				return
			}
			f := nextSettlementRun(t, db, base, []uuid.UUID{target}, settlementProposalSpec{model.DiagnosisUpdate, &target, content})
			recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
			r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f))
			if err == nil || r != nil {
				t.Fatalf("accepted %s %+v", kind, r)
			}
			diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 1)
			diagnosisDecisionCount(t, db, f.owner, "condition_entries", 2)
			diagnosisDecisionCount(t, db, f.owner, "diagnosis_settlements", 1)
		})
	}
	t.Run("an unchanged named field creates no revision", func(t *testing.T) {
		base, entries := settleFixture(t, db)
		target := entries[0].ID
		f := nextSettlementRun(t, db, base, []uuid.UUID{target}, settlementProposalSpec{model.DiagnosisUpdate, &target, map[string]any{"changeSet": []map[string]any{{"field": "medicalName", "before": entries[0].MedicalName, "after": entries[0].MedicalName}}}})
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f))
		if err != nil || r.Outcome != "closed_without_change" || r.Revision != 1 {
			t.Fatalf("no-op %+v %v", r, err)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 1)
	})
}

func TestDiagnosisSettlementEvidenceRecheckCoveragePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("new run evidence rechecks every decision including rejects", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		a := recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		b := recordSettlementChoice(t, db, f, 1, model.DiagnosisReject)
		added := addSettlementBodyEvidence(t, db, f.owner)
		appendSettlementEvent(t, db, f, "evidence_added", nil, nil, map[string]any{"evidenceRefs": []model.DiagnosisEvidenceRef{added}})
		svc := NewDiagnosisSettlementService(db)
		command := diagnosisSettleCommand(f)
		if _, err := svc.Settle(ctx, f.owner, command); !errors.Is(err, ErrDiagnosisRecheckRequired) {
			t.Fatalf("unreviewed addendum: %v", err)
		}
		appendSettlementEvent(t, db, f, "decision_rechecked", &a.ProposalID, &a.ID, diagnosisRecheckDetails{[]model.DiagnosisEvidenceRef{added}, "unaffected"})
		if _, err := svc.Settle(ctx, f.owner, command); !errors.Is(err, ErrDiagnosisRecheckRequired) {
			t.Fatalf("reject did not require recheck: %v", err)
		}
		appendSettlementEvent(t, db, f, "decision_rechecked", &b.ProposalID, &b.ID, diagnosisRecheckDetails{[]model.DiagnosisEvidenceRef{added}, "unaffected"})
		r, err := svc.Settle(ctx, f.owner, command)
		if err != nil || r.Outcome != "committed" {
			t.Fatalf("reviewed addendum %+v %v", r, err)
		}
		var revision model.DiagnosisRevision
		if err := db.Where("user_id=? AND revision=1", f.owner).Take(&revision).Error; err != nil || len(revision.EvidenceRefs) != 2 {
			t.Fatalf("recheck inputs not frozen: %+v %v", revision, err)
		}
		// The addendum was reviewed as unaffected, not silently adopted into a
		// condition's approved formal evidence links.
		diagnosisDecisionCount(t, db, f.owner, "condition_entry_evidence", 1)
	})
	t.Run("unrelated body revision alone does not invalidate", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		recordSettlementChoice(t, db, f, 1, model.DiagnosisReject)
		addSettlementBodyEvidence(t, db, f.owner)
		if r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); err != nil || r.Outcome != "committed" {
			t.Fatalf("unrelated counter invalidated: %+v %v", r, err)
		}
	})
	t.Run("an affected choice cannot be silently restored", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		d := recordSettlementChoice(t, db, f, 0, model.DiagnosisAccept)
		recordSettlementChoice(t, db, f, 1, model.DiagnosisReject)
		appendSettlementEvent(t, db, f, "decision_rechecked", &d.ProposalID, &d.ID, diagnosisRecheckDetails{d.DecidedEvidenceRefs, "affected"})
		appendSettlementEvent(t, db, f, "decision_rechecked", &d.ProposalID, &d.ID, diagnosisRecheckDetails{d.DecidedEvidenceRefs, "unaffected"})
		if _, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); !errors.Is(err, ErrDiagnosisRecheckRequired) {
			t.Fatalf("affected choice restored: %v", err)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
	})
}

func TestDiagnosisSettlementStaleAndCorruptFingerprintPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("another run advances head and rejects an obsolete batch", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		for i := range f.proposals {
			recordSettlementChoice(t, db, f, i, model.DiagnosisAccept)
		}
		g := nextSettlementRun(t, db, f, []uuid.UUID{}, settlementProposalSpec{model.DiagnosisCreate, nil, map[string]any{"proposedEntry": diagnosisSettlementEntry(f, "Synthetic C syndrome")}})
		// The second proposal's assessment was produced for its new analysis.
		p := g.proposals[0]
		content := diagnosisSettlementEntry(g, "Synthetic C syndrome")
		raw, _ := json.Marshal(map[string]any{"proposedEntry": content})
		p.ID = uuid.New()
		p.SupersedesProposalID = &g.proposals[0].ID
		p.Content = raw
		p.ContentFingerprint, _ = model.DiagnosisProposalContentFingerprint(&p)
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		g.proposals[0] = p
		recordSettlementChoice(t, db, g, 0, model.DiagnosisAccept)
		if _, err := NewDiagnosisSettlementService(db).Settle(ctx, g.owner, diagnosisSettleCommand(g)); err != nil {
			t.Fatal(err)
		}
		if _, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); !errors.Is(err, ErrDiagnosisDecisionConflict) {
			t.Fatalf("stale batch: %v", err)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 1)
		diagnosisDecisionCount(t, db, f.owner, "condition_entries", 1)
	})
	t.Run("a stored fingerprint cannot authorize different raw content", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		old := f.proposals[0]
		p := old
		p.ID = uuid.New()
		p.SupersedesProposalID = &old.ID
		raw, _ := json.Marshal(map[string]any{"proposedEntry": diagnosisSettlementEntry(f, "Different unapproved content")})
		p.Content = raw
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		f.proposals[0] = p
		for i := range f.proposals {
			recordSettlementChoice(t, db, f, i, model.DiagnosisAccept)
		}
		if _, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); !errors.Is(err, ErrDiagnosisDecisionConflict) {
			t.Fatalf("unbound raw content: %v", err)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
	})
}

func TestDiagnosisSettlementTerminalImmutabilityAndReceiptFailurePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("receipt failure rolls back projections events and revision", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		for i := range f.proposals {
			recordSettlementChoice(t, db, f, i, model.DiagnosisAccept)
		}
		callback := "test:dx-receipt-failure:" + uuid.NewString()
		sentinel := errors.New("injected settlement receipt failure")
		if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if r, ok := tx.Statement.Dest.(*model.DiagnosisSettlement); ok && r.UserID == f.owner {
				tx.AddError(sentinel)
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
		if r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(f)); !errors.Is(err, sentinel) || r != nil {
			t.Fatalf("unreceipted commit: %+v %v", r, err)
		}
		assertDiagnosisNotAdopted(t, db, f.owner)
		diagnosisDecisionCount(t, db, f.owner, "condition_analysis_events", 2)
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_settlements", 0)
	})
	t.Run("different concurrent request keys still settle only once", func(t *testing.T) {
		f := seedDiagnosisSettlementFixture(t, db)
		for i := range f.proposals {
			recordSettlementChoice(t, db, f, i, model.DiagnosisAccept)
		}
		svc := NewDiagnosisSettlementService(db)
		var wg sync.WaitGroup
		errs := make(chan error, 6)
		for range 6 {
			c := diagnosisSettleCommand(f)
			wg.Go(func() { _, err := svc.Settle(ctx, f.owner, c); errs <- err })
		}
		wg.Wait()
		close(errs)
		success := 0
		for err := range errs {
			if err == nil {
				success++
			} else if !errors.Is(err, ErrDiagnosisAnalysisClosed) {
				t.Fatal(err)
			}
		}
		if success != 1 {
			t.Fatalf("settled %d times", success)
		}
		diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 1)
		if err := db.Model(&model.DiagnosisSettlement{}).Where("user_id=?", f.owner).Update("tool_call_id", "tampered").Error; err == nil {
			t.Fatal("settlement audit is mutable")
		}
		var terminal int64
		if err := db.Model(&model.ConditionAnalysisEvent{}).Where("user_id=? AND kind='committed'", f.owner).Count(&terminal).Error; err != nil || terminal != 1 {
			t.Fatalf("terminal count %d %v", terminal, err)
		}
	})
}
