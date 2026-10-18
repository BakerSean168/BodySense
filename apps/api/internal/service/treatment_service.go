package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrTreatmentDiagnosisNotReady     = errors.New("adopt at least one Diagnosis condition before plan generation")
	ErrTreatmentSafetyBlocked         = errors.New("active safety state blocks ordinary treatment generation")
	ErrTreatmentDiagnosisStale        = errors.New("the Diagnosis changed since the presented revision")
	ErrTreatmentConfigurationMismatch = errors.New("treatment Agent configuration identity mismatch")
)

// treatmentDiagnosisSource is the native Diagnosis projection: one consistent
// read of the owner's committed head and Conditions.
type treatmentDiagnosisSource interface {
	Read(ctx context.Context, owner uuid.UUID) (*NativeDiagnosisView, error)
}

// treatmentDiagnosisFreshness is the derived Condition freshness. A stale
// adopted Condition rests on a source that no longer qualifies, so a plan
// generated from it could never be admitted.
type treatmentDiagnosisFreshness interface {
	EvaluateConditions(context.Context, uuid.UUID, *NativeDiagnosisView) (map[uuid.UUID]DiagnosisFreshness, error)
}

type treatmentBodyStateSource interface {
	GetAgentBodyContext(ctx context.Context, userID uuid.UUID) (*AgentBodyContext, error)
}

type treatmentReasoner interface {
	RecommendTreatment(ctx context.Context, req TreatmentRecommendationRequest) (json.RawMessage, error)
}

type treatmentDeploymentPolicy interface {
	SelectTreatmentRoute(subjectID string) TreatmentRouteSelection
}

// TreatmentService runs the gated Treatment Agent. Since TP-002 slice 3 it
// keeps no Treatment aggregate: its only product is a pending agent
// PlanProposal(create) (see GeneratePlanProposal); acceptance stays the user's
// explicit TrainingPlan decision.
type TreatmentService struct {
	diagnosis  treatmentDiagnosisSource
	bodyState  treatmentBodyStateSource
	profiles   *ProfileService
	reasoner   treatmentReasoner
	deployment treatmentDeploymentPolicy
	plans      treatmentPlanProposer
	freshness  treatmentDiagnosisFreshness
}

// AttachDiagnosisFreshness makes generation refuse (DIAGNOSIS_STALE) while an
// adopted Condition is stale. review_recommended does not block: the user may
// defer a review prompt (diagnosis.md §3.6).
func (s *TreatmentService) AttachDiagnosisFreshness(freshness treatmentDiagnosisFreshness) {
	s.freshness = freshness
}

func NewTreatmentService(
	diagnosis treatmentDiagnosisSource,
	bodyState treatmentBodyStateSource,
	profiles *ProfileService,
	reasoner treatmentReasoner,
	deployment treatmentDeploymentPolicy,
) *TreatmentService {
	return &TreatmentService{
		diagnosis: diagnosis, bodyState: bodyState,
		profiles: profiles, reasoner: reasoner, deployment: deployment,
	}
}

// treatmentConstraintsWithEquipment returns a copy of the caller constraints
// with user_constraints.available_equipment taken from the pinned agent view's
// usable EquipmentAssets (PC-D2), unless the caller stated equipment itself.
// Equipment is a user-stated resource; it never edits an accepted plan.
func treatmentConstraintsWithEquipment(view *AgentBodyContext, constraints map[string]any) map[string]any {
	if view == nil || len(view.Equipment) == 0 {
		return constraints
	}
	if _, set := constraints["available_equipment"]; set {
		return constraints
	}
	equipment := make([]map[string]any, 0, len(view.Equipment))
	for _, asset := range view.Equipment {
		item := map[string]any{"name": asset.DisplayName, "access_kind": asset.AccessKind}
		if asset.Notes != nil {
			item["notes"] = *asset.Notes
		}
		equipment = append(equipment, item)
	}
	merged := make(map[string]any, len(constraints)+1)
	for key, value := range constraints {
		merged[key] = value
	}
	merged["available_equipment"] = equipment
	return merged
}

type TreatmentProposalInput struct {
	// ExpectedDiagnosisRevision, when set, must equal the current Diagnosis
	// revision; the user generated from the Conditions they were shown.
	ExpectedDiagnosisRevision *int64
	UserConstraints           map[string]any
	ChangeReason              string
}

// treatmentAgentPayload is the part of the governed Treatment Agent output Go
// reads. The legacy Treatment fields the Agent still emits are ignored.
type treatmentAgentPayload struct {
	Status              string          `json:"status"`
	EvidenceIDs         []string        `json:"evidence_ids"`
	Governance          json.RawMessage `json:"governance"`
	AgentConfiguration  json.RawMessage `json:"agent_configuration"`
	ExecutionProvenance json.RawMessage `json:"execution_provenance"`
	EvidenceAcquisition json.RawMessage `json:"evidence_acquisition"`
	PlanProposal        json.RawMessage `json:"plan_proposal"`
}

// treatmentGeneration is one gated, frozen, governed Treatment Agent run,
// before anything is persisted.
type treatmentGeneration struct {
	route           TreatmentRouteSelection
	configurationID string
	diagnosis       *NativeDiagnosisView
	facts           TreatmentDecisionFacts
	decision        TreatmentDecision
	bodyRevision    int64
	replayInput     json.RawMessage
	payload         *treatmentAgentPayload
}

// runGeneration applies the Go decision policy, freezes the Agent input, calls
// the Agent and checks its identity, governance and evidence trace. It writes
// nothing; callers decide what the governed output may become.
func (s *TreatmentService) runGeneration(
	ctx context.Context,
	userID uuid.UUID,
	input TreatmentProposalInput,
) (*treatmentGeneration, error) {
	if s.deployment == nil {
		return nil, errors.New("treatment deployment policy is not configured")
	}
	route := s.deployment.SelectTreatmentRoute(userID.String())
	configurationID := route.ServedConfigurationID
	policyRevision := route.ServedDecisionPolicyRevision
	if configurationID == "" || policyRevision == "" {
		return nil, errors.New("treatment deployment policy returned an invalid route")
	}

	if s.diagnosis == nil {
		return nil, ErrTreatmentDiagnosisNotReady
	}
	current, err := s.diagnosis.Read(ctx, userID)
	if err != nil {
		return nil, err
	}
	if current == nil || current.UserID != userID {
		return nil, fmt.Errorf("Treatment Diagnosis owner mismatch")
	}
	if input.ExpectedDiagnosisRevision != nil && *input.ExpectedDiagnosisRevision != current.CurrentRevision {
		return nil, ErrTreatmentDiagnosisStale
	}
	diagnosis := adoptedTreatmentDiagnosis(current)
	facts := TreatmentDecisionFacts{
		DiagnosisRevision:     diagnosis.CurrentRevision,
		AdoptedConditionCount: len(diagnosis.ConditionEntries),
	}
	if !treatmentDiagnosisFactsReady(facts) {
		return nil, ErrTreatmentDiagnosisNotReady
	}
	if err := s.requireFreshDiagnosis(ctx, userID, diagnosis); err != nil {
		return nil, err
	}

	bodyView, err := s.bodyState.GetAgentBodyContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	facts.SafetyEnvelope = bodyView.SafetyEnvelope
	facts.CurrentBodyStateRevision = bodyView.CurrentRevision
	generationDecision := EvaluateTreatmentDecision(policyRevision, TreatmentDecisionGeneration, facts)
	if generationDecision.Outcome != TreatmentAllowProposal {
		return nil, treatmentDecisionError(generationDecision)
	}

	profilePayload := json.RawMessage(`{}`)
	if s.profiles != nil {
		if profile, profileErr := s.profiles.GetProfile(ctx, userID); profileErr == nil && profile != nil {
			profilePayload = rawJSON(profile.AgentContext(), `{}`)
		}
	}
	evidencePayload, err := treatmentDiagnosisEvidence(userID, diagnosis)
	if err != nil {
		return nil, err
	}
	bodyStatePayload := rawJSON(bodyView, `{}`)
	diagnosisPayload := rawJSON(diagnosis, `{}`)
	constraints := treatmentConstraintsWithEquipment(bodyView, input.UserConstraints)
	constraintsPayload := rawJSON(constraints, `{}`)
	replayInput, err := EncodeTreatmentReplayInput(
		bodyView.CurrentRevision,
		bodyStatePayload,
		diagnosisPayload,
		profilePayload,
		constraintsPayload,
		evidencePayload,
		facts,
	)
	if err != nil {
		return nil, fmt.Errorf("freeze Treatment replay input: %w", err)
	}

	raw, err := s.reasoner.RecommendTreatment(ctx, TreatmentRecommendationRequest{
		UserID:            userID.String(),
		ConfigurationID:   configurationID,
		BodyStateRevision: bodyView.CurrentRevision,
		BodyState:         bodyStatePayload,
		Diagnosis:         diagnosisPayload,
		Profile:           profilePayload,
		UserConstraints:   constraintsPayload,
		Evidence:          evidencePayload,
	})
	if err != nil {
		return nil, err
	}
	payload, err := normalizeTreatmentAgentPayload(raw)
	if err != nil {
		return nil, err
	}
	if err := validateTreatmentAgentIdentity(payload, configurationID); err != nil {
		return nil, err
	}
	if treatmentGovernanceRejected(payload.Governance) {
		return nil, fmt.Errorf("%w: agent output rejected by governance", ErrTreatmentSafetyBlocked)
	}
	if _, err := validateEvidenceAvailabilityForConfiguration(configurationID, payload.EvidenceAcquisition); err != nil {
		return nil, err
	}
	return &treatmentGeneration{
		route: route, configurationID: configurationID, diagnosis: diagnosis, facts: facts,
		decision: generationDecision, bodyRevision: bodyView.CurrentRevision,
		replayInput: replayInput, payload: payload,
	}, nil
}

// requireFreshDiagnosis refuses generation while any adopted Condition is
// stale. It reads only; the Conditions stay as the user adopted them.
func (s *TreatmentService) requireFreshDiagnosis(ctx context.Context, owner uuid.UUID, diagnosis *NativeDiagnosisView) error {
	if s.freshness == nil {
		return nil
	}
	report, err := s.freshness.EvaluateConditions(ctx, owner, diagnosis)
	if err != nil {
		return err
	}
	for _, condition := range diagnosis.ConditionEntries {
		freshness, ok := report[condition.ConditionID]
		if !ok {
			return fmt.Errorf("Treatment Diagnosis freshness missing")
		}
		if freshness.Stale() {
			return fmt.Errorf("%w: an adopted Condition rests on a source that changed", ErrTreatmentDiagnosisStale)
		}
	}
	return nil
}

// adoptedTreatmentDiagnosis is the Treatment Agent's Diagnosis input: the
// adopted Conditions of the current revision only. Inactive Conditions stay in
// the Diagnosis window but never shape a plan.
func adoptedTreatmentDiagnosis(view *NativeDiagnosisView) *NativeDiagnosisView {
	adopted := &NativeDiagnosisView{
		UserID: view.UserID, CurrentRevision: view.CurrentRevision, UpdatedAt: view.UpdatedAt,
		ConditionEntries: make([]NativeDiagnosisCondition, 0, len(view.ConditionEntries)),
	}
	for _, condition := range view.ConditionEntries {
		if condition.Disposition == model.ConditionAdopted {
			adopted.ConditionEntries = append(adopted.ConditionEntries, condition)
		}
	}
	return adopted
}

// treatmentDiagnosisEvidence passes only the typed evidence links of the
// adopted Conditions, never an owner-wide evidence pool. Current BodyState and
// Safety remain separate authoritative inputs.
func treatmentDiagnosisEvidence(owner uuid.UUID, diagnosis *NativeDiagnosisView) (json.RawMessage, error) {
	if diagnosis == nil || diagnosis.UserID != owner {
		return nil, fmt.Errorf("Treatment Diagnosis owner mismatch")
	}
	items := make([]map[string]any, 0)
	for _, condition := range diagnosis.ConditionEntries {
		for _, link := range condition.EvidenceLinks {
			items = append(items, map[string]any{
				"evidence_id":          link.EvidenceID,
				"evidence_type":        link.EvidenceType,
				"evidence_version":     link.EvidenceVersion,
				"role":                 link.Role,
				"condition_id":         condition.ConditionID,
				"body_state_authority": "none",
			})
		}
	}
	return json.Marshal(items)
}

func treatmentDecisionError(decision TreatmentDecision) error {
	if len(decision.Reasons) == 0 {
		return ErrTreatmentSafetyBlocked
	}
	switch decision.Reasons[0] {
	case "adopted_condition_required":
		return ErrTreatmentDiagnosisNotReady
	default:
		return fmt.Errorf("%w: %s", ErrTreatmentSafetyBlocked, decision.Reasons[0])
	}
}

func buildTreatmentDecisionTrace(
	decision TreatmentDecision,
	facts TreatmentDecisionFacts,
	diagnosisRevision int64,
	agentConfigurationID string,
) datatypes.JSON {
	trace, _ := json.Marshal(map[string]any{
		"trace_revision":         TreatmentDecisionTraceV1,
		"policy_revision":        decision.PolicyRevision,
		"phase":                  decision.Phase,
		"outcome":                decision.Outcome,
		"reasons":                decision.Reasons,
		"diagnosis_revision":     diagnosisRevision,
		"agent_configuration_id": agentConfigurationID,
		"facts":                  facts,
	})
	return datatypes.JSON(trace)
}

func normalizeTreatmentAgentPayload(raw json.RawMessage) (*treatmentAgentPayload, error) {
	var payload treatmentAgentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode treatment recommendation: %w", err)
	}
	if treatmentGovernanceRejected(payload.Governance) {
		return &payload, nil
	}
	if payload.Status == "" {
		return nil, errors.New("treatment status is required")
	}
	if payload.Status != "proposed" {
		return nil, fmt.Errorf("unexpected treatment status %q", payload.Status)
	}
	return &payload, nil
}

func treatmentGovernanceRejected(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var governance map[string]any
	if json.Unmarshal(raw, &governance) != nil {
		return false
	}
	verdict, _ := governance["verdict"].(string)
	return verdict == "rejected"
}

func validateTreatmentAgentIdentity(payload *treatmentAgentPayload, expectedConfigurationID string) error {
	if payload == nil || len(payload.AgentConfiguration) == 0 {
		return ErrTreatmentConfigurationMismatch
	}
	var configuration struct {
		ID                     string `json:"id"`
		Role                   string `json:"role"`
		DecisionPolicyRevision string `json:"decision_policy_revision"`
	}
	if err := json.Unmarshal(payload.AgentConfiguration, &configuration); err != nil {
		return ErrTreatmentConfigurationMismatch
	}
	registration, ok := knownTreatmentConfigurations[expectedConfigurationID]
	if !ok || configuration.ID != expectedConfigurationID || configuration.Role != "treatment" ||
		configuration.DecisionPolicyRevision != registration.DecisionPolicyRevision {
		return ErrTreatmentConfigurationMismatch
	}
	var execution struct {
		Status       string `json:"status"`
		Runtime      string `json:"runtime"`
		LogicalModel string `json:"logical_model"`
	}
	if len(payload.ExecutionProvenance) == 0 ||
		json.Unmarshal(payload.ExecutionProvenance, &execution) != nil ||
		execution.Status != "executed" || execution.Runtime != "pydantic-ai" ||
		execution.LogicalModel != registration.LogicalModel {
		return ErrTreatmentConfigurationMismatch
	}
	return nil
}

func rawJSON(value any, fallback string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 || string(encoded) == "null" {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(encoded)
}

func normalizeRaw(value json.RawMessage, fallback string) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage(fallback)
	}
	return value
}
