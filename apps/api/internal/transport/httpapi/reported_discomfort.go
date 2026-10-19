package httpapi

import (
	"context"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type reportedDiscomfortReader interface {
	Get(context.Context, uuid.UUID) (*service.ReportedDiscomfortSnapshot, error)
}

// WithReportedDiscomfort exposes the onboarding step 05 self-report (read only).
func (s *PublicServer) WithReportedDiscomfort(reader reportedDiscomfortReader) *PublicServer {
	s.reportedDiscomfort = reader
	return s
}

func (s *PublicServer) GetReportedDiscomfortAreas(
	ctx context.Context,
	_ openapiv1.GetReportedDiscomfortAreasRequestObject,
) (openapiv1.GetReportedDiscomfortAreasResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getReportedDiscomfortError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.reportedDiscomfort == nil {
		return getReportedDiscomfortError(http.StatusInternalServerError, "INTERNAL_ERROR", "reported discomfort is unavailable"), nil
	}
	result, err := s.reportedDiscomfort.Get(ctx, userID)
	if err != nil {
		return getReportedDiscomfortError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load reported discomfort"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.ReportedDiscomfortSnapshot]("ReportedDiscomfortSnapshot", result)
	if err != nil {
		return getReportedDiscomfortError(http.StatusInternalServerError, "INTERNAL_ERROR", "reported discomfort violates the public contract"), nil
	}
	return openapiv1.GetReportedDiscomfortAreas200JSONResponse(response), nil
}

func getReportedDiscomfortError(status int, code, message string) openapiv1.GetReportedDiscomfortAreasResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetReportedDiscomfortAreas401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetReportedDiscomfortAreas500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}
