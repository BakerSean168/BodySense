package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func seedSafetyRestrictions(t *testing.T, db *gorm.DB, owner uuid.UUID) []model.SafetyRestriction {
	t.Helper()
	repo := NewBodyStateRepository(db)
	receipt, err := repo.RecordDirectFacts(context.Background(), owner, bodyFactTestCommand(uuid.NewString(), 0))
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	if err := json.Unmarshal(receipt.FactIDs, &ids); err != nil {
		t.Fatal(err)
	}
	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, CaptureID: "0123456789abcdef01234567", BodyFactID: ids[0], SourceConversationID: uuid.New(), SourceRunID: uuid.New(), SourceMessageID: uuid.New(), SourceInteractionID: uuid.New(), SourceToolCallID: "intake-0123456789abcdef01234567", Numbness: true, Weakness: true, BodyStateRevision: 1, RecordedAt: time.Now().UTC()}
	if err := db.Create(&screen).Error; err != nil {
		t.Fatal(err)
	}
	rows := screenRestrictions(screen)
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestSafetyResolutionOnlyLiftsExactRestrictionPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	parts, _ := json.Marshal([]map[string]string{{"type": "text", "text": model.ProfessionalClearanceStatement}})
	source := seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(parts), "user")
	cmd := model.ResolveSafetyRestrictionCommand{IdempotencyKey: "clear-one", ExpectedRevision: 1, SourceMessageID: source.MessageID, Statement: model.ProfessionalClearanceStatement}
	repo := NewBodyStateRepository(db)
	receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
	if err != nil || receipt == nil || receipt.RestrictionID != rows[0].ID || receipt.Revision != 2 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	replay, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
	if err != nil || replay.ID != receipt.ID {
		t.Fatalf("retry=%+v %v", replay, err)
	}
	assertBodyFactCount(t, db, "safety_resolutions", owner, 1)
	assertBodyFactCount(t, db, "safety_restrictions", owner, 2)
	var evidence model.StatementEvidence
	if err := db.Where("user_id=? AND id=?", owner, receipt.SourceEvidenceID).Take(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if evidence.SourceKind != "user_message" || evidence.Excerpt != cmd.Statement || evidence.SourceMessageID == nil || *evidence.SourceMessageID != source.MessageID {
		t.Fatalf("lost original user source: %+v", evidence)
	}
}

func clearanceCommand(t *testing.T, db *gorm.DB, owner uuid.UUID, role, text string) model.ResolveSafetyRestrictionCommand {
	t.Helper()
	parts, _ := json.Marshal([]map[string]string{{"type": "text", "text": text}})
	source := seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(parts), role)
	return model.ResolveSafetyRestrictionCommand{IdempotencyKey: uuid.NewString(), ExpectedRevision: 1, SourceMessageID: source.MessageID, Statement: model.ProfessionalClearanceStatement}
}

func TestSafetyResolutionRejectsInvalidSourcesAndStaleWritesPostgres(t *testing.T) {
	for _, name := range []string{"foreign-target", "foreign-source", "assistant", "missing-source", "ordinary-positive", "negated", "quoted", "stale", "changed-key", "already-lifted", "deleted-conversation", "predates-restriction", "reused-source"} {
		t.Run(name, func(t *testing.T) {
			db := openBodyStateContextIntegrationDB(t)
			owner := seedBodyStateContextUser(t, db)
			foreign := seedBodyStateContextUser(t, db)
			rows := seedSafetyRestrictions(t, db, owner)
			cmd := clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement)
			repo := NewBodyStateRepository(db)
			target := rows[0].ID
			want := model.ErrInvalidSafetyResolution
			switch name {
			case "foreign-target":
				target = seedSafetyRestrictions(t, db, foreign)[0].ID
				want = model.ErrSafetyRestrictionNotFound
			case "foreign-source":
				cmd = clearanceCommand(t, db, foreign, "user", model.ProfessionalClearanceStatement)
				want = model.ErrSafetyResolutionSourceNotFound
			case "assistant":
				cmd = clearanceCommand(t, db, owner, "assistant", model.ProfessionalClearanceStatement)
				want = model.ErrSafetyResolutionSourceNotFound
			case "missing-source":
				cmd.SourceMessageID = uuid.New()
				want = model.ErrSafetyResolutionSourceNotFound
			case "ordinary-positive":
				cmd = clearanceCommand(t, db, owner, "user", "我没事了")
			case "negated":
				cmd = clearanceCommand(t, db, owner, "user", "并不是"+model.ProfessionalClearanceStatement)
			case "quoted":
				cmd = clearanceCommand(t, db, owner, "user", "你说的“"+model.ProfessionalClearanceStatement+"”是什么意思？")
			case "stale":
				cmd.ExpectedRevision = 0
				want = ErrBodyStateRevisionConflict
			case "deleted-conversation":
				if err := db.Exec("UPDATE conversations SET deleted_at=now() WHERE id=(SELECT conversation_id FROM messages WHERE id=?)", cmd.SourceMessageID).Error; err != nil {
					t.Fatal(err)
				}
				want = model.ErrSafetyResolutionSourceNotFound
			case "predates-restriction":
				if err := db.Model(&model.Message{}).Where("id=?", cmd.SourceMessageID).Update("created_at", time.Now().Add(-24*time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			case "reused-source":
				if _, err := repo.ResolveSafetyRestriction(context.Background(), owner, target, cmd); err != nil {
					t.Fatal(err)
				}
				target = rows[1].ID
				cmd.IdempotencyKey = uuid.NewString()
				cmd.ExpectedRevision = 2
			case "changed-key", "already-lifted":
				if _, err := repo.ResolveSafetyRestriction(context.Background(), owner, target, cmd); err != nil {
					t.Fatal(err)
				}
				if name == "changed-key" {
					target = rows[1].ID
					want = model.ErrSafetyResolutionIdempotencyConflict
				} else {
					cmd.IdempotencyKey = uuid.NewString()
					want = model.ErrSafetyRestrictionAlreadyResolved
				}
			}
			receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, target, cmd)
			if !errors.Is(err, want) || receipt != nil {
				t.Fatalf("receipt=%+v err=%v want=%v", receipt, err, want)
			}
			wantCount := int64(0)
			if name == "changed-key" || name == "already-lifted" || name == "reused-source" {
				wantCount = 1
			}
			assertBodyFactCount(t, db, "safety_resolutions", owner, wantCount)
		})
	}
}

func TestSafetyResolutionNativeReadReblocksInvalidatedSourcePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	repo := NewBodyStateRepository(db)
	receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement))
	if err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListSafetyRestrictions(context.Background(), owner)
	if err != nil || list.CurrentRevision != 2 || len(list.Restrictions) != 2 {
		t.Fatalf("list=%+v %v", list, err)
	}
	active := 0
	for _, row := range list.Restrictions {
		if row.Active {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("lifting one restriction affected overlap: %+v", list)
	}
	if err := db.Model(&model.StatementEvidence{}).Where("user_id=? AND id=?", owner, receipt.SourceEvidenceID).Update("availability", "blocked").Error; err != nil {
		t.Fatal(err)
	}
	list, err = repo.ListSafetyRestrictions(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range list.Restrictions {
		if !row.Active {
			t.Fatalf("invalidated clearance allowed training: %+v", row)
		}
	}
}

func TestSafetyResolutionOutboxFailureRollsBackPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	repo := NewBodyStateRepository(db)
	injected := errors.New("synthetic safety outbox unavailable")
	const callback = "test:safety_resolution_outbox_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "body_state_outbox" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement))
	if !errors.Is(err, injected) || receipt != nil {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	assertBodyFactCount(t, db, "safety_resolutions", owner, 0)
	assertBodyFactCount(t, db, "statement_evidence", owner, 2)
	assertBodyFactCount(t, db, "body_state_revisions", owner, 1)
}

func TestSafetyResolutionConcurrentDistinctLiftsRequireFreshRevisionPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	repo := NewBodyStateRepository(db)
	commands := []model.ResolveSafetyRestrictionCommand{clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement), clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement)}
	results := make(chan error, 2)
	start := make(chan struct{})
	for i := range 2 {
		go func(i int) {
			<-start
			_, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[i].ID, commands[i])
			results <- err
		}(i)
	}
	close(start)
	passed, conflicted := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			passed++
		case errors.Is(err, ErrBodyStateRevisionConflict):
			conflicted++
		default:
			t.Fatal(err)
		}
	}
	if passed != 1 || conflicted != 1 {
		t.Fatalf("passed=%d conflicted=%d", passed, conflicted)
	}
	assertBodyFactCount(t, db, "safety_resolutions", owner, 1)
}

func TestSafetyResolutionConcurrentRetryCommitsOncePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	repo := NewBodyStateRepository(db)
	cmd := clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement)
	type result struct {
		receipt *model.SafetyResolution
		err     error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
			results <- result{receipt, err}
		}()
	}
	close(start)
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || a.receipt.ID != b.receipt.ID {
		t.Fatalf("retry mismatch: %+v %+v", a, b)
	}
	assertBodyFactCount(t, db, "safety_resolutions", owner, 1)
	assertBodyFactCount(t, db, "body_state_revisions", owner, 2)
}

func TestSafetyResolutionAuditOwnerLineageAndAccountErasurePostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	other := seedSafetyRestrictions(t, db, foreign)
	repo := NewBodyStateRepository(db)
	receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*model.SafetyResolution){func(r *model.SafetyResolution) { r.RestrictionID = other[0].ID }, func(r *model.SafetyResolution) { r.SourceEvidenceID = uuid.New() }, func(r *model.SafetyResolution) { r.Revision = 1 }, func(r *model.SafetyResolution) { r.SourceLevel = "clinical_document" }} {
		forged := *receipt
		forged.ID = uuid.New()
		forged.RestrictionID = rows[1].ID
		forged.IdempotencyKey = uuid.NewString()
		forged.SourceMessageID = uuid.New()
		mutate(&forged)
		if err := db.Create(&forged).Error; err == nil {
			t.Fatalf("forged clearance accepted: %+v", forged)
		}
	}
	counts, err := NewPrivacyErasureRepository(db).CountUserData(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, count := range counts {
		if count.Name == "safety_resolutions" {
			found = true
			if count.Count != 1 {
				t.Fatalf("privacy count=%d", count.Count)
			}
		}
	}
	if !found {
		t.Fatal("missing clearance privacy inventory")
	}
	if err := db.Exec("DELETE FROM users WHERE id=?", owner).Error; err != nil {
		t.Fatal(err)
	}
	assertBodyFactCount(t, db, "safety_resolutions", owner, 0)
	assertBodyFactCount(t, db, "safety_restrictions", foreign, 2)
}
