package main

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
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

// The writer is disabled by default, so production wiring must enable
// admission under the same default-off switch as the proposer. The draft is a
// legal selection; a nil owner short-circuits right after the admission gate,
// which keeps the probe DB-free.
func TestEpisodicShadowWriterFromEnvFollowsSwitch(t *testing.T) {
	selection := repository.EpisodicShadowDraft{
		ConversationID: uuid.New(), FromSeq: 1, ToSeq: 2,
		SourceMessageIDs:       []uuid.UUID{uuid.New(), uuid.New()},
		GeneratorVersion:       "synthetic-v1",
		RetentionPolicyVersion: "shadow-policy-v1",
		ContentText:            "Synthetic preference for short coaching summaries.",
	}
	for value, wantAdmitted := range map[string]bool{
		"": false, "false": false, "0": false, "yes": false, "true": true, "1": true,
	} {
		writer := episodicShadowWriterFromEnv(func(string) string { return value }, nil)
		_, _, err := writer.SaveShadowDraft(context.Background(), uuid.Nil, selection)
		if wantAdmitted && errors.Is(err, repository.ErrEpisodicShadowDisabled) {
			t.Fatalf("value %q left admission disabled: %v", value, err)
		}
		if !wantAdmitted && !errors.Is(err, repository.ErrEpisodicShadowDisabled) {
			t.Fatalf("value %q enabled admission: %v", value, err)
		}
	}
}
