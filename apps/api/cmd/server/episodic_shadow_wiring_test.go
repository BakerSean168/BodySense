package main

import (
	"testing"

	"github.com/bodysense/api/internal/service"
)

// The production wiring must not construct a model seam while the default-off
// switch is absent or falsy; only an explicit true/1 selects the HTTP proposer.
func TestEpisodicShadowProposerFromEnvFollowsSwitch(t *testing.T) {
	client := &service.AIClient{}
	for value, wantWired := range map[string]bool{
		"": false, "false": false, "0": false, "yes": false, "true": true, "1": true,
	} {
		proposer := episodicShadowProposerFromEnv(func(string) string { return value }, client)
		if (proposer != nil) != wantWired {
			t.Fatalf("value %q wired=%v, want %v", value, proposer != nil, wantWired)
		}
		if !wantWired {
			continue
		}
		if _, ok := proposer.(*service.EpisodicShadowHTTPProposer); !ok {
			t.Fatalf("value %q produced %T, want the HTTP proposer", value, proposer)
		}
	}
}
