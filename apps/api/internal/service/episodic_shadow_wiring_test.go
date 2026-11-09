package service

import (
	"os"
	"testing"
	"time"

	"github.com/bodysense/api/internal/repository"
)

// Wiring is DB-free on purpose: the trigger only reads pointers here. A
// switch-off environment keeps generation disabled even when a proposer is
// supplied, and a switch-on environment stays disabled without one.
func TestEpisodicDistillationTriggerWiringFollowsSwitch(t *testing.T) {
	distillation := repository.NewEpisodicDistillationRepository(nil)
	writer := repository.NewEpisodicShadowRepository(nil)
	proposer := NewEpisodicShadowHTTPProposer(&AIClient{})
	enabled := func(string) string { return "true" }
	disabled := func(string) string { return "" }

	if trigger := NewEpisodicDistillationTriggerFromEnv(disabled, distillation, writer, proposer); trigger.Enabled() {
		t.Fatal("switch off enabled generation with a wired proposer")
	}
	if trigger := NewEpisodicDistillationTriggerFromEnv(enabled, distillation, writer, nil); trigger.Enabled() {
		t.Fatal("switch on enabled generation without a proposer")
	}
	if trigger := NewEpisodicDistillationTriggerFromEnv(enabled, distillation, writer, proposer); !trigger.Enabled() {
		t.Fatal("switch on with a wired proposer stayed disabled")
	}
}

// The pass and proposal deadlines used to be fixed at 5s and 20s, which no
// real model round trip could meet. Resolution follows the AI runtime and
// consultation execution precedents: unset, non-numeric, zero, or negative
// values keep the safe default.
func TestEpisodicDistillationTimeoutsResolution(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		value   string
		unset   bool
		resolve func() time.Duration
		want    time.Duration
	}{
		{name: "proposal unset uses default", env: episodicProposalTimeoutEnv, unset: true, resolve: episodicProposalTimeout, want: 120 * time.Second},
		{name: "proposal configured seconds override default", env: episodicProposalTimeoutEnv, value: "180", resolve: episodicProposalTimeout, want: 180 * time.Second},
		{name: "proposal non numeric falls back to default", env: episodicProposalTimeoutEnv, value: "two-minutes", resolve: episodicProposalTimeout, want: 120 * time.Second},
		{name: "proposal zero falls back to default", env: episodicProposalTimeoutEnv, value: "0", resolve: episodicProposalTimeout, want: 120 * time.Second},
		{name: "proposal negative falls back to default", env: episodicProposalTimeoutEnv, value: "-30", resolve: episodicProposalTimeout, want: 120 * time.Second},
		{name: "pass budget unset uses default", env: episodicPassBudgetEnv, unset: true, resolve: episodicPassBudget, want: 600 * time.Second},
		{name: "pass budget configured seconds override default", env: episodicPassBudgetEnv, value: "900", resolve: episodicPassBudget, want: 900 * time.Second},
		{name: "pass budget non numeric falls back to default", env: episodicPassBudgetEnv, value: "ten-minutes", resolve: episodicPassBudget, want: 600 * time.Second},
		{name: "pass budget zero falls back to default", env: episodicPassBudgetEnv, value: "0", resolve: episodicPassBudget, want: 600 * time.Second},
		{name: "pass budget negative falls back to default", env: episodicPassBudgetEnv, value: "-30", resolve: episodicPassBudget, want: 600 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Register the ambient values for cleanup before mutating them.
			t.Setenv(episodicProposalTimeoutEnv, "")
			t.Setenv(episodicPassBudgetEnv, "")
			if tc.unset {
				if err := os.Unsetenv(tc.env); err != nil {
					t.Fatalf("unset %s: %v", tc.env, err)
				}
			} else {
				t.Setenv(tc.env, tc.value)
			}

			if got := tc.resolve(); got != tc.want {
				t.Fatalf("resolve() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNewEpisodicDistillationTriggerWiresConfiguredTimeouts(t *testing.T) {
	t.Setenv(episodicProposalTimeoutEnv, "180")
	t.Setenv(episodicPassBudgetEnv, "900")

	trigger := NewEpisodicDistillationTrigger(nil, nil, nil)
	if trigger.proposalTimeout != 180*time.Second {
		t.Fatalf("proposalTimeout = %s, want 180s", trigger.proposalTimeout)
	}
	if trigger.passBudget != 900*time.Second {
		t.Fatalf("passBudget = %s, want 900s", trigger.passBudget)
	}
}
