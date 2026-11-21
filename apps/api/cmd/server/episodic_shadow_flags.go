package main

import (
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"gorm.io/gorm"
)

// episodicShadowProposerFromEnv wires the S2 HTTP proposer only while the
// explicit, default-off switch is on. A falsy or absent value returns a nil
// proposer, which keeps NewEpisodicDistillationTriggerFromEnv inert.
func episodicShadowProposerFromEnv(
	getenv func(string) string, client *service.AIClient,
) service.EpisodicShadowProposer {
	if !service.EpisodicShadowGenerationEnabledFromEnv(getenv) {
		return nil
	}
	return service.NewEpisodicShadowHTTPProposer(client)
}

// episodicShadowWriterFromEnv enables admission on the shadow writer only while
// the same default-off switch is on. NewEpisodicShadowRepository stays
// disabled by default, so without this step every real selection is rejected
// with ErrEpisodicShadowDisabled and never persists. With the switch off the
// writer keeps its disabled semantics.
func episodicShadowWriterFromEnv(
	getenv func(string) string, db *gorm.DB,
) *repository.EpisodicShadowRepository {
	writer := repository.NewEpisodicShadowRepository(db)
	if service.EpisodicShadowGenerationEnabledFromEnv(getenv) {
		return writer.WithShadowAdmissionEnabled(true)
	}
	return writer
}
