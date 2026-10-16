package service

import (
	"context"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// gateRestriction maps one ledger record onto the pure gate input. A recorded
// resolution is a lift only while its source still validates.
func gateRestriction(row model.SafetyRestrictionRecord) safety.Restriction {
	r := safety.Restriction{
		ID: row.ID, UserID: row.UserID, RiskLevel: safety.RiskLevel(row.RiskLevel),
		Scope:      safety.Scope{Kind: safety.ScopeKind(row.ScopeKind), ExerciseIDs: append([]string(nil), row.ExerciseIDs...)},
		LiftPolicy: safety.LiftPolicy(row.LiftPolicy), ValidFrom: row.ValidFrom, ValidUntil: row.ValidUntil,
		CreatedRevision: row.CreatedRevision,
	}
	if row.PlanID != nil {
		r.Scope.PlanID = *row.PlanID
		if r.Scope.PlanID == uuid.Nil {
			// An explicit nil plan is malformed, not "no plan".
			r.Scope.Kind = "invalid"
		}
	}
	if row.Resolution != nil {
		r.Lift = &safety.Lift{Basis: safety.LiftBasis(row.Resolution.BasisKind), Revision: row.Resolution.Revision, SourceAvailable: row.ClearanceAvailable}
	}
	return r
}

// TrainerGateState converts one pinned native BodyState read into the pure
// SafetyState the TrainerGate evaluates. Any projection failure (foreign,
// unpinned, malformed or source-inconsistent ledger) yields an unreadable
// state, which the gate never allows. Every current symptom must be covered by
// a complete structured screen; a BodyState with no current symptom has
// nothing left to screen.
func TrainerGateState(owner uuid.UUID, snapshot *BodyStateReadModel) safety.State {
	if owner == uuid.Nil || snapshot == nil || snapshot.UserID != owner {
		return safety.State{Availability: safety.StateUnreadable}
	}
	envelope, err := ProjectSafetyEnvelopeV2(snapshot)
	if err != nil || envelope.BodyStateRevision != snapshot.CurrentRevision {
		return safety.State{Availability: safety.StateUnreadable}
	}
	state := safety.State{
		Availability:      safety.StateAvailable,
		UserID:            owner,
		Revision:          snapshot.CurrentRevision,
		ScreeningComplete: len(envelope.Coverage.IncompleteSourceRefs) == 0,
		Restrictions:      make([]safety.Restriction, 0, len(snapshot.SafetyRestrictions)),
	}
	for _, row := range snapshot.SafetyRestrictions {
		state.Restrictions = append(state.Restrictions, gateRestriction(row))
	}
	return state
}

// EvaluateTrainerGate is the Go-owned check every Trainer start, resume,
// advance and set action must repeat on the backend. It reads the owner's
// native ledger in one snapshot; a failed read evaluates as unavailable. It
// never writes, never lifts a restriction and never touches a TrainingPlan.
func (s *BodyStateService) EvaluateTrainerGate(ctx context.Context, owner uuid.UUID, action safety.Action, now time.Time) safety.Decision {
	snapshot, err := s.GetBodyState(ctx, owner, 1)
	if err != nil {
		return safety.Evaluate(owner, safety.State{Availability: safety.StateUnreadable}, action, now)
	}
	return safety.Evaluate(owner, TrainerGateState(owner, snapshot), action, now)
}
