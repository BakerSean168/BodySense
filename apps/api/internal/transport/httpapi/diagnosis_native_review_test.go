package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeNativeReviewPrompt struct {
	receipt *model.DiagnosisReviewPromptDismissal
	err     error
	owner   uuid.UUID
	cmd     service.DiagnosisReviewPromptDismissCommand
	called  int
}

func (f *fakeNativeReviewPrompt) Dismiss(_ context.Context, owner uuid.UUID, cmd service.DiagnosisReviewPromptDismissCommand) (*model.DiagnosisReviewPromptDismissal, error) {
	f.owner, f.cmd = owner, cmd
	f.called++
	return f.receipt, f.err
}

type fakeNativeAnalysisRequest struct {
	evidence *model.StatementEvidence
	replayed bool
	err      error
	owner    uuid.UUID
	message  uuid.UUID
	called   int
}

func (f *fakeNativeAnalysisRequest) Bind(_ context.Context, owner, message uuid.UUID) (*model.StatementEvidence, bool, error) {
	f.owner, f.message = owner, message
	f.called++
	return f.evidence, f.replayed, f.err
}

func nativeReviewRoute(t *testing.T, owner uuid.UUID, requests nativeDiagnosisAnalysisRequestApplication, prompts nativeDiagnosisReviewPromptApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer().WithNativeDiagnosisReview(requests, prompts)), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return r
}

func testReviewPromptPayload() map[string]any {
	return map[string]any{"expectedRevision": int64(2), "evaluatedBodyStateRevision": int64(9), "idempotencyKey": "synthetic-dismiss-1"}
}

func testReviewPromptReceipt(owner, condition uuid.UUID) *model.DiagnosisReviewPromptDismissal {
	return &model.DiagnosisReviewPromptDismissal{
		UserID: owner, ID: uuid.New(), ConditionID: condition, BasedOnBodyStateRevision: 4, DismissedThroughBodyStateRevision: 9,
		DiagnosisRevision: 2, ReasonCodes: []string{"related_fact_recorded"}, IdempotencyKey: "synthetic-dismiss-1", DismissedAt: time.Now().UTC(),
	}
}

func TestNativeDiagnosisReviewPromptTransport(t *testing.T) {
	owner, condition := uuid.New(), uuid.New()
	path := "/api/v1/diagnosis/conditions/" + condition.String() + "/review-prompt-dismissals"
	fake := &fakeNativeReviewPrompt{receipt: testReviewPromptReceipt(owner, condition)}
	r := nativeReviewRoute(t, owner, nil, fake)
	result := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, testReviewPromptPayload()), http.StatusOK)
	if fake.called != 1 || fake.owner != owner || fake.cmd.ConditionID != condition || fake.cmd.ExpectedRevision != 2 ||
		fake.cmd.EvaluatedBodyStateRevision != 9 || fake.cmd.IdempotencyKey != "synthetic-dismiss-1" ||
		result["dismissalId"] != fake.receipt.ID.String() || result["dismissedThroughBodyStateRevision"] != float64(9) {
		t.Fatalf("bad dismissal binding: cmd=%+v response=%#v", fake.cmd, result)
	}
	for name, mutate := range map[string]func(map[string]any){
		"owner spoof":        func(p map[string]any) { p["userId"] = uuid.NewString() },
		"edits diagnosis":    func(p map[string]any) { p["disposition"] = "inactive" },
		"zero revision":      func(p map[string]any) { p["expectedRevision"] = 0 },
		"missing evaluated":  func(p map[string]any) { delete(p, "evaluatedBodyStateRevision") },
		"padded idempotency": func(p map[string]any) { p["idempotencyKey"] = " key" },
	} {
		payload := testReviewPromptPayload()
		mutate(payload)
		if got := postNativeDiagnosis(t, r, path, payload); got.Code != http.StatusBadRequest {
			t.Fatalf("%s accepted: %d %s", name, got.Code, got.Body.String())
		}
	}
	if fake.called != 1 {
		t.Fatalf("invalid requests reached the writer: %d", fake.called)
	}
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{model.ErrDiagnosisNotFound, 404, "NOT_FOUND"}, {service.ErrDiagnosisDecisionConflict, 409, "REVISION_CONFLICT"},
		{service.ErrDiagnosisReviewPromptNotShown, 409, "REVIEW_PROMPT_NOT_SHOWN"},
		{service.ErrDiagnosisReviewPromptKeyReused, 409, "IDEMPOTENCY_KEY_REUSED"},
		{service.ErrDiagnosisReviewPromptInvalid, 400, "INVALID_REQUEST"}, {errors.New("storage down"), 503, "DIAGNOSIS_UNAVAILABLE"},
	} {
		fake.err = test.err
		got := postNativeDiagnosis(t, r, path, testReviewPromptPayload())
		data := assertNativeReceiptHTTP(t, got, test.status)
		if errBody, _ := data["error"].(map[string]any); errBody == nil || errBody["code"] != test.code {
			t.Fatalf("%v: body=%s", test.err, got.Body.String())
		}
	}
	fake.err = nil
	fake.receipt = testReviewPromptReceipt(uuid.New(), condition)
	if got := postNativeDiagnosis(t, r, path, testReviewPromptPayload()); got.Code != 503 {
		t.Fatalf("foreign receipt accepted: %d %s", got.Code, got.Body.String())
	}
	moved := testReviewPromptReceipt(owner, condition)
	moved.DismissedThroughBodyStateRevision = 10
	fake.receipt = moved
	if got := postNativeDiagnosis(t, r, path, testReviewPromptPayload()); got.Code != 503 {
		t.Fatalf("receipt for another prompt accepted: %d %s", got.Code, got.Body.String())
	}
	if got := postNativeDiagnosis(t, nativeReviewRoute(t, owner, nil, nil), path, testReviewPromptPayload()); got.Code != 503 {
		t.Fatalf("unwired dismissal accepted: %d", got.Code)
	}
	if got := postNativeDiagnosis(t, nativeReviewRoute(t, uuid.Nil, nil, fake), path, testReviewPromptPayload()); got.Code != 401 {
		t.Fatalf("unauthenticated dismissal accepted: %d", got.Code)
	}
}

func TestNativeDiagnosisAnalysisRequestTransport(t *testing.T) {
	owner, message, conversation := uuid.New(), uuid.New(), uuid.New()
	path := "/api/v1/diagnosis/analysis-requests"
	evidence := &model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: 1,
		SourceConversationID: &conversation, SourceMessageID: &message, Excerpt: "帮我正式分析一下头痛。", Availability: "available", RecordedAt: time.Now().UTC()}
	fake := &fakeNativeAnalysisRequest{evidence: evidence}
	r := nativeReviewRoute(t, owner, fake, nil)
	result := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, map[string]any{"messageId": message}), http.StatusCreated)
	if fake.called != 1 || fake.owner != owner || fake.message != message || result["requestEvidenceId"] != evidence.ID.String() ||
		result["excerpt"] != evidence.Excerpt || result["conversationId"] != conversation.String() {
		t.Fatalf("bad request binding: %#v", result)
	}
	fake.replayed = true
	assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, map[string]any{"messageId": message}), http.StatusOK)
	for name, payload := range map[string]map[string]any{
		"missing message": {}, "agent statement": {"messageId": message, "excerpt": "AI wrote this"}, "not uuid": {"messageId": "x"},
	} {
		if got := postNativeDiagnosis(t, r, path, payload); got.Code != http.StatusBadRequest {
			t.Fatalf("%s accepted: %d %s", name, got.Code, got.Body.String())
		}
	}
	for _, test := range []struct {
		err    error
		status int
	}{{model.ErrDiagnosisRequestSourceNotFound, 404}, {model.ErrDiagnosisRequestSourceInvalid, 400}, {errors.New("down"), 503}} {
		fake.err = test.err
		if got := postNativeDiagnosis(t, r, path, map[string]any{"messageId": message}); got.Code != test.status {
			t.Fatalf("%v: %d %s", test.err, got.Code, got.Body.String())
		}
	}
	fake.err = nil
	other := uuid.New()
	fake.evidence = &model.StatementEvidence{ID: uuid.New(), UserID: owner, SourceKind: "user_message", SourceVersion: 1,
		SourceConversationID: &conversation, SourceMessageID: &other, Excerpt: "x", Availability: "available", RecordedAt: time.Now().UTC()}
	if got := postNativeDiagnosis(t, r, path, map[string]any{"messageId": message}); got.Code != 503 {
		t.Fatalf("evidence for another message accepted: %d", got.Code)
	}
	if got := postNativeDiagnosis(t, nativeReviewRoute(t, uuid.Nil, fake, nil), path, map[string]any{"messageId": message}); got.Code != 401 {
		t.Fatalf("unauthenticated request accepted: %d", got.Code)
	}
}
