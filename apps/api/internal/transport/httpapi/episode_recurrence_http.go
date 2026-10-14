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

// ReportBodyEpisodeRecurrence binds a direct authenticated action to the
// exact old/new source-backed Episode IDs. It does not accept an AI guess,
// medical etiology, a client-created authorization, or an Episode state.
func (s *PublicServer) ReportBodyEpisodeRecurrence(ctx context.Context, req openapiv1.ReportBodyEpisodeRecurrenceRequestObject) (openapiv1.ReportBodyEpisodeRecurrenceResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return recurrenceHTTPError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Id == uuid.Nil || req.Body == nil {
		return recurrenceHTTPError(http.StatusBadRequest, "INVALID_REQUEST", "exact previous Episode and recurrence command required"), nil
	}
	if s.episodeCommands == nil {
		return recurrenceHTTPError(http.StatusServiceUnavailable, "EPISODE_COMMAND_UNAVAILABLE", "Episode commands unavailable"), nil
	}
	command, err := strictJSONConvert[model.EpisodeRecurrenceCommand](req.Body)
	if err != nil || command.Validate(req.Id) != nil {
		return recurrenceHTTPError(http.StatusBadRequest, "INVALID_RECURRENCE_COMMAND", "exact new Episode, Fact, revision and idempotency key required"), nil
	}
	receipt, err := s.episodeCommands.ReportEpisodeRecurrence(applicationRequestContext(ctx), owner, req.Id, command)
	if err != nil {
		status, code, message := classifyEpisodeCommandError(err)
		return recurrenceHTTPError(status, code, message), nil
	}
	if receipt == nil || receipt.ID == uuid.Nil || receipt.RelationID == uuid.Nil ||
		receipt.UserID != owner || receipt.FromEpisodeID != req.Id ||
		receipt.ToEpisodeID != command.NewEpisodeID || receipt.FactID != command.FactID ||
		receipt.FromEpisodeID == receipt.ToEpisodeID || receipt.Revision <= 0 ||
		receipt.RevisionID == uuid.Nil || receipt.EventID == uuid.Nil ||
		receipt.CommittedAt.IsZero() {
		return recurrenceHTTPError(http.StatusInternalServerError, "INVALID_RECURRENCE_COMMIT", "recurrence commit not confirmed"), nil
	}
	result, err := strictOpenAPIConvert[openapiv1.BodyEpisodeRecurrenceReceipt]("BodyEpisodeRecurrenceReceipt", receipt)
	if err != nil {
		return recurrenceHTTPError(http.StatusInternalServerError, "INVALID_RECURRENCE_COMMIT", "recurrence receipt invalid"), nil
	}
	return openapiv1.ReportBodyEpisodeRecurrence200JSONResponse(result), nil
}

func recurrenceHTTPError(status int, code, message string) openapiv1.ReportBodyEpisodeRecurrenceResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.ReportBodyEpisodeRecurrence401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusBadRequest:
		return openapiv1.ReportBodyEpisodeRecurrence400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ReportBodyEpisodeRecurrence404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.ReportBodyEpisodeRecurrence409JSONResponse{RevisionConflictJSONResponse: openapiv1.RevisionConflictJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ReportBodyEpisodeRecurrence503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ReportBodyEpisodeRecurrence500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func (s *PublicServer) ListBodyEpisodeRelations(ctx context.Context, req openapiv1.ListBodyEpisodeRelationsRequestObject) (openapiv1.ListBodyEpisodeRelationsResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return relationReadHTTPError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Id == uuid.Nil {
		return relationReadHTTPError(http.StatusBadRequest, "INVALID_EPISODE_ID", "exact Episode required"), nil
	}
	if s.episodes == nil {
		return relationReadHTTPError(http.StatusServiceUnavailable, "EPISODE_READ_UNAVAILABLE", "Episode relations unavailable"), nil
	}
	before := int64(0)
	if req.Params.BeforeRevision != nil {
		before = *req.Params.BeforeRevision
	}
	if before < 0 {
		return relationReadHTTPError(http.StatusBadRequest, "INVALID_EPISODE_CURSOR", "invalid Episode relation cursor"), nil
	}
	page, err := s.episodes.ListEpisodeRelations(applicationRequestContext(ctx), owner, req.Id, before)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrEpisodeNotFound):
			return relationReadHTTPError(http.StatusNotFound, "EPISODE_NOT_FOUND", "episode unavailable"), nil
		case errors.Is(err, model.ErrInvalidEpisodeEvidence):
			return relationReadHTTPError(http.StatusBadRequest, "INVALID_EPISODE_CURSOR", "invalid Episode relation cursor"), nil
		default:
			return relationReadHTTPError(http.StatusInternalServerError, "EPISODE_RELATION_READ_FAILED", "Episode relation read failed"), nil
		}
	}
	if page == nil || page.UserID != owner || page.EpisodeID != req.Id || page.CurrentRevision < 0 ||
		len(page.Relations) > 20 || page.HasMore != (page.NextBeforeRevision != nil) ||
		(page.NextBeforeRevision != nil && (*page.NextBeforeRevision < 1 || *page.NextBeforeRevision > page.CurrentRevision)) {
		return relationReadHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_RELATIONS", "invalid source-bound relations"), nil
	}
	seen := map[uuid.UUID]bool{}
	for _, rel := range page.Relations {
		if rel.ID == uuid.Nil || seen[rel.ID] || rel.FromEpisodeID == uuid.Nil || rel.ToEpisodeID == uuid.Nil ||
			rel.FromEpisodeID == rel.ToEpisodeID ||
			(rel.FromEpisodeID != req.Id && rel.ToEpisodeID != req.Id) ||
			rel.BasisFactID == uuid.Nil || rel.AuthorizationID == uuid.Nil ||
			rel.CreatedRevision < 1 || rel.CreatedRevision > page.CurrentRevision ||
			rel.Kind != "recurrence_reported" ||
			(rel.ReviewState != "supported" && rel.ReviewState != "needs_review") {
			return relationReadHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_RELATIONS", "invalid source-bound relations"), nil
		}
		seen[rel.ID] = true
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyEpisodeRelationPage]("BodyEpisodeRelationPage", page)
	if err != nil {
		return relationReadHTTPError(http.StatusInternalServerError, "INVALID_EPISODE_RELATIONS", "invalid relation response contract"), nil
	}
	return openapiv1.ListBodyEpisodeRelations200JSONResponse(converted), nil
}
func relationReadHTTPError(status int, code, message string) openapiv1.ListBodyEpisodeRelationsResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.ListBodyEpisodeRelations401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusBadRequest:
		return openapiv1.ListBodyEpisodeRelations400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ListBodyEpisodeRelations404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusServiceUnavailable:
		return openapiv1.ListBodyEpisodeRelations503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(e)}
	default:
		return openapiv1.ListBodyEpisodeRelations500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
