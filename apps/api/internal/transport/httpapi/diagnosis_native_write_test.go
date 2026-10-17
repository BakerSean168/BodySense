package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeNativeDecision struct {
	receipt *model.DiagnosisDecision
	err     error
	owner   uuid.UUID
	cmd     service.DiagnosisProposalDecisionCommand
	called  int
}

func (f *fakeNativeDecision) Record(_ context.Context, owner uuid.UUID, cmd service.DiagnosisProposalDecisionCommand) (*model.DiagnosisDecision, error) {
	f.owner = owner
	f.cmd = cmd
	f.called++
	return f.receipt, f.err
}

type fakeNativeSettlement struct {
	receipt *model.DiagnosisSettlement
	err     error
	owner   uuid.UUID
	cmd     service.DiagnosisSettlementCommand
	called  int
}

func (f *fakeNativeSettlement) Settle(_ context.Context, owner uuid.UUID, cmd service.DiagnosisSettlementCommand) (*model.DiagnosisSettlement, error) {
	f.owner = owner
	f.cmd = cmd
	f.called++
	return f.receipt, f.err
}
func nativeDiagnosisCommandRoute(t *testing.T, owner uuid.UUID, dec nativeDiagnosisDecisionApplication, set nativeDiagnosisSettlementApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer().WithNativeDiagnosisCommands(dec, set)), RouteSecurity{
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
func postNativeDiagnosis(t *testing.T, router *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(content))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(r, req)
	return r
}
func assertNativeReceiptHTTP(t *testing.T, response *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d want=%d body=%s", response.Code, status, response.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if _, leak := data["user_id"]; leak {
		t.Fatal("leaked private SQL owner")
	}
	return data
}
func testDecisionPayload(id uuid.UUID) map[string]any {
	return map[string]any{
		"proposalId": id, "presentedFingerprint": strings.Repeat("a", 64),
		"expectedRevision": int64(3), "decision": "accept",
		"toolCallId": "confirmed-user-tool", "idempotencyKey": "synthetic-command-1",
	}
}
func TestNativeDiagnosisDecisionTransportAndNoAdoption(t *testing.T) {
	owner, run, proposal := uuid.New(), uuid.New(), uuid.New()
	id, receiptID, evidence := uuid.New(), uuid.New(), uuid.New()
	d := &fakeNativeDecision{receipt: &model.DiagnosisDecision{
		UserID: owner, ID: id, RunID: run, ProposalID: proposal,
		ContentFingerprint: strings.Repeat("a", 64), ExpectedRevision: 3,
		DecidedEvidenceRefs: []model.DiagnosisEvidenceRef{{EvidenceType: model.DiagnosisBodyFact, EvidenceID: evidence, Version: "1"}},
		Decision:            model.DiagnosisAccept, ToolCallID: "confirmed-user-tool",
		IdempotencyKey: "synthetic-command-1", ReceiptID: receiptID, DecidedAt: time.Now().UTC(),
	}}
	path := "/api/v1/diagnosis/analysis-runs/" + run.String() + "/decisions"
	r := nativeDiagnosisCommandRoute(t, owner, d, &fakeNativeSettlement{})
	result := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, testDecisionPayload(proposal)), http.StatusOK)
	if d.called != 1 || d.owner != owner || d.cmd.RunID != run || d.cmd.ProposalID != proposal || d.cmd.Decision != model.DiagnosisAccept ||
		result["decisionId"] != id.String() || result["receiptId"] != receiptID.String() || result["decision"] != "accept" {
		t.Fatalf("bad decision binding: cmd=%+v response=%#v", d.cmd, result)
	}
	if _, adopt := result["diagnosisRevisionId"]; adopt {
		t.Fatalf("decision falsely claimed adoption: %#v", result)
	}
	refs, ok := result["decidedEvidenceRefs"].([]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("typed user decision source missing: %#v", result)
	}
	again := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, testDecisionPayload(proposal)), http.StatusOK)
	if again["receiptId"] != result["receiptId"] {
		t.Fatal("idempotent decision replay receipt drift")
	}
	malformed := testDecisionPayload(proposal)
	malformed["presentedFingerprint"] = "mismatched"
	if rec := postNativeDiagnosis(t, r, path, malformed); rec.Code != 400 {
		t.Fatalf("fingerprint must fail validator: %d %s", rec.Code, rec.Body.String())
	}
	withOwner := testDecisionPayload(proposal)
	withOwner["userId"] = uuid.NewString()
	if rec := postNativeDiagnosis(t, r, path, withOwner); rec.Code != 400 {
		t.Fatalf("owner field must be rejected: %d %s", rec.Code, rec.Body.String())
	}
	if d.called != 2 {
		t.Fatalf("invalid requests reached owner writer: %d", d.called)
	}
}
func TestNativeDiagnosisDecisionOwnerErrorsAndReceiptGuards(t *testing.T) {
	owner, run, proposal := uuid.New(), uuid.New(), uuid.New()
	path := "/api/v1/diagnosis/analysis-runs/" + run.String() + "/decisions"
	fake := &fakeNativeDecision{err: model.ErrDiagnosisNotFound}
	r := nativeDiagnosisCommandRoute(t, owner, fake, nil)
	for _, test := range []struct {
		err    error
		status int
	}{
		{model.ErrDiagnosisNotFound, 404}, {service.ErrDiagnosisDecisionConflict, 409},
		{service.ErrDiagnosisSafetyBlocked, 409}, {service.ErrDiagnosisSafetyUnavailable, 503},
		{service.ErrDiagnosisDecisionInvalid, 400}, {errors.New("storage down"), 503},
	} {
		fake.err = test.err
		got := postNativeDiagnosis(t, r, path, testDecisionPayload(proposal))
		if got.Code != test.status {
			t.Fatalf("%v: status=%d want=%d body=%s", test.err, got.Code, test.status, got.Body.String())
		}
	}
	fake.err = nil
	fake.receipt = &model.DiagnosisDecision{UserID: uuid.New(), RunID: run, ProposalID: proposal, ID: uuid.New(), ReceiptID: uuid.New()}
	if got := postNativeDiagnosis(t, r, path, testDecisionPayload(proposal)); got.Code != 503 {
		t.Fatalf("foreign/invalid decision receipt accepted: %d %s", got.Code, got.Body.String())
	}
	unauth := nativeDiagnosisCommandRoute(t, uuid.Nil, fake, nil)
	if got := postNativeDiagnosis(t, unauth, path, testDecisionPayload(proposal)); got.Code != 401 {
		t.Fatalf("unauthenticated write accepted %d %s", got.Code, got.Body.String())
	}
}
func testSettlementPayload() map[string]any {
	return map[string]any{
		"expectedRevision": int64(3), "closeRemaining": false,
		"toolCallId": "confirmed-user-settlement", "idempotencyKey": "synthetic-settle-1",
	}
}
func TestNativeDiagnosisSettlementCommittedAndNoChange(t *testing.T) {
	owner, run, revisionID, entry, decision := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	base := model.DiagnosisSettlement{
		UserID: owner, ID: uuid.New(), RunID: run, ExpectedRevision: 3, Revision: 4,
		Outcome: "committed", DiagnosisRevisionID: &revisionID, ConditionIDs: []uuid.UUID{entry},
		DecisionIDs: []uuid.UUID{decision}, BodyStateRevision: 8, IdempotencyKey: "synthetic-settle-1",
		ToolCallID: "confirmed-user-settlement", ReceiptID: uuid.New(), SettledAt: time.Now().UTC(),
	}
	fake := &fakeNativeSettlement{receipt: &base}
	path := "/api/v1/diagnosis/analysis-runs/" + run.String() + "/settlement"
	r := nativeDiagnosisCommandRoute(t, owner, nil, fake)
	result := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, testSettlementPayload()), 200)
	if fake.called != 1 || fake.owner != owner || fake.cmd.RunID != run || fake.cmd.ExpectedRevision != 3 || fake.cmd.CloseRemaining ||
		result["outcome"] != "committed" || result["revision"] != float64(4) || result["diagnosisRevisionId"] != revisionID.String() ||
		result["receiptId"] != base.ReceiptID.String() {
		t.Fatalf("bad settlement binding: cmd=%+v response=%#v", fake.cmd, result)
	}
	replay := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, testSettlementPayload()), 200)
	if replay["receiptId"] != result["receiptId"] {
		t.Fatal("replayed settlement receipt drift")
	}
	noChange := base
	noChange.ID = uuid.New()
	noChange.ReceiptID = uuid.New()
	noChange.Outcome = "closed_without_change"
	noChange.Revision = 3
	noChange.DiagnosisRevisionID = nil
	noChange.ConditionIDs = nil
	noChange.DecisionIDs = nil
	fake.receipt = &noChange
	data := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, testSettlementPayload()), 200)
	if data["outcome"] != "closed_without_change" || data["revision"] != float64(3) {
		t.Fatalf("no-change was incorrectly adopted: %#v", data)
	}
	if _, adopt := data["diagnosisRevisionId"]; adopt {
		t.Fatalf("no-change receipt claimed adoption: %#v", data)
	}
	if _, ok := data["decisionIds"].([]any); !ok {
		t.Fatalf("empty receipt arrays must not be null: %#v", data)
	}
}
func TestNativeDiagnosisSettlementSafetyAndNegative(t *testing.T) {
	owner, run := uuid.New(), uuid.New()
	path := "/api/v1/diagnosis/analysis-runs/" + run.String() + "/settlement"
	fake := &fakeNativeSettlement{}
	r := nativeDiagnosisCommandRoute(t, owner, nil, fake)
	blocked := model.DiagnosisSettlement{
		UserID: owner, ID: uuid.New(), RunID: run, ExpectedRevision: 3, Revision: 3,
		Outcome: "safety_blocked", ConditionIDs: []uuid.UUID{}, DecisionIDs: []uuid.UUID{},
		BodyStateRevision: 5, IdempotencyKey: "synthetic-settle-1", ToolCallID: "confirmed-user-settlement",
		ReceiptID: uuid.New(), SettledAt: time.Now().UTC(),
	}
	fake.receipt = &blocked
	fake.err = service.ErrDiagnosisSafetyBlocked
	response := postNativeDiagnosis(t, r, path, testSettlementPayload())
	data := assertNativeReceiptHTTP(t, response, 200)
	if data["outcome"] != "safety_blocked" || data["revision"] != float64(3) {
		t.Fatalf("Safety audit should not claim adoption: %#v", data)
	}
	if _, ok := data["diagnosisRevisionId"]; ok {
		t.Fatalf("safety-blocked claimed adoption: %#v", data)
	}
	fake.err = service.ErrDiagnosisChoicesIncomplete
	if got := postNativeDiagnosis(t, r, path, testSettlementPayload()); got.Code != 409 {
		t.Fatalf("incomplete choice should block commit: %d %s", got.Code, got.Body.String())
	}
	fake.err = nil
	fake.receipt = &blocked
	incorrect := blocked
	incorrect.Outcome = "committed"
	incorrect.Revision = 4
	incorrect.DiagnosisRevisionID = nil
	fake.receipt = &incorrect
	if got := postNativeDiagnosis(t, r, path, testSettlementPayload()); got.Code != 503 {
		t.Fatalf("malformed committed receipt adopted: %d %s", got.Code, got.Body.String())
	}
	fake.receipt = &blocked
	missing := testSettlementPayload()
	delete(missing, "closeRemaining")
	if got := postNativeDiagnosis(t, r, path, missing); got.Code != 400 {
		t.Fatalf("implicit close Remaining: %d %s", got.Code, got.Body.String())
	}
	extra := testSettlementPayload()
	extra["userId"] = uuid.NewString()
	if got := postNativeDiagnosis(t, r, path, extra); got.Code != 400 {
		t.Fatalf("owner spoof accepted: %d %s", got.Code, got.Body.String())
	}
	unauth := nativeDiagnosisCommandRoute(t, uuid.Nil, nil, fake)
	if got := postNativeDiagnosis(t, unauth, path, testSettlementPayload()); got.Code != 401 {
		t.Fatalf("unauthorized settlement accepted: %d %s", got.Code, got.Body.String())
	}
}
