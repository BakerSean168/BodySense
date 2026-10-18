package service

import (
	"context"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

type fakeStatementRuntime struct {
	calls  int
	scopes []model.StatementRuntimeScope
	busy   bool
}

func (f *fakeStatementRuntime) EraseStatementRuntimeCopies(_ context.Context, _ uuid.UUID, statementID uuid.UUID, scope model.StatementRuntimeScope) (*model.RuntimeErasureReceipt, error) {
	f.calls++
	f.scopes = append(f.scopes, scope)
	receipt := &model.RuntimeErasureReceipt{Status: "erased", SubjectID: statementID.String()}
	for _, id := range scope.ThreadIDs {
		thread := model.RuntimeThreadReceipt{ThreadID: id.String(), Status: "absent"}
		if f.busy {
			code := "thread_busy"
			thread = model.RuntimeThreadReceipt{ThreadID: id.String(), Status: "outstanding", FailureCode: &code}
			receipt.Status = "outstanding"
		}
		receipt.Threads = append(receipt.Threads, thread)
	}
	return receipt, nil
}

func statementTarget(t *testing.T, receipt *model.StatementErasureReceipt, name model.StatementErasureTargetName) model.StatementErasureTarget {
	t.Helper()
	for _, target := range receipt.Targets {
		if target.Target == name {
			return target
		}
	}
	t.Fatalf("target %s missing", name)
	return model.StatementErasureTarget{}
}

func wantStatementTarget(t *testing.T, receipt *model.StatementErasureReceipt, name model.StatementErasureTargetName, state model.AssetErasureTargetState, code string) {
	t.Helper()
	target := statementTarget(t, receipt, name)
	got := ""
	if target.FailureCode != nil {
		got = *target.FailureCode
	}
	if target.State != state || got != code {
		t.Fatalf("%s: state=%s code=%q want %s %q", name, target.State, got, state, code)
	}
}

// An active plan resting on an adopted condition that cites the statement
// shows up as a plan review and blocks; the plan and Diagnosis are never
// changed. A busy conversation keeps the runtime target retrying. Once the
// user withdrew the fact and the condition, a repeated command finishes:
// erased_with_retention (the frozen decision trail stays), and a further
// repeat is a no-op on the terminal receipt.
func TestBodyFactStatementErasurePlanReviewPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	owner := seedOnboardingContextUser(t, db)
	condition, statement := seedPlanDiagnosis(t, db, owner)
	plans := NewIndependentTrainingPlanService(db)
	draft := independentDraft("Rehab")
	draft.RelatedConditionIDs = []uuid.UUID{condition}
	accepted, err := plans.Accept(ctx, owner, independentDecision(independentProposal(t, plans, owner, draft, uuid.Nil, uuid.Nil)))
	if err != nil {
		t.Fatal(err)
	}
	planBefore := independentPlanSnapshot(t, db, owner, accepted.PlanID)
	runtime := &fakeStatementRuntime{busy: true}
	svc := NewStatementErasureService(repository.NewStatementErasureRepository(db)).WithRuntimeEraser(runtime)

	receipt, err := svc.Request(ctx, owner, statement, "erase-statement")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Request.Status != model.StatementErasureOutstanding {
		t.Fatalf("status=%s", receipt.Request.Status)
	}
	wantStatementTarget(t, receipt, model.StatementErasureDiagnosisRecords, model.AssetErasureTargetBlocked, model.AssetErasureCodeDiagnosisReview)
	wantStatementTarget(t, receipt, model.StatementErasureTrainingPlans, model.AssetErasureTargetBlocked, model.ErasureCodePlanReview)
	wantStatementTarget(t, receipt, model.StatementErasureBodyRecords, model.AssetErasureTargetBlocked, model.ErasureCodeBodyRecordActive)
	wantStatementTarget(t, receipt, model.StatementErasureRuntimeCheckpoints, model.AssetErasureTargetFailed, model.AssetErasureCodeRuntimeThreadBusy)
	wantStatementTarget(t, receipt, model.StatementErasureManifest, model.AssetErasureTargetBlocked, model.AssetErasureCodeTargetsOutstanding)
	subjects := map[model.ErasureReviewSubject]model.ErasureReview{}
	for _, review := range receipt.Reviews {
		subjects[review.Subject] = review
	}
	plan := subjects[model.ErasureReviewTrainingPlan]
	if subjects[model.ErasureReviewDiagnosisCondition].SubjectID != condition || plan.SubjectID != accepted.PlanID ||
		plan.RevisionID == nil || *plan.RevisionID != accepted.RevisionID || subjects[model.ErasureReviewBodyFact].SubjectID == uuid.Nil {
		t.Fatalf("reviews=%+v", receipt.Reviews)
	}
	if runtime.calls != 1 || len(runtime.scopes[0].RecordIDs) != 1 || len(runtime.scopes[0].ThreadIDs) != 1 {
		t.Fatalf("runtime calls=%d scopes=%+v", runtime.calls, runtime.scopes)
	}
	var available string
	if err := db.Raw(`SELECT availability FROM statement_evidence WHERE id=?`, statement).Scan(&available).Error; err != nil || available != "available" {
		t.Fatalf("statement changed early: %q %v", available, err)
	}

	// The user's review: withdraw the derived fact and the condition.
	if err := db.Exec(`UPDATE body_facts SET assertion_status='withdrawn' WHERE user_id=? AND source_evidence_id=?`, owner, statement).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE condition_entries SET disposition='inactive' WHERE user_id=? AND id=?`, owner, condition).Error; err != nil {
		t.Fatal(err)
	}
	runtime.busy = false
	receipt, err = svc.Request(ctx, owner, statement, "erase-statement")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Request.Status != model.StatementErasureErasedWithRetention || len(receipt.Reviews) != 0 {
		t.Fatalf("receipt=%+v", receipt)
	}
	wantStatementTarget(t, receipt, model.StatementErasureDecisionRecords, model.ErasureTargetRetainedByPolicy, "")
	for _, name := range []model.StatementErasureTargetName{model.StatementErasureDiagnosisRecords, model.StatementErasureTrainingPlans,
		model.StatementErasureBodyRecords, model.StatementErasureAssessmentReports, model.StatementErasureRuntimeCheckpoints, model.StatementErasureManifest} {
		wantStatementTarget(t, receipt, name, model.AssetErasureTargetVerified, "")
	}
	if got := independentPlanSnapshot(t, db, owner, accepted.PlanID); got != planBefore {
		t.Fatal("statement erasure changed the plan")
	}
	calls := runtime.calls
	again, err := svc.Request(ctx, owner, statement, "other-key")
	if err != nil || again.Request.ID != receipt.Request.ID || again.Request.Status != model.StatementErasureErasedWithRetention || runtime.calls != calls {
		t.Fatalf("repeat after terminal: %+v %v calls=%d", again, err, runtime.calls)
	}
	if n, err := svc.Recover(ctx, 10); err != nil || n != 0 {
		t.Fatalf("recover terminal: %d %v", n, err)
	}
}
