package httpapi

import (
	"context"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"net/http"
)

func (s *PublicServer) CorrectBodyFactRecord(ctx context.Context, request openapiv1.CorrectBodyFactRecordRequestObject) (openapiv1.CorrectBodyFactRecordResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return correctBodyFactRecordError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return correctBodyFactRecordError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	if s.bodyFacts == nil {
		return correctBodyFactRecordError(http.StatusServiceUnavailable, "FACT_LEDGER_UNAVAILABLE", "record service is unavailable"), nil
	}
	command, err := strictJSONConvert[model.AmendBodyFactCommand](request.Body)
	if err != nil {
		return correctBodyFactRecordError(http.StatusBadRequest, "INVALID_BODY_FACT", "invalid amendment contract"), nil
	}
	receipt, err := s.bodyFacts.CorrectDirectFact(applicationRequestContext(ctx), owner, request.Id, command)
	if err != nil {
		public := classifyBodyFactPublicError(err)
		return correctBodyFactRecordError(public.status, public.code, public.message), nil
	}
	if receipt == nil || receipt.UserID != owner || receipt.ID == uuid.Nil || receipt.RevisionID == uuid.Nil || receipt.EventID == uuid.Nil {
		return correctBodyFactRecordError(http.StatusInternalServerError, "MISSING_COMMIT_RECEIPT", "record amendment was not confirmed committed"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyFactCommit]("BodyFactCommit", receipt)
	if err != nil {
		return correctBodyFactRecordError(http.StatusInternalServerError, "INVALID_COMMIT_RECEIPT", "commit receipt violates the public contract"), nil
	}
	return openapiv1.CorrectBodyFactRecord200JSONResponse(response), nil
}

func correctBodyFactRecordError(status int, code, message string) openapiv1.CorrectBodyFactRecordResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case 400:
		return openapiv1.CorrectBodyFactRecord400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case 401:
		return openapiv1.CorrectBodyFactRecord401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case 404:
		return openapiv1.CorrectBodyFactRecord404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case 409:
		return openapiv1.CorrectBodyFactRecord409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case 503:
		return openapiv1.CorrectBodyFactRecord503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.CorrectBodyFactRecord500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func (s *PublicServer) WithdrawBodyFactRecord(ctx context.Context, request openapiv1.WithdrawBodyFactRecordRequestObject) (openapiv1.WithdrawBodyFactRecordResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return withdrawBodyFactRecordError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return withdrawBodyFactRecordError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}
	if s.bodyFacts == nil {
		return withdrawBodyFactRecordError(http.StatusServiceUnavailable, "FACT_LEDGER_UNAVAILABLE", "record service is unavailable"), nil
	}
	command, err := strictJSONConvert[model.AmendBodyFactCommand](request.Body)
	if err != nil {
		return withdrawBodyFactRecordError(http.StatusBadRequest, "INVALID_BODY_FACT", "invalid amendment contract"), nil
	}
	receipt, err := s.bodyFacts.WithdrawDirectFact(applicationRequestContext(ctx), owner, request.Id, command)
	if err != nil {
		public := classifyBodyFactPublicError(err)
		return withdrawBodyFactRecordError(public.status, public.code, public.message), nil
	}
	if receipt == nil || receipt.UserID != owner || receipt.ID == uuid.Nil || receipt.RevisionID == uuid.Nil || receipt.EventID == uuid.Nil {
		return withdrawBodyFactRecordError(http.StatusInternalServerError, "MISSING_COMMIT_RECEIPT", "record amendment was not confirmed committed"), nil
	}
	response, err := strictOpenAPIConvert[openapiv1.BodyFactCommit]("BodyFactCommit", receipt)
	if err != nil {
		return withdrawBodyFactRecordError(http.StatusInternalServerError, "INVALID_COMMIT_RECEIPT", "commit receipt violates the public contract"), nil
	}
	return openapiv1.WithdrawBodyFactRecord200JSONResponse(response), nil
}

func withdrawBodyFactRecordError(status int, code, message string) openapiv1.WithdrawBodyFactRecordResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case 400:
		return openapiv1.WithdrawBodyFactRecord400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case 401:
		return openapiv1.WithdrawBodyFactRecord401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case 404:
		return openapiv1.WithdrawBodyFactRecord404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case 409:
		return openapiv1.WithdrawBodyFactRecord409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case 503:
		return openapiv1.WithdrawBodyFactRecord503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.WithdrawBodyFactRecord500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
