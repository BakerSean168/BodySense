package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestBodyHypothesisRecordWithdrawOwnerIsolationPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()

	facts, err := repo.RecordDirectFacts(ctx, owner, bodyFactTestCommand("hyp-facts", 0))
	if err != nil {
		t.Fatal(err)
	}
	var factIDs []uuid.UUID
	if err := json.Unmarshal(facts.FactIDs, &factIDs); err != nil || len(factIDs) != 2 {
		t.Fatalf("fact ids: %s %v", facts.FactIDs, err)
	}
	foreignFacts, err := repo.RecordDirectFacts(ctx, foreign, bodyFactTestCommand("hyp-foreign-facts", 0))
	if err != nil {
		t.Fatal(err)
	}
	var foreignFactIDs []uuid.UUID
	if err := json.Unmarshal(foreignFacts.FactIDs, &foreignFactIDs); err != nil {
		t.Fatal(err)
	}

	// A foreign fact cannot be linked, and is reported as not-found even
	// with a stale revision (no cross-tenant revision oracle).
	if _, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{
		IdempotencyKey: "hyp-foreign-link", ExpectedRevision: 99, Statement: "guess",
		RelatedFactIDs: []uuid.UUID{factIDs[0], foreignFactIDs[0]},
	}); !errors.Is(err, model.ErrBodyHypothesisFactNotFound) {
		t.Fatalf("foreign fact link accepted: %v", err)
	}

	cmd := model.RecordBodyHypothesisCommand{IdempotencyKey: "hyp-1", ExpectedRevision: 1,
		Statement: "I think my shoulder pain comes from desk posture", RelatedFactIDs: []uuid.UUID{factIDs[1], factIDs[0]}}
	saved, err := repo.RecordBodyHypothesis(ctx, owner, cmd)
	if err != nil || saved.Revision != 2 || saved.Operation != "record" || saved.HypothesisID == uuid.Nil {
		t.Fatalf("record: %#v %v", saved, err)
	}
	replay, err := repo.RecordBodyHypothesis(ctx, owner, cmd)
	if err != nil || replay.ID != saved.ID || replay.Revision != 2 {
		t.Fatalf("replay did not return original receipt: %#v %v", replay, err)
	}
	changed := cmd
	changed.Statement = "different guess"
	if _, err := repo.RecordBodyHypothesis(ctx, owner, changed); !errors.Is(err, model.ErrBodyHypothesisIdempotencyConflict) {
		t.Fatalf("changed payload reused key: %v", err)
	}
	changed.IdempotencyKey = "hyp-stale"
	if _, err := repo.RecordBodyHypothesis(ctx, owner, changed); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}

	list, err := repo.ListActiveBodyHypotheses(ctx, owner)
	if err != nil || list.CurrentRevision != 2 || len(list.Hypotheses) != 1 {
		t.Fatalf("list: %#v %v", list, err)
	}
	h := list.Hypotheses[0]
	if h.ID != saved.HypothesisID || h.Statement != cmd.Statement || h.Source.Actor != "user" ||
		h.Source.Kind != "direct_edit" || h.Source.ID != saved.SourceEvidenceID || h.Source.Version != 1 ||
		len(h.RelatedFactIDs) != 2 || h.RelatedFactIDs[0] != factIDs[1] || h.RelatedFactIDs[1] != factIDs[0] {
		t.Fatalf("projection: %#v", h)
	}
	other, err := repo.ListActiveBodyHypotheses(ctx, foreign)
	if err != nil || len(other.Hypotheses) != 0 {
		t.Fatalf("foreign owner saw hypothesis: %#v %v", other, err)
	}
	if _, err := repo.WithdrawBodyHypothesis(ctx, foreign, saved.HypothesisID,
		model.WithdrawBodyHypothesisCommand{IdempotencyKey: "foreign-wd", ExpectedRevision: 1}); !errors.Is(err, model.ErrBodyHypothesisNotFound) {
		t.Fatalf("foreign withdrawal: %v", err)
	}
	// The outbox carries ids only, never the statement.
	var outbox model.BodyStateOutbox
	if err := db.Where("user_id=? AND id=?", owner, saved.EventID).Take(&outbox).Error; err != nil ||
		outbox.EventType != "body_state.hypothesis_recorded" || !json.Valid(outbox.Payload) ||
		strings.Contains(string(outbox.Payload), "posture") {
		t.Fatalf("outbox: %#v %v", outbox, err)
	}

	wd := model.WithdrawBodyHypothesisCommand{IdempotencyKey: "hyp-wd", ExpectedRevision: 2, Reason: "physio ruled it out"}
	withdrawn, err := repo.WithdrawBodyHypothesis(ctx, owner, saved.HypothesisID, wd)
	if err != nil || withdrawn.Revision != 3 || withdrawn.Operation != "withdraw" || withdrawn.HypothesisID != saved.HypothesisID {
		t.Fatalf("withdraw: %#v %v", withdrawn, err)
	}
	if again, err := repo.WithdrawBodyHypothesis(ctx, owner, saved.HypothesisID, wd); err != nil || again.ID != withdrawn.ID {
		t.Fatalf("withdraw replay: %#v %v", again, err)
	}
	if _, err := repo.WithdrawBodyHypothesis(ctx, owner, saved.HypothesisID,
		model.WithdrawBodyHypothesisCommand{IdempotencyKey: "hyp-wd-2", ExpectedRevision: 3}); !errors.Is(err, model.ErrBodyHypothesisNotCurrent) {
		t.Fatalf("double withdrawal: %v", err)
	}
	list, err = repo.ListActiveBodyHypotheses(ctx, owner)
	if err != nil || list.CurrentRevision != 3 || len(list.Hypotheses) != 0 {
		t.Fatalf("withdrawn hypothesis still active: %#v %v", list, err)
	}
	// History preserved: row, source, links and linked facts remain.
	var row model.BodyHypothesis
	if err := db.Where("user_id=? AND id=?", owner, saved.HypothesisID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.AssertionStatus != "withdrawn" || row.Statement != cmd.Statement || row.WithdrawnAt == nil ||
		row.WithdrawnRevision == nil || *row.WithdrawnRevision != 3 || row.WithdrawalReason == nil ||
		*row.WithdrawalReason != wd.Reason || row.CreatedRevision != 2 {
		t.Fatalf("withdrawal history: %#v", row)
	}
	assertBodyFactCount(t, db, "body_hypothesis_fact_links", owner, 2)
	assertBodyFactCount(t, db, "body_hypothesis_commits", owner, 2)
	assertBodyFactCount(t, db, "body_hypotheses", owner, 1)
	assertBodyFactCount(t, db, "body_hypotheses", foreign, 0)
	var assertedFacts int64
	if err := db.Table("body_facts").Where("user_id=? AND assertion_status='asserted'", owner).Count(&assertedFacts).Error; err != nil || assertedFacts != 2 {
		t.Fatalf("withdrawing a guess touched facts: %d %v", assertedFacts, err)
	}
	// DB-level owner scoping: a forged cross-tenant link is rejected by FK.
	if err := db.Exec("INSERT INTO body_hypothesis_fact_links(user_id,hypothesis_id,fact_id,ordinal) VALUES (?,?,?,5)",
		owner, saved.HypothesisID, foreignFactIDs[0]).Error; err == nil {
		t.Fatal("cross-tenant fact link accepted by schema")
	}
}
