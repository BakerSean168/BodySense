package repository

import (
	"context"
	"reflect"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestDerivedConcernGroupsFromOneRevisionPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()

	// rev1: symptom A; rev2: episode E over A.
	symptomText := "concern group shoulder ache this week"
	commitA := episodeRecordSymptom(t, repo, owner, 0, symptomText, "present", "2026-10-12")
	factA := episodeLookupFact(t, db, owner, symptomText)
	ep, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(factA.ID, commitA.Revision, "cg-episode"))
	if err != nil || ep.Revision != 2 {
		t.Fatalf("episode: %#v %v", ep, err)
	}
	// rev3: two unrelated facts F1 (symptom) and F2 (lifestyle).
	if _, err := repo.RecordDirectFacts(ctx, owner, bodyFactTestCommand("cg-facts", 2)); err != nil {
		t.Fatal(err)
	}
	f1 := episodeLookupFact(t, db, owner, "synthetic private shoulder statement")
	f2 := episodeLookupFact(t, db, owner, "synthetic private activity habit")
	// rev4: H1 bridges F1 and A (and therefore E). rev5: H2 stands alone.
	h1, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{IdempotencyKey: "cg-h1",
		ExpectedRevision: 3, Statement: "Maybe it is my desk posture", RelatedFactIDs: []uuid.UUID{f1.ID, factA.ID}})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := repo.RecordBodyHypothesis(ctx, owner, model.RecordBodyHypothesisCommand{IdempotencyKey: "cg-h2",
		ExpectedRevision: 4, Statement: "Could be poor sleep"})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil || snap.Revision != 5 || snap.UserID != owner {
		t.Fatalf("snapshot: %#v %v", snap, err)
	}
	got := model.DeriveConcernGroups(*snap)
	if len(got.Groups) != 2 {
		t.Fatalf("groups: %#v", got)
	}
	lone, bridged := got.Groups[0], got.Groups[1]
	if lone.LatestRevision != 5 || lone.DisplayTitle != "Could be poor sleep" ||
		!reflect.DeepEqual(lone.RelatedHypothesisIDs, []uuid.UUID{h2.HypothesisID}) || len(lone.RelatedFactIDs) != 0 {
		t.Fatalf("lone hypothesis group: %#v", lone)
	}
	if bridged.LatestRevision != 4 || bridged.DisplayTitle != "Maybe it is my desk posture" ||
		!reflect.DeepEqual(bridged.RelatedHypothesisIDs, []uuid.UUID{h1.HypothesisID}) ||
		!reflect.DeepEqual(bridged.RelatedFactIDs, []uuid.UUID{factA.ID, f1.ID}) ||
		!reflect.DeepEqual(bridged.RelatedEpisodeIDs, []uuid.UUID{ep.EpisodeID}) ||
		bridged.ID != model.ConcernGroupID(ep.EpisodeID) {
		t.Fatalf("bridged group: %#v", bridged)
	}
	for _, g := range got.Groups {
		for _, id := range g.RelatedFactIDs {
			if id == f2.ID {
				t.Fatal("isolated fact F2 formed/joined a group")
			}
		}
	}
	again, err := repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil || !reflect.DeepEqual(model.DeriveConcernGroups(*again), got) {
		t.Fatalf("projection not deterministic across reads: %v", err)
	}
	if other, err := repo.ReadConcernGroupSnapshot(ctx, foreign); err != nil ||
		len(model.DeriveConcernGroups(*other).Groups) != 0 || other.Revision != 0 {
		t.Fatalf("foreign owner saw groups: %#v %v", other, err)
	}

	// rev6: withdrawing H1 splits the bridge; E keeps an episode-only group
	// with A, and F1 becomes isolated. No row is written by the read.
	if _, err := repo.WithdrawBodyHypothesis(ctx, owner, h1.HypothesisID,
		model.WithdrawBodyHypothesisCommand{IdempotencyKey: "cg-wd", ExpectedRevision: 5}); err != nil {
		t.Fatal(err)
	}
	snap, err = repo.ReadConcernGroupSnapshot(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	split := model.DeriveConcernGroups(*snap)
	if split.CurrentRevision != 6 || len(split.Groups) != 2 ||
		!reflect.DeepEqual(split.Groups[0].RelatedHypothesisIDs, []uuid.UUID{h2.HypothesisID}) ||
		!reflect.DeepEqual(split.Groups[1].RelatedEpisodeIDs, []uuid.UUID{ep.EpisodeID}) ||
		!reflect.DeepEqual(split.Groups[1].RelatedFactIDs, []uuid.UUID{factA.ID}) ||
		len(split.Groups[1].RelatedHypothesisIDs) != 0 || split.Groups[1].DisplayTitle != "shoulder pain" ||
		split.Groups[1].ID != bridged.ID {
		t.Fatalf("after withdrawal: %#v", split)
	}
	var revisions int64
	if err := db.Table("body_state_revisions").Where("user_id=?", owner).Count(&revisions).Error; err != nil || revisions != 6 {
		t.Fatalf("derived read wrote revisions: %d %v", revisions, err)
	}
}
