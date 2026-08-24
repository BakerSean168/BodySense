package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

var ErrKnowledgeOperatorRequired = errors.New("knowledge operator authority is required")

type knowledgeOperatorUserStore interface {
	FindByID(context.Context, uuid.UUID) (*model.User, error)
}

type KnowledgeOperatorAuthority struct {
	users knowledgeOperatorUserStore
}

func NewKnowledgeOperatorAuthority(users knowledgeOperatorUserStore) *KnowledgeOperatorAuthority {
	return &KnowledgeOperatorAuthority{users: users}
}

// Require resolves a durable operator UUID. A caller-supplied label/email/name
// is never accepted as lifecycle authority.
func (a *KnowledgeOperatorAuthority) Require(ctx context.Context, rawActor string) (uuid.UUID, error) {
	if a == nil || a.users == nil {
		return uuid.Nil, fmt.Errorf("%w: user store unavailable", ErrKnowledgeOperatorRequired)
	}
	actorID, err := uuid.Parse(strings.TrimSpace(rawActor))
	if err != nil || actorID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: actor must be a user UUID", ErrKnowledgeOperatorRequired)
	}
	user, err := a.users.FindByID(ctx, actorID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve knowledge operator %s: %w", actorID, err)
	}
	if user.Role != model.UserRoleOperator {
		return uuid.Nil, fmt.Errorf("%w: user %s has role %q", ErrKnowledgeOperatorRequired, actorID, user.Role)
	}
	return user.ID, nil
}
