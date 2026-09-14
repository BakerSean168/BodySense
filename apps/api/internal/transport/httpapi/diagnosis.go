package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type diagnosisApplication interface {
	Analyze(context.Context, uuid.UUID, uuid.UUID) (map[string]any, *service.DiagnosisApplicationError)
}

type diagnosisAnalysisApplication interface {
	List(context.Context, uuid.UUID, int) ([]model.DiagnosisAnalysisRecord, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	ListAssessments(context.Context, uuid.UUID, uuid.UUID) ([]model.DiagnosisCandidateAssessment, error)
	AssessCandidates(context.Context, uuid.UUID, uuid.UUID, map[uuid.UUID]string) ([]model.DiagnosisCandidateAssessment, error)
	PublicPayload(*model.DiagnosisAnalysisRecord) map[string]any
}

type diagnosisFreshnessApplication interface {
	PreviewMany(context.Context, uuid.UUID, []model.DiagnosisAnalysisRecord) (map[uuid.UUID]model.DiagnosisAnalysisFreshness, error)
	Preview(context.Context, uuid.UUID, *model.DiagnosisAnalysisRecord) (*model.DiagnosisAnalysisFreshness, error)
}

type diagnosisReplayApplication interface {
	HistoricalReplay(context.Context, uuid.UUID, uuid.UUID) (*service.DiagnosisReplayReport, error)
	CounterfactualReplay(context.Context, uuid.UUID, uuid.UUID, string) (*service.DiagnosisReplayReport, error)
	ExportRegressionCase(context.Context, uuid.UUID, uuid.UUID) (map[string]any, error)
}

func (s *PublicServer) WithDiagnosis(
	application diagnosisApplication,
	analyses diagnosisAnalysisApplication,
	freshness diagnosisFreshnessApplication,
	replay diagnosisReplayApplication,
) *PublicServer {
	s.diagnosisApplication = application
	s.diagnosisAnalyses = analyses
	s.diagnosisFreshness = freshness
	s.diagnosisReplay = replay
	return s
}

type diagnosisHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

func (r diagnosisHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}

func (r diagnosisHTTPErrorResponse) VisitAnalyzeDiagnosisResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitListDiagnosisAnalysesResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitGetDiagnosisAnalysisResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitAssessDiagnosisCandidatesResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitReplayDiagnosisAnalysisResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitExportDiagnosisRegressionCaseResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func diagnosisUnauthorized() diagnosisHTTPErrorResponse {
	return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}
}

func diagnosisInternal(message string) diagnosisHTTPErrorResponse {
	return diagnosisHTTPErrorResponse{status: http.StatusInternalServerError, code: "INTERNAL_ERROR", message: message}
}

func diagnosisApplicationHTTPError(err *service.DiagnosisApplicationError) diagnosisHTTPErrorResponse {
	if err == nil {
		return diagnosisInternal("diagnosis failed")
	}
	status := http.StatusInternalServerError
	switch err.Code {
	case "NOT_FOUND":
		status = http.StatusNotFound
	case "BODY_STATE_NOT_READY":
		status = http.StatusConflict
	case "AI_SERVICE_ERROR", "INVALID_AI_RESPONSE", "INVALID_AGENT_CONFIGURATION":
		status = http.StatusBadGateway
	case "DIAGNOSIS_DOMAIN_UNAVAILABLE", "AGENT_DEPLOYMENT_POLICY_UNAVAILABLE":
		status = http.StatusServiceUnavailable
	}
	return diagnosisHTTPErrorResponse{status: status, code: err.Code, message: err.Message}
}

func (s *PublicServer) AnalyzeDiagnosis(
	ctx context.Context,
	request openapiv1.AnalyzeDiagnosisRequestObject,
) (openapiv1.AnalyzeDiagnosisResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return diagnosisUnauthorized(), nil
	}
	if s.diagnosisApplication == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_DOMAIN_UNAVAILABLE", message: "BodyState-backed diagnosis services are not configured"}, nil
	}
	payload, appErr := s.diagnosisApplication.Analyze(ctx, uid, request.Id)
	if appErr != nil {
		return diagnosisApplicationHTTPError(appErr), nil
	}
	return openapiv1.AnalyzeDiagnosis200JSONResponse(projectDiagnosisPayload(payload)), nil
}

func (s *PublicServer) ListDiagnosisAnalyses(
	ctx context.Context,
	request openapiv1.ListDiagnosisAnalysesRequestObject,
) (openapiv1.ListDiagnosisAnalysesResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return diagnosisUnauthorized(), nil
	}
	if s.diagnosisAnalyses == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_HISTORY_UNAVAILABLE", message: "diagnosis history is not configured"}, nil
	}
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	analyses, err := s.diagnosisAnalyses.List(ctx, uid, limit)
	if err != nil {
		return diagnosisInternal("failed to load diagnosis history"), nil
	}
	freshnessByID := map[uuid.UUID]model.DiagnosisAnalysisFreshness{}
	if s.diagnosisFreshness != nil {
		if values, freshnessErr := s.diagnosisFreshness.PreviewMany(ctx, uid, analyses); freshnessErr == nil {
			freshnessByID = values
		}
	}
	items := make([]openapiv1.DiagnosisWorkspaceProjection, 0, len(analyses))
	for i := range analyses {
		payload := s.diagnosisAnalyses.PublicPayload(&analyses[i])
		if freshness, ok := freshnessByID[analyses[i].ID]; ok {
			payload["freshness"] = freshness
		}
		item, convertErr := strictDiagnosisProjection(payload)
		if convertErr != nil {
			return nil, convertErr
		}
		items = append(items, item)
	}
	return openapiv1.ListDiagnosisAnalyses200JSONResponse{Analyses: items}, nil
}

func (s *PublicServer) GetDiagnosisAnalysis(
	ctx context.Context,
	request openapiv1.GetDiagnosisAnalysisRequestObject,
) (openapiv1.GetDiagnosisAnalysisResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return diagnosisUnauthorized(), nil
	}
	if s.diagnosisAnalyses == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_HISTORY_UNAVAILABLE", message: "diagnosis history is not configured"}, nil
	}
	analysis, err := s.diagnosisAnalyses.GetByID(ctx, request.AnalysisId, uid)
	if err != nil {
		return diagnosisInternal("failed to load diagnosis analysis"), nil
	}
	if analysis == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "diagnosis analysis not found"}, nil
	}
	assessments, err := s.diagnosisAnalyses.ListAssessments(ctx, uid, request.AnalysisId)
	if err != nil {
		return diagnosisInternal("failed to load candidate assessments"), nil
	}
	payload := s.diagnosisAnalyses.PublicPayload(analysis)
	payload["candidate_assessments"] = assessments
	if s.diagnosisFreshness != nil {
		if freshness, freshnessErr := s.diagnosisFreshness.Preview(ctx, uid, analysis); freshnessErr == nil {
			payload["freshness"] = freshness
		}
	}
	projection, convertErr := strictDiagnosisProjection(payload)
	if convertErr != nil {
		return nil, convertErr
	}
	return openapiv1.GetDiagnosisAnalysis200JSONResponse(projection), nil
}

func (s *PublicServer) AssessDiagnosisCandidates(
	ctx context.Context,
	request openapiv1.AssessDiagnosisCandidatesRequestObject,
) (openapiv1.AssessDiagnosisCandidatesResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return diagnosisUnauthorized(), nil
	}
	if s.diagnosisAnalyses == nil || request.Body == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_HISTORY_UNAVAILABLE", message: "diagnosis history is not configured"}, nil
	}
	states := make(map[uuid.UUID]string, len(request.Body.Candidates))
	for _, item := range request.Body.Candidates {
		states[item.CandidateId] = string(item.State)
	}
	assessments, err := s.diagnosisAnalyses.AssessCandidates(ctx, uid, request.AnalysisId, states)
	if err != nil {
		status := http.StatusBadRequest
		code := "INVALID_CANDIDATE_ASSESSMENT"
		if strings.Contains(err.Error(), "diagnosis analysis not found") {
			status = http.StatusNotFound
			code = "NOT_FOUND"
		}
		return diagnosisHTTPErrorResponse{status: status, code: code, message: err.Error()}, nil
	}
	publicAssessments, convertErr := strictDiagnosisAssessments(assessments)
	if convertErr != nil {
		return nil, convertErr
	}
	return openapiv1.AssessDiagnosisCandidates200JSONResponse{
		AnalysisId: request.AnalysisId, CandidateAssessments: publicAssessments,
	}, nil
}

func (s *PublicServer) ReplayDiagnosisAnalysis(
	ctx context.Context,
	request openapiv1.ReplayDiagnosisAnalysisRequestObject,
) (openapiv1.ReplayDiagnosisAnalysisResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return diagnosisUnauthorized(), nil
	}
	if s.diagnosisReplay == nil || request.Body == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_REPLAY_UNAVAILABLE", message: "diagnosis replay is not configured"}, nil
	}
	var report *service.DiagnosisReplayReport
	switch string(request.Body.Mode) {
	case "historical":
		report, err = s.diagnosisReplay.HistoricalReplay(ctx, uid, request.AnalysisId)
	case "counterfactual":
		if request.Body.ConfigurationId == nil || strings.TrimSpace(*request.Body.ConfigurationId) == "" {
			return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "CONFIGURATION_REQUIRED", message: "counterfactual replay requires configuration_id"}, nil
		}
		report, err = s.diagnosisReplay.CounterfactualReplay(ctx, uid, request.AnalysisId, *request.Body.ConfigurationId)
	default:
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REPLAY_MODE", message: "replay mode must be historical or counterfactual"}, nil
	}
	if err != nil {
		return diagnosisReplayHTTPError(err), nil
	}
	body, convertErr := strictOpenAPIConvert[openapiv1.DiagnosisReplayReport]("DiagnosisReplayReport", report)
	if convertErr != nil {
		return nil, convertErr
	}
	return openapiv1.ReplayDiagnosisAnalysis200JSONResponse(body), nil
}

func (s *PublicServer) ExportDiagnosisRegressionCase(
	ctx context.Context,
	request openapiv1.ExportDiagnosisRegressionCaseRequestObject,
) (openapiv1.ExportDiagnosisRegressionCaseResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return diagnosisUnauthorized(), nil
	}
	if s.diagnosisReplay == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_REPLAY_UNAVAILABLE", message: "diagnosis replay is not configured"}, nil
	}
	exported, err := s.diagnosisReplay.ExportRegressionCase(ctx, uid, request.AnalysisId)
	if err != nil {
		return diagnosisReplayHTTPError(err), nil
	}
	return openapiv1.ExportDiagnosisRegressionCase200JSONResponse(projectDiagnosisPayload(exported)), nil
}

func diagnosisReplayHTTPError(err error) diagnosisHTTPErrorResponse {
	switch {
	case errors.Is(err, service.ErrDiagnosisReplayNotFound):
		return diagnosisHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "diagnosis analysis not found"}
	case errors.Is(err, service.ErrDiagnosisReplayUnavailable):
		return diagnosisHTTPErrorResponse{status: http.StatusConflict, code: "DIAGNOSIS_REPLAY_INPUT_UNAVAILABLE", message: "this historical analysis predates frozen replay input"}
	case strings.Contains(err.Error(), "unknown Diagnosis Agent configuration id"):
		return diagnosisHTTPErrorResponse{status: http.StatusUnprocessableEntity, code: "UNKNOWN_AGENT_CONFIGURATION", message: err.Error()}
	default:
		return diagnosisHTTPErrorResponse{status: http.StatusBadGateway, code: "DIAGNOSIS_REPLAY_FAILED", message: "diagnosis replay failed"}
	}
}

func strictDiagnosisProjection(payload map[string]any) (openapiv1.DiagnosisWorkspaceProjection, error) {
	return strictOpenAPIConvert[openapiv1.DiagnosisWorkspaceProjection]("DiagnosisWorkspaceProjection", projectDiagnosisPayload(payload))
}

func strictDiagnosisAssessments(items []model.DiagnosisCandidateAssessment) ([]openapiv1.DiagnosisCandidateAssessment, error) {
	public := make([]openapiv1.DiagnosisCandidateAssessment, 0, len(items))
	for _, item := range items {
		projected, err := strictOpenAPIConvert[openapiv1.DiagnosisCandidateAssessment](
			"DiagnosisCandidateAssessment",
			map[string]any{
				"id": item.ID, "analysis_id": item.AnalysisID, "candidate_id": item.CandidateID,
				"state": item.State, "assessed_at": item.AssessedAt,
			},
		)
		if err != nil {
			return nil, err
		}
		public = append(public, projected)
	}
	return public, nil
}

// projectDiagnosisPayload removes persistence-only user identity from nested
// read-state while preserving the established public analysis payload.
func projectDiagnosisPayload(payload map[string]any) openapiv1.JsonObject {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return openapiv1.JsonObject{}
	}
	var projected map[string]any
	if json.Unmarshal(encoded, &projected) != nil {
		return openapiv1.JsonObject{}
	}
	if freshness, ok := projected["freshness"].(map[string]any); ok {
		delete(freshness, "user_id")
	}
	if assessments, ok := projected["candidate_assessments"].([]any); ok {
		for _, raw := range assessments {
			if assessment, ok := raw.(map[string]any); ok {
				delete(assessment, "user_id")
			}
		}
	}
	return openapiv1.JsonObject(projected)
}
