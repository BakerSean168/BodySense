package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Review reasons are read-only projections. They never write plan status,
// content or revision (training.md §4.2: no review_recommended status).
const (
	PlanReviewContextChanged     = "context_changed_since_acceptance"
	PlanReviewContextUnavailable = "context_unavailable"
)

const planProposalListLimit = 200

// PlanReview is a deduplicated, read-only hint that the authoritative context
// the current revision was accepted against has changed. Only a new
// PlanProposal accepted by the user can change the prescription.
type PlanReview struct {
	Recommended bool
	Reason      string
}

// TrainingPlanView is one owned accepted plan with its current immutable
// content, a review hint and a freshly evaluated execution gate. A gate that
// does not permit execution is the "paused for Safety" state; it is computed
// on every read and never stored on the plan.
type TrainingPlanView struct {
	Plan     model.TrainingPlan
	Revision model.TrainingPlanRevision
	Draft    model.TrainingPlanDraft
	Review   PlanReview
	Gate     safety.Decision
}

// PlanStatusCommand is an explicit authenticated user's status change.
type PlanStatusCommand struct {
	PlanID                 uuid.UUID
	Status                 trainingplan.PlanStatus
	ExpectedStatusRevision int64
	IdempotencyKey         string
}

// ChangeStatus applies one user-managed transition under the owner lock with
// an expected status revision. It never touches content, revisions, logs or
// another plan, and does not grant execution: the Safety gate stays separate.
func (s *IndependentTrainingPlanService) ChangeStatus(ctx context.Context, owner uuid.UUID, cmd PlanStatusCommand) (*model.TrainingPlanStatusEvent, error) {
	if cmd.PlanID == uuid.Nil || !cmd.Status.Valid() || cmd.ExpectedStatusRevision < 0 || !validPlanKey(cmd.IdempotencyKey) {
		return nil, ErrPlanInvalid
	}
	fingerprint := planDigest(struct {
		Operation string
		Command   PlanStatusCommand
	}{"status", cmd})
	var event *model.TrainingPlanStatusEvent
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.lock(ctx, owner); err != nil {
			return err
		}
		existing, err := s.repo.StatusEventByKey(ctx, owner, cmd.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return ErrPlanConflict
			}
			event = existing
			return nil
		}
		plan, err := s.repo.Plan(ctx, owner, cmd.PlanID)
		if err != nil {
			return err
		}
		if plan.StatusRevision != cmd.ExpectedStatusRevision {
			return ErrPlanConflict
		}
		from := trainingplan.PlanStatus(plan.Status)
		if !trainingplan.StatusTransitionAllowed(from, cmd.Status) {
			return ErrPlanConflict
		}
		event = &model.TrainingPlanStatusEvent{
			ID: uuid.New(), UserID: owner, PlanID: plan.ID, FromStatus: from, ToStatus: cmd.Status,
			StatusRevision: plan.StatusRevision + 1, SourceKind: "user_command",
			IdempotencyKey: cmd.IdempotencyKey, RequestFingerprint: fingerprint, CreatedAt: s.now(),
		}
		return s.repo.CommitStatus(ctx, event)
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

// View returns one owned plan projection; another owner's plan is not found.
func (s *IndependentTrainingPlanService) View(ctx context.Context, owner, planID uuid.UUID) (*TrainingPlanView, error) {
	plan, err := s.repo.Plan(ctx, owner, planID)
	if err != nil {
		return nil, err
	}
	return s.project(ctx, owner, *plan)
}

// ListViews returns every owned accepted plan, in any status, oldest first.
func (s *IndependentTrainingPlanService) ListViews(ctx context.Context, owner uuid.UUID) ([]TrainingPlanView, error) {
	plans, err := s.repo.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	views := make([]TrainingPlanView, 0, len(plans))
	for _, plan := range plans {
		view, err := s.project(ctx, owner, plan)
		if err != nil {
			return nil, err
		}
		views = append(views, *view)
	}
	return views, nil
}

func (s *IndependentTrainingPlanService) project(ctx context.Context, owner uuid.UUID, plan model.TrainingPlan) (*TrainingPlanView, error) {
	if plan.UserID != owner || plan.CurrentAcceptedRevisionID == nil {
		return nil, ErrPlanNotFound
	}
	rev, err := s.repo.Revision(ctx, owner, plan.ID, *plan.CurrentAcceptedRevisionID)
	if err != nil {
		return nil, err
	}
	var draft model.TrainingPlanDraft
	if err := json.Unmarshal(rev.Snapshot, &draft); err != nil {
		return nil, ErrPlanContextUnavailable
	}
	view := &TrainingPlanView{Plan: plan, Revision: *rev, Draft: draft}
	view.Review = s.review(ctx, owner, plan.ID, *rev, draft)
	// Evaluated now, from the native ledger. Missing/unscreened/unreadable
	// Safety never projects allow, and nothing here writes the plan.
	view.Gate = s.gate(ctx, owner, plan.ID, draft, s.now())
	return view, nil
}

// review compares the context fingerprint the current revision was accepted
// against (equal to its proposal's, enforced by EvaluateAcceptance) with the
// current authoritative one. Freshness is as conservative as acceptance.
func (s *IndependentTrainingPlanService) review(ctx context.Context, owner, planID uuid.UUID, rev model.TrainingPlanRevision, draft model.TrainingPlanDraft) PlanReview {
	proposalID, err := uuid.Parse(rev.SourceID)
	if err != nil {
		return PlanReview{Recommended: true, Reason: PlanReviewContextUnavailable}
	}
	proposal, err := s.repo.Proposal(ctx, owner, proposalID)
	if err != nil {
		return PlanReview{Recommended: true, Reason: PlanReviewContextUnavailable}
	}
	var current trainingplan.CurrentContext
	err = s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		current, err = s.currentContext(ctx, owner, planID, draft.RelatedConditionIDs)
		return err
	})
	if err != nil || !current.Available || current.RelevantContextFingerprint == "" {
		return PlanReview{Recommended: true, Reason: PlanReviewContextUnavailable}
	}
	if current.RelevantContextFingerprint != proposal.RelevantContextFingerprint {
		return PlanReview{Recommended: true, Reason: PlanReviewContextChanged}
	}
	return PlanReview{}
}

// ListProposals lists the owner's proposals, optionally filtered by state.
// Pending proposals past their 7-day TTL are reported as expired on read; the
// durable expiry is written by the next accept/reject attempt.
func (s *IndependentTrainingPlanService) ListProposals(ctx context.Context, owner uuid.UUID, state trainingplan.ProposalState) ([]model.PlanProposal, error) {
	switch state {
	case "", trainingplan.ProposalPending, trainingplan.ProposalAccepted, trainingplan.ProposalRejected, trainingplan.ProposalExpired:
	default:
		return nil, ErrPlanInvalid
	}
	rows, err := s.repo.ListProposals(ctx, owner, "", planProposalListLimit)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]model.PlanProposal, 0, len(rows))
	for _, row := range rows {
		row = projectProposalExpiry(row, now)
		if state == "" || row.State == state {
			out = append(out, row)
		}
	}
	return out, nil
}

// GetProposal returns one owned proposal; another owner's id is not found.
func (s *IndependentTrainingPlanService) GetProposal(ctx context.Context, owner, id uuid.UUID) (*model.PlanProposal, error) {
	row, err := s.repo.Proposal(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	projected := projectProposalExpiry(*row, s.now())
	return &projected, nil
}

func projectProposalExpiry(row model.PlanProposal, now time.Time) model.PlanProposal {
	if row.State == trainingplan.ProposalPending && !now.Before(row.ExpiresAt) {
		reason := "timeout"
		row.State = trainingplan.ProposalExpired
		row.ExpiredReason = &reason
	}
	return row
}

// IsPlanAdmission reports the admission verdict carried by err, if any.
func IsPlanAdmission(err error) (trainingplan.Verdict, bool) {
	var admission *PlanAdmissionError
	if errors.As(err, &admission) {
		return admission.Verdict, true
	}
	return "", false
}
