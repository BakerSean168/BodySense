package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type trainingApplication interface {
	ListPlans(context.Context, uuid.UUID) ([]model.TrainingPlan, error)
	GetPlan(context.Context, uuid.UUID, uuid.UUID) (*model.TrainingPlan, error)
	GetTodayTask(context.Context, uuid.UUID, uuid.UUID) (*model.TrainingLog, error)
	CheckIn(context.Context, uuid.UUID, uuid.UUID) error
	UpdateLogWithFeedback(context.Context, uuid.UUID, uuid.UUID, service.TrainingFeedbackInput) (map[string]any, error)
	GetProgress(context.Context, uuid.UUID, uuid.UUID) (map[string]any, error)
	Reassess(context.Context, uuid.UUID, uuid.UUID, service.TrainingFeedbackInput) (map[string]any, error)
}

func (s *PublicServer) WithTraining(training trainingApplication) *PublicServer {
	s.training = training
	return s
}

type trainingHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

func (r trainingHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}
func (r trainingHTTPErrorResponse) VisitListTrainingPlansResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingHTTPErrorResponse) VisitGetTrainingPlanResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingHTTPErrorResponse) VisitGetTrainingTodayTaskResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingHTTPErrorResponse) VisitCheckInTrainingPlanResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingHTTPErrorResponse) VisitUpdateTrainingLogResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingHTTPErrorResponse) VisitGetTrainingProgressResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingHTTPErrorResponse) VisitReassessTrainingPlanResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func trainingUnauthorized() trainingHTTPErrorResponse {
	return trainingHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}
}
func trainingUnavailable() trainingHTTPErrorResponse {
	return trainingHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TRAINING_DOMAIN_UNAVAILABLE", message: "training service is not configured"}
}
func trainingError(err error, fallbackStatus int, fallbackCode string) trainingHTTPErrorResponse {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "plan not found") {
		return trainingHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "training plan not found"}
	}
	message := "training operation failed"
	if err != nil {
		message = err.Error()
	}
	return trainingHTTPErrorResponse{status: fallbackStatus, code: fallbackCode, message: message}
}

func (s *PublicServer) ListTrainingPlans(ctx context.Context, _ openapiv1.ListTrainingPlansRequestObject) (openapiv1.ListTrainingPlansResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return trainingUnauthorized(), nil
	}
	if s.training == nil {
		return trainingUnavailable(), nil
	}
	plans, err := s.training.ListPlans(ctx, uid)
	if err != nil {
		return trainingError(err, http.StatusInternalServerError, "INTERNAL_ERROR"), nil
	}
	public := make([]openapiv1.TrainingPlan, 0, len(plans))
	for i := range plans {
		item, convertErr := strictTrainingPlan(&plans[i])
		if convertErr != nil {
			return nil, convertErr
		}
		public = append(public, *item)
	}
	return openapiv1.ListTrainingPlans200JSONResponse{Plans: public}, nil
}

func (s *PublicServer) GetTrainingPlan(ctx context.Context, request openapiv1.GetTrainingPlanRequestObject) (openapiv1.GetTrainingPlanResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return trainingUnauthorized(), nil
	}
	if s.training == nil {
		return trainingUnavailable(), nil
	}
	plan, err := s.training.GetPlan(ctx, request.Id, uid)
	if err != nil {
		return trainingError(err, http.StatusNotFound, "NOT_FOUND"), nil
	}
	if plan == nil {
		return trainingHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "training plan not found"}, nil
	}
	public, err := strictTrainingPlan(plan)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetTrainingPlan200JSONResponse(*public), nil
}

func (s *PublicServer) GetTrainingTodayTask(ctx context.Context, request openapiv1.GetTrainingTodayTaskRequestObject) (openapiv1.GetTrainingTodayTaskResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return trainingUnauthorized(), nil
	}
	if s.training == nil {
		return trainingUnavailable(), nil
	}
	logEntry, err := s.training.GetTodayTask(ctx, request.Id, uid)
	if err != nil {
		return trainingError(err, http.StatusNotFound, "NOT_FOUND"), nil
	}
	public, err := strictTrainingLog(logEntry)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetTrainingTodayTask200JSONResponse(*public), nil
}

func (s *PublicServer) CheckInTrainingPlan(ctx context.Context, request openapiv1.CheckInTrainingPlanRequestObject) (openapiv1.CheckInTrainingPlanResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return trainingUnauthorized(), nil
	}
	if s.training == nil {
		return trainingUnavailable(), nil
	}
	if err := s.training.CheckIn(ctx, request.Id, uid); err != nil {
		return trainingError(err, http.StatusInternalServerError, "CHECKIN_FAILED"), nil
	}
	return openapiv1.CheckInTrainingPlan200JSONResponse{Message: "checked in"}, nil
}

func (s *PublicServer) UpdateTrainingLog(ctx context.Context, request openapiv1.UpdateTrainingLogRequestObject) (openapiv1.UpdateTrainingLogResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return trainingUnauthorized(), nil
	}
	if s.training == nil || request.Body == nil {
		return trainingUnavailable(), nil
	}
	feedback := mapTrainingLogFeedback(*request.Body)
	result, err := s.training.UpdateLogWithFeedback(ctx, request.Id, uid, feedback)
	if err != nil {
		return trainingError(err, http.StatusInternalServerError, "UPDATE_FAILED"), nil
	}
	publicResult, err := strictTrainingFeedbackResult(result)
	if err != nil {
		return nil, err
	}
	hasProposal := boolFromAny(result["has_proposal"])
	response := openapiv1.TrainingLogUpdateResponse{Message: "training log updated", HasProposal: hasProposal, Result: publicResult}
	if proposal, ok := result["proposal"].(*model.TreatmentRevision); ok && proposal != nil {
		publicProposal, convertErr := strictTreatmentRevision(proposal)
		if convertErr != nil {
			return nil, convertErr
		}
		response.Proposal = &publicProposal
	}
	return openapiv1.UpdateTrainingLog200JSONResponse(response), nil
}

func (s *PublicServer) GetTrainingProgress(ctx context.Context, request openapiv1.GetTrainingProgressRequestObject) (openapiv1.GetTrainingProgressResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return trainingUnauthorized(), nil
	}
	if s.training == nil {
		return trainingUnavailable(), nil
	}
	progress, err := s.training.GetProgress(ctx, request.Id, uid)
	if err != nil {
		return trainingError(err, http.StatusNotFound, "NOT_FOUND"), nil
	}
	body, err := strictOpenAPIConvert[openapiv1.TrainingProgress]("TrainingProgress", progress)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetTrainingProgress200JSONResponse(body), nil
}

func (s *PublicServer) ReassessTrainingPlan(ctx context.Context, request openapiv1.ReassessTrainingPlanRequestObject) (openapiv1.ReassessTrainingPlanResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return trainingUnauthorized(), nil
	}
	if s.training == nil || request.Body == nil {
		return trainingUnavailable(), nil
	}
	feedback := mapTrainingFeedback(request.Body.Feedback)
	result, err := s.training.Reassess(ctx, request.Id, uid, feedback)
	if err != nil {
		return trainingError(err, http.StatusBadRequest, "REASSESSMENT_FAILED"), nil
	}
	publicResult, err := strictTrainingFeedbackResult(result)
	if err != nil {
		return nil, err
	}
	return openapiv1.ReassessTrainingPlan200JSONResponse(publicResult), nil
}

func strictTrainingLog(value *model.TrainingLog) (*openapiv1.TrainingLog, error) {
	if value == nil {
		return nil, nil
	}
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	delete(projected, "user_id")
	converted, err := strictOpenAPIConvert[openapiv1.TrainingLog]("TrainingLog", projected)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func mapTrainingLogFeedback(body openapiv1.TrainingLogUpdateRequest) service.TrainingFeedbackInput {
	feedback := service.TrainingFeedbackInput{
		Notes:           stringOrEmpty(body.Notes),
		SymptomChanges:  stringOrEmpty(body.SymptomChanges),
		TrainingFeeling: stringOrEmpty(body.TrainingFeeling),
		Difficulties:    stringOrEmpty(body.Difficulties),
		BodyRegion:      stringOrEmpty(body.BodyRegion),
		ConcernKey:      stringOrEmpty(body.ConcernKey),
		Trend:           stringOrEmpty(body.Trend),
	}
	if body.FactId != nil {
		value := uuid.UUID(*body.FactId)
		feedback.FactID = &value
	}
	if body.Exercises != nil {
		items := mapTrainingExercises(*body.Exercises)
		feedback.Exercises = &items
	}
	return feedback
}

func mapTrainingFeedback(body openapiv1.TrainingFeedbackInput) service.TrainingFeedbackInput {
	feedback := service.TrainingFeedbackInput{
		SymptomChanges:  stringOrEmpty(body.SymptomChanges),
		TrainingFeeling: stringOrEmpty(body.TrainingFeeling),
		Difficulties:    stringOrEmpty(body.Difficulties),
		BodyRegion:      stringOrEmpty(body.BodyRegion),
		ConcernKey:      stringOrEmpty(body.ConcernKey),
		Trend:           stringOrEmpty(body.Trend),
	}
	if body.FactId != nil {
		value := uuid.UUID(*body.FactId)
		feedback.FactID = &value
	}
	return feedback
}

func mapTrainingExercises(items []openapiv1.TrainingExerciseLogInput) []service.TrainingExerciseLogInput {
	result := make([]service.TrainingExerciseLogInput, 0, len(items))
	for _, item := range items {
		mapped := service.TrainingExerciseLogInput{Name: item.Name, Completed: item.Completed}
		if item.InterventionId != nil {
			value := uuid.UUID(*item.InterventionId)
			mapped.InterventionID = &value
		}
		result = append(result, mapped)
	}
	return result
}

func strictTrainingFeedbackResult(result map[string]any) (openapiv1.TrainingFeedbackResult, error) {
	projected, err := marshalPublicMap(result)
	if err != nil {
		return openapiv1.TrainingFeedbackResult{}, err
	}
	if outcome, ok := projected["outcome"].(map[string]any); ok {
		sanitizeOutcomeMap(outcome)
	}
	if proposal, ok := projected["proposal"].(map[string]any); ok {
		sanitizeTreatmentRevisionMap(proposal)
	}
	return strictOpenAPIConvert[openapiv1.TrainingFeedbackResult]("TrainingFeedbackResult", projected)
}

func boolFromAny(value any) bool {
	result, _ := value.(bool)
	return result
}
