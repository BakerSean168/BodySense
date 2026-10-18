package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeStatementErasure struct {
	receipt *model.StatementErasureReceipt
	err     error
	key     string
}

func (f *fakeStatementErasure) Request(_ context.Context, _, _ uuid.UUID, key string) (*model.StatementErasureReceipt, error) {
	f.key = key
	return f.receipt, f.err
}

func (f *fakeStatementErasure) Receipt(context.Context, uuid.UUID, uuid.UUID) (*model.StatementErasureReceipt, error) {
	return f.receipt, f.err
}

func statementErasureRouter(t *testing.T, erasure statementErasureApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithStatementErasure(erasure)), RouteSecurity{
		Auth:      func(c *gin.Context) { c.Set("user_id", uuid.NewString()); c.Next() },
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}

func syntheticStatementReceipt(status model.StatementErasureStatus) *model.StatementErasureReceipt {
	now := time.Now().UTC()
	receipt := &model.StatementErasureReceipt{Request: model.StatementErasureRequest{ID: uuid.New(), StatementID: uuid.New(), Status: status,
		InventoryRevision: model.StatementErasureInventoryRevision, RequestedAt: now}}
	for _, name := range model.StatementErasureTargets {
		target := model.StatementErasureTarget{Target: name, State: model.AssetErasureTargetVerified, VerifiedAt: &now}
		switch {
		case name == model.StatementErasureDecisionRecords:
			reason := model.ErasureRetentionReplayInputFrozen
			target = model.StatementErasureTarget{Target: name, State: model.ErasureTargetRetainedByPolicy, OutstandingCount: 3, RetentionReason: &reason}
		case name == model.StatementErasureBodyRecords && status == model.StatementErasureOutstanding:
			code := model.ErasureCodeBodyRecordActive
			target = model.StatementErasureTarget{Target: name, State: model.AssetErasureTargetBlocked, OutstandingCount: 1, FailureCode: &code}
		}
		receipt.Targets = append(receipt.Targets, target)
	}
	if status.Terminal() {
		receipt.Request.ErasedAt = &now
	} else {
		revision := uuid.New()
		receipt.Reviews = []model.ErasureReview{
			{Subject: model.ErasureReviewBodyFact, SubjectID: uuid.New()},
			{Subject: model.ErasureReviewTrainingPlan, SubjectID: uuid.New(), RevisionID: &revision},
		}
	}
	return receipt
}

func doStatementErasure(router *gin.Engine, method, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/body-state/statements/"+uuid.NewString()+"/erasure", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestStatementErasureHTTPReceipts(t *testing.T) {
	outstanding := &fakeStatementErasure{receipt: syntheticStatementReceipt(model.StatementErasureOutstanding)}
	rec := doStatementErasure(statementErasureRouter(t, outstanding), http.MethodPost, `{"idempotency_key":"erase-1"}`)
	if rec.Code != http.StatusAccepted || outstanding.key != "erase-1" {
		t.Fatalf("status=%d key=%q body=%s", rec.Code, outstanding.key, rec.Body.String())
	}
	var view openapiv1.StatementErasureReceipt
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.OutstandingTargets) != 1 || len(view.RetainedTargets) != 1 || view.RetainedTargets[0] != openapiv1.StatementErasureTargetName("decision_records") ||
		len(view.Reviews) != 2 || view.Reviews[1].RevisionId == nil {
		t.Fatalf("view=%+v", view)
	}
	for _, target := range view.Targets {
		if target.Target == "decision_records" && (target.RetentionReason == nil || *target.RetentionReason != "replay_input_frozen" || target.FailureCode != nil) {
			t.Fatalf("retained target=%+v", target)
		}
	}

	retained := &fakeStatementErasure{receipt: syntheticStatementReceipt(model.StatementErasureErasedWithRetention)}
	rec = doStatementErasure(statementErasureRouter(t, retained), http.MethodPost, `{"idempotency_key":"erase-1"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"erased_with_retention"`) ||
		!strings.Contains(rec.Body.String(), `"outstanding_targets":[]`) || !strings.Contains(rec.Body.String(), `"reviews":[]`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doStatementErasure(statementErasureRouter(t, retained), http.MethodGet, ""); rec.Code != http.StatusOK {
		t.Fatalf("get status=%d", rec.Code)
	}
}

func TestStatementErasureHTTPErrors(t *testing.T) {
	cases := []struct {
		name, method, body string
		app                statementErasureApplication
		status             int
		code               string
	}{
		{"missing key", http.MethodPost, `{}`, &fakeStatementErasure{}, http.StatusBadRequest, ""},
		{"foreign statement", http.MethodPost, `{"idempotency_key":"k"}`, &fakeStatementErasure{err: model.ErrStatementErasureNotFound}, http.StatusNotFound, "NOT_FOUND"},
		{"key reuse", http.MethodPost, `{"idempotency_key":"k"}`, &fakeStatementErasure{err: model.ErrStatementErasureIdempotencyUsed}, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED"},
		{"no receipt", http.MethodGet, "", &fakeStatementErasure{err: model.ErrStatementErasureNotFound}, http.StatusNotFound, "NOT_FOUND"},
		{"unconfigured", http.MethodGet, "", nil, http.StatusServiceUnavailable, "STATEMENT_ERASURE_UNAVAILABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doStatementErasure(statementErasureRouter(t, tc.app), tc.method, tc.body)
			if rec.Code != tc.status || (tc.code != "" && !strings.Contains(rec.Body.String(), tc.code)) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
