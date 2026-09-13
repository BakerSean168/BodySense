package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type assessmentApplication interface {
	GenerateAssessment(context.Context, uuid.UUID) (*model.AssessmentReport, error)
	GetReport(context.Context, uuid.UUID, uuid.UUID) (*model.AssessmentReport, error)
	ListReports(context.Context, uuid.UUID, int, int) ([]model.AssessmentReport, int64, error)
}

type assessmentReplayApplication interface {
	HistoricalReplay(context.Context, uuid.UUID, uuid.UUID) (*service.AssessmentReplayReport, error)
	CounterfactualReplay(context.Context, uuid.UUID, uuid.UUID, string) (*service.AssessmentReplayReport, error)
	ExportRegressionCase(context.Context, uuid.UUID, uuid.UUID) (map[string]any, error)
}

func (s *PublicServer) WithAssessment(
	assessment assessmentApplication,
	replay assessmentReplayApplication,
) *PublicServer {
	s.assessment = assessment
	s.assessmentReplay = replay
	return s
}

func (s *PublicServer) GenerateAssessment(
	ctx context.Context,
	_ openapiv1.GenerateAssessmentRequestObject,
) (openapiv1.GenerateAssessmentResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return generateAssessmentError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.assessment == nil {
		return generateAssessmentError(http.StatusInternalServerError, "ASSESSMENT_UNAVAILABLE", "assessment service is not configured"), nil
	}
	report, err := s.assessment.GenerateAssessment(ctx, userID)
	if err != nil {
		if errors.Is(err, service.ErrAssessmentOutputRejected) {
			return generateAssessmentError(http.StatusBadGateway, "ASSESSMENT_OUTPUT_REJECTED", "assessment generation did not pass evidence validation"), nil
		}
		return generateAssessmentError(http.StatusInternalServerError, "ASSESSMENT_GENERATION_FAILED", "failed to generate assessment"), nil
	}
	if report == nil {
		return generateAssessmentError(http.StatusInternalServerError, "ASSESSMENT_GENERATION_FAILED", "assessment service returned no report"), nil
	}
	if report.ContractRevision != "assessment-output-v2" {
		return generateAssessmentError(http.StatusInternalServerError, "INTERNAL_ERROR", "generated assessment did not use the v2 public contract"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.AssessmentReportV2]("AssessmentReportV2", report)
	if err != nil {
		return generateAssessmentError(http.StatusInternalServerError, "INTERNAL_ERROR", "assessment report violates the public contract"), nil
	}
	return openapiv1.GenerateAssessment201JSONResponse(response), nil
}

func (s *PublicServer) ListAssessments(
	ctx context.Context,
	request openapiv1.ListAssessmentsRequestObject,
) (openapiv1.ListAssessmentsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return listAssessmentsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.assessment == nil {
		return listAssessmentsError(http.StatusInternalServerError, "ASSESSMENT_UNAVAILABLE", "assessment service is not configured"), nil
	}
	limit := 20
	offset := 0
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if request.Params.Offset != nil {
		offset = *request.Params.Offset
	}
	reports, total, err := s.assessment.ListReports(ctx, userID, limit, offset)
	if err != nil {
		return listAssessmentsError(http.StatusInternalServerError, "ASSESSMENT_LIST_FAILED", "failed to list assessments"), nil
	}
	projected := make([]openapiv1.AssessmentReport, 0, len(reports))
	for index := range reports {
		item, err := assessmentReportToOpenAPI(&reports[index])
		if err != nil {
			return listAssessmentsError(http.StatusInternalServerError, "INTERNAL_ERROR", "assessment report violates the public contract"), nil
		}
		projected = append(projected, item)
	}
	return openapiv1.ListAssessments200JSONResponse{
		Reports: projected,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

func (s *PublicServer) GetAssessment(
	ctx context.Context,
	request openapiv1.GetAssessmentRequestObject,
) (openapiv1.GetAssessmentResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getAssessmentError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.assessment == nil {
		return getAssessmentError(http.StatusInternalServerError, "ASSESSMENT_UNAVAILABLE", "assessment service is not configured"), nil
	}
	report, err := s.assessment.GetReport(ctx, request.Id, userID)
	if err != nil {
		return getAssessmentError(http.StatusInternalServerError, "ASSESSMENT_READ_FAILED", "failed to get assessment"), nil
	}
	if report == nil {
		return getAssessmentError(http.StatusNotFound, "ASSESSMENT_NOT_FOUND", "assessment report not found"), nil
	}
	response, err := assessmentReportToOpenAPI(report)
	if err != nil {
		return getAssessmentError(http.StatusInternalServerError, "INTERNAL_ERROR", "assessment report violates the public contract"), nil
	}
	return openapiv1.GetAssessment200JSONResponse(response), nil
}

func (s *PublicServer) ReplayAssessment(
	ctx context.Context,
	request openapiv1.ReplayAssessmentRequestObject,
) (openapiv1.ReplayAssessmentResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return replayAssessmentError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.assessmentReplay == nil {
		return replayAssessmentError(http.StatusServiceUnavailable, "ASSESSMENT_REPLAY_UNAVAILABLE", "assessment replay is not configured"), nil
	}
	if request.Body == nil {
		return replayAssessmentError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}

	var report *service.AssessmentReplayReport
	switch request.Body.Mode {
	case openapiv1.AssessmentReplayRequestModeHistorical:
		report, err = s.assessmentReplay.HistoricalReplay(ctx, userID, request.Id)
	case openapiv1.AssessmentReplayRequestModeCounterfactual:
		configurationID := ""
		if request.Body.ConfigurationId != nil {
			configurationID = strings.TrimSpace(*request.Body.ConfigurationId)
		}
		if configurationID == "" {
			return replayAssessmentError(http.StatusBadRequest, "INVALID_REQUEST", "counterfactual replay requires configuration_id"), nil
		}
		report, err = s.assessmentReplay.CounterfactualReplay(ctx, userID, request.Id, configurationID)
	default:
		return replayAssessmentError(http.StatusBadRequest, "INVALID_REQUEST", "replay mode must be historical or counterfactual"), nil
	}
	if err != nil {
		return assessmentReplayFailure(err), nil
	}
	if report == nil {
		return replayAssessmentError(http.StatusInternalServerError, "ASSESSMENT_REPLAY_FAILED", "assessment replay returned no report"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.AssessmentReplayReport]("AssessmentReplayReport", report)
	if err != nil {
		return replayAssessmentError(http.StatusInternalServerError, "INTERNAL_ERROR", "assessment replay violates the public contract"), nil
	}
	return openapiv1.ReplayAssessment200JSONResponse(response), nil
}

func (s *PublicServer) ExportAssessmentRegressionCase(
	ctx context.Context,
	request openapiv1.ExportAssessmentRegressionCaseRequestObject,
) (openapiv1.ExportAssessmentRegressionCaseResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return exportAssessmentError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.assessmentReplay == nil {
		return exportAssessmentError(http.StatusServiceUnavailable, "ASSESSMENT_REPLAY_UNAVAILABLE", "assessment replay is not configured"), nil
	}
	payload, err := s.assessmentReplay.ExportRegressionCase(ctx, userID, request.Id)
	if err != nil {
		return assessmentRegressionFailure(err), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.AssessmentRegressionExport]("AssessmentRegressionExport", payload)
	if err != nil {
		return exportAssessmentError(http.StatusInternalServerError, "INTERNAL_ERROR", "assessment regression export violates the public contract"), nil
	}
	return openapiv1.ExportAssessmentRegressionCase200JSONResponse(response), nil
}

func assessmentReportToOpenAPI(report *model.AssessmentReport) (openapiv1.AssessmentReport, error) {
	var response openapiv1.AssessmentReport
	switch report.ContractRevision {
	case "assessment-output-v2":
		typed, err := strictOpenAPIConvert[openapiv1.AssessmentReportV2]("AssessmentReportV2", report)
		if err != nil {
			return response, err
		}
		if err := response.FromAssessmentReportV2(typed); err != nil {
			return response, err
		}
		return response, nil
	case "assessment-output-v1":
		typed, err := strictOpenAPIConvert[openapiv1.AssessmentReportV1]("AssessmentReportV1", report)
		if err != nil {
			return response, err
		}
		if err := response.FromAssessmentReportV1(typed); err != nil {
			return response, err
		}
		return response, nil
	default:
		return response, fmt.Errorf("unsupported assessment contract revision %q", report.ContractRevision)
	}
}

func generateAssessmentError(status int, code, message string) openapiv1.GenerateAssessmentResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.GenerateAssessment401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusBadGateway:
		return openapiv1.GenerateAssessment502JSONResponse{BadGatewayJSONResponse: openapiv1.BadGatewayJSONResponse(e)}
	default:
		return openapiv1.GenerateAssessment500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func listAssessmentsError(status int, code, message string) openapiv1.ListAssessmentsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ListAssessments400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.ListAssessments401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	default:
		return openapiv1.ListAssessments500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func getAssessmentError(status int, code, message string) openapiv1.GetAssessmentResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.GetAssessment401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.GetAssessment404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	default:
		return openapiv1.GetAssessment500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func replayAssessmentError(status int, code, message string) openapiv1.ReplayAssessmentResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ReplayAssessment400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.ReplayAssessment401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ReplayAssessment404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.ReplayAssessment409JSONResponse{ConflictJSONResponse: openapiv1.ConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ReplayAssessment503JSONResponse{ServiceUnavailableJSONResponse: openapiv1.ServiceUnavailableJSONResponse(e)}
	default:
		return openapiv1.ReplayAssessment500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func exportAssessmentError(status int, code, message string) openapiv1.ExportAssessmentRegressionCaseResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.ExportAssessmentRegressionCase401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ExportAssessmentRegressionCase404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.ExportAssessmentRegressionCase409JSONResponse{ConflictJSONResponse: openapiv1.ConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ExportAssessmentRegressionCase503JSONResponse{ServiceUnavailableJSONResponse: openapiv1.ServiceUnavailableJSONResponse(e)}
	default:
		return openapiv1.ExportAssessmentRegressionCase500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func assessmentReplayFailure(err error) openapiv1.ReplayAssessmentResponseObject {
	switch {
	case errors.Is(err, service.ErrAssessmentReplayNotFound):
		return replayAssessmentError(http.StatusNotFound, "ASSESSMENT_NOT_FOUND", err.Error())
	case errors.Is(err, service.ErrAssessmentReplayUnavailable):
		return replayAssessmentError(http.StatusConflict, "ASSESSMENT_REPLAY_INPUT_UNAVAILABLE", err.Error())
	case errors.Is(err, service.ErrAssessmentReplayConfiguration):
		return replayAssessmentError(http.StatusBadRequest, "INVALID_CONFIGURATION", err.Error())
	default:
		return replayAssessmentError(http.StatusInternalServerError, "ASSESSMENT_REPLAY_FAILED", err.Error())
	}
}

func assessmentRegressionFailure(err error) openapiv1.ExportAssessmentRegressionCaseResponseObject {
	switch {
	case errors.Is(err, service.ErrAssessmentReplayNotFound):
		return exportAssessmentError(http.StatusNotFound, "ASSESSMENT_NOT_FOUND", err.Error())
	case errors.Is(err, service.ErrAssessmentReplayUnavailable):
		return exportAssessmentError(http.StatusConflict, "ASSESSMENT_REPLAY_INPUT_UNAVAILABLE", err.Error())
	default:
		return exportAssessmentError(http.StatusInternalServerError, "ASSESSMENT_EXPORT_FAILED", err.Error())
	}
}
