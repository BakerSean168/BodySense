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

type episodeCommandApplication interface {
	CreateEpisode(context.Context, uuid.UUID, model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error)
	LinkEpisodeFact(context.Context, uuid.UUID, uuid.UUID, model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error)
}

func (s *PublicServer) WithEpisodeCommands(commands episodeCommandApplication) *PublicServer {
	s.episodeCommands = commands
	return s
}

func (s *PublicServer) CreateBodyEpisode(ctx context.Context, request openapiv1.CreateBodyEpisodeRequestObject) (openapiv1.CreateBodyEpisodeResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return createEpisodeHTTPError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Body == nil {
		return createEpisodeHTTPError(http.StatusBadRequest, "INVALID_REQUEST", "request body required"), nil
	}
	if s.episodeCommands == nil {
		return createEpisodeHTTPError(http.StatusServiceUnavailable, "EPISODE_COMMAND_UNAVAILABLE", "episode command service unavailable"), nil
	}
	command, err := strictJSONConvert[model.EpisodeAssociationCommand](request.Body)
	if err != nil {
		return createEpisodeHTTPError(http.StatusBadRequest, "INVALID_EPISODE_COMMAND", "unsupported Episode fields"), nil
	}
	receipt, err := s.episodeCommands.CreateEpisode(applicationRequestContext(ctx), owner, command)
	if err != nil {
		status, code, message := classifyEpisodeCommandError(err)
		return createEpisodeHTTPError(status, code, message), nil
	}
	if !validEpisodeCommandReceipt(owner, uuid.Nil, command, "create", receipt) {
		return createEpisodeHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_COMMIT", "Episode commit not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyEpisodeCommandReceipt]("BodyEpisodeCommandReceipt", receipt)
	if err != nil {
		return createEpisodeHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_COMMIT", "Episode commit violates contract"), nil
	}
	return openapiv1.CreateBodyEpisode200JSONResponse(converted), nil
}

func (s *PublicServer) LinkBodyEpisodeFact(ctx context.Context, request openapiv1.LinkBodyEpisodeFactRequestObject) (openapiv1.LinkBodyEpisodeFactResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return linkEpisodeHTTPError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Id == uuid.Nil || request.Body == nil {
		return linkEpisodeHTTPError(http.StatusBadRequest, "INVALID_REQUEST", "exact Episode and request body required"), nil
	}
	if s.episodeCommands == nil {
		return linkEpisodeHTTPError(http.StatusServiceUnavailable, "EPISODE_COMMAND_UNAVAILABLE", "episode command service unavailable"), nil
	}
	command, err := strictJSONConvert[model.EpisodeAssociationCommand](request.Body)
	if err != nil {
		return linkEpisodeHTTPError(http.StatusBadRequest, "INVALID_EPISODE_COMMAND", "unsupported Episode fields"), nil
	}
	receipt, err := s.episodeCommands.LinkEpisodeFact(applicationRequestContext(ctx), owner, request.Id, command)
	if err != nil {
		status, code, message := classifyEpisodeCommandError(err)
		return linkEpisodeHTTPError(status, code, message), nil
	}
	if !validEpisodeCommandReceipt(owner, request.Id, command, "link", receipt) {
		return linkEpisodeHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_COMMIT", "Episode commit not confirmed"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyEpisodeCommandReceipt]("BodyEpisodeCommandReceipt", receipt)
	if err != nil {
		return linkEpisodeHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_COMMIT", "Episode commit violates contract"), nil
	}
	return openapiv1.LinkBodyEpisodeFact200JSONResponse(converted), nil
}

func validEpisodeCommandReceipt(owner, target uuid.UUID, c model.EpisodeAssociationCommand, operation string, receipt *model.EpisodeCommandReceipt) bool {
	if receipt == nil || receipt.ID == uuid.Nil || receipt.UserID != owner || receipt.FactID != c.FactID ||
		receipt.EpisodeID == uuid.Nil || receipt.Revision <= 0 || receipt.RevisionID == uuid.Nil ||
		receipt.EventID == uuid.Nil || receipt.Operation != operation || receipt.CommittedAt.IsZero() {
		return false
	}
	return operation != "link" || receipt.EpisodeID == target
}

func classifyEpisodeCommandError(err error) (int, string, string) {
	switch {
	case errors.Is(err, model.ErrInvalidEpisodeEvidence):
		return http.StatusBadRequest, "INVALID_EPISODE_COMMAND", "invalid Episode command or source"
	case errors.Is(err, model.ErrEpisodeFactUnavailable), errors.Is(err, repository.ErrEpisodeNotFound):
		return http.StatusNotFound, "EPISODE_SOURCE_UNAVAILABLE", "Episode or symptom source unavailable"
	case errors.Is(err, model.ErrEpisodeIdempotencyConflict):
		return http.StatusConflict, "EPISODE_IDEMPOTENCY_CONFLICT", "this command key has been used for another operation"
	case errors.Is(err, model.ErrEpisodeAssociationConflict):
		return http.StatusConflict, "EPISODE_ASSOCIATION_CONFLICT", "Episode and source cannot be associated by this command"
	case errors.Is(err, repository.ErrBodyStateRevisionConflict):
		return http.StatusConflict, "REVISION_CONFLICT", "BodyState was updated; reload and retry"
	default:
		return http.StatusInternalServerError, "EPISODE_COMMAND_FAILED", "Episode command could not be committed"
	}
}

func createEpisodeHTTPError(status int, code, message string) openapiv1.CreateBodyEpisodeResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.CreateBodyEpisode400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.CreateBodyEpisode401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.CreateBodyEpisode404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.CreateBodyEpisode409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.CreateBodyEpisode503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.CreateBodyEpisode500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func linkEpisodeHTTPError(status int, code, message string) openapiv1.LinkBodyEpisodeFactResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.LinkBodyEpisodeFact400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.LinkBodyEpisodeFact401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.LinkBodyEpisodeFact404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.LinkBodyEpisodeFact409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.LinkBodyEpisodeFact503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.LinkBodyEpisodeFact500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
