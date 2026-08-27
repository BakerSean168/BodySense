package dto

// OnboardingContextRequest is a capture contract, not a persistence model.
// Each nested section is routed by the application service to the aggregate
// that owns it: stable identity -> UserProfile; mutable health context -> BodyState.
type OnboardingContextRequest struct {
	ExpectedBodyStateRevision *int64                     `json:"expected_body_state_revision,omitempty"`
	Profile                   OnboardingProfileInput     `json:"profile"`
	BodyMetrics               OnboardingBodyMetricsInput `json:"body_metrics"`
	Lifestyle                 OnboardingLifestyleInput   `json:"lifestyle"`
	InjuryHistory             string                     `json:"injury_history"`
}

type OnboardingProfileInput struct {
	Gender    string `json:"gender"`
	BirthDate string `json:"birth_date"`
}

type OnboardingBodyMetricsInput struct {
	HeightCm *float64 `json:"height_cm"`
	WeightKg *float64 `json:"weight_kg"`
}

type OnboardingLifestyleInput struct {
	Activity   LifestyleSectionInput `json:"activity"`
	Sleep      LifestyleSectionInput `json:"sleep"`
	Exercise   LifestyleSectionInput `json:"exercise"`
	Nutrition  LifestyleSectionInput `json:"nutrition"`
	Substances LifestyleSectionInput `json:"substances"`
	Recovery   LifestyleSectionInput `json:"recovery"`
}

type OnboardingContextResult struct {
	BodyStateRevision *int64 `json:"body_state_revision,omitempty"`
}
