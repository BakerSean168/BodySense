package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// safetyRestrictionApplication is wired only to authenticated public user
// commands. No Agent/internal tool can invoke professional clearance.
type safetyRestrictionApplication interface {
	ResolveSafetyRestriction(context.Context, uuid.UUID, uuid.UUID, model.ResolveSafetyRestrictionCommand) (*model.SafetyResolution, error)
	ListSafetyRestrictions(context.Context, uuid.UUID) (*model.SafetyRestrictionList, error)
}

func (s *PublicServer) WithSafetyRestrictions(app safetyRestrictionApplication) *PublicServer {
	s.safetyRestrictions = app
	return s
}

func (s *PublicServer) ResolveSafetyRestriction(ctx context.Context, req openapiv1.ResolveSafetyRestrictionRequestObject) (openapiv1.ResolveSafetyRestrictionResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return resolveSafetyRestrictionError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil || req.Id == uuid.Nil {
		return resolveSafetyRestrictionError(http.StatusBadRequest, "INVALID_SAFETY_RESOLUTION", "exact target and source required"), nil
	}
	if s.safetyRestrictions == nil {
		return resolveSafetyRestrictionError(http.StatusServiceUnavailable, "SAFETY_UNAVAILABLE", "safety restriction ledger unavailable"), nil
	}
	command, err := strictJSONConvert[model.ResolveSafetyRestrictionCommand](req.Body)
	if err != nil || command.Validate() != nil {
		return resolveSafetyRestrictionError(http.StatusBadRequest, "INVALID_SAFETY_RESOLUTION", "explicit source-bound clearance required"), nil
	}
	receipt, err := s.safetyRestrictions.ResolveSafetyRestriction(applicationRequestContext(ctx), owner, req.Id, command)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidSafetyResolution):
			return resolveSafetyRestrictionError(http.StatusBadRequest, "INVALID_SAFETY_RESOLUTION", "source is not explicit professional clearance"), nil
		case errors.Is(err, model.ErrSafetyRestrictionNotFound), errors.Is(err, model.ErrSafetyResolutionSourceNotFound):
			return resolveSafetyRestrictionError(http.StatusNotFound, "NOT_FOUND", "restriction or source not found"), nil
		case errors.Is(err, model.ErrSafetyRestrictionAlreadyResolved), errors.Is(err, model.ErrSafetyResolutionIdempotencyConflict), errors.Is(err, repository.ErrBodyStateRevisionConflict):
			return resolveSafetyRestrictionError(http.StatusConflict, "SAFETY_RESOLUTION_CONFLICT", "reload restrictions before retrying"), nil
		default:
			return resolveSafetyRestrictionError(http.StatusServiceUnavailable, "SAFETY_UNAVAILABLE", "safety clearance could not be confirmed"), nil
		}
	}
	if receipt == nil || receipt.ID == uuid.Nil || receipt.RevisionID == uuid.Nil || receipt.EventID == uuid.Nil || receipt.UserID != owner || receipt.RestrictionID != req.Id || receipt.Revision != command.ExpectedRevision+1 || receipt.IdempotencyKey != command.IdempotencyKey || receipt.RestrictionVersion != command.ExpectedRestrictionVersion || receipt.BasisKind != command.BasisKind || receipt.SourceVersion != command.SourceVersion || !safetyReceiptSourceMatches(receipt, command) {
		return resolveSafetyRestrictionError(http.StatusInternalServerError, "INVALID_SAFETY_RECEIPT", "clearance commit not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.SafetyResolution]("SafetyResolution", receipt)
	if err != nil {
		return resolveSafetyRestrictionError(http.StatusInternalServerError, "INVALID_SAFETY_RECEIPT", "clearance receipt violates contract"), nil
	}
	return openapiv1.ResolveSafetyRestriction200JSONResponse(converted), nil
}

func (s *PublicServer) ListSafetyRestrictions(ctx context.Context, _ openapiv1.ListSafetyRestrictionsRequestObject) (openapiv1.ListSafetyRestrictionsResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return openapiv1.ListSafetyRestrictions401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope("UNAUTHORIZED", "authentication required"))}, nil
	}
	unavailable := func() (openapiv1.ListSafetyRestrictionsResponseObject, error) {
		return openapiv1.ListSafetyRestrictions503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(errorEnvelope("SAFETY_UNAVAILABLE", "safety restriction read unavailable"))}, nil
	}
	if s.safetyRestrictions == nil {
		return unavailable()
	}
	list, err := s.safetyRestrictions.ListSafetyRestrictions(applicationRequestContext(ctx), owner)
	if err != nil || list == nil || list.UserID != owner {
		return unavailable()
	}
	for _, row := range list.Restrictions {
		if row.ID == uuid.Nil || row.UserID != owner || row.CreatedRevision > list.CurrentRevision || row.Active != (row.Resolution == nil || !row.ClearanceAvailable) {
			return unavailable()
		}
		if row.Resolution != nil && (row.Resolution.UserID != owner || row.Resolution.RestrictionID != row.ID || row.Resolution.Revision > list.CurrentRevision) {
			return unavailable()
		}
	}
	converted, err := strictOpenAPIConvert[openapiv1.SafetyRestrictionList]("SafetyRestrictionList", list)
	if err != nil {
		return unavailable()
	}
	return openapiv1.ListSafetyRestrictions200JSONResponse(converted), nil
}

func resolveSafetyRestrictionError(status int, code, message string) openapiv1.ResolveSafetyRestrictionResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case 400:
		return openapiv1.ResolveSafetyRestriction400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case 401:
		return openapiv1.ResolveSafetyRestriction401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case 404:
		return openapiv1.ResolveSafetyRestriction404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case 409:
		return openapiv1.ResolveSafetyRestriction409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case 503:
		return openapiv1.ResolveSafetyRestriction503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ResolveSafetyRestriction500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func safetyReceiptSourceMatches(r *model.SafetyResolution, c model.ResolveSafetyRestrictionCommand) bool {
	if c.BasisKind == "expiry" {
		return r.SourceEvidenceID == nil && r.SourceMessageID == nil && r.CheckInteractionID == nil && r.Actor == "system" && r.SourceLevel == "go_policy"
	}
	if r.SourceEvidenceID == nil || *r.SourceEvidenceID == uuid.Nil || r.SourceMessageID == nil || *r.SourceMessageID != c.SourceMessageID || r.Actor != "user" || r.SourceLevel != "user_reported" {
		return false
	}
	if c.BasisKind == "user_reverification" {
		return r.CheckInteractionID != nil && *r.CheckInteractionID == c.CheckInteractionID
	}
	return r.CheckInteractionID == nil
}
