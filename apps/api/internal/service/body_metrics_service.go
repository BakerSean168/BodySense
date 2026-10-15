package service

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
)

var ErrInvalidBodyMetric = errors.New("invalid body metric")

type bodyContextStore interface {
	CurrentBodyContext(ctx context.Context, userID uuid.UUID) (*BodyContext, error)
	UpdateBodyContext(ctx context.Context, userID uuid.UUID, expectedRevision *int64, update BodyContextUpdate) (int64, error)
}

// BodyMetricsService projects current height/weight from the user's V2
// BodyObservation reports. Every new measurement stays as history.
type BodyMetricsService struct {
	bodyState bodyContextStore
}

func NewBodyMetricsService(bodyState bodyContextStore) *BodyMetricsService {
	return &BodyMetricsService{bodyState: bodyState}
}

func (s *BodyMetricsService) Get(ctx context.Context, userID uuid.UUID) (*BodyMetricsSnapshot, error) {
	current, err := s.bodyState.CurrentBodyContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := &BodyMetricsSnapshot{
		CurrentRevision: current.Revision,
		Height:          bodyContextMetric(current.Height),
		Weight:          bodyContextMetric(current.Weight),
	}
	if result.Height != nil && result.Weight != nil && result.Height.Value > 0 {
		heightM := result.Height.Value / 100
		bmi := math.Round((result.Weight.Value/(heightM*heightM))*10) / 10
		result.BMI = &bmi
	}
	return result, nil
}

func (s *BodyMetricsService) Update(ctx context.Context, userID uuid.UUID, request UpdateBodyMetricsRequest) (*BodyMetricsSnapshot, error) {
	if err := validateBodyMetricValues(request.HeightCm, request.WeightKg); err != nil {
		return nil, err
	}
	if request.HeightCm != nil || request.WeightKg != nil {
		if _, err := s.bodyState.UpdateBodyContext(ctx, userID, request.ExpectedRevision, BodyContextUpdate{
			HeightCm: request.HeightCm, WeightKg: request.WeightKg, Context: "body_metrics_editor",
		}); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, userID)
}

func validateBodyMetricValues(heightCm, weightKg *float64) error {
	if heightCm != nil && (*heightCm < 50 || *heightCm > 250) {
		return ErrInvalidBodyMetric
	}
	if weightKg != nil && (*weightKg < 20 || *weightKg > 300) {
		return ErrInvalidBodyMetric
	}
	return nil
}
