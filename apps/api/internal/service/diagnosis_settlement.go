package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrDiagnosisChoicesIncomplete = errors.New("diagnosis proposals still require a user decision")
var ErrDiagnosisRecheckRequired = errors.New("diagnosis decisions require evidence recheck or renewed confirmation")
var ErrDiagnosisSettlementInvalid = errors.New("invalid exact diagnosis settlement content")
var ErrDiagnosisMedicalIdentityChanged = errors.New("a different medical identity requires a new condition proposal")

// DiagnosisSettlementCommand explicitly closes a formal run. CloseRemaining
// authorizes expiry, never rejection, of its still-pending proposals. No caller
// can supply diagnosis content, a Safety verdict, or another user's identity.
type DiagnosisSettlementCommand struct {
	RunID            uuid.UUID
	ExpectedRevision int64
	ToolCallID       string
	IdempotencyKey   string
	CloseRemaining   bool
}

func (c DiagnosisSettlementCommand) valid() bool {
	return c.RunID != uuid.Nil && c.ExpectedRevision >= 0 &&
		len(c.IdempotencyKey) > 0 && len(c.IdempotencyKey) <= 128 && strings.TrimSpace(c.IdempotencyKey) == c.IdempotencyKey &&
		len(c.ToolCallID) > 0 && len(c.ToolCallID) <= 256 && strings.TrimSpace(c.ToolCallID) == c.ToolCallID
}

type DiagnosisSettlementService struct {
	repo    *repository.DiagnosisRepository
	sources *repository.DiagnosisSourceRepository
	body    *BodyStateService
	tx      *database.TransactionManager
	now     func() time.Time
	// Test-only seam; production always uses the Go-owned live pinned ledger.
	projectSafety func(*BodyStateReadModel) (SafetyEnvelopeV2, error)
}

func NewDiagnosisSettlementService(db *gorm.DB) *DiagnosisSettlementService {
	return &DiagnosisSettlementService{
		repo: repository.NewDiagnosisRepository(db), sources: repository.NewDiagnosisSourceRepository(db),
		body: NewBodyStateService(repository.NewBodyStateRepository(db)), tx: database.NewTransactionManager(db),
		now: func() time.Time { return time.Now().UTC() }, projectSafety: ProjectSafetyEnvelopeV2,
	}
}

// Settle is the only whole-run adoption entry point. It validates every effective
// decision first, then persists all changed projections, one immutable revision,
// one terminal event and one receipt in the same owner-fenced transaction.
func (s *DiagnosisSettlementService) Settle(ctx context.Context, owner uuid.UUID, c DiagnosisSettlementCommand) (*model.DiagnosisSettlement, error) {
	if owner == uuid.Nil {
		return nil, model.ErrDiagnosisNotFound
	}
	if !c.valid() {
		return nil, ErrDiagnosisDecisionInvalid
	}
	encoded, err := json.Marshal(struct {
		Owner     uuid.UUID
		Operation string
		Command   DiagnosisSettlementCommand
	}{owner, "settle", c})
	if err != nil {
		return nil, ErrDiagnosisDecisionInvalid
	}
	sum := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(sum[:])
	var result *model.DiagnosisSettlement
	err = s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		body, head, err := s.repo.LockDecisionFrame(ctx, owner)
		if err != nil {
			return err
		}
		existing, err := s.repo.FindSettlementByKey(ctx, owner, c.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrDiagnosisDecisionConflict
			}
			result = existing
			return nil // immutable receipt replay; no live re-adoption
		}
		run, err := s.repo.GetAnalysisRun(ctx, owner, c.RunID)
		if err != nil {
			return err
		}
		if err = s.repo.LockAnalysisRun(ctx, owner, run.ID); err != nil {
			return err
		}
		closed, err := s.repo.AnalysisClosed(ctx, owner, run.ID)
		if err != nil {
			return err
		}
		if closed {
			return ErrDiagnosisAnalysisClosed
		}
		if c.ExpectedRevision != head.CurrentRevision || c.ExpectedRevision != run.BasedOnDiagnosisRevision {
			return ErrDiagnosisDecisionConflict
		}
		if body.CurrentRevision < run.BasedOnBodyStateRevision {
			return ErrDiagnosisEvidenceUnavailable
		}
		proposals, decisions, events, err := s.repo.ReadRunChoices(ctx, owner, run.ID)
		if err != nil {
			return err
		}
		choices, err := newDiagnosisRunChoices(proposals, decisions, events)
		if err != nil {
			return err
		}
		snapshot, err := s.body.GetBodyState(ctx, owner, 1)
		if err != nil || snapshot == nil || snapshot.CurrentRevision != body.CurrentRevision {
			return ErrDiagnosisSafetyUnavailable
		}
		envelope, err := s.projectSafety(snapshot)
		if err != nil {
			return ErrDiagnosisSafetyUnavailable
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		result = &model.DiagnosisSettlement{UserID: owner, ID: uuid.New(), RunID: run.ID, ExpectedRevision: c.ExpectedRevision, Revision: c.ExpectedRevision,
			Outcome: "closed_without_change", ConditionIDs: []uuid.UUID{}, DecisionIDs: choices.decisionIDs(), BodyStateRevision: body.CurrentRevision,
			IdempotencyKey: c.IdempotencyKey, RequestFingerprint: fingerprint, ToolCallID: c.ToolCallID, ReceiptID: uuid.New(), SettledAt: now}
		for _, blocker := range envelope.ActiveBlockers {
			if blocker.RiskLevel == "urgent" {
				// Terminal Safety audit survives, but no existing choice becomes an
				// adopted condition. Expiry and terminal receipt are one transaction.
				result.Outcome = "safety_blocked"
				if err := s.expirePending(ctx, owner, *run, choices, "safety_blocked", body.CurrentRevision, now); err != nil {
					return err
				}
				if err := s.repo.InsertSettlement(ctx, result); err != nil {
					return err
				}
				return s.repo.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: owner, RunID: run.ID, Kind: "safety_blocked", BodyStateRevision: body.CurrentRevision, Details: datatypes.JSON(`{"reason":"active_urgent_safety_restriction"}`), RecordedAt: now})
			}
		}
		if !envelope.Coverage.Complete {
			return ErrDiagnosisSafetyUnavailable
		}
		if err := s.sources.AnalysisRequest(ctx, owner, *run); err != nil {
			return diagnosisEvidenceError(err)
		}
		if len(choices.pending) > 0 && !c.CloseRemaining {
			return ErrDiagnosisChoicesIncomplete
		}
		if len(proposals) == 0 && !c.CloseRemaining {
			return ErrDiagnosisChoicesIncomplete
		}
		// Unrelated BodyState revisions alone do not invalidate a diagnosis:
		// recheck the precise current source versions and explicit run addenda.
		rechecks, err := s.recheckChoices(ctx, owner, *run, choices, body.CurrentRevision)
		if err != nil {
			return err
		}
		plan, err := s.planSettlement(ctx, owner, *run, choices, head.CurrentRevision, body.CurrentRevision, now)
		if err != nil {
			return err
		}
		if err := s.expirePending(ctx, owner, *run, choices, "run_closed", body.CurrentRevision, now); err != nil {
			return err
		}
		for _, recheck := range rechecks {
			data, err := json.Marshal(recheck.details)
			if err != nil {
				return ErrDiagnosisSettlementInvalid
			}
			if err := s.repo.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: owner, RunID: run.ID, Kind: "decision_rechecked", ProposalID: &recheck.proposal, DecisionID: &recheck.decision, BodyStateRevision: body.CurrentRevision, Details: datatypes.JSON(data), RecordedAt: now}); err != nil {
				return err
			}
		}
		if len(plan.patch) > 0 {
			patch, err := json.Marshal(plan.patch)
			if err != nil {
				return ErrDiagnosisSettlementInvalid
			}
			refs := settlementCheckedRefs(rechecks)
			revision := &model.DiagnosisRevision{UserID: owner, ID: uuid.New(), PreviousRevision: head.CurrentRevision, Revision: head.CurrentRevision + 1, RunID: &run.ID,
				Patch: datatypes.JSON(patch), DecisionIDs: result.DecisionIDs, BodyStateRevision: body.CurrentRevision, EvidenceRefs: refs,
				ReplayInput: run.ReplayInput, ExecutionProvenance: run.ExecutionProvenance, IdempotencyKey: c.IdempotencyKey, RequestFingerprint: fingerprint, ReceiptID: result.ReceiptID, CommittedAt: now}
			if err := s.repo.InsertSettledRevision(ctx, revision); err != nil {
				return err
			}
			if err := s.writeSettlementPlan(ctx, owner, plan); err != nil {
				return err
			}
			if err := s.repo.AdvanceDiagnosisHead(ctx, owner, head.CurrentRevision, revision.Revision); err != nil {
				return err
			}
			result.Outcome = "committed"
			result.Revision = revision.Revision
			result.DiagnosisRevisionID = &revision.ID
			result.ConditionIDs = plan.conditionIDs()
		}
		if err := s.repo.InsertSettlement(ctx, result); err != nil {
			return err
		}
		details, err := json.Marshal(map[string]any{"outcome": result.Outcome, "diagnosisRevision": result.Revision, "settlementId": result.ID, "receiptId": result.ReceiptID})
		if err != nil {
			return err
		}
		return s.repo.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: owner, RunID: run.ID, Kind: result.Outcome, BodyStateRevision: body.CurrentRevision, Details: datatypes.JSON(details), RecordedAt: now})
	})
	if err != nil {
		return nil, err
	}
	if result.Outcome == "safety_blocked" {
		return result, ErrDiagnosisSafetyBlocked
	}
	return result, nil
}

func (s *DiagnosisSettlementService) expirePending(ctx context.Context, owner uuid.UUID, run model.ConditionAnalysisRun, choices diagnosisRunChoices, reason string, bodyRevision int64, now time.Time) error {
	data, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return err
	}
	for _, id := range choices.pending {
		id := id
		if err := s.repo.AppendAnalysisEvent(ctx, &model.ConditionAnalysisEvent{UserID: owner, RunID: run.ID, Kind: "proposal_expired", ProposalID: &id, BodyStateRevision: bodyRevision, Details: datatypes.JSON(data), RecordedAt: now}); err != nil {
			return err
		}
	}
	return nil
}
