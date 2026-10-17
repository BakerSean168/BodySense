package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestSafetyExpiryUsesGoClockAndExactVersionPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	withReviewedSafetyPolicy(t, db, func(tx *gorm.DB) {
		until := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
		if err := tx.Exec("UPDATE safety_restrictions SET lift_policy='on_expiry', valid_until=? WHERE user_id=? AND id=?", until, owner, rows[0].ID).Error; err != nil {
			t.Fatal(err)
		}
		now := until.Add(-time.Microsecond)
		repo := NewBodyStateRepository(tx)
		repo.safetyClock = func() time.Time { return now }
		cmd := model.ResolveSafetyRestrictionCommand{IdempotencyKey: "expire", ExpectedRevision: 1, ExpectedRestrictionVersion: 1, BasisKind: "expiry"}
		if receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd); receipt != nil || !errors.Is(err, model.ErrInvalidSafetyResolution) {
			t.Fatalf("early expiry=%+v %v", receipt, err)
		}
		now = until
		cmd.ExpectedRestrictionVersion = 2
		if _, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd); !errors.Is(err, ErrBodyStateRevisionConflict) {
			t.Fatalf("stale target=%v", err)
		}
		cmd.ExpectedRestrictionVersion = 1
		receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
		if err != nil || receipt == nil || receipt.Actor != "system" || receipt.SourceVersion != 0 || receipt.SourceMessageID != nil || receipt.SourceEvidenceID != nil || !receipt.LiftedAt.Equal(until) {
			t.Fatalf("expiry receipt=%+v %v", receipt, err)
		}
		retry, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
		if err != nil || retry.ID != receipt.ID {
			t.Fatalf("retry=%+v %v", retry, err)
		}
		list, err := repo.ListSafetyRestrictions(context.Background(), owner)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range list.Restrictions {
			if r.Active != (r.ID != rows[0].ID) {
				t.Fatalf("expiry affected wrong restriction %+v", r)
			}
		}
		assertBodyFactCount(t, tx, "statement_evidence", owner, 2)
	})
}

func seedSafetyReverificationAnswer(t *testing.T, db *gorm.DB, owner uuid.UUID, target uuid.UUID, choice string) (*model.AgentInteraction, *model.InteractionResumeEnvelope) {
	t.Helper()
	source := seedUserMessageFactEnvelope(t, db, owner, datatypes.JSON(`[{"type":"text","text":"Please review my restriction"}]`), "user")
	var run model.Run
	if err := db.Where("id=?", source.RunID).Take(&run).Error; err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	raw, _ := json.Marshal(map[string]any{"purpose": model.SafetyReverificationPurpose, "restriction_id": target, "restriction_version": 1, "question": "untrusted model prose", "fields": []any{}})
	interaction := &model.AgentInteraction{ID: uuid.New(), RunID: run.ID, ConversationID: run.ConversationID, ToolCallID: uuid.NewString(), ToolName: "ask_user", Question: raw, ExpiresAt: &expires}
	if err := NewAgentInteractionRepository(db).CreatePending(context.Background(), interaction); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&run).Update("status", model.RunStatusWaitingUser).Error; err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(map[string]any{"fields": map[string]string{"condition_0": choice}})
	cmd := model.ClaimInteractionResume{UserID: owner, ConversationID: run.ConversationID, InteractionID: interaction.ID, RequestID: uuid.NewString(), Answer: raw}
	var envelope *model.InteractionResumeEnvelope
	err := database.NewTransactionManager(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		envelope, err = NewConsultationRepository(db, "safety-lift-test").ClaimInteractionResume(ctx, cmd)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return interaction, envelope
}

func TestSafetyReverificationRequiresExactAcceptedAskUserPostgres(t *testing.T) {
	for _, name := range []string{"accepted", "negative", "unknown", "foreign-interaction", "foreign-source", "unbound", "changed-answer", "changed-question", "expired-answer", "pending", "wrong-source-version", "wrong-restriction-version"} {
		t.Run(name, func(t *testing.T) {
			db := openBodyStateContextIntegrationDB(t)
			owner := seedBodyStateContextUser(t, db)
			foreign := seedBodyStateContextUser(t, db)
			rows := seedSafetyRestrictions(t, db, owner)
			withReviewedSafetyPolicy(t, db, func(tx *gorm.DB) {
				if err := tx.Exec(`UPDATE safety_restrictions SET lift_policy='user_reverification', lift_conditions='["Test condition from reviewed policy"]' WHERE user_id=? AND id=?`, owner, rows[0].ID).Error; err != nil {
					t.Fatal(err)
				}
				choice := "是"
				if name == "negative" {
					choice = "否"
				}
				if name == "unknown" {
					choice = "不确定"
				}
				interaction, answer := seedSafetyReverificationAnswer(t, tx, owner, rows[0].ID, choice)
				cmd := model.ResolveSafetyRestrictionCommand{IdempotencyKey: "reverify", ExpectedRevision: 1, ExpectedRestrictionVersion: 1, BasisKind: "user_reverification", CheckInteractionID: interaction.ID, SourceMessageID: answer.UserMessage.ID, SourceVersion: 1}
				want := model.ErrInvalidSafetyResolution
				switch name {
				case "foreign-interaction":
					cmd.CheckInteractionID = uuid.New()
					want = model.ErrSafetyResolutionSourceNotFound
				case "foreign-source":
					cmd.SourceMessageID = clearanceCommand(t, tx, foreign, "user", model.ProfessionalClearanceStatement).SourceMessageID
					want = model.ErrSafetyResolutionSourceNotFound
				case "unbound":
					if err := tx.Model(interaction).Update("question", datatypes.JSON(`{"question":"feeling better?","answer_type":"text"}`)).Error; err != nil {
						t.Fatal(err)
					}
				case "changed-answer":
					if err := tx.Model(interaction).Update("answer", datatypes.JSON(`{"fields":{"condition_0":"否"}}`)).Error; err != nil {
						t.Fatal(err)
					}
				case "changed-question":
					if err := tx.Model(interaction).Update("question", datatypes.JSON(`{"question":"forged","answer_type":"text"}`)).Error; err != nil {
						t.Fatal(err)
					}
				case "expired-answer":
					if err := tx.Model(interaction).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
						t.Fatal(err)
					}
				case "pending":
					if err := tx.Model(interaction).Update("status", model.AgentInteractionPending).Error; err != nil {
						t.Fatal(err)
					}
				case "wrong-source-version":
					cmd.SourceVersion = 2
				case "wrong-restriction-version":
					cmd.ExpectedRestrictionVersion = 2
					want = ErrBodyStateRevisionConflict
				}
				repo := NewBodyStateRepository(tx)
				receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
				if name != "accepted" {
					if receipt != nil || !errors.Is(err, want) {
						t.Fatalf("receipt=%+v err=%v want=%v", receipt, err, want)
					}
					assertBodyFactCount(t, tx, "safety_resolutions", owner, 0)
					return
				}
				if err != nil || receipt == nil || receipt.CheckInteractionID == nil || *receipt.CheckInteractionID != interaction.ID {
					t.Fatalf("receipt=%+v err=%v", receipt, err)
				}
				replay, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
				if err != nil || replay.ID != receipt.ID {
					t.Fatalf("replay=%+v %v", replay, err)
				}
				list, err := repo.ListSafetyRestrictions(context.Background(), owner)
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range list.Restrictions {
					if r.Active != (r.ID != rows[0].ID) {
						t.Fatalf("wrong active state %+v", r)
					}
				}
				if err := tx.Model(interaction).Update("answer", datatypes.JSON(`{"fields":{"condition_0":"否"}}`)).Error; err != nil {
					t.Fatal(err)
				}
				list, err = repo.ListSafetyRestrictions(context.Background(), owner)
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range list.Restrictions {
					if !row.Active {
						t.Fatal("changed answer still clears restriction")
					}
				}
				err = tx.Transaction(func(inner *gorm.DB) error {
					if err := inner.Model(&model.StatementEvidence{}).Where("id=?", receipt.SourceEvidenceID).Update("source_version", 2).Error; err != nil {
						return err
					}
					return inner.Exec("SET CONSTRAINTS ALL IMMEDIATE").Error
				})
				if err == nil {
					t.Fatal("FK allowed evidence version rebinding")
				}

			})
		})
	}
}

// Adding a fresh screen and lifting one old restriction contend on the same
// aggregate revision. Neither order may drop the independently sourced risk.
func TestSafetyResolutionConcurrentNewScreenPreservesRestrictionsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	repo := NewBodyStateRepository(db)
	cmd := clearanceCommand(t, db, owner, "user", model.ProfessionalClearanceStatement)
	results := make(chan error, 2)
	start := make(chan struct{})
	go func() {
		<-start
		_, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd)
		results <- err
	}()
	var added uuid.UUID
	go func() {
		<-start
		err := db.Transaction(func(tx *gorm.DB) error {
			state, next, err := bodyStateLockNextRevision(context.Background(), tx, owner, nil)
			if err != nil {
				return err
			}
			var fact model.BodyFact
			if err := tx.Where("user_id=? AND kind='symptom'", owner).Take(&fact).Error; err != nil {
				return err
			}
			if _, err := bodyStatePersistRevision(context.Background(), tx, state, next, "safety.screened", "structured_answer", map[string]any{}); err != nil {
				return err
			}
			screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, CaptureID: "abcdef0123456789abcdef01", BodyFactID: fact.ID, SourceConversationID: uuid.New(), SourceRunID: uuid.New(), SourceMessageID: uuid.New(), SourceInteractionID: uuid.New(), SourceToolCallID: "intake-abcdef0123456789abcdef01", Trauma: true, BodyStateRevision: next, RecordedAt: time.Now().UTC()}
			if err := tx.Create(&screen).Error; err != nil {
				return err
			}
			restrictions := screenRestrictions(screen)
			added = restrictions[0].ID
			return tx.Create(&restrictions).Error
		})
		results <- err
	}()
	close(start)
	passed := 0
	for range 2 {
		err := <-results
		if err == nil {
			passed++
		} else if !errors.Is(err, ErrBodyStateRevisionConflict) {
			t.Fatal(err)
		}
	}
	if passed < 1 {
		t.Fatal("both commands failed")
	}
	list, err := repo.ListSafetyRestrictions(context.Background(), owner)
	if err != nil || len(list.Restrictions) != 3 {
		t.Fatalf("list=%+v %v", list, err)
	}
	for _, r := range list.Restrictions {
		if (r.ID == added || r.ID == rows[1].ID) && !r.Active {
			t.Fatal("concurrent unrelated risk lost")
		}
	}
}

func TestSafetyProfessionalClearancePreservesScopeAndConditionsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	withReviewedSafetyPolicy(t, db, func(tx *gorm.DB) {
		plan := uuid.New()
		if err := tx.Exec(`UPDATE safety_restrictions SET scope_kind='plan',plan_id=?,lift_conditions='["Explicit reviewed condition"]' WHERE user_id=? AND id=?`, plan, owner, rows[0].ID).Error; err != nil {
			t.Fatal(err)
		}
		repo := NewBodyStateRepository(tx)
		cmd := clearanceCommand(t, tx, owner, "user", model.ProfessionalClearanceStatement)
		if _, err := repo.ResolveSafetyRestriction(context.Background(), owner, rows[0].ID, cmd); !errors.Is(err, model.ErrInvalidSafetyResolution) {
			t.Fatalf("missing condition accepted: %v", err)
		}
		var target model.SafetyRestriction
		if err := tx.Where("user_id=? AND id=?", owner, rows[0].ID).Take(&target).Error; err != nil {
			t.Fatal(err)
		}
		statement := model.SafetyProfessionalClearanceStatement(target)
		cmd = clearanceCommand(t, tx, owner, "user", statement)
		cmd.Statement = statement
		receipt, err := repo.ResolveSafetyRestriction(context.Background(), owner, target.ID, cmd)
		if err != nil || receipt == nil || receipt.ScopeKind != "plan" || receipt.SourceLevel != "user_reported" {
			t.Fatalf("scope receipt=%+v %v", receipt, err)
		}
		list, err := repo.ListSafetyRestrictions(context.Background(), owner)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range list.Restrictions {
			if r.ID == target.ID && r.Active {
				t.Fatal("explicit condition clearance not read back")
			}
		}
	})
}
