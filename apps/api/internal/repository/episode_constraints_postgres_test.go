package repository

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestSymptomEpisodeDowngradeRefusesEvidencePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	command := messageFactTestCommand()
	command.IdempotencyKey = "episode-downgrade-source"
	receipt, err := NewBodyStateRepository(db).RecordDirectFacts(context.Background(), owner, command)
	if err != nil {
		t.Fatal(err)
	}
	episodeID := uuid.New()
	if err := db.Exec(`INSERT INTO public.symptom_episodes
      (id,user_id,symptom_type,body_region_ids,laterality,identity_status,created_revision,created_at)
      VALUES (?,?,?,?::jsonb,'unknown','current',?,?)`,
		episodeID, owner, "headache", "[]", receipt.Revision, time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000007_symptom_episode_evidence.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec(string(down)).Error; err == nil || !strings.Contains(err.Error(), "cannot downgrade: symptom episode identity or evidence exists") {
		t.Fatalf("lossy Episode rollback was not refused: %v", err)
	}
}

// Constraints are exercised on the actual vNext schema, not a GORM auto-migrate.
// No production Episode writer is exposed before explicit user association rules.
func TestSymptomEpisodeIdentityEvidenceConstraintsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	other := seedBodyStateContextUser(t, db)
	repository := NewBodyStateRepository(db)
	command := messageFactTestCommand()
	command.IdempotencyKey = "episode-constraint-fixture"
	receipt, err := repository.RecordDirectFacts(context.Background(), owner, command)
	if err != nil || receipt == nil {
		t.Fatalf("seed facts: %#v %v", receipt, err)
	}

	var symptom, lifestyle model.BodyFact
	if err := db.Where("user_id=? AND kind=?", owner, "symptom").First(&symptom).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id=? AND kind=?", owner, "lifestyle").First(&lifestyle).Error; err != nil {
		t.Fatal(err)
	}
	episodeID := uuid.New()
	insertEpisode := func(user, episode uuid.UUID) error {
		return db.Exec(`INSERT INTO public.symptom_episodes
      (id,user_id,symptom_type,body_region_ids,laterality,identity_status,created_revision,created_at)
      VALUES (?,?,?,?::jsonb,'unknown','current',?,?)`,
			episode, user, "shoulder discomfort", "[]", receipt.Revision, time.Now().UTC()).Error
	}
	if err := insertEpisode(owner, episodeID); err != nil {
		t.Fatal(err)
	}
	if err := insertEpisode(other, uuid.New()); err == nil {
		t.Fatal("foreign user cannot reuse another owner's revision")
	}
	otherCommand := messageFactTestCommand()
	otherCommand.IdempotencyKey = "another-owner-episode-source"
	if _, err := repository.RecordDirectFacts(context.Background(), other, otherCommand); err != nil {
		t.Fatal(err)
	}
	var otherSymptom model.BodyFact
	if err := db.Where("user_id=? AND kind=?", other, "symptom").First(&otherSymptom).Error; err != nil {
		t.Fatal(err)
	}
	insertLink := func(episode, fact, evidence uuid.UUID, primary bool, method string, auth *uuid.UUID) error {
		return db.Exec(`INSERT INTO public.episode_fact_links
      (user_id,episode_id,fact_id,fact_kind,basis_evidence_id,role,association_method,is_primary,authorization_id,created_revision)
      VALUES (?,?,?,'symptom',?,'observation',?,?,?,?)`,
			owner, episode, fact, evidence, method, primary, auth, receipt.Revision).Error
	}
	if err := insertLink(episodeID, symptom.ID, symptom.SourceEvidenceID, true, "explicit", nil); err != nil {
		t.Fatal(err)
	}
	if err := insertLink(episodeID, otherSymptom.ID, otherSymptom.SourceEvidenceID, false, "explicit", nil); err == nil {
		t.Fatal("another user's symptom cannot link to this owner's episode")
	}
	if err := insertLink(episodeID, lifestyle.ID, lifestyle.SourceEvidenceID, false, "explicit", nil); err == nil {
		t.Fatal("lifestyle fact must not be linkable as symptom")
	}
	if err := insertLink(episodeID, symptom.ID, uuid.New(), false, "explicit", nil); err == nil {
		t.Fatal("link must use the fact's actual evidence")
	}
	if err := insertLink(uuid.New(), symptom.ID, symptom.SourceEvidenceID, false, "explicit", nil); err == nil {
		t.Fatal("link must identify an existing episode")
	}

	anotherEpisode := uuid.New()
	if err := insertEpisode(owner, anotherEpisode); err != nil {
		t.Fatal(err)
	}
	if err := insertLink(anotherEpisode, symptom.ID, symptom.SourceEvidenceID, true, "explicit", nil); err == nil {
		t.Fatal("one fact cannot have two primary episode identities")
	}
	if err := insertLink(anotherEpisode, symptom.ID, symptom.SourceEvidenceID, false, "user_corrected", nil); err == nil {
		t.Fatal("a corrected association must carry user authorization")
	}
	if err := insertLink(anotherEpisode, symptom.ID, symptom.SourceEvidenceID, false, "explicit", nil); err != nil {
		t.Fatalf("evidence-backed secondary relation should be representable: %v", err)
	}

	insertState := func(episode, fact, evidence uuid.UUID, claim string) error {
		timeJSON, _ := json.Marshal(model.FactTemporal{Precision: "unknown"})
		return db.Exec(`INSERT INTO public.episode_state_evidence
      (user_id,episode_id,fact_id,source_evidence_id,claim,as_of,created_revision)
      VALUES (?,?,?,?,?,?::jsonb,?)`,
			owner, episode, fact, evidence, claim, string(timeJSON), receipt.Revision).Error
	}
	if err := insertState(episodeID, symptom.ID, symptom.SourceEvidenceID, "explicit_current_report"); err != nil {
		t.Fatal(err)
	}
	if err := insertState(anotherEpisode, lifestyle.ID, lifestyle.SourceEvidenceID, "explicit_resolution"); err == nil {
		t.Fatal("unlinked non-symptom fact cannot establish episode state")
	}
	if err := insertState(anotherEpisode, symptom.ID, uuid.New(), "explicit_resolution"); err == nil {
		t.Fatal("wrong evidence cannot create an episode status claim")
	}
	if err := insertState(anotherEpisode, symptom.ID, symptom.SourceEvidenceID, "inferred_resolution"); err == nil {
		t.Fatal("a guessed resolution cannot be persisted as a valid status claim")
	}
	var count int64
	if err := db.Table("public.episode_state_evidence").Where("user_id=?", owner).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unexpected accepted episode status count: %d", count)
	}
	inventory, err := NewPrivacyErasureRepository(db).CountUserData(context.Background(), owner)
	if err != nil {
		t.Fatalf("privacy inventory unavailable for new episode tables: %v", err)
	}
	expected := map[string]int64{"symptom_episodes": 2, "episode_fact_links": 2, "episode_state_evidence": 1}
	for _, item := range inventory {
		if want, ok := expected[item.Name]; ok {
			if item.Count != want {
				t.Fatalf("%s privacy count=%d want=%d", item.Name, item.Count, want)
			}
			delete(expected, item.Name)
		}
	}
	if len(expected) != 0 {
		t.Fatalf("episode data absent from privacy inventory: %v", expected)
	}
	// Account deletion must cascade the new projection identities and sources,
	// not leave protected health history orphaned behind FK restrictions.
	if err := db.Exec("DELETE FROM public.users WHERE id=?", owner).Error; err != nil {
		t.Fatalf("account deletion left episode records behind: %v", err)
	}
	for _, table := range []string{"symptom_episodes", "episode_fact_links", "episode_state_evidence"} {
		var rows int64
		if err := db.Table("public."+table).Where("user_id=?", owner).Count(&rows).Error; err != nil || rows != 0 {
			t.Fatalf("residual %s after user cascade: rows=%d err=%v", table, rows, err)
		}
	}
}
