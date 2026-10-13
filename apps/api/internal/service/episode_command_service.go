package service

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type episodeCommandRepository interface {
	CreateEpisode(context.Context, uuid.UUID, model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error)
	LinkEpisodeFact(context.Context, uuid.UUID, uuid.UUID, model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error)
}

// The only authorized caller is an exact authenticated direct-user HTTP
// mutation. Future model-driven episode associations need a separate verified
// High-confidence source and consent token, not reuse of this service.
type EpisodeCommandService struct{ repository episodeCommandRepository }

func NewEpisodeCommandService(repository episodeCommandRepository) *EpisodeCommandService {
	return &EpisodeCommandService{repository: repository}
}
func (s *EpisodeCommandService) CreateEpisode(ctx context.Context, owner uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	if owner == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner required", model.ErrInvalidEpisodeEvidence)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return s.repository.CreateEpisode(ctx, owner, c)
}
func (s *EpisodeCommandService) LinkEpisodeFact(ctx context.Context, owner, episode uuid.UUID, c model.EpisodeAssociationCommand) (*model.EpisodeCommandReceipt, error) {
	if owner == uuid.Nil || episode == uuid.Nil {
		return nil, fmt.Errorf("%w: authenticated owner and target required", model.ErrInvalidEpisodeEvidence)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return s.repository.LinkEpisodeFact(ctx, owner, episode, c)
}
