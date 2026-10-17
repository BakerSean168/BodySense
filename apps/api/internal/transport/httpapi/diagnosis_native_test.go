package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type nativeDiagnosisReadFake struct {
	view  *service.NativeDiagnosisView
	err   error
	owner uuid.UUID
	calls int
}

func (f *nativeDiagnosisReadFake) Read(_ context.Context, owner uuid.UUID) (*service.NativeDiagnosisView, error) {
	f.owner = owner
	f.calls++
	return f.view, f.err
}
func nativeDiagnosisRoute(t *testing.T, owner uuid.UUID, reader nativeDiagnosisReadApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	server := NewPublicServer().WithNativeDiagnosis(reader)
	RegisterRoutes(router, StrictHandler(server), RouteSecurity{
		Auth: func(c *gin.Context) {
			if owner != uuid.Nil {
				c.Set("user_id", owner.String())
			}
			c.Next()
		},
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}
func getNativeDiagnosisResponse(t *testing.T, router *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/diagnosis", nil))
	return recorder
}
func TestNativeDiagnosisReadEmptyIsOwnerScopedAndReadOnly(t *testing.T) {
	owner := uuid.New()
	fake := &nativeDiagnosisReadFake{view: &service.NativeDiagnosisView{UserID: owner, ConditionEntries: []service.NativeDiagnosisCondition{}}}
	r := getNativeDiagnosisResponse(t, nativeDiagnosisRoute(t, owner, fake))
	if r.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
	if fake.calls != 1 || fake.owner != owner {
		t.Fatalf("reader owner=%s calls=%d", fake.owner, fake.calls)
	}
	var body map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["userId"] != owner.String() || body["currentRevision"] != float64(0) || body["conditionEntries"] == nil {
		t.Fatalf("empty schema: %#v", body)
	}
	if _, ok := body["updatedAt"]; ok {
		t.Fatalf("phantom empty aggregate updatedAt: %#v", body)
	}
	if _, ok := body["user_id"]; ok {
		t.Fatalf("database owner field leaked: %#v", body)
	}
}
func TestNativeDiagnosisAuthenticationAndFailClosed(t *testing.T) {
	owner := uuid.New()
	fake := &nativeDiagnosisReadFake{view: &service.NativeDiagnosisView{UserID: owner, ConditionEntries: []service.NativeDiagnosisCondition{}}}
	unauth := getNativeDiagnosisResponse(t, nativeDiagnosisRoute(t, uuid.Nil, fake))
	if unauth.Code != http.StatusUnauthorized || fake.calls != 0 {
		t.Fatalf("unauthorized: %d calls=%d", unauth.Code, fake.calls)
	}
	unavailable := getNativeDiagnosisResponse(t, nativeDiagnosisRoute(t, owner, nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing service: %d %s", unavailable.Code, unavailable.Body.String())
	}
	fake.err = errors.New("db unavailable")
	failed := getNativeDiagnosisResponse(t, nativeDiagnosisRoute(t, owner, fake))
	if failed.Code != http.StatusServiceUnavailable {
		t.Fatalf("backend error: %d %s", failed.Code, failed.Body.String())
	}
	fake.err = nil
	fake.view.UserID = uuid.New()
	foreign := getNativeDiagnosisResponse(t, nativeDiagnosisRoute(t, owner, fake))
	if foreign.Code != http.StatusServiceUnavailable {
		t.Fatalf("foreign reader result: %d %s", foreign.Code, foreign.Body.String())
	}
}
func TestNativeDiagnosisReadCommittedWithSources(t *testing.T) {
	owner, conditionID, decisionID, runID, factID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	source := model.DiagnosisSourceClaimRef{
		ClaimID: "explicit-user-statement", SourceKind: model.DiagnosisUserSuspicion,
		SourceID: uuid.NewString(), SourceVersion: "1",
		Statement:  "Synthetic user statement, not clinical confirmation",
		RecordedAt: now, ValidityAtAssessment: model.DiagnosisSourceCurrent,
	}
	condition := service.NativeDiagnosisCondition{
		ConditionID: conditionID, MedicalName: "Synthetic named syndrome",
		Assessment: &model.ConditionAssessment{
			Level: model.DiagnosisSuspected, BasedOnRunID: runID, Rationale: "Synthetic evidence",
			InformationGaps: []string{}, DifferentialNotes: []string{},
		},
		SourceClaimRefs: []model.DiagnosisSourceClaimRef{source},
		EvidenceLinks: []service.NativeDiagnosisEvidenceLink{{
			EvidenceType: model.DiagnosisBodyFact, EvidenceID: factID, EvidenceVersion: "1",
			Role: model.DiagnosisSupporting, Rationale: "Not proof of causality", AnalyzedByRunID: runID,
		}},
		CauseAnalysis:   model.ConditionCauseAnalysis{Summary: "Unknown", Rationale: "No causal proof"},
		InformationGaps: []string{},
		HandlingPlan:    model.ConditionHandlingPlan{Summary: "Discuss with clinician", Actions: []string{}, CautionNotes: []string{}},
		Disposition:     model.ConditionAdopted, LatestDecisionID: decisionID, LastAnalysisRunID: &runID,
		CommittedRevision: 1, CreatedAt: now, UpdatedAt: now,
	}
	fake := &nativeDiagnosisReadFake{view: &service.NativeDiagnosisView{
		UserID: owner, CurrentRevision: 1, UpdatedAt: &now, ConditionEntries: []service.NativeDiagnosisCondition{condition},
	}}
	response := getNativeDiagnosisResponse(t, nativeDiagnosisRoute(t, owner, fake))
	if response.Code != http.StatusOK {
		t.Fatalf("committed projection failed schema: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		CurrentRevision  int64 `json:"currentRevision"`
		ConditionEntries []struct {
			ConditionID uuid.UUID `json:"conditionId"`
			MedicalName string    `json:"medicalName"`
			Assessment  struct {
				Level string `json:"level"`
			} `json:"assessment"`
			EvidenceLinks []struct {
				EvidenceVersion string `json:"evidenceVersion"`
			} `json:"evidenceLinks"`
		} `json:"conditionEntries"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CurrentRevision != 1 || len(body.ConditionEntries) != 1 ||
		body.ConditionEntries[0].ConditionID != conditionID ||
		body.ConditionEntries[0].Assessment.Level != "suspected" ||
		len(body.ConditionEntries[0].EvidenceLinks) != 1 ||
		body.ConditionEntries[0].EvidenceLinks[0].EvidenceVersion != "1" {
		t.Fatalf("read committed condition/evidence %s", response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte(`"user_id"`)) ||
		bytes.Contains(response.Body.Bytes(), []byte(`"current_revision"`)) {
		t.Fatalf("persistence-only snake fields leaked: %s", response.Body.String())
	}
}

func TestNativeDiagnosisInvalidCommitProjectionRejected(t *testing.T) {
	owner := uuid.New()
	view := &service.NativeDiagnosisView{UserID: owner, CurrentRevision: 1, ConditionEntries: []service.NativeDiagnosisCondition{
		{ConditionID: uuid.New(), MedicalName: "", CommittedRevision: 2, LatestDecisionID: uuid.New()},
	}}
	response := getNativeDiagnosisResponse(t, nativeDiagnosisRoute(t, owner, &nativeDiagnosisReadFake{view: view}))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("uncommitted/invalid projection leaked: %d %s", response.Code, response.Body.String())
	}
}
