package service

import (
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestDiagnosisHistoricalReplayConfigurationsRemainPinned(t *testing.T) {
	got := DiagnosisHistoricalReplayConfigurations()
	want := []struct {
		version string
		id      string
	}{
		{"v3", "diag-config-5a4a13627e14b4cf"},
		{"v4", "diag-config-4a517fea19cb6c49"},
		{"v5", "diag-config-375187050b203078"},
		{"v6", "diag-config-4377355ba2012ce8"},
		{"v7", "diag-config-4eb948f419994367"},
	}
	if len(got) != len(want) {
		t.Fatalf("historical identity count=%d want=%d", len(got), len(want))
	}
	for i, item := range got {
		if item.Version != want[i].version || item.ConfigurationID != want[i].id || item.DecisionPolicyRevision != DiagnosisDecisionPolicyV1 {
			t.Fatalf("historical identity[%d]=%#v want version=%s id=%s v1", i, item, want[i].version, want[i].id)
		}
		policy, err := DiagnosisDecisionPolicyRevisionForConfiguration(item.ConfigurationID)
		if err != nil || policy != DiagnosisDecisionPolicyV1 {
			t.Fatalf("historical resolver %s policy=%q err=%v", item.ConfigurationID, policy, err)
		}
	}
}

func TestHistoricalDiagnosisReplayV3ThroughV7RemainReadableWithoutModelCall(t *testing.T) {
	for _, historical := range DiagnosisHistoricalReplayConfigurations() {
		historical := historical
		t.Run(historical.Version, func(t *testing.T) {
			diagnosis, _, userID, analysisID := persistReplayTestAnalysis(
				t, historical.ConfigurationID, historical.DecisionPolicyRevision, "region:neck",
			)
			report, err := NewDiagnosisReplayService(diagnosis, nil).HistoricalReplay(
				t.Context(), userID, analysisID,
			)
			if err != nil {
				t.Fatalf("HistoricalReplay(%s): %v", historical.Version, err)
			}
			if report.SourceConfigurationID != historical.ConfigurationID || report.TargetConfigurationID != historical.ConfigurationID {
				t.Fatalf("historical replay identity drifted: %#v", report)
			}
			if !report.ArtifactIntegrity.Match || !report.Comparison.Hard.Match || !report.Comparison.Semantic.Match || !report.Comparison.Presentation.Match {
				t.Fatalf("historical replay %s changed frozen invariants: %#v", historical.Version, report)
			}
		})
	}
}

func TestLegacyDiagnosisReplayInputWithoutSafetyEnvelopeStillDecodes(t *testing.T) {
	raw := []byte(`{"body_state_revision":12,"body_state":{"current_revision":12,"facts":[],"observations":[]},"relevant_history":[],"profile":{}}`)
	input, err := decodeDiagnosisReplayInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if input.BodyStateRevision != 12 || input.SafetyEnvelope != nil {
		t.Fatalf("legacy replay input changed semantics: %#v", input)
	}
	if diagnosisReplayInputFingerprint(input) == "" {
		t.Fatal("legacy replay input must retain a stable fingerprint")
	}
}

func TestDiagnosisHistorySnapshotAllowsAdditionsButRejectsMutation(t *testing.T) {
	now := time.Date(2026, 9, 30, 6, 0, 0, 0, time.UTC)
	analysisID := uuid.New()
	userID := uuid.New()
	candidateID := uuid.New()
	analysis := model.DiagnosisAnalysisRecord{
		ID:                       analysisID,
		UserID:                   userID,
		BodyStateRevision:        12,
		Status:                   "completed",
		Scope:                    "full_body",
		Summary:                  "stable",
		CrossConcernPatterns:     datatypes.JSON(`[]`),
		InformationGaps:          datatypes.JSON(`[]`),
		SafetySummary:            datatypes.JSON(`{"b":2,"a":1}`),
		Citations:                datatypes.JSON(`[]`),
		Governance:               datatypes.JSON(`{"verdict":"accepted"}`),
		AgentConfigurationID:     diagnosisSafetyBudgetConfigID,
		AgentConfiguration:       datatypes.JSON(`{"id":"diag-config-3f64de162dc937ee"}`),
		DecisionTrace:            datatypes.JSON(`{"trace_revision":"diagnosis-decision-trace-v1"}`),
		ExecutionProvenance:      datatypes.JSON(`{"status":"executed"}`),
		EvidenceAcquisitionTrace: datatypes.JSON(`{}`),
		ReplayInput:              datatypes.JSON(`{"body_state_revision":12}`),
		RawOutput:                datatypes.JSON(`{"status":"completed"}`),
		CreatedAt:                now,
	}
	candidate := model.DiagnosisCandidateRecord{
		ID:                    candidateID,
		AnalysisID:            analysisID,
		Ordinal:               0,
		Name:                  "stable candidate",
		Confidence:            "high",
		Basis:                 "frozen",
		TypicalSymptoms:       "none",
		BasisFactIDs:          datatypes.JSON(`[]`),
		BasisObservationIDs:   datatypes.JSON(`[]`),
		SupportingEvidenceIDs: datatypes.JSON(`[]`),
		CounterevidenceIDs:    datatypes.JSON(`[]`),
		MissingInformation:    datatypes.JSON(`[]`),
		SafetyNotes:           datatypes.JSON(`[]`),
		RawPayload:            datatypes.JSON(`{"confidence":"high","name":"stable candidate"}`),
		CreatedAt:             now,
	}
	before, err := BuildDiagnosisHistorySnapshot([]model.DiagnosisAnalysisRecord{analysis}, []model.DiagnosisCandidateRecord{candidate}, now)
	if err != nil {
		t.Fatal(err)
	}

	// JSON key order is not a mutation.
	analysis.SafetySummary = datatypes.JSON(`{"a":1,"b":2}`)
	added := analysis
	added.ID = uuid.New()
	added.RawOutput = datatypes.JSON(`{"status":"partial"}`)
	current, err := BuildDiagnosisHistorySnapshot([]model.DiagnosisAnalysisRecord{analysis, added}, []model.DiagnosisCandidateRecord{candidate}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	verified := VerifyDiagnosisHistorySnapshot(before, current)
	if !verified.Unchanged || verified.AddedAnalysisCount != 1 || verified.AddedCandidateCount != 0 {
		t.Fatalf("additions must not invalidate protected history: %#v", verified)
	}
	if verified.BeforeRootSHA256 != verified.ProtectedRootSHA256 {
		t.Fatalf("protected root drifted without mutation: %#v", verified)
	}

	analysis.Summary = "mutated"
	mutated, err := BuildDiagnosisHistorySnapshot([]model.DiagnosisAnalysisRecord{analysis, added}, []model.DiagnosisCandidateRecord{candidate}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	verified = VerifyDiagnosisHistorySnapshot(before, mutated)
	if verified.Unchanged || len(verified.MutatedAnalysisIDs) != 1 || verified.MutatedAnalysisIDs[0] != analysisID.String() {
		t.Fatalf("immutable mutation was not detected: %#v", verified)
	}
}
