package httpapi

import (
	"context"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// AdmitBodyEpisodeStatus deliberately reuses the strict Fact ID + revision +
// idempotency command shape. The desired status is NEVER accepted from the
// browser: the repository derives an explicit claim from the existing,
// verified, linked and user-authored Fact (or rejects it).
func (s *PublicServer) AdmitBodyEpisodeStatus(ctx context.Context, request openapiv1.AdmitBodyEpisodeStatusRequestObject) (openapiv1.AdmitBodyEpisodeStatusResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return admitEpisodeStatusHTTPError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Id == uuid.Nil || request.Body == nil {
		return admitEpisodeStatusHTTPError(http.StatusBadRequest, "INVALID_REQUEST", "exact Episode and source required"), nil
	}
	if s.episodeCommands == nil {
		return admitEpisodeStatusHTTPError(http.StatusServiceUnavailable, "EPISODE_COMMAND_UNAVAILABLE", "Episode status evidence unavailable"), nil
	}
	command, err := strictJSONConvert[model.EpisodeAssociationCommand](request.Body)
	if err != nil {
		return admitEpisodeStatusHTTPError(http.StatusBadRequest, "INVALID_EPISODE_COMMAND", "unsupported status evidence fields"), nil
	}
	receipt, err := s.episodeCommands.AdmitEpisodeStatus(applicationRequestContext(ctx), owner, request.Id, command)
	if err != nil {
		status, code, message := classifyEpisodeCommandError(err)
		return admitEpisodeStatusHTTPError(status, code, message), nil
	}
	if !validEpisodeCommandReceipt(owner, request.Id, command, "status", receipt) {
		return admitEpisodeStatusHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_COMMIT", "status evidence commit not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyEpisodeCommandReceipt]("BodyEpisodeCommandReceipt", receipt)
	if err != nil {
		return admitEpisodeStatusHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_COMMIT", "status evidence receipt invalid"), nil
	}
	return openapiv1.AdmitBodyEpisodeStatus200JSONResponse(converted), nil
}

func admitEpisodeStatusHTTPError(status int, code, message string) openapiv1.AdmitBodyEpisodeStatusResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.AdmitBodyEpisodeStatus400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.AdmitBodyEpisodeStatus401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.AdmitBodyEpisodeStatus404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.AdmitBodyEpisodeStatus409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.AdmitBodyEpisodeStatus503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.AdmitBodyEpisodeStatus500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
