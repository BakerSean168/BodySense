package service

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestTreatmentAgentIdentityRejectsPolicyAndRuntimeDrift(t *testing.T) {
	validConfig := json.RawMessage(`{
		"id":"treat-config-85718f8e90ac9d80",
		"role":"treatment",
		"decision_policy_revision":"treatment-go-acceptance-v1"
	}`)
	validExecution := json.RawMessage(`{
		"status":"executed",
		"runtime":"pydantic-ai",
		"logical_model":"bodysense-structured"
	}`)

	cases := []struct {
		name      string
		config    json.RawMessage
		execution json.RawMessage
	}{
		{
			name: "decision-policy-drift",
			config: json.RawMessage(`{
				"id":"treat-config-85718f8e90ac9d80",
				"role":"treatment",
				"decision_policy_revision":"treatment-go-acceptance-v2"
			}`),
			execution: validExecution,
		},
		{
			name:   "logical-model-drift",
			config: validConfig,
			execution: json.RawMessage(`{
				"status":"executed",
				"runtime":"pydantic-ai",
				"logical_model":"unexpected-model"
			}`),
		},
		{
			name:   "runtime-drift",
			config: validConfig,
			execution: json.RawMessage(`{
				"status":"executed",
				"runtime":"direct-provider",
				"logical_model":"bodysense-structured"
			}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := &treatmentAgentPayload{
				AgentConfiguration:  tc.config,
				ExecutionProvenance: tc.execution,
			}
			if err := validateTreatmentAgentIdentity(payload, defaultTreatmentConfigurationID); !errors.Is(err, ErrTreatmentConfigurationMismatch) {
				t.Fatalf("expected fail-closed identity mismatch, got %v", err)
			}
		})
	}
}

func TestTreatmentEvidenceGapChallengerRecordsAcquisitionTrace(t *testing.T) {
	h := newTreatmentGenerationHarness(12)
	h.deployment = testTreatmentDeploymentPolicy{configurationID: treatmentEvidenceGapConfigurationID}
	trace := `{
		"trace_revision":"evidence-acquisition-trace-v2",
		"policy_revision":"treatment-evidence-gap-v2",
		"external_evidence_status":"available",
		"budget":{"max_searches":2,"max_results_per_search":5,"used_searches":1,"remaining_searches":1},
		"attempts":[{"gap":{"gap_id":"dose","kind":"external_knowledge","description":"dose evidence","rationale":"changes dose","critical":false,"query":"dose evidence"},"status":"evidence_returned","stop_reason":"evidence_returned","search_performed":true,"retrieval_status":"results_returned","query":"dose evidence","requested_top_k":5,"evidence_ids":["evidence-dose"]}],
		"unresolved_critical_gaps":[]
	}`
	block := strings.Replace(treatmentPlanBlockV1, `"evidenceIds":["selected-analysis-only"]`, `"evidenceIds":["evidence-dose"]`, 1)
	if _, err := h.generate(json.RawMessage(`{
		"status":"proposed","evidence_ids":["evidence-dose"],"evidence_acquisition":`+trace+`,
		"governance":{"verdict":"accepted"},"plan_proposal":`+block+`
	}`), nil); err != nil {
		t.Fatalf("GeneratePlanProposal returned error: %v", err)
	}
	provenance := h.provenance(t)
	if provenance["agentConfigurationId"] != treatmentEvidenceGapConfigurationID {
		t.Fatalf("unexpected Challenger identity: %v", provenance["agentConfigurationId"])
	}
	acquisition, ok := provenance["evidenceAcquisition"].(map[string]any)
	if !ok || acquisition["policy_revision"] != "treatment-evidence-gap-v2" {
		t.Fatalf("EvidenceGap trace was not recorded: %#v", provenance["evidenceAcquisition"])
	}
}

func TestTreatmentGenerationRecordsDecisionTrace(t *testing.T) {
	h := newTreatmentGenerationHarness(14)
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); err != nil {
		t.Fatalf("GeneratePlanProposal returned error: %v", err)
	}
	trace, ok := h.provenance(t)["generationDecisionTrace"].(map[string]any)
	if !ok {
		t.Fatal("generation DecisionTrace missing")
	}
	if trace["trace_revision"] != TreatmentDecisionTraceV1 || trace["phase"] != TreatmentDecisionGeneration || trace["outcome"] != string(TreatmentAllowProposal) {
		t.Fatalf("unexpected generation DecisionTrace: %#v", trace)
	}
	if trace["diagnosis_revision"] != float64(2) || trace["agent_configuration_id"] != defaultTreatmentConfigurationID {
		t.Fatalf("generation DecisionTrace lost durable identities: %#v", trace)
	}
	facts, ok := trace["facts"].(map[string]any)
	if !ok || facts["current_body_state_revision"] != float64(14) || facts["adopted_condition_count"] != float64(1) {
		t.Fatalf("generation DecisionTrace lost authority facts: %#v", trace)
	}
}

func TestTreatmentGenerationRecordsRolloutRoute(t *testing.T) {
	h := newTreatmentGenerationHarness(30)
	const offlineShadowConfigurationID = "treat-config-offline-shadow"
	h.deployment = testTreatmentDeploymentPolicy{
		configurationID:       defaultTreatmentConfigurationID,
		shadowConfigurationID: offlineShadowConfigurationID,
		stage:                 TreatmentRolloutShadow,
		subjectBucket:         2468,
		canaryBPS:             500,
	}
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); err != nil {
		t.Fatalf("served proposal must not depend on a shadow run: %v", err)
	}
	provenance := h.provenance(t)
	if provenance["agentConfigurationId"] != defaultTreatmentConfigurationID {
		t.Fatalf("a shadow route must not replace the served Champion: %#v", provenance)
	}
	raw, _ := json.Marshal(provenance["rolloutProvenance"])
	var route TreatmentRouteSelection
	if err := json.Unmarshal(raw, &route); err != nil {
		t.Fatalf("decode rollout provenance: %v", err)
	}
	if route.Stage != TreatmentRolloutShadow || route.SubjectBucket != 2468 ||
		route.ServedConfigurationID != defaultTreatmentConfigurationID ||
		route.ShadowConfigurationID != offlineShadowConfigurationID {
		t.Fatalf("unexpected Treatment rollout provenance: %#v", route)
	}
}

func TestTreatmentGenerationReadsAvailableEquipmentAsUserConstraint(t *testing.T) {
	h := newTreatmentGenerationHarness(9)
	notes := "medium"
	h.body.equipment = []AgentEquipmentItem{{ID: uuid.New(), DisplayName: "弹力带", Notes: &notes, AccessKind: "owned"}}
	callerConstraints := map[string]any{"time_per_day": "20m"}
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), callerConstraints); err != nil {
		t.Fatalf("GeneratePlanProposal returned error: %v", err)
	}
	captured := *h.captured
	want := json.RawMessage(`{"time_per_day":"20m","available_equipment":[{"name":"弹力带","access_kind":"owned","notes":"medium"}]}`)
	if h.body.viewOwner != h.userID || !jsonBytesEqual(captured.UserConstraints, want) {
		t.Fatalf("equipment constraint: captured=%s", captured.UserConstraints)
	}
	if _, mutated := callerConstraints["available_equipment"]; mutated {
		t.Fatal("caller constraints must not be mutated")
	}
	frozen, err := decodeTreatmentReplayInput(h.plans.inputs[0].Generation.ReplayInput)
	if err != nil || !jsonBytesEqual(frozen.UserConstraints, want) {
		t.Fatalf("replay input must freeze the equipment constraint: %v %s", err, frozen.UserConstraints)
	}
	// The agent BodyState input is the native V2 view, frozen as sent.
	var sent struct {
		Schema          string               `json:"schema"`
		CurrentRevision int64                `json:"current_revision"`
		Equipment       []AgentEquipmentItem `json:"equipment"`
		SafetyState     json.RawMessage      `json:"safety_state"`
	}
	if err := json.Unmarshal(captured.BodyState, &sent); err != nil || sent.Schema != AgentBodyContextSchemaV1 ||
		sent.CurrentRevision != 9 || len(sent.Equipment) != 1 || sent.SafetyState != nil {
		t.Fatalf("Treatment agent BodyState must be the V2 view without Go safety state: %v %s", err, captured.BodyState)
	}
	if !jsonBytesEqual(frozen.BodyState, captured.BodyState) {
		t.Fatalf("replay must freeze the exact agent BodyState: %s", frozen.BodyState)
	}
}

// decodeTreatmentReplayInput reads the frozen Agent request back.
func decodeTreatmentReplayInput(raw json.RawMessage) (TreatmentReplayInput, error) {
	var input TreatmentReplayInput
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return input, errors.New("treatment replay input is unavailable")
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return input, err
	}
	if input.BodyStateRevision <= 0 || len(input.BodyState) == 0 || !json.Valid(input.BodyState) {
		return input, errors.New("treatment replay input is unavailable")
	}
	return input, nil
}
