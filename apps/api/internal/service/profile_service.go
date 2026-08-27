package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// ProfileService handles profile business logic.
type ProfileService struct {
	profileRepo *repository.ProfileRepository
}

// NewProfileService creates a new ProfileService.
func NewProfileService(profileRepo *repository.ProfileRepository) *ProfileService {
	return &ProfileService{profileRepo: profileRepo}
}

// GetProfile retrieves a user profile by user ID.
func (s *ProfileService) GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error) {
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil || profile == nil {
		return profile, err
	}
	normalizeProfileForCurrentContract(profile, time.Now().UTC())
	return profile, nil
}

// CreateOrUpdateProfile creates or updates a user profile.
// Automatically calculates BMI if height and weight are provided.
func (s *ProfileService) CreateOrUpdateProfile(ctx context.Context, userID uuid.UUID, profile *model.UserProfile) error {
	// Set the user ID
	profile.UserID = userID

	// Calculate BMI if both height and weight are provided
	if profile.HeightCm != nil && profile.WeightKg != nil {
		if *profile.HeightCm <= 0 {
			return errors.New("height must be positive")
		}
		if *profile.WeightKg <= 0 {
			return errors.New("weight must be positive")
		}

		// BMI = weight(kg) / height(m)²
		heightM := *profile.HeightCm / 100.0
		bmi := *profile.WeightKg / (heightM * heightM)
		// Round to 1 decimal place
		bmi = math.Round(bmi*10) / 10
		profile.BMI = &bmi
	}

	// Birth date is the canonical source for age. Persisting an age number would
	// become stale over time, so new clients store the date and derive age only
	// when a runtime actually needs it.
	if profile.BirthDate != nil {
		if err := validateBirthDate(profile.BirthDate.Time(), time.Now().UTC()); err != nil {
			return err
		}
	}

	// Legacy API compatibility while existing clients migrate to birth_date.
	if profile.Age != nil && (*profile.Age < 1 || *profile.Age > 150) {
		return errors.New("age must be between 1 and 150")
	}

	return s.profileRepo.CreateOrUpdate(ctx, profile)
}

func normalizeProfileForCurrentContract(profile *model.UserProfile, now time.Time) {
	if profile.BirthDate != nil {
		ageYears := profile.BirthDate.AgeAt(now)
		profile.AgeYears = &ageYears
		profile.Age = nil
	}

	// Retired intake fields stay in the database only for migration/backward-write
	// compatibility. Current API reads and every downstream AI consumer share the
	// new health-context contract, so job titles and duplicate self-description do
	// not silently re-enter reasoning through an older row.
	profile.Occupation = nil
	profile.SleepTime = nil
	profile.WakeTime = nil
	profile.SelfDescription = nil
}

func validateBirthDate(birthDate time.Time, now time.Time) error {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	birthDate = birthDate.UTC()
	if birthDate.IsZero() || birthDate.After(today) || birthDate.Before(today.AddDate(-150, 0, 0)) {
		return errors.New("birth_date must be within the past 150 years")
	}
	return nil
}
