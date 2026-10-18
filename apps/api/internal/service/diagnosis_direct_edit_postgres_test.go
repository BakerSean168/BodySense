package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func directEditCommand(f diagnosisDecisionFixture, condition uuid.UUID, kind model.ConditionDirectEditKind, expected int64) DiagnosisDirectEditCommand {
	return DiagnosisDirectEditCommand{ConditionID: condition, EditKind: kind, RequestEvidenceID: f.request.ID, ExpectedRevision: expected,
		Reason: "User confirmed this condition no longer belongs in the diagnosis", ToolCallID: "user-confirm-direct-edit", IdempotencyKey: uuid.NewString()}
}
func directEditEntry(t *testing.T, db *gorm.DB, owner, id uuid.UUID) model.ConditionEntry {
	t.Helper()
	var entry model.ConditionEntry
	if err := db.Where("user_id=? AND id=?", owner, id).Take(&entry).Error; err != nil {
		t.Fatal(err)
	}
	return entry
}
func directEditHead(t *testing.T, db *gorm.DB, owner uuid.UUID) int64 {
	t.Helper()
	var head model.Diagnosis
	if err := db.Where("user_id=?", owner).Take(&head).Error; err != nil {
		t.Fatal(err)
	}
	return head.CurrentRevision
}

func TestDiagnosisDirectEditWithdrawPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f, entries := settleFixture(t, db)
	a, b := entries[0], entries[1]
	// A formal run opened on the pre-edit head, with a recorded user choice.
	stale := nextSettlementRun(t, db, f, []uuid.UUID{a.ID},
		settlementProposalSpec{model.DiagnosisUpdate, &a.ID, map[string]any{"proposedEntry": map[string]any{"causeAnalysis": map[string]string{"summary": "Old run explanation", "rationale": "No causal certainty claimed"}}}})
	recordSettlementChoice(t, db, stale, 0, model.DiagnosisAccept)

	svc := NewDiagnosisDirectEditService(db)
	cmd := directEditCommand(f, a.ID, model.ConditionWithdraw, 1)
	edit, err := svc.Apply(ctx, f.owner, cmd)
	if err != nil || edit == nil || edit.ResultingDiagnosisRevision != 2 || edit.BasedOnDiagnosisRevision != 1 || edit.ReceiptID == uuid.Nil ||
		edit.RequestMessageID != f.run.RequestMessageID || len(edit.Changes) != 1 || edit.Changes[0] != (model.ConditionDirectEditChange{Field: "disposition", Before: "adopted", After: "inactive"}) {
		t.Fatalf("withdraw %+v %v", edit, err)
	}
	withdrawn := directEditEntry(t, db, f.owner, a.ID)
	if withdrawn.Disposition != model.ConditionInactive || withdrawn.CommittedRevision != 2 || withdrawn.MedicalName != a.MedicalName ||
		withdrawn.LatestDecisionID != a.LatestDecisionID || withdrawn.Assessment == nil || withdrawn.CauseAnalysis != a.CauseAnalysis {
		t.Fatalf("withdraw changed more than disposition: %+v", withdrawn)
	}
	if other := directEditEntry(t, db, f.owner, b.ID); other.Disposition != model.ConditionAdopted || other.CommittedRevision != 1 {
		t.Fatalf("unrelated entry changed: %+v", other)
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_entries", 2)
	diagnosisDecisionCount(t, db, f.owner, "condition_entry_evidence", 2)
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 2)
	diagnosisDecisionCount(t, db, f.owner, "condition_direct_edits", 1)
	var history model.ConditionDispositionChange
	if err := db.Where("user_id=? AND condition_id=?", f.owner, a.ID).Take(&history).Error; err != nil ||
		history.DecisionID != nil || history.DirectEditID == nil || *history.DirectEditID != edit.ID || history.Reason != "user_withdrawal" || history.CommittedRevision != 2 {
		t.Fatalf("withdraw history %+v %v", history, err)
	}
	var revision model.DiagnosisRevision
	if err := db.Where("user_id=? AND revision=2", f.owner).Take(&revision).Error; err != nil ||
		revision.ID != edit.DiagnosisRevisionID || revision.RunID != nil || revision.DirectEditID == nil || *revision.DirectEditID != edit.ID || revision.ReceiptID != edit.ReceiptID {
		t.Fatalf("direct edit revision %+v %v", revision, err)
	}

	// Lost response: exact retry returns the original receipt, even after the source changes.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.request.ID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	replay, err := svc.Apply(ctx, f.owner, cmd)
	if err != nil || replay.ID != edit.ID || !replay.ConfirmedAt.Equal(edit.ConfirmedAt) {
		t.Fatalf("replay %+v %v", replay, err)
	}
	changed := cmd
	changed.Reason = "A different confirmed reason"
	if _, err := svc.Apply(ctx, f.owner, changed); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("changed retry body: %v", err)
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.request.ID).Update("availability", "available").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(ctx, f.owner, directEditCommand(f, a.ID, model.ConditionWithdraw, 2)); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("withdrawn twice: %v", err)
	}

	// The older run's decision cannot overwrite the direct edit.
	if _, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(stale)); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("stale run settled over direct edit: %v", err)
	}
	if after := directEditEntry(t, db, f.owner, a.ID); after.Disposition != model.ConditionInactive || after.CauseAnalysis != a.CauseAnalysis || directEditHead(t, db, f.owner) != 2 {
		t.Fatalf("stale run mutated entry: %+v", after)
	}

	// Re-adoption is not an undo channel: it needs a new formal run decision.
	readopt := nextSettlementRun(t, db, f, []uuid.UUID{a.ID}, settlementProposalSpec{model.DiagnosisSetDisposition, &a.ID, map[string]any{"newDisposition": "adopted", "dispositionReason": "readopted"}})
	recordSettlementChoice(t, db, readopt, 0, model.DiagnosisAccept)
	if r, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, diagnosisSettleCommand(readopt)); err != nil || r.Revision != 3 {
		t.Fatalf("readopt after withdraw %+v %v", r, err)
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_disposition_changes", 2)
}

func TestDiagnosisDirectEditCorrectMetadataPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f, entries := settleFixture(t, db)
	a := entries[0]
	svc := NewDiagnosisDirectEditService(db)
	notes := "Diagnosed by my physiotherapist last spring"
	cmd := directEditCommand(f, a.ID, model.ConditionCorrectMetadata, 1)
	cmd.Notes = &notes
	cmd.Reason = "User corrected the display note"
	edit, err := svc.Apply(ctx, f.owner, cmd)
	if err != nil || edit.ResultingDiagnosisRevision != 2 || edit.Changes[0] != (model.ConditionDirectEditChange{Field: "qualifiers.notes", Before: "", After: notes}) {
		t.Fatalf("correct metadata %+v %v", edit, err)
	}
	corrected := directEditEntry(t, db, f.owner, a.ID)
	if corrected.Qualifiers == nil || corrected.Qualifiers.Notes != notes || corrected.Disposition != model.ConditionAdopted ||
		corrected.MedicalName != a.MedicalName || corrected.HandlingPlan.Summary != a.HandlingPlan.Summary || corrected.CommittedRevision != 2 {
		t.Fatalf("metadata correction changed substance: %+v", corrected)
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_disposition_changes", 0)
	same := cmd
	same.IdempotencyKey = uuid.NewString()
	same.ExpectedRevision = 2
	if _, err := svc.Apply(ctx, f.owner, same); !errors.Is(err, ErrDiagnosisDirectEditInvalid) {
		t.Fatalf("no-op correction created a revision: %v", err)
	}
	cleared := ""
	clear := same
	clear.Notes = &cleared
	if e, err := svc.Apply(ctx, f.owner, clear); err != nil || e.Changes[0].Before != notes || e.Changes[0].After != "" {
		t.Fatalf("clear note %+v %v", e, err)
	}
	if entry := directEditEntry(t, db, f.owner, a.ID); entry.Qualifiers != nil {
		t.Fatalf("cleared note left qualifiers: %+v", entry.Qualifiers)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_revisions", 3)
}

func TestDiagnosisDirectEditGuardsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f, entries := settleFixture(t, db)
	other, otherEntries := settleFixture(t, db)
	a := entries[0]
	svc := NewDiagnosisDirectEditService(db)
	assertUnchanged := func(t *testing.T) {
		t.Helper()
		if directEditHead(t, db, f.owner) != 1 || directEditEntry(t, db, f.owner, a.ID).Disposition != model.ConditionAdopted {
			t.Fatal("rejected direct edit changed diagnosis")
		}
		diagnosisDecisionCount(t, db, f.owner, "condition_direct_edits", 0)
	}
	notes := "note"
	cases := []struct {
		name string
		mut  func(*DiagnosisDirectEditCommand)
		want error
	}{
		{"stale revision", func(c *DiagnosisDirectEditCommand) { c.ExpectedRevision = 0 }, ErrDiagnosisDecisionConflict},
		{"future revision", func(c *DiagnosisDirectEditCommand) { c.ExpectedRevision = 2 }, ErrDiagnosisDecisionConflict},
		{"foreign condition", func(c *DiagnosisDirectEditCommand) { c.ConditionID = otherEntries[0].ID }, model.ErrDiagnosisNotFound},
		{"unknown condition", func(c *DiagnosisDirectEditCommand) { c.ConditionID = uuid.New() }, model.ErrDiagnosisNotFound},
		{"foreign request", func(c *DiagnosisDirectEditCommand) { c.RequestEvidenceID = other.request.ID }, model.ErrDiagnosisNotFound},
		{"withdraw with notes", func(c *DiagnosisDirectEditCommand) { c.Notes = &notes }, ErrDiagnosisDirectEditInvalid},
		{"blank reason", func(c *DiagnosisDirectEditCommand) { c.Reason = " " }, ErrDiagnosisDirectEditInvalid},
		{"unknown kind", func(c *DiagnosisDirectEditCommand) { c.EditKind = "mark_resolved" }, ErrDiagnosisDirectEditInvalid},
		{"correction without notes", func(c *DiagnosisDirectEditCommand) { c.EditKind = model.ConditionCorrectMetadata }, ErrDiagnosisDirectEditInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := directEditCommand(f, a.ID, model.ConditionWithdraw, 1)
			tc.mut(&c)
			if _, err := svc.Apply(ctx, f.owner, c); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			assertUnchanged(t)
		})
	}
	t.Run("unavailable request source", func(t *testing.T) {
		if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.request.ID).Update("availability", "blocked").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", f.owner, f.request.ID).Update("availability", "available")
		})
		if _, err := svc.Apply(ctx, f.owner, directEditCommand(f, a.ID, model.ConditionWithdraw, 1)); !errors.Is(err, ErrDiagnosisEvidenceUnavailable) {
			t.Fatalf("blocked request authorized edit: %v", err)
		}
		assertUnchanged(t)
	})
	t.Run("settlement key namespace", func(t *testing.T) {
		var settled model.DiagnosisSettlement
		if err := db.Where("user_id=?", f.owner).Take(&settled).Error; err != nil {
			t.Fatal(err)
		}
		c := directEditCommand(f, a.ID, model.ConditionWithdraw, 1)
		c.IdempotencyKey = settled.IdempotencyKey
		if _, err := svc.Apply(ctx, f.owner, c); !errors.Is(err, ErrDiagnosisDecisionConflict) {
			t.Fatalf("reused settlement key: %v", err)
		}
		assertUnchanged(t)
	})
	t.Run("urgent safety changes nothing", func(t *testing.T) {
		urgent := NewDiagnosisDirectEditService(db)
		urgent.projectSafety = func(snapshot *BodyStateReadModel) (SafetyEnvelopeV2, error) {
			e, err := ProjectSafetyEnvelopeV2(snapshot)
			e.ActiveBlockers = []SafetyBlockerV1{{RiskLevel: "urgent"}}
			return e, err
		}
		if _, err := urgent.Apply(ctx, f.owner, directEditCommand(f, a.ID, model.ConditionWithdraw, 1)); !errors.Is(err, ErrDiagnosisSafetyBlocked) {
			t.Fatalf("urgent edit: %v", err)
		}
		assertUnchanged(t)
	})
	t.Run("incomplete safety coverage fails closed", func(t *testing.T) {
		uncovered := NewDiagnosisDirectEditService(db)
		uncovered.projectSafety = func(snapshot *BodyStateReadModel) (SafetyEnvelopeV2, error) {
			e, err := ProjectSafetyEnvelopeV2(snapshot)
			e.Coverage.Complete = false
			return e, err
		}
		if _, err := uncovered.Apply(ctx, f.owner, directEditCommand(f, a.ID, model.ConditionWithdraw, 1)); !errors.Is(err, ErrDiagnosisSafetyUnavailable) {
			t.Fatalf("uncovered edit: %v", err)
		}
		assertUnchanged(t)
	})
	t.Run("concurrent retries create one revision", func(t *testing.T) {
		c := directEditCommand(f, a.ID, model.ConditionWithdraw, 1)
		var wg sync.WaitGroup
		ids := make(chan uuid.UUID, 6)
		errs := make(chan error, 6)
		for range 6 {
			wg.Go(func() {
				e, err := svc.Apply(ctx, f.owner, c)
				errs <- err
				if e != nil {
					ids <- e.ID
				}
			})
		}
		wg.Wait()
		close(ids)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		seen := map[uuid.UUID]bool{}
		for id := range ids {
			seen[id] = true
		}
		if len(seen) != 1 || directEditHead(t, db, f.owner) != 2 {
			t.Fatalf("concurrent edits %v", seen)
		}
		diagnosisDecisionCount(t, db, f.owner, "condition_direct_edits", 1)
	})
	t.Run("receipt is immutable", func(t *testing.T) {
		if err := db.Exec("UPDATE condition_direct_edits SET reason='rewritten' WHERE user_id=?", f.owner).Error; err == nil {
			t.Fatal("direct edit receipt was mutated")
		}
		if err := db.Exec("UPDATE condition_disposition_changes SET decision_id=NULL, direct_edit_id=NULL WHERE user_id=?", f.owner).Error; err == nil {
			t.Fatal("disposition history lost its authority")
		}
	})
}
