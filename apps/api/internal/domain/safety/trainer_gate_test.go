package safety

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	owner = uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	other = uuid.MustParse("00000000-0000-0000-0000-0000000000bb")
	planA = uuid.MustParse("00000000-0000-0000-0000-00000000a001")
	planB = uuid.MustParse("00000000-0000-0000-0000-00000000b001")
	now   = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
)

func ts(offset time.Duration) *time.Time { v := now.Add(offset); return &v }

func rid(n byte) uuid.UUID { return uuid.UUID{0x10, 15: n} }

func restriction(n byte, level RiskLevel, scope Scope, policy LiftPolicy) Restriction {
	return Restriction{ID: rid(n), UserID: owner, RiskLevel: level, Scope: scope, LiftPolicy: policy, ValidFrom: now.Add(-time.Hour), CreatedRevision: 3}
}

func all() Scope                   { return Scope{Kind: ScopeAllTraining} }
func plan(id uuid.UUID) Scope      { return Scope{Kind: ScopePlan, PlanID: id} }
func exercise(ids ...string) Scope { return Scope{Kind: ScopeExercise, ExerciseIDs: ids} }

func state(rs ...Restriction) State {
	return State{Availability: StateAvailable, UserID: owner, Revision: 10, ScreeningComplete: true, Restrictions: rs}
}

func perform(planID uuid.UUID, exercises ...Exercise) Action {
	return Action{Kind: ActionPerformSet, PlanID: planID, Exercises: exercises}
}

func concept(id string) Exercise { return Exercise{ExerciseID: id} }

func custom(id string) Exercise { return Exercise{ExerciseID: id, Custom: true} }

func requireClosed(t *testing.T, d Decision, result Result, reason Reason) {
	t.Helper()
	if d.Permits() || d.Result != result || !d.RequiresReview || !reflect.DeepEqual(d.Reasons, []Reason{reason}) || len(d.MatchedRestrictionIDs) != 0 || d.ScopeMatch != ScopeMatchUnknown || d.PolicyRevision != GatePolicyRevision {
		t.Fatalf("want closed %s/%s, got %+v", result, reason, d)
	}
}

func TestCleanScreenedStateAllows(t *testing.T) {
	d := Evaluate(owner, state(), perform(planA, concept("squat")), now)
	if !d.Permits() || d.Result != ResultAllow || d.RequiresReview || len(d.Reasons) != 0 || d.ScopeMatch != ScopeMatchVerified || d.SafetyRevision != 10 {
		t.Fatalf("got %+v", d)
	}
}

func TestUnavailableStateFailsClosed(t *testing.T) {
	cases := map[string]struct {
		owner uuid.UUID
		state State
		now   time.Time
	}{
		"nil owner":          {uuid.Nil, state(), now},
		"unknown":            {owner, func() State { s := state(); s.Availability = StateUnknown; return s }(), now},
		"unreadable":         {owner, func() State { s := state(); s.Availability = StateUnreadable; return s }(), now},
		"bogus availability": {owner, func() State { s := state(); s.Availability = "stale"; return s }(), now},
		"foreign snapshot":   {owner, func() State { s := state(); s.UserID = other; return s }(), now},
		"nil snapshot user":  {owner, func() State { s := state(); s.UserID = uuid.Nil; return s }(), now},
		"unpinned revision":  {owner, func() State { s := state(); s.Revision = 0; return s }(), now},
		"negative revision":  {owner, func() State { s := state(); s.Revision = -1; return s }(), now},
		"zero clock":         {owner, state(), time.Time{}},
		"zero value state":   {owner, State{}, now},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			requireClosed(t, Evaluate(tc.owner, tc.state, perform(planA, concept("squat")), tc.now), ResultUnavailable, ReasonStateUnavailable)
		})
	}
}

func TestMalformedRestrictionFailsClosed(t *testing.T) {
	good := restriction(1, RiskRestricted, all(), LiftProfessionalClearance)
	mutate := func(f func(*Restriction)) Restriction { r := good; f(&r); return r }
	cases := map[string]Restriction{
		"nil id":                       mutate(func(r *Restriction) { r.ID = uuid.Nil }),
		"foreign owner":                mutate(func(r *Restriction) { r.UserID = other }),
		"nil owner":                    mutate(func(r *Restriction) { r.UserID = uuid.Nil }),
		"zero created revision":        mutate(func(r *Restriction) { r.CreatedRevision = 0 }),
		"concurrent newer restriction": mutate(func(r *Restriction) { r.CreatedRevision = 11 }),
		"zero valid from":              mutate(func(r *Restriction) { r.ValidFrom = time.Time{} }),
		"unknown level":                mutate(func(r *Restriction) { r.RiskLevel = "severe" }),
		"empty level":                  mutate(func(r *Restriction) { r.RiskLevel = "" }),
		"unknown scope":                mutate(func(r *Restriction) { r.Scope = Scope{Kind: "session"} }),
		"all training with plan":       mutate(func(r *Restriction) { r.Scope = Scope{Kind: ScopeAllTraining, PlanID: planA} }),
		"all training with exercises":  mutate(func(r *Restriction) { r.Scope = Scope{Kind: ScopeAllTraining, ExerciseIDs: []string{"squat"}} }),
		"plan without plan id":         mutate(func(r *Restriction) { r.Scope = Scope{Kind: ScopePlan} }),
		"plan with exercises":          mutate(func(r *Restriction) { r.Scope = Scope{Kind: ScopePlan, PlanID: planA, ExerciseIDs: []string{"squat"}} }),
		"exercise without ids":         mutate(func(r *Restriction) { r.Scope = exercise() }),
		"exercise blank id":            mutate(func(r *Restriction) { r.Scope = exercise("") }),
		"exercise padded id":           mutate(func(r *Restriction) { r.Scope = exercise(" squat") }),
		"exercise duplicate ids":       mutate(func(r *Restriction) { r.Scope = exercise("squat", "squat") }),
		"unknown lift policy":          mutate(func(r *Restriction) { r.LiftPolicy = "agent_judgement" }),
		"on expiry without until":      mutate(func(r *Restriction) { r.LiftPolicy = LiftOnExpiry }),
		"until not after from":         mutate(func(r *Restriction) { r.ValidUntil = &r.ValidFrom }),
		"urgent plan scope":            mutate(func(r *Restriction) { r.RiskLevel = RiskUrgent; r.Scope = plan(planA) }),
		"urgent exercise scope":        mutate(func(r *Restriction) { r.RiskLevel = RiskUrgent; r.Scope = exercise("squat") }),
		"urgent self reverification": mutate(func(r *Restriction) {
			r.RiskLevel = RiskUrgent
			r.LiftPolicy = LiftUserReverification
		}),
		"urgent auto expiry": mutate(func(r *Restriction) {
			r.RiskLevel = RiskUrgent
			r.LiftPolicy = LiftOnExpiry
			r.ValidUntil = ts(time.Hour)
		}),
		"stale release before creation": mutate(func(r *Restriction) {
			r.Lift = &Lift{Basis: BasisProfessionalClearance, Revision: 3, SourceAvailable: true}
		}),
		"release from a future revision": mutate(func(r *Restriction) {
			r.Lift = &Lift{Basis: BasisProfessionalClearance, Revision: 11, SourceAvailable: true}
		}),
		"clearance downgraded to self reverification": mutate(func(r *Restriction) {
			r.Lift = &Lift{Basis: BasisUserReverification, Revision: 5, SourceAvailable: true}
		}),
		"clearance by expiry": mutate(func(r *Restriction) {
			r.Lift = &Lift{Basis: BasisExpiry, Revision: 5, SourceAvailable: true}
		}),
		"unknown lift basis": mutate(func(r *Restriction) {
			r.Lift = &Lift{Basis: "model_says_fine", Revision: 5, SourceAvailable: true}
		}),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			// One malformed row poisons the whole snapshot, even next to valid rows
			// and even when the malformed row itself would be lifted or out of scope.
			d := Evaluate(owner, state(restriction(9, RiskAdvisory, plan(planB), LiftUserReverification), bad), perform(planA, concept("squat")), now)
			requireClosed(t, d, ResultUnavailable, ReasonStateMalformed)
		})
	}
	t.Run("duplicate restriction id", func(t *testing.T) {
		requireClosed(t, Evaluate(owner, state(good, good), perform(planA, concept("squat")), now), ResultUnavailable, ReasonStateMalformed)
	})
}

func TestUnscreenedStateBlocks(t *testing.T) {
	s := state()
	s.ScreeningComplete = false
	d := Evaluate(owner, s, perform(planA, concept("squat")), now)
	if d.Permits() || d.Result != ResultBlock || !d.RequiresReview || !reflect.DeepEqual(d.Reasons, []Reason{ReasonScreeningIncomplete}) {
		t.Fatalf("got %+v", d)
	}
	// An advisory or a lifted restriction never re-opens an unscreened state.
	s.Restrictions = []Restriction{restriction(1, RiskAdvisory, all(), LiftUserReverification)}
	if d := Evaluate(owner, s, perform(planA, concept("squat")), now); d.Result != ResultBlock || d.Permits() {
		t.Fatalf("got %+v", d)
	}
}

func TestInvalidActionBlocks(t *testing.T) {
	cases := map[string]Action{
		"nil plan":                 {Kind: ActionStartSession},
		"unknown kind":             {Kind: "save_plan", PlanID: planA},
		"empty kind":               {PlanID: planA},
		"advance without exercise": {Kind: ActionAdvanceExercise, PlanID: planA},
		"set without exercise":     {Kind: ActionPerformSet, PlanID: planA},
		"blank exercise":           perform(planA, concept("")),
		"padded exercise":          perform(planA, concept("squat ")),
	}
	for name, action := range cases {
		t.Run(name, func(t *testing.T) {
			requireClosed(t, Evaluate(owner, state(), action, now), ResultBlock, ReasonInvalidAction)
		})
	}
	for _, kind := range []ActionKind{ActionStartSession, ActionResumeSession} {
		if d := Evaluate(owner, state(), Action{Kind: kind, PlanID: planA}, now); d.Result != ResultAllow {
			t.Fatalf("%s: got %+v", kind, d)
		}
	}
}

func TestOverlapTakesStrictestAndKeepsEachReason(t *testing.T) {
	advisory := restriction(1, RiskAdvisory, exercise("squat"), LiftUserReverification)
	restricted := restriction(2, RiskRestricted, plan(planA), LiftUserReverification)
	urgent := restriction(3, RiskUrgent, all(), LiftProfessionalClearance)
	d := Evaluate(owner, state(urgent, advisory, restricted), perform(planA, concept("squat")), now)
	if d.Result != ResultBlock || !d.UrgentCare || !d.RequiresReview || d.Permits() ||
		!reflect.DeepEqual(d.MatchedRestrictionIDs, []uuid.UUID{rid(1), rid(2), rid(3)}) ||
		!reflect.DeepEqual(d.Reasons, []Reason{ReasonAdvisoryRestriction, ReasonRestrictedRestriction, ReasonUrgentRestriction}) {
		t.Fatalf("got %+v", d)
	}
	// Lifting the urgent restriction alone leaves the restricted block intact.
	urgent.Lift = &Lift{Basis: BasisProfessionalClearance, Revision: 7, SourceAvailable: true}
	d = Evaluate(owner, state(advisory, restricted, urgent), perform(planA, concept("squat")), now)
	if d.Result != ResultBlock || d.UrgentCare || !reflect.DeepEqual(d.MatchedRestrictionIDs, []uuid.UUID{rid(1), rid(2)}) {
		t.Fatalf("got %+v", d)
	}
	// A newer, milder restriction never overrides an older stricter one.
	newer := restriction(4, RiskAdvisory, all(), LiftUserReverification)
	newer.CreatedRevision = 9
	d = Evaluate(owner, state(restricted, newer), perform(planA, concept("row")), now)
	if d.Result != ResultBlock || !reflect.DeepEqual(d.Reasons, []Reason{ReasonAdvisoryRestriction, ReasonRestrictedRestriction}) {
		t.Fatalf("got %+v", d)
	}
}

func TestEvaluationIsOrderIndependent(t *testing.T) {
	rs := []Restriction{
		restriction(3, RiskAdvisory, all(), LiftUserReverification),
		restriction(1, RiskRestricted, exercise("squat"), LiftProfessionalClearance),
		restriction(2, RiskAdvisory, plan(planA), LiftUserReverification),
	}
	first := Evaluate(owner, state(rs...), perform(planA, concept("squat")), now)
	second := Evaluate(owner, state(rs[2], rs[0], rs[1]), perform(planA, concept("squat")), now)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("order changed decision: %+v vs %+v", first, second)
	}
}

func TestAdvisoryWarnsWithoutReview(t *testing.T) {
	d := Evaluate(owner, state(restriction(1, RiskAdvisory, all(), LiftUserReverification)), perform(planA, concept("squat")), now)
	if d.Result != ResultWarn || !d.Permits() || d.RequiresReview || d.UrgentCare || !reflect.DeepEqual(d.Reasons, []Reason{ReasonAdvisoryRestriction}) {
		t.Fatalf("got %+v", d)
	}
}

func TestScopeMatching(t *testing.T) {
	planOnly := restriction(1, RiskRestricted, plan(planA), LiftUserReverification)
	if d := Evaluate(owner, state(planOnly), perform(planA, concept("squat")), now); d.Result != ResultBlock {
		t.Fatalf("plan A: %+v", d)
	}
	if d := Evaluate(owner, state(planOnly), perform(planB, concept("squat")), now); d.Result != ResultAllow {
		t.Fatalf("plan B: %+v", d)
	}
	// The same exercise concept in two plans is caught in both: a plan revision,
	// rename or different planExerciseId cannot bypass an exercise restriction.
	byConcept := restriction(2, RiskRestricted, exercise("squat"), LiftUserReverification)
	for _, p := range []uuid.UUID{planA, planB} {
		if d := Evaluate(owner, state(byConcept), perform(p, concept("row"), concept("squat")), now); d.Result != ResultBlock || !reflect.DeepEqual(d.MatchedRestrictionIDs, []uuid.UUID{rid(2)}) {
			t.Fatalf("concept in %s: %+v", p, d)
		}
	}
	if d := Evaluate(owner, state(byConcept), perform(planA, concept("row")), now); d.Result != ResultAllow {
		t.Fatalf("other concept: %+v", d)
	}
	limited := restriction(3, RiskRestricted, Scope{Kind: ScopeExercise, PlanID: planA, ExerciseIDs: []string{"squat"}}, LiftUserReverification)
	if d := Evaluate(owner, state(limited), perform(planA, concept("squat")), now); d.Result != ResultBlock {
		t.Fatalf("limited A: %+v", d)
	}
	if d := Evaluate(owner, state(limited), perform(planB, concept("squat")), now); d.Result != ResultAllow {
		t.Fatalf("limited B: %+v", d)
	}
	// Start/resume without exercises still sees plan and global coverage.
	if d := Evaluate(owner, state(planOnly), Action{Kind: ActionStartSession, PlanID: planA}, now); d.Result != ResultBlock {
		t.Fatalf("start: %+v", d)
	}
}

func TestCustomExerciseNeverDefaultsToSafe(t *testing.T) {
	byConcept := restriction(1, RiskRestricted, exercise("squat"), LiftUserReverification)
	d := Evaluate(owner, state(byConcept), perform(planA, custom("custom:my-squat")), now)
	if d.Result != ResultBlock || d.Permits() || !d.RequiresReview || d.ScopeMatch != ScopeMatchUnknown ||
		!reflect.DeepEqual(d.Reasons, []Reason{ReasonCustomExerciseUnknown}) || !reflect.DeepEqual(d.MatchedRestrictionIDs, []uuid.UUID{rid(1)}) {
		t.Fatalf("got %+v", d)
	}
	advisory := restriction(2, RiskAdvisory, exercise("squat"), LiftUserReverification)
	if d := Evaluate(owner, state(advisory), perform(planA, custom("custom:my-squat")), now); d.Result != ResultBlock || d.ScopeMatch != ScopeMatchUnknown {
		t.Fatalf("advisory unknown: %+v", d)
	}
	// A restriction that lists the stable custom id matches by its own level.
	listed := restriction(3, RiskAdvisory, exercise("custom:my-squat"), LiftUserReverification)
	if d := Evaluate(owner, state(listed), perform(planA, custom("custom:my-squat")), now); d.Result != ResultWarn || d.ScopeMatch != ScopeMatchVerified {
		t.Fatalf("listed custom: %+v", d)
	}
	// Outside the restriction's plan the custom exercise is not in coverage.
	limited := restriction(4, RiskRestricted, Scope{Kind: ScopeExercise, PlanID: planA, ExerciseIDs: []string{"squat"}}, LiftUserReverification)
	if d := Evaluate(owner, state(limited), perform(planB, custom("custom:my-squat")), now); d.Result != ResultAllow {
		t.Fatalf("other plan: %+v", d)
	}
	// Without any exercise restriction a custom exercise is evaluated normally.
	if d := Evaluate(owner, state(), perform(planA, custom("custom:my-squat")), now); d.Result != ResultAllow {
		t.Fatalf("no exercise restrictions: %+v", d)
	}
}

func TestLiftRules(t *testing.T) {
	cleared := restriction(1, RiskRestricted, all(), LiftProfessionalClearance)
	cleared.Lift = &Lift{Basis: BasisProfessionalClearance, Revision: 5, SourceAvailable: true}
	if d := Evaluate(owner, state(cleared), perform(planA, concept("squat")), now); d.Result != ResultAllow {
		t.Fatalf("cleared: %+v", d)
	}
	// A withdrawn or no-longer-valid clearance source re-applies the restriction.
	cleared.Lift.SourceAvailable = false
	if d := Evaluate(owner, state(cleared), perform(planA, concept("squat")), now); d.Result != ResultBlock {
		t.Fatalf("withdrawn source: %+v", d)
	}
	reverified := restriction(2, RiskRestricted, plan(planA), LiftUserReverification)
	reverified.Lift = &Lift{Basis: BasisUserReverification, Revision: 6, SourceAvailable: true}
	if d := Evaluate(owner, state(reverified), perform(planA, concept("squat")), now); d.Result != ResultAllow {
		t.Fatalf("reverified: %+v", d)
	}
	// A professional clearance due date passing is only a review point.
	due := restriction(3, RiskRestricted, all(), LiftProfessionalClearance)
	due.ValidUntil = ts(-time.Minute)
	d := Evaluate(owner, state(due), perform(planA, concept("squat")), now)
	if d.Result != ResultBlock || !d.RequiresReview || !reflect.DeepEqual(d.Reasons, []Reason{ReasonRestrictedRestriction, ReasonReviewPointReached}) {
		t.Fatalf("due: %+v", d)
	}
	selfDue := restriction(4, RiskAdvisory, all(), LiftUserReverification)
	selfDue.ValidUntil = ts(-time.Minute)
	if d := Evaluate(owner, state(selfDue), perform(planA, concept("squat")), now); d.Result != ResultWarn || !d.RequiresReview {
		t.Fatalf("self due: %+v", d)
	}
	expiring := restriction(5, RiskAdvisory, exercise("squat"), LiftOnExpiry)
	expiring.ValidUntil = ts(time.Minute)
	if d := Evaluate(owner, state(expiring), perform(planA, concept("squat")), now); d.Result != ResultWarn {
		t.Fatalf("before expiry: %+v", d)
	}
	expiring.ValidUntil = ts(0)
	if d := Evaluate(owner, state(expiring), perform(planA, concept("squat")), now); d.Result != ResultAllow {
		t.Fatalf("at expiry: %+v", d)
	}
	recorded := restriction(6, RiskRestricted, plan(planA), LiftOnExpiry)
	recorded.ValidUntil = ts(time.Hour)
	recorded.Lift = &Lift{Basis: BasisExpiry, Revision: 8, SourceAvailable: true}
	if d := Evaluate(owner, state(recorded), perform(planA, concept("squat")), now); d.Result != ResultAllow {
		t.Fatalf("recorded expiry: %+v", d)
	}
}

func TestNotYetEffectiveIsAppliedConservatively(t *testing.T) {
	future := restriction(1, RiskRestricted, all(), LiftUserReverification)
	future.ValidFrom = now.Add(time.Hour)
	d := Evaluate(owner, state(future), perform(planA, concept("squat")), now)
	if d.Result != ResultBlock || !reflect.DeepEqual(d.Reasons, []Reason{ReasonRestrictedRestriction, ReasonNotYetEffective}) {
		t.Fatalf("got %+v", d)
	}
}

func TestEvaluateDoesNotMutateInput(t *testing.T) {
	rs := []Restriction{restriction(2, RiskRestricted, all(), LiftUserReverification), restriction(1, RiskAdvisory, all(), LiftUserReverification)}
	s := state(rs...)
	_ = Evaluate(owner, s, perform(planA, concept("squat")), now)
	if s.Restrictions[0].ID != rid(2) || s.Restrictions[1].ID != rid(1) {
		t.Fatal("input order mutated")
	}
}
