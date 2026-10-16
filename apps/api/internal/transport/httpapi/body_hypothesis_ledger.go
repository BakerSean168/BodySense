package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// bodyHypothesisApplication is the V2 user-authored BodyHypothesis boundary.
// Only an authenticated DIRECT user edit may record or withdraw one; owner,
// source and revision are server-owned. There is deliberately no AI/Agent
// tool or internal route onto this boundary: Agent hypotheses belong to the
// ConditionAnalysisRun audit. ConcernGroups are a derived read only.
type bodyHypothesisApplication interface {
	RecordBodyHypothesis(context.Context, uuid.UUID, model.RecordBodyHypothesisCommand) (*model.BodyHypothesisCommit, error)
	WithdrawBodyHypothesis(context.Context, uuid.UUID, uuid.UUID, model.WithdrawBodyHypothesisCommand) (*model.BodyHypothesisCommit, error)
	CorrectBodyHypothesis(context.Context, uuid.UUID, uuid.UUID, model.CorrectBodyHypothesisCommand) (*model.BodyHypothesisCommit, error)
	ListActiveBodyHypotheses(context.Context, uuid.UUID) (*model.BodyHypothesisList, error)
	ListDerivedConcernGroups(context.Context, uuid.UUID) (*model.DerivedConcernGroupList, error)
}

func (s *PublicServer) WithBodyHypotheses(app bodyHypothesisApplication) *PublicServer {
	s.bodyHypotheses = app
	return s
}

func (s *PublicServer) RecordBodyHypothesis(ctx context.Context, req openapiv1.RecordBodyHypothesisRequestObject) (openapiv1.RecordBodyHypothesisResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return recordBodyHypothesisError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil {
		return recordBodyHypothesisError(http.StatusBadRequest, "INVALID_HYPOTHESIS", "hypothesis statement required"), nil
	}
	if s.bodyHypotheses == nil {
		return recordBodyHypothesisError(http.StatusServiceUnavailable, "HYPOTHESIS_UNAVAILABLE", "hypothesis ledger unavailable"), nil
	}
	command, err := strictJSONConvert[model.RecordBodyHypothesisCommand](req.Body)
	if err != nil || command.Validate() != nil {
		return recordBodyHypothesisError(http.StatusBadRequest, "INVALID_HYPOTHESIS", "invalid direct user hypothesis"), nil
	}
	receipt, err := s.bodyHypotheses.RecordBodyHypothesis(applicationRequestContext(ctx), owner, command)
	if err != nil {
		status, code, message := classifyBodyHypothesisPublicError(err)
		return recordBodyHypothesisError(status, code, message), nil
	}
	if !validBodyHypothesisReceipt(receipt, owner, nil, "record", command.ExpectedRevision, command.IdempotencyKey) {
		return recordBodyHypothesisError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_COMMIT", "hypothesis commit not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyHypothesisCommit]("BodyHypothesisCommit", receipt)
	if err != nil {
		return recordBodyHypothesisError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_COMMIT", "hypothesis receipt violates public contract"), nil
	}
	return openapiv1.RecordBodyHypothesis200JSONResponse(converted), nil
}

func (s *PublicServer) WithdrawBodyHypothesisRecord(ctx context.Context, req openapiv1.WithdrawBodyHypothesisRecordRequestObject) (openapiv1.WithdrawBodyHypothesisRecordResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return withdrawBodyHypothesisError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil || req.Id == uuid.Nil {
		return withdrawBodyHypothesisError(http.StatusBadRequest, "INVALID_HYPOTHESIS", "exact target required"), nil
	}
	if s.bodyHypotheses == nil {
		return withdrawBodyHypothesisError(http.StatusServiceUnavailable, "HYPOTHESIS_UNAVAILABLE", "hypothesis ledger unavailable"), nil
	}
	command, err := strictJSONConvert[model.WithdrawBodyHypothesisCommand](req.Body)
	if err != nil || command.Validate() != nil {
		return withdrawBodyHypothesisError(http.StatusBadRequest, "INVALID_HYPOTHESIS", "invalid direct withdrawal"), nil
	}
	receipt, err := s.bodyHypotheses.WithdrawBodyHypothesis(applicationRequestContext(ctx), owner, req.Id, command)
	if err != nil {
		status, code, message := classifyBodyHypothesisPublicError(err)
		return withdrawBodyHypothesisError(status, code, message), nil
	}
	target := req.Id
	if !validBodyHypothesisReceipt(receipt, owner, &target, "withdraw", command.ExpectedRevision, command.IdempotencyKey) {
		return withdrawBodyHypothesisError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_COMMIT", "withdrawal receipt not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyHypothesisCommit]("BodyHypothesisCommit", receipt)
	if err != nil {
		return withdrawBodyHypothesisError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_COMMIT", "withdrawal receipt violates contract"), nil
	}
	return openapiv1.WithdrawBodyHypothesisRecord200JSONResponse(converted), nil
}

func (s *PublicServer) CorrectBodyHypothesisRecord(ctx context.Context, req openapiv1.CorrectBodyHypothesisRecordRequestObject) (openapiv1.CorrectBodyHypothesisRecordResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return correctBodyHypothesisError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil || req.Id == uuid.Nil {
		return correctBodyHypothesisError(http.StatusBadRequest, "INVALID_HYPOTHESIS", "exact target required"), nil
	}
	if s.bodyHypotheses == nil {
		return correctBodyHypothesisError(http.StatusServiceUnavailable, "HYPOTHESIS_UNAVAILABLE", "hypothesis ledger unavailable"), nil
	}
	command, err := strictJSONConvert[model.CorrectBodyHypothesisCommand](req.Body)
	if err != nil || command.Validate() != nil {
		return correctBodyHypothesisError(http.StatusBadRequest, "INVALID_HYPOTHESIS", "invalid direct correction"), nil
	}
	receipt, err := s.bodyHypotheses.CorrectBodyHypothesis(applicationRequestContext(ctx), owner, req.Id, command)
	if err != nil {
		status, code, message := classifyBodyHypothesisPublicError(err)
		return correctBodyHypothesisError(status, code, message), nil
	}
	target := req.Id
	if !validBodyHypothesisReceipt(receipt, owner, &target, "correct", command.ExpectedRevision, command.IdempotencyKey) {
		return correctBodyHypothesisError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_COMMIT", "correction receipt not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyHypothesisCommit]("BodyHypothesisCommit", receipt)
	if err != nil {
		return correctBodyHypothesisError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_COMMIT", "correction receipt violates contract"), nil
	}
	return openapiv1.CorrectBodyHypothesisRecord200JSONResponse(converted), nil
}

func (s *PublicServer) ListBodyHypothesisRecords(ctx context.Context, _ openapiv1.ListBodyHypothesisRecordsRequestObject) (openapiv1.ListBodyHypothesisRecordsResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return listBodyHypothesesError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyHypotheses == nil {
		return listBodyHypothesesError(http.StatusServiceUnavailable, "HYPOTHESIS_UNAVAILABLE", "hypothesis ledger unavailable"), nil
	}
	list, err := s.bodyHypotheses.ListActiveBodyHypotheses(applicationRequestContext(ctx), owner)
	if err != nil {
		status, code, message := classifyBodyHypothesisPublicError(err)
		return listBodyHypothesesError(status, code, message), nil
	}
	if list == nil || list.UserID != owner || list.CurrentRevision < 0 || len(list.Hypotheses) > 100 {
		return listBodyHypothesesError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_PROJECTION", "hypothesis projection unavailable"), nil
	}
	for _, rec := range list.Hypotheses {
		if rec.ID == uuid.Nil || rec.UserID != owner || rec.LineageID == uuid.Nil || rec.CreatedRevision < 1 ||
			rec.CreatedRevision > list.CurrentRevision || rec.Source.Actor != "user" ||
			rec.Source.ID == uuid.Nil || rec.Source.Version < 1 || rec.AssertionStatus != "asserted" || rec.Statement == "" {
			return listBodyHypothesesError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_PROJECTION", "hypothesis projection unavailable"), nil
		}
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyHypothesisRecordList]("BodyHypothesisRecordList", list)
	if err != nil {
		return listBodyHypothesesError(http.StatusInternalServerError, "INVALID_HYPOTHESIS_PROJECTION", "hypothesis response violates contract"), nil
	}
	return openapiv1.ListBodyHypothesisRecords200JSONResponse(converted), nil
}

func (s *PublicServer) ListDerivedConcernGroups(ctx context.Context, _ openapiv1.ListDerivedConcernGroupsRequestObject) (openapiv1.ListDerivedConcernGroupsResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return listConcernGroupsError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.bodyHypotheses == nil {
		return listConcernGroupsError(http.StatusServiceUnavailable, "CONCERN_GROUPS_UNAVAILABLE", "concern group projection unavailable"), nil
	}
	list, err := s.bodyHypotheses.ListDerivedConcernGroups(applicationRequestContext(ctx), owner)
	if err != nil {
		status, code, message := classifyBodyHypothesisPublicError(err)
		return listConcernGroupsError(status, code, message), nil
	}
	if list == nil || list.UserID != owner || list.CurrentRevision < 0 {
		return listConcernGroupsError(http.StatusInternalServerError, "INVALID_CONCERN_GROUP_PROJECTION", "concern group projection unavailable"), nil
	}
	for _, g := range list.Groups {
		if g.ID == uuid.Nil || g.LatestRevision < 1 || g.LatestRevision > list.CurrentRevision ||
			(len(g.RelatedHypothesisIDs) == 0 && len(g.RelatedEpisodeIDs) == 0) {
			return listConcernGroupsError(http.StatusInternalServerError, "INVALID_CONCERN_GROUP_PROJECTION", "concern group projection unavailable"), nil
		}
	}
	converted, err := strictOpenAPIConvert[openapiv1.DerivedConcernGroupList]("DerivedConcernGroupList", list)
	if err != nil {
		return listConcernGroupsError(http.StatusInternalServerError, "INVALID_CONCERN_GROUP_PROJECTION", "concern group response violates contract"), nil
	}
	return openapiv1.ListDerivedConcernGroups200JSONResponse(converted), nil
}

func validBodyHypothesisReceipt(rec *model.BodyHypothesisCommit, owner uuid.UUID, target *uuid.UUID, op string, expectedRevision int64, key string) bool {
	if rec == nil || rec.ID == uuid.Nil || rec.UserID != owner || rec.Operation != op ||
		rec.IdempotencyKey != key || rec.HypothesisID == uuid.Nil || rec.SourceEvidenceID == uuid.Nil ||
		rec.RevisionID == uuid.Nil || rec.EventID == uuid.Nil || rec.Revision <= 0 ||
		rec.Revision != expectedRevision+1 || rec.CommittedAt.IsZero() {
		return false
	}
	switch op {
	case "record":
		return target == nil && rec.TargetHypothesisID == nil
	case "withdraw":
		return target != nil && rec.TargetHypothesisID != nil && *rec.TargetHypothesisID == *target && rec.HypothesisID == *target
	case "correct":
		return target != nil && rec.TargetHypothesisID != nil && *rec.TargetHypothesisID == *target && rec.HypothesisID != *target
	default:
		return false
	}
}

func classifyBodyHypothesisPublicError(err error) (int, string, string) {
	switch {
	case errors.Is(err, model.ErrInvalidBodyHypothesis):
		return http.StatusBadRequest, "INVALID_HYPOTHESIS", "invalid hypothesis request"
	case errors.Is(err, model.ErrBodyHypothesisFactNotFound):
		return http.StatusNotFound, "HYPOTHESIS_FACT_NOT_FOUND", "a related fact is unavailable to this owner"
	case errors.Is(err, model.ErrBodyHypothesisObservationNotFound):
		return http.StatusNotFound, "HYPOTHESIS_OBSERVATION_NOT_FOUND", "a related observation is unavailable to this owner"
	case errors.Is(err, model.ErrBodyHypothesisNotFound):
		return http.StatusNotFound, "HYPOTHESIS_NOT_FOUND", "hypothesis is unavailable to this owner"
	case errors.Is(err, model.ErrBodyHypothesisNotCurrent):
		return http.StatusConflict, "HYPOTHESIS_NOT_CURRENT", "hypothesis has already been withdrawn or corrected"
	case errors.Is(err, model.ErrBodyHypothesisIdempotencyConflict):
		return http.StatusConflict, "HYPOTHESIS_IDEMPOTENCY_CONFLICT", "idempotency key already authorized different input"
	default:
		p := classifyBodyStatePublicError(err)
		return p.status, p.code, p.message
	}
}

func recordBodyHypothesisError(status int, code, message string) openapiv1.RecordBodyHypothesisResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.RecordBodyHypothesis400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.RecordBodyHypothesis401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.RecordBodyHypothesis404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.RecordBodyHypothesis409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.RecordBodyHypothesis503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.RecordBodyHypothesis500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func withdrawBodyHypothesisError(status int, code, message string) openapiv1.WithdrawBodyHypothesisRecordResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.WithdrawBodyHypothesisRecord400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.WithdrawBodyHypothesisRecord401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.WithdrawBodyHypothesisRecord404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.WithdrawBodyHypothesisRecord409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.WithdrawBodyHypothesisRecord503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.WithdrawBodyHypothesisRecord500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func correctBodyHypothesisError(status int, code, message string) openapiv1.CorrectBodyHypothesisRecordResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.CorrectBodyHypothesisRecord400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.CorrectBodyHypothesisRecord401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.CorrectBodyHypothesisRecord404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.CorrectBodyHypothesisRecord409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.CorrectBodyHypothesisRecord503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.CorrectBodyHypothesisRecord500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func listBodyHypothesesError(status int, code, message string) openapiv1.ListBodyHypothesisRecordsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.ListBodyHypothesisRecords401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ListBodyHypothesisRecords503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ListBodyHypothesisRecords500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func listConcernGroupsError(status int, code, message string) openapiv1.ListDerivedConcernGroupsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.ListDerivedConcernGroups401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ListDerivedConcernGroups503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ListDerivedConcernGroups500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
