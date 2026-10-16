package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRecordBodyHypothesisCommandValidate(t *testing.T) {
	fact := uuid.New()
	ok := RecordBodyHypothesisCommand{IdempotencyKey: "k1", Statement: "I think running caused my knee pain", RelatedFactIDs: []uuid.UUID{fact}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid command rejected: %v", err)
	}
	tooMany := make([]uuid.UUID, 21)
	for i := range tooMany {
		tooMany[i] = uuid.New()
	}
	cases := map[string]RecordBodyHypothesisCommand{
		"empty key":      {Statement: "x"},
		"padded key":     {IdempotencyKey: " k", Statement: "x"},
		"neg revision":   {IdempotencyKey: "k", ExpectedRevision: -1, Statement: "x"},
		"empty":          {IdempotencyKey: "k"},
		"padded":         {IdempotencyKey: "k", Statement: " x"},
		"too long":       {IdempotencyKey: "k", Statement: strings.Repeat("a", 2001)},
		"nil fact":       {IdempotencyKey: "k", Statement: "x", RelatedFactIDs: []uuid.UUID{uuid.Nil}},
		"duplicate fact": {IdempotencyKey: "k", Statement: "x", RelatedFactIDs: []uuid.UUID{fact, fact}},
		"too many facts": {IdempotencyKey: "k", Statement: "x", RelatedFactIDs: tooMany},
	}
	for name, c := range cases {
		if err := c.Validate(); !errors.Is(err, ErrInvalidBodyHypothesis) {
			t.Errorf("%s: want ErrInvalidBodyHypothesis, got %v", name, err)
		}
	}
	if err := (RecordBodyHypothesisCommand{IdempotencyKey: "k", Statement: strings.Repeat("痛", 2000)}).Validate(); err != nil {
		t.Fatalf("2000 runes must be accepted: %v", err)
	}
}

func TestWithdrawBodyHypothesisCommandValidate(t *testing.T) {
	if err := (WithdrawBodyHypothesisCommand{IdempotencyKey: "k"}).Validate(); err != nil {
		t.Fatalf("withdraw without reason rejected: %v", err)
	}
	if err := (WithdrawBodyHypothesisCommand{IdempotencyKey: "k", Reason: "doctor ruled it out"}).Validate(); err != nil {
		t.Fatalf("withdraw with reason rejected: %v", err)
	}
	for name, c := range map[string]WithdrawBodyHypothesisCommand{
		"no key":        {},
		"padded reason": {IdempotencyKey: "k", Reason: "x "},
		"long reason":   {IdempotencyKey: "k", Reason: strings.Repeat("a", 501)},
		"neg revision":  {IdempotencyKey: "k", ExpectedRevision: -1},
	} {
		if err := c.Validate(); !errors.Is(err, ErrInvalidBodyHypothesis) {
			t.Errorf("%s: want ErrInvalidBodyHypothesis, got %v", name, err)
		}
	}
}
