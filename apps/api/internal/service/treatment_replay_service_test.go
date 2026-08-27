package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeTreatmentReplayAI struct {
	calls    int
	captured TreatmentRecommendationRequest
	result   json.RawMessage
	err      error
}

func (f *fakeTreatmentReplayAI) RecommendTreatment(_ context.Context, req TreatmentRecommendationRequest) (json.RawMessage, error) {
	f.calls++
	f.captured = req
	return f.result, f.err
}

func treatmentReplayFixture(t *testing.T, configurationID string) (*model.TreatmentRevision, TreatmentReplayInput) {
	t.Helper()
	analysisID := uuid.New()
	facts := TreatmentDecisionFacts{
		DiagnosisStatus:          "completed",
		CandidateCount:           1,
		FreshnessState:           model.DiagnosisFreshnessFresh,
		SafetyState:              json.RawMessage(`{}`),
		CandidateAssessmentReady: true,
		CurrentBodyStateRevision: 12,
		MaterialReviewStatus:     model.TreatmentStatusActive,
	}
	bodyState := json.RawMessage(`{"current_revision":12,"user_id":"private-user","facts":[{"id":"fact-neck","kind":"discomfort","value":"neck load"}]}`)
	diagnosis := json.RawMessage(`{"analysis_id":"` + analysisID.String() + `","status":"completed","candidates":[{"candidate_id":"candidate-neck","concern_key":"region:neck"}]}`)
	assessments := json.RawMessage(`[{"candidate_id":"candidate-neck","state":"confirmed"}]`)
	profile := json.RawMessage(`{"id":"profile-private","user_id":"private-user","gender":"male","birth_date":"1998-05-20"}`)
	constraints := json.RawMessage(`{"equipment":"none"}`)
	evidence := json.RawMessage(`[{"evidence_id":"evidence-neck","summary":"graded activity"}]`)
	rawInput, err := EncodeTreatmentReplayInput(
		12, bodyState, diagnosis, assessments, profile, constraints, evidence, facts,
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := decodeTreatmentReplayInput(rawInput)
	if err != nil {
		t.Fatal(err)
	}
	decision := EvaluateTreatmentDecision(TreatmentDecisionPolicyV1, TreatmentDecisionGeneration, facts)
	revision := &model.TreatmentRevision{
		ID:                        uuid.New(),
		TreatmentID:               uuid.New(),
		Revision:                  1,
		AcceptanceState:           model.TreatmentAcceptanceProposed,
		LifecycleState:            model.TreatmentStatusReviewRecommended,
		SourceBodyStateRevision:   12,
		SourceDiagnosisAnalysisID: analysisID,
		Goal:                      "reduce neck load",
		DurationWeeks:             4,
		Plan: datatypes.JSON(`{
			"summary":"baseline summary","goal":"reduce neck load","duration_weeks":4,
			"interventions":[{"kind":"exercise","title":"chin tuck","description":"controlled","prescription":{"sets":2}}],
			"daily_habits":[],"expected_timeline":"4 weeks","warning_signs":["worsening"],"review_triggers":["new weakness"],"safety_notes":[]
		}`),
		EvidenceIDs:          datatypes.JSON(`[]`),
		Governance:           datatypes.JSON(`{"kind":"treatment","verdict":"accepted","reasons":[],"issues":[]}`),
		AgentConfigurationID: configurationID,
		AgentConfiguration: datatypes.JSON(`{
			"id":"` + configurationID + `","role":"treatment","decision_policy_revision":"treatment-go-acceptance-v1"
		}`),
		ExecutionProvenance: datatypes.JSON(`{
			"status":"executed","runtime":"pydantic-ai","logical_model":"bodysense-structured"
		}`),
		GenerationDecisionTrace: buildTreatmentDecisionTrace(decision, facts, analysisID, configurationID),
		ReplayInput:             datatypes.JSON(rawInput),
	}
	return revision, input
}

func treatmentReplayCounterfactualPayload(t *testing.T, configurationID string) json.RawMessage {
	t.Helper()
	payload := map[string]any{
		"status":               "proposed",
		"summary":              "challenger summary",
		"goal":                 "reduce neck load",
		"duration_weeks":       4,
		"interventions":        []any{map[string]any{"kind": "mobility", "title": "neck mobility", "description": "controlled", "prescription": map[string]any{"sets": 2}}},
		"daily_habits":         []any{},
		"expected_timeline":    "4 weeks",
		"warning_signs":        []any{"worsening"},
		"review_triggers":      []any{"new weakness"},
		"safety_notes":         []any{},
		"evidence_ids":         []any{},
		"governance":           map[string]any{"kind": "treatment", "verdict": "accepted", "reasons": []any{}, "issues": []any{}},
		"agent_configuration":  map[string]any{"id": configurationID, "role": "treatment", "decision_policy_revision": TreatmentDecisionPolicyV1},
		"execution_provenance": map[string]any{"status": "executed", "runtime": "pydantic-ai", "logical_model": treatmentLogicalModelV1},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTreatmentHistoricalReplayIsDeterministicAndDoesNotCallAI(t *testing.T) {
	revision, _ := treatmentReplayFixture(t, defaultTreatmentConfigurationID)
	repo := &fakeTreatmentRepo{proposal: revision}
	ai := &fakeTreatmentReplayAI{}
	replay := NewTreatmentReplayService(repo, ai)

	report, err := replay.HistoricalReplay(context.Background(), uuid.New(), revision.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ai.calls != 0 {
		t.Fatalf("historical replay must not call AI, got %d calls", ai.calls)
	}
	if report.Mode != "historical" || report.GenerationDecision.Outcome != TreatmentAllowProposal {
		t.Fatalf("unexpected historical replay report: %#v", report)
	}
	if !report.ArtifactIntegrity.Match || !report.Comparison.Hard.Match || !report.Comparison.Semantic.Match || !report.Comparison.Presentation.Match {
		t.Fatalf("historical replay must reproduce stored artifact: %#v", report)
	}
}

func TestTreatmentCounterfactualReplayUsesExactFrozenInputAndIsReadOnly(t *testing.T) {
	revision, frozen := treatmentReplayFixture(t, defaultTreatmentConfigurationID)
	repo := &fakeTreatmentRepo{proposal: revision}
	ai := &fakeTreatmentReplayAI{result: treatmentReplayCounterfactualPayload(t, treatmentEvidenceGapConfigurationID)}
	replay := NewTreatmentReplayService(repo, ai)
	originalAcceptance := revision.AcceptanceState

	report, err := replay.CounterfactualReplay(
		context.Background(), uuid.New(), revision.ID, treatmentEvidenceGapConfigurationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ai.calls != 1 || ai.captured.ConfigurationID != treatmentEvidenceGapConfigurationID {
		t.Fatalf("counterfactual replay did not select exact Challenger: calls=%d req=%#v", ai.calls, ai.captured)
	}
	if ai.captured.BodyStateRevision != frozen.BodyStateRevision ||
		!jsonBytesEqual(ai.captured.BodyState, frozen.BodyState) ||
		!jsonBytesEqual(ai.captured.DiagnosisAnalysis, frozen.DiagnosisAnalysis) ||
		!jsonBytesEqual(ai.captured.CandidateAssessments, frozen.CandidateAssessments) ||
		!jsonBytesEqual(ai.captured.Profile, frozen.Profile) ||
		!jsonBytesEqual(ai.captured.UserConstraints, frozen.UserConstraints) ||
		!jsonBytesEqual(ai.captured.Evidence, frozen.Evidence) {
		t.Fatalf("counterfactual replay did not use exact frozen input: req=%#v frozen=%#v", ai.captured, frozen)
	}
	if revision.AcceptanceState != originalAcceptance || repo.current != nil || repo.acceptCalled {
		t.Fatalf("counterfactual replay mutated Treatment state: revision=%#v repo=%#v", revision, repo)
	}
	if !report.Comparison.Hard.Match {
		t.Fatalf("expected hard contract to remain green: %#v", report.Comparison.Hard)
	}
	if report.Comparison.Semantic.Match {
		t.Fatalf("intervention kind drift must be semantic mismatch: %#v", report.Comparison.Semantic)
	}
	if report.Comparison.Presentation.Match {
		t.Fatalf("summary/title drift must be presentation mismatch: %#v", report.Comparison.Presentation)
	}
}

func TestTreatmentReplayOldRevisionFailsClosed(t *testing.T) {
	revision, _ := treatmentReplayFixture(t, defaultTreatmentConfigurationID)
	revision.ReplayInput = datatypes.JSON(`{}`)
	ai := &fakeTreatmentReplayAI{}
	replay := NewTreatmentReplayService(&fakeTreatmentRepo{proposal: revision}, ai)

	_, err := replay.HistoricalReplay(context.Background(), uuid.New(), revision.ID)
	if !errors.Is(err, ErrTreatmentReplayUnavailable) {
		t.Fatalf("old revision must fail replay closed, got %v", err)
	}
	if ai.calls != 0 {
		t.Fatalf("unavailable historical replay must not call AI, got %d", ai.calls)
	}
}

func TestTreatmentReplayArtifactIntegrityDetectsFrozenRevisionDrift(t *testing.T) {
	revision, frozen := treatmentReplayFixture(t, defaultTreatmentConfigurationID)
	frozen.BodyStateRevision = 99
	raw, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	revision.ReplayInput = datatypes.JSON(raw)
	replay := NewTreatmentReplayService(&fakeTreatmentRepo{proposal: revision}, nil)

	report, err := replay.HistoricalReplay(context.Background(), uuid.New(), revision.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.ArtifactIntegrity.Match {
		t.Fatalf("frozen revision drift must fail artifact integrity: %#v", report.ArtifactIntegrity)
	}
}

func TestTreatmentRegressionExportRedactsDirectIdentifiers(t *testing.T) {
	revision, _ := treatmentReplayFixture(t, defaultTreatmentConfigurationID)
	replay := NewTreatmentReplayService(&fakeTreatmentRepo{proposal: revision}, nil)

	exported, err := replay.ExportRegressionCase(context.Background(), uuid.New(), revision.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"private-user", "profile-private", "private@example.test", "Private User"} {
		if contains := stringContains(text, forbidden); contains {
			t.Fatalf("regression export leaked %q: %s", forbidden, text)
		}
	}
	if !stringContains(text, "fact-neck") || !stringContains(text, "candidate-neck") {
		t.Fatalf("regression export removed domain evidence identities: %s", text)
	}
}

func stringContains(value, needle string) bool {
	return len(needle) > 0 && strings.Contains(value, needle)
}

func TestTreatmentCounterfactualReplayPreflightBlockSkipsAI(t *testing.T) {
	revision, frozen := treatmentReplayFixture(t, defaultTreatmentConfigurationID)
	frozen.GenerationFacts.SafetyState = json.RawMessage(`{"has_red_flags":true,"status":"requires_review"}`)
	raw, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	revision.ReplayInput = datatypes.JSON(raw)
	ai := &fakeTreatmentReplayAI{result: treatmentReplayCounterfactualPayload(t, treatmentEvidenceGapConfigurationID)}
	replay := NewTreatmentReplayService(&fakeTreatmentRepo{proposal: revision}, ai)

	report, err := replay.CounterfactualReplay(
		context.Background(), uuid.New(), revision.ID, treatmentEvidenceGapConfigurationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ai.calls != 0 {
		t.Fatalf("Go preflight block must bypass AI, got %d calls", ai.calls)
	}
	if report.GenerationDecision.Outcome != TreatmentBlock || report.Comparison.Hard.Match {
		t.Fatalf("blocked counterfactual must be visible as hard drift: %#v", report)
	}
}

func TestTreatmentCounterfactualReplayRejectsUnknownConfigurationWithoutAI(t *testing.T) {
	revision, _ := treatmentReplayFixture(t, defaultTreatmentConfigurationID)
	ai := &fakeTreatmentReplayAI{}
	replay := NewTreatmentReplayService(&fakeTreatmentRepo{proposal: revision}, ai)

	_, err := replay.CounterfactualReplay(context.Background(), uuid.New(), revision.ID, "treat-config-unknown")
	if err == nil || !strings.Contains(err.Error(), "unknown Treatment Agent configuration id") {
		t.Fatalf("expected unknown configuration error, got %v", err)
	}
	if ai.calls != 0 {
		t.Fatalf("unknown configuration must fail before AI, got %d calls", ai.calls)
	}
}
