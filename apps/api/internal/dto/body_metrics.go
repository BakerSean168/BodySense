package dto

import "time"

type BodyMetricValue struct {
	Value      float64    `json:"value"`
	Unit       string     `json:"unit"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
}

type BodyMetricsSnapshot struct {
	CurrentRevision int64            `json:"current_revision"`
	Height          *BodyMetricValue `json:"height,omitempty"`
	Weight          *BodyMetricValue `json:"weight,omitempty"`
	BMI             *float64         `json:"bmi,omitempty"`
}

type UpdateBodyMetricsRequest struct {
	ExpectedRevision *int64   `json:"expected_revision,omitempty"`
	HeightCm         *float64 `json:"height_cm,omitempty"`
	WeightKg         *float64 `json:"weight_kg,omitempty"`
}
