// Package trainingplan contains pure precommit rules for the independent
// TrainingPlan V2 domain. It does not persist decisions or authorize users.
package trainingplan

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProposalTTL applies only to Training PlanProposal, never Diagnosis proposals.
// An earlier explicit expiry or a changed authoritative context can block an
// acceptance sooner.
const ProposalTTL = 7 * 24 * time.Hour

type ProposalKind string

const (
	ProposalCreate ProposalKind = "create"
	ProposalUpdate ProposalKind = "update"
)

type ProposalState string

const (
	ProposalPending  ProposalState = "pending"
	ProposalAccepted ProposalState = "accepted"
	ProposalRejected ProposalState = "rejected"
	ProposalExpired  ProposalState = "expired"
)

type Initiator string

const (
	InitiatorUser  Initiator = "user"
	InitiatorAgent Initiator = "agent"
)

// Proposal is a read snapshot of an existing persisted, unaccepted proposal.
// The persistence/authorization layers must attest that these values come
// from the authenticated owner's durable proposal, not client-supplied JSON.
type Proposal struct {
	ID                         uuid.UUID
	UserID                     uuid.UUID
	Kind                       ProposalKind
	State                      ProposalState
	InitiatedBy                Initiator
	SourceID                   string
	PlanID                     uuid.UUID
	BasedOnRevisionID          uuid.UUID
	ContentFingerprint         string
	RelevantContextFingerprint string
	CreatedAt                  time.Time
	ExpiresAt                  time.Time
}

// UserAcceptance describes a server-verified explicit decision by its owner.
// A request's identifiers alone are never proof of user authorization.
type UserAcceptance struct {
	ProposalID           uuid.UUID
	ActorUserID          uuid.UUID
	DecisionID           uuid.UUID
	DecisionSourceID     string
	DecisionKind         string
	PresentedFingerprint string
	TargetPlanID         uuid.UUID
	ExpectedRevisionID   uuid.UUID
	IdempotencyKey       string
}

// CurrentContext is an authoritative read snapshot acquired by the same
// transactional owner as proposal acceptance. The fingerprint must cover
// every *relevant* BodyState, Diagnosis, and Safety change; its construction
// and provenance are the responsibility of a future persisted Go adapter.
type CurrentContext struct {
	// UserID and TargetPlanID must be obtained from trusted owner-scoped
	// reads, not from unverified proposal or request JSON. Creating a new
	// plan has no TargetPlanID even if other active plans already exist.
	UserID                        uuid.UUID
	TargetPlanID                  uuid.UUID
	Available                     bool
	RelevantContextFingerprint    string
	CurrentAcceptedPlanRevisionID uuid.UUID
}

// Verdict is a read-only precommit decision. EligibleForCommit never means
// accepted or activated: a verified user decision, serialized transaction,
// durable receipt, unique idempotency key and immutable revision write must
// still succeed atomically.
type Verdict string

const (
	EligibleForCommit   Verdict = "eligible_for_transactional_commit"
	ProposalUnavailable Verdict = "proposal_unavailable"
	ProposalNotPending  Verdict = "proposal_not_pending"
	InvalidProposal     Verdict = "invalid_proposal"
	InvalidDecision     Verdict = "invalid_user_decision"
	InvalidExpiry       Verdict = "invalid_expiry"
	ProposalTimedOut    Verdict = "proposal_timed_out"
	ContentChanged      Verdict = "content_fingerprint_mismatch"
	ContextUnavailable  Verdict = "context_unavailable"
	ContextChanged      Verdict = "relevant_context_changed"
	PlanRevisionChanged Verdict = "plan_revision_changed"
)

// EvaluateAcceptance is deterministic and side-effect-free. It cannot accept
// an agent suggestion, an unverified user source or stale medical evidence on
// its own: those must be independently authenticated by the Go write boundary.
func EvaluateAcceptance(
	proposal Proposal,
	decision UserAcceptance,
	current CurrentContext,
	now time.Time,
) Verdict {
	if proposal.ID == uuid.Nil || proposal.UserID == uuid.Nil ||
		decision.ProposalID != proposal.ID ||
		decision.ActorUserID != proposal.UserID {
		// Do not reveal whether another user's plan/proposal exists.
		return ProposalUnavailable
	}
	if proposal.State != ProposalPending {
		// A previously accepted proposal is not a license to run the write
		// again: only a persisted matching idempotency receipt can replay.
		return ProposalNotPending
	}
	if proposal.Kind != ProposalCreate && proposal.Kind != ProposalUpdate {
		return InvalidProposal
	}
	if (proposal.InitiatedBy != InitiatorAgent && proposal.InitiatedBy != InitiatorUser) ||
		strings.TrimSpace(proposal.SourceID) == "" ||
		strings.TrimSpace(proposal.ContentFingerprint) == "" ||
		strings.TrimSpace(proposal.RelevantContextFingerprint) == "" {
		return InvalidProposal
	}
	if decision.DecisionID == uuid.Nil ||
		strings.TrimSpace(decision.DecisionSourceID) == "" ||
		decision.DecisionKind != "accept" ||
		strings.TrimSpace(decision.IdempotencyKey) == "" {
		return InvalidDecision
	}
	if now.IsZero() || proposal.CreatedAt.IsZero() ||
		proposal.ExpiresAt.IsZero() ||
		now.Before(proposal.CreatedAt) ||
		!proposal.ExpiresAt.After(proposal.CreatedAt) ||
		proposal.ExpiresAt.After(proposal.CreatedAt.Add(ProposalTTL)) {
		return InvalidExpiry
	}
	if !now.Before(proposal.ExpiresAt) ||
		!now.Before(proposal.CreatedAt.Add(ProposalTTL)) {
		return ProposalTimedOut
	}
	if decision.PresentedFingerprint != proposal.ContentFingerprint {
		return ContentChanged
	}
	if !current.Available ||
		current.UserID == uuid.Nil || current.UserID != proposal.UserID ||
		strings.TrimSpace(current.RelevantContextFingerprint) == "" {
		// Never let a coincidentally equal digest from another user
		// satisfy evidence and authorization freshness.
		return ContextUnavailable
	}
	if current.RelevantContextFingerprint != proposal.RelevantContextFingerprint {
		return ContextChanged
	}
	switch proposal.Kind {
	case ProposalCreate:
		// Creation has no existing plan identity or base accepted revision.
		if proposal.PlanID != uuid.Nil || proposal.BasedOnRevisionID != uuid.Nil ||
			decision.TargetPlanID != uuid.Nil || decision.ExpectedRevisionID != uuid.Nil {
			return InvalidProposal
		}
		// The context for a new independent plan is user-global.
		// No existing plan's identity/revision may be substituted.
		if current.TargetPlanID != uuid.Nil ||
			current.CurrentAcceptedPlanRevisionID != uuid.Nil {
			return ContextUnavailable
		}
	case ProposalUpdate:
		if proposal.PlanID == uuid.Nil || proposal.BasedOnRevisionID == uuid.Nil {
			return InvalidProposal
		}
		if decision.TargetPlanID != proposal.PlanID ||
			decision.ExpectedRevisionID != proposal.BasedOnRevisionID {
			return InvalidDecision
		}
		if current.TargetPlanID != proposal.PlanID ||
			current.CurrentAcceptedPlanRevisionID != proposal.BasedOnRevisionID {
			// Exact plan identity is required even when two same-owner
			// plans happen to share an upstream context fingerprint.
			return PlanRevisionChanged
		}
	}
	return EligibleForCommit
}
