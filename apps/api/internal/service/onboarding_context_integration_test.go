package service

import (
	"context"
	"os"
	"testing"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openOnboardingContextIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run onboarding context integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	return db
}

func seedOnboardingContextUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	if err := db.Exec(
		"INSERT INTO users(id,email,password_hash) VALUES (?,?,?)",
		userID, "onboarding-context-"+userID.String()+"@example.test", "synthetic-hash",
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id = ?", userID).Error })
	return userID
}

func newOnboardingIntegrationService(db *gorm.DB) *OnboardingContextService {
	profileService := NewProfileService(repository.NewProfileRepository(db))
	bodyStateService := NewBodyStateService(repository.NewBodyStateRepository(db))
	return NewOnboardingContextService(
		profileService,
		bodyStateService,
		database.NewTransactionManager(db),
	)
}

func TestOnboardingContextCommitsProfileAndOneBodyStateRevisionPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	userID := seedOnboardingContextUser(t, db)
	svc := newOnboardingIntegrationService(db)

	result, err := svc.Submit(context.Background(), userID, validOnboardingContextRequest())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.BodyStateRevision == nil || *result.BodyStateRevision != 1 {
		t.Fatalf("body state revision=%v, want exactly R1", result.BodyStateRevision)
	}

	var profile model.UserProfile
	if err := db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.BirthDate == nil || profile.Gender == nil || *profile.Gender != "male" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	var factCount int64
	if err := db.Model(&model.BodyStateFact{}).Where("user_id = ?", userID).Count(&factCount).Error; err != nil {
		t.Fatalf("count BodyState facts: %v", err)
	}
	if factCount != 7 {
		t.Fatalf("BodyState facts=%d, want 6 lifestyle + injury", factCount)
	}
	var observationCount int64
	if err := db.Model(&model.BodyStateObservation{}).Where("user_id = ?", userID).Count(&observationCount).Error; err != nil {
		t.Fatalf("count BodyState observations: %v", err)
	}
	if observationCount != 2 {
		t.Fatalf("BodyState observations=%d, want height + weight", observationCount)
	}
	var revisionCount int64
	if err := db.Model(&model.BodyStateRevision{}).Where("user_id = ?", userID).Count(&revisionCount).Error; err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if revisionCount != 1 {
		t.Fatalf("onboarding revisions=%d, want one semantic revision", revisionCount)
	}
}

func TestOnboardingContextRollsBackProfileWhenBodyStateRevisionConflictsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	userID := seedOnboardingContextUser(t, db)
	bodyState := NewBodyStateService(repository.NewBodyStateRepository(db))
	if _, _, err := bodyState.UpsertFact(ctx, userID, nil, model.BodyStateFact{
		Kind: "seed.context", Value: "seed", Origin: "user_reported", ReviewState: "confirmed",
	}); err != nil {
		t.Fatalf("seed BodyState revision: %v", err)
	}

	svc := newOnboardingIntegrationService(db)
	request := validOnboardingContextRequest()
	stale := int64(0)
	request.ExpectedBodyStateRevision = &stale
	_, err := svc.Submit(ctx, userID, request)
	if err == nil {
		t.Fatal("expected BodyState revision conflict")
	}

	var profileCount int64
	if err := db.Model(&model.UserProfile{}).Where("user_id = ?", userID).Count(&profileCount).Error; err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if profileCount != 0 {
		t.Fatalf("profile write escaped failed coordinated transaction, rows=%d", profileCount)
	}
	var state model.BodyState
	if err := db.First(&state, "user_id = ?", userID).Error; err != nil {
		t.Fatalf("load BodyState: %v", err)
	}
	if state.CurrentRevision != 1 {
		t.Fatalf("failed onboarding changed BodyState revision to %d", state.CurrentRevision)
	}
}
