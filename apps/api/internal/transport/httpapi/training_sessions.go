package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

// trainerSessionApplication is the Go-owned Trainer runtime (TP-003). HTTP
// supplies only the authenticated owner and the user's explicit command; it
// never accepts a caller-owned user id, an Agent action or a Safety verdict.
type trainerSessionApplication interface {
	Start(context.Context, uuid.UUID, service.StartTrainerSessionCommand) (*service.TrainerSessionView, error)
	RecordSetAction(context.Context, uuid.UUID, service.TrainerSetActionCommand) (*model.TrainingSetAction, error)
	Pause(context.Context, uuid.UUID, service.TrainerTransitionCommand) (*service.TrainerSessionView, error)
	Resume(context.Context, uuid.UUID, service.TrainerTransitionCommand) (*service.TrainerSessionView, error)
	Finish(context.Context, uuid.UUID, service.FinishTrainerSessionCommand) (*service.TrainerSessionView, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (*service.TrainerSessionView, error)
	List(context.Context, uuid.UUID, string) ([]service.TrainerSessionView, error)
}

func (s *PublicServer) WithTrainerSessions(sessions trainerSessionApplication) *PublicServer {
	s.trainerSessions = sessions
	return s
}

type trainerHTTPError struct {
	status  int
	code    string
	message string
}

func (r trainerHTTPError) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}
func (r trainerHTTPError) VisitListTrainingSessionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainerHTTPError) VisitStartTrainingSessionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainerHTTPError) VisitGetTrainingSessionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainerHTTPError) VisitRecordTrainingSetActionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainerHTTPError) VisitPauseTrainingSessionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainerHTTPError) VisitResumeTrainingSessionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainerHTTPError) VisitFinishTrainingSessionResponse(w http.ResponseWriter) error {
	return r.write(w)
}

var (
	trainerUnauthorized = trainerHTTPError{http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"}
	trainerUnavailable  = trainerHTTPError{http.StatusServiceUnavailable, "TRAINER_UNAVAILABLE", "trainer runtime unavailable"}
	trainerInconsistent = trainerHTTPError{http.StatusServiceUnavailable, "TRAINER_UNAVAILABLE", "trainer projection violates contract"}
	trainerBadRequest   = trainerHTTPError{http.StatusBadRequest, "INVALID_REQUEST", "invalid trainer command"}
)

func (s *PublicServer) trainerOwner(ctx context.Context) (uuid.UUID, *trainerHTTPError) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return uuid.Nil, &trainerUnauthorized
	}
	if s.trainerSessions == nil {
		return uuid.Nil, &trainerUnavailable
	}
	return owner, nil
}

// trainerCommandError maps runtime errors. Foreign and missing ids are the
// same owner-masked 404; every Safety refusal is a visible 409, never success.
func trainerCommandError(err error) trainerHTTPError {
	var gate *service.TrainerGateError
	if errors.As(err, &gate) {
		reasons := make([]string, 0, len(gate.Decision.Reasons))
		for _, reason := range gate.Decision.Reasons {
			reasons = append(reasons, string(reason))
		}
		result := gate.Decision.Result
		if result != safety.ResultBlock {
			result = safety.ResultUnavailable
		}
		return trainerHTTPError{http.StatusConflict, "TRAINER_GATE_BLOCKED", "safety gate " + string(result) + ": " + strings.Join(reasons, ",")}
	}
	switch {
	case errors.Is(err, service.ErrTrainerNotFound):
		return trainerHTTPError{http.StatusNotFound, "NOT_FOUND", "training plan or session not found"}
	case errors.Is(err, service.ErrTrainerInvalid):
		return trainerBadRequest
	case errors.Is(err, service.ErrTrainerSafetyStale):
		return trainerHTTPError{http.StatusConflict, "SAFETY_STALE", "safety changed since it was shown; review the current gate and confirm again"}
	case errors.Is(err, service.ErrTrainerConflict):
		return trainerHTTPError{http.StatusConflict, "REVISION_CONFLICT", "plan, session or idempotency key conflict; refresh and retry"}
	case errors.Is(err, service.ErrTrainerPlanNotActive):
		return trainerHTTPError{http.StatusConflict, "PLAN_NOT_ACTIVE", "only an active plan can be started"}
	case errors.Is(err, service.ErrTrainerSessionInProgress):
		return trainerHTTPError{http.StatusConflict, "SESSION_IN_PROGRESS", "another training session is in progress"}
	case errors.Is(err, service.ErrTrainerSessionEnded):
		return trainerHTTPError{http.StatusConflict, "SESSION_ENDED", "training session already ended"}
	case errors.Is(err, service.ErrTrainerSessionPaused):
		return trainerHTTPError{http.StatusConflict, "SESSION_PAUSED", "training session is paused; resume it first"}
	case errors.Is(err, service.ErrTrainerSessionNotPaused):
		return trainerHTTPError{http.StatusConflict, "SESSION_NOT_PAUSED", "training session is not paused"}
	case errors.Is(err, service.ErrTrainerSetRecorded):
		return trainerHTTPError{http.StatusConflict, "SET_ALREADY_RECORDED", "this set or exercise already has a recorded action"}
	}
	return trainerUnavailable
}

// Wire shapes mirror the OpenAPI components; strictOpenAPIConvert validates.
type trainerGateReceiptWire struct {
	Result         string `json:"result"`
	SafetyRevision int64  `json:"safetyRevision"`
	PolicyRevision string `json:"policyRevision"`
}
type trainerSetActionWire struct {
	ActionID              uuid.UUID              `json:"actionId"`
	SessionID             uuid.UUID              `json:"sessionId"`
	PlanExerciseID        string                 `json:"planExerciseId"`
	ExerciseID            string                 `json:"exerciseId"`
	Scope                 string                 `json:"scope"`
	SetNumber             *int                   `json:"setNumber,omitempty"`
	Kind                  string                 `json:"kind"`
	CorrectsActionID      *uuid.UUID             `json:"correctsActionId,omitempty"`
	ActualReps            *int                   `json:"actualReps,omitempty"`
	ActualDurationSeconds *int                   `json:"actualDurationSeconds,omitempty"`
	SessionRevision       int64                  `json:"sessionRevision"`
	Gate                  trainerGateReceiptWire `json:"gate"`
	IdempotencyKey        string                 `json:"idempotencyKey"`
	RecordedAt            time.Time              `json:"recordedAt"`
}
type trainerTransitionWire struct {
	TransitionID    uuid.UUID               `json:"transitionId"`
	Kind            string                  `json:"kind"`
	SessionRevision int64                   `json:"sessionRevision"`
	Gate            *trainerGateReceiptWire `json:"gate,omitempty"`
	IdempotencyKey  string                  `json:"idempotencyKey"`
	RecordedAt      time.Time               `json:"recordedAt"`
}
type trainerSessionWire struct {
	SessionID      uuid.UUID               `json:"sessionId"`
	PlanID         uuid.UUID               `json:"planId"`
	PlanRevisionID uuid.UUID               `json:"planRevisionId"`
	TemplateID     string                  `json:"templateId"`
	TimeZone       string                  `json:"timeZone"`
	TrainingDate   string                  `json:"trainingDate"`
	State          string                  `json:"state"`
	EndedReason    *string                 `json:"endedReason,omitempty"`
	Revision       int64                   `json:"revision"`
	StartedAt      time.Time               `json:"startedAt"`
	EndedAt        *time.Time              `json:"endedAt,omitempty"`
	StartGate      trainerGateReceiptWire  `json:"startGate"`
	Actions        []trainerSetActionWire  `json:"actions"`
	Transitions    []trainerTransitionWire `json:"transitions"`
}

func trainerActionWire(owner uuid.UUID, a model.TrainingSetAction) (trainerSetActionWire, error) {
	if a.UserID != owner {
		return trainerSetActionWire{}, errors.New("set action owner mismatch")
	}
	return trainerSetActionWire{
		ActionID: a.ID, SessionID: a.SessionID, PlanExerciseID: a.PlanExerciseID, ExerciseID: a.ExerciseID, Scope: a.Scope, SetNumber: a.SetNumber,
		Kind: a.Kind, CorrectsActionID: a.CorrectsActionID, ActualReps: a.ActualReps, ActualDurationSeconds: a.ActualDurationSeconds,
		SessionRevision: a.SessionRevision, IdempotencyKey: a.IdempotencyKey, RecordedAt: a.RecordedAt,
		Gate: trainerGateReceiptWire{Result: a.GateResult, SafetyRevision: a.SafetyRevision, PolicyRevision: a.GatePolicyRevision},
	}, nil
}

func trainerActionRecord(owner uuid.UUID, a model.TrainingSetAction) (openapiv1.TrainingSetActionRecord, error) {
	wire, err := trainerActionWire(owner, a)
	if err != nil {
		return openapiv1.TrainingSetActionRecord{}, err
	}
	return strictOpenAPIConvert[openapiv1.TrainingSetActionRecord]("TrainingSetActionRecord", wire)
}

func trainerSessionRecord(owner uuid.UUID, v service.TrainerSessionView) (openapiv1.TrainingSessionRecord, error) {
	s := v.Session
	if s.UserID != owner {
		return openapiv1.TrainingSessionRecord{}, errors.New("session owner mismatch")
	}
	wire := trainerSessionWire{
		SessionID: s.ID, PlanID: s.PlanID, PlanRevisionID: s.PlanRevisionID, TemplateID: s.TemplateID, TimeZone: s.TimeZone,
		TrainingDate: s.TrainingDate.String(), State: s.State, EndedReason: s.EndedReason, Revision: s.Revision,
		StartedAt: s.StartedAt, EndedAt: s.EndedAt, Actions: []trainerSetActionWire{}, Transitions: []trainerTransitionWire{},
		StartGate: trainerGateReceiptWire{Result: s.StartGateResult, SafetyRevision: s.StartSafetyRevision, PolicyRevision: s.StartGatePolicyRevision},
	}
	for _, a := range v.Actions {
		if a.SessionID != s.ID {
			return openapiv1.TrainingSessionRecord{}, errors.New("set action session mismatch")
		}
		action, err := trainerActionWire(owner, a)
		if err != nil {
			return openapiv1.TrainingSessionRecord{}, err
		}
		wire.Actions = append(wire.Actions, action)
	}
	for _, t := range v.Transitions {
		if t.UserID != owner || t.SessionID != s.ID {
			return openapiv1.TrainingSessionRecord{}, errors.New("transition owner or session mismatch")
		}
		transition := trainerTransitionWire{
			TransitionID: t.ID, Kind: t.Kind, SessionRevision: t.SessionRevision, IdempotencyKey: t.IdempotencyKey, RecordedAt: t.RecordedAt,
		}
		if t.SafetyRevision != nil && t.GateResult != nil && t.GatePolicyRevision != nil {
			transition.Gate = &trainerGateReceiptWire{Result: *t.GateResult, SafetyRevision: *t.SafetyRevision, PolicyRevision: *t.GatePolicyRevision}
		}
		wire.Transitions = append(wire.Transitions, transition)
	}
	return strictOpenAPIConvert[openapiv1.TrainingSessionRecord]("TrainingSessionRecord", wire)
}

func (s *PublicServer) ListTrainingSessions(ctx context.Context, req openapiv1.ListTrainingSessionsRequestObject) (openapiv1.ListTrainingSessionsResponseObject, error) {
	owner, fail := s.trainerOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	state := ""
	if req.Params.State != nil {
		state = string(*req.Params.State)
	}
	views, err := s.trainerSessions.List(applicationRequestContext(ctx), owner, state)
	if err != nil {
		return trainerCommandError(err), nil
	}
	out := openapiv1.ListTrainingSessions200JSONResponse{Sessions: []openapiv1.TrainingSessionRecord{}}
	for _, v := range views {
		record, err := trainerSessionRecord(owner, v)
		if err != nil {
			return trainerInconsistent, nil
		}
		out.Sessions = append(out.Sessions, record)
	}
	return out, nil
}

func (s *PublicServer) StartTrainingSession(ctx context.Context, req openapiv1.StartTrainingSessionRequestObject) (openapiv1.StartTrainingSessionResponseObject, error) {
	owner, fail := s.trainerOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	if req.Body == nil {
		return trainerBadRequest, nil
	}
	cmd := service.StartTrainerSessionCommand{
		PlanID: uuid.UUID(req.Body.PlanId), ExpectedPlanRevisionID: uuid.UUID(req.Body.ExpectedPlanRevisionId), TemplateID: req.Body.TemplateId,
		TimeZone: req.Body.TimeZone, ExpectedSafetyRevision: req.Body.ExpectedSafetyRevision, IdempotencyKey: string(req.Body.IdempotencyKey),
	}
	view, err := s.trainerSessions.Start(applicationRequestContext(ctx), owner, cmd)
	if err != nil {
		return trainerCommandError(err), nil
	}
	if view == nil || view.Session.PlanID != cmd.PlanID || view.Session.StartIdempotencyKey != cmd.IdempotencyKey {
		return trainerInconsistent, nil
	}
	record, err := trainerSessionRecord(owner, *view)
	if err != nil {
		return trainerInconsistent, nil
	}
	return openapiv1.StartTrainingSession200JSONResponse(record), nil
}

func (s *PublicServer) GetTrainingSession(ctx context.Context, req openapiv1.GetTrainingSessionRequestObject) (openapiv1.GetTrainingSessionResponseObject, error) {
	owner, fail := s.trainerOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	view, err := s.trainerSessions.Get(applicationRequestContext(ctx), owner, uuid.UUID(req.SessionId))
	if err != nil {
		return trainerCommandError(err), nil
	}
	if view == nil || view.Session.ID != uuid.UUID(req.SessionId) {
		return trainerInconsistent, nil
	}
	record, err := trainerSessionRecord(owner, *view)
	if err != nil {
		return trainerInconsistent, nil
	}
	return openapiv1.GetTrainingSession200JSONResponse(record), nil
}

func (s *PublicServer) RecordTrainingSetAction(ctx context.Context, req openapiv1.RecordTrainingSetActionRequestObject) (openapiv1.RecordTrainingSetActionResponseObject, error) {
	owner, fail := s.trainerOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	if req.Body == nil {
		return trainerBadRequest, nil
	}
	cmd := service.TrainerSetActionCommand{
		SessionID: uuid.UUID(req.SessionId), PlanExerciseID: req.Body.PlanExerciseId, Scope: string(req.Body.Scope), Kind: string(req.Body.Kind),
		ActualReps: req.Body.ActualReps, ActualDurationSeconds: req.Body.ActualDurationSeconds,
		ExpectedSessionRevision: req.Body.ExpectedSessionRevision, ExpectedSafetyRevision: req.Body.ExpectedSafetyRevision,
		IdempotencyKey: string(req.Body.IdempotencyKey),
	}
	if req.Body.SetNumber != nil {
		cmd.SetNumber = *req.Body.SetNumber
	}
	if req.Body.CorrectsActionId != nil {
		cmd.CorrectsActionID = uuid.UUID(*req.Body.CorrectsActionId)
	}
	action, err := s.trainerSessions.RecordSetAction(applicationRequestContext(ctx), owner, cmd)
	if err != nil {
		return trainerCommandError(err), nil
	}
	if action == nil || action.SessionID != cmd.SessionID || action.IdempotencyKey != cmd.IdempotencyKey {
		return trainerInconsistent, nil
	}
	record, err := trainerActionRecord(owner, *action)
	if err != nil {
		return trainerInconsistent, nil
	}
	return openapiv1.RecordTrainingSetAction200JSONResponse(record), nil
}

func (s *PublicServer) PauseTrainingSession(ctx context.Context, req openapiv1.PauseTrainingSessionRequestObject) (openapiv1.PauseTrainingSessionResponseObject, error) {
	owner, fail := s.trainerOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	if req.Body == nil {
		return trainerBadRequest, nil
	}
	cmd := service.TrainerTransitionCommand{
		SessionID: uuid.UUID(req.SessionId), ExpectedSessionRevision: req.Body.ExpectedSessionRevision, IdempotencyKey: string(req.Body.IdempotencyKey),
	}
	view, err := s.trainerSessions.Pause(applicationRequestContext(ctx), owner, cmd)
	record, fail := trainerTransitionRecord(owner, cmd, view, err)
	if fail != nil {
		return *fail, nil
	}
	return openapiv1.PauseTrainingSession200JSONResponse(record), nil
}

func (s *PublicServer) ResumeTrainingSession(ctx context.Context, req openapiv1.ResumeTrainingSessionRequestObject) (openapiv1.ResumeTrainingSessionResponseObject, error) {
	owner, fail := s.trainerOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	if req.Body == nil {
		return trainerBadRequest, nil
	}
	cmd := service.TrainerTransitionCommand{
		SessionID: uuid.UUID(req.SessionId), ExpectedSessionRevision: req.Body.ExpectedSessionRevision,
		ExpectedSafetyRevision: req.Body.ExpectedSafetyRevision, IdempotencyKey: string(req.Body.IdempotencyKey),
	}
	view, err := s.trainerSessions.Resume(applicationRequestContext(ctx), owner, cmd)
	record, fail := trainerTransitionRecord(owner, cmd, view, err)
	if fail != nil {
		return *fail, nil
	}
	return openapiv1.ResumeTrainingSession200JSONResponse(record), nil
}

// trainerTransitionRecord maps a pause/resume result; the session must be the one
// commanded.
func trainerTransitionRecord(owner uuid.UUID, cmd service.TrainerTransitionCommand, view *service.TrainerSessionView, err error) (openapiv1.TrainingSessionRecord, *trainerHTTPError) {
	if err != nil {
		fail := trainerCommandError(err)
		return openapiv1.TrainingSessionRecord{}, &fail
	}
	if view == nil || view.Session.ID != cmd.SessionID {
		return openapiv1.TrainingSessionRecord{}, &trainerInconsistent
	}
	record, err := trainerSessionRecord(owner, *view)
	if err != nil {
		return openapiv1.TrainingSessionRecord{}, &trainerInconsistent
	}
	return record, nil
}

func (s *PublicServer) FinishTrainingSession(ctx context.Context, req openapiv1.FinishTrainingSessionRequestObject) (openapiv1.FinishTrainingSessionResponseObject, error) {
	owner, fail := s.trainerOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	if req.Body == nil {
		return trainerBadRequest, nil
	}
	cmd := service.FinishTrainerSessionCommand{
		SessionID: uuid.UUID(req.SessionId), EndedReason: string(req.Body.EndedReason),
		ExpectedSessionRevision: req.Body.ExpectedSessionRevision, IdempotencyKey: string(req.Body.IdempotencyKey),
	}
	view, err := s.trainerSessions.Finish(applicationRequestContext(ctx), owner, cmd)
	if err != nil {
		return trainerCommandError(err), nil
	}
	if view == nil || view.Session.ID != cmd.SessionID || view.Session.State != service.TrainerSessionEnded {
		return trainerInconsistent, nil
	}
	record, err := trainerSessionRecord(owner, *view)
	if err != nil {
		return trainerInconsistent, nil
	}
	return openapiv1.FinishTrainingSession200JSONResponse(record), nil
}
