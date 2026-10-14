package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type retirementShareSpy struct {
	reads   int
	creates int
	revokes int
}

func (s *retirementShareSpy) GetSharedConversation(context.Context, string) (*model.ConversationShare, error) {
	s.reads++
	return &model.ConversationShare{
		ShareToken:       "synthetic-secret-token",
		SnapshotTitle:    "SYNTHETIC_PRIVATE_MEDICAL_TITLE",
		SnapshotMessages: []byte("[{\"role\":\"user\",\"text\":\"SYNTHETIC_PRIVATE_MEDICAL_TEXT\"}]"),
	}, nil
}

func (s *retirementShareSpy) ShareConversation(context.Context, uuid.UUID, uuid.UUID) (*model.ConversationShare, string, error) {
	s.creates++
	return &model.ConversationShare{ShareToken: "synthetic-secret-token"}, "synthetic://share", nil
}

func (s *retirementShareSpy) UnshareConversation(context.Context, uuid.UUID, uuid.UUID) error {
	s.revokes++
	return nil
}

func TestPCD8RetiredChatSharingBlocksReadsAndCreatesButAllowsRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	spy := &retirementShareSpy{}
	r := gin.New()
	RegisterRoutes(r,
		StrictHandler(NewPublicServer(nil).WithConversations(
			&fakeConversationApplication{}, spy, &fakeRuntimeEventApplication{},
		)),
		RouteSecurity{
			Auth: func(c *gin.Context) {
				// Independent negative check: unauthenticated clients may
				// neither share nor revoke another owner's legacy tokens.
				if c.GetHeader("X-Test-Authenticated") != "yes" {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				c.Set("user_id", uuid.NewString())
				c.Next()
			},
			Operator:         func(c *gin.Context) { c.Next() },
			Validator:        RequestValidator(spec),
			RetireChatShares: true,
		},
	)
	conversationID := uuid.NewString()
	cases := []struct {
		label         string
		method        string
		path          string
		status        int
		authenticated bool
	}{
		{"valid old public token", http.MethodGet, "/api/v1/shared-conversations/synthetic-secret-token", http.StatusNotFound, false},
		{"malformed old public token", http.MethodGet, "/api/v1/shared-conversations/invalid-token!", http.StatusNotFound, false},
		{"unauthenticated revoke old token", http.MethodDelete, "/api/v1/conversations/" + conversationID + "/share", http.StatusUnauthorized, false},
		{"unauthenticated create share", http.MethodPost, "/api/v1/conversations/" + conversationID + "/share", http.StatusUnauthorized, false},
		{"authenticated create share", http.MethodPost, "/api/v1/conversations/" + conversationID + "/share", http.StatusNotFound, true},
		{"authenticated revoke old token", http.MethodDelete, "/api/v1/conversations/" + conversationID + "/share", http.StatusOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.authenticated {
				req.Header.Set("X-Test-Authenticated", "yes")
			}
			r.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.status, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "SYNTHETIC_PRIVATE_MEDICAL") ||
				strings.Contains(rec.Body.String(), "synthetic-secret-token") {
				t.Fatal("public health snapshot leaked through retired endpoint")
			}
		})
	}
	if spy.reads != 0 || spy.creates != 0 || spy.revokes != 1 {
		t.Fatalf("sharing provider calls: read=%d create=%d revoke=%d; want 0,0,1",
			spy.reads, spy.creates, spy.revokes)
	}
}
