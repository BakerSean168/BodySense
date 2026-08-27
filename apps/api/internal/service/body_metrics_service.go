package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var ErrInvalidBodyMetric = errors.New("invalid body metric")

type bodyMetricsBodyState interface {
	GetSnapshot(ctx context.Context, userID uuid.UUID, historyLimit int) (*BodyStateSnapshot, error)
	ApplyCurrentContextPatch(ctx context.Context, userID uuid.UUID, expectedRevision *int64, patch model.BodyStateCurrentContextPatch, source string) (*model.BodyStateRevision, error)
}

// BodyMetricsService projects current anthropometric measurements from BodyState.
// Every replacement measurement remains available as historical observation.
type BodyMetricsService struct {
	bodyState bodyMetricsBodyState
}

func NewBodyMetricsService(bodyState bodyMetricsBodyState) *BodyMetricsService {
	return &BodyMetricsService{bodyState: bodyState}
}

func (s *BodyMetricsService) Get(ctx context.Context, userID uuid.UUID) (*dto.BodyMetricsSnapshot, error) {
	snapshot, err := s.bodyState.GetSnapshot(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	result := &dto.BodyMetricsSnapshot{CurrentRevision: snapshot.CurrentRevision}
	for _, observation := range snapshot.Observations {
		switch observation.Kind {
		case model.BodyStateObservationKindHeight:
			result.Height = metricValueFromObservation(observation, "cm")
		case model.BodyStateObservationKindWeight:
			result.Weight = metricValueFromObservation(observation, "kg")
		}
	}
	if result.Height != nil && result.Weight != nil && result.Height.Value > 0 {
		heightM := result.Height.Value / 100
		bmi := math.Round((result.Weight.Value/(heightM*heightM))*10) / 10
		result.BMI = &bmi
	}
	return result, nil
}

func (s *BodyMetricsService) Update(ctx context.Context, userID uuid.UUID, request dto.UpdateBodyMetricsRequest) (*dto.BodyMetricsSnapshot, error) {
	if err := validateBodyMetricValues(request.HeightCm, request.WeightKg); err != nil {
		return nil, err
	}
	patch := model.BodyStateCurrentContextPatch{}
	observedAt := time.Now().UTC()
	if request.HeightCm != nil {
		patch.Observations = append(patch.Observations, metricMutationAt(model.BodyStateObservationKindHeight, *request.HeightCm, "cm", observedAt, "user_measurement"))
	}
	if request.WeightKg != nil {
		patch.Observations = append(patch.Observations, metricMutationAt(model.BodyStateObservationKindWeight, *request.WeightKg, "kg", observedAt, "user_measurement"))
	}
	if len(patch.Observations) > 0 {
		if _, err := s.bodyState.ApplyCurrentContextPatch(ctx, userID, request.ExpectedRevision, patch, "body_metrics"); err != nil {
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

func metricMutationAt(kind string, value float64, unit string, observedAt time.Time, sourceType string) model.BodyStateCurrentObservationMutation {
	valueJSON, _ := json.Marshal(map[string]any{"value": value, "unit": unit})
	provenance, _ := json.Marshal(map[string]any{"source_type": sourceType})
	return model.BodyStateCurrentObservationMutation{
		Kind: kind,
		Replacement: &model.BodyStateObservation{
			Method:         "self_report",
			Value:          datatypes.JSON(valueJSON),
			Condition:      datatypes.JSON(`{}`),
			Provenance:     datatypes.JSON(provenance),
			ObservedAt:     &observedAt,
			ReviewState:    "confirmed",
			LifecycleState: "active",
		},
	}
}

func metricValueFromObservation(observation model.BodyStateObservation, fallbackUnit string) *dto.BodyMetricValue {
	var raw struct {
		Value float64 `json:"value"`
		Unit  string  `json:"unit"`
	}
	if json.Unmarshal(observation.Value, &raw) != nil || raw.Value <= 0 {
		return nil
	}
	if raw.Unit == "" {
		raw.Unit = fallbackUnit
	}
	return &dto.BodyMetricValue{Value: raw.Value, Unit: raw.Unit, ObservedAt: observation.ObservedAt}
}
