package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrDiagnosisDecisionConflict = model.ErrDiagnosisDecisionConflict
var ErrDiagnosisAnalysisClosed = model.ErrDiagnosisAnalysisClosed
var ErrDiagnosisDecisionInvalid = errors.New("invalid explicit diagnosis decision")
var ErrDiagnosisEvidenceUnavailable = errors.New("diagnosis decision evidence unavailable")
var ErrDiagnosisSafetyUnavailable = errors.New("diagnosis decision safety coverage unavailable")
var ErrDiagnosisSafetyBlocked = errors.New("diagnosis analysis terminated by urgent safety")

// DiagnosisProposalDecisionCommand is an explicit user's selection of exactly
// the immutable proposal presented by the trusted transport. Owner is supplied
// separately from authentication; Agent payloads cannot choose it. Recording a
// choice is NOT adoption. Atomic settlement is the next bounded DX-002 slice.
type DiagnosisProposalDecisionCommand struct {
	RunID                uuid.UUID
	ProposalID           uuid.UUID
	PresentedFingerprint string
	ExpectedRevision     int64
	Decision             model.DiagnosisDecisionKind
	ToolCallID           string
	IdempotencyKey       string
}

func (c DiagnosisProposalDecisionCommand) valid() bool {
	fingerprint, err := hex.DecodeString(c.PresentedFingerprint)
	return err == nil && len(fingerprint) == 32 && c.PresentedFingerprint == strings.ToLower(c.PresentedFingerprint) &&
		c.RunID != uuid.Nil && c.ProposalID != uuid.Nil && c.ExpectedRevision >= 0 &&
		(c.Decision == model.DiagnosisAccept || c.Decision == model.DiagnosisReject) &&
		len(c.IdempotencyKey) > 0 && len(c.IdempotencyKey) <= 128 && strings.TrimSpace(c.IdempotencyKey) == c.IdempotencyKey &&
		len(c.ToolCallID) > 0 && len(c.ToolCallID) <= 256 && strings.TrimSpace(c.ToolCallID) == c.ToolCallID
}

type DiagnosisProposalDecisionService struct {
	repo    *repository.DiagnosisRepository
	sources *repository.DiagnosisSourceRepository
	body    *BodyStateService
	tx      *database.TransactionManager
	now     func() time.Time
	// Test seam for typed future-policy signals; production always projects
	// the pinned Go-owned BodyState and never accepts caller/Agent risk levels.
	projectSafety func(*BodyStateReadModel) (SafetyEnvelopeV2, error)
}

func NewDiagnosisProposalDecisionService(db *gorm.DB) *DiagnosisProposalDecisionService {
	return &DiagnosisProposalDecisionService{repo: repository.NewDiagnosisRepository(db), sources: repository.NewDiagnosisSourceRepository(db), body: NewBodyStateService(repository.NewBodyStateRepository(db)), tx: database.NewTransactionManager(db), now: func() time.Time { return time.Now().UTC() }, projectSafety: ProjectSafetyEnvelopeV2}
}

// Record atomically appends a choice receipt and its ordered event. No path in
// this service creates a ConditionEntry or DiagnosisRevision. Existing decisions
// survive urgent termination as audit only; they cannot reopen the closed run.
func (s *DiagnosisProposalDecisionService) Record(ctx context.Context, owner uuid.UUID, command DiagnosisProposalDecisionCommand) (*model.DiagnosisDecision, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if !command.valid() {
		return nil, ErrDiagnosisDecisionInvalid
	}
	encoded, err := json.Marshal(struct {
		Owner   uuid.UUID
		Command DiagnosisProposalDecisionCommand
	}{owner, command})
	if err != nil {
		return nil, ErrDiagnosisDecisionInvalid
	}
	sum := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(sum[:])
	var result *model.DiagnosisDecision
	var terminalError error
	err = s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		body, head, err := s.repo.LockDecisionFrame(ctx, owner)
		if err != nil {
			return err
		}
		existing, err := s.repo.FindDecisionByKey(ctx, owner, command.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrDiagnosisDecisionConflict
			}
			// Retry reads the original choice only. It does not re-execute clinical
			// analysis, mutate live state or produce an adoption acknowledgement.
			result = existing
			return nil
		}
		run, err := s.repo.GetAnalysisRun(ctx, owner, command.RunID)
		if err != nil {
			return err
		}
		closed, err := s.repo.AnalysisClosed(ctx, owner, run.ID)
		if err != nil {
			return err
		}
		if closed {
			return ErrDiagnosisAnalysisClosed
		}
		proposal, err := s.repo.GetProposal(ctx, owner, command.ProposalID)
		if err != nil {
			return err
		}
		if proposal.RunID != run.ID {
			return model.ErrDiagnosisNotFound
		}
		if command.ExpectedRevision != head.CurrentRevision || command.ExpectedRevision != run.BasedOnDiagnosisRevision || proposal.BaseRevision != command.ExpectedRevision || proposal.ContentFingerprint != command.PresentedFingerprint {
			return ErrDiagnosisDecisionConflict
		}
		unavailable, err := s.repo.ProposalUnavailable(ctx, owner, run.ID, proposal.ID)
		if err != nil {
			return err
		}
		if unavailable {
			return ErrDiagnosisDecisionConflict
		}
		prior, err := s.repo.FindDecisionForProposal(ctx, owner, proposal.ID)
		if err != nil {
			return err
		}
		if prior != nil {
			return ErrDiagnosisDecisionConflict
		}
		if body.CurrentRevision < run.BasedOnBodyStateRevision {
			return ErrDiagnosisEvidenceUnavailable
		}
		if err := s.sources.AnalysisRequest(ctx, owner, *run); err != nil {
			return diagnosisEvidenceError(err)
		}
		snapshot, err := s.body.GetBodyState(ctx, owner, 1)
		if err != nil || snapshot == nil || snapshot.CurrentRevision != body.CurrentRevision {
			return ErrDiagnosisSafetyUnavailable
		}
		envelope, err := s.projectSafety(snapshot)
		if err != nil {
			return ErrDiagnosisSafetyUnavailable
		}
		for _, blocker := range envelope.ActiveBlockers {
			if blocker.RiskLevel == "urgent" {
				if err := s.repo.BlockAnalysisForUrgent(ctx, owner, run.ID, body.CurrentRevision, s.now()); err != nil {
					return err
				}
				terminalError = ErrDiagnosisSafetyBlocked
				return nil // commit the terminal audit; never commit a diagnosis
			}
		}
		if !envelope.Coverage.Complete {
			return ErrDiagnosisSafetyUnavailable
		}
		refs, err := diagnosisDecidedEvidence(proposal)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if err := s.sources.Evidence(ctx, owner, ref, body.CurrentRevision); err != nil {
				return diagnosisEvidenceError(err)
			}
		}
		result = &model.DiagnosisDecision{UserID: owner, ID: uuid.New(), ProposalID: proposal.ID, RunID: run.ID, ContentFingerprint: proposal.ContentFingerprint, DecidedEvidenceRefs: refs, Decision: command.Decision, ToolCallID: command.ToolCallID, DecidedAt: s.now(), IdempotencyKey: command.IdempotencyKey, RequestFingerprint: fingerprint, ExpectedRevision: command.ExpectedRevision, ReceiptID: uuid.New()}
		return s.repo.InsertDecisionWithEvent(ctx, result, body.CurrentRevision)
	})
	if err != nil {
		return nil, err
	}
	if terminalError != nil {
		return nil, terminalError
	}
	return result, nil
}
func diagnosisEvidenceError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, model.ErrDiagnosisNotFound) {
		return model.ErrDiagnosisNotFound
	}
	return fmt.Errorf("%w: %w", ErrDiagnosisEvidenceUnavailable, err)
}
func diagnosisDecidedEvidence(p *model.DiagnosisProposal) ([]model.DiagnosisEvidenceRef, error) {
	if len(p.EvidenceRefs)+len(p.CounterEvidenceRefs) > 1000 {
		return nil, ErrDiagnosisDecisionInvalid
	}
	refs := make([]model.DiagnosisEvidenceRef, 0, len(p.EvidenceRefs)+len(p.CounterEvidenceRefs))
	seen := map[string]bool{}
	for _, group := range [][]model.DiagnosisEvidenceRef{p.EvidenceRefs, p.CounterEvidenceRefs} {
		for _, ref := range group {
			key := string(ref.EvidenceType) + ":" + ref.EvidenceID.String() + ":" + ref.Version
			if !seen[key] {
				seen[key] = true
				refs = append(refs, ref)
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if a.EvidenceType != b.EvidenceType {
			return a.EvidenceType < b.EvidenceType
		}
		if a.EvidenceID != b.EvidenceID {
			return a.EvidenceID.String() < b.EvidenceID.String()
		}
		return a.Version < b.Version
	})
	return refs, nil
}
