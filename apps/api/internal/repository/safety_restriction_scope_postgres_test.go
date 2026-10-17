package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errRollbackSafetyScope = errors.New("rollback simulated safety policy")

// withReviewedSafetyPolicy runs fn in a rolled-back transaction in which the
// screen policy's mapping guard is lifted, standing in for a later reviewed Go
// policy revision. It lets the structural constraints be proven on their own.
func withReviewedSafetyPolicy(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB)) {
	t.Helper()
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("ALTER TABLE safety_restrictions DROP CONSTRAINT safety_restrictions_screen_policy").Error; err != nil {
			return err
		}
		fn(tx)
		return errRollbackSafetyScope
	})
	if !errors.Is(err, errRollbackSafetyScope) {
		t.Fatal(err)
	}
}

func rejectsInSavepoint(tx *gorm.DB, statement string, args ...any) bool {
	return tx.Transaction(func(inner *gorm.DB) error { return inner.Exec(statement, args...).Error }) != nil
}

// The screen policy may only persist its approved mapping: a forged urgent,
// scoped or self-lifting screen restriction is rejected by the baseline.
func TestSafetyRestrictionScreenPolicyMappingPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	target := seedSafetyRestrictions(t, db, owner)[0].ID
	for name, statement := range map[string]string{
		"advisory":            "UPDATE safety_restrictions SET risk_level='advisory' WHERE user_id=? AND id=?",
		"urgent":              "UPDATE safety_restrictions SET risk_level='urgent' WHERE user_id=? AND id=?",
		"exercise scope":      "UPDATE safety_restrictions SET scope_kind='exercise' WHERE user_id=? AND id=?",
		"self reverification": "UPDATE safety_restrictions SET lift_policy='user_reverification' WHERE user_id=? AND id=?",
		"review point":        "UPDATE safety_restrictions SET valid_until=valid_from + interval '1 day' WHERE user_id=? AND id=?",
		"unknown policy":      "UPDATE safety_restrictions SET policy_revision='agent-risk-v1' WHERE user_id=? AND id=?",
	} {
		t.Run(name, func(t *testing.T) {
			if !rejectsInSavepoint(db, statement, owner, target) {
				t.Fatalf("screen policy accepted %s", name)
			}
		})
	}
}

// The baseline enforces the structural Safety rules for every policy so a
// misbehaving writer cannot persist an incoherent restriction; the Go gate
// re-validates on read.
func TestSafetyRestrictionScopeConstraintsPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	rows := seedSafetyRestrictions(t, db, owner)
	foreignRows := seedSafetyRestrictions(t, db, foreign)
	target := rows[0].ID
	plan := uuid.New()
	withReviewedSafetyPolicy(t, db, func(tx *gorm.DB) {
		for name, statement := range map[string]string{
			"unknown level":           "UPDATE safety_restrictions SET risk_level='severe' WHERE user_id=? AND id=?",
			"unknown scope":           "UPDATE safety_restrictions SET scope_kind='session' WHERE user_id=? AND id=?",
			"unknown lift":            "UPDATE safety_restrictions SET lift_policy='agent_judgement' WHERE user_id=? AND id=?",
			"plan scope without plan": "UPDATE safety_restrictions SET scope_kind='plan', lift_policy='user_reverification' WHERE user_id=? AND id=?",
			"global scope with plan":  "UPDATE safety_restrictions SET plan_id=gen_random_uuid() WHERE user_id=? AND id=?",
			"expiry without until":    "UPDATE safety_restrictions SET lift_policy='on_expiry' WHERE user_id=? AND id=?",
			"until not after from":    "UPDATE safety_restrictions SET valid_until=valid_from WHERE user_id=? AND id=?",
			"urgent plan scope":       "UPDATE safety_restrictions SET risk_level='urgent', scope_kind='plan', plan_id=gen_random_uuid() WHERE user_id=? AND id=?",
			"urgent exercise scope":   "UPDATE safety_restrictions SET risk_level='urgent', scope_kind='exercise' WHERE user_id=? AND id=?",
			"urgent reverification":   "UPDATE safety_restrictions SET risk_level='urgent', lift_policy='user_reverification' WHERE user_id=? AND id=?",
			"urgent auto expiry":      "UPDATE safety_restrictions SET risk_level='urgent', lift_policy='on_expiry', valid_until=valid_from + interval '1 day' WHERE user_id=? AND id=?",
		} {
			if !rejectsInSavepoint(tx, statement, owner, target) {
				t.Errorf("baseline accepted %s", name)
			}
		}
		for name, args := range map[string][]any{
			"blank exercise":            {owner, target, ""},
			"padded exercise":           {owner, target, " squat"},
			"oversized exercise":        {owner, target, strings.Repeat("a", 129)},
			"cross-owner restriction":   {owner, foreignRows[0].ID, "squat"},
			"missing restriction":       {owner, uuid.New(), "squat"},
			"foreign owner on own rows": {foreign, target, "squat"},
		} {
			if !rejectsInSavepoint(tx, "INSERT INTO safety_restriction_exercises(user_id, restriction_id, exercise_id) VALUES (?,?,?)", args...) {
				t.Errorf("baseline accepted exercise scope %s", name)
			}
		}

		// Valid widened shapes persist and read back in one snapshot.
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{"UPDATE safety_restrictions SET risk_level='advisory', scope_kind='exercise', plan_id=?, lift_policy='on_expiry', valid_until=valid_from + interval '7 days' WHERE user_id=? AND id=?", []any{plan, owner, target}},
			{"INSERT INTO safety_restriction_exercises(user_id, restriction_id, exercise_id) VALUES (?,?,'squat'), (?,?,'deadlift')", []any{owner, target, owner, target}},
			{"UPDATE safety_restrictions SET scope_kind='plan', plan_id=?, lift_policy='user_reverification', lift_conditions='[\"confirmed condition\"]' WHERE user_id=? AND id=?", []any{plan, owner, rows[1].ID}},
		} {
			if err := tx.Exec(statement.sql, statement.args...).Error; err != nil {
				t.Fatalf("widened shape rejected: %v", err)
			}
		}
		list, err := NewBodyStateRepository(tx).ListSafetyRestrictions(context.Background(), owner)
		if err != nil || len(list.Restrictions) != 2 {
			t.Fatalf("list=%+v err=%v", list, err)
		}
		byID := map[uuid.UUID]model.SafetyRestrictionRecord{}
		for _, row := range list.Restrictions {
			if row.UserID != owner {
				t.Fatalf("foreign row leaked: %+v", row)
			}
			byID[row.ID] = row
		}
		exercise := byID[target]
		if exercise.RiskLevel != "advisory" || exercise.ScopeKind != "exercise" || exercise.PlanID == nil || *exercise.PlanID != plan || exercise.ValidUntil == nil ||
			len(exercise.ExerciseIDs) != 2 || exercise.ExerciseIDs[0] != "deadlift" || exercise.ExerciseIDs[1] != "squat" || !exercise.Active {
			t.Fatalf("exercise scope not round-tripped: %+v", exercise)
		}
		planScoped := byID[rows[1].ID]
		if planScoped.ScopeKind != "plan" || planScoped.PlanID == nil || *planScoped.PlanID != plan || planScoped.ExerciseIDs == nil || len(planScoped.ExerciseIDs) != 0 {
			t.Fatalf("plan scope not round-tripped: %+v", planScoped)
		}
		// Exercise coverage attached to a non-exercise restriction is unreadable,
		// never silently ignored.
		if err := tx.Exec("INSERT INTO safety_restriction_exercises(user_id, restriction_id, exercise_id) VALUES (?,?,'squat')", owner, rows[1].ID).Error; err != nil {
			t.Fatal(err)
		}
		if list, err := NewBodyStateRepository(tx).ListSafetyRestrictions(context.Background(), owner); err == nil {
			t.Fatalf("incoherent exercise scope read as %+v", list)
		}
	})

	// Account erasure removes exercise scope with its restriction.
	if err := db.Exec("INSERT INTO safety_restriction_exercises(user_id, restriction_id, exercise_id) VALUES (?,?,'squat')", foreign, foreignRows[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM users WHERE id=?", foreign).Error; err != nil {
		t.Fatal(err)
	}
	assertBodyFactCount(t, db, "safety_restriction_exercises", foreign, 0)
}
