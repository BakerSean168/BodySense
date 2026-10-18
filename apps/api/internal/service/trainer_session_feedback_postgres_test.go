package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// newReviewedTrainerFixture accepts the plan after Safety is screened, so the
// review projection starts with an unchanged context.
func newReviewedTrainerFixture(t *testing.T) *trainerFixture {
	t.Helper()
	db := openOnboardingContextIntegrationDB(t)
	f := &trainerFixture{db: db, svc: NewTrainerSessionService(db), plans: NewIndependentTrainingPlanService(db), owner: seedOnboardingContextUser(t, db)}
	seedTrainerSafety(t, db, f.owner)
	f.plan = acceptIndependentPlan(t, f.plans, f.owner, "Feedback")
	view, err := f.plans.View(context.Background(), f.owner, f.plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	f.template = view.Draft.Content.Sessions[0].TemplateID
	f.exercise = view.Draft.Content.Sessions[0].Stages[0].Exercises[0].PlanExerciseID
	return f
}

func feedback(session uuid.UUID, text, key string) TrainerFeedbackCommand {
	return TrainerFeedbackCommand{SessionID: session, Text: text, IdempotencyKey: key}
}

// Feedback is the user's verbatim words on an ended session, stored once. It
// is never gated and only surfaces as the read-only review hint of the plan
// revision the session executed: plan status, content and revisions, the
// session and its receipts, and BodyState are untouched.
func TestTrainerSessionFeedbackPostgres(t *testing.T) {
	f := newReviewedTrainerFixture(t)
	ctx := context.Background()
	before, err := f.plans.View(ctx, f.owner, f.plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Review.Recommended || before.Review.SessionFeedbackCount == nil || *before.Review.SessionFeedbackCount != 0 {
		t.Fatalf("baseline review: %+v", before.Review)
	}
	started, err := f.svc.Start(ctx, f.owner, f.start("start"))
	if err != nil {
		t.Fatal(err)
	}
	sid := started.Session.ID
	if _, err = f.svc.RecordFeedback(ctx, f.owner, feedback(sid, "too early", "early")); !errors.Is(err, ErrTrainerSessionNotEnded) {
		t.Fatalf("feedback on a live session: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(sid, 1, TrainerSetPerformed, 0)); err != nil {
		t.Fatal(err)
	}
	ended, err := f.svc.Finish(ctx, f.owner, FinishTrainerSessionCommand{SessionID: sid, EndedReason: TrainerEndedFinished, ExpectedSessionRevision: 1, IdempotencyKey: "finish"})
	if err != nil {
		t.Fatal(err)
	}
	if ended.Feedback != nil {
		t.Fatalf("feedback before any was given: %+v", ended.Feedback)
	}
	body := repository.NewBodyStateRepository(f.db)
	var head model.BodyState
	if err = f.db.Where("user_id=?", f.owner).Take(&head).Error; err != nil {
		t.Fatal(err)
	}

	text := "  膝盖有点酸，\n但比上周轻松。  "
	cmd := feedback(sid, text, "fb-1")
	got, err := f.svc.RecordFeedback(ctx, f.owner, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got.Feedback == nil || got.Feedback.Text != text || got.Feedback.PlanRevisionID != f.plan.RevisionID || got.Session.Revision != ended.Session.Revision || len(got.Actions) != 1 {
		t.Fatalf("feedback view: %+v", got)
	}
	replay, err := f.svc.RecordFeedback(ctx, f.owner, cmd)
	if err != nil || replay.Feedback.ID != got.Feedback.ID {
		t.Fatalf("feedback replay: %+v %v", replay, err)
	}
	if _, err = f.svc.RecordFeedback(ctx, f.owner, feedback(sid, "different words", "fb-1")); !errors.Is(err, ErrTrainerConflict) {
		t.Fatalf("feedback key reuse: %v", err)
	}
	if _, err = f.svc.RecordFeedback(ctx, f.owner, feedback(sid, "second thoughts", "fb-2")); !errors.Is(err, ErrTrainerFeedbackRecorded) {
		t.Fatalf("second feedback: %v", err)
	}
	for _, bad := range []TrainerFeedbackCommand{feedback(sid, "", "b1"), feedback(sid, " \n\t", "b2"), feedback(sid, strings.Repeat("字", 2001), "b3"), feedback(sid, "nul\x00", "b4"), feedback(sid, "\xff", "b5"), feedback(uuid.Nil, "x", "b6"), feedback(sid, "x", " ")} {
		if _, err = f.svc.RecordFeedback(ctx, f.owner, bad); !errors.Is(err, ErrTrainerInvalid) {
			t.Fatalf("invalid feedback accepted: %q %v", bad.Text, err)
		}
	}
	if _, err = f.svc.RecordFeedback(ctx, f.owner, feedback(sid, strings.Repeat("字", 2000), "max")); !errors.Is(err, ErrTrainerFeedbackRecorded) {
		t.Fatalf("2000 characters must be valid: %v", err)
	}
	// Owner isolation: a foreign session is not found.
	other := seedOnboardingContextUser(t, f.db)
	if _, err = f.svc.RecordFeedback(ctx, other, feedback(sid, "not mine", "foreign")); !errors.Is(err, ErrTrainerNotFound) {
		t.Fatalf("foreign session feedback: %v", err)
	}
	if f.count(t, "training_session_feedback") != 1 {
		t.Fatal("refused feedback wrote a row")
	}
	if f.count(t, "training_set_actions") != 1 || f.count(t, "training_session_transitions") != 0 {
		t.Fatal("feedback wrote a receipt")
	}

	// The review hint is the only place feedback surfaces; the plan is unchanged.
	after, err := f.plans.View(ctx, f.owner, f.plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Review.Recommended || after.Review.Reason != PlanReviewSessionFeedback || after.Review.SessionFeedbackCount == nil || *after.Review.SessionFeedbackCount != 1 {
		t.Fatalf("review after feedback: %+v", after.Review)
	}
	if after.Plan.Status != before.Plan.Status || after.Plan.StatusRevision != before.Plan.StatusRevision ||
		*after.Plan.CurrentAcceptedRevisionID != *before.Plan.CurrentAcceptedRevisionID || after.Revision.ID != before.Revision.ID {
		t.Fatalf("feedback changed the plan: %+v -> %+v", before.Plan, after.Plan)
	}
	var planRevisions int64
	if err = f.db.Table("training_plan_revisions").Where("user_id=?", f.owner).Count(&planRevisions).Error; err != nil {
		t.Fatal(err)
	}
	if planRevisions != 1 || f.count(t, "training_plan_status_events") != 0 {
		t.Fatal("feedback wrote a plan revision or status event")
	}
	var headAfter model.BodyState
	if err = f.db.Where("user_id=?", f.owner).Take(&headAfter).Error; err != nil || headAfter.CurrentRevision != head.CurrentRevision {
		t.Fatalf("feedback changed BodyState: %d -> %d %v", head.CurrentRevision, headAfter.CurrentRevision, err)
	}
	// Feedback only counts for the revision the session executed.
	sessions := repository.NewTrainerSessionRepository(f.db)
	if n, err := sessions.PlanRevisionFeedbackCount(ctx, f.owner, f.plan.PlanID, uuid.New()); err != nil || n != 0 {
		t.Fatalf("other revision count: %d %v", n, err)
	}
	// A second session ends while Safety is clean; then a new unscreened
	// symptom blocks the gate. Feedback is never gated, and a context change
	// outranks feedback while the count stays visible.
	next, err := f.svc.Start(ctx, f.owner, f.start("start-2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Finish(ctx, f.owner, FinishTrainerSessionCommand{SessionID: next.Session.ID, EndedReason: TrainerEndedStoppedEarly, ExpectedSessionRevision: 0, IdempotencyKey: "stop-2"}); err != nil {
		t.Fatal(err)
	}
	if _, err = body.RecordDirectFacts(ctx, f.owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: head.CurrentRevision, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic new ankle pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordFeedback(ctx, f.owner, feedback(next.Session.ID, "stopped early, ankle hurt", "fb-next")); err != nil {
		t.Fatalf("feedback under a blocking gate: %v", err)
	}
	changed, err := f.plans.View(ctx, f.owner, f.plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Gate.Permits() || changed.Review.Reason != PlanReviewContextChanged || changed.Review.SessionFeedbackCount == nil || *changed.Review.SessionFeedbackCount != 2 {
		t.Fatalf("context change with feedback: %+v gate=%s", changed.Review, changed.Gate.Result)
	}

	// The database refuses feedback on a live session, rewrites and a second row.
	live, err := newReviewedLiveSession(t, f)
	if err != nil {
		t.Fatal(err)
	}
	row := model.TrainingSessionFeedback{ID: uuid.New(), UserID: f.owner, SessionID: live.ID, PlanID: live.PlanID, PlanRevisionID: live.PlanRevisionID, Text: "x", IdempotencyKey: "db-live", RequestFingerprint: strings.Repeat("a", 64), RecordedAt: live.StartedAt}
	if err = f.db.Create(&row).Error; err == nil {
		t.Fatal("database accepted feedback on a live session")
	}
	if err = f.db.Model(&model.TrainingSessionFeedback{}).Where("user_id=? AND session_id=?", f.owner, sid).Update("text", "rewritten").Error; err == nil {
		t.Fatal("database allowed rewriting feedback")
	}
	dup := model.TrainingSessionFeedback{ID: uuid.New(), UserID: f.owner, SessionID: sid, PlanID: f.plan.PlanID, PlanRevisionID: f.plan.RevisionID, Text: "x", IdempotencyKey: "db-dup", RequestFingerprint: strings.Repeat("a", 64), RecordedAt: live.StartedAt}
	if err = f.db.Create(&dup).Error; err == nil {
		t.Fatal("database accepted a second feedback row")
	}
}

// newReviewedLiveSession inserts an in-progress session row directly (the gate
// blocks by now), only to probe the database feedback guard.
func newReviewedLiveSession(t *testing.T, f *trainerFixture) (*model.TrainingSession, error) {
	t.Helper()
	var ended model.TrainingSession
	if err := f.db.Where("user_id=? AND state='ended'", f.owner).Order("started_at").Take(&ended).Error; err != nil {
		return nil, err
	}
	live := ended
	live.ID, live.State, live.Revision, live.EndedReason, live.EndedAt = uuid.New(), TrainerSessionInProgress, 0, nil, nil
	live.StartIdempotencyKey, live.FinishIdempotencyKey, live.FinishRequestFingerprint = "db-live-start", nil, nil
	return &live, f.db.Create(&live).Error
}
