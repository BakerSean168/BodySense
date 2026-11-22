package service

import (
	"context"
	"errors"
	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"sync"
	"testing"
)

// Concurrent starts serialize on the owner: exactly one session opens.
func TestTrainerSessionConcurrentStartPostgres(t *testing.T) {
	f := newTrainerFixture(t)
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = f.svc.Start(context.Background(), f.owner, f.start(uuid.NewString()))
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrTrainerSessionInProgress) {
			t.Fatalf("concurrent start: %v", err)
		}
	}
	if ok != 1 || f.count(t, "training_sessions") != 1 {
		t.Fatalf("concurrent starts opened %d sessions", ok)
	}
}

// Pause is ungated and freezes set actions; resume re-runs the TrainerGate at
// the exact Safety revision, needs an active plan and no other session in
// progress; a paused session can always be stopped.
func TestTrainerSessionPauseResumePostgres(t *testing.T) {
	f := newTrainerFixture(t)
	ctx := context.Background()
	started, err := f.svc.Start(ctx, f.owner, f.start("start"))
	if err != nil {
		t.Fatal(err)
	}
	id := started.Session.ID
	if _, err = f.svc.Pause(ctx, f.owner, transition(id, 0, 1, "bad")); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("pause carrying a safety revision: %v", err)
	}
	paused, err := f.svc.Pause(ctx, f.owner, transition(id, 0, 0, "pause-1"))
	if err != nil {
		t.Fatal(err)
	}
	if s := paused.Session; s.State != TrainerSessionPaused || s.Revision != 1 || len(paused.Transitions) != 1 ||
		paused.Transitions[0].Kind != TrainerTransitionPause || paused.Transitions[0].GateResult != nil {
		t.Fatalf("paused: %+v", paused)
	}
	if replay, err := f.svc.Pause(ctx, f.owner, transition(id, 0, 0, "pause-1")); err != nil || replay.Session.Revision != 1 || len(replay.Transitions) != 1 {
		t.Fatalf("pause replay: %+v %v", replay, err)
	}
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 1, 1, "pause-1")); !errors.Is(err, ErrTrainerConflict) {
		t.Fatalf("pause key reused for resume: %v", err)
	}
	if _, err = f.svc.Pause(ctx, f.owner, transition(id, 1, 0, "pause-2")); !errors.Is(err, ErrTrainerSessionPaused) {
		t.Fatalf("pause twice: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(id, 1, TrainerSetPerformed, 1)); !errors.Is(err, ErrTrainerSessionPaused) {
		t.Fatalf("set action while paused: %v", err)
	}
	// A paused session does not hold the single in-progress slot, and while
	// another session runs it cannot be resumed.
	other, err := f.svc.Start(ctx, f.owner, f.start("other"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 1, 1, "resume-busy")); !errors.Is(err, ErrTrainerSessionInProgress) {
		t.Fatalf("resume beside another in-progress session: %v", err)
	}
	if _, err = f.svc.Finish(ctx, f.owner, FinishTrainerSessionCommand{SessionID: other.Session.ID, EndedReason: TrainerEndedStoppedEarly, IdempotencyKey: "stop-other"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 1, 2, "resume-stale")); !errors.Is(err, ErrTrainerSafetyStale) {
		t.Fatalf("resume at another safety revision: %v", err)
	}
	resumed, err := f.svc.Resume(ctx, f.owner, transition(id, 1, 1, "resume-1"))
	if err != nil {
		t.Fatal(err)
	}
	last := resumed.Transitions[len(resumed.Transitions)-1]
	if resumed.Session.State != TrainerSessionInProgress || resumed.Session.Revision != 2 || last.Kind != TrainerTransitionResume ||
		*last.SafetyRevision != 1 || *last.GateResult != "allow" || last.SessionRevision != 2 {
		t.Fatalf("resumed: %+v %+v", resumed.Session, last)
	}
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 2, 1, "resume-2")); !errors.Is(err, ErrTrainerSessionNotPaused) {
		t.Fatalf("resume while in progress: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(id, 1, TrainerSetPerformed, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Pause(ctx, f.owner, transition(id, 3, 0, "pause-3")); err != nil {
		t.Fatal(err)
	}
	// A plan the user paused is not resumed into.
	if _, err = f.plans.ChangeStatus(ctx, f.owner, planStatusCmd(f.plan.PlanID, trainingplan.PlanPaused, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 4, 1, "resume-plan")); !errors.Is(err, ErrTrainerPlanNotActive) {
		t.Fatalf("resume into a paused plan: %v", err)
	}
	if _, err = f.plans.ChangeStatus(ctx, f.owner, planStatusCmd(f.plan.PlanID, trainingplan.PlanActive, 1)); err != nil {
		t.Fatal(err)
	}
	// New unscreened Safety blocks resume and writes nothing; stopping works.
	_, err = repository.NewBodyStateRepository(f.db).RecordDirectFacts(ctx, f.owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 1, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic new shoulder pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 4, 1, "resume-old")); !errors.Is(err, ErrTrainerSafetyStale) {
		t.Fatalf("resume at the safety revision before the change: %v", err)
	}
	var gate *TrainerGateError
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 4, 2, "resume-blocked")); !errors.As(err, &gate) || gate.Decision.Permits() {
		t.Fatalf("resume under unscreened safety: %v", err)
	}
	if f.count(t, "training_session_transitions") != 3 {
		t.Fatal("refused resume wrote a receipt")
	}
	ended, err := f.svc.Finish(ctx, f.owner, FinishTrainerSessionCommand{SessionID: id, EndedReason: TrainerEndedStoppedEarly, ExpectedSessionRevision: 4, IdempotencyKey: "stop"})
	if err != nil || ended.Session.State != TrainerSessionEnded || ended.Session.Revision != 5 || len(ended.Actions) != 1 || len(ended.Transitions) != 3 {
		t.Fatalf("stop a paused session: %+v %v", ended, err)
	}
	if _, err = f.svc.Resume(ctx, f.owner, transition(id, 5, 2, "resume-ended")); !errors.Is(err, ErrTrainerSessionEnded) {
		t.Fatalf("resume after end: %v", err)
	}
	if views, err := f.svc.List(ctx, f.owner, TrainerSessionPaused); err != nil || len(views) != 0 {
		t.Fatalf("paused list: %+v %v", views, err)
	}
	if err = f.db.Exec("UPDATE training_session_transitions SET kind='resume' WHERE user_id=?", f.owner).Error; err == nil {
		t.Fatal("transition receipt rewritten")
	}
}

// Corrections append and retract exactly one receipt of the same target; the
// original stays. Skipping an exercise is one receipt, never per-set skips.
func TestTrainerSessionCorrectionsPostgres(t *testing.T) {
	f := newTrainerFixture(t)
	ctx := context.Background()
	started, err := f.svc.Start(ctx, f.owner, f.start("start"))
	if err != nil {
		t.Fatal(err)
	}
	id := started.Session.ID
	reps, seconds := 8, 45
	first := f.set(id, 1, TrainerSetPerformed, 0)
	first.ActualReps, first.ActualDurationSeconds = &reps, &seconds
	performed, err := f.svc.RecordSetAction(ctx, f.owner, first)
	if err != nil || *performed.ActualReps != 8 || *performed.ActualDurationSeconds != 45 || *performed.SetNumber != 1 || performed.Scope != TrainerScopeSet {
		t.Fatalf("performed with actuals: %+v %v", performed, err)
	}
	correction, err := f.svc.RecordSetAction(ctx, f.owner, f.correct(*performed, 1))
	if err != nil || *correction.CorrectsActionID != performed.ID || correction.SessionRevision != 2 || correction.SafetyRevision != 1 {
		t.Fatalf("correction: %+v %v", correction, err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.correct(*performed, 2)); !errors.Is(err, ErrTrainerConflict) {
		t.Fatalf("second correction of one receipt: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.correct(*correction, 2)); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("correction of a correction: %v", err)
	}
	otherSet := f.correct(*performed, 2)
	otherSet.SetNumber = 2
	unknown := f.correct(*performed, 2)
	unknown.CorrectsActionID = uuid.New()
	for _, cmd := range []TrainerSetActionCommand{otherSet, unknown} {
		if _, err = f.svc.RecordSetAction(ctx, f.owner, cmd); !errors.Is(err, ErrTrainerInvalid) {
			t.Fatalf("correction of another target: %v", err)
		}
	}
	// The retracted set can be recorded again as what really happened.
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(id, 1, TrainerSetSkipped, 2)); err != nil {
		t.Fatal(err)
	}
	skip, err := f.svc.RecordSetAction(ctx, f.owner, f.skipExercise(id, 3))
	if err != nil || skip.SetNumber != nil || skip.Scope != TrainerScopeExercise {
		t.Fatalf("skip exercise: %+v %v", skip, err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(id, 2, TrainerSetPerformed, 4)); !errors.Is(err, ErrTrainerSetRecorded) {
		t.Fatalf("set of a skipped exercise: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.skipExercise(id, 4)); !errors.Is(err, ErrTrainerSetRecorded) {
		t.Fatalf("exercise skipped twice: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.correct(*skip, 4)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(id, 2, TrainerSetPerformed, 5)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.skipExercise(id, 6)); !errors.Is(err, ErrTrainerSetRecorded) {
		t.Fatalf("skip an exercise with every set recorded: %v", err)
	}
	view, err := f.svc.Get(ctx, f.owner, id)
	if err != nil || len(view.Actions) != 6 || view.Session.Revision != 6 {
		t.Fatalf("history: %+v %v", view, err)
	}
	effective := TrainerEffectiveActions(view.Actions)
	if len(effective) != 2 || effective[0].Kind != TrainerSetSkipped || *effective[0].SetNumber != 1 ||
		effective[1].Kind != TrainerSetPerformed || *effective[1].SetNumber != 2 {
		t.Fatalf("effective receipts: %+v", effective)
	}
	// The database refuses a correction without a target, one aimed at another
	// session, and actuals on a non-performed receipt.
	for _, insert := range []string{
		`INSERT INTO training_set_actions(id,user_id,session_id,plan_exercise_id,exercise_id,scope,set_number,kind,session_revision,idempotency_key,request_fingerprint,safety_revision,gate_result,gate_policy_revision,recorded_at)
VALUES (gen_random_uuid(),?,?,'e','squat','set',1,'corrected',99,'db-1',repeat('a',64),1,'allow','trainer-gate-v1',now())`,
		`INSERT INTO training_set_actions(id,user_id,session_id,plan_exercise_id,exercise_id,scope,set_number,kind,corrects_action_id,session_revision,idempotency_key,request_fingerprint,safety_revision,gate_result,gate_policy_revision,recorded_at)
VALUES (gen_random_uuid(),?,?,'e','squat','set',1,'corrected',gen_random_uuid(),99,'db-2',repeat('a',64),1,'allow','trainer-gate-v1',now())`,
		`INSERT INTO training_set_actions(id,user_id,session_id,plan_exercise_id,exercise_id,scope,set_number,kind,actual_reps,session_revision,idempotency_key,request_fingerprint,safety_revision,gate_result,gate_policy_revision,recorded_at)
VALUES (gen_random_uuid(),?,?,'e','squat','set',1,'skipped',5,99,'db-3',repeat('a',64),1,'allow','trainer-gate-v1',now())`,
	} {
		if err = f.db.Exec(insert, f.owner, id).Error; err == nil {
			t.Fatalf("database accepted: %s", insert)
		}
	}
}
