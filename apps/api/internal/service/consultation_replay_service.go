package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ErrConsultationReplayUnavailable is returned when a run predates the
// North-Star provenance columns (no frozen configuration identity).
var ErrConsultationReplayUnavailable = errors.New("consultation run predates North-Star provenance")

// ConsultationReplayInput is the frozen, sanitized turn input captured on the
// run record (profile + BodyState revision + relevant history + business
// context envelope). It never contains raw image bytes or credentials.
type ConsultationReplayInput struct {
	ConfigurationID   string          `json:"configuration_id"`
	ConversationID    string          `json:"conversation_id"`
	UserMessage       string          `json:"user_message"`
	Profile           json.RawMessage `json:"profile"`
	BodyState         json.RawMessage `json:"body_state"`
	BodyStateRevision int64           `json:"body_state_revision,omitempty"`
	RelevantHistory   json.RawMessage `json:"relevant_history,omitempty"`
	CurrentDiagnosis  json.RawMessage `json:"current_diagnosis,omitempty"`
	CurrentTreatment  json.RawMessage `json:"current_treatment,omitempty"`
	RecentOutcomes    json.RawMessage `json:"recent_outcomes,omitempty"`
	Phase             string          `json:"phase"`
}

// ConsultationRunDecision is the deterministic Go decision authority replay:
// which immutable configuration was selected, which policy revision gated it,
// and whether the persisted provenance is internally consistent.
type ConsultationRunDecision struct {
	RunID                      string         `json:"run_id"`
	SourceConfigurationID      string         `json:"source_configuration_id"`
	DecisionPolicyRevision     string         `json:"decision_policy_revision"`
	PersistedConfigurationID   string         `json:"persisted_configuration_id"`
	ConfigurationIdentityMatch bool           `json:"configuration_identity_match"`
	ReplayInputFrozen          bool           `json:"replay_input_frozen"`
	ExecutionProvenance        datatypes.JSON `json:"execution_provenance"`
	InputFingerprint           string         `json:"input_fingerprint,omitempty"`
}

// ConsultationReplayService replays the Go decision authority for completed
// Consultation runs without calling the model. It is the consultation analogue
// of the Diagnosis/Treatment/Assessment replay services.
type ConsultationReplayService struct {
	runRepo runRepo
}

func NewConsultationReplayService(runRepo runRepo) *ConsultationReplayService {
	return &ConsultationReplayService{runRepo: runRepo}
}

// HistoricalReplay recomputes the Go decision authority for a completed run.
// It never calls the model and never mutates state.
func (s *ConsultationReplayService) HistoricalReplay(
	ctx context.Context,
	userID uuid.UUID,
	runID uuid.UUID,
) (*ConsultationRunDecision, error) {
	run, err := s.getRun(ctx, userID, runID)
	if err != nil {
		return nil, err
	}
	if run.AgentConfigurationID == "" {
		return nil, ErrConsultationReplayUnavailable
	}

	policyRevision, err := ConsultationDecisionPolicyRevisionForConfiguration(run.AgentConfigurationID)
	if err != nil {
		// Fail closed: an unknown configuration is a governance anomaly.
		return nil, fmt.Errorf("consultation replay: %w", err)
	}

	input := ConsultationReplayInput{}
	replayRaw := json.RawMessage(run.ReplayInput)
	hasReplay := len(run.ReplayInput) > 2 // not "{}"
	if hasReplay {
		_ = json.Unmarshal(replayRaw, &input)
	}

	return &ConsultationRunDecision{
		RunID:                      run.ID.String(),
		SourceConfigurationID:      run.AgentConfigurationID,
		DecisionPolicyRevision:     policyRevision,
		PersistedConfigurationID:   run.AgentConfigurationID,
		ConfigurationIdentityMatch: true,
		ReplayInputFrozen:          hasReplay && input.ConfigurationID == run.AgentConfigurationID,
		ExecutionProvenance:        run.ExecutionProvenance,
		InputFingerprint:           consultationReplayInputFingerprint(input),
	}, nil
}

// CounterfactualReplay checks a completed run against another immutable
// configuration without calling the model. Read-only.
func (s *ConsultationReplayService) CounterfactualReplay(
	ctx context.Context,
	userID uuid.UUID,
	runID uuid.UUID,
	targetConfigurationID string,
) (*ConsultationRunDecision, error) {
	if err := validateConsultationConfigurationID(targetConfigurationID); err != nil {
		return nil, err
	}
	run, err := s.getRun(ctx, userID, runID)
	if err != nil {
		return nil, err
	}
	if run.AgentConfigurationID == "" {
		return nil, ErrConsultationReplayUnavailable
	}

	policyRevision, err := ConsultationDecisionPolicyRevisionForConfiguration(targetConfigurationID)
	if err != nil {
		return nil, fmt.Errorf("consultation counterfactual: %w", err)
	}

	input := ConsultationReplayInput{}
	hasReplay := len(run.ReplayInput) > 2
	if hasReplay {
		_ = json.Unmarshal(json.RawMessage(run.ReplayInput), &input)
	}

	return &ConsultationRunDecision{
		RunID:                      run.ID.String(),
		SourceConfigurationID:      targetConfigurationID,
		DecisionPolicyRevision:     policyRevision,
		PersistedConfigurationID:   run.AgentConfigurationID,
		ConfigurationIdentityMatch: targetConfigurationID == run.AgentConfigurationID,
		ReplayInputFrozen:          hasReplay,
		ExecutionProvenance:        run.ExecutionProvenance,
		InputFingerprint:           consultationReplayInputFingerprint(input),
	}, nil
}

func (s *ConsultationReplayService) getRun(
	ctx context.Context,
	userID uuid.UUID,
	runID uuid.UUID,
) (*model.Run, error) {
	if s == nil || s.runRepo == nil {
		return nil, errors.New("consultation replay service unavailable")
	}
	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load run: %w", err)
	}
	if run == nil || run.UserID != userID {
		return nil, errors.New("run not found")
	}
	return run, nil
}

func consultationReplayInputFingerprint(input ConsultationReplayInput) string {
	// Deterministic over the frozen input only; never over the model output.
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
