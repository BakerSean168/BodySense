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
	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// These errors are deliberately owner-masked. Slice 2 maps NotFound to HTTP 404.
var ErrPlanNotFound = repository.ErrIndependentPlanNotFound
var ErrPlanConflict = repository.ErrIndependentPlanConflict
var ErrPlanInvalid = errors.New("invalid plan proposal or user decision")
var ErrPlanContextUnavailable = errors.New("plan acceptance context unavailable")

type PlanAdmissionError struct{ Verdict trainingplan.Verdict }

func (e *PlanAdmissionError) Error() string { return string(e.Verdict) }

type PlanProposalInput struct {
	Draft              model.TrainingPlanDraft
	PlanID             uuid.UUID
	ExpectedRevisionID uuid.UUID
	InitiatedBy        trainingplan.Initiator
	SourceID           string
	IdempotencyKey     string
	// Generation is set only by the Go Agent generation path, never from a
	// request body. It is excluded from the user request fingerprint.
	Generation *PlanGeneration `json:"-"`
}

// PlanGeneration is the Go-built audit of one Agent generation. The command
// fingerprint binds the user's generation command (not the Agent output), so a
// retried key replays the stored proposal instead of a new Agent result.
type PlanGeneration struct {
	CommandFingerprint      string
	FrozenBodyStateRevision int64
	Provenance              json.RawMessage
	ReplayInput             json.RawMessage
}

// planProposalRequestFingerprint is what an idempotency key is bound to: the
// whole user input, or the user's generation command for an Agent proposal.
func planProposalRequestFingerprint(input PlanProposalInput) string {
	if input.Generation != nil {
		return planGenerationRequestFingerprint(input.Generation.CommandFingerprint)
	}
	return planDigest(input)
}

func planGenerationRequestFingerprint(command string) string {
	return planDigest(struct {
		Kind    string
		Command string
	}{"agent_generation", command})
}

func validPlanGeneration(input PlanProposalInput) bool {
	g := input.Generation
	if g == nil {
		return true
	}
	return input.InitiatedBy == trainingplan.InitiatorAgent && input.PlanID == uuid.Nil &&
		validPlanHexDigest(g.CommandFingerprint) && g.FrozenBodyStateRevision >= 0 &&
		planJSONObject(g.Provenance) && planJSONObject(g.ReplayInput)
}

func validPlanHexDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}

func planJSONObject(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	return len(raw) > 0 && json.Unmarshal(raw, &object) == nil && object != nil
}

// PlanDecision is an explicit authenticated user's manual command. ActorUserID
// is supplied separately by the trusted caller, never by an Agent payload.
// DecisionID identifies this user action; the receipt is its durable source.
type PlanDecision struct {
	ProposalID           uuid.UUID
	DecisionID           uuid.UUID
	IdempotencyKey       string
	PresentedFingerprint string
	TargetPlanID         uuid.UUID
	ExpectedRevisionID   uuid.UUID
}
type PlanAcceptance struct {
	Receipt    model.PlanDecisionReceipt
	PlanID     uuid.UUID
	RevisionID uuid.UUID
	// Read-only current execution projection. Saving a plan never grants a
	// Trainer action; every start/continue must evaluate the gate again.
	Gate safety.Decision
}

// IndependentTrainingPlanService is the slice 1 application entry point used
// directly by acceptance tests. HTTP and generation adapters follow in slice 2.
// It is independent of Treatment and requires no feature flag.
type IndependentTrainingPlanService struct {
	repo     *repository.IndependentTrainingPlanRepository
	owner    *repository.TrainingRepository
	sessions *repository.TrainerSessionRepository
	body     *BodyStateService
	tx       *database.TransactionManager
	now      func() time.Time
}

func NewIndependentTrainingPlanService(db *gorm.DB) *IndependentTrainingPlanService {
	return &IndependentTrainingPlanService{
		repo: repository.NewIndependentTrainingPlanRepository(db), owner: repository.NewTrainingRepository(db),
		sessions: repository.NewTrainerSessionRepository(db),
		body:     NewBodyStateService(repository.NewBodyStateRepository(db)), tx: database.NewTransactionManager(db), now: func() time.Time { return time.Now().UTC() },
	}
}
func planDigest(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	} // only validated Go values and DB JSON reach here
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
func validPlanKey(key string) bool {
	return len(key) > 0 && len(key) <= 128 && strings.TrimSpace(key) == key
}
func (s *IndependentTrainingPlanService) lock(ctx context.Context, owner uuid.UUID) error {
	err := s.owner.LockOwnerForPlanActivation(ctx, owner)
	if errors.Is(err, repository.ErrTrainingPlanOwnerNotFound) {
		return ErrPlanNotFound
	}
	return err
}
func (s *IndependentTrainingPlanService) currentContext(ctx context.Context, owner, plan uuid.UUID, conditions []uuid.UUID) (trainingplan.CurrentContext, error) {
	current, _, err := s.currentContextAtRevision(ctx, owner, plan, conditions)
	return current, err
}

// currentContextAtRevision also returns the locked BodyState revision.
func (s *IndependentTrainingPlanService) currentContextAtRevision(ctx context.Context, owner, plan uuid.UUID, conditions []uuid.UUID) (trainingplan.CurrentContext, int64, error) {
	revision, diagnosis, err := s.repo.LockContext(ctx, owner, conditions)
	if err != nil {
		if errors.Is(err, ErrPlanNotFound) {
			return trainingplan.CurrentContext{}, 0, ErrPlanNotFound
		}
		return trainingplan.CurrentContext{}, 0, fmt.Errorf("%w: %w", ErrPlanContextUnavailable, err)
	}
	snapshot, err := s.body.GetBodyState(ctx, owner, 1)
	if err != nil || snapshot == nil || snapshot.CurrentRevision != revision {
		return trainingplan.CurrentContext{}, 0, ErrPlanContextUnavailable
	}
	// Include source availability and the temporal applicability of every
	// restriction. A review point or validity boundary cannot replay old context.
	sort.Slice(snapshot.SafetyScreens, func(i, j int) bool {
		return snapshot.SafetyScreens[i].ID.String() < snapshot.SafetyScreens[j].ID.String()
	})
	now := s.now()
	timing := []bool{}
	for _, restriction := range snapshot.SafetyRestrictions {
		timing = append(timing, !now.Before(restriction.ValidFrom))
		if restriction.ValidUntil != nil {
			timing = append(timing, !now.Before(*restriction.ValidUntil))
		}
	}
	current := trainingplan.CurrentContext{UserID: owner, TargetPlanID: plan, Available: true, RelevantContextFingerprint: planDigest(struct {
		Owner        uuid.UUID
		BodyRevision int64
		Diagnosis    json.RawMessage
		Screens      []model.SafetyScreen
		Restrictions []model.SafetyRestrictionRecord
		Timing       []bool
	}{owner, revision, diagnosis, snapshot.SafetyScreens, snapshot.SafetyRestrictions, timing})}

	if plan != uuid.Nil {
		row, err := s.repo.Plan(ctx, owner, plan)
		if err != nil {
			return current, revision, err
		}
		current.CurrentAcceptedPlanRevisionID = *row.CurrentAcceptedRevisionID
	}
	return current, revision, nil
}
func (s *IndependentTrainingPlanService) Propose(ctx context.Context, owner uuid.UUID, input PlanProposalInput) (*model.PlanProposal, error) {
	if !validPlanKey(input.IdempotencyKey) || strings.TrimSpace(input.SourceID) == "" || (input.InitiatedBy != trainingplan.InitiatorUser && input.InitiatedBy != trainingplan.InitiatorAgent) {
		return nil, ErrPlanInvalid
	}
	if err := validatePlanDraft(input.Draft); err != nil {
		return nil, err
	}
	if (input.PlanID == uuid.Nil) != (input.ExpectedRevisionID == uuid.Nil) || !validPlanGeneration(input) {
		return nil, ErrPlanInvalid
	}
	requestFingerprint := planProposalRequestFingerprint(input)
	var proposal *model.PlanProposal
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.ProposalByKey(ctx, owner, input.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != requestFingerprint || (existing.GenerationProvenance != nil) != (input.Generation != nil) {
				return ErrPlanConflict
			}
			proposal = existing
			return nil
		}
		current, bodyRevision, err := s.currentContextAtRevision(ctx, owner, input.PlanID, input.Draft.RelatedConditionIDs)
		if err != nil {
			return err
		}
		if current.CurrentAcceptedPlanRevisionID != input.ExpectedRevisionID {
			return ErrPlanConflict
		}
		// The Agent saw a frozen BodyState; any change while it ran is stale.
		if input.Generation != nil && input.Generation.FrozenBodyStateRevision != bodyRevision {
			return ErrPlanConflict
		}
		if err := s.repo.ValidateBodyRefs(ctx, owner, input.Draft.RelatedBodyRefs); err != nil {
			return err
		}
		raw, err := json.Marshal(input.Draft)
		if err != nil {
			return ErrPlanInvalid
		}
		now := s.now()
		proposal = &model.PlanProposal{ID: uuid.New(), UserID: owner, Kind: trainingplan.ProposalCreate, State: trainingplan.ProposalPending, InitiatedBy: input.InitiatedBy, SourceID: input.SourceID, Draft: raw, ContentFingerprint: planDigest(input.Draft), RelevantContextFingerprint: current.RelevantContextFingerprint, CreatedAt: now, ExpiresAt: now.Add(trainingplan.ProposalTTL), IdempotencyKey: input.IdempotencyKey, RequestFingerprint: requestFingerprint}
		if input.Generation != nil {
			proposal.GenerationProvenance = input.Generation.Provenance
			proposal.ReplayInput = input.Generation.ReplayInput
		}
		if input.PlanID != uuid.Nil {
			proposal.Kind = trainingplan.ProposalUpdate
			proposal.PlanID = &input.PlanID
			proposal.BasedOnRevisionID = &input.ExpectedRevisionID
		}
		return s.repo.InsertProposal(ctx, proposal)
	})
	if err != nil {
		return nil, err
	}
	return proposal, nil
}

// ProposalByIdempotencyKey reads the owner's proposal stored under key, or nil.
// The generation path uses it to replay a retried command before any Agent call.
func (s *IndependentTrainingPlanService) ProposalByIdempotencyKey(ctx context.Context, owner uuid.UUID, key string) (*model.PlanProposal, error) {
	if !validPlanKey(key) {
		return nil, ErrPlanInvalid
	}
	return s.repo.ProposalByKey(ctx, owner, key)
}

func proposalSnapshot(p *model.PlanProposal) trainingplan.Proposal {
	result := trainingplan.Proposal{ID: p.ID, UserID: p.UserID, Kind: p.Kind, State: p.State, InitiatedBy: p.InitiatedBy, SourceID: p.SourceID, ContentFingerprint: p.ContentFingerprint, RelevantContextFingerprint: p.RelevantContextFingerprint, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt}
	if p.PlanID != nil {
		result.PlanID = *p.PlanID
	}
	if p.BasedOnRevisionID != nil {
		result.BasedOnRevisionID = *p.BasedOnRevisionID
	}
	return result
}
func (s *IndependentTrainingPlanService) Accept(ctx context.Context, owner uuid.UUID, decision PlanDecision) (*PlanAcceptance, error) {
	if !validPlanKey(decision.IdempotencyKey) || decision.DecisionID == uuid.Nil || decision.ProposalID == uuid.Nil {
		return nil, ErrPlanInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Decision  PlanDecision
	}{"accept", decision})
	var result *PlanAcceptance
	var denied error
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		receipt, err := s.repo.Receipt(ctx, owner, decision.IdempotencyKey)
		if err != nil {
			return err
		}
		if receipt != nil {
			if receipt.Fingerprint != fingerprint {
				return ErrPlanConflict
			}
			result = &PlanAcceptance{Receipt: *receipt, PlanID: *receipt.PlanID, RevisionID: *receipt.RevisionID}
			// Replay returns the original immutable mutation identity, plus a freshly
			// evaluated gate. It cannot replay a historical allow after new risk.
			rev, err := s.repo.Revision(ctx, owner, result.PlanID, result.RevisionID)
			if err != nil {
				return err
			}
			var draft model.TrainingPlanDraft
			if err = json.Unmarshal(rev.Snapshot, &draft); err != nil {
				return err
			}
			if _, err = s.currentContext(ctx, owner, result.PlanID, draft.RelatedConditionIDs); err != nil {
				return err
			}
			result.Gate = s.gate(ctx, owner, result.PlanID, draft, s.now())
			return nil
		}
		p, err := s.repo.Proposal(ctx, owner, decision.ProposalID)
		if err != nil {
			return err
		}
		if decision.TargetPlanID != uuid.Nil {
			if _, err := s.repo.Plan(ctx, owner, decision.TargetPlanID); err != nil {
				return err
			}
			if decision.ExpectedRevisionID != uuid.Nil {
				if _, err := s.repo.Revision(ctx, owner, decision.TargetPlanID, decision.ExpectedRevisionID); err != nil {
					return err
				}
			}
		}
		var draft model.TrainingPlanDraft
		if err = json.Unmarshal(p.Draft, &draft); err != nil {
			return ErrPlanInvalid
		}
		if err = validatePlanDraft(draft); err != nil {
			return err
		}
		if planDigest(draft) != p.ContentFingerprint {
			return ErrPlanInvalid
		}
		target := uuid.Nil
		if p.PlanID != nil {
			target = *p.PlanID
		}
		current, err := s.currentContext(ctx, owner, target, draft.RelatedConditionIDs)
		if err != nil {
			return err
		}
		now := s.now()
		verdict := trainingplan.EvaluateAcceptance(proposalSnapshot(p), trainingplan.UserAcceptance{ProposalID: decision.ProposalID, ActorUserID: owner, DecisionID: decision.DecisionID, DecisionSourceID: decision.DecisionID.String(), DecisionKind: "accept", PresentedFingerprint: decision.PresentedFingerprint, TargetPlanID: decision.TargetPlanID, ExpectedRevisionID: decision.ExpectedRevisionID, IdempotencyKey: decision.IdempotencyKey}, current, now)
		if verdict != trainingplan.EligibleForCommit {
			denied = &PlanAdmissionError{Verdict: verdict}
			reason := ""
			switch verdict {
			case trainingplan.ProposalTimedOut:
				reason = "timeout"
			case trainingplan.ContextChanged:
				reason = "relevant_context_changed"
			case trainingplan.PlanRevisionChanged:
				reason = "base_revision_changed"
			}
			if reason != "" {
				p.State = trainingplan.ProposalExpired
				p.ExpiredReason = &reason
				return s.repo.FinishProposal(ctx, p)
			}
			return denied
		}
		if err = s.repo.ValidateBodyRefs(ctx, owner, draft.RelatedBodyRefs); err != nil {
			return err
		}
		planID := target
		if planID == uuid.Nil {
			planID = uuid.New()
		}
		gate := s.gate(ctx, owner, planID, draft, now)
		// A gate projection is required, but restricted execution does not prohibit
		// saving a prescription (training.md 4.5). Unreadable ledger is not allow.
		receipt = &model.PlanDecisionReceipt{ID: uuid.New(), UserID: owner, ProposalID: p.ID, DecisionID: decision.DecisionID, Operation: "accept", IdempotencyKey: decision.IdempotencyKey, Fingerprint: fingerprint, CreatedAt: now}
		rev := &model.TrainingPlanRevision{ID: uuid.New(), UserID: owner, PlanID: planID, ParentRevisionID: p.BasedOnRevisionID, Snapshot: p.Draft, SourceKind: "user_accepted_proposal", SourceID: p.ID.String(), AuthorizationID: receipt.ID, CreatedAt: now}
		// A whole-document replace is an exact, verifiable change set; no implicit
		// changes are generated by the acceptance path.
		changes, _ := json.Marshal([]any{map[string]any{"op": "replace", "path": "", "value": draft}})
		rev.ChangeSet = changes
		plan := &model.TrainingPlan{ID: planID, UserID: owner, Title: draft.Title, Goal: draft.Goal, Status: "active", CurrentAcceptedRevisionID: &rev.ID, Phases: json.RawMessage(`[]`), CreatedAt: now}
		if err = s.repo.CommitRevision(ctx, plan, rev, draft, target == uuid.Nil); err != nil {
			return err
		}
		receipt.PlanID = &planID
		receipt.RevisionID = &rev.ID
		if err = s.repo.InsertReceipt(ctx, receipt); err != nil {
			return err
		}
		p.State = trainingplan.ProposalAccepted
		p.DecisionID = &decision.DecisionID
		if err = s.repo.FinishProposal(ctx, p); err != nil {
			return err
		}
		result = &PlanAcceptance{Receipt: *receipt, PlanID: planID, RevisionID: rev.ID, Gate: gate}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if denied != nil {
		return nil, denied
	}
	return result, nil
}
func (s *IndependentTrainingPlanService) gate(ctx context.Context, owner, plan uuid.UUID, draft model.TrainingPlanDraft, now time.Time) safety.Decision {
	action := safety.Action{Kind: safety.ActionStartSession, PlanID: plan}
	for _, session := range draft.Content.Sessions {
		for _, stage := range session.Stages {
			for _, exercise := range stage.Exercises {
				action.Exercises = append(action.Exercises, safety.Exercise{ExerciseID: exercise.ExerciseID, Custom: strings.HasPrefix(exercise.ExerciseID, "custom:")})
			}
		}
	}
	return s.body.EvaluateTrainerGate(ctx, owner, action, now)
}
func (s *IndependentTrainingPlanService) Reject(ctx context.Context, owner uuid.UUID, decision PlanDecision) (*model.PlanDecisionReceipt, error) {
	if !validPlanKey(decision.IdempotencyKey) || decision.DecisionID == uuid.Nil || decision.ProposalID == uuid.Nil {
		return nil, ErrPlanInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Decision  PlanDecision
	}{"reject", decision})
	var receipt *model.PlanDecisionReceipt
	var denied error
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		var err error
		receipt, err = s.repo.Receipt(ctx, owner, decision.IdempotencyKey)
		if err != nil {
			return err
		}
		if receipt != nil {
			if receipt.Fingerprint != fingerprint {
				return ErrPlanConflict
			}
			return nil
		}
		p, err := s.repo.Proposal(ctx, owner, decision.ProposalID)
		if err != nil {
			return err
		}
		if decision.TargetPlanID != uuid.Nil {
			if _, err := s.repo.Plan(ctx, owner, decision.TargetPlanID); err != nil {
				return err
			}
			if decision.ExpectedRevisionID != uuid.Nil {
				if _, err := s.repo.Revision(ctx, owner, decision.TargetPlanID, decision.ExpectedRevisionID); err != nil {
					return err
				}
			}
		}
		if p.State != trainingplan.ProposalPending {
			return ErrPlanConflict
		}
		if !s.now().Before(p.ExpiresAt) {
			reason := "timeout"
			p.State = trainingplan.ProposalExpired
			p.ExpiredReason = &reason
			denied = &PlanAdmissionError{Verdict: trainingplan.ProposalTimedOut}
			return s.repo.FinishProposal(ctx, p)
		}
		snapshot := proposalSnapshot(p)
		if decision.PresentedFingerprint != p.ContentFingerprint || decision.TargetPlanID != snapshot.PlanID || decision.ExpectedRevisionID != snapshot.BasedOnRevisionID {
			return ErrPlanConflict
		}
		receipt = &model.PlanDecisionReceipt{ID: uuid.New(), UserID: owner, ProposalID: p.ID, DecisionID: decision.DecisionID, Operation: "reject", IdempotencyKey: decision.IdempotencyKey, Fingerprint: fingerprint, CreatedAt: s.now()}
		if err = s.repo.InsertReceipt(ctx, receipt); err != nil {
			return err
		}
		p.State = trainingplan.ProposalRejected
		p.DecisionID = &decision.DecisionID
		return s.repo.FinishProposal(ctx, p)
	})
	if err != nil {
		return nil, err
	}
	if denied != nil {
		return nil, denied
	}
	return receipt, nil
}
func (s *IndependentTrainingPlanService) List(ctx context.Context, owner uuid.UUID) ([]model.TrainingPlan, error) {
	return s.repo.List(ctx, owner)
}
func (s *IndependentTrainingPlanService) Get(ctx context.Context, owner, plan uuid.UUID) (*model.TrainingPlan, error) {
	return s.repo.Plan(ctx, owner, plan)
}
func (s *IndependentTrainingPlanService) Revision(ctx context.Context, owner, plan, revision uuid.UUID) (*model.TrainingPlanRevision, error) {
	return s.repo.Revision(ctx, owner, plan, revision)
}
