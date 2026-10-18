package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func profileText(value string) *string { return &value }

func seedProfileAsset(t *testing.T, db *gorm.DB, owner uuid.UUID, kind string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec(`INSERT INTO user_assets(id,user_id,kind,original_name,mime_type,byte_size,content_sha256,storage_backend,storage_key)
		VALUES (?,?,?,?,?,?,?,?,?)`, id, owner, kind, "avatar.jpg", "image/jpeg", 10,
		strings.Repeat("a", 64), "local", "profile-test/"+id.String()).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

func profileFields(display string) model.ProfileFields {
	birth, _ := model.ParseDateOnly("1998-02-01")
	return model.ProfileFields{
		DisplayName: profileText(display), Gender: profileText("male"), BirthDate: &birth,
		Locale: profileText("zh-CN"), TimeZone: profileText("Asia/Shanghai"), UnitPreference: profileText("metric"),
	}
}

func TestProfileReplaceRevisionReplayAndClearPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewProfileService(repository.NewProfileRepository(db))
	ctx := context.Background()

	created, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ExpectedRevision: 0, ProfileFields: profileFields(" BakerSean ")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Revision != 1 || *created.DisplayName != "BakerSean" || created.AgeYears == nil ||
		created.Preferences.TimeZone == nil || *created.Preferences.TimeZone != "Asia/Shanghai" {
		t.Fatalf("created=%+v", created)
	}
	// Exact replay of the update that produced revision 1 is idempotent.
	replayed, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ExpectedRevision: 0, ProfileFields: profileFields("BakerSean")})
	if err != nil || replayed.Revision != 1 {
		t.Fatalf("replay revision=%v err=%v", replayed, err)
	}
	// A different body against the stale revision is a conflict.
	if _, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ExpectedRevision: 0, ProfileFields: profileFields("Other")}); !errors.Is(err, model.ErrProfileRevisionConflict) {
		t.Fatalf("stale write err=%v", err)
	}
	// Clearing every optional field is a full replacement with nulls.
	cleared, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ExpectedRevision: 1})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared.Revision != 2 || cleared.DisplayName != nil || cleared.Gender != nil || cleared.BirthDate != nil ||
		cleared.AgeYears != nil || cleared.Preferences.TimeZone != nil || cleared.Preferences.UnitPreference != nil {
		t.Fatalf("cleared=%+v", cleared)
	}
}

func TestProfileRejectsInvalidPreferencesAndDatabaseGuardsPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewProfileService(repository.NewProfileRepository(db))
	for name, mutate := range map[string]func(*model.ProfileFields){
		"server-local zone": func(f *model.ProfileFields) { f.TimeZone = profileText("Local") },
		"unknown zone":      func(f *model.ProfileFields) { f.TimeZone = profileText("Mars/Olympus") },
		"bad locale":        func(f *model.ProfileFields) { f.Locale = profileText("Chinese") },
		"long name":         func(f *model.ProfileFields) { f.DisplayName = profileText(strings.Repeat("名", 41)) },
		"unit":              func(f *model.ProfileFields) { f.UnitPreference = profileText("stone") },
	} {
		fields := profileFields("BakerSean")
		mutate(&fields)
		if _, err := svc.ReplaceProfile(context.Background(), owner, model.ReplaceProfileCommand{ProfileFields: fields}); !errors.Is(err, model.ErrInvalidProfile) {
			t.Fatalf("%s: err=%v want invalid", name, err)
		}
	}
	// The baseline refuses shapes the service would never write.
	if err := db.Exec("INSERT INTO user_profiles(user_id, display_name, time_zone) VALUES (?, ' padded ', 'Local')", owner).Error; err == nil {
		t.Fatal("database accepted an untrimmed display name and server-local zone")
	}
}

func TestProfileOnboardingPreservesDisplayFieldsAndAdvancesRevisionPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	svc := NewProfileService(repository.NewProfileRepository(db))
	ctx := context.Background()
	if _, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ProfileFields: profileFields("BakerSean")}); err != nil {
		t.Fatalf("create: %v", err)
	}
	birth, _ := model.ParseDateOnly("1990-05-06")
	if err := svc.CreateOrUpdateProfile(ctx, owner, &model.UserProfile{Gender: profileText("female"), BirthDate: &birth}); err != nil {
		t.Fatalf("onboarding identity: %v", err)
	}
	stored, err := svc.GetProfile(ctx, owner)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Revision != 2 || *stored.DisplayName != "BakerSean" || *stored.Gender != "female" ||
		*stored.Preferences.UnitPreference != "metric" {
		t.Fatalf("stored=%+v", stored)
	}
	// An editor still holding revision 1 must not silently overwrite it.
	if _, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ExpectedRevision: 1, ProfileFields: profileFields("BakerSean")}); !errors.Is(err, model.ErrProfileRevisionConflict) {
		t.Fatalf("stale editor err=%v", err)
	}
}

func TestProfileAvatarIsOwnerScopedAvailablePhotoPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	stranger := seedOnboardingContextUser(t, db)
	svc := NewProfileService(repository.NewProfileRepository(db))
	ctx := context.Background()

	foreignPhoto := seedProfileAsset(t, db, stranger, "photo")
	ownReport := seedProfileAsset(t, db, owner, "report")
	ownPhoto := seedProfileAsset(t, db, owner, "photo")
	for name, asset := range map[string]uuid.UUID{"foreign": foreignPhoto, "report": ownReport, "missing": uuid.New()} {
		fields := profileFields("BakerSean")
		fields.AvatarAssetID = &asset
		if _, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ProfileFields: fields}); !errors.Is(err, model.ErrProfileAvatarUnavailable) {
			t.Fatalf("%s avatar err=%v", name, err)
		}
	}
	fields := profileFields("BakerSean")
	fields.AvatarAssetID = &ownPhoto
	created, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ProfileFields: fields})
	if err != nil || created.AvatarAssetID == nil || *created.AvatarAssetID != ownPhoto {
		t.Fatalf("own avatar=%+v err=%v", created, err)
	}
	// Source removal hides the avatar on read and blocks re-attaching it.
	if err := db.Exec("UPDATE user_assets SET availability='source_removed', source_removed_at=now() WHERE id=?", ownPhoto).Error; err != nil {
		t.Fatalf("remove source: %v", err)
	}
	stored, err := svc.GetProfile(ctx, owner)
	if err != nil || stored.AvatarAssetID != nil || *stored.DisplayName != "BakerSean" {
		t.Fatalf("after source removal=%+v err=%v", stored, err)
	}
	fields.DisplayName = profileText("Renamed")
	if _, err := svc.ReplaceProfile(ctx, owner, model.ReplaceProfileCommand{ExpectedRevision: 1, ProfileFields: fields}); !errors.Is(err, model.ErrProfileAvatarUnavailable) {
		t.Fatalf("re-attach removed avatar err=%v", err)
	}
}

// PC3-11: changing time zone and unit preference never rewrites the stored
// BodyState observation values, units or times.
func TestProfilePreferenceChangeLeavesObservationsUntouchedPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	owner := seedOnboardingContextUser(t, db)
	onboarding := newOnboardingIntegrationService(db)
	if _, err := onboarding.Submit(context.Background(), owner, validOnboardingContextRequest()); err != nil {
		t.Fatalf("onboarding: %v", err)
	}
	fingerprint := func() string {
		var digest string
		if err := db.Raw("SELECT coalesce(md5(string_agg(o::text, '|' ORDER BY o.id)), '') FROM body_observations o WHERE o.user_id = ?", owner).Scan(&digest).Error; err != nil {
			t.Fatalf("fingerprint: %v", err)
		}
		return digest
	}
	before := fingerprint()
	if before == "" {
		t.Fatal("onboarding produced no observations")
	}
	svc := NewProfileService(repository.NewProfileRepository(db))
	current, err := svc.GetProfile(context.Background(), owner)
	if err != nil || current == nil {
		t.Fatalf("profile=%v err=%v", current, err)
	}
	fields := profileFields("BakerSean")
	fields.TimeZone = profileText("America/New_York")
	fields.UnitPreference = profileText("imperial")
	if _, err := svc.ReplaceProfile(context.Background(), owner, model.ReplaceProfileCommand{ExpectedRevision: current.Revision, ProfileFields: fields}); err != nil {
		t.Fatalf("change preferences: %v", err)
	}
	if after := fingerprint(); after != before {
		t.Fatal("preference change rewrote BodyState observations")
	}
}

func TestProfileAgentContextCarriesOnlySelfReportedIdentity(t *testing.T) {
	birth, _ := model.ParseDateOnly("1998-02-01")
	age := 28
	avatar := uuid.New()
	profile := &model.UserProfile{
		DisplayName: profileText("BakerSean"), AvatarAssetID: &avatar, Gender: profileText("male"),
		BirthDate: &birth, AgeYears: &age, TimeZone: profileText("Asia/Tokyo"),
	}
	encoded, err := json.Marshal(profile.AgentContext())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != `{"gender":"male","birth_date":"1998-02-01","age_years":28}` {
		t.Fatalf("agent context=%s", encoded)
	}
	var nilProfile *model.UserProfile
	if empty := nilProfile.AgentContext(); empty.Gender != nil {
		t.Fatalf("nil profile context=%+v", empty)
	}
}
