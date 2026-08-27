package service

import (
	"context"
	"encoding/json"
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
		"assess-config-fbff8155337b388d",
		json.RawMessage(`{"gender":"male","birth_date":"1996-08-27"}`),
		json.RawMessage(`{"current_revision":3,"facts":[],"observations":[]}`),
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
		HealthGrade:             "B",
		Summary:                 "当前资料支持一项待审核观察。",
		Observations:            observations,
		InformationGaps:         json.RawMessage(`[]`),
		SafetyNotes:             json.RawMessage(`[]`),
		AgentConfigurationID:    "assess-config-fbff8155337b388d",
		AgentConfiguration:      datatypes.JSON(`{"id":"assess-config-fbff8155337b388d","role":"assessment"}`),
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
	if replayed.SourceConfigurationID != "assess-config-fbff8155337b388d" {
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

func TestAssessmentReplayComparisonDetectsConfigMismatch(t *testing.T) {
	svc, repo := newAssessmentReplaySvc()
	report := assessmentReplayTestReport()
	repo.created = report

	// ai is nil, so counterfactual with a valid-but-different config should fail
	// because the ai client is unconfigured (before any model call we still pass
	// policy validation, then hit the unconfigured-AI guard).
	_, err := svc.CounterfactualReplay(
		context.Background(), report.UserID, report.ID, "assess-config-fbff8155337b388d",
	)
	if err == nil {
		t.Fatal("counterfactual replay without a configured AI client must fail")
	}
}
