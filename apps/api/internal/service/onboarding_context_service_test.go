package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeOnboardingProfileWriter struct {
	profiles []*model.UserProfile
	err      error
}

func (f *fakeOnboardingProfileWriter) CreateOrUpdateProfile(_ context.Context, userID uuid.UUID, profile *model.UserProfile) error {
	if f.err != nil {
		return f.err
	}
	copy := *profile
	copy.UserID = userID
	f.profiles = append(f.profiles, &copy)
	return nil
}

type fakeOnboardingBodyStateWriter struct {
	patches []model.BodyStateCurrentContextPatch
	err     error
}

func (f *fakeOnboardingBodyStateWriter) ApplyCurrentContextPatch(_ context.Context, _ uuid.UUID, _ *int64, patch model.BodyStateCurrentContextPatch, _ string) (*model.BodyStateRevision, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.patches = append(f.patches, patch)
	return &model.BodyStateRevision{Revision: 1}, nil
}

type fakeOnboardingTransactions struct {
	calls int
	err   error
}

func (f *fakeOnboardingTransactions) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return fn(ctx)
}

func validOnboardingContextRequest() dto.OnboardingContextRequest {
	height := 178.5
	weight := 75.0
	return dto.OnboardingContextRequest{
		Profile:     dto.OnboardingProfileInput{Gender: "male", BirthDate: "1998-05-20"},
		BodyMetrics: dto.OnboardingBodyMetricsInput{HeightCm: &height, WeightKg: &weight},
		Lifestyle: dto.OnboardingLifestyleInput{
			Activity:   dto.LifestyleSectionInput{Summary: "工作日久坐为主"},
			Sleep:      dto.LifestyleSectionInput{Summary: "轮班，平均睡 6-7 小时"},
			Exercise:   dto.LifestyleSectionInput{Summary: "力量训练每周 3 次"},
			Nutrition:  dto.LifestyleSectionInput{Summary: "三餐通常规律"},
			Substances: dto.LifestyleSectionInput{Summary: "每天咖啡两杯，不吸烟"},
			Recovery:   dto.LifestyleSectionInput{Summary: "工作日压力偏高"},
		},
		InjuryHistory: "两年前左膝轻微拉伤",
	}
}

func TestOnboardingContextSubmitsStableProfileAndOneBodyStatePatch(t *testing.T) {
	profiles := &fakeOnboardingProfileWriter{}
	bodyState := &fakeOnboardingBodyStateWriter{}
	transactions := &fakeOnboardingTransactions{}
	svc := NewOnboardingContextService(profiles, bodyState, transactions)

	result, err := svc.Submit(context.Background(), uuid.New(), validOnboardingContextRequest())
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if transactions.calls != 1 {
		t.Fatalf("transaction calls=%d, want 1", transactions.calls)
	}
	if len(profiles.profiles) != 1 {
		t.Fatalf("profile writes=%d, want 1", len(profiles.profiles))
	}
	profile := profiles.profiles[0]
	if profile.Gender == nil || *profile.Gender != "male" || profile.BirthDate == nil {
		t.Fatalf("unexpected stable profile: %#v", profile)
	}
	if len(bodyState.patches) != 1 {
		t.Fatalf("BodyState patch calls=%d, want 1", len(bodyState.patches))
	}
	patch := bodyState.patches[0]
	if len(patch.Facts) != 7 {
		t.Fatalf("fact mutations=%d, want six lifestyle + injury", len(patch.Facts))
	}
	if len(patch.Observations) != 2 {
		t.Fatalf("observation mutations=%d, want height + weight", len(patch.Observations))
	}
	if patch.Facts[6].Kind != model.BodyStateFactKindInjuryHistory {
		t.Fatalf("last fact kind=%q, want injury summary", patch.Facts[6].Kind)
	}
	if result.BodyStateRevision == nil || *result.BodyStateRevision != 1 {
		t.Fatalf("result revision=%v, want 1", result.BodyStateRevision)
	}
}

func TestOnboardingContextRejectsInvalidMetricsBeforeTransaction(t *testing.T) {
	profiles := &fakeOnboardingProfileWriter{}
	bodyState := &fakeOnboardingBodyStateWriter{}
	transactions := &fakeOnboardingTransactions{}
	svc := NewOnboardingContextService(profiles, bodyState, transactions)
	request := validOnboardingContextRequest()
	invalid := 400.0
	request.BodyMetrics.WeightKg = &invalid

	_, err := svc.Submit(context.Background(), uuid.New(), request)
	if !errors.Is(err, ErrInvalidOnboardingContext) {
		t.Fatalf("error=%v, want ErrInvalidOnboardingContext", err)
	}
	if transactions.calls != 0 || len(profiles.profiles) != 0 || len(bodyState.patches) != 0 {
		t.Fatalf("invalid request reached persistence: transactions=%d profiles=%d patches=%d", transactions.calls, len(profiles.profiles), len(bodyState.patches))
	}
}
