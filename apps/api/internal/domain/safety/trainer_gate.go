// Package safety contains the pure, Go-owned TrainerGate for the V2 Safety
// domain. It reads a pinned SafetyState snapshot and decides whether one
// Trainer action may proceed. It never persists, lifts or downgrades a
// restriction, never modifies a TrainingPlan and never trusts model output:
// callers must build State only from the authenticated owner's Go ledger.
package safety

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GatePolicyRevision versions the overlap/fail-closed rules below.
const GatePolicyRevision = "trainer-gate-v1"

type RiskLevel string

const (
	RiskAdvisory   RiskLevel = "advisory"
	RiskRestricted RiskLevel = "restricted"
	RiskUrgent     RiskLevel = "urgent"
)

type ScopeKind string

const (
	ScopeAllTraining ScopeKind = "all_training"
	ScopePlan        ScopeKind = "plan"
	ScopeExercise    ScopeKind = "exercise"
)

type LiftPolicy string

const (
	LiftOnExpiry              LiftPolicy = "on_expiry"
	LiftUserReverification    LiftPolicy = "user_reverification"
	LiftProfessionalClearance LiftPolicy = "professional_clearance"
)

// LiftBasis is the recorded reason one restriction stopped applying. Expiry
// is also derived from ValidUntil at evaluation time for on_expiry only.
type LiftBasis string

const (
	BasisExpiry                LiftBasis = "expiry"
	BasisUserReverification    LiftBasis = "user_reverification"
	BasisProfessionalClearance LiftBasis = "professional_clearance"
)

// Scope is the restriction's execution coverage. Exercise scope matches the
// standard exercise concept id (or a stable custom exercise id), never a
// display name or a plan-local planExerciseId.
type Scope struct {
	Kind        ScopeKind
	PlanID      uuid.UUID // required for plan, optional for exercise, nil for all_training
	ExerciseIDs []string  // required (non-empty) for exercise only
}

// Lift is a recorded, per-restriction lift receipt as read from the ledger.
// SourceAvailable must be false when the lift's source was withdrawn or no
// longer validates; such a lift is ineffective and the restriction applies.
type Lift struct {
	Basis           LiftBasis
	Revision        int64
	SourceAvailable bool
}

// Restriction is one independently addressable SafetyRestriction. It has no
// persisted status: whether it applies is derived from validity and lift.
type Restriction struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	RiskLevel       RiskLevel
	Scope           Scope
	LiftPolicy      LiftPolicy
	ValidFrom       time.Time
	ValidUntil      *time.Time
	CreatedRevision int64
	Lift            *Lift
}

// Availability says whether the caller could read an authoritative state.
// Anything other than StateAvailable is evaluated as unavailable.
type Availability string

const (
	StateAvailable  Availability = "available"
	StateUnknown    Availability = ""
	StateUnreadable Availability = "unreadable"
)

// State is a SafetyState snapshot pinned to one BodyState revision.
// ScreeningComplete must be false whenever any current symptom lacks a
// complete structured screen, or screening cannot be established at all.
type State struct {
	Availability      Availability
	UserID            uuid.UUID
	Revision          int64
	ScreeningComplete bool
	Restrictions      []Restriction
}

type ActionKind string

const (
	ActionStartSession    ActionKind = "start_session"
	ActionResumeSession   ActionKind = "resume_session"
	ActionAdvanceExercise ActionKind = "advance_exercise"
	ActionPerformSet      ActionKind = "perform_set"
)

// Exercise identifies one exercise an action would execute. Custom marks a
// user-defined exercise whose risk cannot be matched to a standard concept;
// it matches only restrictions that list its stable id explicitly.
type Exercise struct {
	ExerciseID string
	Custom     bool
}

// Action is one Trainer execution step on an exact plan. Saving or editing a
// plan is not a Trainer action and is never gated here.
type Action struct {
	Kind      ActionKind
	PlanID    uuid.UUID
	Exercises []Exercise
}

type Result string

const (
	ResultAllow       Result = "allow"
	ResultWarn        Result = "warn"
	ResultBlock       Result = "block"
	ResultUnavailable Result = "unavailable"
)

type ScopeMatch string

const (
	ScopeMatchVerified ScopeMatch = "verified"
	ScopeMatchUnknown  ScopeMatch = "unknown"
)

type Reason string

const (
	ReasonStateUnavailable      Reason = "safety_state_unavailable"
	ReasonStateMalformed        Reason = "safety_state_malformed"
	ReasonScreeningIncomplete   Reason = "safety_screening_incomplete"
	ReasonInvalidAction         Reason = "invalid_trainer_action"
	ReasonUrgentRestriction     Reason = "urgent_restriction"
	ReasonRestrictedRestriction Reason = "restricted_restriction"
	ReasonAdvisoryRestriction   Reason = "advisory_restriction"
	ReasonCustomExerciseUnknown Reason = "custom_exercise_unverifiable"
	ReasonReviewPointReached    Reason = "review_point_reached"
	ReasonNotYetEffective       Reason = "restriction_not_yet_effective"
)

// Decision is a read-only projection; it is never stored as a second source
// of truth. Only allow and warn permit the action; warn requires the user to
// see the advisory reasons first. UrgentCare asks the UI to lead with the
// urgent-care notice.
type Decision struct {
	Result                Result
	MatchedRestrictionIDs []uuid.UUID
	Reasons               []Reason
	RequiresReview        bool
	UrgentCare            bool
	ScopeMatch            ScopeMatch
	SafetyRevision        int64
	PolicyRevision        string
}

// Permits reports whether the backend may open the action. Every other value
// keeps the action closed.
func (d Decision) Permits() bool {
	return d.Result == ResultAllow || d.Result == ResultWarn
}

// ValidateRestriction checks one restriction's structural coherence against
// the approved product rules (SAFE-D1..D5). It encodes no clinical threshold.
func ValidateRestriction(r Restriction) bool {
	if r.ID == uuid.Nil || r.UserID == uuid.Nil || r.CreatedRevision <= 0 || r.ValidFrom.IsZero() {
		return false
	}
	switch r.RiskLevel {
	case RiskAdvisory, RiskRestricted, RiskUrgent:
	default:
		return false
	}
	switch r.Scope.Kind {
	case ScopeAllTraining:
		if r.Scope.PlanID != uuid.Nil || len(r.Scope.ExerciseIDs) != 0 {
			return false
		}
	case ScopePlan:
		if r.Scope.PlanID == uuid.Nil || len(r.Scope.ExerciseIDs) != 0 {
			return false
		}
	case ScopeExercise:
		if len(r.Scope.ExerciseIDs) == 0 {
			return false
		}
		seen := make(map[string]bool, len(r.Scope.ExerciseIDs))
		for _, id := range r.Scope.ExerciseIDs {
			if id == "" || id != strings.TrimSpace(id) || seen[id] {
				return false
			}
			seen[id] = true
		}
	default:
		return false
	}
	switch r.LiftPolicy {
	case LiftOnExpiry:
		if r.ValidUntil == nil {
			return false
		}
	case LiftUserReverification, LiftProfessionalClearance:
	default:
		return false
	}
	if r.ValidUntil != nil && !r.ValidUntil.After(r.ValidFrom) {
		return false
	}
	// Urgent blocks all training and is never auto-expired or self-cleared.
	if r.RiskLevel == RiskUrgent && (r.Scope.Kind != ScopeAllTraining || r.LiftPolicy != LiftProfessionalClearance) {
		return false
	}
	if r.Lift != nil {
		if r.Lift.Revision <= r.CreatedRevision {
			return false
		}
		if !liftBasisMatches(r.LiftPolicy, r.Lift.Basis) {
			return false
		}
	}
	return true
}

// A lift basis must be exactly the restriction's policy: an Agent or user
// statement can never relax professional clearance into self-reverification.
func liftBasisMatches(policy LiftPolicy, basis LiftBasis) bool {
	switch policy {
	case LiftOnExpiry:
		return basis == BasisExpiry
	case LiftUserReverification:
		return basis == BasisUserReverification
	case LiftProfessionalClearance:
		return basis == BasisProfessionalClearance
	}
	return false
}

// lifted is true only for a source-valid recorded lift, or a reached
// ValidUntil under on_expiry. ValidUntil under any other policy is only a
// review point and keeps the restriction applied.
func lifted(r Restriction, now time.Time) bool {
	if r.Lift != nil && r.Lift.SourceAvailable {
		return true
	}
	return r.LiftPolicy == LiftOnExpiry && r.ValidUntil != nil && !now.Before(*r.ValidUntil)
}

func failClosed(result Result, revision int64, reasons ...Reason) Decision {
	return Decision{Result: result, MatchedRestrictionIDs: []uuid.UUID{}, Reasons: reasons, RequiresReview: true, ScopeMatch: ScopeMatchUnknown, SafetyRevision: revision, PolicyRevision: GatePolicyRevision}
}

// Evaluate decides one Trainer action for owner at now. It is deterministic
// and side-effect free. Unknown, unreadable, foreign, inconsistent or
// unscreened state never allows; overlapping restrictions take the strictest
// result (urgent block > restricted block > advisory warn) and each matched
// restriction keeps its own reason. A future ValidFrom (clock skew or a
// scheduled restriction) is applied conservatively, not skipped.
func Evaluate(owner uuid.UUID, state State, action Action, now time.Time) Decision {
	if owner == uuid.Nil || state.Availability != StateAvailable || state.UserID != owner {
		// A foreign snapshot is reported exactly like a missing one.
		return failClosed(ResultUnavailable, 0, ReasonStateUnavailable)
	}
	if state.Revision <= 0 || now.IsZero() {
		return failClosed(ResultUnavailable, 0, ReasonStateUnavailable)
	}
	seen := make(map[uuid.UUID]bool, len(state.Restrictions))
	for _, r := range state.Restrictions {
		if r.UserID != owner || seen[r.ID] || !ValidateRestriction(r) || r.CreatedRevision > state.Revision || (r.Lift != nil && r.Lift.Revision > state.Revision) {
			return failClosed(ResultUnavailable, state.Revision, ReasonStateMalformed)
		}
		seen[r.ID] = true
	}
	if !validAction(action) {
		return failClosed(ResultBlock, state.Revision, ReasonInvalidAction)
	}

	decision := Decision{Result: ResultAllow, MatchedRestrictionIDs: []uuid.UUID{}, Reasons: []Reason{}, ScopeMatch: ScopeMatchVerified, SafetyRevision: state.Revision, PolicyRevision: GatePolicyRevision}
	reasons := map[Reason]bool{}
	escalate := func(result Result) {
		if rank(result) > rank(decision.Result) {
			decision.Result = result
		}
	}
	if !state.ScreeningComplete {
		escalate(ResultBlock)
		reasons[ReasonScreeningIncomplete] = true
		decision.RequiresReview = true
	}

	restrictions := append([]Restriction(nil), state.Restrictions...)
	sort.Slice(restrictions, func(i, j int) bool { return restrictions[i].ID.String() < restrictions[j].ID.String() })
	for _, r := range restrictions {
		if lifted(r, now) {
			continue
		}
		matched, unknown := matches(r, action)
		if unknown {
			// Insufficient custom-exercise match information blocks the path
			// and asks for review; it is never presented as safe.
			decision.ScopeMatch = ScopeMatchUnknown
			decision.RequiresReview = true
			reasons[ReasonCustomExerciseUnknown] = true
			escalate(ResultBlock)
			decision.MatchedRestrictionIDs = append(decision.MatchedRestrictionIDs, r.ID)
			continue
		}
		if !matched {
			continue
		}
		decision.MatchedRestrictionIDs = append(decision.MatchedRestrictionIDs, r.ID)
		switch r.RiskLevel {
		case RiskUrgent:
			escalate(ResultBlock)
			decision.UrgentCare = true
			decision.RequiresReview = true
			reasons[ReasonUrgentRestriction] = true
		case RiskRestricted:
			escalate(ResultBlock)
			decision.RequiresReview = true
			reasons[ReasonRestrictedRestriction] = true
		case RiskAdvisory:
			escalate(ResultWarn)
			reasons[ReasonAdvisoryRestriction] = true
		}
		if r.LiftPolicy != LiftOnExpiry && r.ValidUntil != nil && !now.Before(*r.ValidUntil) {
			decision.RequiresReview = true
			reasons[ReasonReviewPointReached] = true
		}
		if now.Before(r.ValidFrom) {
			reasons[ReasonNotYetEffective] = true
		}
	}
	for reason := range reasons {
		decision.Reasons = append(decision.Reasons, reason)
	}
	sort.Slice(decision.Reasons, func(i, j int) bool { return decision.Reasons[i] < decision.Reasons[j] })
	return decision
}

func rank(result Result) int {
	switch result {
	case ResultAllow:
		return 0
	case ResultWarn:
		return 1
	case ResultBlock:
		return 2
	}
	return 3
}

func validAction(action Action) bool {
	if action.PlanID == uuid.Nil {
		return false
	}
	switch action.Kind {
	case ActionStartSession, ActionResumeSession:
	case ActionAdvanceExercise, ActionPerformSet:
		if len(action.Exercises) == 0 {
			return false
		}
	default:
		return false
	}
	for _, exercise := range action.Exercises {
		if exercise.ExerciseID == "" || exercise.ExerciseID != strings.TrimSpace(exercise.ExerciseID) {
			return false
		}
	}
	return true
}

// matches reports whether r covers the action, and whether the match is
// unknowable because a custom exercise sits inside an exercise-scoped
// restriction's plan coverage without being listed by its stable id.
func matches(r Restriction, action Action) (matched, unknown bool) {
	switch r.Scope.Kind {
	case ScopeAllTraining:
		return true, false
	case ScopePlan:
		return r.Scope.PlanID == action.PlanID, false
	case ScopeExercise:
		if r.Scope.PlanID != uuid.Nil && r.Scope.PlanID != action.PlanID {
			return false, false
		}
		listed := make(map[string]bool, len(r.Scope.ExerciseIDs))
		for _, id := range r.Scope.ExerciseIDs {
			listed[id] = true
		}
		for _, exercise := range action.Exercises {
			if listed[exercise.ExerciseID] {
				return true, false
			}
		}
		for _, exercise := range action.Exercises {
			if exercise.Custom {
				return false, true
			}
		}
	}
	return false, false
}
