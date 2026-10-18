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
	ErrTreatmentDiagnosisNotReady           = errors.New("diagnosis analysis is not eligible for treatment")
	ErrTreatmentSafetyBlocked               = errors.New("active safety state blocks ordinary treatment generation")
	ErrTreatmentAnalysisStale               = errors.New("diagnosis analysis requires review before treatment")
	ErrTreatmentCandidateAssessmentRequired = errors.New("at least one diagnosis candidate must be assessed as confirmed or unsure")
	ErrTreatmentConfigurationMismatch       = errors.New("treatment Agent configuration identity mismatch")
)

type treatmentDiagnosisSource interface {
	GetByID(ctx context.Context, analysisID, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	GetLatest(ctx context.Context, userID uuid.UUID) (*model.DiagnosisAnalysisRecord, error)
	ListAssessments(ctx context.Context, userID, analysisID uuid.UUID) ([]model.DiagnosisCandidateAssessment, error)
	PublicPayload(analysis *model.DiagnosisAnalysisRecord) map[string]any
}

type treatmentBodyStateSource interface {
	GetAgentBodyContext(ctx context.Context, userID uuid.UUID) (*AgentBodyContext, error)
}

type treatmentFreshnessSource interface {
	GetOrEvaluate(ctx context.Context, userID uuid.UUID, analysis *model.DiagnosisAnalysisRecord) (*model.DiagnosisAnalysisFreshness, error)
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
	freshness  treatmentFreshnessSource
	profiles   *ProfileService
	reasoner   treatmentReasoner
	deployment treatmentDeploymentPolicy
	plans      treatmentPlanProposer
}

func NewTreatmentService(
	diagnosis treatmentDiagnosisSource,
	bodyState treatmentBodyStateSource,
	freshness treatmentFreshnessSource,
	profiles *ProfileService,
	reasoner treatmentReasoner,
	deployment treatmentDeploymentPolicy,
) *TreatmentService {
	return &TreatmentService{
		diagnosis: diagnosis, bodyState: bodyState, freshness: freshness,
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
	DiagnosisAnalysisID uuid.UUID
	UserConstraints     map[string]any
	ChangeReason        string
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
	analysis        *model.DiagnosisAnalysisRecord
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

	analysis, err := s.diagnosis.GetByID(ctx, input.DiagnosisAnalysisID, userID)
	if err != nil {
		return nil, err
	}
	if analysis != nil && analysis.UserID != userID {
		return nil, fmt.Errorf("Treatment analysis owner mismatch")
	}
	facts := TreatmentDecisionFacts{CandidateAssessmentReady: true}
	if analysis != nil {
		facts.DiagnosisStatus = analysis.Status
		facts.CandidateCount = len(analysis.Candidates)
	}
	if !treatmentDiagnosisFactsReady(facts) {
		return nil, ErrTreatmentDiagnosisNotReady
	}

	if s.freshness != nil {
		freshness, freshnessErr := s.freshness.GetOrEvaluate(ctx, userID, analysis)
		if freshnessErr != nil {
			return nil, freshnessErr
		}
		if freshness != nil {
			facts.FreshnessState = freshness.State
		}
		if treatmentFreshnessBlocks(facts.FreshnessState) {
			return nil, ErrTreatmentDiagnosisNotReady
		}
	}

	bodyView, err := s.bodyState.GetAgentBodyContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	facts.SafetyEnvelope = bodyView.SafetyEnvelope
	facts.CurrentBodyStateRevision = bodyView.CurrentRevision
	if decision := EvaluateTreatmentDecision(policyRevision, TreatmentDecisionGeneration, facts); decision.Outcome == TreatmentBlock {
		return nil, treatmentDecisionError(decision)
	}

	assessments, err := s.diagnosis.ListAssessments(ctx, userID, analysis.ID)
	if err != nil {
		return nil, err
	}
	facts.CandidateAssessmentReady = treatmentCandidateAssessmentsReady(analysis, assessments)
	generationDecision := EvaluateTreatmentDecision(
		policyRevision,
		TreatmentDecisionGeneration,
		facts,
	)
	if generationDecision.Outcome != TreatmentAllowProposal {
		return nil, treatmentDecisionError(generationDecision)
	}

	profilePayload := json.RawMessage(`{}`)
	if s.profiles != nil {
		if profile, profileErr := s.profiles.GetProfile(ctx, userID); profileErr == nil && profile != nil {
			profilePayload = rawJSON(profile, `{}`)
		}
	}
	evidencePayload, err := treatmentAnalysisEvidence(userID, analysis)
	if err != nil {
		return nil, err
	}
	bodyStatePayload := rawJSON(bodyView, `{}`)
	diagnosisPayload := rawJSON(s.diagnosis.PublicPayload(analysis), `{}`)
	assessmentsPayload := rawJSON(assessments, `[]`)
	constraints := treatmentConstraintsWithEquipment(bodyView, input.UserConstraints)
	constraintsPayload := rawJSON(constraints, `{}`)
	replayInput, err := EncodeTreatmentReplayInput(
		bodyView.CurrentRevision,
		bodyStatePayload,
		diagnosisPayload,
		assessmentsPayload,
		profilePayload,
		constraintsPayload,
		evidencePayload,
		facts,
	)
	if err != nil {
		return nil, fmt.Errorf("freeze Treatment replay input: %w", err)
	}

	raw, err := s.reasoner.RecommendTreatment(ctx, TreatmentRecommendationRequest{
		UserID:               userID.String(),
		ConfigurationID:      configurationID,
		BodyStateRevision:    bodyView.CurrentRevision,
		BodyState:            bodyStatePayload,
		DiagnosisAnalysis:    diagnosisPayload,
		CandidateAssessments: assessmentsPayload,
		Profile:              profilePayload,
		UserConstraints:      constraintsPayload,
		Evidence:             evidencePayload,
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
		route: route, configurationID: configurationID, analysis: analysis, facts: facts,
		decision: generationDecision, bodyRevision: bodyView.CurrentRevision,
		replayInput: replayInput, payload: payload,
	}, nil
}

func treatmentCandidateAssessmentsReady(
	analysis *model.DiagnosisAnalysisRecord,
	assessments []model.DiagnosisCandidateAssessment,
) bool {
	if analysis == nil || len(analysis.Candidates) == 0 {
		return false
	}
	candidateIDs := make(map[uuid.UUID]struct{}, len(analysis.Candidates))
	for _, candidate := range analysis.Candidates {
		candidateIDs[candidate.ID] = struct{}{}
	}
	for _, assessment := range assessments {
		if _, belongs := candidateIDs[assessment.CandidateID]; !belongs {
			continue
		}
		if assessment.State == "confirmed" || assessment.State == "unsure" {
			return true
		}
	}
	return false
}

func treatmentDecisionError(decision TreatmentDecision) error {
	if len(decision.Reasons) == 0 {
		return ErrTreatmentSafetyBlocked
	}
	switch decision.Reasons[0] {
	case "diagnosis_not_ready":
		return ErrTreatmentDiagnosisNotReady
	case "diagnosis_not_fresh":
		return ErrTreatmentAnalysisStale
	case "active_body_state_safety_concern", "malformed_or_unknown_safety_state",
		"unsupported_decision_policy_revision", "unsupported_decision_phase":
		return fmt.Errorf("%w: %s", ErrTreatmentSafetyBlocked, decision.Reasons[0])
	case "candidate_assessment_required":
		return ErrTreatmentCandidateAssessmentRequired
	default:
		return fmt.Errorf("%w: %s", ErrTreatmentSafetyBlocked, decision.Reasons[0])
	}
}

func buildTreatmentDecisionTrace(
	decision TreatmentDecision,
	facts TreatmentDecisionFacts,
	diagnosisAnalysisID uuid.UUID,
	agentConfigurationID string,
) datatypes.JSON {
	trace, _ := json.Marshal(map[string]any{
		"trace_revision":         TreatmentDecisionTraceV1,
		"policy_revision":        decision.PolicyRevision,
		"phase":                  decision.Phase,
		"outcome":                decision.Outcome,
		"reasons":                decision.Reasons,
		"diagnosis_analysis_id":  diagnosisAnalysisID,
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
