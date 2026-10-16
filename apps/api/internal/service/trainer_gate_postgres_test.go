package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errRollbackTrainerGate = errors.New("rollback simulated safety policy")

// The service adapter reads the owner's native ledger in one snapshot and
// keeps every unscreened, malformed or foreign state closed.
func TestTrainerGateReadsNativeSafetyLedgerPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	stranger := seedOnboardingContextUser(t, db)
	repo := repository.NewBodyStateRepository(db)
	svc := NewBodyStateService(repo)
	ctx := context.Background()
	planA, planB := uuid.New(), uuid.New()
	squat := safety.Action{Kind: safety.ActionPerformSet, PlanID: planA, Exercises: []safety.Exercise{{ExerciseID: "squat"}}}
	row := safety.Action{Kind: safety.ActionPerformSet, PlanID: planA, Exercises: []safety.Exercise{{ExerciseID: "row"}}}

	receipt, err := repo.RecordDirectFacts(ctx, owner, model.RecordBodyFactsCommand{IdempotencyKey: uuid.NewString(), Facts: []model.BodyFactInput{
		{Kind: "symptom", Statement: "synthetic knee statement", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present","pattern":"unknown"}`)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var factIDs []uuid.UUID
	if err := json.Unmarshal(receipt.FactIDs, &factIDs); err != nil || len(factIDs) != 1 {
		t.Fatalf("fact ids %s: %v", receipt.FactIDs, err)
	}
	now := time.Now().UTC()
	if d := svc.EvaluateTrainerGate(ctx, owner, squat, now); d.Permits() || d.Result != safety.ResultBlock || len(d.Reasons) != 1 || d.Reasons[0] != safety.ReasonScreeningIncomplete {
		t.Fatalf("unscreened symptom: %+v", d)
	}
	// A user with no BodyState at all is not "screened clean".
	if d := svc.EvaluateTrainerGate(ctx, stranger, row, now); d.Permits() || len(d.MatchedRestrictionIDs) != 0 {
		t.Fatalf("empty state allowed: %+v", d)
	}

	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, CaptureID: "0123456789abcdef01234567", BodyFactID: factIDs[0], SourceConversationID: uuid.New(), SourceRunID: uuid.New(), SourceMessageID: uuid.New(), SourceInteractionID: uuid.New(), SourceToolCallID: "intake-0123456789abcdef01234567", Numbness: true, BodyStateRevision: 1, RecordedAt: now}
	if err := db.Create(&screen).Error; err != nil {
		t.Fatal(err)
	}
	restriction := model.SafetyRestriction{ID: uuid.New(), UserID: owner, SafetyScreenID: screen.ID, Concept: "numbness", PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training", LiftPolicy: "professional_clearance", Reason: "synthetic numbness", CreatedRevision: 1, ValidFrom: now}
	if err := db.Create(&restriction).Error; err != nil {
		t.Fatal(err)
	}
	d := svc.EvaluateTrainerGate(ctx, owner, row, now)
	if d.Permits() || d.Result != safety.ResultBlock || len(d.MatchedRestrictionIDs) != 1 || d.MatchedRestrictionIDs[0] != restriction.ID || d.SafetyRevision != 1 {
		t.Fatalf("screened global restriction: %+v", d)
	}
	// Owner isolation: another user's gate neither sees nor reports this risk.
	if d := svc.EvaluateTrainerGate(ctx, stranger, row, now); d.Permits() || len(d.MatchedRestrictionIDs) != 0 {
		t.Fatalf("foreign restriction leaked: %+v", d)
	}

	// A later reviewed policy (simulated in a rolled-back transaction) may
	// narrow a restriction; the gate must honour plan and exercise scope.
	err = db.Transaction(func(tx *gorm.DB) error {
		scoped := NewBodyStateService(repository.NewBodyStateRepository(tx))
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{"ALTER TABLE safety_restrictions DROP CONSTRAINT safety_restrictions_screen_policy", nil},
			{"UPDATE safety_restrictions SET risk_level='advisory', scope_kind='exercise', plan_id=?, lift_policy='user_reverification' WHERE user_id=? AND id=?", []any{planA, owner, restriction.ID}},
			{"INSERT INTO safety_restriction_exercises(user_id, restriction_id, exercise_id) VALUES (?,?,'squat')", []any{owner, restriction.ID}},
		} {
			if err := tx.Exec(statement.sql, statement.args...).Error; err != nil {
				t.Fatal(err)
			}
		}
		if d := scoped.EvaluateTrainerGate(ctx, owner, squat, now); d.Result != safety.ResultWarn || !d.Permits() {
			t.Errorf("advisory exercise: %+v", d)
		}
		if d := scoped.EvaluateTrainerGate(ctx, owner, row, now); d.Result != safety.ResultAllow {
			t.Errorf("other exercise: %+v", d)
		}
		other := squat
		other.PlanID = planB
		if d := scoped.EvaluateTrainerGate(ctx, owner, other, now); d.Result != safety.ResultAllow {
			t.Errorf("other plan: %+v", d)
		}
		custom := safety.Action{Kind: safety.ActionAdvanceExercise, PlanID: planA, Exercises: []safety.Exercise{{ExerciseID: "custom:my-squat", Custom: true}}}
		if d := scoped.EvaluateTrainerGate(ctx, owner, custom, now); d.Permits() || d.ScopeMatch != safety.ScopeMatchUnknown {
			t.Errorf("custom exercise: %+v", d)
		}
		// Exercise rows left on a now-global restriction make the ledger
		// unreadable: never allow.
		if err := tx.Exec("UPDATE safety_restrictions SET risk_level='urgent', scope_kind='all_training', plan_id=NULL, lift_policy='professional_clearance' WHERE user_id=? AND id=?", owner, restriction.ID).Error; err != nil {
			t.Fatal(err)
		}
		if d := scoped.EvaluateTrainerGate(ctx, owner, row, now); d.Permits() || d.Result != safety.ResultUnavailable {
			t.Errorf("incoherent exercise scope: %+v", d)
		}
		if err := tx.Exec("DELETE FROM safety_restriction_exercises WHERE user_id=?", owner).Error; err != nil {
			t.Fatal(err)
		}
		if d := scoped.EvaluateTrainerGate(ctx, owner, row, now); d.Result != safety.ResultBlock || !d.UrgentCare {
			t.Errorf("urgent restriction: %+v", d)
		}
		return errRollbackTrainerGate
	})
	if !errors.Is(err, errRollbackTrainerGate) {
		t.Fatal(err)
	}
}
