package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type bodyFactApplication interface {
	RecordDirectFacts(context.Context, uuid.UUID, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
	ListFacts(context.Context, uuid.UUID, int64) (*model.BodyFactPage, error)
}

func (s *PublicServer) WithBodyFacts(application bodyFactApplication) *PublicServer {
	s.bodyFacts = application
	return s
}

func (s *PublicServer) RecordBodyFacts(ctx context.Context, request openapiv1.RecordBodyFactsRequestObject) (openapiv1.RecordBodyFactsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil || userID == uuid.Nil {
		return recordBodyFactsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return recordBodyFactsError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	if s.bodyFacts == nil {
		return recordBodyFactsError(http.StatusServiceUnavailable, "FACT_LEDGER_UNAVAILABLE", "fact ledger is unavailable"), nil
	}
	// The contract's discriminated input union terminates at this adapter.
	// Unknown fields cannot silently become domain commands.
	command, err := strictJSONConvert[model.RecordBodyFactsCommand](request.Body)
	if err != nil {
		return recordBodyFactsError(http.StatusBadRequest, "INVALID_BODY_FACT", "invalid statement batch"), nil
	}
	receipt, err := s.bodyFacts.RecordDirectFacts(applicationRequestContext(ctx), userID, command)
	if err != nil {
		public := classifyBodyFactPublicError(err)
		return recordBodyFactsError(public.status, public.code, public.message), nil
	}
	if receipt == nil || receipt.UserID != userID || receipt.ID == uuid.Nil || receipt.EventID == uuid.Nil || receipt.RevisionID == uuid.Nil {
		return recordBodyFactsError(http.StatusInternalServerError, "MISSING_COMMIT_RECEIPT", "statement batch was not confirmed committed"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyFactCommit]("BodyFactCommit", receipt)
	if err != nil {
		return recordBodyFactsError(http.StatusInternalServerError, "INVALID_COMMIT_RECEIPT", "statement commit receipt violates the contract"), nil
	}
	return openapiv1.RecordBodyFacts200JSONResponse(response), nil
}

func (s *PublicServer) ListBodyFactRecords(ctx context.Context, request openapiv1.ListBodyFactRecordsRequestObject) (openapiv1.ListBodyFactRecordsResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil || userID == uuid.Nil {
		return listBodyFactsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyFacts == nil {
		return listBodyFactsError(http.StatusServiceUnavailable, "FACT_LEDGER_UNAVAILABLE", "fact ledger is unavailable"), nil
	}
	var before int64
	if request.Params.BeforeRevision != nil {
		before = *request.Params.BeforeRevision
	}
	page, err := s.bodyFacts.ListFacts(applicationRequestContext(ctx), userID, before)
	if err != nil {
		public := classifyBodyFactPublicError(err)
		return listBodyFactsError(public.status, public.code, public.message), nil
	}
	if page == nil || page.UserID != userID {
		return listBodyFactsError(http.StatusInternalServerError, "INTERNAL_ERROR", "fact ledger returned no projection"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyFactRecordPage]("BodyFactRecordPage", page)
	if err != nil {
		return listBodyFactsError(http.StatusInternalServerError, "INVALID_FACT_PROJECTION", "fact projection violates the contract"), nil
	}
	return openapiv1.ListBodyFactRecords200JSONResponse(response), nil
}

func classifyBodyFactPublicError(err error) bodyStatePublicError {
	switch {
	case errors.Is(err, model.ErrInvalidBodyFact):
		return bodyStatePublicError{status: http.StatusBadRequest, code: "INVALID_BODY_FACT", message: err.Error()}
	case errors.Is(err, model.ErrBodyFactIdempotencyConflict):
		return bodyStatePublicError{status: http.StatusConflict, code: "IDEMPOTENCY_CONFLICT", message: "idempotency key was already used for a different statement batch"}
	default:
		return classifyBodyStatePublicError(err)
	}
}

func recordBodyFactsError(status int, code, message string) openapiv1.RecordBodyFactsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.RecordBodyFacts400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.RecordBodyFacts401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.RecordBodyFacts409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.RecordBodyFacts503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.RecordBodyFacts500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func listBodyFactsError(status int, code, message string) openapiv1.ListBodyFactRecordsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ListBodyFactRecords400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.ListBodyFactRecords401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.ListBodyFactRecords409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ListBodyFactRecords503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ListBodyFactRecords500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
