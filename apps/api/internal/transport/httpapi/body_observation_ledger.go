package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// This boundary is deliberately independent of legacy review status and
// clinical measurement interpretation. Only an authenticated DIRECT self
// report may be recorded; source metadata is entirely server-owned.
type bodyObservationApplication interface {
	RecordDirectObservation(context.Context, uuid.UUID, model.RecordBodyObservationCommand) (*model.BodyObservationCommit, error)
	CorrectDirectObservation(context.Context, uuid.UUID, uuid.UUID, model.AmendBodyObservationCommand) (*model.BodyObservationCommit, error)
	WithdrawDirectObservation(context.Context, uuid.UUID, uuid.UUID, model.AmendBodyObservationCommand) (*model.BodyObservationCommit, error)
	ListDirectObservations(context.Context, uuid.UUID, int64) (*model.BodyObservationPage, error)
}

func (s *PublicServer) WithBodyObservations(app bodyObservationApplication) *PublicServer {
	s.bodyObservations = app
	return s
}
func (s *PublicServer) RecordBodyObservation(ctx context.Context, req openapiv1.RecordBodyObservationRequestObject) (openapiv1.RecordBodyObservationResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return recordBodyObservationError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil {
		return recordBodyObservationError(http.StatusBadRequest, "INVALID_REQUEST", "observation statement required"), nil
	}
	if s.bodyObservations == nil {
		return recordBodyObservationError(http.StatusServiceUnavailable, "OBSERVATION_UNAVAILABLE", "observation ledger unavailable"), nil
	}
	command, err := strictJSONConvert[model.RecordBodyObservationCommand](req.Body)
	if err != nil || command.Validate() != nil {
		return recordBodyObservationError(http.StatusBadRequest, "INVALID_OBSERVATION", "invalid direct user observation"), nil
	}
	receipt, err := s.bodyObservations.RecordDirectObservation(applicationRequestContext(ctx), owner, command)
	if err != nil {
		status, code, message := classifyBodyObservationPublicError(err)
		return recordBodyObservationError(status, code, message), nil
	}
	if receipt == nil || receipt.ID == uuid.Nil || receipt.UserID != owner ||
		receipt.IdempotencyKey != command.IdempotencyKey || receipt.Operation != "record" ||
		receipt.TargetObservationID != nil || receipt.Revision != command.ExpectedRevision+1 ||
		receipt.ObservationID == uuid.Nil || receipt.SourceEvidenceID == uuid.Nil ||
		receipt.RevisionID == uuid.Nil || receipt.EventID == uuid.Nil || receipt.Revision <= 0 ||
		receipt.CommittedAt.IsZero() {
		return recordBodyObservationError(http.StatusInternalServerError, "INVALID_OBSERVATION_COMMIT", "observation commit not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyObservationCommit]("BodyObservationCommit", receipt)
	if err != nil {
		return recordBodyObservationError(http.StatusInternalServerError, "INVALID_OBSERVATION_COMMIT", "observation receipt violates public contract"), nil
	}
	return openapiv1.RecordBodyObservation200JSONResponse(converted), nil
}

func (s *PublicServer) ListBodyObservationRecords(ctx context.Context, req openapiv1.ListBodyObservationRecordsRequestObject) (openapiv1.ListBodyObservationRecordsResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return listBodyObservationsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyObservations == nil {
		return listBodyObservationsError(http.StatusServiceUnavailable, "OBSERVATION_UNAVAILABLE", "observation ledger unavailable"), nil
	}
	before := int64(0)
	if req.Params.BeforeRevision != nil {
		before = *req.Params.BeforeRevision
	}
	if before < 0 {
		return listBodyObservationsError(http.StatusBadRequest, "INVALID_OBSERVATION_CURSOR", "invalid observation cursor"), nil
	}
	page, err := s.bodyObservations.ListDirectObservations(applicationRequestContext(ctx), owner, before)
	if err != nil {
		status, code, message := classifyBodyObservationPublicError(err)
		return listBodyObservationsError(status, code, message), nil
	}
	if page == nil || page.UserID != owner || page.CurrentRevision < 0 ||
		len(page.Observations) > 20 || page.HasMore != (page.NextBeforeRevision != nil) ||
		(page.NextBeforeRevision != nil && (*page.NextBeforeRevision < 1 || *page.NextBeforeRevision > page.CurrentRevision)) {
		return listBodyObservationsError(http.StatusInternalServerError, "INVALID_OBSERVATION_PROJECTION", "observation source projection unavailable"), nil
	}
	for _, rec := range page.Observations {
		if rec.ID == uuid.Nil || rec.UserID != owner || rec.CreatedRevision < 1 ||
			rec.CreatedRevision > page.CurrentRevision || rec.Source.ID == uuid.Nil ||
			rec.Source.Version < 1 || rec.Source.Kind != "user_report" ||
			rec.AssertionStatus != "asserted" || rec.Statement == "" ||
			rec.ObservedAt.Validate() != nil {
			return listBodyObservationsError(http.StatusInternalServerError, "INVALID_OBSERVATION_PROJECTION", "observation source projection unavailable"), nil
		}
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyObservationRecordPage]("BodyObservationRecordPage", page)
	if err != nil {
		return listBodyObservationsError(http.StatusInternalServerError, "INVALID_OBSERVATION_PROJECTION", "observation response violates contract"), nil
	}
	return openapiv1.ListBodyObservationRecords200JSONResponse(converted), nil
}
func classifyBodyObservationPublicError(err error) (int, string, string) {
	switch {
	case errors.Is(err, model.ErrInvalidBodyObservation):
		return http.StatusBadRequest, "INVALID_OBSERVATION", "invalid observation request"
	case errors.Is(err, model.ErrBodyObservationNotFound):
		return http.StatusNotFound, "OBSERVATION_NOT_FOUND", "observation is unavailable to this owner"
	case errors.Is(err, model.ErrBodyObservationNotCurrent):
		return http.StatusConflict, "OBSERVATION_NOT_CURRENT", "observation has already been corrected or withdrawn"
	case errors.Is(err, model.ErrBodyObservationIdempotencyConflict):
		return http.StatusConflict, "OBSERVATION_IDEMPOTENCY_CONFLICT", "idempotency key already authorized different input"
	default:
		p := classifyBodyStatePublicError(err)
		return p.status, p.code, p.message
	}
}
func recordBodyObservationError(status int, code, message string) openapiv1.RecordBodyObservationResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.RecordBodyObservation400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.RecordBodyObservation401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.RecordBodyObservation409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.RecordBodyObservation503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.RecordBodyObservation500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
func listBodyObservationsError(status int, code, message string) openapiv1.ListBodyObservationRecordsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ListBodyObservationRecords400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.ListBodyObservationRecords401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ListBodyObservationRecords503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ListBodyObservationRecords500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
