package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openBodyStateContextIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run BodyState context integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	return db
}

func seedBodyStateContextUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	if err := db.Exec(
		"INSERT INTO users(id,email,password_hash) VALUES (?,?,?)",
		userID, "body-state-context-"+userID.String()+"@example.test", "synthetic-hash",
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id = ?", userID).Error })
	return userID
}

func TestBodyStateCurrentContextPatchIsOneRevisionAndPreservesTemporalHistoryPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	ctx := context.Background()
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	t0 := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	revision, err := repo.ApplyCurrentContextPatch(ctx, userID, nil, model.BodyStateCurrentContextPatch{
		Facts: []model.BodyStateCurrentFactMutation{
			{
				Kind:        model.BodyStateFactKindLifestyleActivity,
				EffectiveAt: t0,
				Replacement: &model.BodyStateFact{
					Value: "久坐为主", Details: datatypes.JSON(`{"sitting_hours":8}`),
					Origin: "user_reported", ReviewState: "confirmed",
				},
			},
			{
				Kind:        model.BodyStateFactKindLifestyleSleep,
				EffectiveAt: t0,
				Replacement: &model.BodyStateFact{
					Value: "作息规律", Details: datatypes.JSON(`{"regularity":"regular"}`),
					Origin: "user_reported", ReviewState: "confirmed",
				},
			},
		},
	}, "integration")
	if err != nil {
		t.Fatalf("initial context patch: %v", err)
	}
	if revision == nil || revision.Revision != 1 {
		t.Fatalf("initial revision=%v, want R1", revision)
	}

	t1 := time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC)
	expected := int64(1)
	revision, err = repo.ApplyCurrentContextPatch(ctx, userID, &expected, model.BodyStateCurrentContextPatch{
		Facts: []model.BodyStateCurrentFactMutation{
			{
				Kind:        model.BodyStateFactKindLifestyleActivity,
				EffectiveAt: t1,
				Replacement: &model.BodyStateFact{
					Value: "现在走动和站立更多", Details: datatypes.JSON(`{"sitting_hours":4}`),
					Origin: "user_reported", ReviewState: "confirmed",
				},
			},
			{
				Kind:        model.BodyStateFactKindLifestyleSleep,
				EffectiveAt: t1,
				Replacement: &model.BodyStateFact{
					Value: "作息规律", Details: datatypes.JSON(`{"regularity":"regular"}`),
					Origin: "user_reported", ReviewState: "confirmed",
				},
			},
		},
	}, "integration")
	if err != nil {
		t.Fatalf("changed context patch: %v", err)
	}
	if revision == nil || revision.Revision != 2 {
		t.Fatalf("changed revision=%v, want exactly R2", revision)
	}

	var activity []model.BodyStateFact
	if err := db.Where("user_id = ? AND kind = ?", userID, model.BodyStateFactKindLifestyleActivity).
		Order("created_revision ASC").Find(&activity).Error; err != nil {
		t.Fatalf("load activity history: %v", err)
	}
	if len(activity) != 2 {
		t.Fatalf("activity history rows=%d, want 2", len(activity))
	}
	if activity[0].LifecycleState != "inactive" || activity[0].ValidUntil == nil || !activity[0].ValidUntil.Equal(t1) {
		t.Fatalf("old activity was not closed at transition time: %#v", activity[0])
	}
	if activity[1].SupersedesFactID == nil || *activity[1].SupersedesFactID != activity[0].ID {
		t.Fatalf("new activity does not supersede old row: %#v", activity[1])
	}
	var sleepCount int64
	if err := db.Model(&model.BodyStateFact{}).
		Where("user_id = ? AND kind = ?", userID, model.BodyStateFactKindLifestyleSleep).
		Count(&sleepCount).Error; err != nil {
		t.Fatalf("count sleep rows: %v", err)
	}
	if sleepCount != 1 {
		t.Fatalf("unchanged sleep should not create history noise, rows=%d", sleepCount)
	}

	heightValue := datatypes.JSON(`{"value":178.5,"unit":"cm"}`)
	weightValue := datatypes.JSON(`{"value":75,"unit":"kg"}`)
	expected = 2
	revision, err = repo.ApplyCurrentContextPatch(ctx, userID, &expected, model.BodyStateCurrentContextPatch{
		Observations: []model.BodyStateCurrentObservationMutation{
			{Kind: model.BodyStateObservationKindHeight, Replacement: &model.BodyStateObservation{Value: heightValue, ReviewState: "confirmed"}},
			{Kind: model.BodyStateObservationKindWeight, Replacement: &model.BodyStateObservation{Value: weightValue, ReviewState: "confirmed"}},
		},
	}, "integration")
	if err != nil {
		t.Fatalf("measurement patch: %v", err)
	}
	if revision == nil || revision.Revision != 3 {
		t.Fatalf("height + weight must share one revision, got %v", revision)
	}

	expected = 3
	newWeight := datatypes.JSON(`{"value":73,"unit":"kg"}`)
	revision, err = repo.ApplyCurrentContextPatch(ctx, userID, &expected, model.BodyStateCurrentContextPatch{
		Observations: []model.BodyStateCurrentObservationMutation{{
			Kind:        model.BodyStateObservationKindWeight,
			Replacement: &model.BodyStateObservation{Value: newWeight, ReviewState: "confirmed"},
		}},
	}, "integration")
	if err != nil {
		t.Fatalf("weight transition: %v", err)
	}
	if revision == nil || revision.Revision != 4 {
		t.Fatalf("weight transition revision=%v, want R4", revision)
	}
	var weights []model.BodyStateObservation
	if err := db.Where("user_id = ? AND kind = ?", userID, model.BodyStateObservationKindWeight).
		Order("created_revision ASC").Find(&weights).Error; err != nil {
		t.Fatalf("load weight history: %v", err)
	}
	if len(weights) != 2 || weights[0].LifecycleState != "inactive" {
		t.Fatalf("weight history=%#v", weights)
	}
	if weights[1].SupersedesObservationID == nil || *weights[1].SupersedesObservationID != weights[0].ID {
		t.Fatalf("weight supersedes chain missing: %#v", weights[1])
	}
}

func TestBodyStateAcceptLifestyleCandidatePromotesItAndClosesPreviousCurrentPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	ctx := context.Background()
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)
	t0 := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	revision, err := repo.ApplyCurrentContextPatch(ctx, userID, nil, model.BodyStateCurrentContextPatch{
		Facts: []model.BodyStateCurrentFactMutation{{
			Kind:        model.BodyStateFactKindLifestyleSleep,
			EffectiveAt: t0,
			Replacement: &model.BodyStateFact{
				Value: "作息规律，通常睡 7-8 小时", Details: datatypes.JSON(`{"regularity":"regular"}`),
				Origin: "user_edited", ReviewState: "confirmed",
			},
		}},
	}, "integration")
	if err != nil || revision == nil || revision.Revision != 1 {
		t.Fatalf("seed confirmed lifestyle: revision=%v err=%v", revision, err)
	}

	candidate, revision, err := repo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
		Kind:                  model.BodyStateFactKindLifestyleSleep,
		Value:                 "最近换夜班，通常凌晨 5 点睡",
		Details:               datatypes.JSON(`{"shift_work":true}`),
		Origin:                "ai_extracted",
		ReviewState:           "unverified",
		LifecycleState:        "active",
		ExcludedFromReasoning: true,
		SourceKey:             "consultation:test:lifestyle:sleep",
	}, "consultation")
	if err != nil || candidate == nil || revision == nil || revision.Revision != 2 {
		t.Fatalf("persist candidate: candidate=%v revision=%v err=%v", candidate, revision, err)
	}

	pending, err := repo.ListReviewableFacts(ctx, userID, 50)
	if err != nil || len(pending) != 1 || pending[0].ID != candidate.ID {
		t.Fatalf("pending candidates=%#v err=%v", pending, err)
	}
	currentBefore, err := repo.GetCurrent(ctx, userID)
	if err != nil {
		t.Fatalf("get current before accept: %v", err)
	}
	if len(currentBefore.Facts) != 1 || currentBefore.Facts[0].Value != "作息规律，通常睡 7-8 小时" {
		t.Fatalf("unverified candidate leaked into current reasoning: %#v", currentBefore.Facts)
	}

	effectiveAt := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	expected := int64(2)
	accepted, revision, err := repo.AcceptCurrentFactCandidate(
		ctx, userID, &expected, candidate.ID, effectiveAt, "user_review",
	)
	if err != nil {
		t.Fatalf("accept candidate: %v", err)
	}
	if revision == nil || revision.Revision != 3 {
		t.Fatalf("accept revision=%v, want R3", revision)
	}
	if accepted == nil || accepted.ReviewState != "confirmed" || accepted.ExcludedFromReasoning {
		t.Fatalf("candidate was not promoted to confirmed reasoning fact: %#v", accepted)
	}
	if accepted.ValidFrom == nil || !accepted.ValidFrom.Equal(effectiveAt) {
		t.Fatalf("candidate valid_from=%v, want %v", accepted.ValidFrom, effectiveAt)
	}

	var rows []model.BodyStateFact
	if err := db.Where("user_id = ? AND kind = ?", userID, model.BodyStateFactKindLifestyleSleep).
		Order("created_revision ASC").Find(&rows).Error; err != nil {
		t.Fatalf("load sleep history: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("sleep history rows=%d, want 2", len(rows))
	}
	if rows[0].LifecycleState != "inactive" || rows[0].ValidUntil == nil || !rows[0].ValidUntil.Equal(effectiveAt) {
		t.Fatalf("previous current was not closed correctly: %#v", rows[0])
	}
	if rows[1].ID != candidate.ID || rows[1].SupersedesFactID == nil || *rows[1].SupersedesFactID != rows[0].ID {
		t.Fatalf("accepted candidate does not supersede previous current: %#v", rows[1])
	}

	currentAfter, err := repo.GetCurrent(ctx, userID)
	if err != nil {
		t.Fatalf("get current after accept: %v", err)
	}
	if len(currentAfter.Facts) != 1 || currentAfter.Facts[0].ID != candidate.ID {
		t.Fatalf("accepted candidate is not the sole current fact: %#v", currentAfter.Facts)
	}
}

func TestBodyStateRejectFactExcludesCandidateFromReasoningPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	ctx := context.Background()
	userID := seedBodyStateContextUser(t, db)
	repo := NewBodyStateRepository(db)

	candidate, revision, err := repo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
		Kind:                  model.BodyStateFactKindLifestyleSubstances,
		Value:                 "AI 误提取：每天饮酒",
		Origin:                "ai_extracted",
		ReviewState:           "unverified",
		LifecycleState:        "active",
		ExcludedFromReasoning: true,
		SourceKey:             "consultation:test:lifestyle:substances",
	}, "consultation")
	if err != nil || candidate == nil || revision == nil {
		t.Fatalf("persist candidate: candidate=%v revision=%v err=%v", candidate, revision, err)
	}
	expected := revision.Revision
	rejected, reviewRevision, err := repo.UpdateFactReviewState(
		ctx, userID, &expected, candidate.ID, "rejected", "user_review",
	)
	if err != nil {
		t.Fatalf("reject candidate: %v", err)
	}
	if reviewRevision == nil || rejected == nil || rejected.ReviewState != "rejected" || !rejected.ExcludedFromReasoning {
		t.Fatalf("rejected fact remains eligible for reasoning: fact=%#v revision=%v", rejected, reviewRevision)
	}
	pending, err := repo.ListReviewableFacts(ctx, userID, 50)
	if err != nil || len(pending) != 0 {
		t.Fatalf("rejected fact remained reviewable: pending=%#v err=%v", pending, err)
	}
}
