package repository

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestBodyHypothesisObservationLinksOwnerIsolationPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()

	mine, err := repo.RecordDirectObservation(ctx, owner, observationDirectCommand(0, "obs-mine", "My weight this morning"))
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := repo.RecordDirectObservation(ctx, foreign, observationDirectCommand(0, "obs-theirs", "Their weight"))
	if err != nil {
		t.Fatal(err)
	}
	// Foreign observation: not-found even with a stale revision (no oracle).
	if _, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{IdempotencyKey: "h-foreign",
		ExpectedRevision: 77, Statement: "guess", RelatedObservationIDs: []uuid.UUID{mine.ObservationID, theirs.ObservationID}}); !errors.Is(err, model.ErrBodyHypothesisObservationNotFound) {
		t.Fatalf("foreign observation link: %v", err)
	}
	if _, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{IdempotencyKey: "h-unknown",
		ExpectedRevision: 1, Statement: "guess", RelatedObservationIDs: []uuid.UUID{uuid.New()}}); !errors.Is(err, model.ErrBodyHypothesisObservationNotFound) {
		t.Fatalf("unknown observation link: %v", err)
	}
	saved, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{IdempotencyKey: "h-obs",
		ExpectedRevision: 1, Statement: "Weight gain is from stopping running", RelatedObservationIDs: []uuid.UUID{mine.ObservationID}})
	if err != nil || saved.Revision != 2 || saved.TargetHypothesisID != nil {
		t.Fatalf("record with observation: %#v %v", saved, err)
	}
	list, err := repo.ListActiveBodyHypotheses(ctx, owner)
	if err != nil || len(list.Hypotheses) != 1 || list.Hypotheses[0].LineageID != saved.HypothesisID ||
		list.Hypotheses[0].SupersedesHypothesisID != nil ||
		!reflect.DeepEqual(list.Hypotheses[0].RelatedObservationIDs, []uuid.UUID{mine.ObservationID}) {
		t.Fatalf("list: %#v %v", list, err)
	}
	snap, err := repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	groups := model.DeriveConcernGroups(*snap)
	if len(groups.Groups) != 1 || !reflect.DeepEqual(groups.Groups[0].RelatedObservationIDs, []uuid.UUID{mine.ObservationID}) ||
		groups.Groups[0].ID != model.ConcernGroupID(saved.HypothesisID) {
		t.Fatalf("observation-linked group: %#v", groups)
	}
	// A withdrawn observation drops out of reads; the stored link stays.
	if _, err := repo.WithdrawDirectObservation(ctx, owner, mine.ObservationID,
		model.AmendBodyObservationCommand{IdempotencyKey: "obs-wd", ExpectedRevision: 2}); err != nil {
		t.Fatal(err)
	}
	list, err = repo.ListActiveBodyHypotheses(ctx, owner)
	if err != nil || len(list.Hypotheses[0].RelatedObservationIDs) != 0 {
		t.Fatalf("withdrawn observation still linked: %#v %v", list, err)
	}
	snap, err = repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if g := model.DeriveConcernGroups(*snap); len(g.Groups) != 1 || len(g.Groups[0].RelatedObservationIDs) != 0 ||
		g.Groups[0].ID != groups.Groups[0].ID {
		t.Fatalf("group after observation withdrawal: %#v", g)
	}
	assertBodyFactCount(t, db, "body_hypothesis_observation_links", owner, 1)
	// A withdrawn observation cannot be newly linked.
	if _, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{IdempotencyKey: "h-wd-obs",
		ExpectedRevision: 3, Statement: "other", RelatedObservationIDs: []uuid.UUID{mine.ObservationID}}); !errors.Is(err, model.ErrBodyHypothesisObservationNotFound) {
		t.Fatalf("withdrawn observation linked: %v", err)
	}
	// DB-level: a forged cross-tenant observation link violates the FK.
	if err := db.Exec("INSERT INTO body_hypothesis_observation_links(user_id,hypothesis_id,observation_id,ordinal) VALUES (?,?,?,3)",
		owner, saved.HypothesisID, theirs.ObservationID).Error; err == nil {
		t.Fatal("cross-tenant observation link accepted by schema")
	}
}

func TestBodyHypothesisCorrectionKeepsHistoryAndGroupIDPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()

	facts, err := repo.RecordDirectFacts(ctx, owner, bodyFactTestCommand("fix-facts", 0)) // rev1
	if err != nil {
		t.Fatal(err)
	}
	var factIDs []uuid.UUID
	if err := json.Unmarshal(facts.FactIDs, &factIDs); err != nil || len(factIDs) != 2 {
		t.Fatalf("fact ids: %s %v", facts.FactIDs, err)
	}
	obs, err := repo.RecordDirectObservation(ctx, owner, observationDirectCommand(1, "fix-obs", "My resting pulse")) // rev2
	if err != nil {
		t.Fatal(err)
	}
	original, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{IdempotencyKey: "fix-h", // rev3
		ExpectedRevision: 2, Statement: "I think it is my desk posture", RelatedFactIDs: []uuid.UUID{factIDs[0]}})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	before := model.DeriveConcernGroups(*snap)
	if len(before.Groups) != 1 || before.Groups[0].ID != model.ConcernGroupID(original.HypothesisID) {
		t.Fatalf("before: %#v", before)
	}

	cmd := model.CorrectBodyHypothesisCommand{IdempotencyKey: "fix-1", ExpectedRevision: 3,
		Replacement: &model.BodyHypothesisReplacement{Statement: "Actually it is my new running shoes",
			RelatedFactIDs: []uuid.UUID{factIDs[0], factIDs[1]}, RelatedObservationIDs: []uuid.UUID{obs.ObservationID}}}
	// Foreign target: not-found before the (stale) revision check.
	if _, err := repo.CorrectBodyHypothesis(ctx, foreign, original.HypothesisID,
		model.CorrectBodyHypothesisCommand{IdempotencyKey: "x", ExpectedRevision: 9, Replacement: cmd.Replacement}); !errors.Is(err, model.ErrBodyHypothesisNotFound) {
		t.Fatalf("foreign correction: %v", err)
	}
	noop := model.CorrectBodyHypothesisCommand{IdempotencyKey: "fix-noop", ExpectedRevision: 3,
		Replacement: &model.BodyHypothesisReplacement{Statement: "I think it is my desk posture", RelatedFactIDs: []uuid.UUID{factIDs[0]}}}
	if _, err := repo.CorrectBodyHypothesis(ctx, owner, original.HypothesisID, noop); !errors.Is(err, model.ErrInvalidBodyHypothesis) {
		t.Fatalf("identical correction accepted: %v", err)
	}
	stale := cmd
	stale.IdempotencyKey, stale.ExpectedRevision = "fix-stale", 2
	if _, err := repo.CorrectBodyHypothesis(ctx, owner, original.HypothesisID, stale); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale correction: %v", err)
	}

	fixed, err := repo.CorrectBodyHypothesis(ctx, owner, original.HypothesisID, cmd) // rev4
	if err != nil || fixed.Operation != "correct" || fixed.Revision != 4 || fixed.TargetHypothesisID == nil ||
		*fixed.TargetHypothesisID != original.HypothesisID || fixed.HypothesisID == original.HypothesisID ||
		fixed.SourceEvidenceID == original.SourceEvidenceID {
		t.Fatalf("correct: %#v %v", fixed, err)
	}
	if replay, err := repo.CorrectBodyHypothesis(ctx, owner, original.HypothesisID, cmd); err != nil || replay.ID != fixed.ID {
		t.Fatalf("correction replay: %#v %v", replay, err)
	}
	changed := cmd
	changed.Replacement = &model.BodyHypothesisReplacement{Statement: "something else"}
	if _, err := repo.CorrectBodyHypothesis(ctx, owner, original.HypothesisID, changed); !errors.Is(err, model.ErrBodyHypothesisIdempotencyConflict) {
		t.Fatalf("changed correction reused key: %v", err)
	}

	// History: the original row, source and links stay; successor in lineage.
	var old, successor model.BodyHypothesis
	if err := db.Where("user_id=? AND id=?", owner, original.HypothesisID).Take(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=? AND id=?", owner, fixed.HypothesisID).Take(&successor).Error; err != nil {
		t.Fatal(err)
	}
	if old.AssertionStatus != "superseded" || old.Statement != "I think it is my desk posture" || old.WithdrawnAt != nil ||
		old.LineageID != original.HypothesisID || successor.AssertionStatus != "asserted" ||
		successor.SupersedesHypothesisID == nil || *successor.SupersedesHypothesisID != original.HypothesisID ||
		successor.LineageID != original.HypothesisID || successor.CreatedRevision != 4 {
		t.Fatalf("history: old %#v successor %#v", old, successor)
	}
	assertBodyFactCount(t, db, "body_hypothesis_fact_links", owner, 3)
	assertBodyFactCount(t, db, "body_hypothesis_observation_links", owner, 1)
	assertBodyFactCount(t, db, "body_hypotheses", owner, 2)
	var outbox model.BodyStateOutbox
	if err := db.Where("user_id=? AND id=?", owner, fixed.EventID).Take(&outbox).Error; err != nil ||
		outbox.EventType != "body_state.hypothesis_corrected" || strings.Contains(string(outbox.Payload), "shoes") ||
		!strings.Contains(string(outbox.Payload), original.HypothesisID.String()) {
		t.Fatalf("outbox: %#v %v", outbox, err)
	}
	var change struct{ ChangeType string }
	if err := db.Raw("SELECT change_type FROM body_state_revisions WHERE user_id=? AND revision=4", owner).Scan(&change).Error; err != nil ||
		change.ChangeType != "hypothesis.corrected" {
		t.Fatalf("revision change: %#v %v", change, err)
	}

	list, err := repo.ListActiveBodyHypotheses(ctx, owner)
	if err != nil || len(list.Hypotheses) != 1 || list.Hypotheses[0].ID != fixed.HypothesisID ||
		list.Hypotheses[0].LineageID != original.HypothesisID || list.Hypotheses[0].Statement != cmd.Replacement.Statement ||
		!reflect.DeepEqual(list.Hypotheses[0].RelatedFactIDs, cmd.Replacement.RelatedFactIDs) ||
		!reflect.DeepEqual(list.Hypotheses[0].RelatedObservationIDs, cmd.Replacement.RelatedObservationIDs) {
		t.Fatalf("list after correction: %#v %v", list, err)
	}
	// DC-032: the group keeps its id across the correction even though its
	// member hypothesis, facts and observations all changed.
	snap, err = repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	after := model.DeriveConcernGroups(*snap)
	if len(after.Groups) != 1 || after.Groups[0].ID != before.Groups[0].ID || after.Groups[0].LatestRevision != 4 ||
		!reflect.DeepEqual(after.Groups[0].RelatedObservationIDs, []uuid.UUID{obs.ObservationID}) {
		t.Fatalf("group id moved on correction: %#v -> %#v", before, after)
	}

	// The superseded hypothesis can neither be corrected nor withdrawn.
	again := cmd
	again.IdempotencyKey, again.ExpectedRevision = "fix-2", 4
	if _, err := repo.CorrectBodyHypothesis(ctx, owner, original.HypothesisID, again); !errors.Is(err, model.ErrBodyHypothesisNotCurrent) {
		t.Fatalf("superseded corrected again: %v", err)
	}
	if _, err := repo.WithdrawBodyHypothesis(ctx, owner, original.HypothesisID,
		model.WithdrawBodyHypothesisCommand{IdempotencyKey: "wd-old", ExpectedRevision: 4}); !errors.Is(err, model.ErrBodyHypothesisNotCurrent) {
		t.Fatalf("superseded withdrawn: %v", err)
	}
	// Second correction in the lineage keeps lineage and group id.
	second, err := repo.CorrectBodyHypothesis(ctx, owner, fixed.HypothesisID, model.CorrectBodyHypothesisCommand{
		IdempotencyKey: "fix-3", ExpectedRevision: 4,
		Replacement: &model.BodyHypothesisReplacement{Statement: "Probably both shoes and posture",
			RelatedFactIDs: []uuid.UUID{factIDs[1]}, RelatedObservationIDs: []uuid.UUID{}}})
	if err != nil {
		t.Fatal(err)
	}
	snap, err = repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if g := model.DeriveConcernGroups(*snap); len(g.Groups) != 1 || g.Groups[0].ID != before.Groups[0].ID ||
		!reflect.DeepEqual(g.Groups[0].RelatedHypothesisIDs, []uuid.UUID{second.HypothesisID}) {
		t.Fatalf("second correction: %#v", g)
	}
	wd, err := repo.WithdrawBodyHypothesis(ctx, owner, second.HypothesisID,
		model.WithdrawBodyHypothesisCommand{IdempotencyKey: "wd-new", ExpectedRevision: 5})
	if err != nil || wd.TargetHypothesisID == nil || *wd.TargetHypothesisID != second.HypothesisID {
		t.Fatalf("withdraw successor: %#v %v", wd, err)
	}
	// Schema: one corrective successor per hypothesis; lineage self-consistency.
	if err := db.Exec(`INSERT INTO body_hypotheses(id,user_id,statement,source_evidence_id,assertion_status,created_revision,recorded_at,supersedes_hypothesis_id,lineage_id)
		VALUES (?,?,?,?,'asserted',6,now(),?,?)`, uuid.New(), owner, "dup successor", fixed.SourceEvidenceID,
		original.HypothesisID, original.HypothesisID).Error; err == nil {
		t.Fatal("second successor of one hypothesis accepted by schema")
	}
	if err := db.Exec(`INSERT INTO body_hypotheses(id,user_id,statement,source_evidence_id,assertion_status,created_revision,recorded_at,lineage_id)
		VALUES (?,?,?,?,'asserted',6,now(),?)`, uuid.New(), owner, "orphan lineage", fixed.SourceEvidenceID,
		original.HypothesisID).Error; err == nil {
		t.Fatal("uncorrected hypothesis with a foreign lineage accepted by schema")
	}
}
