package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrDiagnosisBodyStateNotReady = errors.New("body state does not yet contain structured information for formal analysis")
var ErrDiagnosisAgentUnavailable = errors.New("formal diagnosis agent unavailable")

// DiagnosisFormalAnalysisCommand is the user's explicit request to start a
// formal analysis, grounded in their own versioned statement. It carries no
// owner, BodyState, Safety verdict or Agent content.
type DiagnosisFormalAnalysisCommand struct {
	RequestEvidenceID uuid.UUID
	Trigger           model.ConditionAnalysisTrigger
	ScopeDescription  string
	ExpectedRevision  int64
	IdempotencyKey    string
}

type diagnosisFormalAgent interface {
	AnalyzeDiagnosis(context.Context, DiagnosisRequest) (json.RawMessage, error)
}
type diagnosisProfileReader interface {
	GetProfile(context.Context, uuid.UUID) (*model.UserProfile, error)
}

// DiagnosisFormalAnalysisService freezes the Agent's input, applies Go Safety
// and decision policy, calls the Diagnosis Agent and hands its formal output to
// the ingestion boundary.
type DiagnosisFormalAnalysisService struct {
	ingestion  *DiagnosisProposalIngestionService
	body       *BodyStateService
	profiles   diagnosisProfileReader
	ai         diagnosisFormalAgent
	deployment *AgentDeploymentPolicy
}

func NewDiagnosisFormalAnalysisService(db *gorm.DB, profiles diagnosisProfileReader, ai diagnosisFormalAgent, deployment *AgentDeploymentPolicy) *DiagnosisFormalAnalysisService {
	ingestion := NewDiagnosisProposalIngestionService(db)
	return &DiagnosisFormalAnalysisService{ingestion: ingestion, body: ingestion.body, profiles: profiles, ai: ai, deployment: deployment}
}

func diagnosisGoAuthoredInsufficient(reason string) json.RawMessage {
	encoded, _ := json.Marshal(DiagnosisAgentProposals{SchemaRevision: DiagnosisAgentProposalsSchemaV1, Outcome: DiagnosisAgentOutcomeInsufficient, Summary: "go_decision_policy:" + reason, Proposals: []DiagnosisAgentProposal{}})
	return encoded
}

// diagnosisFrozenEvidenceRefs lists exactly the asserted, versioned sources the
// Agent sees. An Agent reference outside this set is rejected at ingestion.
func diagnosisFrozenEvidenceRefs(snapshot *BodyStateReadModel) []model.DiagnosisEvidenceRef {
	refs := []model.DiagnosisEvidenceRef{}
	for _, fact := range snapshot.Facts {
		if fact.AssertionStatus == "asserted" && fact.CreatedRevision > 0 {
			refs = append(refs, model.DiagnosisEvidenceRef{EvidenceType: model.DiagnosisBodyFact, EvidenceID: fact.ID, Version: strconv.FormatInt(fact.CreatedRevision, 10)})
		}
	}
	for _, observation := range snapshot.Observations {
		if observation.AssertionStatus == "asserted" && observation.CreatedRevision > 0 {
			refs = append(refs, model.DiagnosisEvidenceRef{EvidenceType: model.DiagnosisBodyObservation, EvidenceID: observation.ID, Version: strconv.FormatInt(observation.CreatedRevision, 10)})
		}
	}
	return refs
}

func diagnosisDecisionBlocks(outcome DiagnosisDecisionOutcome) bool {
	return outcome == DiagnosisBlock || outcome == DiagnosisEscalate
}

// Start runs one formal analysis. A retried key replays its original run
// without calling the Agent again. Any Agent, Safety or validation failure
// writes nothing except Go's own terminal Safety audit.
func (s *DiagnosisFormalAnalysisService) Start(ctx context.Context, owner uuid.UUID, c DiagnosisFormalAnalysisCommand) (*DiagnosisProposalIngestionReceipt, error) {
	if s == nil || s.ingestion == nil || s.body == nil {
		return nil, ErrDiagnosisAgentUnavailable
	}
	ingestion := DiagnosisProposalIngestion{RequestEvidenceID: c.RequestEvidenceID, Trigger: c.Trigger, ScopeDescription: c.ScopeDescription, ExpectedRevision: c.ExpectedRevision, IdempotencyKey: c.IdempotencyKey}
	if replayed, err := s.ingestion.Replay(ctx, owner, ingestion); err != nil || replayed != nil {
		return replayed, err
	}
	// The owner's own available request statement is checked before the Agent
	// sees anything; ingestion re-verifies it under lock.
	request, err := s.ingestion.repo.ResolveAnalysisRequest(ctx, owner, c.RequestEvidenceID)
	if err != nil {
		return nil, err
	}
	if request.SourceKind != "user_message" || request.SourceMessageID == nil || request.Availability != "available" {
		return nil, model.ErrDiagnosisNotFound
	}
	snapshot, err := s.body.GetBodyState(ctx, owner, 50)
	if err != nil || snapshot == nil {
		return nil, fmt.Errorf("%w: load body state", ErrDiagnosisSafetyUnavailable)
	}
	if snapshot.CurrentRevision == 0 || (len(snapshot.Facts) == 0 && len(snapshot.Observations) == 0) {
		return nil, ErrDiagnosisBodyStateNotReady
	}
	if s.deployment == nil {
		return nil, ErrDiagnosisAgentUnavailable
	}
	route := s.deployment.SelectDiagnosisRoute(owner.String())
	if route.ServedDecisionPolicyRevision != DiagnosisDecisionPolicyV2 {
		return nil, ErrDiagnosisAgentUnavailable
	}
	envelope, err := ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		return nil, ErrDiagnosisSafetyUnavailable
	}
	bodyStateJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, ErrDiagnosisIngestionInvalid
	}
	historyJSON, _ := json.Marshal(snapshot.RecentRevisions)
	profileJSON := json.RawMessage(`{}`)
	if s.profiles != nil {
		if profile, profileErr := s.profiles.GetProfile(ctx, owner); profileErr == nil && profile != nil {
			if encoded, marshalErr := json.Marshal(profile.AgentContext()); marshalErr == nil {
				profileJSON = encoded
			}
		}
	}
	replayInput, err := EncodeDiagnosisReplayInput(snapshot.CurrentRevision, bodyStateJSON, historyJSON, profileJSON, &envelope)
	if err != nil {
		return nil, ErrDiagnosisIngestionInvalid
	}
	ingestion.BodyStateRevision = snapshot.CurrentRevision
	ingestion.ReplayInput = replayInput
	ingestion.InputEvidenceRefs = diagnosisFrozenEvidenceRefs(snapshot)
	provenance := map[string]any{"configurationId": route.ServedConfigurationID, "decisionPolicyRevision": route.ServedDecisionPolicyRevision, "rolloutStage": route.Stage}
	ingestion.ExecutionProvenance = provenance

	probe := map[string]any{"status": "completed", "candidates": []any{map[string]any{"name": "preflight"}}, "governance": map[string]any{"verdict": "accepted"}, "safety_findings": []any{}}
	pre := EvaluateDiagnosisDecisionV2(&envelope, snapshot.CurrentRevision, probe)
	provenance["preflightOutcome"] = pre.Outcome
	switch {
	case diagnosisDecisionBlocks(pre.Outcome):
		ingestion.GoSafetyBlocked = true
		provenance["agentInvoked"] = false
		return s.ingestion.Ingest(ctx, owner, ingestion)
	case pre.Outcome != DiagnosisAllowNormal:
		ingestion.AgentOutput = diagnosisGoAuthoredInsufficient(firstReason(pre.Reasons))
		provenance["agentInvoked"] = false
		return s.ingestion.Ingest(ctx, owner, ingestion)
	}
	if s.ai == nil {
		return nil, ErrDiagnosisAgentUnavailable
	}
	result, err := s.ai.AnalyzeDiagnosis(ctx, DiagnosisRequest{
		UserID: owner.String(), ConfigurationID: route.ServedConfigurationID,
		BodyStateRevision: snapshot.CurrentRevision, BodyState: bodyStateJSON,
		SafetyEnvelope: &envelope, RelevantHistory: historyJSON, Profile: profileJSON,
	})
	if err != nil {
		log.Printf("formal diagnosis agent failed for BodyState R%d user %s: %v", snapshot.CurrentRevision, owner, err)
		return nil, ErrDiagnosisAgentUnavailable
	}
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil || parsed == nil {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	if !diagnosisAgentConfigurationMatches(parsed, route.ServedConfigurationID) {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	provenance["agentInvoked"] = true
	provenance["agentConfiguration"] = parsed["agent_configuration"]
	provenance["agentExecution"] = parsed["execution_provenance"]
	// Go, not the Agent, decides whether this output may become proposals.
	decision := EvaluateDiagnosisDecisionV2(&envelope, snapshot.CurrentRevision, parsed)
	provenance["decision"] = decision
	switch {
	case diagnosisDecisionBlocks(decision.Outcome):
		ingestion.GoSafetyBlocked = true
	case decision.Outcome == DiagnosisAllowNormal || decision.Outcome == DiagnosisAllowDegraded:
		formal, ok := parsed["formal_proposals"]
		if !ok || formal == nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		if ingestion.AgentOutput, err = json.Marshal(formal); err != nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
	default:
		ingestion.AgentOutput = diagnosisGoAuthoredInsufficient(firstReason(decision.Reasons))
	}
	return s.ingestion.Ingest(ctx, owner, ingestion)
}

func firstReason(reasons []string) string {
	if len(reasons) == 0 {
		return "unspecified"
	}
	return reasons[0]
}
