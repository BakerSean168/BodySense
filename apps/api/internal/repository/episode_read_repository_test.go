package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func episodeRecordSymptom(t *testing.T, repo *BodyStateRepository, owner uuid.UUID, expected int64, statement, presence, at string) model.BodyFactCommit {
	t.Helper()
	details, _ := json.Marshal(model.SymptomFactDetails{SymptomType: "shoulder pain", Presence: presence})
	command := model.RecordBodyFactsCommand{
		IdempotencyKey: uuid.NewString(), ExpectedRevision: expected,
		Facts: []model.BodyFactInput{{
			Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
			Temporal: model.FactTemporal{Precision: "day", Start: at}, Details: json.RawMessage(details),
		}},
	}
	receipt, err := repo.RecordDirectFacts(context.Background(), owner, command)
	if err != nil || receipt == nil {
		t.Fatalf("record symptom: %#v %v", receipt, err)
	}
	return *receipt
}

func episodeSeedIdentity(t *testing.T, db *gorm.DB, owner uuid.UUID, revision int64) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec(`INSERT INTO public.symptom_episodes
    (id,user_id,symptom_type,body_region_ids,laterality,identity_status,created_revision,created_at)
    VALUES (?,?,?,?::jsonb,'unknown','current',?,?)`,
		id, owner, "shoulder pain", "[]", revision, time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func episodeSeedLink(t *testing.T, db *gorm.DB, owner, episode uuid.UUID, fact model.BodyFact, role string, revision int64) {
	t.Helper()
	if err := db.Exec(`INSERT INTO public.episode_fact_links
    (user_id,episode_id,fact_id,fact_kind,basis_evidence_id,role,association_method,is_primary,created_revision)
    VALUES (?,?,?,'symptom',?,?,'explicit',true,?)`,
		owner, episode, fact.ID, fact.SourceEvidenceID, role, revision).Error; err != nil {
		t.Fatal(err)
	}
}

func episodeSeedClaim(t *testing.T, db *gorm.DB, owner, episode uuid.UUID, fact model.BodyFact, claim string, revision int64) {
	t.Helper()
	if err := db.Exec(`INSERT INTO public.episode_state_evidence
    (user_id,episode_id,fact_id,source_evidence_id,claim,as_of,created_revision)
    VALUES (?,?,?,?,?,?::jsonb,?)`,
		owner, episode, fact.ID, fact.SourceEvidenceID, claim, string(fact.Temporal), revision).Error; err != nil {
		t.Fatal(err)
	}
}

func episodeLookupFact(t *testing.T, db *gorm.DB, owner uuid.UUID, statement string) model.BodyFact {
	t.Helper()
	var fact model.BodyFact
	if err := db.Where("user_id = ? AND statement = ?", owner, statement).First(&fact).Error; err != nil {
		t.Fatal(err)
	}
	return fact
}

func TestSymptomEpisodeReadOwnerEvidenceAndRevocationPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	startText := "My shoulder hurts on Oct 1"
	endText := "That shoulder episode completely ended on Oct 4"
	episodeRecordSymptom(t, repo, owner, 0, startText, "present", "2026-10-01")
	endReceipt := episodeRecordSymptom(t, repo, owner, 1, endText, "absent", "2026-10-04")
	start := episodeLookupFact(t, db, owner, startText)
	end := episodeLookupFact(t, db, owner, endText)
	episode := episodeSeedIdentity(t, db, owner, endReceipt.Revision)
	episodeSeedLink(t, db, owner, episode, start, "onset", endReceipt.Revision)
	episodeSeedLink(t, db, owner, episode, end, "resolution", endReceipt.Revision)
	episodeSeedClaim(t, db, owner, episode, start, "explicit_current_report", endReceipt.Revision)
	episodeSeedClaim(t, db, owner, episode, end, "explicit_resolution", endReceipt.Revision)

	page, err := repo.ListEpisodes(context.Background(), owner, 0)
	if err != nil || page.UserID != owner || page.CurrentRevision != 2 || len(page.Episodes) != 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	entry := page.Episodes[0]
	if entry.ID != episode || entry.UserID != owner || entry.IdentityStatus != "current" ||
		entry.Projection.State != "ended" || entry.Projection.Reason != "explicit_resolution" ||
		entry.Projection.StateAsOf != "2026-10-04" || entry.Projection.BasedOnRevision != 2 ||
		len(entry.Projection.BasisFactIDs) != 2 || len(entry.Links) != 2 {
		t.Fatalf("evidence-backed ended projection=%#v", entry)
	}
	// Read models must contain only IDs, not user health statements or source excerpts.
	encoded, _ := json.Marshal(entry)
	if strings.Contains(string(encoded), startText) || strings.Contains(string(encoded), endText) {
		t.Fatal("episode projection copied private source statements")
	}

	read, err := repo.GetEpisode(context.Background(), owner, episode)
	if err != nil || read.ID != episode || read.Projection.State != "ended" {
		t.Fatalf("one-record read=%#v err=%v", read, err)
	}
	if foreign, err := repo.GetEpisode(context.Background(), other, episode); !errors.Is(err, ErrEpisodeNotFound) || foreign != nil {
		t.Fatalf("foreign owner leaked another episode: %#v %v", foreign, err)
	}
	otherPage, err := repo.ListEpisodes(context.Background(), other, 0)
	if err != nil || len(otherPage.Episodes) != 0 {
		t.Fatalf("foreign page=%#v %v", otherPage, err)
	}

	// A blocked source is not visible even as an old Fact ID, and cannot
	// support an ended projection. The still-valid onset supports ongoing only.
	if err := db.Model(&model.StatementEvidence{}).Where("user_id = ? AND id = ?", owner, end.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	blocked, err := repo.GetEpisode(context.Background(), owner, episode)
	if err != nil || blocked.Projection.State != "ongoing" || len(blocked.Links) != 1 ||
		blocked.Links[0].FactID != start.ID || len(blocked.Projection.BasisFactIDs) != 1 {
		t.Fatalf("blocked source still drives/readable in projection: %#v %v", blocked, err)
	}

	// Direct correction/withdrawal is a semantic revision, not time passing.
	changed, err := repo.WithdrawDirectFact(context.Background(), owner, start.ID, model.AmendBodyFactCommand{
		IdempotencyKey: "withdraw-episode-onset", ExpectedRevision: 2,
	})
	if err != nil || changed.Revision != 3 {
		t.Fatalf("withdraw= %#v %v", changed, err)
	}
	invalidated, err := repo.GetEpisode(context.Background(), owner, episode)
	if err != nil || invalidated.IdentityStatus != "invalidated" || invalidated.Projection.State != "unknown" ||
		invalidated.Projection.Reason != "insufficient_evidence" || invalidated.Projection.BasedOnRevision != 3 ||
		len(invalidated.Links) != 0 || len(invalidated.Projection.BasisFactIDs) != 0 {
		t.Fatalf("sole remaining active source withdrawn: %#v %v", invalidated, err)
	}
	// Source-unavailability must never affect another authenticated user.
	again, err := repo.ListEpisodes(context.Background(), other, 0)
	if err != nil || len(again.Episodes) != 0 {
		t.Fatalf("other owner was affected: %#v %v", again, err)
	}

	// Privacy source blocking must also hide the Episode's own symptom label.
	// An ID that no longer has any available original source is not a
	// source-free alternate health record that survives deletion controls.
	if err := db.Model(&model.StatementEvidence{}).
		Where("user_id = ? AND id = ?", owner, start.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	if item, err := repo.GetEpisode(context.Background(), owner, episode); !errors.Is(err, ErrEpisodeNotFound) || item != nil {
		t.Fatalf("fully blocked identity leaked: %#v %v", item, err)
	}
	hidden, err := repo.ListEpisodes(context.Background(), owner, 0)
	if err != nil || len(hidden.Episodes) != 0 {
		t.Fatalf("fully blocked episode was listed: %#v %v", hidden, err)
	}
}

func TestSymptomEpisodeReadLinkWithoutClaimIsUnknownPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	receipt := episodeRecordSymptom(t, repo, owner, 0, "I have no headache today", "absent", "2026-10-13")
	fact := episodeLookupFact(t, db, owner, "I have no headache today")
	episode := episodeSeedIdentity(t, db, owner, receipt.Revision)
	episodeSeedLink(t, db, owner, episode, fact, "observation", receipt.Revision)
	entry, err := repo.GetEpisode(context.Background(), owner, episode)
	if err != nil || entry.Projection.State != "unknown" || entry.Projection.Reason != "insufficient_evidence" || len(entry.Links) != 1 {
		t.Fatalf("a negative symptom is not explicit episode resolution: %#v %v", entry, err)
	}
}

type episodeReadSnapshotBarrierKey struct{}

func TestSymptomEpisodeReadRemainsConsistentAcrossSourceBlockPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	statement := "My headache episode explicitly ended"
	receipt := episodeRecordSymptom(t, repo, owner, 0, statement, "absent", "2026-10-13")
	fact := episodeLookupFact(t, db, owner, statement)
	episode := episodeSeedIdentity(t, db, owner, receipt.Revision)
	episodeSeedLink(t, db, owner, episode, fact, "resolution", receipt.Revision)
	episodeSeedClaim(t, db, owner, episode, fact, "explicit_resolution", receipt.Revision)

	readReached, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	const callback = "episode_read_test_source_snapshot_barrier"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "body_states" && tx.Statement.Context.Value(episodeReadSnapshotBarrierKey{}) == true {
			once.Do(func() { close(readReached); <-release })
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })

	type outcome struct {
		record *model.EpisodeRecord
		err    error
	}
	result := make(chan outcome, 1)
	readCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		record, err := repo.GetEpisode(context.WithValue(readCtx, episodeReadSnapshotBarrierKey{}, true), owner, episode)
		result <- outcome{record, err}
	}()
	select {
	case <-readReached:
	case <-readCtx.Done():
		close(release)
		t.Fatal("Episode first-snapshot barrier did not trigger")
	}
	// The source becomes unavailable in a committed second transaction
	// between the aggregate and link queries of the original reader.
	mutation := db.Model(&model.StatementEvidence{}).
		Where("user_id = ? AND id = ?", owner, fact.SourceEvidenceID).
		Update("availability", "blocked")
	close(release)
	if mutation.Error != nil {
		t.Fatal(mutation.Error)
	}
	var previous outcome
	select {
	case previous = <-result:
	case <-readCtx.Done():
		t.Fatal("Episode snapshot reader timed out")
	}
	if previous.err != nil || previous.record == nil ||
		previous.record.Projection.State != "ended" || len(previous.record.Links) != 1 {
		t.Fatalf("partially mixed revision and source snapshot: %#v", previous)
	}
	if current, err := repo.GetEpisode(context.Background(), owner, episode); !errors.Is(err, ErrEpisodeNotFound) || current != nil {
		t.Fatalf("fresh read ignored source blocking: %#v %v", current, err)
	}
}

func TestSymptomEpisodeReadPaginationKeepsWholeRevisionsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	// More than twenty separate domain revisions: second page must not repeat
	// or discard a semantically complete revision batch.
	for i := int64(0); i < 21; i++ {
		statement := fmt.Sprintf("Shoulder discomfort report %d", i)
		receipt := episodeRecordSymptom(t, repo, owner, i, statement, "present", "2026-10-01")
		fact := episodeLookupFact(t, db, owner, statement)
		episode := episodeSeedIdentity(t, db, owner, receipt.Revision)
		episodeSeedLink(t, db, owner, episode, fact, "observation", receipt.Revision)
	}
	first, err := repo.ListEpisodes(context.Background(), owner, 0)
	if err != nil || first.CurrentRevision != 21 || len(first.Episodes) != 20 || !first.HasMore ||
		first.NextBeforeRevision == nil || *first.NextBeforeRevision != 2 {
		t.Fatalf("page one incorrect: %#v %v", first, err)
	}
	second, err := repo.ListEpisodes(context.Background(), owner, *first.NextBeforeRevision)
	if err != nil || len(second.Episodes) != 1 || second.HasMore || second.Episodes[0].CreatedRevision != 1 {
		t.Fatalf("page two incorrect: %#v %v", second, err)
	}
	if _, err := repo.ListEpisodes(context.Background(), uuid.Nil, 0); !errors.Is(err, model.ErrInvalidEpisodeEvidence) {
		t.Fatalf("nil owner: %v", err)
	}
	if _, err := repo.GetEpisode(context.Background(), owner, uuid.Nil); !errors.Is(err, model.ErrInvalidEpisodeEvidence) {
		t.Fatalf("nil identity: %v", err)
	}
}
