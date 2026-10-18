package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// seedTrainerSafety records one user symptom with a complete negative
// structured screen: BodyState revision 1 whose TrainerGate allows.
func seedTrainerSafety(t *testing.T, db *gorm.DB, owner uuid.UUID) {
	t.Helper()
	repo := repository.NewBodyStateRepository(db)
	receipt, err := repo.RecordDirectFacts(context.Background(), owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic knee pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	var factIDs []uuid.UUID
	if err = json.Unmarshal(receipt.FactIDs, &factIDs); err != nil || len(factIDs) != 1 {
		t.Fatalf("fact ids: %v", err)
	}
	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, CaptureID: "0123456789abcdef01234567", BodyFactID: factIDs[0], SourceConversationID: uuid.New(), SourceRunID: uuid.New(), SourceMessageID: uuid.New(), SourceInteractionID: uuid.New(), SourceToolCallID: "intake-0123456789abcdef01234567", BodyStateRevision: 1, RecordedAt: time.Now().UTC()}
	if err = db.Create(&screen).Error; err != nil {
		t.Fatal(err)
	}
}

// trainerFixture is one owner with an accepted active plan (one template, one
// two-set exercise) and screened-clean Safety at revision 1.
type trainerFixture struct {
	db       *gorm.DB
	svc      *TrainerSessionService
	plans    *IndependentTrainingPlanService
	owner    uuid.UUID
	plan     *PlanAcceptance
	template string
	exercise string
}

func newTrainerFixture(t *testing.T) *trainerFixture {
	t.Helper()
	db := openOnboardingContextIntegrationDB(t)
	f := &trainerFixture{db: db, svc: NewTrainerSessionService(db), plans: NewIndependentTrainingPlanService(db), owner: seedOnboardingContextUser(t, db)}
	f.plan = acceptIndependentPlan(t, f.plans, f.owner, "Trainer")
	view, err := f.plans.View(context.Background(), f.owner, f.plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	f.template = view.Draft.Content.Sessions[0].TemplateID
	f.exercise = view.Draft.Content.Sessions[0].Stages[0].Exercises[0].PlanExerciseID
	seedTrainerSafety(t, db, f.owner)
	return f
}

func (f *trainerFixture) start(key string) StartTrainerSessionCommand {
	return StartTrainerSessionCommand{PlanID: f.plan.PlanID, ExpectedPlanRevisionID: f.plan.RevisionID, TemplateID: f.template, TimeZone: "Asia/Shanghai", ExpectedSafetyRevision: 1, IdempotencyKey: key}
}

func (f *trainerFixture) set(session uuid.UUID, number int, kind string, expected int64) TrainerSetActionCommand {
	return TrainerSetActionCommand{SessionID: session, PlanExerciseID: f.exercise, Scope: TrainerScopeSet, SetNumber: number, Kind: kind, ExpectedSessionRevision: expected, ExpectedSafetyRevision: 1, IdempotencyKey: uuid.NewString()}
}

func (f *trainerFixture) skipExercise(session uuid.UUID, expected int64) TrainerSetActionCommand {
	cmd := f.set(session, 0, TrainerSetSkipped, expected)
	cmd.Scope = TrainerScopeExercise
	return cmd
}

func (f *trainerFixture) correct(target model.TrainingSetAction, expected int64) TrainerSetActionCommand {
	cmd := f.set(target.SessionID, trainerSetNumber(target), TrainerSetCorrected, expected)
	cmd.Scope, cmd.CorrectsActionID = target.Scope, target.ID
	return cmd
}

func transition(session uuid.UUID, expected, safety int64, key string) TrainerTransitionCommand {
	return TrainerTransitionCommand{SessionID: session, ExpectedSessionRevision: expected, ExpectedSafetyRevision: safety, IdempotencyKey: key}
}

func (f *trainerFixture) count(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	if err := f.db.Table(table).Where("user_id=?", f.owner).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// Start pins the accepted revision and the IANA local date; set actions are
// idempotent, revisioned receipts; finishing is explicit and final.
func TestTrainerSessionRuntimePostgres(t *testing.T) {
	f := newTrainerFixture(t)
	ctx := context.Background()
	// 00:30 in Shanghai on Oct 14 is still Oct 13 in UTC and on the server.
	startedAt := time.Date(2026, 10, 13, 16, 30, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return startedAt }
	started, err := f.svc.Start(ctx, f.owner, f.start("start-1"))
	if err != nil {
		t.Fatal(err)
	}
	s := started.Session
	if s.TrainingDate.String() != "2026-10-14" || s.TimeZone != "Asia/Shanghai" || s.PlanRevisionID != f.plan.RevisionID ||
		s.State != TrainerSessionInProgress || s.Revision != 0 || s.StartSafetyRevision != 1 || s.StartGateResult != "allow" {
		t.Fatalf("start receipt: %+v", s)
	}
	replay, err := f.svc.Start(ctx, f.owner, f.start("start-1"))
	if err != nil || replay.Session.ID != s.ID {
		t.Fatalf("start replay: %+v %v", replay, err)
	}
	changed := f.start("start-1")
	changed.TimeZone = "Europe/Berlin"
	if _, err = f.svc.Start(ctx, f.owner, changed); !errors.Is(err, ErrTrainerConflict) {
		t.Fatalf("start key reuse: %v", err)
	}
	if _, err = f.svc.Start(ctx, f.owner, f.start("start-2")); !errors.Is(err, ErrTrainerSessionInProgress) {
		t.Fatalf("second in-progress session: %v", err)
	}

	first := f.set(s.ID, 1, TrainerSetPerformed, 0)
	receipt, err := f.svc.RecordSetAction(ctx, f.owner, first)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SessionRevision != 1 || receipt.SafetyRevision != 1 || receipt.GateResult != "allow" || receipt.ExerciseID != "squat" || receipt.Kind != TrainerSetPerformed {
		t.Fatalf("set receipt: %+v", receipt)
	}
	// Duplicate delivery of the same command is idempotent: same receipt, no row.
	again, err := f.svc.RecordSetAction(ctx, f.owner, first)
	if err != nil || again.ID != receipt.ID || f.count(t, "training_set_actions") != 1 {
		t.Fatalf("duplicate set action: %+v %v", again, err)
	}
	reused := first
	reused.Kind = TrainerSetSkipped
	if _, err = f.svc.RecordSetAction(ctx, f.owner, reused); !errors.Is(err, ErrTrainerConflict) {
		t.Fatalf("set key reuse: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(s.ID, 1, TrainerSetSkipped, 1)); !errors.Is(err, ErrTrainerSetRecorded) {
		t.Fatalf("second action on one set: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(s.ID, 2, TrainerSetSkipped, 0)); !errors.Is(err, ErrTrainerConflict) {
		t.Fatalf("stale session revision: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(s.ID, 3, TrainerSetPerformed, 1)); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("set beyond prescription: %v", err)
	}
	unknown := f.set(s.ID, 1, TrainerSetPerformed, 1)
	unknown.PlanExerciseID = "not-in-template"
	if _, err = f.svc.RecordSetAction(ctx, f.owner, unknown); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("exercise outside pinned template: %v", err)
	}
	// Crossing local midnight keeps the session's training date.
	f.svc.now = func() time.Time { return startedAt.Add(24 * time.Hour) }
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(s.ID, 2, TrainerSetSkipped, 1)); err != nil {
		t.Fatal(err)
	}
	finish := FinishTrainerSessionCommand{SessionID: s.ID, EndedReason: TrainerEndedFinished, ExpectedSessionRevision: 2, IdempotencyKey: "finish-1"}
	ended, err := f.svc.Finish(ctx, f.owner, finish)
	if err != nil {
		t.Fatal(err)
	}
	if e := ended.Session; e.State != TrainerSessionEnded || *e.EndedReason != TrainerEndedFinished || e.Revision != 3 ||
		e.TrainingDate.String() != "2026-10-14" || len(ended.Actions) != 2 || ended.Actions[1].Kind != TrainerSetSkipped {
		t.Fatalf("finished session: %+v", ended)
	}
	if replayed, err := f.svc.Finish(ctx, f.owner, finish); err != nil || replayed.Session.Revision != 3 {
		t.Fatalf("finish replay: %+v %v", replayed, err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(s.ID, 2, TrainerSetPerformed, 3)); !errors.Is(err, ErrTrainerSessionEnded) {
		t.Fatalf("action after finish: %v", err)
	}
	// Receipts and ended sessions are immutable in the database too.
	if err = f.db.Exec("UPDATE training_set_actions SET kind='skipped' WHERE user_id=?", f.owner).Error; err == nil {
		t.Fatal("set receipt rewritten")
	}
	if err = f.db.Exec("UPDATE training_sessions SET ended_reason='stopped_early' WHERE user_id=?", f.owner).Error; err == nil {
		t.Fatal("ended session rewritten")
	}
	// A second same-day session is a new instance with its own id.
	f.svc.now = func() time.Time { return startedAt.Add(time.Hour) }
	second, err := f.svc.Start(ctx, f.owner, f.start("start-3"))
	if err != nil || second.Session.ID == s.ID || second.Session.TrainingDate.String() != "2026-10-14" {
		t.Fatalf("second same-day session: %+v %v", second, err)
	}
	views, err := f.svc.List(ctx, f.owner, "")
	if err != nil || len(views) != 2 || len(views[1].Actions) != 2 || len(views[0].Actions) != 0 {
		t.Fatalf("list: %+v %v", views, err)
	}
	// A plan is only read: the runtime never writes its rows.
	if view, err := f.plans.View(ctx, f.owner, f.plan.PlanID); err != nil || view.Plan.StatusRevision != 0 || view.Revision.ID != f.plan.RevisionID {
		t.Fatalf("runtime wrote the plan: %+v %v", view, err)
	}
}

// Stale Safety blocks before the gate; a non-permitting gate blocks; neither
// writes a receipt or advances the session.
func TestTrainerSessionSafetyGatePostgres(t *testing.T) {
	f := newTrainerFixture(t)
	ctx := context.Background()
	stale := f.start("stale")
	stale.ExpectedSafetyRevision = 2
	if _, err := f.svc.Start(ctx, f.owner, stale); !errors.Is(err, ErrTrainerSafetyStale) {
		t.Fatalf("future safety revision: %v", err)
	}
	started, err := f.svc.Start(ctx, f.owner, f.start("start"))
	if err != nil {
		t.Fatal(err)
	}
	// A new unscreened symptom moves Safety to revision 2 mid-session.
	_, err = repository.NewBodyStateRepository(f.db).RecordDirectFacts(ctx, f.owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 1, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic new shoulder pain", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RecordSetAction(ctx, f.owner, f.set(started.Session.ID, 1, TrainerSetPerformed, 0)); !errors.Is(err, ErrTrainerSafetyStale) {
		t.Fatalf("stale restriction version: %v", err)
	}
	current := f.set(started.Session.ID, 1, TrainerSetPerformed, 0)
	current.ExpectedSafetyRevision = 2
	var gate *TrainerGateError
	if _, err = f.svc.RecordSetAction(ctx, f.owner, current); !errors.As(err, &gate) || gate.Decision.Result != safety.ResultBlock || gate.Decision.Permits() {
		t.Fatalf("unscreened safety: %v", err)
	}
	if f.count(t, "training_set_actions") != 0 {
		t.Fatal("blocked action wrote a receipt")
	}
	// Stopping is never gated and backfills nothing.
	ended, err := f.svc.Finish(ctx, f.owner, FinishTrainerSessionCommand{SessionID: started.Session.ID, EndedReason: TrainerEndedStoppedEarly, ExpectedSessionRevision: 0, IdempotencyKey: "stop"})
	if err != nil || ended.Session.Revision != 1 || len(ended.Actions) != 0 {
		t.Fatalf("stop under safety block: %+v %v", ended, err)
	}
	blocked := f.start("blocked")
	blocked.ExpectedSafetyRevision = 2
	if _, err = f.svc.Start(ctx, f.owner, blocked); !errors.As(err, &gate) {
		t.Fatalf("start under safety block: %v", err)
	}
	// A user who never recorded Safety (revision 0) is never "screened clean".
	bare := seedOnboardingContextUser(t, f.db)
	plan := acceptIndependentPlan(t, f.plans, bare, "Bare")
	view, err := f.plans.View(ctx, bare, plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	cmd := StartTrainerSessionCommand{PlanID: plan.PlanID, ExpectedPlanRevisionID: plan.RevisionID, TemplateID: view.Draft.Content.Sessions[0].TemplateID, TimeZone: "UTC", ExpectedSafetyRevision: 1, IdempotencyKey: "bare"}
	if _, err = f.svc.Start(ctx, bare, cmd); !errors.Is(err, ErrTrainerSafetyStale) {
		t.Fatalf("missing safety state: %v", err)
	}
}

// Plan state, plan revision and owner isolation are enforced on start; a
// foreign plan or session is not found and cannot be linked in the database.
func TestTrainerSessionOwnerAndPlanPostgres(t *testing.T) {
	f := newTrainerFixture(t)
	ctx := context.Background()
	foreign := seedOnboardingContextUser(t, f.db)
	seedTrainerSafety(t, f.db, foreign)
	if _, err := f.svc.Start(ctx, foreign, f.start("foreign")); !errors.Is(err, ErrTrainerNotFound) {
		t.Fatalf("foreign plan: %v", err)
	}
	moved := f.start("moved")
	moved.ExpectedPlanRevisionID = uuid.New()
	if _, err := f.svc.Start(ctx, f.owner, moved); !errors.Is(err, ErrTrainerConflict) {
		t.Fatalf("stale plan revision: %v", err)
	}
	missing := f.start("template")
	missing.TemplateID = "not-a-template"
	if _, err := f.svc.Start(ctx, f.owner, missing); !errors.Is(err, ErrTrainerInvalid) {
		t.Fatalf("unknown template: %v", err)
	}
	for _, zone := range []string{"Local", "", "Mars/Olympus", " Asia/Shanghai"} {
		bad := f.start("zone")
		bad.TimeZone = zone
		if _, err := f.svc.Start(ctx, f.owner, bad); !errors.Is(err, ErrTrainerInvalid) {
			t.Fatalf("time zone %q: %v", zone, err)
		}
	}
	if _, err := f.plans.ChangeStatus(ctx, f.owner, planStatusCmd(f.plan.PlanID, trainingplan.PlanPaused, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Start(ctx, f.owner, f.start("paused")); !errors.Is(err, ErrTrainerPlanNotActive) {
		t.Fatalf("paused plan: %v", err)
	}
	if _, err := f.plans.ChangeStatus(ctx, f.owner, planStatusCmd(f.plan.PlanID, trainingplan.PlanActive, 1)); err != nil {
		t.Fatal(err)
	}
	started, err := f.svc.Start(ctx, f.owner, f.start("own"))
	if err != nil {
		t.Fatal(err)
	}
	id := started.Session.ID
	if _, err = f.svc.Get(ctx, foreign, id); !errors.Is(err, ErrTrainerNotFound) {
		t.Fatalf("foreign get: %v", err)
	}
	if _, err = f.svc.RecordSetAction(ctx, foreign, f.set(id, 1, TrainerSetPerformed, 0)); !errors.Is(err, ErrTrainerNotFound) {
		t.Fatalf("foreign set action: %v", err)
	}
	if _, err = f.svc.Finish(ctx, foreign, FinishTrainerSessionCommand{SessionID: id, EndedReason: TrainerEndedFinished, IdempotencyKey: "x"}); !errors.Is(err, ErrTrainerNotFound) {
		t.Fatalf("foreign finish: %v", err)
	}
	if views, err := f.svc.List(ctx, foreign, ""); err != nil || len(views) != 0 {
		t.Fatalf("foreign list: %+v %v", views, err)
	}
	err = f.db.Exec(`INSERT INTO training_set_actions(id,user_id,session_id,plan_exercise_id,exercise_id,scope,set_number,kind,session_revision,idempotency_key,request_fingerprint,safety_revision,gate_result,gate_policy_revision,recorded_at)
VALUES (?,?,?,'e','squat','set',1,'performed',1,'k',repeat('a',64),1,'allow','trainer-gate-v1',now())`, uuid.New(), foreign, id).Error
	if err == nil {
		t.Fatal("receipt linked to another owner's session")
	}
}

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
