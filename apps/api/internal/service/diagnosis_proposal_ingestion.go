package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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

func diagnosisFrozenInputIndex(refs []model.DiagnosisEvidenceRef) (map[string]string, []model.DiagnosisEvidenceRef, error) {
	if len(refs) > 2000 {
		return nil, nil, ErrDiagnosisIngestionInvalid
	}
	index := map[string]string{}
	out := make([]model.DiagnosisEvidenceRef, 0, len(refs))
	for _, ref := range refs {
		v, err := strconv.ParseInt(ref.Version, 10, 64)
		if err != nil || v <= 0 || strconv.FormatInt(v, 10) != ref.Version || ref.EvidenceID == uuid.Nil {
			return nil, nil, ErrDiagnosisIngestionInvalid
		}
		switch ref.EvidenceType {
		case model.DiagnosisBodyFact, model.DiagnosisBodyObservation:
		default:
			return nil, nil, ErrDiagnosisIngestionInvalid
		}
		key := string(ref.EvidenceType) + ":" + ref.EvidenceID.String()
		if _, dup := index[key]; dup {
			return nil, nil, ErrDiagnosisIngestionInvalid
		}
		index[key] = ref.Version
		out = append(out, ref)
	}
	return index, out, nil
}

func decodeDiagnosisAgentProposals(raw json.RawMessage, goBlocked bool) (*DiagnosisAgentProposals, error) {
	if goBlocked && len(raw) == 0 {
		return &DiagnosisAgentProposals{SchemaRevision: DiagnosisAgentProposalsSchemaV1, Outcome: DiagnosisAgentOutcomeInsufficient, Proposals: []DiagnosisAgentProposal{}}, nil
	}
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var out DiagnosisAgentProposals
	if err := decoder.Decode(&out); err != nil {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	if out.SchemaRevision != DiagnosisAgentProposalsSchemaV1 || utf8.RuneCountInString(out.Summary) > diagnosisAgentMaxText {
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	switch out.Outcome {
	case DiagnosisAgentOutcomeProposals:
		if len(out.Proposals) == 0 || len(out.Proposals) > diagnosisAgentMaxProposals {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
	case DiagnosisAgentOutcomeInsufficient:
		if len(out.Proposals) != 0 {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
	default:
		return nil, ErrDiagnosisAgentOutputInvalid
	}
	return &out, nil
}

type diagnosisAgentDraft struct {
	statement string
	proposal  model.DiagnosisProposal
	clinical  diagnosisClinicalContent
}

func diagnosisAgentText(s string, required bool) bool {
	n := utf8.RuneCountInString(s)
	return n <= diagnosisAgentMaxText && (!required || strings.TrimSpace(s) != "")
}
func diagnosisAgentList(items []string) ([]string, bool) {
	if len(items) > diagnosisAgentMaxListItems {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !diagnosisAgentText(item, true) {
			return nil, false
		}
		out = append(out, item)
	}
	return out, true
}

// buildDiagnosisAgentDrafts turns Agent proposals into exact immutable content.
// Every evidence reference must resolve to the frozen input the Agent saw; a
// foreign, unseen or later evidence ID rejects the whole output.
func buildDiagnosisAgentDrafts(run *model.ConditionAnalysisRun, out *DiagnosisAgentProposals, versions map[string]string) ([]diagnosisAgentDraft, error) {
	drafts := make([]diagnosisAgentDraft, 0, len(out.Proposals))
	names := map[string]bool{}
	for _, ap := range out.Proposals {
		if ap.ChangeKind != model.DiagnosisCreate {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		name := strings.TrimSpace(ap.MedicalName)
		normalized := strings.ToLower(strings.Join(strings.Fields(name), " "))
		if name == "" || utf8.RuneCountInString(name) > 240 || names[normalized] {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		names[normalized] = true
		if ap.Assessment.Level != model.DiagnosisSuspected && ap.Assessment.Level != model.DiagnosisLikely {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		if !diagnosisAgentText(ap.Assessment.Rationale, true) || !diagnosisAgentText(ap.CauseAnalysis.Summary, true) ||
			!diagnosisAgentText(ap.CauseAnalysis.Rationale, true) || !diagnosisAgentText(ap.HandlingPlan.Summary, true) {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		gaps, ok1 := diagnosisAgentList(ap.Assessment.InformationGaps)
		diffs, ok2 := diagnosisAgentList(ap.Assessment.DifferentialNotes)
		entryGaps, ok3 := diagnosisAgentList(ap.InformationGaps)
		actions, ok4 := diagnosisAgentList(ap.HandlingPlan.Actions)
		cautions, ok5 := diagnosisAgentList(ap.HandlingPlan.CautionNotes)
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || len(ap.Evidence) == 0 || len(ap.Evidence) > diagnosisAgentMaxEvidence {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		supporting := []model.DiagnosisEvidenceRef{}
		counter := []model.DiagnosisEvidenceRef{}
		links := make([]diagnosisProposedEvidence, 0, len(ap.Evidence))
		seen := map[string]bool{}
		for _, ev := range ap.Evidence {
			version, ok := versions[string(ev.EvidenceType)+":"+ev.EvidenceID.String()]
			if !ok || ev.EvidenceID == uuid.Nil || !diagnosisAgentText(ev.Rationale, true) {
				return nil, ErrDiagnosisAgentOutputInvalid
			}
			ref := model.DiagnosisEvidenceRef{EvidenceType: ev.EvidenceType, EvidenceID: ev.EvidenceID, Version: version}
			key := evidenceKey(ref) + ":" + string(ev.Role)
			if seen[key] {
				return nil, ErrDiagnosisAgentOutputInvalid
			}
			seen[key] = true
			switch ev.Role {
			case model.DiagnosisSupporting, model.DiagnosisContext:
				supporting = append(supporting, ref)
			case model.DiagnosisCounter:
				counter = append(counter, ref)
			default:
				return nil, ErrDiagnosisAgentOutputInvalid
			}
			links = append(links, diagnosisProposedEvidence{EvidenceType: ev.EvidenceType, EvidenceID: ev.EvidenceID, EvidenceVersion: version, Role: ev.Role, Rationale: ev.Rationale, AnalyzedByRunID: run.ID})
		}
		hasSupport := false
		for _, ev := range ap.Evidence {
			hasSupport = hasSupport || ev.Role == model.DiagnosisSupporting
		}
		if !hasSupport {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		clinical := diagnosisClinicalContent{
			MedicalName: name, ConceptRef: ap.ConceptRef, Qualifiers: ap.Qualifiers,
			Assessment:      &model.ConditionAssessment{Level: ap.Assessment.Level, Rationale: ap.Assessment.Rationale, InformationGaps: gaps, DifferentialNotes: diffs, BasedOnRunID: run.ID},
			SourceClaimRefs: []model.DiagnosisSourceClaimRef{}, EvidenceLinks: links,
			CauseAnalysis: ap.CauseAnalysis, InformationGaps: entryGaps,
			HandlingPlan: model.ConditionHandlingPlan{Summary: ap.HandlingPlan.Summary, Actions: actions, CautionNotes: cautions},
		}
		normalizeDiagnosisClinical(&clinical)
		entry, err := json.Marshal(clinical)
		if err != nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		content, err := json.Marshal(map[string]json.RawMessage{"proposedEntry": entry})
		if err != nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		p := model.DiagnosisProposal{
			UserID: run.UserID, ID: uuid.New(), RunID: run.ID, ChangeKind: model.DiagnosisCreate,
			Content: datatypes.JSON(content), BaseRevision: run.BasedOnDiagnosisRevision,
			EvidenceRefs: supporting, CounterEvidenceRefs: counter,
		}
		if p.ContentFingerprint, err = model.DiagnosisProposalContentFingerprint(&p); err != nil {
			return nil, ErrDiagnosisAgentOutputInvalid
		}
		drafts = append(drafts, diagnosisAgentDraft{statement: name, proposal: p, clinical: clinical})
	}
	return drafts, nil
}
