package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

var ErrInvalidOnboardingContext = errors.New("invalid onboarding context")

type onboardingProfileWriter interface {
	CreateOrUpdateProfile(ctx context.Context, userID uuid.UUID, profile *model.UserProfile) error
}

type onboardingBodyStateWriter interface {
	UpdateBodyContext(ctx context.Context, userID uuid.UUID, expectedRevision *int64, update BodyContextUpdate) (int64, error)
}

type onboardingTransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

// OnboardingContextService is the single application command behind first-use
// capture. It coordinates stable Profile identity and one semantically coherent
// BodyState baseline mutation in the same database transaction.
type OnboardingContextService struct {
	profiles     onboardingProfileWriter
	bodyState    onboardingBodyStateWriter
	transactions onboardingTransactionManager
}

func NewOnboardingContextService(
	profiles onboardingProfileWriter,
	bodyState onboardingBodyStateWriter,
	transactions onboardingTransactionManager,
) *OnboardingContextService {
	return &OnboardingContextService{
		profiles: profiles, bodyState: bodyState, transactions: transactions,
	}
}

func (s *OnboardingContextService) Submit(
	ctx context.Context,
	userID uuid.UUID,
	request OnboardingContextRequest,
) (*OnboardingContextResult, error) {
	if s.transactions == nil {
		return nil, errors.New("onboarding transaction manager is required")
	}
	gender := strings.TrimSpace(request.Profile.Gender)
	if gender == "" {
		return nil, fmt.Errorf("%w: gender is required", ErrInvalidOnboardingContext)
	}
	birthDate, err := model.ParseDateOnly(strings.TrimSpace(request.Profile.BirthDate))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid birth_date", ErrInvalidOnboardingContext)
	}
	if err := validateBirthDate(birthDate.Time(), time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOnboardingContext, err)
	}
	if request.BodyMetrics.HeightCm == nil || request.BodyMetrics.WeightKg == nil {
		return nil, fmt.Errorf("%w: height_cm and weight_kg are required", ErrInvalidOnboardingContext)
	}
	if err := validateBodyMetricValues(request.BodyMetrics.HeightCm, request.BodyMetrics.WeightKg); err != nil {
		return nil, fmt.Errorf("%w: body metrics are outside the supported range", ErrInvalidOnboardingContext)
	}

	update := buildOnboardingBodyContext(request)
	profile := &model.UserProfile{
		Gender:    &gender,
		BirthDate: &birthDate,
	}
	var committed int64
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.profiles.CreateOrUpdateProfile(txCtx, userID, profile); err != nil {
			return fmt.Errorf("persist stable onboarding profile: %w", err)
		}
		var err error
		committed, err = s.bodyState.UpdateBodyContext(
			txCtx,
			userID,
			request.ExpectedBodyStateRevision,
			update,
		)
		if err != nil {
			return fmt.Errorf("persist onboarding BodyState context: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := &OnboardingContextResult{}
	if committed > 0 {
		revision := committed
		result.BodyStateRevision = &revision
	}
	return result, nil
}

func buildOnboardingBodyContext(request OnboardingContextRequest) BodyContextUpdate {
	injury := request.InjuryHistory
	return BodyContextUpdate{
		HeightCm: request.BodyMetrics.HeightCm,
		WeightKg: request.BodyMetrics.WeightKg,
		Injury:   &injury,
		Lifestyle: map[string]*LifestyleSectionInput{
			"activity":   {Summary: request.Lifestyle.Activity.Summary, Details: request.Lifestyle.Activity.Details},
			"sleep":      {Summary: request.Lifestyle.Sleep.Summary, Details: request.Lifestyle.Sleep.Details},
			"exercise":   {Summary: request.Lifestyle.Exercise.Summary, Details: request.Lifestyle.Exercise.Details},
			"nutrition":  {Summary: request.Lifestyle.Nutrition.Summary, Details: request.Lifestyle.Nutrition.Details},
			"substances": {Summary: request.Lifestyle.Substances.Summary, Details: request.Lifestyle.Substances.Details},
			"recovery":   {Summary: request.Lifestyle.Recovery.Summary, Details: request.Lifestyle.Recovery.Details},
		},
		Context: "onboarding",
	}
}
