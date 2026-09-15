package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type treatmentApplication interface {
	GenerateProposalForLatest(context.Context, uuid.UUID, map[string]any, string) (*model.TreatmentRevision, error)
	GenerateProposal(context.Context, uuid.UUID, service.TreatmentProposalInput) (*model.TreatmentRevision, error)
	PreviewCurrentReview(context.Context, uuid.UUID) (*model.Treatment, error)
	EvaluateCurrentReview(context.Context, uuid.UUID) (*model.Treatment, error)
	ListRevisions(context.Context, uuid.UUID, int) ([]model.TreatmentRevision, error)
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.TreatmentRevision, error)
	RejectProposal(context.Context, uuid.UUID, uuid.UUID) error
	RecordOutcome(context.Context, uuid.UUID, model.Outcome) (*model.Outcome, bool, error)
	ListOutcomes(context.Context, uuid.UUID, int) ([]model.Outcome, error)
}

type treatmentTrainingApplication interface {
	AcceptTreatmentAndEnsurePlan(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (*model.Treatment, *model.TrainingPlan, error)
}

type treatmentReplayApplication interface {
	HistoricalReplay(context.Context, uuid.UUID, uuid.UUID) (*service.TreatmentReplayReport, error)
	CounterfactualReplay(context.Context, uuid.UUID, uuid.UUID, string) (*service.TreatmentReplayReport, error)
	ExportRegressionCase(context.Context, uuid.UUID, uuid.UUID) (map[string]any, error)
}

func (s *PublicServer) WithTreatment(
	treatments treatmentApplication,
	training treatmentTrainingApplication,
	replay treatmentReplayApplication,
) *PublicServer {
	s.treatments = treatments
	s.treatmentTraining = training
	s.treatmentReplay = replay
	return s
}

type treatmentHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

func (r treatmentHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}

func (r treatmentHTTPErrorResponse) VisitGenerateTreatmentProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitGetCurrentTreatmentResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitReviewCurrentTreatmentResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitListTreatmentRevisionsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitGetTreatmentRevisionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitReplayTreatmentRevisionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitExportTreatmentRegressionCaseResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitAcceptTreatmentRevisionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitRejectTreatmentRevisionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitRecordOutcomeResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r treatmentHTTPErrorResponse) VisitListOutcomesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func treatmentUnauthorized() treatmentHTTPErrorResponse {
	return treatmentHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}
}

func treatmentInternal(message string) treatmentHTTPErrorResponse {
	return treatmentHTTPErrorResponse{status: http.StatusInternalServerError, code: "INTERNAL_ERROR", message: message}
}

func treatmentOperationHTTPError(err error) treatmentHTTPErrorResponse {
	switch {
	case errors.Is(err, service.ErrTreatmentDiagnosisNotReady):
		return treatmentHTTPErrorResponse{status: http.StatusConflict, code: "DIAGNOSIS_NOT_READY", message: err.Error()}
	case errors.Is(err, service.ErrTreatmentSafetyBlocked):
		return treatmentHTTPErrorResponse{status: http.StatusConflict, code: "TREATMENT_SAFETY_BLOCKED", message: err.Error()}
	case errors.Is(err, service.ErrTreatmentAnalysisStale):
		return treatmentHTTPErrorResponse{status: http.StatusConflict, code: "DIAGNOSIS_STALE", message: err.Error()}
	case errors.Is(err, service.ErrTreatmentCandidateAssessmentRequired):
		return treatmentHTTPErrorResponse{status: http.StatusConflict, code: "DIAGNOSIS_ASSESSMENT_REQUIRED", message: err.Error()}
	case errors.Is(err, service.ErrTreatmentProposalOutdated):
		return treatmentHTTPErrorResponse{status: http.StatusConflict, code: "TREATMENT_PROPOSAL_OUTDATED", message: err.Error()}
	case errors.Is(err, service.ErrTrainingProjectionFailed):
		return treatmentHTTPErrorResponse{status: http.StatusInternalServerError, code: "TRAINING_PROJECTION_FAILED", message: err.Error()}
	default:
		return treatmentHTTPErrorResponse{status: http.StatusBadRequest, code: "TREATMENT_OPERATION_FAILED", message: err.Error()}
	}
}

func treatmentReplayHTTPError(err error) treatmentHTTPErrorResponse {
	switch {
	case errors.Is(err, service.ErrTreatmentReplayNotFound):
		return treatmentHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "treatment revision not found"}
	case errors.Is(err, service.ErrTreatmentReplayUnavailable):
		return treatmentHTTPErrorResponse{status: http.StatusConflict, code: "TREATMENT_REPLAY_INPUT_UNAVAILABLE", message: "this historical revision predates frozen replay input"}
	case errors.Is(err, service.ErrTreatmentReplayConfiguration):
		return treatmentHTTPErrorResponse{status: http.StatusUnprocessableEntity, code: "UNKNOWN_AGENT_CONFIGURATION", message: err.Error()}
	default:
		return treatmentHTTPErrorResponse{status: http.StatusBadGateway, code: "TREATMENT_REPLAY_FAILED", message: "treatment replay failed"}
	}
}

func (s *PublicServer) GenerateTreatmentProposal(
	ctx context.Context,
	request openapiv1.GenerateTreatmentProposalRequestObject,
) (openapiv1.GenerateTreatmentProposalResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil || request.Body == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	constraints := map[string]any{}
	if request.Body.UserConstraints != nil {
		constraints = map[string]any(*request.Body.UserConstraints)
	}
	changeReason := ""
	if request.Body.ChangeReason != nil {
		changeReason = *request.Body.ChangeReason
	}
	var revision *model.TreatmentRevision
	if request.Body.DiagnosisAnalysisId == nil {
		revision, err = s.treatments.GenerateProposalForLatest(ctx, uid, constraints, changeReason)
	} else {
		revision, err = s.treatments.GenerateProposal(ctx, uid, service.TreatmentProposalInput{
			DiagnosisAnalysisID: *request.Body.DiagnosisAnalysisId,
			UserConstraints:     constraints,
			ChangeReason:        changeReason,
		})
	}
	if err != nil {
		return treatmentOperationHTTPError(err), nil
	}
	proposal, err := strictTreatmentRevision(revision)
	if err != nil {
		return nil, err
	}
	return openapiv1.GenerateTreatmentProposal201JSONResponse{Proposal: proposal}, nil
}

func (s *PublicServer) GetCurrentTreatment(
	ctx context.Context,
	_ openapiv1.GetCurrentTreatmentRequestObject,
) (openapiv1.GetCurrentTreatmentResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	current, err := s.treatments.PreviewCurrentReview(ctx, uid)
	if err != nil {
		return treatmentInternal("failed to load current treatment"), nil
	}
	projected, err := strictTreatment(current)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetCurrentTreatment200JSONResponse{Treatment: projected}, nil
}

func (s *PublicServer) ReviewCurrentTreatment(
	ctx context.Context,
	_ openapiv1.ReviewCurrentTreatmentRequestObject,
) (openapiv1.ReviewCurrentTreatmentResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	current, err := s.treatments.EvaluateCurrentReview(ctx, uid)
	if err != nil {
		return treatmentInternal("failed to review current treatment"), nil
	}
	projected, err := strictTreatment(current)
	if err != nil {
		return nil, err
	}
	return openapiv1.ReviewCurrentTreatment200JSONResponse{Treatment: projected}, nil
}

func (s *PublicServer) ListTreatmentRevisions(
	ctx context.Context,
	request openapiv1.ListTreatmentRevisionsRequestObject,
) (openapiv1.ListTreatmentRevisionsResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	revisions, err := s.treatments.ListRevisions(ctx, uid, limit)
	if err != nil {
		return treatmentInternal("failed to load treatment revisions"), nil
	}
	projected := make([]openapiv1.TreatmentRevision, 0, len(revisions))
	for i := range revisions {
		item, convertErr := strictTreatmentRevision(&revisions[i])
		if convertErr != nil {
			return nil, convertErr
		}
		projected = append(projected, item)
	}
	return openapiv1.ListTreatmentRevisions200JSONResponse{Revisions: projected}, nil
}

func (s *PublicServer) GetTreatmentRevision(
	ctx context.Context,
	request openapiv1.GetTreatmentRevisionRequestObject,
) (openapiv1.GetTreatmentRevisionResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	revision, err := s.treatments.GetRevision(ctx, uid, request.RevisionId)
	if err != nil {
		return treatmentInternal("failed to load treatment revision"), nil
	}
	if revision == nil {
		return treatmentHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "treatment revision not found"}, nil
	}
	projected, err := strictTreatmentRevision(revision)
	if err != nil {
		return nil, err
	}
	return openapiv1.GetTreatmentRevision200JSONResponse(projected), nil
}

func (s *PublicServer) ReplayTreatmentRevision(
	ctx context.Context,
	request openapiv1.ReplayTreatmentRevisionRequestObject,
) (openapiv1.ReplayTreatmentRevisionResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatmentReplay == nil || request.Body == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_REPLAY_UNAVAILABLE", message: "treatment replay service is not configured"}, nil
	}
	var report *service.TreatmentReplayReport
	switch string(request.Body.Mode) {
	case "historical":
		report, err = s.treatmentReplay.HistoricalReplay(ctx, uid, request.RevisionId)
	case "counterfactual":
		if request.Body.ConfigurationId == nil || strings.TrimSpace(*request.Body.ConfigurationId) == "" {
			return treatmentHTTPErrorResponse{status: http.StatusBadRequest, code: "CONFIGURATION_REQUIRED", message: "counterfactual replay requires configuration_id"}, nil
		}
		report, err = s.treatmentReplay.CounterfactualReplay(ctx, uid, request.RevisionId, *request.Body.ConfigurationId)
	default:
		return treatmentHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REPLAY_MODE", message: "replay mode must be historical or counterfactual"}, nil
	}
	if err != nil {
		return treatmentReplayHTTPError(err), nil
	}
	body, err := strictOpenAPIConvert[openapiv1.TreatmentReplayReport]("TreatmentReplayReport", report)
	if err != nil {
		return nil, err
	}
	return openapiv1.ReplayTreatmentRevision200JSONResponse(body), nil
}

func (s *PublicServer) ExportTreatmentRegressionCase(
	ctx context.Context,
	request openapiv1.ExportTreatmentRegressionCaseRequestObject,
) (openapiv1.ExportTreatmentRegressionCaseResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatmentReplay == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_REPLAY_UNAVAILABLE", message: "treatment replay service is not configured"}, nil
	}
	exported, err := s.treatmentReplay.ExportRegressionCase(ctx, uid, request.RevisionId)
	if err != nil {
		return treatmentReplayHTTPError(err), nil
	}
	return openapiv1.ExportTreatmentRegressionCase200JSONResponse(openapiv1.JsonObject(exported)), nil
}

func (s *PublicServer) AcceptTreatmentRevision(
	ctx context.Context,
	request openapiv1.AcceptTreatmentRevisionRequestObject,
) (openapiv1.AcceptTreatmentRevisionResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatmentTraining == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TRAINING_DOMAIN_UNAVAILABLE", message: "training service is not configured"}, nil
	}
	var consultationID *uuid.UUID
	if request.Body != nil && request.Body.ConsultationId != nil {
		value := uuid.UUID(*request.Body.ConsultationId)
		consultationID = &value
	}
	treatment, plan, err := s.treatmentTraining.AcceptTreatmentAndEnsurePlan(ctx, uid, request.RevisionId, consultationID)
	if err != nil {
		return treatmentOperationHTTPError(err), nil
	}
	publicTreatment, err := strictTreatment(treatment)
	if err != nil || publicTreatment == nil {
		return nil, err
	}
	publicPlan, err := strictTrainingPlan(plan)
	if err != nil || publicPlan == nil {
		return nil, err
	}
	return openapiv1.AcceptTreatmentRevision200JSONResponse{
		Treatment: *publicTreatment, TrainingPlan: *publicPlan,
	}, nil
}

func (s *PublicServer) RejectTreatmentRevision(
	ctx context.Context,
	request openapiv1.RejectTreatmentRevisionRequestObject,
) (openapiv1.RejectTreatmentRevisionResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	if err := s.treatments.RejectProposal(ctx, uid, request.RevisionId); err != nil {
		return treatmentOperationHTTPError(err), nil
	}
	return openapiv1.RejectTreatmentRevision204Response{}, nil
}

func (s *PublicServer) RecordOutcome(
	ctx context.Context,
	request openapiv1.RecordOutcomeRequestObject,
) (openapiv1.RecordOutcomeResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil || request.Body == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	occurredAt := time.Now().UTC()
	if request.Body.OccurredAt != nil {
		occurredAt = *request.Body.OccurredAt
	}
	value := datatypes.JSON([]byte(`{}`))
	if request.Body.Value != nil {
		value = jsonObjectBytes(*request.Body.Value)
	}
	provenance := datatypes.JSON([]byte(`{}`))
	if request.Body.Provenance != nil {
		provenance = jsonObjectBytes(*request.Body.Provenance)
	}
	outcome := model.Outcome{
		TreatmentID:          uuidPointer(request.Body.TreatmentId),
		TreatmentRevisionID:  uuidPointer(request.Body.TreatmentRevisionId),
		InterventionID:       uuidPointer(request.Body.InterventionId),
		SourceType:           request.Body.SourceType,
		SourceKey:            request.Body.SourceKey,
		Kind:                 request.Body.Kind,
		ConcernKey:           stringOrEmpty(request.Body.ConcernKey),
		BodyRegion:           stringOrEmpty(request.Body.BodyRegion),
		Value:                value,
		Notes:                stringOrEmpty(request.Body.Notes),
		AssociationStatement: stringOrEmpty(request.Body.AssociationStatement),
		CausalityLevel:       optionalEnumString(request.Body.CausalityLevel),
		OccurredAt:           occurredAt,
		Provenance:           provenance,
	}
	stored, created, err := s.treatments.RecordOutcome(ctx, uid, outcome)
	if err != nil {
		return treatmentOperationHTTPError(err), nil
	}
	publicOutcome, err := strictOutcome(stored)
	if err != nil || publicOutcome == nil {
		return nil, err
	}
	response := openapiv1.OutcomeMutationResponse{Outcome: *publicOutcome, Created: created}
	if created {
		return openapiv1.RecordOutcome201JSONResponse(response), nil
	}
	return openapiv1.RecordOutcome200JSONResponse(response), nil
}

func (s *PublicServer) ListOutcomes(
	ctx context.Context,
	request openapiv1.ListOutcomesRequestObject,
) (openapiv1.ListOutcomesResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return treatmentUnauthorized(), nil
	}
	if s.treatments == nil {
		return treatmentHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "TREATMENT_DOMAIN_UNAVAILABLE", message: "treatment service is not configured"}, nil
	}
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	items, err := s.treatments.ListOutcomes(ctx, uid, limit)
	if err != nil {
		return treatmentInternal("failed to load outcomes"), nil
	}
	publicItems := make([]openapiv1.Outcome, 0, len(items))
	for i := range items {
		item, convertErr := strictOutcome(&items[i])
		if convertErr != nil {
			return nil, convertErr
		}
		publicItems = append(publicItems, *item)
	}
	return openapiv1.ListOutcomes200JSONResponse{Outcomes: publicItems}, nil
}

func strictTreatment(value *model.Treatment) (*openapiv1.Treatment, error) {
	if value == nil {
		return nil, nil
	}
	projected, err := publicTreatmentMap(value)
	if err != nil {
		return nil, err
	}
	converted, err := strictOpenAPIConvert[openapiv1.Treatment]("Treatment", projected)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func strictTreatmentRevision(value *model.TreatmentRevision) (openapiv1.TreatmentRevision, error) {
	projected, err := publicTreatmentRevisionMap(value)
	if err != nil {
		return openapiv1.TreatmentRevision{}, err
	}
	return strictOpenAPIConvert[openapiv1.TreatmentRevision]("TreatmentRevision", projected)
}

func strictTrainingPlan(value *model.TrainingPlan) (*openapiv1.TrainingPlan, error) {
	if value == nil {
		return nil, nil
	}
	projected, err := publicTrainingPlanMap(value)
	if err != nil {
		return nil, err
	}
	converted, err := strictOpenAPIConvert[openapiv1.TrainingPlan]("TrainingPlan", projected)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func strictOutcome(value *model.Outcome) (*openapiv1.Outcome, error) {
	if value == nil {
		return nil, nil
	}
	projected, err := publicOutcomeMap(value)
	if err != nil {
		return nil, err
	}
	converted, err := strictOpenAPIConvert[openapiv1.Outcome]("Outcome", projected)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func publicTreatmentMap(value *model.Treatment) (map[string]any, error) {
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	sanitizeTreatmentMap(projected)
	return projected, nil
}

func sanitizeTreatmentMap(projected map[string]any) {
	delete(projected, "user_id")
	if current, ok := projected["current"].(map[string]any); ok {
		sanitizeTreatmentRevisionMap(current)
	}
}

func publicTreatmentRevisionMap(value *model.TreatmentRevision) (map[string]any, error) {
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	sanitizeTreatmentRevisionMap(projected)
	return projected, nil
}

func sanitizeTreatmentRevisionMap(projected map[string]any) {
	if interventions, ok := projected["interventions"].([]any); ok {
		for _, raw := range interventions {
			if intervention, ok := raw.(map[string]any); ok {
				delete(intervention, "user_id")
			}
		}
	}
}

func publicTrainingPlanMap(value *model.TrainingPlan) (map[string]any, error) {
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	sanitizeTrainingPlanMap(projected)
	return projected, nil
}

func sanitizeTrainingPlanMap(projected map[string]any) {
	delete(projected, "user_id")
}

func publicOutcomeMap(value *model.Outcome) (map[string]any, error) {
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	sanitizeOutcomeMap(projected)
	return projected, nil
}

func sanitizeOutcomeMap(projected map[string]any) {
	delete(projected, "user_id")
}

func marshalPublicMap(value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return nil, err
	}
	return projected, nil
}

func jsonObjectBytes(value openapiv1.JsonObject) datatypes.JSON {
	encoded, _ := json.Marshal(value)
	return datatypes.JSON(encoded)
}

func uuidPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalEnumString[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
