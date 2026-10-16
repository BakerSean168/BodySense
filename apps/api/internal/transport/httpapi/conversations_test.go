package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeConversationApplication struct {
	conversation *model.Conversation
	callers      []uuid.UUID
}

func (f *fakeConversationApplication) GetOrCreateConversation(_ context.Context, userID uuid.UUID) (*model.Conversation, []model.Message, error) {
	f.callers = append(f.callers, userID)
	return f.conversation, nil, nil
}

func (*fakeConversationApplication) GetConversationByID(context.Context, uuid.UUID, uuid.UUID) (*model.Conversation, error) {
	return nil, nil
}

func (*fakeConversationApplication) ListRuns(context.Context, uuid.UUID, uuid.UUID) ([]model.Run, error) {
	return nil, nil
}

type fakeRuntimeEventApplication struct{}

func (*fakeRuntimeEventApplication) ListRunEvents(context.Context, uuid.UUID, uuid.UUID, int, int) ([]model.RuntimeEvent, bool, error) {
	return nil, false, nil
}

func newConversationRouteTestRouter(
	t *testing.T,
	conversations conversationApplication,
	authMiddleware gin.HandlerFunc,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	if authMiddleware == nil {
		authMiddleware = func(c *gin.Context) {
			c.Set("user_id", uuid.NewString())
			c.Next()
		}
	}
	r := gin.New()
	server := NewPublicServer().WithConversations(conversations, &fakeRuntimeEventApplication{})
	RegisterRoutes(r, StrictHandler(server), RouteSecurity{
		Auth:      authMiddleware,
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r
}

func TestGetConversationReturnsSingletonPublicProjectionWithoutPersistenceInternals(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	owner := uuid.New()
	conversation := &model.Conversation{
		ID:                     uuid.New(),
		UserID:                 owner,
		Status:                 model.ConversationStatusActive,
		DefaultModel:           "glm-5.3",
		Provider:               "private-provider",
		ProviderConversationID: "private-provider-conversation",
		ActiveRunID:            ptrUUID(uuid.New()),
		Metadata:               []byte(`{"surface":"consultation"}`),
		CreatedAt:              now,
		UpdatedAt:              now,
		LastMessageAt:          &now,
	}
	app := &fakeConversationApplication{conversation: conversation}
	r := newConversationRouteTestRouter(t, app, func(c *gin.Context) {
		c.Set("user_id", owner.String())
		c.Next()
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/conversation", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(app.callers) != 1 || app.callers[0] != owner {
		t.Fatalf("get-or-create must be scoped to the authenticated owner, got %v", app.callers)
	}

	var body struct {
		Conversation map[string]any   `json:"conversation"`
		Messages     []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	item := body.Conversation
	for _, forbidden := range []string{
		"user_id",
		"title",
		"title_status",
		"pinned",
		"pinned_at",
		"title_agent_configuration_id",
		"title_agent_configuration",
		"title_execution_provenance",
		"title_decision_trace",
		"provider",
		"provider_conversation_id",
		"provider_last_response_id",
		"active_run_id",
		"active_stream_id",
		"deleted_at",
	} {
		if _, ok := item[forbidden]; ok {
			t.Fatalf("public conversation leaked field %q: %v", forbidden, item)
		}
	}
	if item["id"] != conversation.ID.String() || item["status"] != "active" {
		t.Fatalf("public projection lost required fields: %v", item)
	}
}

func TestGetConversationRequiresAuthentication(t *testing.T) {
	app := &fakeConversationApplication{conversation: &model.Conversation{ID: uuid.New()}}
	r := newConversationRouteTestRouter(t, app, func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/conversation", nil))
	if rec.Code != http.StatusUnauthorized || len(app.callers) != 0 {
		t.Fatalf("unauthenticated request must not reach get-or-create: status=%d callers=%d", rec.Code, len(app.callers))
	}
}

// TestRetiredMultiConversationAndShareRoutesAreGone proves the multi-session
// and public-sharing surfaces no longer exist (PC-D4/PC-D8), whether or not
// the caller is authenticated.
func TestRetiredMultiConversationAndShareRoutesAreGone(t *testing.T) {
	authCalls := 0
	r := newConversationRouteTestRouter(t, &fakeConversationApplication{conversation: &model.Conversation{ID: uuid.New()}}, func(c *gin.Context) {
		authCalls++
		c.Set("user_id", uuid.NewString())
		c.Next()
	})
	id := uuid.NewString()
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/conversations"},
		{http.MethodPost, "/api/v1/conversations"},
		{http.MethodGet, "/api/v1/conversations/" + id},
		{http.MethodPatch, "/api/v1/conversations/" + id},
		{http.MethodDelete, "/api/v1/conversations/" + id},
		{http.MethodPatch, "/api/v1/conversations/" + id + "/pin"},
		{http.MethodPut, "/api/v1/conversations/" + id + "/title"},
		{http.MethodPost, "/api/v1/conversations/" + id + "/title"},
		{http.MethodPost, "/api/v1/conversations/" + id + "/share"},
		{http.MethodDelete, "/api/v1/conversations/" + id + "/share"},
		{http.MethodGet, "/api/v1/shared-conversations/public-token"},
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, nil))
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s must be gone, got %d body=%s", route.method, route.path, rec.Code, rec.Body.String())
		}
	}
	if authCalls != 0 {
		t.Fatalf("retired routes must not reach any handler chain, auth ran %d time(s)", authCalls)
	}
}

func ptrUUID(value uuid.UUID) *uuid.UUID { return &value }
