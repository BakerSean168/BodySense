package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type episodeReadRepository interface {
	ListEpisodes(context.Context, uuid.UUID, int64) (*model.EpisodePage, error)
	GetEpisode(context.Context, uuid.UUID, uuid.UUID) (*model.EpisodeRecord, error)
	ListEpisodeRelations(context.Context, uuid.UUID, uuid.UUID, int64) (*model.EpisodeRelationPage, error)
}

// EpisodeReadService exposes only owner-scoped, immutable-source-backed
// projections. Authenticated context must be supplied by its future canonical
// public transport; no model-supplied patient ID or Episode writer is accepted.
type EpisodeReadService struct {
	repository episodeReadRepository
}

func NewEpisodeReadService(repository episodeReadRepository) *EpisodeReadService {
	return &EpisodeReadService{repository: repository}
}

func (s *EpisodeReadService) ListEpisodes(ctx context.Context, owner uuid.UUID, beforeRevision int64) (*model.EpisodePage, error) {
	if owner == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: authenticated owner and valid cursor required", model.ErrInvalidEpisodeEvidence)
	}
	return s.repository.ListEpisodes(ctx, owner, beforeRevision)
}

// Authenticated owner-only, source-bound recurrence lineage projection.
func (s *EpisodeReadService) ListEpisodeRelations(ctx context.Context, owner, episode uuid.UUID, beforeRevision int64) (*model.EpisodeRelationPage, error) {
	if owner == uuid.Nil || episode == uuid.Nil || beforeRevision < 0 {
		return nil, fmt.Errorf("%w: authenticated owner, target and cursor required", model.ErrInvalidEpisodeEvidence)
	}
	return s.repository.ListEpisodeRelations(ctx, owner, episode, beforeRevision)
}

func (s *EpisodeReadService) GetEpisode(ctx context.Context, owner, episode uuid.UUID) (*model.EpisodeRecord, error) {
	if owner == uuid.Nil || episode == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner and exact episode ID required", model.ErrInvalidEpisodeEvidence)
	}
	return s.repository.GetEpisode(ctx, owner, episode)
}
