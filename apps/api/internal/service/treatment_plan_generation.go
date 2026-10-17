package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Treatment generation → PlanProposal(create) (TP-002 slice 2b). The Treatment
// Agent's governed output carries a strict training-plan-agent-proposal-v1
// block. Go assigns every identity and persists it only as a pending
// PlanProposal; acceptance stays the user's explicit decision.
const (
	TrainingPlanAgentProposalSchemaV1      = "training-plan-agent-proposal-v1"
	TrainingPlanGenerationProvenanceV1     = "training-plan-generation-provenance-v1"
	treatmentPlanGenerationSourcePrefix    = "treatment_agent:"
	treatmentPlanGenerationOutcomeProposed = "proposed"
	treatmentPlanGenerationOutcomeNone     = "unavailable"
)

var (
	// ErrTreatmentPlanGenerationUnavailable means no PlanProposal writer is wired.
	ErrTreatmentPlanGenerationUnavailable = errors.New("training plan generation is not configured")
	// ErrTreatmentPlanUnavailable means the governed output carried no plan.
	ErrTreatmentPlanUnavailable = errors.New("treatment Agent produced no training plan")
	// ErrTreatmentPlanOutputInvalid means the plan block failed strict validation.
	ErrTreatmentPlanOutputInvalid = errors.New("treatment Agent training plan output is invalid")
)

type treatmentPlanProposer interface {
	Propose(ctx context.Context, owner uuid.UUID, input PlanProposalInput) (*model.PlanProposal, error)
	ProposalByIdempotencyKey(ctx context.Context, owner uuid.UUID, key string) (*model.PlanProposal, error)
}

// AttachPlanProposer wires the independent TrainingPlan service that persists
// generated PlanProposals.
func (s *TreatmentService) AttachPlanProposer(plans treatmentPlanProposer) {
	if s != nil {
		s.plans = plans
	}
}

// TreatmentPlanGenerationInput is the user's generation command. A nil
// DiagnosisAnalysisID selects the latest analysis.
type TreatmentPlanGenerationInput struct {
	DiagnosisAnalysisID uuid.UUID
	UserConstraints     map[string]any
	ChangeReason        string
	IdempotencyKey      string
}

type TreatmentPlanGenerationResult struct {
	Proposal *model.PlanProposal
	// Replayed is true when the key already held this command's proposal and
	// the Agent was not called again.
	Replayed bool
}

type planAgentExerciseV1 struct {
	ExerciseID      string           `json:"exerciseId"`
	NameSnapshot    string           `json:"nameSnapshot"`
	Sets            int              `json:"sets"`
	Target          model.PlanTarget `json:"target"`
	RestSeconds     *float64         `json:"restSeconds,omitempty"`
	Side            string           `json:"side,omitempty"`
	LoadOrIntensity string           `json:"loadOrIntensity,omitempty"`
	Instructions    string           `json:"instructions,omitempty"`
}
type planAgentStageV1 struct {
	Phase     string                `json:"phase"`
	Exercises []planAgentExerciseV1 `json:"exercises"`
}
type planAgentSessionV1 struct {
	Title  string             `json:"title"`
	Stages []planAgentStageV1 `json:"stages"`
}
type planAgentContentV1 struct {
	Sessions    []planAgentSessionV1 `json:"sessions"`
	Frequency   model.PlanFrequency  `json:"frequency"`
	Constraints []string             `json:"constraints"`
}
type planAgentProposalV1 struct {
	SchemaRevision string              `json:"schemaRevision"`
	Outcome        string              `json:"outcome"`
	Reason         string              `json:"reason,omitempty"`
	Title          string              `json:"title"`
	Goal           string              `json:"goal"`
	Summary        string              `json:"summary"`
	Content        *planAgentContentV1 `json:"content"`
	EvidenceIDs    []string            `json:"evidenceIds"`
}

// GeneratePlanProposal runs the gated Treatment Agent and persists its plan as
// one pending agent PlanProposal(create). It never creates, changes or
// activates a TrainingPlan and never writes a Treatment revision. A retried
// key with the same command replays the stored proposal without an Agent call.
func (s *TreatmentService) GeneratePlanProposal(
	ctx context.Context,
	userID uuid.UUID,
	input TreatmentPlanGenerationInput,
) (*TreatmentPlanGenerationResult, error) {
	if s == nil || s.plans == nil {
		return nil, ErrTreatmentPlanGenerationUnavailable
	}
	if !validPlanKey(input.IdempotencyKey) {
		return nil, ErrPlanInvalid
	}
	changeReason := strings.TrimSpace(input.ChangeReason)
	constraints := input.UserConstraints
	if constraints == nil {
		constraints = map[string]any{}
	}
	command := planDigest(struct {
		Owner               uuid.UUID
		DiagnosisAnalysisID uuid.UUID
		UserConstraints     map[string]any
		ChangeReason        string
	}{userID, input.DiagnosisAnalysisID, constraints, changeReason})

	existing, err := s.plans.ProposalByIdempotencyKey(ctx, userID, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.UserID != userID || existing.GenerationProvenance == nil ||
			existing.RequestFingerprint != planGenerationRequestFingerprint(command) {
			return nil, ErrPlanConflict
		}
		return &TreatmentPlanGenerationResult{Proposal: existing, Replayed: true}, nil
	}

	analysisID := input.DiagnosisAnalysisID
	if analysisID == uuid.Nil {
		latest, latestErr := s.diagnosis.GetLatest(ctx, userID)
		if latestErr != nil {
			return nil, latestErr
		}
		if latest == nil {
			return nil, ErrTreatmentDiagnosisNotReady
		}
		analysisID = latest.ID
	}
	gen, err := s.runGeneration(ctx, userID, TreatmentProposalInput{
		DiagnosisAnalysisID: analysisID,
		UserConstraints:     constraints,
		ChangeReason:        changeReason,
	})
	if err != nil {
		return nil, err
	}
	block, draft, err := decodeTreatmentPlanProposal(gen.payload.PlanProposal, gen.payload.EvidenceIDs)
	if err != nil {
		return nil, err
	}
	provenance, err := json.Marshal(map[string]any{
		"schemaRevision":              TrainingPlanGenerationProvenanceV1,
		"agentProposalSchemaRevision": TrainingPlanAgentProposalSchemaV1,
		"commandFingerprint":          command,
		"sourceDiagnosisAnalysisId":   gen.analysis.ID,
		"sourceBodyStateRevision":     gen.bodyRevision,
		"changeReason":                changeReason,
		"summary":                     block.Summary,
		"evidenceIds":                 block.EvidenceIDs,
		"agentConfigurationId":        gen.configurationID,
		"agentConfiguration":          normalizeRaw(gen.payload.AgentConfiguration, `{}`),
		"executionProvenance":         normalizeRaw(gen.payload.ExecutionProvenance, `{}`),
		"governance":                  normalizeRaw(gen.payload.Governance, `{}`),
		"evidenceAcquisition":         normalizeRaw(gen.payload.EvidenceAcquisition, `{}`),
		"generationDecisionTrace":     json.RawMessage(buildTreatmentDecisionTrace(gen.decision, gen.facts, gen.analysis.ID, gen.configurationID)),
		"rolloutProvenance":           gen.route,
		"replayInputDigest":           sha256Hex(gen.replayInput),
		"agentOutputDigest":           sha256Hex(gen.payload.PlanProposal),
	})
	if err != nil {
		return nil, fmt.Errorf("encode plan generation provenance: %w", err)
	}
	proposal, err := s.plans.Propose(ctx, userID, PlanProposalInput{
		Draft:          draft,
		InitiatedBy:    trainingplan.InitiatorAgent,
		SourceID:       treatmentPlanGenerationSourcePrefix + gen.configurationID,
		IdempotencyKey: input.IdempotencyKey,
		Generation: &PlanGeneration{
			CommandFingerprint:      command,
			FrozenBodyStateRevision: gen.bodyRevision,
			Provenance:              provenance,
			ReplayInput:             gen.replayInput,
		},
	})
	if err != nil {
		if errors.Is(err, ErrPlanInvalid) {
			return nil, fmt.Errorf("%w: %w", ErrTreatmentPlanOutputInvalid, err)
		}
		return nil, err
	}
	// A concurrent retry of the same command may have stored its proposal first;
	// Propose then returns that row instead of the one built here.
	inserted := len(proposal.GenerationProvenance) > 0 && &proposal.GenerationProvenance[0] == &provenance[0]
	return &TreatmentPlanGenerationResult{Proposal: proposal, Replayed: !inserted}, nil
}

// decodeTreatmentPlanProposal strictly decodes the Agent block and materializes
// a TrainingPlanDraft with Go-assigned template and entry identities. It never
// repairs content: any unknown field (including an Agent-chosen ID), a missing
// plan, foreign evidence or an invalid draft rejects the whole output.
func decodeTreatmentPlanProposal(raw json.RawMessage, agentEvidenceIDs []string) (*planAgentProposalV1, model.TrainingPlanDraft, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, model.TrainingPlanDraft{}, ErrTreatmentPlanUnavailable
	}
	var block planAgentProposalV1
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&block); err != nil {
		return nil, model.TrainingPlanDraft{}, fmt.Errorf("%w: %v", ErrTreatmentPlanOutputInvalid, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, model.TrainingPlanDraft{}, fmt.Errorf("%w: trailing data", ErrTreatmentPlanOutputInvalid)
	}
	if block.SchemaRevision != TrainingPlanAgentProposalSchemaV1 {
		return nil, model.TrainingPlanDraft{}, fmt.Errorf("%w: schema revision %q", ErrTreatmentPlanOutputInvalid, block.SchemaRevision)
	}
	switch block.Outcome {
	case treatmentPlanGenerationOutcomeNone:
		return nil, model.TrainingPlanDraft{}, ErrTreatmentPlanUnavailable
	case treatmentPlanGenerationOutcomeProposed:
	default:
		return nil, model.TrainingPlanDraft{}, fmt.Errorf("%w: outcome %q", ErrTreatmentPlanOutputInvalid, block.Outcome)
	}
	if block.Reason != "" || block.Content == nil {
		return nil, model.TrainingPlanDraft{}, fmt.Errorf("%w: proposed block shape", ErrTreatmentPlanOutputInvalid)
	}
	allowed := make(map[string]bool, len(agentEvidenceIDs))
	for _, id := range agentEvidenceIDs {
		allowed[id] = true
	}
	for _, id := range block.EvidenceIDs {
		if !allowed[id] {
			return nil, model.TrainingPlanDraft{}, fmt.Errorf("%w: evidence %q was not cited by the governed output", ErrTreatmentPlanOutputInvalid, id)
		}
	}
	if block.EvidenceIDs == nil {
		block.EvidenceIDs = []string{}
	}
	constraints := block.Content.Constraints
	if constraints == nil {
		constraints = []string{}
	}
	draft := model.TrainingPlanDraft{
		Title: block.Title, Goal: block.Goal,
		RelatedConditionIDs: []uuid.UUID{}, RelatedBodyRefs: []model.PlanBodyRef{},
		Content: model.TrainingPlanContent{
			Sessions:    make([]model.PlanSessionTemplate, 0, len(block.Content.Sessions)),
			Frequency:   block.Content.Frequency,
			Schedule:    model.PlanSchedule{Mode: "frequency_only"},
			Constraints: constraints,
		},
	}
	for _, session := range block.Content.Sessions {
		template := model.PlanSessionTemplate{TemplateID: uuid.NewString(), Title: session.Title, Stages: make([]model.PlanStage, 0, len(session.Stages))}
		for _, stage := range session.Stages {
			planStage := model.PlanStage{Phase: stage.Phase, Exercises: make([]model.PlanExercise, 0, len(stage.Exercises))}
			for _, exercise := range stage.Exercises {
				planStage.Exercises = append(planStage.Exercises, model.PlanExercise{
					PlanExerciseID: uuid.NewString(), ExerciseID: exercise.ExerciseID, NameSnapshot: exercise.NameSnapshot,
					Sets: exercise.Sets, Target: exercise.Target, RestSeconds: exercise.RestSeconds, Side: exercise.Side,
					LoadOrIntensity: exercise.LoadOrIntensity, Instructions: exercise.Instructions,
				})
			}
			template.Stages = append(template.Stages, planStage)
		}
		draft.Content.Sessions = append(draft.Content.Sessions, template)
	}
	if err := validatePlanDraft(draft); err != nil {
		return nil, model.TrainingPlanDraft{}, fmt.Errorf("%w: %w", ErrTreatmentPlanOutputInvalid, err)
	}
	return &block, draft, nil
}
