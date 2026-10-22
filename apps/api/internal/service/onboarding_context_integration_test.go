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

func TestOnboardingContextCommitsProfileAndV2LedgerPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	userID := seedOnboardingContextUser(t, db)
	svc := newOnboardingIntegrationService(db)

	result, err := svc.Submit(context.Background(), userID, validOnboardingContextRequest())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// One fact batch (six lifestyle + injury) plus one observation per metric.
	if result.BodyStateRevision == nil || *result.BodyStateRevision != 3 {
		t.Fatalf("body state revision=%v, want R3", result.BodyStateRevision)
	}

	var profile model.UserProfile
	if err := db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.BirthDate == nil || profile.Gender == nil || *profile.Gender != "male" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	count := func(table string) int64 {
		var n int64
		if err := db.Table(table).Where("user_id = ?", userID).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	if got := count("body_facts"); got != 7 {
		t.Fatalf("body_facts=%d, want 6 lifestyle + injury", got)
	}
	if got := count("body_observations"); got != 2 {
		t.Fatalf("body_observations=%d, want height + weight", got)
	}
	current, err := NewBodyStateService(repository.NewBodyStateRepository(db)).CurrentBodyContext(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentBodyContext: %v", err)
	}
	if current.Height == nil || current.Height.RawValue != "178.5" || current.Injury == nil || current.Injury.Statement != "两年前左膝轻微拉伤" || len(current.Lifestyle) != 6 {
		t.Fatalf("unexpected ledger projection: %#v", current)
	}
}

func TestOnboardingContextExactReplayLeavesDatabaseVersionsUnchangedPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := newOnboardingIntegrationService(db)
	request := validOnboardingContextRequest()
	initial := int64(0)
	request.ExpectedBodyStateRevision = &initial

	for attempt := 1; attempt <= 2; attempt++ {
		result, err := svc.Submit(context.Background(), owner, request)
		if err != nil || result == nil || result.BodyStateRevision == nil || *result.BodyStateRevision != 3 {
			t.Fatalf("attempt %d: result=%+v error=%v, want R3", attempt, result, err)
		}
	}
	var profile model.UserProfile
	if err := db.Where("user_id = ?", owner).Take(&profile).Error; err != nil {
		t.Fatal(err)
	}
	if profile.Revision != 1 {
		t.Fatalf("exact onboarding replay changed profile revision to %d", profile.Revision)
	}
	var revisionCount int64
	if err := db.Table("body_state_revisions").Where("user_id = ?", owner).Count(&revisionCount).Error; err != nil {
		t.Fatal(err)
	}
	if revisionCount != 3 {
		t.Fatalf("exact onboarding replay created %d BodyState revisions, want 3", revisionCount)
	}
}

func TestOnboardingContextRollsBackProfileWhenBodyStateRevisionConflictsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	userID := seedOnboardingContextUser(t, db)
	bodyState := repository.NewBodyStateRepository(db)
	if _, err := bodyState.RecordDirectFacts(ctx, userID, model.RecordBodyFactsCommand{IdempotencyKey: "seed", Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "seed", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: []byte(`{}`)}}}); err != nil {
		t.Fatal(err)
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
