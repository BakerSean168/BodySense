package service

import (
	"context"
	"errors"
	"testing"

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

func validOnboardingContextRequest() OnboardingContextRequest {
	height := 178.5
	weight := 75.0
	return OnboardingContextRequest{
		Profile:     OnboardingProfileInput{Gender: "male", BirthDate: "1998-05-20"},
		BodyMetrics: OnboardingBodyMetricsInput{HeightCm: &height, WeightKg: &weight},
		Lifestyle: OnboardingLifestyleInput{
			Activity:   LifestyleSectionInput{Summary: "工作日久坐为主"},
			Sleep:      LifestyleSectionInput{Summary: "轮班，平均睡 6-7 小时"},
			Exercise:   LifestyleSectionInput{Summary: "力量训练每周 3 次"},
			Nutrition:  LifestyleSectionInput{Summary: "三餐通常规律"},
			Substances: LifestyleSectionInput{Summary: "每天咖啡两杯，不吸烟"},
			Recovery:   LifestyleSectionInput{Summary: "工作日压力偏高"},
		},
		InjuryHistory: "两年前左膝轻微拉伤",
	}
}

func TestOnboardingContextSubmitsStableProfileAndOneBodyStatePatch(t *testing.T) {
	profiles := &fakeOnboardingProfileWriter{}
	bodyState := &memoryBodyContextStore{}
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
	if len(bodyState.updates) != 1 {
		t.Fatalf("BodyState update calls=%d, want 1", len(bodyState.updates))
	}
	if len(bodyState.ledger.facts) != 7 || len(bodyState.ledger.observations) != 2 {
		t.Fatalf("want six lifestyle + injury facts and height + weight: facts=%d observations=%d",
			len(bodyState.ledger.facts), len(bodyState.ledger.observations))
	}
	if bodyState.ledger.commands != 3 {
		t.Fatalf("onboarding must be one fact batch plus one observation per metric, got %d commands", bodyState.ledger.commands)
	}
	if result.BodyStateRevision == nil || *result.BodyStateRevision != 3 {
		t.Fatalf("result revision=%v, want 3", result.BodyStateRevision)
	}
}

// An onboarding retry after a successful save (for example, when Assessment
// later fails) is a no-op at the BodyState boundary, not revision zero.
func TestOnboardingContextExactReplayReturnsExistingRevision(t *testing.T) {
	profiles := &fakeOnboardingProfileWriter{}
	bodyState := &memoryBodyContextStore{}
	svc := NewOnboardingContextService(profiles, bodyState, &fakeOnboardingTransactions{})
	owner := uuid.New()
	request := validOnboardingContextRequest()
	initial := int64(0)
	request.ExpectedBodyStateRevision = &initial

	first, err := svc.Submit(context.Background(), owner, request)
	if err != nil || first == nil || first.BodyStateRevision == nil || *first.BodyStateRevision != 3 {
		t.Fatalf("first onboarding result=%+v, err=%v, want R3", first, err)
	}
	second, err := svc.Submit(context.Background(), owner, request)
	if err != nil || second == nil || second.BodyStateRevision == nil || *second.BodyStateRevision != 3 {
		t.Fatalf("replay result=%+v, err=%v, want existing R3", second, err)
	}
	if bodyState.ledger.revision != 3 || bodyState.ledger.commands != 3 {
		t.Fatalf("replay created ledger writes: revision=%d commands=%d", bodyState.ledger.revision, bodyState.ledger.commands)
	}
}

func TestOnboardingContextRejectsInvalidMetricsBeforeTransaction(t *testing.T) {
	profiles := &fakeOnboardingProfileWriter{}
	bodyState := &memoryBodyContextStore{}
	transactions := &fakeOnboardingTransactions{}
	svc := NewOnboardingContextService(profiles, bodyState, transactions)
	request := validOnboardingContextRequest()
	invalid := 400.0
	request.BodyMetrics.WeightKg = &invalid

	_, err := svc.Submit(context.Background(), uuid.New(), request)
	if !errors.Is(err, ErrInvalidOnboardingContext) {
		t.Fatalf("error=%v, want ErrInvalidOnboardingContext", err)
	}
	if transactions.calls != 0 || len(profiles.profiles) != 0 || len(bodyState.updates) != 0 {
		t.Fatalf("invalid request reached persistence: transactions=%d profiles=%d updates=%d", transactions.calls, len(profiles.profiles), len(bodyState.updates))
	}
}
