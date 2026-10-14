package trainingplan

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func proposalFixture() (Proposal, UserAcceptance, CurrentContext, time.Time) {
	created := time.Date(2026, 10, 14, 2, 0, 0, 0, time.UTC)
	owner, id := uuid.New(), uuid.New()
	proposal := Proposal{
		ID:                         id,
		UserID:                     owner,
		Kind:                       ProposalCreate,
		State:                      ProposalPending,
		InitiatedBy:                InitiatorAgent,
		SourceID:                   "synthetic-proposal-source",
		ContentFingerprint:         "synthetic-exact-payload-fingerprint",
		RelevantContextFingerprint: "synthetic-body-diagnosis-safety-version",
		CreatedAt:                  created,
		ExpiresAt:                  created.Add(ProposalTTL),
	}
	decision := UserAcceptance{
		ProposalID:           id,
		ActorUserID:          owner,
		DecisionID:           uuid.New(),
		DecisionSourceID:     "synthetic-user-acceptance-source",
		DecisionKind:         "accept",
		PresentedFingerprint: proposal.ContentFingerprint,
		IdempotencyKey:       "synthetic-decision-key",
	}
	current := CurrentContext{
		UserID:                     owner,
		Available:                  true,
		RelevantContextFingerprint: proposal.RelevantContextFingerprint,
	}
	return proposal, decision, current, created.Add(time.Hour)
}

func updateFixture() (Proposal, UserAcceptance, CurrentContext, time.Time) {
	proposal, decision, context, now := proposalFixture()
	proposal.Kind = ProposalUpdate
	proposal.PlanID = uuid.New()
	proposal.BasedOnRevisionID = uuid.New()
	decision.TargetPlanID = proposal.PlanID
	decision.ExpectedRevisionID = proposal.BasedOnRevisionID
	context.TargetPlanID = proposal.PlanID
	context.CurrentAcceptedPlanRevisionID = proposal.BasedOnRevisionID
	return proposal, decision, context, now
}

func TestPlanProposalAdmissionRequiresExactAuthorizedDecision(t *testing.T) {
	type specimen struct {
		proposal Proposal
		decision UserAcceptance
		current  CurrentContext
		now      time.Time
	}
	base := func() specimen {
		p, d, c, at := proposalFixture()
		return specimen{p, d, c, at}
	}
	tests := []struct {
		name   string
		modify func(*specimen)
		want   Verdict
	}{
		{"valid agent preview requires explicit user acceptance", func(*specimen) {}, EligibleForCommit},
		{"valid direct user-initiated proposal", func(s *specimen) { s.proposal.InitiatedBy = InitiatorUser }, EligibleForCommit},
		{"wrong user hidden", func(s *specimen) { s.decision.ActorUserID = uuid.New() }, ProposalUnavailable},
		{"missing actor hidden", func(s *specimen) { s.decision.ActorUserID = uuid.Nil }, ProposalUnavailable},
		{"wrong proposal ID hidden", func(s *specimen) { s.decision.ProposalID = uuid.New() }, ProposalUnavailable},
		{"missing proposal ID hidden", func(s *specimen) { s.proposal.ID = uuid.Nil }, ProposalUnavailable},
		{"missing user ID hidden", func(s *specimen) { s.proposal.UserID = uuid.Nil }, ProposalUnavailable},
		{"replayed accepted without receipt denied", func(s *specimen) { s.proposal.State = ProposalAccepted }, ProposalNotPending},
		{"rejected cannot activate", func(s *specimen) { s.proposal.State = ProposalRejected }, ProposalNotPending},
		{"expired state cannot activate", func(s *specimen) { s.proposal.State = ProposalExpired }, ProposalNotPending},
		{"unknown proposal state denied", func(s *specimen) { s.proposal.State = "garbage" }, ProposalNotPending},
		{"unknown kind denied", func(s *specimen) { s.proposal.Kind = "destroy" }, InvalidProposal},
		{"unknown initiator denied", func(s *specimen) { s.proposal.InitiatedBy = "assistant" }, InvalidProposal},
		{"empty proposal source denied", func(s *specimen) { s.proposal.SourceID = " " }, InvalidProposal},
		{"empty stored fingerprint denied", func(s *specimen) { s.proposal.ContentFingerprint = "" }, InvalidProposal},
		{"empty stored context stamp denied", func(s *specimen) { s.proposal.RelevantContextFingerprint = "" }, InvalidProposal},
		{"missing decision ID rejected", func(s *specimen) { s.decision.DecisionID = uuid.Nil }, InvalidDecision},
		{"missing user-decision source rejected", func(s *specimen) { s.decision.DecisionSourceID = " \t" }, InvalidDecision},
		{"agent review is not user accept", func(s *specimen) { s.decision.DecisionKind = "propose" }, InvalidDecision},
		{"rejection is not acceptance", func(s *specimen) { s.decision.DecisionKind = "reject" }, InvalidDecision},
		{"empty idempotency rejected", func(s *specimen) { s.decision.IdempotencyKey = "" }, InvalidDecision},
		{"fingerprint mismatch denied", func(s *specimen) { s.decision.PresentedFingerprint = "changed" }, ContentChanged},
		{"missing evidence unavailable", func(s *specimen) { s.current.Available = false }, ContextUnavailable},
		{"cross-user evidence with same digest unavailable", func(s *specimen) { s.current.UserID = uuid.New() }, ContextUnavailable},
		{"missing evidence owner unavailable", func(s *specimen) { s.current.UserID = uuid.Nil }, ContextUnavailable},
		{"empty current context unavailable", func(s *specimen) { s.current.RelevantContextFingerprint = "" }, ContextUnavailable},
		{"relevant Safety or BodyState change stale", func(s *specimen) { s.current.RelevantContextFingerprint = "new-version" }, ContextChanged},
		{"create cannot target existing plan", func(s *specimen) { s.decision.TargetPlanID = uuid.New() }, InvalidProposal},
		{"create cannot carry accepted base", func(s *specimen) { s.proposal.BasedOnRevisionID = uuid.New() }, InvalidProposal},
		{"create cannot reuse a plan ID", func(s *specimen) { s.proposal.PlanID = uuid.New() }, InvalidProposal},
		{"create decision cannot pin old revision", func(s *specimen) { s.decision.ExpectedRevisionID = uuid.New() }, InvalidProposal},
		{"create context cannot select existing plan", func(s *specimen) { s.current.TargetPlanID = uuid.New() }, ContextUnavailable},
		{"create context cannot carry old accepted revision", func(s *specimen) { s.current.CurrentAcceptedPlanRevisionID = uuid.New() }, ContextUnavailable},
		{"zero current clock denied", func(s *specimen) { s.now = time.Time{} }, InvalidExpiry},
		{"zero created at denied", func(s *specimen) { s.proposal.CreatedAt = time.Time{} }, InvalidExpiry},
		{"zero expires at denied", func(s *specimen) { s.proposal.ExpiresAt = time.Time{} }, InvalidExpiry},
		{"future-created proposal denied", func(s *specimen) { s.now = s.proposal.CreatedAt.Add(-time.Second) }, InvalidExpiry},
		{"expiration before creation denied", func(s *specimen) { s.proposal.ExpiresAt = s.proposal.CreatedAt.Add(-time.Second) }, InvalidExpiry},
		{"zero expiration window denied", func(s *specimen) { s.proposal.ExpiresAt = s.proposal.CreatedAt }, InvalidExpiry},
		{"expiry may not be extended beyond seven days", func(s *specimen) { s.proposal.ExpiresAt = s.proposal.CreatedAt.Add(ProposalTTL + time.Second) }, InvalidExpiry},
		{"exact seven-day timeout denied", func(s *specimen) { s.now = s.proposal.CreatedAt.Add(ProposalTTL) }, ProposalTimedOut},
		{"early explicit expiry honored", func(s *specimen) { s.proposal.ExpiresAt = s.proposal.CreatedAt.Add(time.Minute) }, ProposalTimedOut},
		{"one nanosecond before deadline valid", func(s *specimen) { s.now = s.proposal.ExpiresAt.Add(-time.Nanosecond) }, EligibleForCommit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := base()
			tc.modify(&s)
			beforeProposal, beforeDecision, beforeContext := s.proposal, s.decision, s.current
			got := EvaluateAcceptance(s.proposal, s.decision, s.current, s.now)
			if got != tc.want {
				t.Fatalf("verdict=%q, want %q", got, tc.want)
			}
			if !reflect.DeepEqual(s.proposal, beforeProposal) ||
				!reflect.DeepEqual(s.decision, beforeDecision) ||
				!reflect.DeepEqual(s.current, beforeContext) {
				t.Fatal("precommit rule mutated a proposal, decision or evidence snapshot")
			}
		})
	}
}

func TestPlanProposalUpdateRevisionAuthorization(t *testing.T) {
	type specimen struct {
		proposal Proposal
		decision UserAcceptance
		current  CurrentContext
		now      time.Time
	}
	fresh := func() specimen { p, d, c, n := updateFixture(); return specimen{p, d, c, n} }
	tests := []struct {
		name   string
		modify func(*specimen)
		want   Verdict
	}{
		{"valid exact update", func(*specimen) {}, EligibleForCommit},
		{"missing proposal plan ID", func(s *specimen) { s.proposal.PlanID = uuid.Nil }, InvalidProposal},
		{"missing proposal base revision", func(s *specimen) { s.proposal.BasedOnRevisionID = uuid.Nil }, InvalidProposal},
		{"decision target differs", func(s *specimen) { s.decision.TargetPlanID = uuid.New() }, InvalidDecision},
		{"decision target omitted", func(s *specimen) { s.decision.TargetPlanID = uuid.Nil }, InvalidDecision},
		{"decision expected revision differs", func(s *specimen) { s.decision.ExpectedRevisionID = uuid.New() }, InvalidDecision},
		{"decision expected revision omitted", func(s *specimen) { s.decision.ExpectedRevisionID = uuid.Nil }, InvalidDecision},
		{"current accepted base changed", func(s *specimen) { s.current.CurrentAcceptedPlanRevisionID = uuid.New() }, PlanRevisionChanged},
		{"current plan is a different same-owner plan", func(s *specimen) { s.current.TargetPlanID = uuid.New() }, PlanRevisionChanged},
		{"current target plan missing", func(s *specimen) { s.current.TargetPlanID = uuid.Nil }, PlanRevisionChanged},
		{"current accepted revision unavailable", func(s *specimen) { s.current.CurrentAcceptedPlanRevisionID = uuid.Nil }, PlanRevisionChanged},
		{"changed context even with exact base denied", func(s *specimen) { s.current.RelevantContextFingerprint = "new-safety-revision" }, ContextChanged},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fresh()
			tc.modify(&s)
			if got := EvaluateAcceptance(s.proposal, s.decision, s.current, s.now); got != tc.want {
				t.Fatalf("verdict=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestPlanProposalSevenDaysIsElapsedTimeNotCalendarDays(t *testing.T) {
	p, d, c, at := proposalFixture()
	// DST/local calendar shifts are irrelevant: 7*24 h uses instants.
	p.CreatedAt = time.Date(2026, 3, 8, 6, 59, 59, 0, time.UTC)
	p.ExpiresAt = p.CreatedAt.Add(ProposalTTL)
	at = p.ExpiresAt.Add(-time.Second)
	if got := EvaluateAcceptance(p, d, c, at); got != EligibleForCommit {
		t.Fatalf("before expiry: %v", got)
	}
	at = p.ExpiresAt
	if got := EvaluateAcceptance(p, d, c, at); got != ProposalTimedOut {
		t.Fatalf("at expiry: %v", got)
	}
}
