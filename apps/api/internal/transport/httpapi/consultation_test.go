package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	consultationruntime "github.com/bodysense/api/internal/consultation"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeConsultationRuntime struct {
	startCalls int
	lastUser   uuid.UUID
	lastStart  consultationruntime.StartRunInput
}

func (f *fakeConsultationRuntime) StartRun(_ context.Context, w http.ResponseWriter, uid uuid.UUID, input consultationruntime.StartRunInput) *consultationruntime.HTTPError {
	f.startCalls++
	f.lastUser = uid
	f.lastStart = input
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "event: run.started\ndata: {\"version\":1}\n\n")
	return nil
}
func (*fakeConsultationRuntime) CancelRun(context.Context, uuid.UUID, uuid.UUID, string) *consultationruntime.HTTPError {
	return nil
}
func (*fakeConsultationRuntime) ResumeInteraction(context.Context, http.ResponseWriter, uuid.UUID, uuid.UUID, uuid.UUID, consultationruntime.ResumeInteractionInput) *consultationruntime.HTTPError {
	return nil
}

type fakeConsultationSessions struct{}

func (*fakeConsultationSessions) GetConsultation(context.Context, uuid.UUID, uuid.UUID) (*model.ConsultationSession, error) {
	return nil, nil
}

type fakeConsultationInteractions struct{}

func (*fakeConsultationInteractions) GetPendingInteractions(context.Context, uuid.UUID) ([]model.AgentInteraction, error) {
	return nil, nil
}
func (*fakeConsultationInteractions) GetInteractionMetrics(context.Context, uuid.UUID, *uuid.UUID) (service.InteractionMetrics, error) {
	return service.InteractionMetrics{}, nil
}

type fakeConsultationReplay struct{}

func (*fakeConsultationReplay) HistoricalReplay(context.Context, uuid.UUID, uuid.UUID) (*service.ConsultationRunDecision, error) {
	return nil, service.ErrConsultationReplayUnavailable
}
func (*fakeConsultationReplay) CounterfactualReplay(context.Context, uuid.UUID, uuid.UUID, string) (*service.ConsultationRunDecision, error) {
	return nil, service.ErrConsultationReplayUnavailable
}

type fakeConsultationThreads struct{}

func (*fakeConsultationThreads) RefreshAndGetThread(context.Context, uuid.UUID, uuid.UUID) (*model.ThreadProjection, []model.ThreadProjectionMessage, []model.ThreadProjectionToolCall, *uuid.UUID, []model.RuntimeEvent, error) {
	return nil, nil, nil, nil, nil, nil
}

type fakeConsultationBodyState struct{}

func (*fakeConsultationBodyState) GetSnapshot(context.Context, uuid.UUID, int) (*service.BodyStateSnapshot, error) {
	return nil, nil
}

func newConsultationRouteTestRouter(t *testing.T, runtime consultationRuntimeApplication) (*gin.Engine, uuid.UUID) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	spec, err := openapiv1.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI spec: %v", err)
	}
	r := gin.New()
	server := NewPublicServer(nil).WithConsultation(
		runtime,
		&fakeConsultationSessions{},
		&fakeConsultationInteractions{},
		&fakeConsultationReplay{},
		&fakeConsultationThreads{},
		&fakeConsultationBodyState{},
	)
	RegisterRoutes(r, StrictHandler(server), RouteSecurity{
		Auth: func(c *gin.Context) {
			c.Set("user_id", userID.String())
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r, userID
}

func TestStartConsultationRunStreamsOnceThroughGeneratedBoundary(t *testing.T) {
	runtime := &fakeConsultationRuntime{}
	r, userID := newConsultationRouteTestRouter(t, runtime)
	body := `{
		"conversationId":null,
		"clientMessageId":"tmp-client-1",
		"requestId":"request-1",
		"message":{"role":"user","parts":[{"type":"text","text":"我的右肩不舒服"}],"metadata":{"surface":"body_explorer"}}
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consultation-runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content-type=%q", got)
	}
	if got := rec.Body.String(); got != "event: run.started\ndata: {\"version\":1}\n\n" {
		t.Fatalf("stream was rewritten or duplicated: %q", got)
	}
	if runtime.startCalls != 1 || runtime.lastUser != userID {
		t.Fatalf("runtime calls=%d uid=%s want=%s", runtime.startCalls, runtime.lastUser, userID)
	}
	if runtime.lastStart.RequestID != "request-1" || len(runtime.lastStart.Message.Parts) != 1 || runtime.lastStart.Message.Parts[0].Text != "我的右肩不舒服" {
		t.Fatalf("unexpected mapped runtime input: %+v", runtime.lastStart)
	}
}

func TestStartConsultationRunRejectsInvalidImageBeforeRuntime(t *testing.T) {
	runtime := &fakeConsultationRuntime{}
	r, _ := newConsultationRouteTestRouter(t, runtime)
	body := `{
		"conversationId":null,
		"clientMessageId":"tmp-client-1",
		"requestId":"request-1",
		"message":{"role":"user","parts":[{"type":"image","upload_id":"not-a-uuid"}]}
	}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consultation-runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if runtime.startCalls != 0 {
		t.Fatalf("runtime called for contract-invalid request: %d", runtime.startCalls)
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}
