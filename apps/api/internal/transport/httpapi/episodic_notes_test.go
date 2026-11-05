package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type fakeEpisodicNoteControls struct {
	listFn    func(context.Context, uuid.UUID) ([]model.EpisodicNote, error)
	deleteFn  func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	listOwner uuid.UUID
	calls     int
}

func (f *fakeEpisodicNoteControls) ListOwned(ctx context.Context, owner uuid.UUID) ([]model.EpisodicNote, error) {
	f.calls++
	f.listOwner = owner
	return f.listFn(ctx, owner)
}

func (f *fakeEpisodicNoteControls) DeleteOwned(ctx context.Context, owner, noteID uuid.UUID) (bool, error) {
	f.calls++
	return f.deleteFn(ctx, owner, noteID)
}

// The generated strict handlers pass Gin's *gin.Context, whose Value method
// exposes keys set by the auth middleware. Using that same carrier here keeps
// the test faithful to production and avoids an untyped context key.
func episodicNoteTestContext(owner uuid.UUID) context.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_id", owner.String())
	return c
}

func sampleEpisodicNote(owner, conversation uuid.UUID) model.EpisodicNote {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	return model.EpisodicNote{
		ID:                     uuid.New(),
		UserID:                 owner,
		ConversationID:         conversation,
		FromSeq:                3,
		ToSeq:                  8,
		SourceMessageIDs:       pq.StringArray{uuid.New().String(), uuid.New().String()},
		SourceAssetIDs:         pq.StringArray{uuid.New().String()},
		GeneratorVersion:       "episodic-generator-v1",
		RetentionPolicyVersion: "episodic-retention-v1",
		ContentText:            "用户偏好早上训练。",
		Status:                 "shadow",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

func TestListEpisodicNotesRequiresAuthentication(t *testing.T) {
	server := NewPublicServer().WithEpisodicNotesControls(&fakeEpisodicNoteControls{
		listFn: func(context.Context, uuid.UUID) ([]model.EpisodicNote, error) {
			t.Fatal("repository must not be read without an authenticated owner")
			return nil, nil
		},
	}, true)

	response, err := server.ListEpisodicNotes(context.Background(), openapiv1.ListEpisodicNotesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(openapiv1.ListEpisodicNotes401JSONResponse); !ok {
		t.Fatalf("expected 401, got %T", response)
	}
}

func TestListEpisodicNotesProjectsOnlyOwnedShadowFields(t *testing.T) {
	owner := uuid.New()
	note := sampleEpisodicNote(owner, uuid.New())
	controls := &fakeEpisodicNoteControls{
		listFn: func(context.Context, uuid.UUID) ([]model.EpisodicNote, error) {
			return []model.EpisodicNote{note}, nil
		},
	}
	server := NewPublicServer().WithEpisodicNotesControls(controls, true)

	response, err := server.ListEpisodicNotes(episodicNoteTestContext(owner), openapiv1.ListEpisodicNotesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	ok, hasOK := response.(openapiv1.ListEpisodicNotes200JSONResponse)
	if !hasOK {
		t.Fatalf("expected 200, got %T", response)
	}
	if controls.listOwner != owner {
		t.Fatalf("repository was queried for %s, want caller %s", controls.listOwner, owner)
	}
	if len(ok.Body.Notes) != 1 {
		t.Fatalf("unexpected note count: %d", len(ok.Body.Notes))
	}
	item := ok.Body.Notes[0]
	if item.Id != note.ID || item.ContentText != note.ContentText ||
		item.FromSeq != note.FromSeq || item.ToSeq != note.ToSeq ||
		item.Status != openapiv1.Shadow ||
		item.GeneratorVersion != note.GeneratorVersion ||
		item.RetentionPolicyVersion != note.RetentionPolicyVersion {
		t.Fatalf("public projection lost or changed a visible field: %+v", item)
	}
	if ok.Headers.CacheControl == nil || *ok.Headers.CacheControl != "no-store" {
		t.Fatal("episodic note reads must not be cacheable")
	}

	encoded, err := json.Marshal(ok.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	for _, forbidden := range []string{
		"sourceMessageIds", "sourceAssetIds", "userId", "conversationId",
		note.UserID.String(), note.ConversationID.String(),
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public projection leaked %q: %s", forbidden, body)
		}
	}
	for _, id := range note.SourceMessageIDs {
		if strings.Contains(body, id) {
			t.Fatalf("public projection leaked a source message id: %s", body)
		}
	}
	for _, id := range note.SourceAssetIDs {
		if strings.Contains(body, id) {
			t.Fatalf("public projection leaked a source asset id: %s", body)
		}
	}
}

func TestDeleteEpisodicNoteMasksForeignAndUnknownIDs(t *testing.T) {
	owner := uuid.New()
	controls := &fakeEpisodicNoteControls{
		deleteFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return false, repository.ErrEpisodicNoteNotFound
		},
	}
	server := NewPublicServer().WithEpisodicNotesControls(controls, true)

	for _, noteID := range []uuid.UUID{uuid.New(), uuid.New()} {
		response, err := server.DeleteEpisodicNote(episodicNoteTestContext(owner), openapiv1.DeleteEpisodicNoteRequestObject{NoteId: noteID})
		if err != nil {
			t.Fatal(err)
		}
		notFound, ok := response.(openapiv1.DeleteEpisodicNote404JSONResponse)
		if !ok {
			t.Fatalf("expected owner-masked 404, got %T", response)
		}
		if notFound.Error.Code != "NOT_FOUND" {
			t.Fatalf("unexpected error code: %s", notFound.Error.Code)
		}
	}
}

func TestDeleteEpisodicNoteIsIdempotentForSameOwner(t *testing.T) {
	owner := uuid.New()
	noteID := uuid.New()
	results := []bool{true, false}
	controls := &fakeEpisodicNoteControls{
		deleteFn: func(_ context.Context, gotOwner, gotNote uuid.UUID) (bool, error) {
			if gotOwner != owner || gotNote != noteID {
				t.Fatalf("delete scoped to %s/%s, want %s/%s", gotOwner, gotNote, owner, noteID)
			}
			result := results[0]
			results = results[1:]
			return result, nil
		},
	}
	server := NewPublicServer().WithEpisodicNotesControls(controls, true)

	for attempt, wantDeleted := range []bool{true, false} {
		response, err := server.DeleteEpisodicNote(episodicNoteTestContext(owner), openapiv1.DeleteEpisodicNoteRequestObject{NoteId: noteID})
		if err != nil {
			t.Fatal(err)
		}
		ok, hasOK := response.(openapiv1.DeleteEpisodicNote200JSONResponse)
		if !hasOK {
			t.Fatalf("attempt %d: expected 200, got %T", attempt, response)
		}
		if ok.Body.NoteId != noteID || ok.Body.Deleted != wantDeleted || ok.Body.Message == "" {
			t.Fatalf("attempt %d: unexpected body %+v", attempt, ok.Body)
		}
		if ok.Headers.CacheControl == nil || *ok.Headers.CacheControl != "no-store" {
			t.Fatal("episodic note deletion must not be cacheable")
		}
	}
}

func TestEpisodicNotesControlsDisabledBehavesAsAbsent(t *testing.T) {
	owner := uuid.New()
	controls := &fakeEpisodicNoteControls{
		listFn: func(context.Context, uuid.UUID) ([]model.EpisodicNote, error) {
			t.Fatal("disabled list must not reach the repository")
			return nil, nil
		},
		deleteFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			t.Fatal("disabled delete must not reach the repository")
			return false, nil
		},
	}
	// Default-off means even a wired repository is unreachable.
	server := NewPublicServer().WithEpisodicNotesControls(controls, false)

	list, err := server.ListEpisodicNotes(episodicNoteTestContext(owner), openapiv1.ListEpisodicNotesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := list.(openapiv1.ListEpisodicNotes404JSONResponse); !ok {
		t.Fatalf("expected disabled list 404, got %T", list)
	}
	deleted, err := server.DeleteEpisodicNote(episodicNoteTestContext(owner), openapiv1.DeleteEpisodicNoteRequestObject{NoteId: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := deleted.(openapiv1.DeleteEpisodicNote404JSONResponse); !ok {
		t.Fatalf("expected disabled delete 404, got %T", deleted)
	}
	if controls.calls != 0 {
		t.Fatalf("disabled surface called the repository %d times", controls.calls)
	}
}

func TestListEpisodicNotesRepositoryFailureStaysInternal(t *testing.T) {
	owner := uuid.New()
	server := NewPublicServer().WithEpisodicNotesControls(&fakeEpisodicNoteControls{
		listFn: func(context.Context, uuid.UUID) ([]model.EpisodicNote, error) {
			return nil, errors.New("database unavailable")
		},
	}, true)

	response, err := server.ListEpisodicNotes(episodicNoteTestContext(owner), openapiv1.ListEpisodicNotesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(openapiv1.ListEpisodicNotes500JSONResponse); !ok {
		t.Fatalf("expected 500, got %T", response)
	}
}

func TestEpisodicNotesControlsEnabledFromEnvDefaultsOff(t *testing.T) {
	env := func(value string) func(string) string {
		return func(key string) string {
			if key != episodicNotesControlsEnv {
				t.Fatalf("unexpected env key %q", key)
			}
			return value
		}
	}
	for _, value := range []string{"", "false", "0", "yes", " TRUE "} {
		want := value == " TRUE "
		if got := EpisodicNotesControlsEnabledFromEnv(env(value)); got != want {
			t.Fatalf("%q: got %v, want %v", value, got, want)
		}
	}
	if !EpisodicNotesControlsEnabledFromEnv(env("1")) || !EpisodicNotesControlsEnabledFromEnv(env("true")) {
		t.Fatal("explicit true/1 must enable the surface")
	}
}
