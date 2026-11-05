package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// episodicNotesIntegrationRouter wires the real S6-A repository behind the
// generated public routes, acting as one fixed authenticated user. Passing a
// nil user leaves the request unauthenticated.
func episodicNotesIntegrationRouter(t *testing.T, db *gorm.DB, actingUser *uuid.UUID, enabled bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	router := gin.New()
	RegisterRoutes(
		router,
		StrictHandler(NewPublicServer().WithEpisodicNotesControls(repository.NewEpisodicNoteControlsRepository(db), enabled)),
		RouteSecurity{
			Auth: func(c *gin.Context) {
				if actingUser != nil {
					c.Set("user_id", actingUser.String())
				}
				c.Next()
			},
			Operator:  func(c *gin.Context) { c.Next() },
			Validator: RequestValidator(spec),
		},
	)
	return router
}

func episodicNotesRequest(t *testing.T, router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestEpisodicNotesHTTPControlsPostgres verifies the S6-B public semantics
// against PostgreSQL 18 end to end: authenticated owner-only listing, no
// provenance leakage, owner-masked 404s, idempotent delete and the default-off
// kill switch behaving as an absent route.
func TestEpisodicNotesHTTPControlsPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("needs isolated native PostgreSQL, not live env")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	owner, foreign := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, foreign} {
		if err := db.Exec("INSERT INTO users(id,email) VALUES(?,?)",
			id, "episodic-s6b-"+id.String()+"@example.test").Error; err != nil {
			t.Fatal(err)
		}
		cleanupID := id
		t.Cleanup(func() {
			if err := db.Exec("DELETE FROM users WHERE id=?", cleanupID).Error; err != nil {
				t.Errorf("delete isolated user: %v", err)
			}
		})
	}
	ownerConversation, foreignConversation := uuid.New(), uuid.New()
	for _, pair := range [][2]uuid.UUID{{owner, ownerConversation}, {foreign, foreignConversation}} {
		if err := db.Exec("INSERT INTO conversations(id,user_id) VALUES(?,?)", pair[1], pair[0]).Error; err != nil {
			t.Fatal(err)
		}
	}

	seed := func(user, conversation uuid.UUID, from, to int, content string) model.EpisodicNote {
		t.Helper()
		now := time.Date(2026, 11, 5, 12, from, 0, 0, time.UTC)
		note := model.EpisodicNote{
			ID:                     uuid.New(),
			UserID:                 user,
			ConversationID:         conversation,
			FromSeq:                from,
			ToSeq:                  to,
			SourceMessageIDs:       pq.StringArray{uuid.New().String()},
			SourceAssetIDs:         pq.StringArray{},
			GeneratorVersion:       "episodic-generator-v1",
			RetentionPolicyVersion: "episodic-retention-v1",
			ContentText:            content,
			Status:                 "shadow",
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if err := db.Create(&note).Error; err != nil {
			t.Fatalf("seed episodic note: %v", err)
		}
		return note
	}
	first := seed(owner, ownerConversation, 1, 2, "合成第一条记忆")
	second := seed(owner, ownerConversation, 3, 4, "合成第二条记忆")
	foreignNote := seed(foreign, foreignConversation, 7, 8, "其他账号的记忆")

	ownerRouter := episodicNotesIntegrationRouter(t, db, &owner, true)
	foreignRouter := episodicNotesIntegrationRouter(t, db, &foreign, true)
	disabledRouter := episodicNotesIntegrationRouter(t, db, &owner, false)
	anonymousRouter := episodicNotesIntegrationRouter(t, db, nil, true)

	// Default-off and unauthenticated surfaces never expose data.
	if got := episodicNotesRequest(t, disabledRouter, http.MethodGet, "/api/v1/profile/episodic-notes"); got.Code != http.StatusNotFound {
		t.Fatalf("disabled list status=%d body=%s", got.Code, got.Body.String())
	}
	if got := episodicNotesRequest(t, disabledRouter, http.MethodDelete, "/api/v1/profile/episodic-notes/"+first.ID.String()); got.Code != http.StatusNotFound {
		t.Fatalf("disabled delete status=%d body=%s", got.Code, got.Body.String())
	}
	if got := episodicNotesRequest(t, anonymousRouter, http.MethodGet, "/api/v1/profile/episodic-notes"); got.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list status=%d body=%s", got.Code, got.Body.String())
	}

	list := episodicNotesRequest(t, ownerRouter, http.MethodGet, "/api/v1/profile/episodic-notes")
	if list.Code != http.StatusOK {
		t.Fatalf("owner list status=%d body=%s", list.Code, list.Body.String())
	}
	if cache := list.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("owner list Cache-Control=%q", cache)
	}
	var page openapiv1.EpisodicNoteList
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Notes) != 2 || page.Notes[0].Id != second.ID || page.Notes[1].Id != first.ID {
		t.Fatalf("owner list is not newest-first or not owner-scoped: %+v", page.Notes)
	}
	raw := list.Body.String()
	for _, forbidden := range []string{
		"sourceMessageIds", "sourceAssetIds", "userId", "conversationId",
		owner.String(), ownerConversation.String(),
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("list projection leaked %q: %s", forbidden, raw)
		}
	}

	// A foreign ID and an unknown ID are indistinguishable owner-masked 404s.
	for _, id := range []uuid.UUID{foreignNote.ID, uuid.New()} {
		got := episodicNotesRequest(t, ownerRouter, http.MethodDelete, "/api/v1/profile/episodic-notes/"+id.String())
		if got.Code != http.StatusNotFound {
			t.Fatalf("masked delete %s status=%d body=%s", id, got.Code, got.Body.String())
		}
	}
	if foreignList := episodicNotesRequest(t, foreignRouter, http.MethodGet, "/api/v1/profile/episodic-notes"); foreignList.Code != http.StatusOK {
		t.Fatalf("foreign list status=%d", foreignList.Code)
	}

	removed := episodicNotesRequest(t, ownerRouter, http.MethodDelete, "/api/v1/profile/episodic-notes/"+first.ID.String())
	if removed.Code != http.StatusOK {
		t.Fatalf("owner delete status=%d body=%s", removed.Code, removed.Body.String())
	}
	if cache := removed.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("owner delete Cache-Control=%q", cache)
	}
	var receipt openapiv1.EpisodicNoteDeleteResponse
	if err := json.Unmarshal(removed.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.NoteId != first.ID || !receipt.Deleted {
		t.Fatalf("unexpected delete receipt: %+v", receipt)
	}

	repeated := episodicNotesRequest(t, ownerRouter, http.MethodDelete, "/api/v1/profile/episodic-notes/"+first.ID.String())
	if repeated.Code != http.StatusOK {
		t.Fatalf("idempotent delete status=%d body=%s", repeated.Code, repeated.Body.String())
	}
	if err := json.Unmarshal(repeated.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Deleted {
		t.Fatal("a repeated delete must report no second deletion")
	}

	after := episodicNotesRequest(t, ownerRouter, http.MethodGet, "/api/v1/profile/episodic-notes")
	if after.Code != http.StatusOK {
		t.Fatalf("post-delete list status=%d body=%s", after.Code, after.Body.String())
	}
	if err := json.Unmarshal(after.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Notes) != 1 || page.Notes[0].Id != second.ID {
		t.Fatalf("deleted note survived or untouched note vanished: %+v", page.Notes)
	}

	var tombstones int64
	if err := db.Model(&repository.EpisodicNoteDeletion{}).Where("user_id = ?", owner).Count(&tombstones).Error; err != nil {
		t.Fatal(err)
	}
	if tombstones != 1 {
		t.Fatalf("expected exactly one owner tombstone, got %d", tombstones)
	}
	var remaining int64
	if err := db.Model(&model.EpisodicNote{}).Where("user_id = ?", owner).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("expected one remaining owner note, got %d", remaining)
	}
}
