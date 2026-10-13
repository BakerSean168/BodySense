package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func episodeAssociationCommand(fact uuid.UUID, revision int64, key string) model.EpisodeAssociationCommand {
	return model.EpisodeAssociationCommand{IdempotencyKey: key, ExpectedRevision: revision, FactID: fact}
}

func TestSymptomEpisodeExplicitCreateLinkAndReceiptPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	firstText := "I experienced shoulder discomfort yesterday"
	firstFactCommit := episodeRecordSymptom(t, repo, owner, 0, firstText, "present", "2026-10-12")
	first := episodeLookupFact(t, db, owner, firstText)
	create := episodeAssociationCommand(first.ID, firstFactCommit.Revision, "explicit-episode-create")
	receipt, err := repo.CreateEpisode(ctx, owner, create)
	if err != nil || receipt == nil || receipt.Revision != 2 || receipt.ID == uuid.Nil || receipt.EpisodeID == uuid.Nil ||
		receipt.RevisionID == uuid.Nil || receipt.EventID == uuid.Nil || receipt.Operation != "create" {
		t.Fatalf("not a durable Episode commit: %#v %v", receipt, err)
	}
	var identity model.SymptomEpisode
	if err := db.Where("user_id=? AND id=?", owner, receipt.EpisodeID).First(&identity).Error; err != nil {
		t.Fatal(err)
	}
	if identity.SymptomType != "shoulder pain" || identity.IdentityStatus != "current" || identity.CreatedRevision != 2 {
		t.Fatalf("derived Episode identity incorrect: %#v", identity)
	}
	detail, err := repo.GetEpisode(ctx, owner, receipt.EpisodeID)
	if err != nil || detail.Projection.State != "unknown" || detail.Projection.Reason != "insufficient_evidence" ||
		detail.Projection.BasedOnRevision != 2 || len(detail.Links) != 1 || detail.Links[0].FactID != first.ID ||
		detail.Links[0].Role != "observation" {
		t.Fatalf("new Episode invented status/onset: %#v %v", detail, err)
	}

	replay, err := repo.CreateEpisode(ctx, owner, create)
	if err != nil || replay.ID != receipt.ID || replay.EpisodeID != receipt.EpisodeID || replay.Revision != 2 {
		t.Fatalf("retry created a new revision: %#v %v", replay, err)
	}
	collision := create
	collision.FactID = uuid.New()
	if _, err := repo.CreateEpisode(ctx, owner, collision); !errors.Is(err, model.ErrEpisodeIdempotencyConflict) {
		t.Fatalf("same key different fact ignored fingerprint: %v", err)
	}
	if _, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(first.ID, 1, "new-key-reused-fact")); !errors.Is(err, ErrBodyStateRevisionConflict) {
		t.Fatalf("stale expected revision was accepted: %v", err)
	}
	if _, err := repo.CreateEpisode(ctx, other, episodeAssociationCommand(first.ID, 0, "foreign-fact")); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("foreign user gained another user's symptom: %v", err)
	}

	otherText := "I still have shoulder discomfort today"
	secondCommit := episodeRecordSymptom(t, repo, owner, 2, otherText, "present", "2026-10-13")
	second := episodeLookupFact(t, db, owner, otherText)
	linked, err := repo.LinkEpisodeFact(ctx, owner, receipt.EpisodeID, episodeAssociationCommand(second.ID, secondCommit.Revision, "explicit-link"))
	if err != nil || linked.EpisodeID != receipt.EpisodeID || linked.FactID != second.ID || linked.Revision != 4 || linked.Operation != "link" {
		t.Fatalf("link did not commit atomically: %#v %v", linked, err)
	}
	linkReplay, err := repo.LinkEpisodeFact(ctx, owner, receipt.EpisodeID, episodeAssociationCommand(second.ID, 3, "explicit-link"))
	if err != nil || linkReplay.ID != linked.ID || linkReplay.Revision != 4 {
		t.Fatalf("link retry did not return original receipt: %#v %v", linkReplay, err)
	}
	loaded, err := repo.GetEpisode(ctx, owner, receipt.EpisodeID)
	if err != nil || len(loaded.Links) != 2 || loaded.Projection.State != "unknown" || loaded.Projection.BasedOnRevision != 4 {
		t.Fatalf("link forged active/resolved state: %#v %v", loaded, err)
	}
	if _, err := repo.LinkEpisodeFact(ctx, other, receipt.EpisodeID, episodeAssociationCommand(second.ID, 0, "foreign-episode")); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("cross-owner association was authorized: %v", err)
	}
	if _, err := repo.LinkEpisodeFact(ctx, owner, receipt.EpisodeID, episodeAssociationCommand(second.ID, 4, "duplicate-primary")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("fact admitted twice into primary history: %v", err)
	}
	for name, expected := range map[string]int64{"episode_command_receipts": 2, "symptom_episodes": 1, "episode_fact_links": 2, "episode_state_evidence": 0} {
		var count int64
		if err := db.Table(name).Where("user_id=?", owner).Count(&count).Error; err != nil || count != expected {
			t.Fatalf("%s=%d want=%d: %v", name, count, expected, err)
		}
	}
	var audit []model.BodyStateRevision
	if err := db.Where("user_id = ? AND change_type LIKE ?", owner, "episode.%").Find(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if len(audit) != 2 || strings.Contains(fmt.Sprintf("%v", audit), firstText) || strings.Contains(fmt.Sprintf("%v", audit), otherText) {
		t.Fatalf("source statement was copied into generic Episode audit: %#v", audit)
	}
	inventory, err := NewPrivacyErasureRepository(db).CountUserData(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	reported := int64(-1)
	for _, row := range inventory {
		if row.Name == "episode_command_receipts" {
			reported = row.Count
		}
	}
	if reported != 2 {
		t.Fatalf("Episode receipts absent from privacy inventory: %d", reported)
	}
	if err := db.Exec("DELETE FROM users WHERE id=?", owner).Error; err != nil {
		t.Fatalf("account cascade blocked by Episode command receipt: %v", err)
	}
	for _, name := range []string{"episode_command_receipts", "symptom_episodes", "episode_fact_links", "body_state_revisions", "body_state_outbox"} {
		var count int64
		if err := db.Table(name).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("erasure left %s rows=%d err=%v", name, count, err)
		}
	}
}

func TestSymptomEpisodeRejectsUnverifiedAndNegativeSourcePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	absenceText := "I have no shoulder discomfort today"
	episodeRecordSymptom(t, repo, owner, 0, absenceText, "absent", "2026-10-13")
	absent := episodeLookupFact(t, db, owner, absenceText)
	if _, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(absent.ID, 1, "negative-create")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("negative absence automatically created Episode: %v", err)
	}
	presentText := "I notice shoulder pain"
	episodeRecordSymptom(t, repo, owner, 1, presentText, "present", "2026-10-13")
	present := episodeLookupFact(t, db, owner, presentText)
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, present.SourceEvidenceID).
		Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(present.ID, 2, "blocked-source")); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("blocked evidence was admitted: %v", err)
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, present.SourceEvidenceID).
		Update("availability", "available").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.BodyFact{}).Where("user_id=? AND id=?", owner, present.ID).
		Update("assertion_status", "withdrawn").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(present.ID, 2, "withdrawn-source")); !errors.Is(err, model.ErrEpisodeFactUnavailable) {
		t.Fatalf("withdrawn Fact was admitted: %v", err)
	}
	invalid := episodeAssociationCommand(uuid.Nil, 2, "bad-id")
	if _, err := repo.CreateEpisode(ctx, owner, invalid); !errors.Is(err, model.ErrInvalidEpisodeEvidence) {
		t.Fatalf("nil source accepted: %v", err)
	}
	if _, err := repo.CreateEpisode(ctx, uuid.Nil, episodeAssociationCommand(present.ID, 0, "nil-owner")); !errors.Is(err, model.ErrInvalidEpisodeEvidence) {
		t.Fatalf("nil owner accepted: %v", err)
	}
	var count int64
	if err := db.Table("episode_command_receipts").Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("invalid sources created commits: %d %v", count, err)
	}
}

func TestSymptomEpisodeConcurrentRetryOnlyOneRevisionPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	text := "My shoulder aches this afternoon"
	episodeRecordSymptom(t, repo, owner, 0, text, "present", "2026-10-13")
	fact := episodeLookupFact(t, db, owner, text)
	cmd := episodeAssociationCommand(fact.ID, 1, "parallel-episode-key")
	const calls = 7
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	ids := make(chan uuid.UUID, calls)
	for range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := repo.CreateEpisode(context.Background(), owner, cmd)
			if err == nil && r != nil {
				ids <- r.ID
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		if err != nil {
			t.Errorf("same-key concurrent command failed: %v", err)
		}
	}
	var expected uuid.UUID
	for id := range ids {
		if expected != uuid.Nil && expected != id {
			t.Fatal("concurrent create returned different receipt IDs")
		}
		expected = id
	}
	if expected == uuid.Nil {
		t.Fatal("no concurrent command succeeded")
	}
	for _, name := range []string{"episode_command_receipts", "symptom_episodes", "episode_fact_links"} {
		var count int64
		if err := db.Table(name).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("concurrent %s=%d want=1 err=%v", name, count, err)
		}
	}
	var state model.BodyState
	if err := db.Where("user_id=?", owner).First(&state).Error; err != nil || state.CurrentRevision != 2 {
		t.Fatalf("concurrent commands wrote redundant revisions: %#v %v", state, err)
	}
}

func TestSymptomEpisodeCreateRollbackDoesNotReturnReceiptPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	text := "Today I have shoulder pain"
	episodeRecordSymptom(t, repo, owner, 0, text, "present", "2026-10-13")
	fact := episodeLookupFact(t, db, owner, text)
	failure := errors.New("test-failed-last-episode-receipt")
	const callback = "episode_receipt_forced_rollback_test"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "episode_command_receipts" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	receipt, err := repo.CreateEpisode(context.Background(), owner, episodeAssociationCommand(fact.ID, 1, "rollback-episode"))
	if receipt != nil || !errors.Is(err, failure) {
		t.Fatalf("failed commit returned receipt %#v err=%v", receipt, err)
	}
	for _, name := range []string{"episode_command_receipts", "symptom_episodes", "episode_fact_links"} {
		var count int64
		if err := db.Table(name).Where("user_id=?", owner).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("partial %s survived rollback: %d %v", name, count, err)
		}
	}
	var state model.BodyState
	if err := db.Where("user_id=?", owner).First(&state).Error; err != nil || state.CurrentRevision != 1 {
		t.Fatalf("failed receipt advanced BodyState revision: %#v %v", state, err)
	}
}

func TestSymptomEpisodeReceiptDowngradeRefusesDestructiveHistoryPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	text := "My shoulder is hurting"
	episodeRecordSymptom(t, repo, owner, 0, text, "present", "2026-10-13")
	fact := episodeLookupFact(t, db, owner, text)
	receipt, err := repo.CreateEpisode(context.Background(), owner, episodeAssociationCommand(fact.ID, 1, "no-lossy-down"))
	if err != nil || receipt == nil {
		t.Fatalf("episode create: %#v %v", receipt, err)
	}
	down, err := os.ReadFile("../../migrations/000008_episode_command_receipts.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec(string(down)).Error; err == nil || !strings.Contains(err.Error(), "cannot downgrade: episode command receipts exist") {
		t.Fatalf("Episode receipt loss was not refused: %v", err)
	}
}

func TestSymptomEpisodeIncompatibleLinkAndNegativeEvidencePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	ctx := context.Background()
	record := func(revision int64, statement, symptomType, side, presence string) model.BodyFact {
		t.Helper()
		details, _ := json.Marshal(model.SymptomFactDetails{
			SymptomType: symptomType, Laterality: side, Presence: presence,
		})
		receipt, err := repo.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{
			IdempotencyKey: uuid.NewString(), ExpectedRevision: revision,
			Facts: []model.BodyFactInput{{
				Kind: "symptom", Statement: statement, BodyRegionIDs: []string{},
				Temporal: model.FactTemporal{Precision: "unknown"}, Details: details,
			}},
		})
		if err != nil || receipt == nil {
			t.Fatalf("fixture: %#v %v", receipt, err)
		}
		return episodeLookupFact(t, db, owner, statement)
	}
	first := record(0, "Right shoulder is sore", "shoulder pain", "right", "present")
	created, err := repo.CreateEpisode(ctx, owner, episodeAssociationCommand(first.ID, 1, "first-right"))
	if err != nil {
		t.Fatal(err)
	}
	left := record(2, "Left shoulder is sore", "shoulder pain", "left", "present")
	if _, err := repo.LinkEpisodeFact(ctx, owner, created.EpisodeID,
		episodeAssociationCommand(left.ID, 3, "wrong-side")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("incompatible side auto-linked to wrong Episode: %v", err)
	}
	otherType := record(3, "Migraine headache is back", "migraine", "right", "present")
	if _, err := repo.LinkEpisodeFact(ctx, owner, created.EpisodeID,
		episodeAssociationCommand(otherType.ID, 4, "wrong-type")); !errors.Is(err, model.ErrEpisodeAssociationConflict) {
		t.Fatalf("unrelated symptom type auto-linked to existing Episode: %v", err)
	}
	negative := record(4, "Today my right shoulder no longer hurts", "shoulder pain", "right", "absent")
	linked, err := repo.LinkEpisodeFact(ctx, owner, created.EpisodeID,
		episodeAssociationCommand(negative.ID, 5, "explicit-negative-observation"))
	if err != nil || linked.Revision != 6 {
		t.Fatalf("explicit negative symptom link failed: %#v %v", linked, err)
	}
	detail, err := repo.GetEpisode(ctx, owner, created.EpisodeID)
	if err != nil || detail.Projection.State != "unknown" || detail.Projection.Reason != "insufficient_evidence" ||
		len(detail.Links) != 2 {
		t.Fatalf("a negative symptom was incorrectly promoted to resolved: %#v %v", detail, err)
	}
	var state model.BodyState
	if err := db.Where("user_id=?", owner).First(&state).Error; err != nil || state.CurrentRevision != 6 {
		t.Fatalf("failed conflicting actions modified revision: %#v %v", state, err)
	}
}
