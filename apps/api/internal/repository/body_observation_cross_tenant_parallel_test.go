package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// Concurrent tenants can reuse an identical key and payload without sharing a
// persisted receipt. Each phase is started together, then replayed together;
// the commit identities must remain tenant-local and stable across retries.
func TestBodyObservationConcurrentCrossTenantExactReplayPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	repo := NewBodyStateRepository(db)
	const tenants = 4
	var owners [tenants]uuid.UUID
	for i := range owners {
		owners[i] = seedBodyStateContextUser(t, db)
	}

	record := observationDirectCommand(0, "parallel-shared-record", "I measured 67.5 kg")
	replacement := record.Observation
	replacement.Statement = "Correction: I measured 65.5 kg"
	replacement.RawValue = "65.5"
	correct := model.AmendBodyObservationCommand{
		IdempotencyKey: "parallel-shared-correct", ExpectedRevision: 1,
		Replacement: &replacement,
	}
	withdraw := model.AmendBodyObservationCommand{
		IdempotencyKey: "parallel-shared-withdraw", ExpectedRevision: 2,
	}
	type result struct {
		index int
		value *model.BodyObservationCommit
		err   error
	}
	runConcurrent := func(label string, execute func(int) (*model.BodyObservationCommit, error)) [tenants]*model.BodyObservationCommit {
		t.Helper()
		start := make(chan struct{})
		results := make(chan result, tenants)
		for i := range owners {
			go func(index int) {
				<-start
				value, err := execute(index)
				results <- result{index: index, value: value, err: err}
			}(i)
		}
		close(start)
		var commits [tenants]*model.BodyObservationCommit
		var failure error
		for range owners {
			r := <-results
			if r.err != nil || r.value == nil {
				if failure == nil {
					failure = fmt.Errorf("%s tenant %d: commit=%#v err=%v", label, r.index, r.value, r.err)
				}
				continue
			}
			commits[r.index] = r.value
		}
		if failure != nil {
			t.Fatal(failure)
		}
		return commits
	}

	records := runConcurrent("record", func(i int) (*model.BodyObservationCommit, error) {
		return repo.RecordDirectObservation(context.Background(), owners[i], record)
	})
	corrections := runConcurrent("correct", func(i int) (*model.BodyObservationCommit, error) {
		return repo.CorrectDirectObservation(context.Background(), owners[i], records[i].ObservationID, correct)
	})
	withdrawals := runConcurrent("withdraw", func(i int) (*model.BodyObservationCommit, error) {
		return repo.WithdrawDirectObservation(context.Background(), owners[i], corrections[i].ObservationID, withdraw)
	})
	replayed := [3][tenants]*model.BodyObservationCommit{
		runConcurrent("record replay", func(i int) (*model.BodyObservationCommit, error) {
			return repo.RecordDirectObservation(context.Background(), owners[i], record)
		}),
		runConcurrent("correction replay", func(i int) (*model.BodyObservationCommit, error) {
			return repo.CorrectDirectObservation(context.Background(), owners[i], records[i].ObservationID, correct)
		}),
		runConcurrent("withdrawal replay", func(i int) (*model.BodyObservationCommit, error) {
			return repo.WithdrawDirectObservation(context.Background(), owners[i], corrections[i].ObservationID, withdraw)
		}),
	}
	originals := [3][tenants]*model.BodyObservationCommit{records, corrections, withdrawals}
	operations := [3]string{"record", "correct", "withdraw"}
	for step, group := range originals {
		for i, first := range group {
			retry := replayed[step][i]
			if first.UserID != owners[i] || first.ID == uuid.Nil ||
				first.Operation != operations[step] ||
				first.Revision != int64(step+1) || first.RevisionID == uuid.Nil ||
				first.EventID == uuid.Nil || first.ObservationID == uuid.Nil ||
				first.SourceEvidenceID == uuid.Nil {
				t.Fatalf("phase %d tenant %d invalid committed identity: %#v", step, i, first)
			}
			if retry.ID != first.ID || retry.UserID != first.UserID ||
				retry.Operation != first.Operation ||
				retry.Revision != first.Revision || retry.RevisionID != first.RevisionID ||
				retry.EventID != first.EventID || retry.ObservationID != first.ObservationID ||
				retry.SourceEvidenceID != first.SourceEvidenceID ||
				!retry.CommittedAt.Equal(first.CommittedAt) {
				t.Fatalf("phase %d tenant %d replay changed receipt: first=%#v retry=%#v", step, i, first, retry)
			}
			for other := i + 1; other < tenants; other++ {
				second := group[other]
				if first.UserID == second.UserID || first.ID == second.ID ||
					first.RevisionID == second.RevisionID || first.EventID == second.EventID ||
					first.ObservationID == second.ObservationID ||
					first.SourceEvidenceID == second.SourceEvidenceID {
					t.Fatalf("phase %d tenants %d and %d shared persistent identity: %#v %#v", step, i, other, first, second)
				}
			}
		}
	}
	for i, owner := range owners {
		page, err := repo.ListDirectObservations(context.Background(), owner, 0)
		if err != nil || page == nil || page.CurrentRevision != 3 || len(page.Observations) != 0 {
			t.Fatalf("tenant %d withdrawal projection: %#v err=%v", i, page, err)
		}
		for table, want := range map[string]int64{
			"body_observation_commits": 3,
			"body_state_revisions":     3,
			"body_state_outbox":        3,
			"body_observations":        2,
			"statement_evidence":       2,
		} {
			var got int64
			if err := db.Table(table).Where("user_id = ?", owner).Count(&got).Error; err != nil || got != want {
				t.Fatalf("tenant %d %s count=%d want=%d err=%v", i, table, got, want, err)
			}
		}
	}
}
