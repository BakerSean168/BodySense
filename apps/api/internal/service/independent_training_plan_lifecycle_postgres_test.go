package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

func acceptIndependentPlan(t *testing.T, svc *IndependentTrainingPlanService, owner uuid.UUID, title string) *PlanAcceptance {
	t.Helper()
	p := independentProposal(t, svc, owner, independentDraft(title), uuid.Nil, uuid.Nil)
	a, err := svc.Accept(context.Background(), owner, independentDecision(p))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func planStatusCmd(plan uuid.UUID, status trainingplan.PlanStatus, expected int64) PlanStatusCommand {
	return PlanStatusCommand{PlanID: plan, Status: status, ExpectedStatusRevision: expected, IdempotencyKey: uuid.NewString()}
}

// User-managed lifecycle: explicit transitions only, expected status revision,
// idempotent retry, owner isolation, and plan A never touches plan B.
func TestIndependentTrainingStatusLifecyclePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	foreign := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	a := acceptIndependentPlan(t, svc, owner, "A")
	b := acceptIndependentPlan(t, svc, owner, "B")
	beforeB := independentPlanSnapshot(t, db, owner, b.PlanID)

	pause := planStatusCmd(a.PlanID, trainingplan.PlanPaused, 0)
	event, err := svc.ChangeStatus(ctx, owner, pause)
	if err != nil || event.FromStatus != "active" || event.ToStatus != "paused" || event.StatusRevision != 1 {
		t.Fatalf("pause: %+v %v", event, err)
	}
	// Exact retry returns the original event; same key with another payload conflicts.
	replay, err := svc.ChangeStatus(ctx, owner, pause)
	if err != nil || replay.ID != event.ID {
		t.Fatalf("status retry: %+v %v", replay, err)
	}
	changed := pause
	changed.Status = trainingplan.PlanCompleted
	if _, err = svc.ChangeStatus(ctx, owner, changed); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("status key reuse: %v", err)
	}
	// Stale expected status revision is a conflict and writes nothing.
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanActive, 0)); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("stale expected status revision: %v", err)
	}
	// Not a transition: paused->paused; completed->paused below.
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanPaused, 1)); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("same-status transition: %v", err)
	}
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, "review_recommended", 1)); !errors.Is(err, ErrPlanInvalid) {
		t.Fatalf("review_recommended is not a status: %v", err)
	}
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanCompleted, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanPaused, 2)); !errors.Is(err, ErrPlanConflict) {
		t.Fatalf("completed->paused: %v", err)
	}
	reopened, err := svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanActive, 2))
	if err != nil || reopened.StatusRevision != 3 || reopened.PlanID != a.PlanID {
		t.Fatalf("reopen same planId: %+v %v", reopened, err)
	}
	view, err := svc.View(ctx, owner, a.PlanID)
	if err != nil || view.Plan.Status != "active" || view.Plan.StatusRevision != 3 || view.Revision.ID != a.RevisionID {
		t.Fatalf("status changes must not create content revisions: %+v %v", view, err)
	}
	if got := independentPlanSnapshot(t, db, owner, b.PlanID); got != beforeB {
		t.Fatal("status of plan A changed plan B")
	}
	// Owner isolation: another owner's ids are indistinguishable from missing.
	if _, err = svc.ChangeStatus(ctx, foreign, planStatusCmd(a.PlanID, trainingplan.PlanPaused, 3)); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("foreign status change: %v", err)
	}
	if _, err = svc.View(ctx, foreign, a.PlanID); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("foreign view: %v", err)
	}
	if _, err = svc.GetProposal(ctx, foreign, a.Receipt.ProposalID); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("foreign proposal: %v", err)
	}
	if rows, err := svc.ListProposals(ctx, foreign, ""); err != nil || len(rows) != 0 {
		t.Fatalf("foreign proposal list leaked: %d %v", len(rows), err)
	}
	if views, err := svc.ListViews(ctx, foreign); err != nil || len(views) != 0 {
		t.Fatalf("foreign plan list leaked: %d %v", len(views), err)
	}
	// Events are immutable and owner/plan scoped by composite FK.
	if err = db.Exec("UPDATE training_plan_status_events SET to_status='completed' WHERE user_id=?", owner).Error; err == nil {
		t.Fatal("status event mutated")
	}
	if err = db.Exec(`INSERT INTO training_plan_status_events(id,user_id,plan_id,from_status,to_status,status_revision,source_kind,idempotency_key,request_fingerprint,created_at)
 VALUES (?,?,?,'active','paused',9,'user_command','x',repeat('a',64),now())`, uuid.New(), foreign, a.PlanID).Error; err == nil {
		t.Fatal("cross-owner status event accepted")
	}
	var events int64
	if err = db.Table("training_plan_status_events").Where("user_id=? AND plan_id=?", owner, a.PlanID).Count(&events).Error; err != nil || events != 3 {
		t.Fatalf("one audit event per transition: %d %v", events, err)
	}
	// Proposal listing: state filter and TTL projection on read.
	pending := independentProposal(t, svc, owner, independentDraft("C"), uuid.Nil, uuid.Nil)
	rows, err := svc.ListProposals(ctx, owner, trainingplan.ProposalPending)
	if err != nil || len(rows) != 1 || rows[0].ID != pending.ID {
		t.Fatalf("pending list: %+v %v", rows, err)
	}
	if _, err = svc.ListProposals(ctx, owner, "bogus"); !errors.Is(err, ErrPlanInvalid) {
		t.Fatalf("invalid state filter: %v", err)
	}
	svc.now = func() time.Time { return pending.ExpiresAt }
	got, err := svc.GetProposal(ctx, owner, pending.ID)
	if err != nil || got.State != trainingplan.ProposalExpired || got.ExpiredReason == nil || *got.ExpiredReason != "timeout" {
		t.Fatalf("expired projection: %+v %v", got, err)
	}
}

// Concurrent same-owner commands serialize on users FOR NO KEY UPDATE: exactly
// one status change wins per expected revision, while independent plans all
// stay active and unaffected.
func TestIndependentTrainingConcurrentActivationPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	a := acceptIndependentPlan(t, svc, owner, "A")
	b := acceptIndependentPlan(t, svc, owner, "B")
	for _, plan := range []uuid.UUID{a.PlanID, b.PlanID} {
		if _, err := svc.ChangeStatus(ctx, owner, planStatusCmd(plan, trainingplan.PlanPaused, 0)); err != nil {
			t.Fatal(err)
		}
	}
	const racers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanActive, 1))
		}(i)
	}
	// In parallel: reactivate B and accept a brand-new plan C.
	var bErr, cErr error
	var c *PlanAcceptance
	cProposal := independentProposal(t, svc, owner, independentDraft("C"), uuid.Nil, uuid.Nil)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, bErr = svc.ChangeStatus(ctx, owner, planStatusCmd(b.PlanID, trainingplan.PlanActive, 1))
	}()
	go func() {
		defer wg.Done()
		<-start
		c, cErr = svc.Accept(ctx, owner, independentDecision(cProposal))
	}()
	close(start)
	wg.Wait()
	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case !errors.Is(err, ErrPlanConflict):
			t.Fatalf("racer error: %v", err)
		}
	}
	if wins != 1 || bErr != nil || cErr != nil {
		t.Fatalf("wins=%d b=%v c=%v", wins, bErr, cErr)
	}
	views, err := svc.ListViews(ctx, owner)
	if err != nil || len(views) != 3 {
		t.Fatalf("views: %d %v", len(views), err)
	}
	for _, v := range views {
		if v.Plan.Status != "active" {
			t.Fatalf("multi-active lost: %s %s", v.Plan.Title, v.Plan.Status)
		}
		want := int64(2)
		if v.Plan.ID == c.PlanID {
			want = 0
		}
		if v.Plan.StatusRevision != want {
			t.Fatalf("plan %s status revision %d want %d", v.Plan.Title, v.Plan.StatusRevision, want)
		}
	}
	var events int64
	if err = db.Table("training_plan_status_events").Where("user_id=? AND plan_id=?", owner, a.PlanID).Count(&events).Error; err != nil || events != 2 {
		t.Fatalf("lost-update or double write on A: %d %v", events, err)
	}
}

// Safety and BodyState changes never write a plan: they only surface a
// read-only review hint and a fail-closed execution gate. A user status
// change never grants execution either.
func TestIndependentTrainingNoAutoModificationAndSafetyPausePostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewIndependentTrainingPlanService(db)
	ctx := context.Background()
	a := acceptIndependentPlan(t, svc, owner, "A")
	view, err := svc.View(ctx, owner, a.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Review.Recommended {
		t.Fatalf("fresh plan flagged for review: %+v", view.Review)
	}
	// Never-initialized Safety (revision 0) is unavailable, never allow.
	if view.Gate.Permits() || view.Gate.Result != safety.ResultUnavailable {
		t.Fatalf("missing safety permitted execution: %+v", view.Gate)
	}
	before := independentPlanSnapshot(t, db, owner, a.PlanID)
	repo := repository.NewBodyStateRepository(db)
	_, err = repo.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic new shoulder pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	view, err = svc.View(ctx, owner, a.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Review.Recommended || view.Review.Reason != PlanReviewContextChanged {
		t.Fatalf("context change must recommend review: %+v", view.Review)
	}
	// An unscreened symptom keeps execution closed (Safety pause), fail-closed.
	if view.Gate.Permits() {
		t.Fatalf("unscreened symptom permitted execution: %+v", view.Gate)
	}
	if got := independentPlanSnapshot(t, db, owner, a.PlanID); got != before {
		t.Fatal("body/safety change auto-modified the plan")
	}
	if view.Plan.Status != "active" || view.Plan.StatusRevision != 0 {
		t.Fatalf("safety wrote plan status: %+v", view.Plan)
	}
	// User pause/resume is allowed under a Safety block and still cannot open execution.
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanPaused, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanActive, 1)); err != nil {
		t.Fatal(err)
	}
	view, err = svc.View(ctx, owner, a.PlanID)
	if err != nil || view.Gate.Permits() || view.Revision.ID != a.RevisionID {
		t.Fatalf("status change granted execution or changed content: %+v %v", view, err)
	}
	// Reading many times is side-effect free on plan rows.
	after := independentPlanSnapshot(t, db, owner, a.PlanID)
	for i := 0; i < 3; i++ {
		if _, err = svc.ListViews(ctx, owner); err != nil {
			t.Fatal(err)
		}
	}
	if got := independentPlanSnapshot(t, db, owner, a.PlanID); got != after {
		t.Fatal("read projection wrote the plan")
	}
	// Only a user-accepted proposal changes content: a stale update expires.
	stale := independentProposal(t, svc, owner, independentDraft("A2"), a.PlanID, a.RevisionID)
	if _, err = svc.ChangeStatus(ctx, owner, planStatusCmd(a.PlanID, trainingplan.PlanPaused, 2)); err != nil {
		t.Fatal(err)
	}
	_, err = repo.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 1, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic knee pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Accept(ctx, owner, independentDecision(stale)); err == nil {
		t.Fatal("stale proposal accepted after context change")
	}
	view, err = svc.View(ctx, owner, a.PlanID)
	if err != nil || view.Revision.ID != a.RevisionID || view.Plan.Status != "paused" {
		t.Fatalf("stale proposal modified the plan: %+v %v", view, err)
	}
}
