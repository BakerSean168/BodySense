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

type episodeReadApplication interface {
	ListEpisodes(context.Context, uuid.UUID, int64) (*model.EpisodePage, error)
	GetEpisode(context.Context, uuid.UUID, uuid.UUID) (*model.EpisodeRecord, error)
}

func (s *PublicServer) WithEpisodes(read episodeReadApplication) *PublicServer {
	s.episodes = read
	return s
}

func (s *PublicServer) ListBodyEpisodes(ctx context.Context, request openapiv1.ListBodyEpisodesRequestObject) (openapiv1.ListBodyEpisodesResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return listBodyEpisodesError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.episodes == nil {
		return listBodyEpisodesError(http.StatusServiceUnavailable, "EPISODE_READ_UNAVAILABLE", "episode read service unavailable"), nil
	}
	before := int64(0)
	if request.Params.BeforeRevision != nil {
		before = *request.Params.BeforeRevision
	}
	if before < 0 {
		return listBodyEpisodesError(http.StatusBadRequest, "INVALID_EPISODE_CURSOR", "invalid episode cursor"), nil
	}
	page, err := s.episodes.ListEpisodes(applicationRequestContext(ctx), owner, before)
	if err != nil {
		if errors.Is(err, model.ErrInvalidEpisodeEvidence) {
			return listBodyEpisodesError(http.StatusBadRequest, "INVALID_EPISODE_CURSOR", "invalid episode cursor"), nil
		}
		return listBodyEpisodesError(http.StatusInternalServerError, "EPISODE_READ_FAILED", "episode read failed"), nil
	}
	if page == nil || page.UserID != owner || page.CurrentRevision < 0 ||
		(page.HasMore != (page.NextBeforeRevision != nil)) ||
		(page.NextBeforeRevision != nil &&
			(*page.NextBeforeRevision < 1 || *page.NextBeforeRevision > page.CurrentRevision)) {
		return listBodyEpisodesError(http.StatusInternalServerError, "INVALID_EPISODE_PROJECTION", "episode source projection unavailable"), nil
	}
	for _, episode := range page.Episodes {
		if episode.UserID != owner || !validEpisodeRecord(episode, page.CurrentRevision) {
			return listBodyEpisodesError(http.StatusInternalServerError, "INVALID_EPISODE_PROJECTION", "episode source projection unavailable"), nil
		}
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyEpisodePage]("BodyEpisodePage", page)
	if err != nil {
		return listBodyEpisodesError(http.StatusInternalServerError, "INVALID_EPISODE_PROJECTION", "episode projection violates the public contract"), nil
	}
	return openapiv1.ListBodyEpisodes200JSONResponse(converted), nil
}

func (s *PublicServer) GetBodyEpisode(ctx context.Context, request openapiv1.GetBodyEpisodeRequestObject) (openapiv1.GetBodyEpisodeResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return getBodyEpisodeError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if request.Id == uuid.Nil {
		return getBodyEpisodeError(http.StatusBadRequest, "INVALID_EPISODE_ID", "episode identity required"), nil
	}
	if s.episodes == nil {
		return getBodyEpisodeError(http.StatusServiceUnavailable, "EPISODE_READ_UNAVAILABLE", "episode read service unavailable"), nil
	}
	episode, err := s.episodes.GetEpisode(applicationRequestContext(ctx), owner, request.Id)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrEpisodeNotFound):
			return getBodyEpisodeError(http.StatusNotFound, "EPISODE_NOT_FOUND", "episode unavailable"), nil
		case errors.Is(err, model.ErrInvalidEpisodeEvidence):
			return getBodyEpisodeError(http.StatusBadRequest, "INVALID_EPISODE_ID", "invalid episode identity"), nil
		default:
			return getBodyEpisodeError(http.StatusInternalServerError, "EPISODE_READ_FAILED", "episode read failed"), nil
		}
	}
	if episode == nil || episode.ID != request.Id || episode.UserID != owner ||
		!validEpisodeRecord(*episode, episode.Projection.BasedOnRevision) {
		return getBodyEpisodeError(http.StatusInternalServerError, "INVALID_EPISODE_PROJECTION", "episode source projection unavailable"), nil
	}
	converted, err := strictOpenAPIConvert[openapiv1.BodyEpisodeRecord]("BodyEpisodeRecord", episode)
	if err != nil {
		return getBodyEpisodeError(http.StatusInternalServerError, "INVALID_EPISODE_PROJECTION", "episode projection violates the public contract"), nil
	}
	return openapiv1.GetBodyEpisode200JSONResponse(converted), nil
}

// Defensive integrity checks guard the transport even when a fake, stale or
// incorrectly wired application returns a success-shaped object. These do not
// replace the source ownership/availability checks in the PostgreSQL reader.
func validEpisodeRecord(record model.EpisodeRecord, revision int64) bool {
	if record.ID == uuid.Nil || record.CreatedRevision <= 0 || record.CreatedRevision > revision ||
		record.Projection.BasedOnRevision != revision {
		return false
	}
	linked := make(map[uuid.UUID]bool, len(record.Links))
	for _, link := range record.Links {
		if link.FactID == uuid.Nil || linked[link.FactID] ||
			link.LinkedRevision <= 0 || link.LinkedRevision > revision {
			return false
		}
		linked[link.FactID] = true
	}
	if record.IdentityStatus == "current" && len(linked) == 0 {
		return false
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range record.Projection.BasisFactIDs {
		if id == uuid.Nil || seen[id] || !linked[id] {
			return false
		}
		seen[id] = true
	}
	switch record.Projection.State {
	case "ongoing":
		return record.IdentityStatus == "current" && record.Projection.Reason == "explicit_current_report" && len(seen) > 0
	case "ended":
		return record.IdentityStatus == "current" && record.Projection.Reason == "explicit_resolution" && len(seen) > 0
	case "unknown":
		return (record.Projection.Reason == "insufficient_evidence" && len(seen) == 0) ||
			(record.IdentityStatus == "current" && record.Projection.Reason == "conflicting_evidence" && len(seen) >= 2)
	default:
		return false
	}
}

func listBodyEpisodesError(status int, code, message string) openapiv1.ListBodyEpisodesResponseObject {
	envelope := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.ListBodyEpisodes400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(envelope)}
	case http.StatusUnauthorized:
		return openapiv1.ListBodyEpisodes401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(envelope)}
	case http.StatusServiceUnavailable:
		return openapiv1.ListBodyEpisodes503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(envelope)}
	default:
		return openapiv1.ListBodyEpisodes500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(envelope)}
	}
}

func getBodyEpisodeError(status int, code, message string) openapiv1.GetBodyEpisodeResponseObject {
	envelope := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.GetBodyEpisode400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(envelope)}
	case http.StatusUnauthorized:
		return openapiv1.GetBodyEpisode401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(envelope)}
	case http.StatusNotFound:
		return openapiv1.GetBodyEpisode404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(envelope)}
	case http.StatusServiceUnavailable:
		return openapiv1.GetBodyEpisode503JSONResponse{ValidationUnavailableJSONResponse: openapiv1.ValidationUnavailableJSONResponse(envelope)}
	default:
		return openapiv1.GetBodyEpisode500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(envelope)}
	}
}
