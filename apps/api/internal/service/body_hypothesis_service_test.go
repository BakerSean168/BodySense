package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeHypothesisLedger struct{ calls int }

func (f *fakeHypothesisLedger) RecordBodyHypothesis(context.Context, uuid.UUID, model.RecordBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	f.calls++
	return &model.BodyHypothesisCommit{}, nil
}
func (f *fakeHypothesisLedger) WithdrawBodyHypothesis(context.Context, uuid.UUID, uuid.UUID, model.WithdrawBodyHypothesisCommand) (*model.BodyHypothesisCommit, error) {
	f.calls++
	return &model.BodyHypothesisCommit{}, nil
}
func (f *fakeHypothesisLedger) ListActiveBodyHypotheses(context.Context, uuid.UUID) (*model.BodyHypothesisList, error) {
	f.calls++
	return &model.BodyHypothesisList{}, nil
}

func (f *fakeHypothesisLedger) ReadConcernGroupSnapshot(_ context.Context, owner uuid.UUID) (*model.ConcernGroupSnapshot, error) {
	f.calls++
	return &model.ConcernGroupSnapshot{UserID: owner}, nil
}

func TestBodyHypothesisServiceConcernGroupsOwnerGuard(t *testing.T) {
	ledger := &fakeHypothesisLedger{}
	svc := NewBodyHypothesisService(ledger)
	if _, err := svc.ListDerivedConcernGroups(context.Background(), uuid.Nil); !errors.Is(err, model.ErrInvalidBodyHypothesis) || ledger.calls != 0 {
		t.Fatalf("nil owner reached ledger: %v", err)
	}
	owner := uuid.New()
	got, err := svc.ListDerivedConcernGroups(context.Background(), owner)
	if err != nil || got.UserID != owner || got.Groups == nil || len(got.Groups) != 0 {
		t.Fatalf("empty projection: %#v %v", got, err)
	}
}

func TestBodyHypothesisServiceRejectsBeforeLedger(t *testing.T) {
	ledger := &fakeHypothesisLedger{}
	svc := NewBodyHypothesisService(ledger)
	ctx := context.Background()
	valid := model.RecordBodyHypothesisCommand{IdempotencyKey: "k", Statement: "maybe it is a sprain"}
	if _, err := svc.RecordBodyHypothesis(ctx, uuid.Nil, valid); !errors.Is(err, model.ErrInvalidBodyHypothesis) {
		t.Fatalf("nil owner accepted: %v", err)
	}
	if _, err := svc.RecordBodyHypothesis(ctx, uuid.New(), model.RecordBodyHypothesisCommand{IdempotencyKey: "k"}); !errors.Is(err, model.ErrInvalidBodyHypothesis) {
		t.Fatalf("empty statement accepted: %v", err)
	}
	if _, err := svc.WithdrawBodyHypothesis(ctx, uuid.New(), uuid.Nil, model.WithdrawBodyHypothesisCommand{IdempotencyKey: "k"}); !errors.Is(err, model.ErrInvalidBodyHypothesis) {
		t.Fatalf("nil target accepted: %v", err)
	}
	if _, err := svc.ListActiveBodyHypotheses(ctx, uuid.Nil); !errors.Is(err, model.ErrInvalidBodyHypothesis) {
		t.Fatalf("nil owner list accepted: %v", err)
	}
	if ledger.calls != 0 {
		t.Fatalf("invalid input reached ledger %d times", ledger.calls)
	}
	if _, err := svc.RecordBodyHypothesis(ctx, uuid.New(), valid); err != nil || ledger.calls != 1 {
		t.Fatalf("valid record not forwarded: %v calls=%d", err, ledger.calls)
	}
}
