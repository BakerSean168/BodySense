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

type nativeDiagnosisStartApplication interface {
	Start(context.Context, uuid.UUID, service.DiagnosisFormalAnalysisCommand) (*service.DiagnosisProposalIngestionReceipt, error)
}

// WithNativeDiagnosisAnalysis wires the formal-analysis start command. The run
// projection reader is the existing owner-scoped one; it is required so the
// response is the persisted run, never the Agent's raw output.
func (s *PublicServer) WithNativeDiagnosisAnalysis(starter nativeDiagnosisStartApplication, reader nativeDiagnosisRunReadApplication) *PublicServer {
	s.nativeDiagnosisStarter = starter
	s.nativeDiagnosisRuns = reader
	return s
}

func (s *PublicServer) StartNativeDiagnosisRun(ctx context.Context, req openapiv1.StartNativeDiagnosisRunRequestObject) (openapiv1.StartNativeDiagnosisRunResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if req.Body == nil || req.Body.RequestEvidenceId == uuid.Nil || req.Body.Trigger != openapiv1.NativeDiagnosisRunStartRequestTrigger(model.ConditionNewAnalysis) {
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "explicit formal analysis request required"}, nil
	}
	if s.nativeDiagnosisStarter == nil || s.nativeDiagnosisRuns == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal analysis unavailable"}, nil
	}
	cmd := service.DiagnosisFormalAnalysisCommand{
		RequestEvidenceID: uuid.UUID(req.Body.RequestEvidenceId), Trigger: model.ConditionNewAnalysis,
		ExpectedRevision: req.Body.ExpectedRevision, IdempotencyKey: req.Body.IdempotencyKey,
	}
	if req.Body.ScopeDescription != nil {
		cmd.ScopeDescription = *req.Body.ScopeDescription
	}
	appCtx := applicationRequestContext(ctx)
	receipt, err := s.nativeDiagnosisStarter.Start(appCtx, owner, cmd)
	// A Go Safety block is a durable terminal run; return its projection.
	if err != nil && !(errors.Is(err, service.ErrDiagnosisSafetyBlocked) && receipt != nil && receipt.Outcome == "safety_blocked") {
		return nativeDiagnosisStartError(err), nil
	}
	if receipt == nil || receipt.RunID == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "formal analysis receipt inconsistent"}, nil
	}
	run, err := s.nativeDiagnosisRuns.Read(appCtx, owner, receipt.RunID)
	if err != nil || run == nil || run.OwnerID != owner || run.RunID != receipt.RunID || len(run.Proposals) != len(receipt.ProposalIDs) {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "formal run projection inconsistent"}, nil
	}
	out, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisRunProjection]("NativeDiagnosisRunProjection", run)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "formal run projection violates contract"}, nil
	}
	if receipt.Replayed {
		return openapiv1.StartNativeDiagnosisRun200JSONResponse(out), nil
	}
	return openapiv1.StartNativeDiagnosisRun201JSONResponse(out), nil
}

func nativeDiagnosisStartError(err error) diagnosisHTTPErrorResponse {
	switch {
	case errors.Is(err, service.ErrDiagnosisIngestionInvalid):
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "invalid formal analysis request"}
	case errors.Is(err, service.ErrDiagnosisIngestionStale):
		return diagnosisHTTPErrorResponse{status: http.StatusConflict, code: "REVISION_CONFLICT", message: "body state changed during analysis; retry with a new request"}
	case errors.Is(err, service.ErrDiagnosisBodyStateNotReady):
		return diagnosisHTTPErrorResponse{status: http.StatusConflict, code: "BODY_STATE_NOT_READY", message: "body state does not yet contain enough structured information"}
	case errors.Is(err, service.ErrDiagnosisAgentOutputInvalid):
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "AGENT_OUTPUT_REJECTED", message: "diagnosis agent output failed Go validation; nothing was saved"}
	case errors.Is(err, service.ErrDiagnosisAgentUnavailable):
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "AI_SERVICE_ERROR", message: "diagnosis agent unavailable; nothing was saved"}
	default:
		return nativeDiagnosisCommandError(err)
	}
}

func (r diagnosisHTTPErrorResponse) VisitStartNativeDiagnosisRunResponse(w http.ResponseWriter) error {
	return r.write(w)
}
