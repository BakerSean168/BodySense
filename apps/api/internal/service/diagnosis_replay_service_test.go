package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
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
	input, err := EncodeDiagnosisReplayInput(revision, bodyState, json.RawMessage(`[{"user_id":"private-user-id","revision":11}]`), json.RawMessage(`{"id":"profile-private","user_id":"private-user-id","email":"private@example.com","birth_date":"1996-08-27","age_years":30}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func TestDiagnosisReplayInputFreezesSafetyEnvelope(t *testing.T) {
	envelope := &SafetyEnvelopeV2{SchemaRevision: SafetyEnvelopeSchemaV2, PolicyRevision: SafetyEnvelopePolicyV1, BodyStateRevision: 12, Assertions: []SafetyAssertionV1{}, ActiveBlockers: []SafetyBlockerV1{}}
	raw, err := EncodeDiagnosisReplayInput(12, json.RawMessage(`{"current_revision":12}`), nil, nil, envelope)
	if err != nil {
		t.Fatal(err)
	}
	var input DiagnosisReplayInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if input.SafetyEnvelope == nil || input.SafetyEnvelope.BodyStateRevision != 12 {
		t.Fatalf("safety envelope was not frozen: %s", raw)
	}
	if _, err := EncodeDiagnosisReplayInput(13, json.RawMessage(`{"current_revision":13}`), nil, nil, envelope); err == nil {
		t.Fatal("mismatched safety revision accepted")
	}
}

func TestV8CounterfactualReplayRequiresFrozenEnvelopeAndBypassesIncompleteCoverage(t *testing.T) {
	diagnosis, repo, userID, _ := persistReplayTestAnalysis(t, diagnosisDecisionAuthorityConfigID, DiagnosisDecisionPolicyV1, "region:neck")
	baseline := map[string]any{}
	if err := json.Unmarshal(repo.byID.RawOutput, &baseline); err != nil {
		t.Fatal(err)
	}
	input, err := decodeDiagnosisReplayInput(json.RawMessage(repo.byID.ReplayInput))
	if err != nil {
		t.Fatal(err)
	}
	replay := NewDiagnosisReplayService(diagnosis, nil)
	if _, err := replay.counterfactualCompare(context.Background(), userID, repo.byID, input, baseline, diagnosisStructuredSafetyConfigID); err == nil || !strings.Contains(err.Error(), "frozen structured safety envelope") {
		t.Fatalf("old replay invented v8 safety input: %v", err)
	}
	envelope := completeSafetyEnvelope()
	envelope.Coverage.Complete = false
	envelope.Coverage.IncompleteSourceRefs = []string{"body-state:fact:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}
	input.SafetyEnvelope = envelope
	report, err := replay.counterfactualCompare(context.Background(), userID, repo.byID, input, baseline, diagnosisStructuredSafetyConfigID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Replay.Status != "insufficient_information" || report.Replay.DecisionOutcome != string(DiagnosisAbstain) {
		t.Fatalf("v8 replay failed to abstain from frozen incomplete coverage: %+v", report.Replay)
	}
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
	if configurationID == defaultDiagnosisConfigurationID || configurationID == diagnosisClaimSurfaceConfigID {
		payload["evidence_acquisition"] = map[string]any{
			"trace_revision":           evidenceAvailabilityTraceV2,
			"policy_revision":          "diagnosis-evidence-gap-v2",
			"external_evidence_status": externalEvidenceNotRequired,
			"attempts":                 []any{},
			"unresolved_critical_gaps": []any{},
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
		_ = json.Unmarshal(replayTestRaw(diagnosisClaimSurfaceConfigID, DiagnosisDecisionPolicyV1, "region:shoulder"), &counterfactual)
		counterfactual["status"] = "partial"
		counterfactual["governance"].(map[string]any)["verdict"] = "degraded"
		delete(counterfactual, "decision_authority")
		encoded, _ := json.Marshal(counterfactual)
		_, _ = w.Write(encoded)
	}))
	defer server.Close()
	t.Setenv("AI_SERVICE_URL", server.URL)

	report, err := NewDiagnosisReplayService(diagnosis, NewAIClient()).CounterfactualReplay(
		context.Background(), userID, analysisID, diagnosisClaimSurfaceConfigID,
	)
	if err != nil {
		t.Fatalf("CounterfactualReplay: %v", err)
	}
	if captured.BodyStateRevision != 12 || captured.ConfigurationID != diagnosisClaimSurfaceConfigID {
		t.Fatalf("counterfactual must use frozen revision and selected config: %#v", captured)
	}
	if !json.Valid(captured.BodyState) || !strings.Contains(string(captured.BodyState), "fact-neck-1") {
		t.Fatalf("counterfactual must use the frozen BodyState: %s", captured.BodyState)
	}
	if report.Comparison.Hard.Match {
		t.Fatal("configuration authority change from pre-envelope to v4 must be visible as a hard comparison change")
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

func TestDiagnosisReplayAuthorityEvidenceRecognizesLegacyProseFalsePositive(t *testing.T) {
	input := DiagnosisReplayInput{
		BodyStateRevision: 3,
		SafetyEnvelope: &SafetyEnvelopeV2{
			SchemaRevision:    SafetyEnvelopeSchemaV2,
			PolicyRevision:    SafetyEnvelopePolicyV1,
			BodyStateRevision: 3,
			Coverage: SafetyCoverageV1{
				Revision:          SafetyCoverageRevisionV1,
				CaptureRevision:   SafetyCaptureRevisionV1,
				RequiredConcepts:  []string{"trauma", "radiating_pain", "numbness", "weakness", "dizziness"},
				CoveredSourceRefs: []string{"body-state:fact:test"},
				Complete:          true,
			},
			Assertions: []SafetyAssertionV1{
				{Concept: SafetyTrauma, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: "body-state:fact:test", SourceKind: SafetyBodyStateFact},
				{Concept: SafetyRadiatingPain, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: "body-state:fact:test", SourceKind: SafetyBodyStateFact},
			},
			ActiveBlockers: []SafetyBlockerV1{},
			RequiresReview: false,
		},
	}
	baseline := map[string]any{
		"status": "safety_blocked",
		"governance": map[string]any{
			"verdict": "rejected",
			"issues": []any{
				map[string]any{"policy": "red_flag_safety", "details": map[string]any{"category": "radiating_pain"}},
				map[string]any{"policy": "red_flag_safety", "details": map[string]any{"category": "trauma"}},
			},
		},
		"decision_authority": map[string]any{
			"outcome":         "block",
			"policy_revision": DiagnosisDecisionPolicyV1,
			"reasons":         []any{"agent_output_failed_safety_governance"},
		},
		"agent_configuration": map[string]any{"decision_policy_revision": DiagnosisDecisionPolicyV1},
	}
	replayed := map[string]any{
		"status":              "completed",
		"governance":          map[string]any{"verdict": "accepted", "issues": []any{}},
		"decision_authority":  map[string]any{"outcome": "allow-normal", "policy_revision": DiagnosisDecisionPolicyV2, "reasons": []any{}},
		"agent_configuration": map[string]any{"decision_policy_revision": DiagnosisDecisionPolicyV2},
		"safety_findings":     []any{},
		"candidates":          []any{map[string]any{"name": "benign"}},
	}

	evidence := diagnosisReplayAuthorityEvidence(input, baseline, replayed)
	if !evidence.SafetyEnvelopePresent || !evidence.CoverageComplete || evidence.ActiveBlockerCount != 0 || evidence.RequiresReview {
		t.Fatalf("structured safety evidence drifted: %#v", evidence)
	}
	if !evidence.Baseline.LegacyProseGovernanceOnly || evidence.Baseline.DecisionPolicyRevision != DiagnosisDecisionPolicyV1 ||
		evidence.Baseline.LegacyProseSafetySource != LegacyProseSafetySourcePostAgent ||
		!slices.Equal(evidence.Baseline.LegacyProseSafetyCategories, []string{"radiating_pain", "trauma"}) ||
		!slices.Equal(evidence.ConfirmedAbsentConcepts, []string{"radiating_pain", "trauma"}) {
		t.Fatalf("legacy false-positive evidence was not recognized: %#v", evidence.Baseline)
	}
	if evidence.Replay.DecisionPolicyRevision != DiagnosisDecisionPolicyV2 || evidence.Replay.GovernanceVerdict != "accepted" || evidence.Replay.SafetyFindingCount != 0 {
		t.Fatalf("structured challenger evidence drifted: %#v", evidence.Replay)
	}
}

func TestDiagnosisReplayAuthorityEvidenceRecognizesLegacyPreAgentSafetyFalsePositive(t *testing.T) {
	input := DiagnosisReplayInput{
		BodyStateRevision: 5,
		SafetyEnvelope: &SafetyEnvelopeV2{
			SchemaRevision:    SafetyEnvelopeSchemaV2,
			PolicyRevision:    SafetyEnvelopePolicyV1,
			BodyStateRevision: 5,
			Coverage: SafetyCoverageV1{
				Revision:          SafetyCoverageRevisionV1,
				CaptureRevision:   SafetyCaptureRevisionV1,
				RequiredConcepts:  []string{"trauma", "radiating_pain", "numbness", "weakness", "dizziness"},
				CoveredSourceRefs: []string{"body-state:fact:test"},
				Complete:          true,
			},
			Assertions: []SafetyAssertionV1{
				{Concept: SafetyTrauma, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: "body-state:fact:test", SourceKind: SafetyBodyStateFact},
				{Concept: SafetyRadiatingPain, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: "body-state:fact:test", SourceKind: SafetyBodyStateFact},
			},
			ActiveBlockers: []SafetyBlockerV1{},
			RequiresReview: false,
		},
	}
	baseline := map[string]any{
		"status":     "safety_blocked",
		"governance": map[string]any{"verdict": "accepted", "issues": []any{}},
		"safety_summary": map[string]any{
			"red_flags": map[string]any{
				"has_red_flags": true,
				"flags": []any{
					map[string]any{"source": "conversation", "category": "radiating_pain", "matched_text": "放射痛"},
					map[string]any{"source": "conversation", "category": "trauma", "matched_text": "外伤"},
				},
			},
		},
		"execution_provenance": map[string]any{"status": "bypassed", "reason": "python_pre_agent_safety_gate"},
		"decision_authority": map[string]any{
			"outcome":         "block",
			"policy_revision": DiagnosisDecisionPolicyV1,
			"reasons":         []any{"agent_output_failed_safety_governance"},
		},
		"agent_configuration": map[string]any{"decision_policy_revision": DiagnosisDecisionPolicyV1},
	}
	replayed := map[string]any{
		"status":              "completed",
		"governance":          map[string]any{"verdict": "accepted", "issues": []any{}},
		"decision_authority":  map[string]any{"outcome": "allow-normal", "policy_revision": DiagnosisDecisionPolicyV2, "reasons": []any{}},
		"agent_configuration": map[string]any{"decision_policy_revision": DiagnosisDecisionPolicyV2},
		"safety_findings":     []any{},
		"candidates":          []any{map[string]any{"name": "benign"}},
	}

	evidence := diagnosisReplayAuthorityEvidence(input, baseline, replayed)
	if !evidence.Baseline.LegacyProseGovernanceOnly ||
		evidence.Baseline.LegacyProseSafetySource != LegacyProseSafetySourcePreAgent ||
		!slices.Equal(evidence.Baseline.LegacyProseSafetyCategories, []string{"radiating_pain", "trauma"}) ||
		!slices.Equal(evidence.ConfirmedAbsentConcepts, []string{"radiating_pain", "trauma"}) {
		t.Fatalf("legacy pre-agent false-positive evidence was not recognized: %#v", evidence)
	}
}

func TestLegacyPreAgentSafetyEvidenceFailsClosedWithoutExactProof(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"status":     "safety_blocked",
			"governance": map[string]any{"verdict": "accepted", "issues": []any{}},
			"safety_summary": map[string]any{
				"red_flags": map[string]any{
					"has_red_flags": true,
					"flags":         []any{map[string]any{"category": "trauma"}},
				},
			},
			"execution_provenance": map[string]any{"status": "bypassed", "reason": "python_pre_agent_safety_gate"},
			"decision_authority": map[string]any{
				"outcome":         "block",
				"policy_revision": DiagnosisDecisionPolicyV1,
				"reasons":         []any{"agent_output_failed_safety_governance"},
			},
			"agent_configuration": map[string]any{"decision_policy_revision": DiagnosisDecisionPolicyV1},
		}
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"wrong-provenance-reason", func(p map[string]any) { p["execution_provenance"].(map[string]any)["reason"] = "other" }},
		{"agent-not-bypassed", func(p map[string]any) { p["execution_provenance"].(map[string]any)["status"] = "executed" }},
		{"red-flags-not-asserted", func(p map[string]any) {
			p["safety_summary"].(map[string]any)["red_flags"].(map[string]any)["has_red_flags"] = false
		}},
		{"missing-category", func(p map[string]any) {
			p["safety_summary"].(map[string]any)["red_flags"].(map[string]any)["flags"] = []any{map[string]any{"matched_text": "外伤"}}
		}},
		{"governance-not-accepted", func(p map[string]any) { p["governance"].(map[string]any)["verdict"] = "rejected" }},
		{"mixed-governance-issue", func(p map[string]any) {
			p["governance"].(map[string]any)["issues"] = []any{map[string]any{"policy": "other"}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := base()
			tc.mutate(payload)
			if replayLegacyProseGovernanceOnly(payload) {
				t.Fatalf("unproven pre-agent block must fail closed: %#v", payload)
			}
		})
	}
}

func TestDiagnosisReplayAuthorityEvidenceRequiresConfirmedAbsentCoverage(t *testing.T) {
	envelope := &SafetyEnvelopeV2{
		SchemaRevision: SafetyEnvelopeSchemaV2, PolicyRevision: SafetyEnvelopePolicyV1, BodyStateRevision: 3,
		Coverage: SafetyCoverageV1{
			Revision: SafetyCoverageRevisionV1, CaptureRevision: SafetyCaptureRevisionV1, Complete: true,
			RequiredConcepts:  []string{"trauma", "radiating_pain"},
			CoveredSourceRefs: []string{"body-state:fact:a", "body-state:fact:b"},
		},
		Assertions: []SafetyAssertionV1{
			{Concept: SafetyTrauma, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: "body-state:fact:a", SourceKind: SafetyBodyStateFact},
			{Concept: SafetyTrauma, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: "body-state:fact:b", SourceKind: SafetyBodyStateFact},
			{Concept: SafetyRadiatingPain, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyConfirmed, SourceRef: "body-state:fact:a", SourceKind: SafetyBodyStateFact},
			{Concept: SafetyRadiatingPain, Polarity: SafetyAbsent, Temporality: SafetyCurrent, ReviewState: SafetyUnverified, SourceRef: "body-state:fact:b", SourceKind: SafetyBodyStateFact},
		},
	}
	got := replayConfirmedAbsentConcepts(envelope)
	if !slices.Equal(got, []string{"trauma"}) {
		t.Fatalf("only concepts confirmed absent across every covered source are authoritative: %#v", got)
	}
}

func TestLegacyProseAuthorityEvidenceRejectsMixedGovernanceIssues(t *testing.T) {
	payload := map[string]any{
		"status": "safety_blocked",
		"governance": map[string]any{
			"verdict": "rejected",
			"issues": []any{
				map[string]any{"policy": "red_flag_safety", "details": map[string]any{"category": "trauma"}},
				map[string]any{"policy": "forbidden_claim_surface"},
			},
		},
		"decision_authority": map[string]any{
			"outcome":         "block",
			"policy_revision": DiagnosisDecisionPolicyV1,
			"reasons":         []any{"agent_output_failed_safety_governance"},
		},
		"agent_configuration": map[string]any{"decision_policy_revision": DiagnosisDecisionPolicyV1},
	}
	if replayLegacyProseGovernanceOnly(payload) {
		t.Fatal("mixed governance failures must never be authorized as a prose false-positive migration")
	}
}
