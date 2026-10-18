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

type fakeNativeDirectEdit struct {
	receipt *model.ConditionDirectEdit
	err     error
	owner   uuid.UUID
	cmd     service.DiagnosisDirectEditCommand
	called  int
}

func (f *fakeNativeDirectEdit) Apply(_ context.Context, owner uuid.UUID, cmd service.DiagnosisDirectEditCommand) (*model.ConditionDirectEdit, error) {
	f.owner = owner
	f.cmd = cmd
	f.called++
	return f.receipt, f.err
}
func nativeDirectEditRoute(t *testing.T, owner uuid.UUID, edits nativeDiagnosisDirectEditApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	RegisterRoutes(r, StrictHandler(NewPublicServer().WithNativeDiagnosisDirectEdits(edits)), RouteSecurity{
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
func testDirectEditPayload(evidence uuid.UUID, kind string) map[string]any {
	return map[string]any{
		"editKind": kind, "requestEvidenceId": evidence, "expectedRevision": int64(3),
		"reason": "User confirmed this condition is wrong", "toolCallId": "confirmed-user-direct-edit", "idempotencyKey": "synthetic-edit-1",
	}
}
func testDirectEditReceipt(owner, condition, evidence uuid.UUID, kind model.ConditionDirectEditKind, change model.ConditionDirectEditChange) *model.ConditionDirectEdit {
	return &model.ConditionDirectEdit{
		UserID: owner, ID: uuid.New(), ConditionID: condition, EditKind: kind, Changes: []model.ConditionDirectEditChange{change},
		Reason: "User confirmed this condition is wrong", RequestMessageID: uuid.New(), RequestEvidenceID: evidence, RequestSourceVersion: 1,
		ConfirmationToolCallID: "confirmed-user-direct-edit", BasedOnDiagnosisRevision: 3, ResultingDiagnosisRevision: 4,
		DiagnosisRevisionID: uuid.New(), BodyStateRevision: 7, IdempotencyKey: "synthetic-edit-1", ReceiptID: uuid.New(), ConfirmedAt: time.Now().UTC(),
	}
}

func TestNativeDiagnosisDirectEditTransport(t *testing.T) {
	owner, condition, evidence := uuid.New(), uuid.New(), uuid.New()
	path := "/api/v1/diagnosis/conditions/" + condition.String() + "/direct-edits"
	fake := &fakeNativeDirectEdit{receipt: testDirectEditReceipt(owner, condition, evidence, model.ConditionWithdraw,
		model.ConditionDirectEditChange{Field: "disposition", Before: "adopted", After: "inactive"})}
	r := nativeDirectEditRoute(t, owner, fake)
	result := assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, testDirectEditPayload(evidence, "withdraw")), http.StatusOK)
	if fake.called != 1 || fake.owner != owner || fake.cmd.ConditionID != condition || fake.cmd.EditKind != model.ConditionWithdraw ||
		fake.cmd.RequestEvidenceID != evidence || fake.cmd.ExpectedRevision != 3 || fake.cmd.Notes != nil ||
		result["editId"] != fake.receipt.ID.String() || result["revision"] != float64(4) || result["expectedRevision"] != float64(3) ||
		result["diagnosisRevisionId"] != fake.receipt.DiagnosisRevisionID.String() || result["receiptId"] != fake.receipt.ReceiptID.String() {
		t.Fatalf("bad withdraw binding: cmd=%+v response=%#v", fake.cmd, result)
	}
	if changes, ok := result["changes"].([]any); !ok || len(changes) != 1 {
		t.Fatalf("exact change missing: %#v", result)
	}

	notes := "Named by my physiotherapist"
	fake.receipt = testDirectEditReceipt(owner, condition, evidence, model.ConditionCorrectMetadata,
		model.ConditionDirectEditChange{Field: "qualifiers.notes", Before: "", After: notes})
	correct := testDirectEditPayload(evidence, "correct_metadata")
	correct["metadata"] = map[string]any{"notes": notes}
	assertNativeReceiptHTTP(t, postNativeDiagnosis(t, r, path, correct), http.StatusOK)
	if fake.cmd.EditKind != model.ConditionCorrectMetadata || fake.cmd.Notes == nil || *fake.cmd.Notes != notes {
		t.Fatalf("notes not bound: %+v", fake.cmd)
	}

	for name, mutate := range map[string]func(map[string]any){
		"lifecycle kind":     func(p map[string]any) { p["editKind"] = "mark_resolved" },
		"owner spoof":        func(p map[string]any) { p["userId"] = uuid.NewString() },
		"substantive field":  func(p map[string]any) { p["metadata"] = map[string]any{"notes": "x", "medicalName": "Other"} },
		"blank reason":       func(p map[string]any) { p["reason"] = " " },
		"missing request":    func(p map[string]any) { delete(p, "requestEvidenceId") },
		"missing revision":   func(p map[string]any) { delete(p, "expectedRevision") },
		"padded idempotency": func(p map[string]any) { p["idempotencyKey"] = " key" },
	} {
		payload := testDirectEditPayload(evidence, "withdraw")
		mutate(payload)
		if got := postNativeDiagnosis(t, r, path, payload); got.Code != http.StatusBadRequest {
			t.Fatalf("%s accepted: %d %s", name, got.Code, got.Body.String())
		}
	}
	if fake.called != 2 {
		t.Fatalf("invalid requests reached the writer: %d", fake.called)
	}
}

func TestNativeDiagnosisDirectEditErrorsAndReceiptGuards(t *testing.T) {
	owner, condition, evidence := uuid.New(), uuid.New(), uuid.New()
	path := "/api/v1/diagnosis/conditions/" + condition.String() + "/direct-edits"
	fake := &fakeNativeDirectEdit{}
	r := nativeDirectEditRoute(t, owner, fake)
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{model.ErrDiagnosisNotFound, 404, "NOT_FOUND"}, {service.ErrDiagnosisDecisionConflict, 409, "REVISION_CONFLICT"},
		{service.ErrDiagnosisEvidenceUnavailable, 409, "REVISION_CONFLICT"}, {service.ErrDiagnosisSafetyBlocked, 409, "SAFETY_BLOCKED"},
		{service.ErrDiagnosisSafetyUnavailable, 503, "SAFETY_UNAVAILABLE"}, {service.ErrDiagnosisDirectEditInvalid, 400, "INVALID_REQUEST"},
		{errors.New("storage down"), 503, "DIAGNOSIS_UNAVAILABLE"},
	} {
		fake.err = test.err
		got := postNativeDiagnosis(t, r, path, testDirectEditPayload(evidence, "withdraw"))
		data := assertNativeReceiptHTTP(t, got, test.status)
		if errBody, _ := data["error"].(map[string]any); errBody == nil || errBody["code"] != test.code {
			t.Fatalf("%v: body=%s", test.err, got.Body.String())
		}
	}
	fake.err = nil
	foreign := testDirectEditReceipt(uuid.New(), condition, evidence, model.ConditionWithdraw, model.ConditionDirectEditChange{Field: "disposition", Before: "adopted", After: "inactive"})
	fake.receipt = foreign
	if got := postNativeDiagnosis(t, r, path, testDirectEditPayload(evidence, "withdraw")); got.Code != 503 {
		t.Fatalf("foreign receipt accepted: %d %s", got.Code, got.Body.String())
	}
	skipped := testDirectEditReceipt(owner, condition, evidence, model.ConditionWithdraw, model.ConditionDirectEditChange{Field: "disposition", Before: "adopted", After: "inactive"})
	skipped.ResultingDiagnosisRevision = 5
	fake.receipt = skipped
	if got := postNativeDiagnosis(t, r, path, testDirectEditPayload(evidence, "withdraw")); got.Code != 503 {
		t.Fatalf("non-sequential revision accepted: %d %s", got.Code, got.Body.String())
	}
	unwired := nativeDirectEditRoute(t, owner, nil)
	if got := postNativeDiagnosis(t, unwired, path, testDirectEditPayload(evidence, "withdraw")); got.Code != 503 {
		t.Fatalf("unwired edit accepted: %d %s", got.Code, got.Body.String())
	}
	unauth := nativeDirectEditRoute(t, uuid.Nil, fake)
	if got := postNativeDiagnosis(t, unauth, path, testDirectEditPayload(evidence, "withdraw")); got.Code != 401 {
		t.Fatalf("unauthenticated edit accepted: %d %s", got.Code, got.Body.String())
	}
}
