package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

// No HTTP or Agent action can mark these states adopted. The only persistence
// authority remains the Go decision/settlement services from DX-002 slices.
type nativeDiagnosisRunReadApplication interface {
	Read(context.Context, uuid.UUID, uuid.UUID) (*service.NativeDiagnosisRunProjection, error)
	List(context.Context, uuid.UUID, int, *uuid.UUID) (*service.NativeDiagnosisRunPage, error)
}

func (s *PublicServer) WithNativeDiagnosisRuns(reader nativeDiagnosisRunReadApplication) *PublicServer {
	s.nativeDiagnosisRuns = reader
	return s
}

func (s *PublicServer) GetNativeDiagnosisRun(ctx context.Context, request openapiv1.GetNativeDiagnosisRunRequestObject) (openapiv1.GetNativeDiagnosisRunResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if request.RunId == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "formal run ID required"}, nil
	}
	if s.nativeDiagnosisRuns == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run read unavailable"}, nil
	}
	run, err := s.nativeDiagnosisRuns.Read(applicationRequestContext(ctx), owner, uuid.UUID(request.RunId))
	if err != nil {
		if errors.Is(err, model.ErrDiagnosisNotFound) {
			return diagnosisHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "formal run not found"}, nil
		}
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run read unavailable"}, nil
	}
	if run == nil || run.OwnerID != owner || run.RunID != uuid.UUID(request.RunId) || run.CurrentDiagnosisRevision < run.BasedOnDiagnosisRevision {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run projection inconsistent"}, nil
	}
	result, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisRunProjection]("NativeDiagnosisRunProjection", run)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run projection unavailable"}, nil
	}
	return openapiv1.GetNativeDiagnosisRun200JSONResponse(result), nil
}

func (s *PublicServer) ListNativeDiagnosisRuns(ctx context.Context, request openapiv1.ListNativeDiagnosisRunsRequestObject) (openapiv1.ListNativeDiagnosisRunsResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 || limit > 50 {
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "formal run limit out of bounds"}, nil
	}
	if s.nativeDiagnosisRuns == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run index unavailable"}, nil
	}
	var before *uuid.UUID
	if request.Params.BeforeRunId != nil {
		id := uuid.UUID(*request.Params.BeforeRunId)
		before = &id
	}
	page, err := s.nativeDiagnosisRuns.List(applicationRequestContext(ctx), owner, limit, before)
	if err != nil {
		if errors.Is(err, model.ErrDiagnosisNotFound) {
			return diagnosisHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "formal run cursor not found"}, nil
		}
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run index unavailable"}, nil
	}
	if page == nil || page.OwnerID != owner || (page.HasMore && (page.NextBeforeRunID == nil || len(page.Runs) == 0)) {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run index inconsistent"}, nil
	}
	result, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisRunPage]("NativeDiagnosisRunPage", page)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal run index projection unavailable"}, nil
	}
	return openapiv1.ListNativeDiagnosisRuns200JSONResponse(result), nil
}

func (r diagnosisHTTPErrorResponse) VisitGetNativeDiagnosisRunResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitListNativeDiagnosisRunsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
