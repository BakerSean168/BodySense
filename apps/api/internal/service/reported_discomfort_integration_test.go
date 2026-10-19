package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/repository"
)

func newDiscomfortIntegrationService(t *testing.T) (*OnboardingContextService, *ReportedDiscomfortService, *repository.ReportedDiscomfortRepository) {
	t.Helper()
	db := openOnboardingContextIntegrationDB(t)
	repo := repository.NewReportedDiscomfortRepository(db)
	svc := NewOnboardingContextService(
		NewProfileService(repository.NewProfileRepository(db)),
		NewBodyStateService(repository.NewBodyStateRepository(db)),
		database.NewTransactionManager(db),
	).WithReportedDiscomfort(repo)
	return svc, NewReportedDiscomfortService(repo), repo
}

func TestReportedDiscomfortOnboardingRoundTripPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	userID := seedOnboardingContextUser(t, db)
	svc, reader, repo := newDiscomfortIntegrationService(t)
	ctx := context.Background()

	before, err := reader.Get(ctx, userID)
	if err != nil || before.Report != nil {
		t.Fatalf("before onboarding: %#v err=%v", before, err)
	}
	request := validOnboardingContextRequest()
	request.DiscomfortAreas = []string{"hip", "neck", "arm_wrist"}
	if _, err := svc.Submit(ctx, userID, request); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	got, err := reader.Get(ctx, userID)
	if err != nil || got.Report == nil {
		t.Fatalf("read back: %#v err=%v", got, err)
	}
	if want := []string{"neck", "hip", "arm_wrist"}; !reflect.DeepEqual(got.Report.AreaCodes, want) || got.Report.Source != "onboarding" {
		t.Fatalf("report=%#v want %v", got.Report, want)
	}
	// The answer is not a symptom: no symptom fact, no safety screen.
	var symptoms, screens int64
	db.Table("body_facts").Where("user_id = ? AND kind = 'symptom'", userID).Count(&symptoms)
	db.Table("safety_screens").Where("user_id = ?", userID).Count(&screens)
	if symptoms != 0 || screens != 0 {
		t.Fatalf("self-report created symptom facts=%d safety screens=%d", symptoms, screens)
	}

	// A later answer replaces the row (one per user).
	if err := repo.Replace(ctx, userID, []string{"none"}, time.Now()); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ = reader.Get(ctx, userID)
	var rows int64
	db.Table("reported_discomfort_areas").Where("user_id = ?", userID).Count(&rows)
	if rows != 1 || !reflect.DeepEqual(got.Report.AreaCodes, []string{"none"}) {
		t.Fatalf("replace: rows=%d report=%#v", rows, got.Report)
	}

	// The schema refuses a "none" mixed with an area and unknown codes even
	// when the application check is bypassed.
	for _, raw := range []string{`["none","neck"]`, `["knee"]`, `[]`, `"neck"`} {
		if err := db.Exec("UPDATE reported_discomfort_areas SET area_codes = ?::jsonb WHERE user_id = ?", raw, userID).Error; err == nil {
			t.Fatalf("schema accepted area_codes %s", raw)
		}
	}

	// Account erasure cascades.
	if err := db.Exec("DELETE FROM users WHERE id = ?", userID).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}
	db.Table("reported_discomfort_areas").Where("user_id = ?", userID).Count(&rows)
	if rows != 0 {
		t.Fatalf("self-report survived account deletion")
	}
}

func TestReportedDiscomfortRollsBackWithOnboardingPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	userID := seedOnboardingContextUser(t, db)
	svc, reader, _ := newDiscomfortIntegrationService(t)
	ctx := context.Background()
	request := validOnboardingContextRequest()
	request.DiscomfortAreas = []string{"leg"}
	stale := int64(7)
	request.ExpectedBodyStateRevision = &stale
	if _, err := svc.Submit(ctx, userID, request); err == nil {
		t.Fatal("expected BodyState revision conflict")
	}
	got, err := reader.Get(ctx, userID)
	if err != nil || got.Report != nil {
		t.Fatalf("self-report escaped the failed onboarding transaction: %#v err=%v", got, err)
	}
}
