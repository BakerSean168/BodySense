package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func replayTestInput(t *testing.T, revision int64) json.RawMessage {
	t.Helper()
	bodyState := json.RawMessage(`{"user_id":"private-user-id","current_revision":12,"safety_state":{},"facts":[{"id":"fact-neck-1","kind":"discomfort","concern_key":"region:neck","value":"mild neck load"}],"observations":[]}`)
	if revision != 12 {
		var payload map[string]any
		if err := json.Unmarshal(bodyState, &payload); err != nil {
			t.Fatal(err)
		}
		payload["current_revision"] = revision
		bodyState, _ = json.Marshal(payload)
	}
	input, err := EncodeDiagnosisReplayInput(revision, bodyState, json.RawMessage(`[{"user_id":"private-user-id","revision":11}]`), json.RawMessage(`{"id":"profile-private","user_id":"private-user-id","email":"private@example.com","birth_date":"1996-08-27","age_years":30}`))
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func replayTestRaw(configurationID, decisionRevision, concernKey string) json.RawMessage {
	payload := map[string]any{
		"status":  "completed",
		"scope":   "full_body",
		"summary": "stable baseline summary",
		"candidates": []any{map[string]any{
			"concern_key":    concernKey,
			"name":           "neck load pattern",
			"confidence":     "high",
			"basis":          "frozen fact",
			"basis_fact_ids": []string{"fact-neck-1"},
		}},
		"cross_concern_patterns": []any{},
		"information_gaps":       []any{},
		"safety_summary":         map[string]any{},
		"governance": map[string]any{
			"kind": "diagnosis", "verdict": "accepted", "reasons": []any{}, "issues": []any{},
		},
		"agent_configuration": map[string]any{
			"id": configurationID, "role": "diagnosis", "decision_policy_revision": decisionRevision,
		},
		"execution_provenance": map[string]any{"status": "executed", "runtime": "pydantic-ai"},
	}
	if decisionRevision == DiagnosisDecisionPolicyV1 {
		payload["decision_authority"] = map[string]any{
			"policy_revision": decisionRevision,
			"outcome":         string(DiagnosisAllowNormal),
			"reasons":         []any{},
		}
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func persistReplayTestAnalysis(t *testing.T, configurationID, decisionRevision, concernKey string) (*DiagnosisAnalysisService, *fakeDiagnosisAnalysisRepository, uuid.UUID, uuid.UUID) {
	t.Helper()
	repo := &fakeDiagnosisAnalysisRepository{}
	diagnosis := NewDiagnosisAnalysisService(repo)
	userID := uuid.New()
	analysis, err := diagnosis.PersistAIResultWithReplayInput(
		context.Background(), userID, 12,
		replayTestRaw(configurationID, decisionRevision, concernKey),
		replayTestInput(t, 12),
	)
	if err != nil {
		t.Fatalf("persist replay fixture: %v", err)
	}
	repo.byID = analysis
	return diagnosis, repo, userID, analysis.ID
}

func TestHistoricalDiagnosisReplayRecomputesFrozenDecisionWithoutModelCall(t *testing.T) {
	diagnosis, _, userID, analysisID := persistReplayTestAnalysis(
		t, diagnosisDecisionAuthorityConfigID, DiagnosisDecisionPolicyV1, "region:neck",
	)
	replay := NewDiagnosisReplayService(diagnosis, nil)

	report, err := replay.HistoricalReplay(context.Background(), userID, analysisID)
	if err != nil {
		t.Fatalf("HistoricalReplay: %v", err)
	}
	if report.Mode != "historical" || report.TargetConfigurationID != diagnosisDecisionAuthorityConfigID {
		t.Fatalf("unexpected historical report identity: %#v", report)
	}
	if !report.ArtifactIntegrity.Match || !report.Comparison.Hard.Match || !report.Comparison.Semantic.Match || !report.Comparison.Presentation.Match {
		t.Fatalf("frozen historical replay must reproduce stored invariants: %#v", report)
	}
	if report.Replay.DecisionOutcome != string(DiagnosisAllowNormal) {
		t.Fatalf("unexpected replayed authority outcome: %s", report.Replay.DecisionOutcome)
	}
	if report.InputFingerprint == "" {
		t.Fatal("replay input must have a stable fingerprint")
	}
}

func TestHistoricalDiagnosisReplayFailsClosedWhenFrozenInputPredatesPhase8(t *testing.T) {
	repo := &fakeDiagnosisAnalysisRepository{}
	diagnosis := NewDiagnosisAnalysisService(repo)
	userID := uuid.New()
	analysis, err := diagnosis.PersistAIResult(
		context.Background(), userID, 12,
		replayTestRaw(diagnosisV1ConfigurationID, DiagnosisDecisionPolicyPreEnvelope, "region:neck"),
	)
	if err != nil {
		t.Fatal(err)
	}
	repo.byID = analysis

	_, err = NewDiagnosisReplayService(diagnosis, nil).HistoricalReplay(context.Background(), userID, analysis.ID)
	if err == nil || !errors.Is(err, ErrDiagnosisReplayUnavailable) {
		t.Fatalf("expected explicit replay-unavailable error, got %v", err)
	}
}

func TestCounterfactualDiagnosisReplayUsesFrozenInputAndSelectedConfigurationWithoutPersistence(t *testing.T) {
	diagnosis, repo, userID, analysisID := persistReplayTestAnalysis(
		t, diagnosisV1ConfigurationID, DiagnosisDecisionPolicyPreEnvelope, "region:neck",
	)
	originalID := repo.createdAnalysis.ID
	var captured DiagnosisRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/diagnosis/analyze" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode replay request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		var counterfactual map[string]any
		_ = json.Unmarshal(replayTestRaw(diagnosisDecisionAuthorityConfigID, DiagnosisDecisionPolicyV1, "region:shoulder"), &counterfactual)
		counterfactual["status"] = "partial"
		counterfactual["governance"].(map[string]any)["verdict"] = "degraded"
		delete(counterfactual, "decision_authority")
		encoded, _ := json.Marshal(counterfactual)
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	t.Setenv("AI_SERVICE_URL", server.URL)

	report, err := NewDiagnosisReplayService(diagnosis, NewAIClient()).CounterfactualReplay(
		context.Background(), userID, analysisID, diagnosisDecisionAuthorityConfigID,
	)
	if err != nil {
		t.Fatalf("CounterfactualReplay: %v", err)
	}
	if captured.BodyStateRevision != 12 || captured.ConfigurationID != diagnosisDecisionAuthorityConfigID {
		t.Fatalf("counterfactual must use frozen revision and selected config: %#v", captured)
	}
	if !json.Valid(captured.BodyState) || !strings.Contains(string(captured.BodyState), "fact-neck-1") {
		t.Fatalf("counterfactual must use the frozen BodyState: %s", captured.BodyState)
	}
	if report.Comparison.Hard.Match {
		t.Fatal("configuration authority change from pre-envelope to v1 must be visible as a hard comparison change")
	}
	if report.Comparison.Semantic.Match {
		t.Fatal("changed concern key must be visible as semantic drift")
	}
	if repo.createdAnalysis.ID != originalID {
		t.Fatal("counterfactual replay must not persist a new DiagnosisAnalysis")
	}
}

func TestDiagnosisReplayExportsQualificationShapedRegressionCaseWithoutRealUserID(t *testing.T) {
	diagnosis, _, userID, analysisID := persistReplayTestAnalysis(
		t, diagnosisDecisionAuthorityConfigID, DiagnosisDecisionPolicyV1, "region:neck",
	)
	exported, err := NewDiagnosisReplayService(diagnosis, nil).ExportRegressionCase(
		context.Background(), userID, analysisID,
	)
	if err != nil {
		t.Fatalf("ExportRegressionCase: %v", err)
	}
	if exported["schema_target"] != DiagnosisRegressionExportSchema {
		t.Fatalf("unexpected schema target: %#v", exported["schema_target"])
	}
	caseDoc := exported["case"].(map[string]any)
	inputs := caseDoc["inputs"].(map[string]any)
	if inputs["user_id"] != "historical-regression" {
		t.Fatalf("regression export must not embed durable user id: %#v", inputs["user_id"])
	}
	encoded, _ := json.Marshal(exported)
	if strings.Contains(string(encoded), "private-user-id") || strings.Contains(string(encoded), "private@example.com") || strings.Contains(string(encoded), "profile-private") {
		t.Fatalf("regression export must scrub direct identifiers: %s", encoded)
	}
	if !strings.Contains(string(encoded), "fact-neck-1") {
		t.Fatalf("regression export must retain domain evidence identities: %s", encoded)
	}
	metadata := caseDoc["metadata"].(map[string]any)
	if metadata["split"] != "regression" || metadata["expected_status"] != "completed" {
		t.Fatalf("unexpected regression metadata: %#v", metadata)
	}
}

func TestCounterfactualFrozenPreservesLegacyRejectedBaselineForUnsafeRelaxationDetection(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request DiagnosisRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(replayTestRaw(request.ConfigurationID, DiagnosisDecisionPolicyV1, "region:neck"))
	}))
	defer server.Close()
	t.Setenv("AI_SERVICE_URL", server.URL)

	baseline := json.RawMessage(`{
		"governance":{"kind":"diagnosis","verdict":"rejected","reasons":["legacy hard reject"],"issues":[]},
		"safety_fallback":"blocked",
		"agent_configuration":{"id":"diag-config-f492eb1c0c6676ae","role":"diagnosis"}
	}`)
	report, err := NewDiagnosisReplayService(nil, NewAIClient()).CounterfactualFrozen(
		context.Background(), uuid.New(), replayTestInput(t, 12), baseline,
		diagnosisV1ConfigurationID, diagnosisDecisionAuthorityConfigID,
	)
	if err != nil {
		t.Fatalf("CounterfactualFrozen: %v", err)
	}
	if calls != 1 || report.Baseline.DecisionOutcome != string(DiagnosisBlock) || report.Replay.DecisionOutcome != string(DiagnosisAllowNormal) {
		t.Fatalf("unexpected transient comparison: calls=%d report=%#v", calls, report)
	}
	repo := &fakeDiagnosisRolloutRepository{}
	rollout := NewDiagnosisRolloutService(repo)
	route := rolloutTestRoute(diagnosisV1ConfigurationID, diagnosisDecisionAuthorityConfigID)
	if err := rollout.RecordComparison(context.Background(), route, uuid.Nil, report, nil); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 1 || !repo.items[0].UnsafeRelaxation || repo.items[0].SourceAnalysisID != nil {
		t.Fatalf("transient rejected baseline must remain observable without fake analysis identity: %#v", repo.items)
	}
}
