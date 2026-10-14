package httpapi

import (
	"context"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Corrections never fabricate a later measurement: only user-authorized
// corrections to an exact original statement are admitted.
func (s *PublicServer) CorrectBodyObservationRecord(ctx context.Context, req openapiv1.CorrectBodyObservationRecordRequestObject) (openapiv1.CorrectBodyObservationRecordResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return correctBodyObservationError(401, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil || req.Id == uuid.Nil {
		return correctBodyObservationError(400, "INVALID_OBSERVATION", "exact target and correction required"), nil
	}
	if s.bodyObservations == nil {
		return correctBodyObservationError(503, "OBSERVATION_UNAVAILABLE", "observation ledger unavailable"), nil
	}
	command, err := strictJSONConvert[model.AmendBodyObservationCommand](req.Body)
	if err != nil || command.Validate(true) != nil {
		return correctBodyObservationError(400, "INVALID_OBSERVATION", "invalid user report correction"), nil
	}
	receipt, err := s.bodyObservations.CorrectDirectObservation(applicationRequestContext(ctx), owner, req.Id, command)
	if err != nil {
		status, code, message := classifyBodyObservationPublicError(err)
		return correctBodyObservationError(status, code, message), nil
	}
	if !validObservationAmendmentReceipt(receipt, owner, req.Id, "correct", command.ExpectedRevision, command.IdempotencyKey) {
		return correctBodyObservationError(500, "INVALID_OBSERVATION_COMMIT", "corrective receipt not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyObservationCommit]("BodyObservationCommit", receipt)
	if err != nil {
		return correctBodyObservationError(500, "INVALID_OBSERVATION_COMMIT", "corrective receipt violates contract"), nil
	}
	return openapiv1.CorrectBodyObservationRecord200JSONResponse(converted), nil
}
func (s *PublicServer) WithdrawBodyObservationRecord(ctx context.Context, req openapiv1.WithdrawBodyObservationRecordRequestObject) (openapiv1.WithdrawBodyObservationRecordResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return withdrawBodyObservationError(401, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil || req.Id == uuid.Nil {
		return withdrawBodyObservationError(400, "INVALID_OBSERVATION", "exact target required"), nil
	}
	if s.bodyObservations == nil {
		return withdrawBodyObservationError(503, "OBSERVATION_UNAVAILABLE", "observation ledger unavailable"), nil
	}
	command, err := strictJSONConvert[model.AmendBodyObservationCommand](req.Body)
	if err != nil || command.Validate(false) != nil {
		return withdrawBodyObservationError(400, "INVALID_OBSERVATION", "invalid direct withdrawal"), nil
	}
	receipt, err := s.bodyObservations.WithdrawDirectObservation(applicationRequestContext(ctx), owner, req.Id, command)
	if err != nil {
		status, code, message := classifyBodyObservationPublicError(err)
		return withdrawBodyObservationError(status, code, message), nil
	}
	if !validObservationAmendmentReceipt(receipt, owner, req.Id, "withdraw", command.ExpectedRevision, command.IdempotencyKey) {
		return withdrawBodyObservationError(500, "INVALID_OBSERVATION_COMMIT", "withdrawal receipt not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyObservationCommit]("BodyObservationCommit", receipt)
	if err != nil {
		return withdrawBodyObservationError(500, "INVALID_OBSERVATION_COMMIT", "withdrawal receipt violates contract"), nil
	}
	return openapiv1.WithdrawBodyObservationRecord200JSONResponse(converted), nil
}
func validObservationAmendmentReceipt(rec *model.BodyObservationCommit, owner, target uuid.UUID, op string, expectedRevision int64, key string) bool {
	if rec == nil || rec.UserID != owner || rec.ID == uuid.Nil || rec.TargetObservationID == nil ||
		*rec.TargetObservationID != target || rec.Operation != op || rec.ObservationID == uuid.Nil ||
		rec.SourceEvidenceID == uuid.Nil || rec.RevisionID == uuid.Nil || rec.EventID == uuid.Nil ||
		rec.IdempotencyKey != key || rec.Revision <= 0 || rec.Revision != expectedRevision+1 || rec.CommittedAt.IsZero() {
		return false
	}
	return (op == "correct" && rec.ObservationID != target) || (op == "withdraw" && rec.ObservationID == target)
}
func correctBodyObservationError(status int, code, message string) openapiv1.CorrectBodyObservationRecordResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.CorrectBodyObservationRecord400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.CorrectBodyObservationRecord401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.CorrectBodyObservationRecord404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.CorrectBodyObservationRecord409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.CorrectBodyObservationRecord503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.CorrectBodyObservationRecord500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
func withdrawBodyObservationError(status int, code, message string) openapiv1.WithdrawBodyObservationRecordResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.WithdrawBodyObservationRecord400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.WithdrawBodyObservationRecord401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.WithdrawBodyObservationRecord404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.WithdrawBodyObservationRecord409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.WithdrawBodyObservationRecord503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.WithdrawBodyObservationRecord500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
