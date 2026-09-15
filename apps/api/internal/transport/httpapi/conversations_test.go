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
	conversations []model.Conversation
	hasMore       bool
}

func (f *fakeConversationApplication) ListConversations(context.Context, uuid.UUID, *time.Time, int) ([]model.Conversation, bool, error) {
	return f.conversations, f.hasMore, nil
}
func (*fakeConversationApplication) GetConversation(context.Context, uuid.UUID, uuid.UUID) (*model.Conversation, []model.Message, error) {
	return nil, nil, nil
}
func (*fakeConversationApplication) GetConversationByID(context.Context, uuid.UUID, uuid.UUID) (*model.Conversation, error) {
	return nil, nil
}
func (*fakeConversationApplication) DeleteConversation(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (*fakeConversationApplication) PinConversation(context.Context, uuid.UUID, uuid.UUID, bool) error {
	return nil
}
func (*fakeConversationApplication) RenameTitle(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (*fakeConversationApplication) GenerateTitle(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (*fakeConversationApplication) UpdateConversationStatus(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (*fakeConversationApplication) ListRuns(context.Context, uuid.UUID, uuid.UUID) ([]model.Run, error) {
	return nil, nil
}

type fakeConversationShareApplication struct {
	shared *model.ConversationShare
}

func (*fakeConversationShareApplication) ShareConversation(context.Context, uuid.UUID, uuid.UUID) (*model.ConversationShare, string, error) {
	return nil, "", nil
}
func (*fakeConversationShareApplication) UnshareConversation(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *fakeConversationShareApplication) GetSharedConversation(context.Context, string) (*model.ConversationShare, error) {
	return f.shared, nil
}

type fakeRuntimeEventApplication struct{}

func (*fakeRuntimeEventApplication) ListRunEvents(context.Context, uuid.UUID, uuid.UUID, int, int) ([]model.RuntimeEvent, bool, error) {
	return nil, false, nil
}

func newConversationRouteTestRouter(
	t *testing.T,
	conversations conversationApplication,
	shares conversationShareApplication,
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
	server := NewPublicServer(nil).WithConversations(conversations, shares, &fakeRuntimeEventApplication{})
	RegisterRoutes(r, StrictHandler(server), RouteSecurity{
		Auth:      authMiddleware,
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r
}

func TestConversationListUsesPublicProjectionWithoutPersistenceInternals(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	conversation := model.Conversation{
		ID:                        uuid.New(),
		UserID:                    uuid.New(),
		Title:                     "Public title",
		TitleStatus:               "generated",
		TitleAgentConfigurationID: "private-config-id",
		TitleAgentConfiguration:   []byte(`{"private":true}`),
		TitleExecutionProvenance:  []byte(`{"provider":"private"}`),
		TitleDecisionTrace:        []byte(`{"trace":"private"}`),
		Status:                    "active",
		Pinned:                    true,
		DefaultModel:              "glm-5.3",
		Provider:                  "private-provider",
		ProviderConversationID:    "private-provider-conversation",
		ActiveRunID:               ptrUUID(uuid.New()),
		Metadata:                  []byte(`{"surface":"consultation"}`),
		CreatedAt:                 now,
		UpdatedAt:                 now,
		LastMessageAt:             &now,
	}
	r := newConversationRouteTestRouter(
		t,
		&fakeConversationApplication{conversations: []model.Conversation{conversation}},
		&fakeConversationShareApplication{},
		nil,
	)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Conversations []map[string]any `json:"conversations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Conversations) != 1 {
		t.Fatalf("conversation count=%d", len(body.Conversations))
	}
	item := body.Conversations[0]
	for _, forbidden := range []string{
		"user_id",
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
			t.Fatalf("public conversation leaked persistence field %q: %v", forbidden, item)
		}
	}
	if item["id"] != conversation.ID.String() || item["title"] != conversation.Title {
		t.Fatalf("public projection lost required fields: %v", item)
	}
}

func TestSharedConversationRouteIsPublicAndDoesNotInvokeBearerAuth(t *testing.T) {
	share := &model.ConversationShare{
		ShareToken:       "public-token",
		SnapshotTitle:    "Shared title",
		SnapshotMessages: []byte(`[]`),
	}
	authCalls := 0
	r := newConversationRouteTestRouter(
		t,
		&fakeConversationApplication{},
		&fakeConversationShareApplication{shared: share},
		func(c *gin.Context) {
			authCalls++
			c.AbortWithStatus(http.StatusTeapot)
		},
	)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/conversations/share/public-token", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authCalls != 0 {
		t.Fatalf("public share unexpectedly invoked bearer auth %d time(s)", authCalls)
	}
	var body openapiv1.SharedConversationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Title != "Shared title" || len(body.Messages) != 0 {
		t.Fatalf("unexpected public share response: %+v", body)
	}
}

func ptrUUID(value uuid.UUID) *uuid.UUID { return &value }
