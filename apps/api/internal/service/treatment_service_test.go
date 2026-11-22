package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

type fakeTreatmentDiagnosis struct {
	view *NativeDiagnosisView
	err  error
}

func (f *fakeTreatmentDiagnosis) Read(context.Context, uuid.UUID) (*NativeDiagnosisView, error) {
	return f.view, f.err
}

// testCompleteSafetyEnvelope is a pinned native envelope whose coverage is
// complete: one screened source and no blockers.
func testCompleteSafetyEnvelope(revision int64) *SafetyEnvelopeV2 {
	envelope := testNativeSafetyEnvelope(revision)
	envelope.Coverage.CoveredSourceRefs = []string{"body-state:safety-screen:test"}
	envelope.Coverage.IncompleteSourceRefs = []string{}
	envelope.Coverage.Complete = true
	return envelope
}

func testAdoptedCondition(owner uuid.UUID, revision int64, disposition model.ConditionDisposition) NativeDiagnosisCondition {
	run := uuid.New()
	return NativeDiagnosisCondition{
		ConditionID: uuid.New(), MedicalName: "neck strain", Disposition: disposition, CommittedRevision: revision,
		SourceClaimRefs: []model.DiagnosisSourceClaimRef{}, InformationGaps: []string{},
		EvidenceLinks: []NativeDiagnosisEvidenceLink{{
			EvidenceType: model.DiagnosisEvidenceType("body_fact"), EvidenceID: uuid.New(), EvidenceVersion: "3",
			Role: model.DiagnosisEvidenceRole("supports"), Rationale: "reported", AnalyzedByRunID: run,
		}},
	}
}

type fakeTreatmentBodyState struct {
	snapshot          *BodyStateReadModel
	equipment         []AgentEquipmentItem
	viewOwner         uuid.UUID
	safetyUnavailable bool
	safetyBlocked     bool
	safetyIncomplete  bool
}

func (f *fakeTreatmentBodyState) GetAgentBodyContext(_ context.Context, owner uuid.UUID) (*AgentBodyContext, error) {
	f.viewOwner = owner
	envelope := testCompleteSafetyEnvelope(f.snapshot.CurrentRevision)
	if f.safetyIncomplete {
		envelope = testNativeSafetyEnvelope(f.snapshot.CurrentRevision)
	}
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
	diagnosis  *NativeDiagnosisView
	body       *fakeTreatmentBodyState
	plans      *fakePlanProposer
	captured   *TreatmentRecommendationRequest
	deployment testTreatmentDeploymentPolicy
	expected   *int64
}

func newTreatmentGenerationHarness(revision int64) *treatmentGenerationHarness {
	userID := uuid.New()
	return &treatmentGenerationHarness{
		userID: userID,
		diagnosis: &NativeDiagnosisView{
			UserID: userID, CurrentRevision: 2,
			ConditionEntries: []NativeDiagnosisCondition{testAdoptedCondition(userID, 2, model.ConditionAdopted)},
		},
		body:     &fakeTreatmentBodyState{snapshot: &BodyStateReadModel{UserID: userID, CurrentRevision: revision}},
		plans:    &fakePlanProposer{},
		captured: &TreatmentRecommendationRequest{},
	}
}

func (h *treatmentGenerationHarness) service(raw json.RawMessage) *TreatmentService {
	svc := NewTreatmentService(&fakeTreatmentDiagnosis{view: h.diagnosis}, h.body, nil,
		fakeTreatmentReasoner{raw: raw, capture: h.captured}, h.deployment)
	svc.AttachPlanProposer(h.plans)
	return svc
}

func (h *treatmentGenerationHarness) generate(raw json.RawMessage, constraints map[string]any) (*TreatmentPlanGenerationResult, error) {
	return h.service(raw).GeneratePlanProposal(context.Background(), h.userID, TreatmentPlanGenerationInput{
		ExpectedDiagnosisRevision: h.expected, UserConstraints: constraints, IdempotencyKey: "gen-" + uuid.NewString(),
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

func TestTreatmentGenerationSendsAdoptedConditionsAndFreezesTheRequest(t *testing.T) {
	h := newTreatmentGenerationHarness(9)
	adopted := h.diagnosis.ConditionEntries[0]
	inactive := testAdoptedCondition(h.userID, 2, model.ConditionInactive)
	h.diagnosis.ConditionEntries = append(h.diagnosis.ConditionEntries, inactive)
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); err != nil {
		t.Fatalf("GeneratePlanProposal returned error: %v", err)
	}
	captured := *h.captured
	var sentDiagnosis NativeDiagnosisView
	if err := json.Unmarshal(captured.Diagnosis, &sentDiagnosis); err != nil {
		t.Fatal(err)
	}
	if sentDiagnosis.CurrentRevision != 2 || len(sentDiagnosis.ConditionEntries) != 1 ||
		sentDiagnosis.ConditionEntries[0].ConditionID != adopted.ConditionID {
		t.Fatalf("Treatment must receive only the adopted Conditions of the current revision: %s", captured.Diagnosis)
	}
	var evidence []map[string]any
	if err := json.Unmarshal(captured.Evidence, &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0]["evidence_id"] != adopted.EvidenceLinks[0].EvidenceID.String() ||
		evidence[0]["condition_id"] != adopted.ConditionID.String() || evidence[0]["body_state_authority"] != "none" {
		t.Fatalf("adopted Condition evidence lost or inactive evidence leaked: %s", captured.Evidence)
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
		!jsonBytesEqual(frozen.Diagnosis, captured.Diagnosis) ||
		!jsonBytesEqual(frozen.Profile, captured.Profile) ||
		!jsonBytesEqual(frozen.UserConstraints, captured.UserConstraints) ||
		!jsonBytesEqual(frozen.Evidence, captured.Evidence) {
		t.Fatalf("frozen replay input differs from actual Agent request: frozen=%#v captured=%#v", frozen, captured)
	}
	provenance := h.provenance(t)
	if provenance["agentConfigurationId"] != defaultTreatmentConfigurationID || provenance["agentConfiguration"] == nil || provenance["executionProvenance"] == nil {
		t.Fatalf("proposal lost Agent provenance: %#v", provenance)
	}
	ids, _ := provenance["sourceConditionIds"].([]any)
	if provenance["sourceDiagnosisRevision"] != float64(2) || len(ids) != 1 || ids[0] != adopted.ConditionID.String() {
		t.Fatalf("proposal lost its Diagnosis source: %#v", provenance)
	}
}

func TestTreatmentGenerationRequiresAnAdoptedCondition(t *testing.T) {
	for name, mutate := range map[string]func(h *treatmentGenerationHarness){
		"no diagnosis": func(h *treatmentGenerationHarness) { h.diagnosis = &NativeDiagnosisView{UserID: h.userID} },
		"only inactive": func(h *treatmentGenerationHarness) {
			h.diagnosis.ConditionEntries[0].Disposition = model.ConditionInactive
		},
		"empty revision": func(h *treatmentGenerationHarness) { h.diagnosis.ConditionEntries = nil },
	} {
		t.Run(name, func(t *testing.T) {
			h := newTreatmentGenerationHarness(7)
			mutate(h)
			if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); !errors.Is(err, ErrTreatmentDiagnosisNotReady) {
				t.Fatalf("expected adopted-Condition gate, got %v", err)
			}
			if h.captured.ConfigurationID != "" || len(h.plans.inputs) != 0 {
				t.Fatal("the Diagnosis gate must stop before the Agent")
			}
		})
	}
}

func TestTreatmentGenerationRejectsAStalePresentedRevision(t *testing.T) {
	h := newTreatmentGenerationHarness(7)
	presented := int64(1)
	h.expected = &presented
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); !errors.Is(err, ErrTreatmentDiagnosisStale) {
		t.Fatalf("expected stale Diagnosis revision, got %v", err)
	}
	if h.captured.ConfigurationID != "" {
		t.Fatal("a stale revision must stop before the Agent")
	}
	current := int64(2)
	h.expected = &current
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); err != nil {
		t.Fatalf("the current revision must generate: %v", err)
	}
}

func TestTreatmentRejectsForeignDiagnosisBeforeAgent(t *testing.T) {
	h := newTreatmentGenerationHarness(7)
	h.diagnosis.UserID = uuid.New()
	_, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil)
	if err == nil || h.captured.UserID != "" || len(h.plans.inputs) != 0 {
		t.Fatalf("foreign Diagnosis reached Agent: err=%v request=%+v", err, *h.captured)
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

func TestTreatmentIncompleteSafetyCoverageBlocksBeforeAgent(t *testing.T) {
	h := newTreatmentGenerationHarness(7)
	h.body.safetyIncomplete = true
	if _, err := h.generate(treatmentAgentRawWithPlan(treatmentPlanBlockV1), nil); !errors.Is(err, ErrTreatmentSafetyBlocked) {
		t.Fatalf("incomplete Safety coverage must block generation: %v", err)
	}
	if h.captured.ConfigurationID != "" || len(h.plans.inputs) != 0 {
		t.Fatal("incomplete Safety must block before the Agent and persistence")
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

	var evidence []NativeDiagnosisEvidenceLink
	if got := string(rawJSON(evidence, `[]`)); got != `[]` {
		t.Fatalf("nil slice must use array fallback, got %s", got)
	}
}
