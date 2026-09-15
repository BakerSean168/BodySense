package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func assessmentReplayTestReport() *model.AssessmentReport {
	now := time.Now().UTC()
	reportID := uuid.New()
	env, _ := encodeAssessmentReplayInput(
		defaultAssessmentConfigurationID,
		json.RawMessage(`{"gender":"male","birth_date":"1996-08-27"}`),
		json.RawMessage(`{"current_revision":3,"facts":[],"observations":[]}`),
		json.RawMessage(`[]`),
		json.RawMessage(`[]`),
		json.RawMessage(`{"has_analysis":true,"summaries":["正面观轻微高低肩"]}`),
		[]string{"data:image/jpeg;base64,AAAA"},
	)
	fingerprint := assessmentReplayInputFingerprintOfRaw(env)
	observations, _ := json.Marshal([]any{map[string]any{"kind": "posture_alignment", "label": "高低肩倾向"}})
	return &model.AssessmentReport{
		ID:                      reportID,
		UserID:                  uuid.New(),
		Status:                  "completed",
		ContractRevision:        assessmentOutputContractV2,
		EvidenceCoverage:        json.RawMessage(`{"status":"sufficient","available_sources":["posture_analysis"],"domains":{}}`),
		EvidenceGaps:            json.RawMessage(`[]`),
		Summary:                 "当前资料支持一项待审核观察。",
		Observations:            observations,
		InformationGaps:         json.RawMessage(`[]`),
		SafetyNotes:             json.RawMessage(`[]`),
		AgentConfigurationID:    defaultAssessmentConfigurationID,
		AgentConfiguration:      datatypes.JSON(fmt.Sprintf(`{"id":%q,"role":"assessment"}`, defaultAssessmentConfigurationID)),
		ExecutionProvenance:     datatypes.JSON(`{"status":"executed","runtime":"pydantic-ai"}`),
		GenerationDecisionTrace: datatypes.JSON(fmt.Sprintf(`{"status":"generated","outcome":"accepted","replay_input_fingerprint":%q}`, fingerprint)),
		ReplayInput:             datatypes.JSON(env),
		CreatedAt:               now,
	}
}

func newAssessmentReplaySvc() (*AssessmentReplayService, *fakeAssessmentRepository) {
	repo := &fakeAssessmentRepository{}
	return NewAssessmentReplayService(repo, nil), repo
}

func TestAssessmentHistoricalReplayRebuildsImmutableBaselineWithoutModel(t *testing.T) {
	svc, repo := newAssessmentReplaySvc()
	report := assessmentReplayTestReport()
	repo.created = report

	replayed, err := svc.HistoricalReplay(context.Background(), report.UserID, report.ID)
	if err != nil {
		t.Fatalf("HistoricalReplay: %v", err)
	}
	if replayed.Mode != "historical" {
		t.Fatalf("expected historical mode, got %q", replayed.Mode)
	}
	if replayed.SourceConfigurationID != defaultAssessmentConfigurationID {
		t.Fatalf("unexpected source config: %q", replayed.SourceConfigurationID)
	}
	if replayed.Baseline.Status != "completed" || replayed.Replay.Status != "completed" {
		t.Fatalf("baseline/replay status drift: %+v / %+v", replayed.Baseline, replayed.Replay)
	}
	if replayed.Baseline.ObservationCount != 1 {
		t.Fatalf("expected one observation, got %d", replayed.Baseline.ObservationCount)
	}
	if !replayed.ArtifactIntegrity.Match {
		t.Fatalf("artifact integrity must hold: %+v", replayed.ArtifactIntegrity)
	}
	// Regression guard for the S1 bug: a clean historical self-replay must report
	// Hard.Match == true (agent role identity must be validated, not the literal
	// string "assessment" compared against the config id).
	if !replayed.Comparison.Hard.Match {
		t.Fatalf("clean historical replay must report hard match, got %+v", replayed.Comparison.Hard)
	}
	if replayed.Comparison.Semantic.Match != true {
		t.Fatalf("clean historical replay must report semantic match, got %+v", replayed.Comparison.Semantic)
	}
	// ai is nil => historical replay performs no model call
	if svc.ai != nil {
		t.Fatal("historical replay must not require an AI client")
	}
}

func TestAssessmentReplayRejectsUnavailableFrozenInput(t *testing.T) {
	svc, repo := newAssessmentReplaySvc()
	report := assessmentReplayTestReport()
	report.ReplayInput = datatypes.JSON(`{}`)
	repo.created = report

	_, err := svc.HistoricalReplay(context.Background(), report.UserID, report.ID)
	if err == nil {
		t.Fatal("replay must fail when frozen input is unavailable")
	}
}

func TestAssessmentReplayCounterfactualRequiresRegisteredConfiguration(t *testing.T) {
	svc, repo := newAssessmentReplaySvc()
	report := assessmentReplayTestReport()
	repo.created = report

	// Unknown target configuration must fail closed without any AI call.
	_, err := svc.CounterfactualReplay(
		context.Background(), report.UserID, report.ID, "assess-config-does-not-exist",
	)
	if err == nil {
		t.Fatal("counterfactual replay must reject an unknown target configuration")
	}
}

func TestAssessmentCounterfactualRequiresConfiguredAIClientForCurrentConfiguration(t *testing.T) {
	svc, repo := newAssessmentReplaySvc()
	report := assessmentReplayTestReport()
	repo.created = report

	// The only runtime-addressable Assessment configuration is current. With no
	// AI client, replay passes policy validation and then fails at the model-call guard.
	_, err := svc.CounterfactualReplay(
		context.Background(), report.UserID, report.ID, defaultAssessmentConfigurationID,
	)
	if err == nil {
		t.Fatal("counterfactual replay without a configured AI client must fail")
	}
}

func TestAssessmentReplayRejectsRetiredSourceContract(t *testing.T) {
	svc, repo := newAssessmentReplaySvc()
	report := assessmentReplayTestReport()
	report.ContractRevision = "assessment-output-v1"
	report.AgentConfigurationID = retiredAssessmentV1ConfigurationID
	repo.created = report

	_, err := svc.HistoricalReplay(context.Background(), report.UserID, report.ID)
	if !errors.Is(err, ErrAssessmentReplayConfiguration) {
		t.Fatalf("retired source must fail closed, got %v", err)
	}
}

func TestAssessmentRegressionExportCarriesEvidenceContractSemantics(t *testing.T) {
	configID := defaultAssessmentConfigurationID
	env, _ := encodeAssessmentReplayInput(
		configID,
		json.RawMessage(`{"gender":"female","birth_date":"1996-08-27"}`),
		json.RawMessage(`{"current_revision":0,"facts":[],"observations":[]}`),
		json.RawMessage(`[]`),
		json.RawMessage(`[]`),
		json.RawMessage(`{}`),
		nil,
	)
	fingerprint := assessmentReplayInputFingerprintOfRaw(env)
	report := &model.AssessmentReport{
		ID:                      uuid.New(),
		UserID:                  uuid.New(),
		Status:                  "insufficient_information",
		ContractRevision:        assessmentOutputContractV2,
		EvidenceCoverage:        json.RawMessage(`{"status":"insufficient","available_sources":[],"domains":{}}`),
		EvidenceGaps:            json.RawMessage(`[{"dimension":"posture","description":"当前未提供已完成的体态分析。","needed_sources":["posture_analysis"],"required":false}]`),
		Observations:            json.RawMessage(`[]`),
		Summary:                 "当前资料支持 0 项待审核观察；0/6 个证据领域已有资料，6/6 个领域当前未提供资料。",
		SafetyNotes:             json.RawMessage(`[]`),
		AgentConfigurationID:    configID,
		AgentConfiguration:      datatypes.JSON(fmt.Sprintf(`{"id":%q,"role":"assessment"}`, configID)),
		ExecutionProvenance:     datatypes.JSON(`{"status":"skipped_no_evidence","runtime":"deterministic","usage":{"requests":0}}`),
		GenerationDecisionTrace: datatypes.JSON(fmt.Sprintf(`{"status":"derived_without_model","outcome":"accepted","replay_input_fingerprint":%q}`, fingerprint)),
		ReplayInput:             datatypes.JSON(env),
		CreatedAt:               time.Now().UTC(),
	}
	svc, repo := newAssessmentReplaySvc()
	repo.created = report

	exported, err := svc.ExportRegressionCase(context.Background(), report.UserID, report.ID)
	if err != nil {
		t.Fatalf("ExportRegressionCase: %v", err)
	}
	if exported["schema_target"] != AssessmentRegressionExportSchema || AssessmentRegressionExportSchema != "assessment_qualification_v2" {
		t.Fatalf("unexpected regression schema: %#v", exported["schema_target"])
	}
	casePayload, _ := exported["case"].(map[string]any)
	metadata, _ := casePayload["metadata"].(map[string]any)
	if metadata["expected_contract_revision"] != assessmentOutputContractV2 {
		t.Fatalf("missing contract revision metadata: %#v", metadata)
	}
	if metadata["expected_evidence_coverage_status"] != "insufficient" {
		t.Fatalf("missing evidence coverage metadata: %#v", metadata)
	}
	if metadata["expected_evidence_gap_count"] != 1 {
		t.Fatalf("missing evidence gap count: %#v", metadata)
	}
	if executed, _ := metadata["expected_agent_executed"].(bool); executed {
		t.Fatalf("no-evidence regression must expect no model execution: %#v", metadata)
	}
	fields, _ := metadata["forbidden_output_fields"].([]string)
	if !stringSliceContains(fields, "health_grade") || !stringSliceContains(fields, "dimension_scores") {
		t.Fatalf("v2 regression must forbid legacy score fields: %#v", metadata)
	}
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
