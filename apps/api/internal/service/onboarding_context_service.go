package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrInvalidOnboardingContext = errors.New("invalid onboarding context")

type onboardingProfileWriter interface {
	CreateOrUpdateProfile(ctx context.Context, userID uuid.UUID, profile *model.UserProfile) error
}

type onboardingBodyStateWriter interface {
	ApplyCurrentContextPatch(ctx context.Context, userID uuid.UUID, expectedRevision *int64, patch model.BodyStateCurrentContextPatch, source string) (*model.BodyStateRevision, error)
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
	request dto.OnboardingContextRequest,
) (*dto.OnboardingContextResult, error) {
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

	now := time.Now().UTC()
	patch := buildOnboardingBodyStatePatch(request, now)
	profile := &model.UserProfile{
		Gender:    &gender,
		BirthDate: &birthDate,
	}
	var committed *model.BodyStateRevision
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.profiles.CreateOrUpdateProfile(txCtx, userID, profile); err != nil {
			return fmt.Errorf("persist stable onboarding profile: %w", err)
		}
		var err error
		committed, err = s.bodyState.ApplyCurrentContextPatch(
			txCtx,
			userID,
			request.ExpectedBodyStateRevision,
			patch,
			"onboarding",
		)
		if err != nil {
			return fmt.Errorf("persist onboarding BodyState context: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := &dto.OnboardingContextResult{}
	if committed != nil {
		revision := committed.Revision
		result.BodyStateRevision = &revision
	}
	return result, nil
}

func buildOnboardingBodyStatePatch(
	request dto.OnboardingContextRequest,
	effectiveAt time.Time,
) model.BodyStateCurrentContextPatch {
	lifestyle := dto.UpdateLifestyleRequest{
		Activity:   &request.Lifestyle.Activity,
		Sleep:      &request.Lifestyle.Sleep,
		Exercise:   &request.Lifestyle.Exercise,
		Nutrition:  &request.Lifestyle.Nutrition,
		Substances: &request.Lifestyle.Substances,
		Recovery:   &request.Lifestyle.Recovery,
	}
	patch := buildLifestyleContextPatch(lifestyle, effectiveAt, "user_reported", "onboarding")
	patch.Observations = append(
		patch.Observations,
		metricMutationAt(
			model.BodyStateObservationKindHeight,
			*request.BodyMetrics.HeightCm,
			"cm",
			effectiveAt,
			"onboarding",
		),
		metricMutationAt(
			model.BodyStateObservationKindWeight,
			*request.BodyMetrics.WeightKg,
			"kg",
			effectiveAt,
			"onboarding",
		),
	)
	patch.Facts = append(patch.Facts, injuryHistoryMutation(
		request.InjuryHistory,
		effectiveAt,
		"user_reported",
		"onboarding",
	))
	return patch
}

func injuryHistoryMutation(
	summary string,
	effectiveAt time.Time,
	origin string,
	sourceType string,
) model.BodyStateCurrentFactMutation {
	summary = strings.TrimSpace(summary)
	var replacement *model.BodyStateFact
	if summary != "" {
		provenance, _ := json.Marshal(map[string]any{"source_type": sourceType})
		replacement = &model.BodyStateFact{
			ConcernKey:     "history:injury",
			Kind:           model.BodyStateFactKindInjuryHistory,
			Value:          summary,
			Details:        datatypes.JSON(`{}`),
			Origin:         origin,
			ReviewState:    "confirmed",
			LifecycleState: "active",
			Trend:          "unknown",
			Provenance:     datatypes.JSON(provenance),
		}
	}
	return model.BodyStateCurrentFactMutation{
		Kind: model.BodyStateFactKindInjuryHistory, Replacement: replacement, EffectiveAt: effectiveAt,
	}
}
