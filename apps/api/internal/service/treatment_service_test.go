package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type fakeTreatmentDiagnosis struct {
	analysis    *model.DiagnosisAnalysisRecord
	assessments []model.DiagnosisCandidateAssessment
}

func (f *fakeTreatmentDiagnosis) GetByID(context.Context, uuid.UUID, uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return f.analysis, nil
}
func (f *fakeTreatmentDiagnosis) GetLatest(context.Context, uuid.UUID) (*model.DiagnosisAnalysisRecord, error) {
	return f.analysis, nil
}
func (f *fakeTreatmentDiagnosis) ListAssessments(context.Context, uuid.UUID, uuid.UUID) ([]model.DiagnosisCandidateAssessment, error) {
	return f.assessments, nil
}
func (f *fakeTreatmentDiagnosis) PublicPayload(analysis *model.DiagnosisAnalysisRecord) map[string]any {
	return map[string]any{"analysis_id": analysis.ID}
}

type fakeTreatmentBodyState struct {
	snapshot          *BodyStateReadModel
	equipment         []AgentEquipmentItem
	viewOwner         uuid.UUID
	safetyUnavailable bool
	safetyBlocked     bool
}

func (f *fakeTreatmentBodyState) GetAgentBodyContext(_ context.Context, owner uuid.UUID) (*AgentBodyContext, error) {
	f.viewOwner = owner
	envelope := testNativeSafetyEnvelope(f.snapshot.CurrentRevision)
	if f.safetyBlocked {
		envelope = testBlockedSafetyEnvelope(f.snapshot.CurrentRevision)
	}
	if f.safetyUnavailable {
		envelope = nil
	}
	return &AgentBodyContext{
		SafetyEnvelope: envelope,
		Schema:         AgentBodyContextSchemaV1, UserID: owner, CurrentRevision: f.snapshot.CurrentRevision, RecordsAvailable: true,
		Facts: []AgentBodyFact{}, Observations: []AgentBodyObservation{}, Episodes: []AgentSymptomEpisode{},
		UserHypotheses: []AgentUserHypothesis{}, Equipment: append([]AgentEquipmentItem{}, f.equipment...),
		Safety: AgentSafetySummary{Status: AgentSafetyNoCurrentSymptoms, ActiveBlockers: []string{}, UnscreenedFactIDs: []uuid.UUID{}},
	}, nil
}

type fakeTreatmentFreshness struct{ state string }

func (f fakeTreatmentFreshness) GetOrEvaluate(context.Context, uuid.UUID, *model.DiagnosisAnalysisRecord) (*model.DiagnosisAnalysisFreshness, error) {
	return &model.DiagnosisAnalysisFreshness{State: f.state}, nil
}

type testTreatmentDeploymentPolicy struct {
	configurationID       string
	shadowConfigurationID string
	stage                 string
	subjectBucket         int
	canaryBPS             int
}

func (p testTreatmentDeploymentPolicy) SelectTreatmentRoute(_ string) TreatmentRouteSelection {
	served := p.configurationID
	if served == "" {
		served = defaultTreatmentConfigurationID
	}
	stage := p.stage
	if stage == "" {
		stage = TreatmentRolloutChampion
	}
	champion := served
	challenger := ""
	if p.shadowConfigurationID != "" {
		// Rollout-focused tests explicitly model the historical v1 -> v2 pair.
		// Champion-only tests model the current v2 baseline with no Challenger.
		if (served == treatmentV1ConfigurationID && p.shadowConfigurationID == treatmentEvidenceGapConfigurationID) ||
			(served == treatmentEvidenceGapConfigurationID && p.shadowConfigurationID == treatmentV1ConfigurationID) {
			champion = treatmentV1ConfigurationID
			challenger = treatmentEvidenceGapConfigurationID
		} else {
			challenger = p.shadowConfigurationID
		}
	}
	registration := knownTreatmentConfigurations[served]
	route := TreatmentRouteSelection{
		Stage: stage, SubjectBucket: p.subjectBucket, ServedConfigurationID: served,
		ServedDecisionPolicyRevision: registration.DecisionPolicyRevision,
		ShadowConfigurationID:        p.shadowConfigurationID,
		ChampionConfigurationID:      champion, ChallengerConfigurationID: challenger,
		CanaryBPS: p.canaryBPS,
	}
	if route.ShadowConfigurationID != "" {
		route.ShadowDecisionPolicyRevision = knownTreatmentConfigurations[route.ShadowConfigurationID].DecisionPolicyRevision
	}
	return route
}

type fakeTreatmentReasoner struct {
	raw     json.RawMessage
	capture *TreatmentRecommendationRequest
}

func (f fakeTreatmentReasoner) RecommendTreatment(_ context.Context, req TreatmentRecommendationRequest) (json.RawMessage, error) {
	if f.capture != nil {
		*f.capture = req
	}
	if len(f.raw) == 0 {
		return f.raw, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(f.raw, &payload); err != nil {
		return f.raw, nil
	}
	if _, exists := payload["agent_configuration"]; !exists {
		payload["agent_configuration"] = map[string]any{
			"id": req.ConfigurationID, "role": "treatment",
			"decision_policy_revision": TreatmentDecisionPolicyV1,
		}
	}
	if _, exists := payload["execution_provenance"]; !exists {
		payload["execution_provenance"] = map[string]any{
			"status": "executed", "runtime": "pydantic-ai",
			"logical_model": treatmentLogicalModelV1,
		}
	}
	if req.ConfigurationID == treatmentEvidenceGapConfigurationID {
		if _, exists := payload["evidence_acquisition"]; !exists {
			payload["evidence_acquisition"] = map[string]any{
				"trace_revision":           evidenceAvailabilityTraceV2,
				"policy_revision":          "treatment-evidence-gap-v2",
				"external_evidence_status": externalEvidenceNotRequired,
				"attempts":                 []any{},
			}
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}
func jsonBytesEqual(left, right json.RawMessage) bool {
	var leftValue any
	var rightValue any
	return json.Unmarshal(left, &leftValue) == nil &&
		json.Unmarshal(right, &rightValue) == nil &&
		reflect.DeepEqual(leftValue, rightValue)
}

// treatmentGenerationHarness wires a TreatmentService to fakes so each gate
// test states only what it changes. Generation persists through the
// PlanProposal writer only (TP-002 slice 3 removed the Treatment aggregate).
type treatmentGenerationHarness struct {
	userID     uuid.UUID
	analysis   *model.DiagnosisAnalysisRecord
	body       *fakeTreatmentBodyState
	plans      *fakePlanProposer
	captured   *TreatmentRecommendationRequest
	deployment testTreatmentDeploymentPolicy
	assessed   bool
}

func newTreatmentGenerationHarness(revision int64) *treatmentGenerationHarness {
	userID := uuid.New()
	return &treatmentGenerationHarness{
		userID: userID,
		analysis: &model.DiagnosisAnalysisRecord{
			ID: uuid.New(), UserID: userID, Status: "completed", BodyStateRevision: revision,
			DecisionTrace: datatypes.JSON(`{"evidence":{"body_state_authority":"none","body_state_refs":[],"citations":[]}}`),
			Candidates:    []model.DiagnosisCandidateRecord{{ID: uuid.New(), ConcernKey: "region:neck", Name: "pattern", Confidence: "中"}},
		},
		body:     &fakeTreatmentBodyState{snapshot: &BodyStateReadModel{UserID: userID, CurrentRevision: revision}},
		plans:    &fakePlanProposer{},
		captured: &TreatmentRecommendationRequest{},
		assessed: true,
	}
}

func (h *treatmentGenerationHarness) service(raw json.RawMessage) *TreatmentService {
	diagnosis := &fakeTreatmentDiagnosis{analysis: h.analysis}
	if h.assessed && len(h.analysis.Candidates) > 0 {
		diagnosis.assessments = []model.DiagnosisCandidateAssessment{{CandidateID: h.analysis.Candidates[0].ID, State: "confirmed"}}
	}
	svc := NewTreatmentService(diagnosis, h.body, fakeTreatmentFreshness{state: model.DiagnosisFreshnessFresh}, nil,
		fakeTreatmentReasoner{raw: raw, capture: h.captured}, h.deployment)
	svc.AttachPlanProposer(h.plans)
	return svc
}

func (h *treatmentGenerationHarness) generate(raw json.RawMessage, constraints map[string]any) (*TreatmentPlanGenerationResult, error) {
	return h.service(raw).GeneratePlanProposal(context.Background(), h.userID, TreatmentPlanGenerationInput{
		DiagnosisAnalysisID: h.analysis.ID, UserConstraints: constraints, IdempotencyKey: "gen-" + uuid.NewString(),
	})
}

func (h *treatmentGenerationHarness) provenance(t *testing.T) map[string]any {
	t.Helper()
	if len(h.plans.inputs) != 1 || h.plans.inputs[0].Generation == nil {
		t.Fatalf("expected one generated proposal, got %d", len(h.plans.inputs))
	}
	var provenance map[string]any
	if err := json.Unmarshal(h.plans.inputs[0].Generation.Provenance, &provenance); err != nil {
		t.Fatal(err)
	}
	return provenance
}

func TestTreatmentGenerationSendsSelectedAnalysisEvidenceAndFreezesTheRequest(t *testing.T) {
	h := newTreatmentGenerationHarness(9)
	h.analysis.BodyStateRevision = 7
	h.analysis.DecisionTrace = datatypes.JSON(`{"evidence":{"body_state_authority":"none","body_state_refs":[{"evidence_type":"body_fact","evidence_id":"11111111-1111-4111-8111-111111111111","version":3,"statement_evidence_id":"22222222-2222-4222-8222-222222222222","assertion_status":"asserted"}],"citations":[{"evidence_id":"selected-analysis-only","summary":"audit citation","evidence_type":"body_fact","body_state_authority":"authoritative"}]}}`)
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); err != nil {
		t.Fatalf("GeneratePlanProposal returned error: %v", err)
	}
	captured := *h.captured
	var selectedEvidence []map[string]any
	if err := json.Unmarshal(captured.Evidence, &selectedEvidence); err != nil {
		t.Fatal(err)
	}
	if len(selectedEvidence) != 2 || selectedEvidence[0]["statement_evidence_id"] != "22222222-2222-4222-8222-222222222222" || selectedEvidence[0]["version"] != float64(3) {
		t.Fatalf("selected native source references lost: %s", captured.Evidence)
	}
	if selectedEvidence[1]["evidence_type"] != "external_citation" || selectedEvidence[1]["body_state_authority"] != "none" {
		t.Fatalf("Agent citation lost audit-only classification: %s", captured.Evidence)
	}
	if captured.ConfigurationID != defaultTreatmentConfigurationID {
		t.Fatalf("Treatment request lost config identity: %#v", captured)
	}
	frozen, err := decodeTreatmentReplayInput(h.plans.inputs[0].Generation.ReplayInput)
	if err != nil {
		t.Fatalf("decode frozen Treatment replay input: %v", err)
	}
	if frozen.BodyStateRevision != captured.BodyStateRevision ||
		!jsonBytesEqual(frozen.BodyState, captured.BodyState) ||
		!jsonBytesEqual(frozen.DiagnosisAnalysis, captured.DiagnosisAnalysis) ||
		!jsonBytesEqual(frozen.CandidateAssessments, captured.CandidateAssessments) ||
		!jsonBytesEqual(frozen.Profile, captured.Profile) ||
		!jsonBytesEqual(frozen.UserConstraints, captured.UserConstraints) ||
		!jsonBytesEqual(frozen.Evidence, captured.Evidence) {
		t.Fatalf("frozen replay input differs from actual Agent request: frozen=%#v captured=%#v", frozen, captured)
	}
	provenance := h.provenance(t)
	if provenance["agentConfigurationId"] != defaultTreatmentConfigurationID || provenance["agentConfiguration"] == nil || provenance["executionProvenance"] == nil {
		t.Fatalf("proposal lost Agent provenance: %#v", provenance)
	}
}

func TestTreatmentRejectsForeignOrUnavailableAnalysisEvidenceBeforeAgent(t *testing.T) {
	for _, name := range []string{"foreign owner", "missing audit", "malformed audit", "invalid authority"} {
		t.Run(name, func(t *testing.T) {
			h := newTreatmentGenerationHarness(7)
			h.analysis.DecisionTrace = nil
			switch name {
			case "foreign owner":
				h.analysis.UserID = uuid.New()
			case "malformed audit":
				h.analysis.DecisionTrace = datatypes.JSON(`{"evidence":`)
			case "invalid authority":
				h.analysis.DecisionTrace = datatypes.JSON(`{"evidence":{"body_state_authority":"authoritative"}}`)
			}
			_, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil)
			if err == nil || h.captured.UserID != "" || len(h.plans.inputs) != 0 {
				t.Fatalf("invalid evidence reached Agent: err=%v request=%+v", err, *h.captured)
			}
		})
	}
}

func TestTreatmentGenerationMapsGovernanceRejectionToSafetyBlock(t *testing.T) {
	h := newTreatmentGenerationHarness(9)
	_, err := h.generate(json.RawMessage(`{
		"governance":{"verdict":"rejected","kind":"treatment","reasons":["unsafe current-user claim"],"issues":[]},
		"safety_fallback":"safe fallback"
	}`), nil)
	if !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("governance rejection must surface as Treatment safety block: %v", err)
	}
	if len(h.plans.inputs) != 0 {
		t.Fatal("governance-rejected output must not persist a PlanProposal")
	}
}

func TestTreatmentGenerationRejectsConfigurationMismatchBeforePersistence(t *testing.T) {
	h := newTreatmentGenerationHarness(7)
	_, err := h.generate(json.RawMessage(`{
		"status":"proposed","summary":"plan","goal":"reduce load","duration_weeks":4,
		"agent_configuration":{"id":"treat-config-wrong","role":"treatment"},
		"execution_provenance":{"status":"executed","runtime":"pydantic-ai"},
		"plan_proposal":`+treatmentPlanBlockV1+`
	}`), nil)
	if !errors.Is(err, ErrTreatmentConfigurationMismatch) {
		t.Fatalf("expected configuration mismatch, got %v", err)
	}
	if len(h.plans.inputs) != 0 {
		t.Fatal("configuration mismatch must fail before a PlanProposal is written")
	}
}

func TestTreatmentGenerationRequiresCandidateAssessment(t *testing.T) {
	h := newTreatmentGenerationHarness(7)
	h.assessed = false
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); !errors.Is(err, ErrTreatmentCandidateAssessmentRequired) {
		t.Fatalf("expected candidate assessment gate, got %v", err)
	}
	if h.captured.ConfigurationID != "" {
		t.Fatal("the assessment gate must stop before the Agent")
	}
}

func TestTreatmentMalformedSafetyStateFailsClosedBeforeAgent(t *testing.T) {
	h := newTreatmentGenerationHarness(20)
	h.body.safetyUnavailable = true
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("malformed safety must block generation: %v", err)
	}
	if h.captured.ConfigurationID != "" || len(h.plans.inputs) != 0 {
		t.Fatal("malformed safety must block before the Agent and persistence")
	}
}

func TestRawJSONUsesFallbackForNilContainers(t *testing.T) {
	var constraints map[string]any
	if got := string(rawJSON(constraints, `{}`)); got != `{}` {
		t.Fatalf("nil map must use object fallback, got %s", got)
	}

	var evidence []diagnosisBodyEvidenceRef
	if got := string(rawJSON(evidence, `[]`)); got != `[]` {
		t.Fatalf("nil slice must use array fallback, got %s", got)
	}
}

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
	if trace["diagnosis_analysis_id"] != h.analysis.ID.String() || trace["agent_configuration_id"] != defaultTreatmentConfigurationID {
		t.Fatalf("generation DecisionTrace lost durable identities: %#v", trace)
	}
	facts, ok := trace["facts"].(map[string]any)
	if !ok || facts["current_body_state_revision"] != float64(14) || facts["candidate_assessment_ready"] != true {
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
