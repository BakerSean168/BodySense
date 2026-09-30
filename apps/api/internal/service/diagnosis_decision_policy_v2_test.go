package service

import (
	"encoding/json"
	"testing"
)

func completeSafetyEnvelope() *SafetyEnvelopeV2 {
	return &SafetyEnvelopeV2{
		SchemaRevision: SafetyEnvelopeSchemaV2, PolicyRevision: SafetyEnvelopePolicyV1, BodyStateRevision: 12,
		Assertions: []SafetyAssertionV1{}, ActiveBlockers: []SafetyBlockerV1{},
		Coverage: SafetyCoverageV1{
			Revision: SafetyCoverageRevisionV1, CaptureRevision: SafetyCaptureRevisionV1,
			RequiredConcepts:  []string{"trauma", "radiating_pain", "numbness", "weakness", "dizziness"},
			CoveredSourceRefs: []string{"body-state:fact:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}, IncompleteSourceRefs: []string{}, Complete: true,
		},
	}
}

func completedV2Payload() map[string]any {
	return map[string]any{"status": "completed", "candidates": []any{map[string]any{"name": "candidate", "basis": "头晕"}}, "safety_summary": "头晕", "red_flags": map[string]any{"has_red_flags": true}, "governance": map[string]any{"verdict": "accepted"}, "safety_findings": []any{}}
}

func TestDiagnosisDecisionPolicyV2DenyOverrides(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*SafetyEnvelopeV2, map[string]any)
		outcome DiagnosisDecisionOutcome
		reason  string
	}{
		{"normal ignores prose and legacy flags", func(*SafetyEnvelopeV2, map[string]any) {}, DiagnosisAllowNormal, ""},
		{"blocker", func(e *SafetyEnvelopeV2, _ map[string]any) {
			e.RequiresReview = true
			e.ActiveBlockers = []SafetyBlockerV1{{Concept: SafetyDizziness, SourceRef: "body-state:fact:x", SourceKind: SafetyBodyStateFact, Reason: SafetyStructuredCurrentSignal}}
		}, DiagnosisBlock, "active_structured_safety_blocker"},
		{"coverage incomplete", func(e *SafetyEnvelopeV2, _ map[string]any) {
			e.Coverage.Complete = false
			e.Coverage.IncompleteSourceRefs = []string{"body-state:fact:b"}
		}, DiagnosisAbstain, "structured_safety_capture_incomplete"},
		{"governance rejected", func(_ *SafetyEnvelopeV2, p map[string]any) { p["governance"] = map[string]any{"verdict": "rejected"} }, DiagnosisBlock, "agent_output_failed_safety_governance"},
		{"typed finding", func(_ *SafetyEnvelopeV2, p map[string]any) {
			p["safety_findings"] = []any{map[string]any{"concept": "dizziness", "polarity": "uncertain", "temporality": "current", "source_refs": []any{"body-state:fact:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}}
		}, DiagnosisEscalate, "new_structured_runtime_safety_signal"},
		{"invalid finding", func(_ *SafetyEnvelopeV2, p map[string]any) {
			p["safety_findings"] = []any{map[string]any{"concept": "dizziness", "polarity": "absent", "temporality": "current", "source_refs": []any{"body-state:fact:x"}}}
		}, DiagnosisBlock, "malformed_structured_safety_findings"},
		{"critical gap", func(_ *SafetyEnvelopeV2, p map[string]any) {
			p["evidence_acquisition"] = map[string]any{"unresolved_critical_gaps": []any{"missing"}}
		}, DiagnosisAbstain, "critical_evidence_gap_unresolved"},
		{"insufficient", func(_ *SafetyEnvelopeV2, p map[string]any) { p["status"] = "insufficient_information" }, DiagnosisAbstain, "insufficient_information"},
		{"partial", func(_ *SafetyEnvelopeV2, p map[string]any) { p["status"] = "partial" }, DiagnosisAllowDegraded, "partial_or_degraded_analysis"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, p := completeSafetyEnvelope(), completedV2Payload()
			tt.mutate(e, p)
			got := EvaluateDiagnosisDecisionV2(e, 12, p)
			if got.Outcome != tt.outcome || (tt.reason != "" && (len(got.Reasons) == 0 || got.Reasons[0] != tt.reason)) {
				t.Fatalf("unexpected decision: %+v", got)
			}
			if tt.outcome != DiagnosisAllowNormal && len(ApplyDiagnosisDecision(p, got)["candidates"].([]any)) != 0 && tt.outcome != DiagnosisAllowDegraded {
				t.Fatal("denied decision retained ordinary candidates")
			}
		})
	}
	if got := EvaluateDiagnosisDecisionV2(nil, 12, completedV2Payload()); got.Outcome != DiagnosisBlock {
		t.Fatalf("missing envelope allowed: %+v", got)
	}
	if got := EvaluateDiagnosisDecisionV2(completeSafetyEnvelope(), 13, completedV2Payload()); got.Outcome != DiagnosisBlock {
		t.Fatalf("revision mismatch allowed: %+v", got)
	}
}

func TestStructuredDiagnosisPreflightBypassesIncompleteAndBlocked(t *testing.T) {
	route := DiagnosisRouteSelection{ServedConfigurationID: diagnosisStructuredSafetyConfigID, ServedDecisionPolicyRevision: DiagnosisDecisionPolicyV2}
	for _, test := range []struct {
		name    string
		mutate  func(*SafetyEnvelopeV2)
		outcome DiagnosisDecisionOutcome
		status  string
	}{
		{"incomplete", func(e *SafetyEnvelopeV2) {
			e.Coverage.Complete = false
			e.Coverage.IncompleteSourceRefs = []string{"body-state:fact:b"}
		}, DiagnosisAbstain, "insufficient_information"},
		{"blocked", func(e *SafetyEnvelopeV2) {
			e.RequiresReview = true
			e.ActiveBlockers = []SafetyBlockerV1{{Concept: SafetyDizziness, SourceRef: "body-state:fact:a", SourceKind: SafetyBodyStateFact, Reason: SafetyStructuredCurrentSignal}}
		}, DiagnosisBlock, "safety_blocked"},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := completeSafetyEnvelope()
			test.mutate(e)
			var payload map[string]any
			if err := json.Unmarshal(structuredDiagnosisPreflight(e, 12, diagnosisStructuredSafetyConfigID, route), &payload); err != nil {
				t.Fatal(err)
			}
			decision := payload["decision_authority"].(map[string]any)
			if decision["outcome"] != string(test.outcome) || payload["status"] != test.status || len(payload["candidates"].([]any)) != 0 {
				t.Fatalf("unexpected preflight payload: %+v", payload)
			}
			governance := payload["governance"].(map[string]any)
			if _, legacy := payload["safety_summary"].(string); legacy {
				t.Fatalf("v8 preflight included legacy SafetyState prose: %+v", payload)
			}
			if test.outcome == DiagnosisAbstain {
				if governance["verdict"] != "accepted" || len(governance["reasons"].([]any)) != 0 ||
					len(decision["reasons"].([]any)) != 1 || decision["reasons"].([]any)[0] != "structured_safety_capture_incomplete" ||
					payload["execution_provenance"].(map[string]any)["reason"] != "structured_safety_coverage_incomplete" ||
					payload["summary"] != "结构化安全信息尚未完整采集，请完成安全信号确认后重新分析。" {
					t.Fatalf("inconsistent coverage abstain: %+v", payload)
				}
			} else if governance["verdict"] != "rejected" ||
				len(decision["reasons"].([]any)) != 1 || decision["reasons"].([]any)[0] != "active_structured_safety_blocker" ||
				len(governance["reasons"].([]any)) != 1 || governance["reasons"].([]any)[0] != "active_structured_safety_blocker" ||
				payload["execution_provenance"].(map[string]any)["reason"] != "active_structured_safety_blocker" {
				t.Fatalf("inconsistent structured blocker: %+v", payload)
			}
		})
	}
	if blocked := structuredDiagnosisPreflight(completeSafetyEnvelope(), 12, diagnosisStructuredSafetyConfigID, route); blocked != nil {
		t.Fatalf("complete clear coverage bypassed agent: %s", blocked)
	}
}

func TestStructuredDiagnosisConfigurationRegistration(t *testing.T) {
	if diagnosisStructuredSafetyConfigID != "diag-config-62d312942b76a154" {
		t.Fatal("v8 configuration identity drift")
	}
	revision, err := DiagnosisDecisionPolicyRevisionForConfiguration(diagnosisStructuredSafetyConfigID)
	if err != nil || revision != DiagnosisDecisionPolicyV2 {
		t.Fatalf("v8 decision policy registration: %q, %v", revision, err)
	}
	if defaultDiagnosisConfigurationID != diagnosisSafetyBudgetConfigID {
		t.Fatal("current default Champion must be v10")
	}
	record := knownDiagnosisPromotionRecords["diagnosis_promotion_v6"]
	if record.ChampionConfigurationID != diagnosisDecisionAuthorityConfigID || record.ChallengerConfigurationID != diagnosisStructuredSafetyConfigID {
		t.Fatalf("v8 promotion registration: %+v", record)
	}
}
