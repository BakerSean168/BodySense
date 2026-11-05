package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

const episodicNotesControlsEnv = "EPISODIC_NOTES_CONTROLS_ENABLED"

// episodicNoteControlsApplication is the narrow S6-A owner-scoped boundary.
// It deliberately exposes only inspection and deletion: no note generation,
// no model retrieval and no clinical mutation is reachable from this server.
type episodicNoteControlsApplication interface {
	ListOwned(context.Context, uuid.UUID) ([]model.EpisodicNote, error)
	DeleteOwned(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

// EpisodicNotesControlsEnabledFromEnv reads the default-off kill switch.
// Only an explicit true/1 opts in; every other value (including unset) keeps
// the routes externally indistinguishable from not existing.
func EpisodicNotesControlsEnabledFromEnv(getenv func(string) string) bool {
	value := strings.TrimSpace(getenv(episodicNotesControlsEnv))
	return strings.EqualFold(value, "true") || value == "1"
}

func (s *PublicServer) WithEpisodicNotesControls(
	notes episodicNoteControlsApplication,
	enabled bool,
) *PublicServer {
	s.episodicNotes = notes
	s.episodicNotesEnabled = enabled
	return s
}

// episodicNoteSummaryProjection is the only public projection of a note.
// Source message/asset IDs, owner and Conversation IDs stay server-side; the
// sequence range is shown only as provenance for the user's own review.
type episodicNoteSummaryProjection struct {
	ID                     uuid.UUID `json:"id"`
	ContentText            string    `json:"contentText"`
	FromSeq                int       `json:"fromSeq"`
	ToSeq                  int       `json:"toSeq"`
	Status                 string    `json:"status"`
	GeneratorVersion       string    `json:"generatorVersion"`
	RetentionPolicyVersion string    `json:"retentionPolicyVersion"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type episodicNoteListProjection struct {
	Notes []episodicNoteSummaryProjection `json:"notes"`
}

func (s *PublicServer) ListEpisodicNotes(
	ctx context.Context,
	_ openapiv1.ListEpisodicNotesRequestObject,
) (openapiv1.ListEpisodicNotesResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return listEpisodicNotesError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if !s.episodicNotesEnabled || s.episodicNotes == nil {
		// Disabled is indistinguishable from absent. Never reveal whether the
		// control surface exists before product/privacy sign-off.
		return listEpisodicNotesError(http.StatusNotFound, "NOT_FOUND", "episodic notes are not available"), nil
	}
	notes, err := s.episodicNotes.ListOwned(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrEpisodicNoteNotFound) {
			return listEpisodicNotesError(http.StatusNotFound, "NOT_FOUND", "episodic notes are not available"), nil
		}
		return listEpisodicNotesError(http.StatusInternalServerError, "EPISODIC_NOTES_UNAVAILABLE", "unable to read episodic notes"), nil
	}
	projection := episodicNoteListProjection{Notes: make([]episodicNoteSummaryProjection, 0, len(notes))}
	for _, note := range notes {
		projection.Notes = append(projection.Notes, episodicNoteSummaryProjection{
			ID:                     note.ID,
			ContentText:            note.ContentText,
			FromSeq:                note.FromSeq,
			ToSeq:                  note.ToSeq,
			Status:                 note.Status,
			GeneratorVersion:       note.GeneratorVersion,
			RetentionPolicyVersion: note.RetentionPolicyVersion,
			CreatedAt:              note.CreatedAt,
			UpdatedAt:              note.UpdatedAt,
		})
	}
	body, err := strictOpenAPIConvert[openapiv1.EpisodicNoteList]("EpisodicNoteList", projection)
	if err != nil {
		return listEpisodicNotesError(http.StatusInternalServerError, "INTERNAL_ERROR", "episodic note list violates the public contract"), nil
	}
	noStore := "no-store"
	return openapiv1.ListEpisodicNotes200JSONResponse{
		Body: body,
		Headers: openapiv1.ListEpisodicNotes200ResponseHeaders{
			CacheControl: &noStore,
		},
	}, nil
}

func (s *PublicServer) DeleteEpisodicNote(
	ctx context.Context,
	request openapiv1.DeleteEpisodicNoteRequestObject,
) (openapiv1.DeleteEpisodicNoteResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return deleteEpisodicNoteError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if !s.episodicNotesEnabled || s.episodicNotes == nil {
		return deleteEpisodicNoteError(http.StatusNotFound, "NOT_FOUND", "episodic note not found"), nil
	}
	deleted, err := s.episodicNotes.DeleteOwned(ctx, userID, request.NoteId)
	if err != nil {
		if errors.Is(err, repository.ErrEpisodicNoteNotFound) {
			// Foreign, unknown, already-user-deleted by another owner and
			// disabled are all one owner-masked response.
			return deleteEpisodicNoteError(http.StatusNotFound, "NOT_FOUND", "episodic note not found"), nil
		}
		return deleteEpisodicNoteError(http.StatusInternalServerError, "EPISODIC_NOTES_UNAVAILABLE", "unable to delete episodic note"), nil
	}
	message := "episodic note was already removed"
	if deleted {
		message = "episodic note removed"
	}
	noStore := "no-store"
	return openapiv1.DeleteEpisodicNote200JSONResponse{
		Body: openapiv1.EpisodicNoteDeleteResponse{
			NoteId:  request.NoteId,
			Deleted: deleted,
			Message: message,
		},
		Headers: openapiv1.DeleteEpisodicNote200ResponseHeaders{
			CacheControl: &noStore,
		},
	}, nil
}

func listEpisodicNotesError(status int, code, message string) openapiv1.ListEpisodicNotesResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.ListEpisodicNotes401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.ListEpisodicNotes404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	default:
		return openapiv1.ListEpisodicNotes500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func deleteEpisodicNoteError(status int, code, message string) openapiv1.DeleteEpisodicNoteResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusUnauthorized:
		return openapiv1.DeleteEpisodicNote401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.DeleteEpisodicNote404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	default:
		return openapiv1.DeleteEpisodicNote500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
