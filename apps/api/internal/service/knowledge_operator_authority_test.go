package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type knowledgeOperatorUserStoreStub struct {
	user *model.User
	err  error
}

func (s knowledgeOperatorUserStoreStub) FindByID(_ context.Context, _ uuid.UUID) (*model.User, error) {
	return s.user, s.err
}

func TestKnowledgeOperatorAuthorityRequiresDurableOperatorUUID(t *testing.T) {
	operatorID := uuid.New()
	authority := NewKnowledgeOperatorAuthority(knowledgeOperatorUserStoreStub{user: &model.User{ID: operatorID, Role: model.UserRoleOperator}})
	resolved, err := authority.Require(context.Background(), operatorID.String())
	if err != nil || resolved != operatorID {
		t.Fatalf("resolved=%s err=%v", resolved, err)
	}
}

func TestKnowledgeOperatorAuthorityRejectsLabelsAndMembers(t *testing.T) {
	authority := NewKnowledgeOperatorAuthority(knowledgeOperatorUserStoreStub{user: &model.User{ID: uuid.New(), Role: model.UserRoleMember}})
	if _, err := authority.Require(context.Background(), "operator@example.com"); !errors.Is(err, ErrKnowledgeOperatorRequired) {
		t.Fatalf("expected non-UUID actor to fail closed, got %v", err)
	}
	memberID := uuid.New()
	authority = NewKnowledgeOperatorAuthority(knowledgeOperatorUserStoreStub{user: &model.User{ID: memberID, Role: model.UserRoleMember}})
	if _, err := authority.Require(context.Background(), memberID.String()); !errors.Is(err, ErrKnowledgeOperatorRequired) {
		t.Fatalf("expected member to fail closed, got %v", err)
	}
}
