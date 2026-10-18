package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type trainerFake struct {
	owner     uuid.UUID
	sessionOf uuid.UUID // owner stamped on returned rows; defaults to caller
	start     service.StartTrainerSessionCommand
	action    service.TrainerSetActionCommand
	finish    service.FinishTrainerSessionCommand
	moved     service.TrainerTransitionCommand
	feedback  service.TrainerFeedbackCommand
	err       error
}

func (f *trainerFake) rowOwner(owner uuid.UUID) uuid.UUID {
	if f.sessionOf != uuid.Nil {
		return f.sessionOf
	}
	return owner
}

func (f *trainerFake) session(owner, id uuid.UUID, state string) service.TrainerSessionView {
	date, _ := model.ParseDateOnly("2026-10-14")
	return service.TrainerSessionView{Session: model.TrainingSession{
		ID: id, UserID: f.rowOwner(owner), PlanID: f.start.PlanID, PlanRevisionID: uuid.New(), TemplateID: "t1", TimeZone: "Asia/Shanghai",
		TrainingDate: date, State: state, StartedAt: time.Now().UTC(), StartIdempotencyKey: f.start.IdempotencyKey,
		StartSafetyRevision: 1, StartGateResult: "allow", StartGatePolicyRevision: safety.GatePolicyRevision,
	}, Actions: []model.TrainingSetAction{}, Transitions: []model.TrainingSessionTransition{}}
}

func (f *trainerFake) Start(_ context.Context, owner uuid.UUID, cmd service.StartTrainerSessionCommand) (*service.TrainerSessionView, error) {
	f.owner, f.start = owner, cmd
	if f.err != nil {
		return nil, f.err
	}
	v := f.session(owner, uuid.New(), service.TrainerSessionInProgress)
	return &v, nil
}
func (f *trainerFake) RecordSetAction(_ context.Context, owner uuid.UUID, cmd service.TrainerSetActionCommand) (*model.TrainingSetAction, error) {
	f.owner, f.action = owner, cmd
	if f.err != nil {
		return nil, f.err
	}
	return &model.TrainingSetAction{ID: uuid.New(), UserID: f.rowOwner(owner), SessionID: cmd.SessionID, PlanExerciseID: cmd.PlanExerciseID, ExerciseID: "squat",
		Scope: cmd.Scope, SetNumber: setNumberOf(cmd), Kind: cmd.Kind, CorrectsActionID: correctsOf(cmd), ActualReps: cmd.ActualReps, SessionRevision: cmd.ExpectedSessionRevision + 1, IdempotencyKey: cmd.IdempotencyKey,
		SafetyRevision: cmd.ExpectedSafetyRevision, GateResult: "warn", GatePolicyRevision: safety.GatePolicyRevision, RecordedAt: time.Now().UTC()}, nil
}
func setNumberOf(cmd service.TrainerSetActionCommand) *int {
	if cmd.SetNumber == 0 {
		return nil
	}
	return &cmd.SetNumber
}

func correctsOf(cmd service.TrainerSetActionCommand) *uuid.UUID {
	if cmd.CorrectsActionID == uuid.Nil {
		return nil
	}
	return &cmd.CorrectsActionID
}

func (f *trainerFake) transition(owner uuid.UUID, cmd service.TrainerTransitionCommand, kind, state string) (*service.TrainerSessionView, error) {
	f.owner, f.moved = owner, cmd
	if f.err != nil {
		return nil, f.err
	}
	v := f.session(owner, cmd.SessionID, state)
	v.Session.Revision = cmd.ExpectedSessionRevision + 1
	t := model.TrainingSessionTransition{ID: uuid.New(), UserID: f.rowOwner(owner), SessionID: cmd.SessionID, Kind: kind,
		SessionRevision: v.Session.Revision, IdempotencyKey: cmd.IdempotencyKey, RecordedAt: time.Now().UTC()}
	if kind == service.TrainerTransitionResume {
		result, policy := "allow", safety.GatePolicyRevision
		t.SafetyRevision, t.GateResult, t.GatePolicyRevision = &cmd.ExpectedSafetyRevision, &result, &policy
	}
	v.Transitions = append(v.Transitions, t)
	return &v, nil
}
func (f *trainerFake) Pause(_ context.Context, owner uuid.UUID, cmd service.TrainerTransitionCommand) (*service.TrainerSessionView, error) {
	return f.transition(owner, cmd, service.TrainerTransitionPause, service.TrainerSessionPaused)
}
func (f *trainerFake) Resume(_ context.Context, owner uuid.UUID, cmd service.TrainerTransitionCommand) (*service.TrainerSessionView, error) {
	return f.transition(owner, cmd, service.TrainerTransitionResume, service.TrainerSessionInProgress)
}
func (f *trainerFake) Finish(_ context.Context, owner uuid.UUID, cmd service.FinishTrainerSessionCommand) (*service.TrainerSessionView, error) {
	f.owner, f.finish = owner, cmd
	if f.err != nil {
		return nil, f.err
	}
	v := f.session(owner, cmd.SessionID, service.TrainerSessionEnded)
	now, reason := time.Now().UTC(), cmd.EndedReason
	v.Session.EndedAt, v.Session.EndedReason, v.Session.Revision = &now, &reason, cmd.ExpectedSessionRevision+1
	return &v, nil
}
func (f *trainerFake) RecordFeedback(_ context.Context, owner uuid.UUID, cmd service.TrainerFeedbackCommand) (*service.TrainerSessionView, error) {
	f.owner, f.feedback = owner, cmd
	if f.err != nil {
		return nil, f.err
	}
	v := f.session(owner, cmd.SessionID, service.TrainerSessionEnded)
	now, reason := time.Now().UTC(), service.TrainerEndedFinished
	v.Session.EndedAt, v.Session.EndedReason, v.Session.Revision = &now, &reason, 1
	v.Feedback = &model.TrainingSessionFeedback{ID: uuid.New(), UserID: f.rowOwner(owner), SessionID: cmd.SessionID, PlanID: v.Session.PlanID,
		PlanRevisionID: v.Session.PlanRevisionID, Text: cmd.Text, IdempotencyKey: cmd.IdempotencyKey, RecordedAt: now}
	return &v, nil
}
func (f *trainerFake) Get(_ context.Context, owner, id uuid.UUID) (*service.TrainerSessionView, error) {
	f.owner = owner
	if f.err != nil {
		return nil, f.err
	}
	v := f.session(owner, id, service.TrainerSessionInProgress)
	return &v, nil
}
func (f *trainerFake) List(_ context.Context, owner uuid.UUID, _ string) ([]service.TrainerSessionView, error) {
	f.owner = owner
	return []service.TrainerSessionView{f.session(owner, uuid.New(), service.TrainerSessionInProgress)}, f.err
}

func trainerRoute(t *testing.T, owner uuid.UUID, app trainerSessionApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithTrainerSessions(app)), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}

func startBody() map[string]any {
	return map[string]any{"planId": uuid.NewString(), "expectedPlanRevisionId": uuid.NewString(), "templateId": "t1",
		"timeZone": "Asia/Shanghai", "expectedSafetyRevision": 1, "idempotencyKey": "start-1"}
}

func TestTrainerRoutesRequireAuthenticatedOwner(t *testing.T) {
	fake := &trainerFake{}
	r := trainerRoute(t, uuid.Nil, fake)
	if rec := doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions", startBody()); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
	if fake.owner != uuid.Nil {
		t.Fatal("service reached without owner")
	}
}

func TestTrainerStartAndSetActionRoundTrip(t *testing.T) {
	owner := uuid.New()
	fake := &trainerFake{}
	r := trainerRoute(t, owner, fake)
	rec := doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions", startBody())
	if rec.Code != http.StatusOK || fake.owner != owner || fake.start.TimeZone != "Asia/Shanghai" || fake.start.ExpectedSafetyRevision != 1 {
		t.Fatalf("start: %d %s %+v", rec.Code, rec.Body.String(), fake.start)
	}
	var session openapiv1.TrainingSessionRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil || session.TrainingDate.String() != "2026-10-14" || session.State != "in_progress" || session.Actions == nil {
		t.Fatalf("session record: %v %s", err, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "user_id") || strings.Contains(rec.Body.String(), "fingerprint") {
		t.Fatalf("leaked owner or fingerprint: %s", rec.Body.String())
	}
	body := map[string]any{"planExerciseId": "e1", "scope": "set", "setNumber": 2, "kind": "skipped", "expectedSessionRevision": 1, "expectedSafetyRevision": 3, "idempotencyKey": "set-2"}
	rec = doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions/"+session.SessionId.String()+"/set-actions", body)
	if rec.Code != http.StatusOK || fake.action.SessionID != uuid.UUID(session.SessionId) || fake.action.Kind != "skipped" || fake.action.ExpectedSafetyRevision != 3 {
		t.Fatalf("set action: %d %s %+v", rec.Code, rec.Body.String(), fake.action)
	}
	var receipt openapiv1.TrainingSetActionRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &receipt); err != nil || receipt.SessionRevision != 2 || receipt.Gate.SafetyRevision != 3 || receipt.Gate.Result != "warn" {
		t.Fatalf("receipt: %v %s", err, rec.Body.String())
	}
	rec = doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions/"+session.SessionId.String()+"/finish",
		map[string]any{"endedReason": "stopped_early", "expectedSessionRevision": 2, "idempotencyKey": "finish"})
	if rec.Code != http.StatusOK || fake.finish.EndedReason != "stopped_early" || !strings.Contains(rec.Body.String(), `"state":"ended"`) {
		t.Fatalf("finish: %d %s", rec.Code, rec.Body.String())
	}
}

// The contract rejects inferred or unknown action kinds and smuggled fields
// before the runtime is reached.
func TestTrainerContractRejectsUnknownCommands(t *testing.T) {
	owner := uuid.New()
	fake := &trainerFake{}
	r := trainerRoute(t, owner, fake)
	smuggled := startBody()
	smuggled["userId"] = uuid.NewString()
	missing := startBody()
	delete(missing, "expectedSafetyRevision")
	for _, body := range []map[string]any{smuggled, missing} {
		if rec := doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("start body %v: %d", body, rec.Code)
		}
	}
	for _, kind := range []string{"completed", "auto"} {
		body := map[string]any{"planExerciseId": "e1", "scope": "set", "setNumber": 1, "kind": kind, "expectedSessionRevision": 0, "expectedSafetyRevision": 1, "idempotencyKey": "k"}
		if rec := doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions/"+uuid.NewString()+"/set-actions", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("kind %s: %d", kind, rec.Code)
		}
	}
	setBody := func(change map[string]any, drop string) map[string]any {
		body := map[string]any{"planExerciseId": "e1", "scope": "set", "setNumber": 1, "kind": "performed", "expectedSessionRevision": 0, "expectedSafetyRevision": 1, "idempotencyKey": "k"}
		for k, v := range change {
			body[k] = v
		}
		delete(body, drop)
		return body
	}
	for name, body := range map[string]map[string]any{
		"no scope":         setBody(nil, "scope"),
		"unknown scope":    setBody(map[string]any{"scope": "session"}, ""),
		"too many reps":    setBody(map[string]any{"actualReps": 1001}, ""),
		"zero duration":    setBody(map[string]any{"actualDurationSeconds": 0}, ""),
		"bad correct id":   setBody(map[string]any{"kind": "corrected", "correctsActionId": "nope"}, ""),
		"smuggled verdict": setBody(map[string]any{"gate": map[string]any{"result": "allow"}}, ""),
	} {
		if rec := doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions/"+uuid.NewString()+"/set-actions", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", name, rec.Code)
		}
	}
	id := uuid.NewString()
	for path, body := range map[string]map[string]any{
		"/resume": {"expectedSessionRevision": 1, "idempotencyKey": "r"},
		"/pause":  {"expectedSessionRevision": 1, "expectedSafetyRevision": 1, "idempotencyKey": "p"},
	} {
		if rec := doPlanRequest(t, r, http.MethodPost, "/api/v1/training-sessions/"+id+path, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s body %v: %d", path, body, rec.Code)
		}
	}
	if fake.owner != uuid.Nil {
		t.Fatal("invalid command reached the runtime")
	}
}

func TestTrainerErrorsAreVisibleNeverSuccess(t *testing.T) {
	owner := uuid.New()
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{&service.TrainerGateError{Decision: safety.Decision{Result: safety.ResultBlock, Reasons: []safety.Reason{safety.ReasonRestrictedRestriction}}}, http.StatusConflict, "TRAINER_GATE_BLOCKED"},
		{&service.TrainerGateError{Decision: safety.Decision{Result: "maybe"}}, http.StatusConflict, "TRAINER_GATE_BLOCKED"},
		{service.ErrTrainerSafetyStale, http.StatusConflict, "SAFETY_STALE"},
		{service.ErrTrainerNotFound, http.StatusNotFound, "NOT_FOUND"},
		{service.ErrTrainerConflict, http.StatusConflict, "REVISION_CONFLICT"},
		{service.ErrTrainerPlanNotActive, http.StatusConflict, "PLAN_NOT_ACTIVE"},
		{service.ErrTrainerSessionInProgress, http.StatusConflict, "SESSION_IN_PROGRESS"},
		{service.ErrTrainerSessionEnded, http.StatusConflict, "SESSION_ENDED"},
		{service.ErrTrainerSetRecorded, http.StatusConflict, "SET_ALREADY_RECORDED"},
		{service.ErrTrainerSessionPaused, http.StatusConflict, "SESSION_PAUSED"},
		{service.ErrTrainerSessionNotPaused, http.StatusConflict, "SESSION_NOT_PAUSED"},
		{service.ErrTrainerInvalid, http.StatusBadRequest, "INVALID_REQUEST"},
		{context.DeadlineExceeded, http.StatusServiceUnavailable, "TRAINER_UNAVAILABLE"},
	}
	for _, c := range cases {
		fake := &trainerFake{err: c.err}
		rec := doPlanRequest(t, trainerRoute(t, owner, fake), http.MethodPost, "/api/v1/training-sessions", startBody())
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.code) {
			t.Fatalf("%v: %d %s", c.err, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "maybe") {
			t.Fatalf("unknown gate result leaked: %s", rec.Body.String())
		}
	}
}

func TestTrainerForeignProjectionIsNeverServed(t *testing.T) {
	owner := uuid.New()
	fake := &trainerFake{sessionOf: uuid.New()}
	r := trainerRoute(t, owner, fake)
	if rec := doPlanRequest(t, r, http.MethodGet, "/api/v1/training-sessions/"+uuid.NewString(), nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("cross-owner session served: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doPlanRequest(t, r, http.MethodGet, "/api/v1/training-sessions", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("cross-owner list served: %d", rec.Code)
	}
	fake = &trainerFake{err: service.ErrTrainerNotFound}
	if rec := doPlanRequest(t, trainerRoute(t, owner, fake), http.MethodGet, "/api/v1/training-sessions/"+uuid.NewString(), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing session: %d", rec.Code)
	}
}

// Exercise skips, corrections with actuals, pause and resume pass only the
// user's explicit command through and project their receipts.
func TestTrainerPauseResumeAndCorrectionRoundTrip(t *testing.T) {
	owner := uuid.New()
	fake := &trainerFake{}
	r := trainerRoute(t, owner, fake)
	base := "/api/v1/training-sessions/" + uuid.NewString()
	rec := doPlanRequest(t, r, http.MethodPost, base+"/set-actions", map[string]any{"planExerciseId": "e1", "scope": "set", "setNumber": 1, "kind": "performed",
		"actualReps": 8, "actualDurationSeconds": 40, "expectedSessionRevision": 0, "expectedSafetyRevision": 1, "idempotencyKey": "set-1"})
	if rec.Code != http.StatusOK || *fake.action.ActualReps != 8 || *fake.action.ActualDurationSeconds != 40 || !strings.Contains(rec.Body.String(), `"actualReps":8`) {
		t.Fatalf("performed with actuals: %d %s", rec.Code, rec.Body.String())
	}
	rec = doPlanRequest(t, r, http.MethodPost, base+"/set-actions", map[string]any{"planExerciseId": "e1", "scope": "exercise", "kind": "skipped",
		"expectedSessionRevision": 1, "expectedSafetyRevision": 1, "idempotencyKey": "skip"})
	if rec.Code != http.StatusOK || fake.action.Scope != "exercise" || fake.action.SetNumber != 0 || strings.Contains(rec.Body.String(), "setNumber") {
		t.Fatalf("exercise skip: %d %s %+v", rec.Code, rec.Body.String(), fake.action)
	}
	target := uuid.New()
	rec = doPlanRequest(t, r, http.MethodPost, base+"/set-actions", map[string]any{"planExerciseId": "e1", "scope": "exercise", "kind": "corrected",
		"correctsActionId": target.String(), "expectedSessionRevision": 2, "expectedSafetyRevision": 1, "idempotencyKey": "fix"})
	if rec.Code != http.StatusOK || fake.action.CorrectsActionID != target || !strings.Contains(rec.Body.String(), target.String()) {
		t.Fatalf("correction: %d %s", rec.Code, rec.Body.String())
	}
	rec = doPlanRequest(t, r, http.MethodPost, base+"/pause", map[string]any{"expectedSessionRevision": 3, "idempotencyKey": "pause"})
	if rec.Code != http.StatusOK || fake.moved.ExpectedSafetyRevision != 0 || !strings.Contains(rec.Body.String(), `"state":"paused"`) ||
		strings.Contains(rec.Body.String(), `"gate"`) {
		t.Fatalf("pause: %d %s", rec.Code, rec.Body.String())
	}
	rec = doPlanRequest(t, r, http.MethodPost, base+"/resume", map[string]any{"expectedSessionRevision": 4, "expectedSafetyRevision": 2, "idempotencyKey": "resume"})
	var session openapiv1.TrainingSessionRecord
	if rec.Code != http.StatusOK || fake.moved.ExpectedSafetyRevision != 2 || json.Unmarshal(rec.Body.Bytes(), &session) != nil ||
		session.State != "in_progress" || len(session.Transitions) != 1 || session.Transitions[0].Gate == nil || session.Transitions[0].Gate.SafetyRevision != 2 {
		t.Fatalf("resume: %d %s", rec.Code, rec.Body.String())
	}
	fake.err = service.ErrTrainerSessionPaused
	if rec = doPlanRequest(t, r, http.MethodPost, base+"/pause", map[string]any{"expectedSessionRevision": 5, "idempotencyKey": "again"}); rec.Code != http.StatusConflict {
		t.Fatalf("pause twice: %d", rec.Code)
	}
}

// Feedback passes the user's words through verbatim, projects them on the
// session and maps its refusals; the contract rejects blank, oversized or
// smuggled bodies before the runtime.
func TestTrainerFeedbackRoundTrip(t *testing.T) {
	owner := uuid.New()
	fake := &trainerFake{}
	r := trainerRoute(t, owner, fake)
	base := "/api/v1/training-sessions/" + uuid.NewString() + "/feedback"
	text := "  有点累，\n膝盖还好  "
	rec := doPlanRequest(t, r, http.MethodPost, base, map[string]any{"text": text, "idempotencyKey": "fb"})
	var session openapiv1.TrainingSessionRecord
	if rec.Code != http.StatusOK || fake.feedback.Text != text || fake.owner != owner || json.Unmarshal(rec.Body.Bytes(), &session) != nil ||
		session.Feedback == nil || session.Feedback.Text != text || session.Feedback.IdempotencyKey != "fb" {
		t.Fatalf("feedback: %d %s", rec.Code, rec.Body.String())
	}
	for name, body := range map[string]map[string]any{
		"blank":    {"text": " \n ", "idempotencyKey": "b"},
		"empty":    {"text": "", "idempotencyKey": "b"},
		"too long": {"text": strings.Repeat("a", 2001), "idempotencyKey": "b"},
		"no key":   {"text": "ok"},
		"smuggled": {"text": "ok", "idempotencyKey": "b", "planRevisionId": uuid.NewString()},
		"verdict":  {"text": "ok", "idempotencyKey": "b", "recommended": true},
		"no text":  {"idempotencyKey": "b"},
		"bad key":  {"text": "ok", "idempotencyKey": " b"},
		"not text": {"text": 3, "idempotencyKey": "b"},
		"user":     {"text": "ok", "idempotencyKey": "b", "userId": uuid.NewString()},
	} {
		fake.feedback = service.TrainerFeedbackCommand{}
		if rec := doPlanRequest(t, r, http.MethodPost, base, body); rec.Code != http.StatusBadRequest || fake.feedback.IdempotencyKey != "" {
			t.Fatalf("%s accepted: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	for err, code := range map[error]string{service.ErrTrainerSessionNotEnded: "SESSION_NOT_ENDED", service.ErrTrainerFeedbackRecorded: "FEEDBACK_ALREADY_RECORDED",
		service.ErrTrainerConflict: "REVISION_CONFLICT", service.ErrTrainerNotFound: "NOT_FOUND"} {
		fake.err = err
		if rec := doPlanRequest(t, r, http.MethodPost, base, map[string]any{"text": "ok", "idempotencyKey": "x"}); !strings.Contains(rec.Body.String(), code) || rec.Code < 400 {
			t.Fatalf("%v: %d %s", err, rec.Code, rec.Body.String())
		}
	}
	// Feedback of another owner is never served.
	foreign := &trainerFake{sessionOf: uuid.New()}
	if rec := doPlanRequest(t, trainerRoute(t, owner, foreign), http.MethodPost, base, map[string]any{"text": "ok", "idempotencyKey": "f"}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("foreign feedback served: %d", rec.Code)
	}
}
