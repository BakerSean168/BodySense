package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

var diagnosisFingerprintPattern = regexp.MustCompile("^[0-9a-f]{64}$")

type diagnosisRunReadRepository interface {
	ReadRunSnapshot(context.Context, uuid.UUID, uuid.UUID) (*repository.DiagnosisRunSnapshot, error)
	ListRunPage(context.Context, uuid.UUID, int, *uuid.UUID) (*repository.DiagnosisRunPage, error)
}
type diagnosisRunFreshnessEvaluator interface {
	EvaluateRun(context.Context, uuid.UUID, *NativeDiagnosisRunProjection) (*DiagnosisFreshness, error)
}

type NativeDiagnosisRunReadService struct {
	repo      diagnosisRunReadRepository
	freshness diagnosisRunFreshnessEvaluator
}

func NewNativeDiagnosisRunReadService(repo diagnosisRunReadRepository) *NativeDiagnosisRunReadService {
	return &NativeDiagnosisRunReadService{repo: repo}
}

// WithFreshness makes Read derive an open run's freshness, so a run whose
// base Diagnosis revision or cited sources changed is visibly stale before
// its settlement fails with a conflict.
func (s *NativeDiagnosisRunReadService) WithFreshness(freshness diagnosisRunFreshnessEvaluator) *NativeDiagnosisRunReadService {
	s.freshness = freshness
	return s
}

// NativeDiagnosisRunSummary is a terminal-event-derived status, not a mutable
// Agent state or a substitute for Go's live Safety/evidence validation.
type NativeDiagnosisRunSummary struct {
	RunID                      uuid.UUID                      `json:"runId"`
	Trigger                    model.ConditionAnalysisTrigger `json:"trigger"`
	BasedOnBodyStateRevision   int64                          `json:"basedOnBodyStateRevision"`
	BasedOnDiagnosisRevision   int64                          `json:"basedOnDiagnosisRevision"`
	StartedAt                  time.Time                      `json:"startedAt"`
	State                      string                         `json:"state"`
	Outcome                    *string                        `json:"outcome,omitempty"`
	CommittedDiagnosisRevision *int64                         `json:"committedDiagnosisRevision,omitempty"`
}
type NativeDiagnosisRunPage struct {
	OwnerID         uuid.UUID                   `json:"-"`
	Runs            []NativeDiagnosisRunSummary `json:"runs"`
	HasMore         bool                        `json:"hasMore"`
	NextBeforeRunID *uuid.UUID                  `json:"nextBeforeRunId,omitempty"`
}
type NativeDiagnosisRunProjection struct {
	OwnerID uuid.UUID `json:"-"`
	NativeDiagnosisRunSummary
	CurrentDiagnosisRevision int64                            `json:"currentDiagnosisRevision"`
	RequestMessageID         uuid.UUID                        `json:"requestMessageId"`
	RequestEvidenceID        uuid.UUID                        `json:"requestEvidenceId"`
	RequestSourceVersion     int64                            `json:"requestSourceVersion"`
	Scope                    model.ConditionAnalysisScope     `json:"scope"`
	InputEvidenceRefs        []model.DiagnosisEvidenceRef     `json:"inputEvidenceRefs"`
	AgentHypotheses          []model.ConditionAgentHypothesis `json:"agentHypotheses"`
	Proposals                []NativeDiagnosisRunProposal     `json:"proposals"`
	Events                   []NativeDiagnosisRunEvent        `json:"events"`
	Settlement               *NativeDiagnosisRunSettlement    `json:"settlement,omitempty"`
	// Freshness is derived for open runs only; terminal runs are history.
	Freshness *DiagnosisFreshness `json:"freshness,omitempty"`
}
type NativeDiagnosisRunProposal struct {
	ProposalID             uuid.UUID                    `json:"proposalId"`
	RunID                  uuid.UUID                    `json:"runId"`
	TargetConditionID      *uuid.UUID                   `json:"targetConditionId,omitempty"`
	ChangeKind             model.DiagnosisChangeKind    `json:"changeKind"`
	Content                json.RawMessage              `json:"content"`
	ContentFingerprint     string                       `json:"contentFingerprint"`
	BaseRevision           int64                        `json:"baseRevision"`
	EvidenceRefs           []model.DiagnosisEvidenceRef `json:"evidenceRefs"`
	CounterEvidenceRefs    []model.DiagnosisEvidenceRef `json:"counterEvidenceRefs"`
	SupersedesProposalID   *uuid.UUID                   `json:"supersedesProposalId,omitempty"`
	SupersededByProposalID *uuid.UUID                   `json:"supersededByProposalId,omitempty"`
	CreatedAt              time.Time                    `json:"createdAt"`
	State                  string                       `json:"state"`
	Reason                 *string                      `json:"reason,omitempty"`
	Decision               *NativeDiagnosisRunDecision  `json:"decision,omitempty"`
}
type NativeDiagnosisRunDecision struct {
	DecisionID         uuid.UUID                   `json:"decisionId"`
	Decision           model.DiagnosisDecisionKind `json:"decision"`
	ContentFingerprint string                      `json:"contentFingerprint"`
	ReceiptID          uuid.UUID                   `json:"receiptId"`
	DecidedAt          time.Time                   `json:"decidedAt"`
}
type NativeDiagnosisRunEvent struct {
	Sequence   int64      `json:"sequence"`
	Kind       string     `json:"kind"`
	ProposalID *uuid.UUID `json:"proposalId,omitempty"`
	DecisionID *uuid.UUID `json:"decisionId,omitempty"`
	Reason     *string    `json:"reason,omitempty"`
	RecordedAt time.Time  `json:"recordedAt"`
}
type NativeDiagnosisRunSettlement struct {
	SettlementID        uuid.UUID  `json:"settlementId"`
	Outcome             string     `json:"outcome"`
	Revision            int64      `json:"revision"`
	DiagnosisRevisionID *uuid.UUID `json:"diagnosisRevisionId,omitempty"`
	ReceiptID           uuid.UUID  `json:"receiptId"`
	SettledAt           time.Time  `json:"settledAt"`
}

func (s *NativeDiagnosisRunReadService) List(ctx context.Context, owner uuid.UUID, limit int, before *uuid.UUID) (*NativeDiagnosisRunPage, error) {
	page, err := s.repo.ListRunPage(ctx, owner, limit, before)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, errors.New("diagnosis run index unavailable")
	}
	result := &NativeDiagnosisRunPage{OwnerID: owner, Runs: []NativeDiagnosisRunSummary{}, HasMore: page.HasMore, NextBeforeRunID: page.NextBeforeRunID}
	eventsByRun := make(map[uuid.UUID][]model.ConditionAnalysisEvent)
	settlementsByRun := make(map[uuid.UUID]*model.DiagnosisSettlement)
	known := make(map[uuid.UUID]bool, len(page.Runs))
	for _, run := range page.Runs {
		if run.UserID != owner || run.ID == uuid.Nil || known[run.ID] {
			return nil, errors.New("diagnosis run list violated owner boundary")
		}
		known[run.ID] = true
	}
	for _, event := range page.TerminalEvents {
		if event.UserID != owner || !known[event.RunID] {
			return nil, errors.New("diagnosis run index contains foreign event")
		}
		eventsByRun[event.RunID] = append(eventsByRun[event.RunID], event)
	}
	for i := range page.Settlements {
		receipt := &page.Settlements[i]
		if receipt.UserID != owner || !known[receipt.RunID] || settlementsByRun[receipt.RunID] != nil {
			return nil, errors.New("diagnosis run index contains foreign settlement")
		}
		settlementsByRun[receipt.RunID] = receipt
	}
	for _, run := range page.Runs {
		summary, err := projectDiagnosisRunStatus(run, eventsByRun[run.ID], settlementsByRun[run.ID])
		if err != nil {
			return nil, err
		}
		result.Runs = append(result.Runs, summary)
	}
	if page.HasMore && (page.NextBeforeRunID == nil || len(result.Runs) == 0 || *page.NextBeforeRunID != result.Runs[len(result.Runs)-1].RunID) {
		return nil, errors.New("diagnosis index page cursor inconsistent")
	}
	return result, nil
}

func (s *NativeDiagnosisRunReadService) Read(ctx context.Context, owner, runID uuid.UUID) (*NativeDiagnosisRunProjection, error) {
	snap, err := s.repo.ReadRunSnapshot(ctx, owner, runID)
	if err != nil {
		return nil, err
	}
	if snap == nil || snap.Run.ID != runID || snap.Run.UserID != owner || snap.CurrentDiagnosisRevision < snap.Run.BasedOnDiagnosisRevision {
		return nil, errors.New("diagnosis run snapshot owner/revision inconsistent")
	}
	summary, err := projectDiagnosisRunStatus(snap.Run, snap.Events, snap.Settlement)
	if err != nil {
		return nil, err
	}
	if summary.CommittedDiagnosisRevision != nil && *summary.CommittedDiagnosisRevision > snap.CurrentDiagnosisRevision {
		return nil, errors.New("run commitment is ahead of owner diagnosis head")
	}
	run := &NativeDiagnosisRunProjection{
		OwnerID: owner, NativeDiagnosisRunSummary: summary, CurrentDiagnosisRevision: snap.CurrentDiagnosisRevision,
		RequestMessageID: snap.Run.RequestMessageID, RequestEvidenceID: snap.Run.RequestEvidenceID,
		RequestSourceVersion: snap.Run.RequestSourceVersion, Scope: snap.Run.Scope,
		InputEvidenceRefs: snap.Run.InputEvidenceRefs, AgentHypotheses: snap.Run.AgentHypotheses,
		Proposals: []NativeDiagnosisRunProposal{}, Events: []NativeDiagnosisRunEvent{},
	}
	if run.Scope.ConditionIDs == nil {
		run.Scope.ConditionIDs = []uuid.UUID{}
	}
	if run.Scope.EvidenceRefs == nil {
		run.Scope.EvidenceRefs = []model.DiagnosisEvidenceRef{}
	}
	if run.InputEvidenceRefs == nil {
		run.InputEvidenceRefs = []model.DiagnosisEvidenceRef{}
	}
	if run.AgentHypotheses == nil {
		run.AgentHypotheses = []model.ConditionAgentHypothesis{}
	}
	for i := range run.AgentHypotheses {
		if run.AgentHypotheses[i].EvidenceRefs == nil {
			run.AgentHypotheses[i].EvidenceRefs = []model.DiagnosisEvidenceRef{}
		}
	}
	if snap.Settlement != nil {
		a := snap.Settlement
		run.Settlement = &NativeDiagnosisRunSettlement{
			SettlementID: a.ID, Outcome: a.Outcome, Revision: a.Revision,
			DiagnosisRevisionID: a.DiagnosisRevisionID, ReceiptID: a.ReceiptID, SettledAt: a.SettledAt,
		}
	}
	proposals := make(map[uuid.UUID]int, len(snap.Proposals))
	for _, p := range snap.Proposals {
		if p.UserID != owner || p.RunID != runID || p.ID == uuid.Nil || proposals[p.ID] != 0 ||
			p.BaseRevision != snap.Run.BasedOnDiagnosisRevision || !diagnosisFingerprintPattern.MatchString(p.ContentFingerprint) ||
			len(p.Content) == 0 || !json.Valid(p.Content) || p.Content[0] != '{' {
			return nil, errors.New("diagnosis proposal source/revision inconsistent")
		}
		view := NativeDiagnosisRunProposal{
			ProposalID: p.ID, RunID: p.RunID, TargetConditionID: p.TargetConditionID,
			ChangeKind: p.ChangeKind, Content: json.RawMessage(p.Content), ContentFingerprint: p.ContentFingerprint,
			BaseRevision: p.BaseRevision, EvidenceRefs: p.EvidenceRefs, CounterEvidenceRefs: p.CounterEvidenceRefs,
			SupersedesProposalID: p.SupersedesProposalID, CreatedAt: p.CreatedAt, State: "pending",
		}
		if view.EvidenceRefs == nil {
			view.EvidenceRefs = []model.DiagnosisEvidenceRef{}
		}
		if view.CounterEvidenceRefs == nil {
			view.CounterEvidenceRefs = []model.DiagnosisEvidenceRef{}
		}
		proposals[p.ID] = len(run.Proposals) + 1
		run.Proposals = append(run.Proposals, view)
	}
	for _, p := range snap.Proposals {
		if p.SupersedesProposalID == nil {
			continue
		}
		predecessor, ok := proposals[*p.SupersedesProposalID]
		if !ok || predecessor == proposals[p.ID] || run.Proposals[predecessor-1].SupersededByProposalID != nil {
			return nil, errors.New("diagnosis proposal supersession lineage inconsistent")
		}
		id := p.ID
		run.Proposals[predecessor-1].SupersededByProposalID = &id
		run.Proposals[predecessor-1].State = "superseded"
	}
	decided := make(map[uuid.UUID]bool, len(snap.Decisions))
	recordedDecisionEvents := make(map[uuid.UUID]bool, len(snap.Decisions))
	for _, d := range snap.Decisions {
		i, ok := proposals[d.ProposalID]
		if !ok || d.UserID != owner || d.RunID != runID || d.ID == uuid.Nil || d.ReceiptID == uuid.Nil ||
			decided[d.ProposalID] || d.DecidedAt.IsZero() || (d.Decision != model.DiagnosisAccept && d.Decision != model.DiagnosisReject) ||
			d.ExpectedRevision != snap.Run.BasedOnDiagnosisRevision ||
			d.ContentFingerprint != run.Proposals[i-1].ContentFingerprint {
			return nil, errors.New("diagnosis decision source/revision inconsistent")
		}
		decided[d.ProposalID] = true
		run.Proposals[i-1].Decision = &NativeDiagnosisRunDecision{
			DecisionID: d.ID, Decision: d.Decision, ContentFingerprint: d.ContentFingerprint,
			ReceiptID: d.ReceiptID, DecidedAt: d.DecidedAt,
		}
		if run.Proposals[i-1].State == "pending" {
			run.Proposals[i-1].State = "decided"
		}
	}
	var lastSeq int64
	for _, event := range snap.Events {
		if event.UserID != owner || event.RunID != runID || event.Sequence != lastSeq+1 {
			return nil, errors.New("diagnosis audit sequence inconsistent")
		}
		lastSeq = event.Sequence
		e := NativeDiagnosisRunEvent{
			Sequence: event.Sequence, Kind: event.Kind, ProposalID: event.ProposalID,
			DecisionID: event.DecisionID, RecordedAt: event.RecordedAt,
		}
		// Event reason is a narrow allowlisted interpretation, not a replay
		// payload or free-form Agent text inadvertently returned to the client.
		if event.Kind == "proposal_superseded" || event.Kind == "proposal_expired" {
			reason, err := diagnosisEventReason(json.RawMessage(event.Details))
			if err != nil {
				return nil, err
			}
			e.Reason = reason
			if event.ProposalID == nil {
				return nil, errors.New("proposal event missing proposal ID")
			}
			i, exists := proposals[*event.ProposalID]
			if !exists {
				return nil, errors.New("proposal event refers to missing proposal")
			}
			if event.Kind == "proposal_superseded" {
				run.Proposals[i-1].State = "superseded"
			} else {
				run.Proposals[i-1].State = "expired"
			}
			run.Proposals[i-1].Reason = reason
		}
		if event.Kind == "decision_recorded" {
			if event.ProposalID == nil || event.DecisionID == nil {
				return nil, errors.New("decision audit incomplete")
			}
			i, exists := proposals[*event.ProposalID]
			if !exists || recordedDecisionEvents[*event.DecisionID] || run.Proposals[i-1].Decision == nil || run.Proposals[i-1].Decision.DecisionID != *event.DecisionID {
				return nil, errors.New("orphan decision audit")
			}
			recordedDecisionEvents[*event.DecisionID] = true
		}
		run.Events = append(run.Events, e)
	}
	for _, p := range run.Proposals {
		if p.Decision != nil && !recordedDecisionEvents[p.Decision.DecisionID] {
			return nil, errors.New("decision receipt lacks an immutable recorded event")
		}
	}
	if run.State != "open" {
		for i := range run.Proposals {
			if run.Proposals[i].State == "pending" {
				// A terminal Safety event from Record deliberately has no
				// settlement/expiry event, but still prevents future selection.
				if run.Outcome != nil && *run.Outcome == "safety_blocked" {
					run.Proposals[i].State = "expired"
					reason := "safety_blocked"
					run.Proposals[i].Reason = &reason
				} else {
					return nil, errors.New("terminal run has unexpired proposal")
				}
			}
		}
	}
	if s.freshness != nil && run.State == "open" {
		freshness, err := s.freshness.EvaluateRun(ctx, owner, run)
		if err != nil {
			return nil, err
		}
		if freshness == nil {
			return nil, errors.New("open diagnosis run freshness missing")
		}
		run.Freshness = freshness
	}
	return run, nil
}

func projectDiagnosisRunStatus(run model.ConditionAnalysisRun, events []model.ConditionAnalysisEvent, settlement *model.DiagnosisSettlement) (NativeDiagnosisRunSummary, error) {
	summary := NativeDiagnosisRunSummary{
		RunID: run.ID, Trigger: run.Trigger, BasedOnDiagnosisRevision: run.BasedOnDiagnosisRevision,
		BasedOnBodyStateRevision: run.BasedOnBodyStateRevision, StartedAt: run.StartedAt, State: "open",
	}
	if run.ID == uuid.Nil || run.UserID == uuid.Nil || run.BasedOnDiagnosisRevision < 0 || run.BasedOnBodyStateRevision < 0 || run.StartedAt.IsZero() {
		return summary, errors.New("invalid formal analysis run metadata")
	}
	var terminal string
	for _, ev := range events {
		if ev.UserID != run.UserID || ev.RunID != run.ID {
			return summary, errors.New("foreign event in formal run")
		}
		switch ev.Kind {
		case "committed", "closed_without_change", "safety_blocked", "insufficient_evidence":
			if terminal != "" {
				return summary, fmt.Errorf("multiple terminal run outcomes")
			}
			terminal = ev.Kind
		}
	}
	if settlement != nil {
		if settlement.UserID != run.UserID || settlement.RunID != run.ID ||
			settlement.ID == uuid.Nil || settlement.ReceiptID == uuid.Nil || settlement.ExpectedRevision != run.BasedOnDiagnosisRevision ||
			settlement.Outcome != terminal || settlement.SettledAt.IsZero() {
			return summary, errors.New("formal run terminal settlement inconsistent")
		}
		if terminal == "committed" {
			if settlement.DiagnosisRevisionID == nil || *settlement.DiagnosisRevisionID == uuid.Nil || settlement.Revision != run.BasedOnDiagnosisRevision+1 {
				return summary, errors.New("committed diagnosis revision missing")
			}
			revision := settlement.Revision
			summary.CommittedDiagnosisRevision = &revision
		} else if settlement.DiagnosisRevisionID != nil || settlement.Revision != run.BasedOnDiagnosisRevision {
			return summary, errors.New("nonadoption settlement claimed revision")
		}
	} else if terminal == "committed" || terminal == "closed_without_change" {
		return summary, errors.New("terminal diagnosis has no durable settlement")
	}
	if terminal != "" {
		outcome := terminal
		summary.Outcome = &outcome
		if terminal == "committed" {
			summary.State = "committed"
		} else {
			summary.State = "closed_without_change"
		}
	}
	return summary, nil
}

func diagnosisEventReason(details json.RawMessage) (*string, error) {
	if len(details) == 0 {
		return nil, nil
	}
	var data struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(details, &data); err != nil {
		return nil, fmt.Errorf("invalid diagnosis audit details: %w", err)
	}
	switch data.Reason {
	case "timeout", "base_revision_changed", "run_closed", "safety_blocked", "revised_after_supplement", "evidence_changed":
		s := data.Reason
		return &s, nil
	default:
		return nil, nil
	}
}
