package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Review/freshness policies decode V2 revision metadata by these keys and
// resolve the ids through GetBodyFactsByIDs; this pins both contracts.
func TestBodyStateRevisionReadsOwnerIsolationPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	empty := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()

	head, err := repo.GetBodyStateHead(ctx, empty)
	if err != nil || head.CurrentRevision != 0 || head.UserID != empty {
		t.Fatalf("empty head: %#v %v", head, err)
	}

	commit, err := repo.RecordDirectFacts(ctx, owner, bodyFactTestCommand("revision-read-facts", 0))
	if err != nil {
		t.Fatal(err)
	}
	var factIDs []uuid.UUID
	if err := json.Unmarshal(commit.FactIDs, &factIDs); err != nil || len(factIDs) != 2 {
		t.Fatalf("fact ids: %s %v", commit.FactIDs, err)
	}
	foreignCommit, err := repo.RecordDirectFacts(ctx, foreign, bodyFactTestCommand("revision-read-foreign", 0))
	if err != nil {
		t.Fatal(err)
	}
	var foreignIDs []uuid.UUID
	if err := json.Unmarshal(foreignCommit.FactIDs, &foreignIDs); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.WithdrawDirectFact(ctx, owner, factIDs[0], model.AmendBodyFactCommand{IdempotencyKey: "revision-read-withdraw", ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}

	head, err = repo.GetBodyStateHead(ctx, owner)
	if err != nil || head.CurrentRevision != 2 {
		t.Fatalf("owner head: %#v %v", head, err)
	}

	revisions, err := repo.ListRevisionsAfter(ctx, owner, 0, 10)
	if err != nil || len(revisions) != 2 || revisions[0].ChangeType != "facts.recorded" || revisions[1].ChangeType != "facts.withdrawn" {
		t.Fatalf("revisions: %#v %v", revisions, err)
	}
	var recorded struct {
		FactIDs []uuid.UUID `json:"fact_ids"`
	}
	if err := json.Unmarshal(revisions[0].Changes, &recorded); err != nil || len(recorded.FactIDs) != 2 {
		t.Fatalf("facts.recorded metadata: %s %v", revisions[0].Changes, err)
	}
	var withdrawn struct {
		TargetFactID uuid.UUID `json:"target_fact_id"`
	}
	if err := json.Unmarshal(revisions[1].Changes, &withdrawn); err != nil || withdrawn.TargetFactID != factIDs[0] {
		t.Fatalf("facts.withdrawn metadata: %s %v", revisions[1].Changes, err)
	}

	facts, err := repo.GetBodyFactsByIDs(ctx, owner, []uuid.UUID{factIDs[0], factIDs[1], foreignIDs[0]})
	if err != nil || len(facts) != 2 {
		t.Fatalf("owner lookup must resolve own facts (withdrawn included) and nothing foreign: %#v %v", facts, err)
	}
	statuses := map[uuid.UUID]string{}
	for _, fact := range facts {
		if fact.UserID != owner {
			t.Fatalf("foreign fact leaked: %#v", fact)
		}
		statuses[fact.ID] = fact.AssertionStatus
	}
	if statuses[factIDs[0]] != "withdrawn" || statuses[factIDs[1]] != "asserted" {
		t.Fatalf("assertion statuses: %#v", statuses)
	}
	if none, err := repo.GetBodyFactsByIDs(ctx, foreign, factIDs); err != nil || len(none) != 0 {
		t.Fatalf("foreign owner resolved another user's facts: %#v %v", none, err)
	}
}
