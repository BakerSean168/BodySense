package service

import (
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
// real model round trip could meet. Budgets now arrive through the injected
// configuration; this pins the env→trigger wiring, including the fallback
// rules covered by the config package.
func TestNewEpisodicDistillationTriggerFromEnvWiresConfiguredTimeouts(t *testing.T) {
	getenv := func(name string) string {
		return map[string]string{
			"EPISODIC_PROPOSAL_TIMEOUT_SECONDS": "180",
			"EPISODIC_PASS_BUDGET_SECONDS":      "900",
		}[name]
	}

	trigger := NewEpisodicDistillationTriggerFromEnv(getenv, nil, nil, nil)
	if trigger.proposalTimeout != 180*time.Second {
		t.Fatalf("proposalTimeout = %s, want 180s", trigger.proposalTimeout)
	}
	if trigger.passBudget != 900*time.Second {
		t.Fatalf("passBudget = %s, want 900s", trigger.passBudget)
	}
}

// A writer left at its default-off admission is a wiring fault, not a model
// failure: it must terminate the segment as admission_disabled instead of
// being retried as model_error.
func TestEpisodicFailureReasonClassifiesDisabledAdmission(t *testing.T) {
	reason, terminal := episodicFailureReason(repository.ErrEpisodicShadowDisabled)
	if reason != "admission_disabled" || !terminal {
		t.Fatalf("reason = %q, terminal = %v, want admission_disabled and terminal", reason, terminal)
	}
}
