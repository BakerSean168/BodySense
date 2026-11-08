package service

import (
	"testing"

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
