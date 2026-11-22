package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

// DiagnosisAgentProposalsSchemaV1 is the only Agent -> Go formal-analysis
// output contract. The Agent proposes; Go assigns every identity, version,
// fingerprint and base revision, and nothing in it can adopt a condition.
const DiagnosisAgentProposalsSchemaV1 = "diagnosis-agent-proposals-v1"

const (
	DiagnosisAgentOutcomeProposals    = "proposals"
	DiagnosisAgentOutcomeInsufficient = "insufficient_evidence"
	diagnosisAgentMaxProposals        = 50
	diagnosisAgentMaxEvidence         = 200
	diagnosisAgentMaxText             = 4000
	diagnosisAgentMaxListItems        = 50
)

var ErrDiagnosisAgentOutputInvalid = errors.New("formal diagnosis agent output rejected")
var ErrDiagnosisIngestionStale = errors.New("formal diagnosis analysis input is stale")
var ErrDiagnosisIngestionInvalid = errors.New("invalid formal diagnosis analysis command")

// DiagnosisAgentProposals is decoded strictly: unknown fields (an Agent-chosen
// ID, disposition, revision, owner or fact) reject the whole output.
type DiagnosisAgentProposals struct {
	SchemaRevision string                   `json:"schemaRevision"`
	Outcome        string                   `json:"outcome"`
	Summary        string                   `json:"summary"`
	Proposals      []DiagnosisAgentProposal `json:"proposals"`
}

// DiagnosisAgentProposal is one named-condition create proposal. Evidence is
// referenced by type + ID only; the exact version comes from the frozen input.
type DiagnosisAgentProposal struct {
	ChangeKind      model.DiagnosisChangeKind    `json:"changeKind"`
	MedicalName     string                       `json:"medicalName"`
	ConceptRef      *model.MedicalConceptRef     `json:"conceptRef,omitempty"`
	Qualifiers      *model.ConditionQualifiers   `json:"qualifiers,omitempty"`
	Assessment      DiagnosisAgentAssessment     `json:"assessment"`
	CauseAnalysis   model.ConditionCauseAnalysis `json:"causeAnalysis"`
	InformationGaps []string                     `json:"informationGaps"`
	HandlingPlan    model.ConditionHandlingPlan  `json:"handlingPlan"`
	Evidence        []DiagnosisAgentEvidence     `json:"evidence"`
}

// DiagnosisAgentAssessment is the AI's read-only judgment; basedOnRunId is Go's.
type DiagnosisAgentAssessment struct {
	Level             model.DiagnosisLevel `json:"level"`
	Rationale         string               `json:"rationale"`
	InformationGaps   []string             `json:"informationGaps"`
	DifferentialNotes []string             `json:"differentialNotes"`
}

type DiagnosisAgentEvidence struct {
	EvidenceType model.DiagnosisEvidenceType `json:"evidenceType"`
	EvidenceID   uuid.UUID                   `json:"evidenceId"`
	Role         model.DiagnosisEvidenceRole `json:"role"`
	Rationale    string                      `json:"rationale"`
}

// DiagnosisProposalIngestion is built by Go from authenticated state. The
// frozen input fields describe exactly what the Agent saw; the owner is a
// separate trusted argument and never part of the Agent payload.
type DiagnosisProposalIngestion struct {
	RequestEvidenceID uuid.UUID
	Trigger           model.ConditionAnalysisTrigger
	ScopeDescription  string
	ExpectedRevision  int64
	IdempotencyKey    string

	BodyStateRevision   int64
	InputEvidenceRefs   []model.DiagnosisEvidenceRef
	ReplayInput         json.RawMessage
	ExecutionProvenance map[string]any
	// GoSafetyBlocked is Go's own decision-policy verdict on this run, never an
	// Agent claim. A blocked run is terminal with zero proposals.
	GoSafetyBlocked bool
	AgentOutput     json.RawMessage
}

// DiagnosisProposalIngestionReceipt acknowledges persisted proposals only. It is
// never a decision or DiagnosisRevision; adoption still requires user choices
// and the DX-002 whole-run settlement.
type DiagnosisProposalIngestionReceipt struct {
	RunID       uuid.UUID
	Outcome     string // proposals | insufficient_evidence | safety_blocked
	ProposalIDs []uuid.UUID
	Replayed    bool
}

func (c DiagnosisProposalIngestion) requestValid() bool {
	return c.RequestEvidenceID != uuid.Nil && c.Trigger == model.ConditionNewAnalysis && c.ExpectedRevision >= 0 &&
		len(c.IdempotencyKey) > 0 && len(c.IdempotencyKey) <= 128 && strings.TrimSpace(c.IdempotencyKey) == c.IdempotencyKey &&
		utf8.RuneCountInString(c.ScopeDescription) <= 500
}

// diagnosisIngestionFingerprint binds the user's command, not the Agent output:
// a lost response retried with the same key replays the original run instead
// of re-running the model or accepting a second, different output.
func diagnosisIngestionFingerprint(owner uuid.UUID, c DiagnosisProposalIngestion) (string, error) {
	encoded, err := json.Marshal(struct {
		Owner            uuid.UUID
		Operation        string
		RequestEvidence  uuid.UUID
		Trigger          model.ConditionAnalysisTrigger
		ScopeDescription string
		ExpectedRevision int64
	}{owner, "formal_analysis", c.RequestEvidenceID, c.Trigger, c.ScopeDescription, c.ExpectedRevision})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func DiagnosisReplayInputDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type DiagnosisProposalIngestionService struct {
	repo       *repository.DiagnosisRepository
	sources    *repository.DiagnosisSourceRepository
	body       *BodyStateService
	tx         *database.TransactionManager
	settlement *DiagnosisSettlementService
	now        func() time.Time
	// Test seam; production always projects the live pinned Go Safety ledger.
	projectSafety func(*BodyStateReadModel) (SafetyEnvelopeV2, error)
}

func NewDiagnosisProposalIngestionService(db *gorm.DB) *DiagnosisProposalIngestionService {
	return &DiagnosisProposalIngestionService{
		repo: repository.NewDiagnosisRepository(db), sources: repository.NewDiagnosisSourceRepository(db),
		body: NewBodyStateService(repository.NewBodyStateRepository(db)), tx: database.NewTransactionManager(db),
		settlement: NewDiagnosisSettlementService(db),
		now:        func() time.Time { return time.Now().UTC() }, projectSafety: ProjectSafetyEnvelopeV2,
	}
}

// Replay returns the receipt of an already-ingested command without touching
// the Agent, or nil when the key is unused. A different body is a conflict.
func (s *DiagnosisProposalIngestionService) Replay(ctx context.Context, owner uuid.UUID, c DiagnosisProposalIngestion) (*DiagnosisProposalIngestionReceipt, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if !c.requestValid() {
		return nil, ErrDiagnosisIngestionInvalid
	}
	fingerprint, err := diagnosisIngestionFingerprint(owner, c)
	if err != nil {
		return nil, ErrDiagnosisIngestionInvalid
	}
	return s.replay(ctx, owner, c.IdempotencyKey, fingerprint)
}

func (s *DiagnosisProposalIngestionService) replay(ctx context.Context, owner uuid.UUID, key, fingerprint string) (*DiagnosisProposalIngestionReceipt, error) {
	existing, err := s.repo.FindAnalysisRunByKey(ctx, owner, key)
	if err != nil || existing == nil {
		return nil, err
	}
	if existing.RequestFingerprint != fingerprint {
		return nil, ErrDiagnosisDecisionConflict
	}
	ids, err := s.repo.ListRunProposalIDs(ctx, owner, existing.ID)
	if err != nil {
		return nil, err
	}
	terminal, err := s.repo.RunTerminalKind(ctx, owner, existing.ID)
	if err != nil {
		return nil, err
	}
	outcome := DiagnosisAgentOutcomeProposals
	switch terminal {
	case "insufficient_evidence", "safety_blocked":
		outcome = terminal
	}
	return &DiagnosisProposalIngestionReceipt{RunID: existing.ID, Outcome: outcome, ProposalIDs: ids, Replayed: true}, nil
}

// Ingest validates the Agent output completely, then writes one formal run,
// its proposals and ordered audit events in one owner-fenced transaction, or
// nothing. It never writes a ConditionEntry, decision, revision or BodyState.
func (s *DiagnosisProposalIngestionService) Ingest(ctx context.Context, owner uuid.UUID, c DiagnosisProposalIngestion) (*DiagnosisProposalIngestionReceipt, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if !c.requestValid() || c.BodyStateRevision <= 0 || len(c.ReplayInput) == 0 || !json.Valid(c.ReplayInput) || bytes.TrimSpace(c.ReplayInput)[0] != '{' {
		return nil, ErrDiagnosisIngestionInvalid
	}
	fingerprint, err := diagnosisIngestionFingerprint(owner, c)
	if err != nil {
		return nil, ErrDiagnosisIngestionInvalid
	}
	inputVersions, inputRefs, err := diagnosisFrozenInputIndex(c.InputEvidenceRefs)
	if err != nil {
		return nil, ErrDiagnosisIngestionInvalid
	}
	output, err := decodeDiagnosisAgentProposals(c.AgentOutput, c.GoSafetyBlocked)
	if err != nil {
		return nil, err
	}
	provenance := map[string]any{}
	for k, v := range c.ExecutionProvenance {
		provenance[k] = v
	}
	provenance["agentOutputSchemaRevision"] = DiagnosisAgentProposalsSchemaV1
	provenance["replayInputDigest"] = DiagnosisReplayInputDigest(c.ReplayInput)
	provenance["agentOutputDigest"] = DiagnosisReplayInputDigest(c.AgentOutput)
	provenanceJSON, err := json.Marshal(provenance)
	if err != nil {
		return nil, ErrDiagnosisIngestionInvalid
	}

	var receipt *DiagnosisProposalIngestionReceipt
	var terminalErr error
	err = s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if _, err := s.repo.EnsureAggregate(ctx, owner); err != nil {
			return err
		}
		body, head, err := s.repo.LockDecisionFrame(ctx, owner)
		if err != nil {
			return err
		}
		replayed, err := s.replay(ctx, owner, c.IdempotencyKey, fingerprint)
		if err != nil {
			return err
		}
		if replayed != nil {
			receipt = replayed
			return nil
		}
		if c.ExpectedRevision != head.CurrentRevision {
			return ErrDiagnosisDecisionConflict
		}
		if body.CurrentRevision != c.BodyStateRevision {
			return ErrDiagnosisIngestionStale
		}
		request, err := s.repo.ResolveAnalysisRequest(ctx, owner, c.RequestEvidenceID)
		if err != nil {
			return err
		}
		if request.SourceMessageID == nil || request.SourceKind != "user_message" {
			return model.ErrDiagnosisNotFound
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		run := &model.ConditionAnalysisRun{
			UserID: owner, ID: uuid.New(), RequestMessageID: *request.SourceMessageID, RequestEvidenceID: request.ID,
			RequestSourceVersion: request.SourceVersion, Trigger: c.Trigger,
			Scope:                    model.ConditionAnalysisScope{ConditionIDs: []uuid.UUID{}, EvidenceRefs: []model.DiagnosisEvidenceRef{}, Description: c.ScopeDescription},
			BasedOnBodyStateRevision: c.BodyStateRevision, BasedOnDiagnosisRevision: head.CurrentRevision,
			InputEvidenceRefs: inputRefs, AgentHypotheses: []model.ConditionAgentHypothesis{},
			ReplayInput: datatypes.JSON(c.ReplayInput), ExecutionProvenance: datatypes.JSON(provenanceJSON),
			IdempotencyKey: c.IdempotencyKey, RequestFingerprint: fingerprint, StartedAt: now,
		}
		drafts, err := buildDiagnosisAgentDrafts(run, output, inputVersions)
		if err != nil {
			return err
		}
		for _, d := range drafts {
			run.AgentHypotheses = append(run.AgentHypotheses, model.ConditionAgentHypothesis{Statement: d.statement, EvidenceRefs: append(append([]model.DiagnosisEvidenceRef{}, d.proposal.EvidenceRefs...), d.proposal.CounterEvidenceRefs...)})
		}
		if err := s.repo.InsertAnalysisRun(ctx, run); err != nil {
			return err
		}
		if err := s.sources.AnalysisRequest(ctx, owner, *run); err != nil {
			return diagnosisEvidenceError(err)
		}
		if err := s.repo.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: owner, RunID: run.ID, Kind: "run_started", BodyStateRevision: body.CurrentRevision, Details: datatypes.JSON(`{}`), RecordedAt: now}); err != nil {
			return err
		}
		receipt = &DiagnosisProposalIngestionReceipt{RunID: run.ID, ProposalIDs: []uuid.UUID{}}
		snapshot, err := s.body.GetBodyState(ctx, owner, 1)
		if err != nil || snapshot == nil || snapshot.CurrentRevision != body.CurrentRevision {
			return ErrDiagnosisSafetyUnavailable
		}
		envelope, err := s.projectSafety(snapshot)
		if err != nil {
			return ErrDiagnosisSafetyUnavailable
		}
		urgent := c.GoSafetyBlocked
		for _, blocker := range envelope.ActiveBlockers {
			urgent = urgent || blocker.RiskLevel == "urgent"
		}
		if urgent {
			// Terminal Safety audit with zero proposals: the Agent's output, if
			// any, is never persisted as a choice the user could adopt.
			receipt.Outcome = "safety_blocked"
			terminalErr = ErrDiagnosisSafetyBlocked
			return s.repo.BlockAnalysisForUrgent(ctx, owner, run.ID, body.CurrentRevision, now)
		}
		if !envelope.Coverage.Complete {
			return ErrDiagnosisSafetyUnavailable
		}
		if output.Outcome == DiagnosisAgentOutcomeInsufficient {
			receipt.Outcome = DiagnosisAgentOutcomeInsufficient
			details, err := json.Marshal(map[string]string{"summary": output.Summary})
			if err != nil {
				return ErrDiagnosisAgentOutputInvalid
			}
			return s.repo.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: owner, RunID: run.ID, Kind: "insufficient_evidence", BodyStateRevision: body.CurrentRevision, Details: datatypes.JSON(details), RecordedAt: now})
		}
		receipt.Outcome = DiagnosisAgentOutcomeProposals
		for _, d := range drafts {
			// Reuse the exact settlement-time clinical content rules so an
			// ingested proposal is one the user can actually choose and settle.
			if _, err := s.settlement.validateSettlementClinical(ctx, owner, *run, d.proposal, d.clinical, diagnosisClinicalFields, body.CurrentRevision); err != nil {
				if errors.Is(err, ErrDiagnosisSettlementInvalid) || errors.Is(err, ErrDiagnosisDecisionConflict) {
					return ErrDiagnosisAgentOutputInvalid
				}
				return err
			}
			p := d.proposal
			p.CreatedAt = now
			if err := s.repo.InsertProposal(ctx, &p, body.CurrentRevision); err != nil {
				return err
			}
			receipt.ProposalIDs = append(receipt.ProposalIDs, p.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if terminalErr != nil {
		return receipt, terminalErr
	}
	return receipt, nil
}
