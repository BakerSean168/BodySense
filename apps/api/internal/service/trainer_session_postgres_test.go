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
	return TrainerSetActionCommand{SessionID: session, PlanExerciseID: f.exercise, SetNumber: number, Kind: kind, ExpectedSessionRevision: expected, ExpectedSafetyRevision: 1, IdempotencyKey: uuid.NewString()}
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
	err = f.db.Exec(`INSERT INTO training_set_actions(id,user_id,session_id,plan_exercise_id,exercise_id,set_number,kind,session_revision,idempotency_key,request_fingerprint,safety_revision,gate_result,gate_policy_revision,recorded_at)
VALUES (?,?,?,'e','squat',1,'performed',1,'k',repeat('a',64),1,'allow','trainer-gate-v1',now())`, uuid.New(), foreign, id).Error
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
