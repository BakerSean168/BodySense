package service

import (
	"context"
	"errors"
	"math"

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
	return s.profileRepo.GetByUserID(ctx, userID)
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

	// Validate age if provided
	if profile.Age != nil {
		if *profile.Age < 1 || *profile.Age > 150 {
			return errors.New("age must be between 1 and 150")
		}
	}

	return s.profileRepo.CreateOrUpdate(ctx, profile)
}
