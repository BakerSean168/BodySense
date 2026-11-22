package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/bodysense/api/internal/model"
	"log/slog"
	"sort"
	"strings"
)

func assessmentRegressionForbiddenFields(_ string) []string {
	return []string{"treatment", "training_plan", "prescription", "health_grade", "dimension_scores"}
}

func assessmentReplayBaseline(report *model.AssessmentReport) (map[string]any, error) {
	baseline := map[string]any{
		"contract_revision": assessmentOutputContractV2,
		"status":            string(report.Status),
		"summary":           report.Summary,
	}
	if len(report.EvidenceCoverage) > 0 {
		var coverage map[string]any
		if json.Unmarshal(report.EvidenceCoverage, &coverage) == nil {
			baseline["evidence_coverage"] = coverage
		}
	}
	if len(report.EvidenceGaps) > 0 {
		var evidenceGaps []any
		if json.Unmarshal(report.EvidenceGaps, &evidenceGaps) == nil {
			baseline["evidence_gaps"] = evidenceGaps
		}
	}
	var obs []any
	if len(report.Observations) > 0 {
		if err := json.Unmarshal(report.Observations, &obs); err != nil {
			// Observations drive the hard observation_count check and the
			// exported expected_observations baseline; silently degrading to
			// empty would make a corrupted report replay as a match.
			return nil, fmt.Errorf("assessment replay baseline: report %s observations: %w", report.ID, err)
		}
		baseline["observations"] = obs
	}
	var gaps []any
	if len(report.InformationGaps) > 0 {
		if err := json.Unmarshal(report.InformationGaps, &gaps); err != nil {
			// Information gaps feed the semantic information_gaps comparison;
			// degrading to empty would mask real gaps behind a corrupted column.
			return nil, fmt.Errorf("assessment replay baseline: report %s information_gaps: %w", report.ID, err)
		}
	}
	baseline["information_gaps"] = gaps
	var safety []any
	if len(report.SafetyNotes) > 0 {
		// Safety notes are informational only: no replay check consumes them,
		// so a corrupt column degrades the snapshot instead of the verdict.
		if err := json.Unmarshal(report.SafetyNotes, &safety); err != nil {
			slog.Warn(
				"assessment replay baseline: corrupt safety_notes JSON, defaulting to empty",
				slog.String("report_id", report.ID.String()),
				slog.String("field", "safety_notes"),
				"error", err,
			)
		}
	}
	baseline["safety_notes"] = safety
	baseline["agent_configuration"] = map[string]any{"id": report.AgentConfigurationID, "role": "assessment"}
	return baseline, nil
}

func assessmentReplaySnapshot(payload map[string]any) AssessmentReplaySnapshot {
	obs := observationsFromPayload(payload)
	kinds := make([]string, 0, len(obs))
	for _, o := range obs {
		if k, ok := o["kind"].(string); ok && k != "" {
			kinds = append(kinds, k)
		}
	}
	sort.Strings(kinds)
	gaps := stringSlice(payload["information_gaps"])
	sort.Strings(gaps)
	evidenceGapDescriptions := assessmentReplayEvidenceGapDescriptions(payload["evidence_gaps"])
	return AssessmentReplaySnapshot{
		ContractRevision:       firstString(payload["contract_revision"], assessmentOutputContractV2),
		Status:                 firstString(payload["status"], ""),
		EvidenceCoverageStatus: assessmentReplayCoverageStatus(payload["evidence_coverage"]),
		ObservationCount:       len(obs),
		ObservationKinds:       kinds,
		InformationGaps:        gaps,
		EvidenceGaps:           evidenceGapDescriptions,
		SafetyNoteCount:        len(stringSlice(payload["safety_notes"])),
		Summary:                firstString(payload["summary"], ""),
	}
}

func assessmentReplayCoverageStatus(value any) string {
	coverage, _ := value.(map[string]any)
	status, _ := coverage["status"].(string)
	return status
}

func assessmentReplayEvidenceGapDescriptions(value any) []string {
	raw, _ := value.([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		description, _ := obj["description"].(string)
		if description != "" {
			out = append(out, description)
		}
	}
	sort.Strings(out)
	return out
}

func observationsFromPayload(payload map[string]any) []map[string]any {
	raw, _ := payload["observations"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func assessmentReplayArtifactIntegrity(
	report *model.AssessmentReport,
	input AssessmentReplayInput,
	baseline map[string]any,
) (AssessmentReplayLayer, error) {
	recomputedFingerprint := assessmentReplayInputFingerprint(input)
	// A corrupt generation trace must surface as a replay error, not masquerade
	// as an ordinary fingerprint mismatch inside the integrity layer.
	storedFingerprint, err := storedReplayInputFingerprint(json.RawMessage(report.GenerationDecisionTrace))
	if err != nil {
		return AssessmentReplayLayer{}, fmt.Errorf("assessment replay integrity: report %s: %w", report.ID, err)
	}
	checks := []AssessmentReplayCheck{
		{
			Name:      "source_configuration_id_matches_replay_envelope",
			Match:     report.AgentConfigurationID == input.ConfigurationID,
			Baseline:  report.AgentConfigurationID,
			Candidate: input.ConfigurationID,
		},
		{
			Name:      "replay_input_fingerprint_matches_generation_trace",
			Match:     storedFingerprint != "" && storedFingerprint == recomputedFingerprint,
			Baseline:  storedFingerprint,
			Candidate: recomputedFingerprint,
		},
		{Name: "replay_input_frozen", Match: len(input.Profile) > 0},
		{Name: "report_immutable_status", Match: baseline["status"] != nil},
	}
	all := true
	for i := range checks {
		all = all && checks[i].Match
	}
	return AssessmentReplayLayer{Match: all, Checks: checks}, nil
}

func buildAssessmentReplayReport(
	mode string,
	report *model.AssessmentReport,
	input AssessmentReplayInput,
	targetConfigurationID string,
	baseline map[string]any,
	replayed map[string]any,
	replayRaw json.RawMessage,
) (*AssessmentReplayReport, error) {
	integrity, err := assessmentReplayArtifactIntegrity(report, input, baseline)
	if err != nil {
		return nil, err
	}
	return &AssessmentReplayReport{
		Mode:                  mode,
		SourceReportID:        report.ID,
		SourceConfigurationID: report.AgentConfigurationID,
		TargetConfigurationID: targetConfigurationID,
		InputFingerprint:      assessmentReplayInputFingerprint(input),
		ArtifactIntegrity:     integrity,
		Baseline:              assessmentReplaySnapshot(baseline),
		Replay:                assessmentReplaySnapshot(replayed),
		Comparison:            compareAssessmentReplayOutputs(baseline, replayed),
		Output:                replayRaw,
	}, nil
}

func assessmentReplayInputFingerprint(input AssessmentReplayInput) string {
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func assessmentReplayInputFingerprintOfRaw(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// storedReplayInputFingerprint reads the fingerprint frozen at generation time
// from the report's generation decision trace.
func storedReplayInputFingerprint(trace json.RawMessage) (string, error) {
	var payload struct {
		ReplayInputFingerprint string `json:"replay_input_fingerprint"`
	}
	if err := json.Unmarshal(trace, &payload); err != nil {
		return "", fmt.Errorf("decode generation decision trace: %w", err)
	}
	return payload.ReplayInputFingerprint, nil
}

func assessmentReplayConfigurationMatches(payload map[string]any, expectedID string) bool {
	configuration, ok := payload["agent_configuration"].(map[string]any)
	if !ok {
		return false
	}
	id, _ := configuration["id"].(string)
	role, _ := configuration["role"].(string)
	return id == expectedID && role == "assessment"
}

func assessmentReplayRoleIsAssessment(payload map[string]any) bool {
	configuration, ok := payload["agent_configuration"].(map[string]any)
	if !ok {
		return false
	}
	role, _ := configuration["role"].(string)
	return role == "assessment"
}

func cloneAssessmentPayload(payload map[string]any) map[string]any {
	recomputed := make(map[string]any, len(payload))
	for key, value := range payload {
		recomputed[key] = value
	}
	return recomputed
}

func compareAssessmentReplayOutputs(baseline, replayed map[string]any) AssessmentReplayComparison {
	bSnap := assessmentReplaySnapshot(baseline)
	rSnap := assessmentReplaySnapshot(replayed)

	hardChecks := []AssessmentReplayCheck{
		{Name: "contract_revision", Match: bSnap.ContractRevision == rSnap.ContractRevision, Baseline: bSnap.ContractRevision, Candidate: rSnap.ContractRevision},
		{Name: "status", Match: bSnap.Status == rSnap.Status, Baseline: bSnap.Status, Candidate: rSnap.Status},
		{Name: "observation_count", Match: bSnap.ObservationCount == rSnap.ObservationCount, Baseline: fmt.Sprintf("%d", bSnap.ObservationCount), Candidate: fmt.Sprintf("%d", rSnap.ObservationCount)},
		// Identity of the Agent role is checked here; the exact configuration ID
		// equality is verified separately (counterfactual at the call site via
		// assessmentReplayConfigurationMatches, historical via ArtifactIntegrity).
		{Name: "agent_configuration_role", Match: assessmentReplayRoleIsAssessment(replayed)},
	}
	semanticChecks := []AssessmentReplayCheck{
		{Name: "observation_kinds", Match: equalStringSlices(bSnap.ObservationKinds, rSnap.ObservationKinds), Baseline: strings.Join(bSnap.ObservationKinds, ","), Candidate: strings.Join(rSnap.ObservationKinds, ",")},
		{Name: "information_gaps", Match: equalStringSlices(bSnap.InformationGaps, rSnap.InformationGaps), Baseline: strings.Join(bSnap.InformationGaps, "|"), Candidate: strings.Join(rSnap.InformationGaps, "|")},
		{Name: "evidence_coverage_status", Match: bSnap.EvidenceCoverageStatus == rSnap.EvidenceCoverageStatus, Baseline: bSnap.EvidenceCoverageStatus, Candidate: rSnap.EvidenceCoverageStatus},
		{Name: "evidence_gaps", Match: equalStringSlices(bSnap.EvidenceGaps, rSnap.EvidenceGaps), Baseline: strings.Join(bSnap.EvidenceGaps, "|"), Candidate: strings.Join(rSnap.EvidenceGaps, "|")},
	}
	presentationChecks := []AssessmentReplayCheck{
		{Name: "summary", Match: bSnap.Summary == rSnap.Summary, Baseline: bSnap.Summary, Candidate: rSnap.Summary},
	}
	return AssessmentReplayComparison{
		Hard:         assessmentReplayLayer(hardChecks),
		Semantic:     assessmentReplayLayer(semanticChecks),
		Presentation: assessmentReplayLayer(presentationChecks),
	}
}

func assessmentReplayLayer(checks []AssessmentReplayCheck) AssessmentReplayLayer {
	all := true
	for i := range checks {
		all = all && checks[i].Match
	}
	return AssessmentReplayLayer{Match: all, Checks: checks}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func firstString(value any, fallback string) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fallback
}

func stringSlice(value any) []string {
	raw, _ := value.([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
