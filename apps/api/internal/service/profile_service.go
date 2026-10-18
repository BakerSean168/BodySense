package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// ErrInvalidProfile is the shared Profile validation error.
var ErrInvalidProfile = model.ErrInvalidProfile

// ProfileService owns the minimal optional Profile (PC-D1): display identity
// and preferences. Health facts and observations are deliberately excluded and
// belong to BodyState; age is derived on read and never stored.
type ProfileService struct {
	profileRepo *repository.ProfileRepository
}

func NewProfileService(profileRepo *repository.ProfileRepository) *ProfileService {
	return &ProfileService{profileRepo: profileRepo}
}

func (s *ProfileService) GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error) {
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil || profile == nil {
		return profile, err
	}
	normalizeProfileForCurrentContract(profile, time.Now().UTC())
	return profile, nil
}

// CreateOrUpdateProfile records the onboarding identity answers (gender and
// birth date) without touching display fields or preferences.
func (s *ProfileService) CreateOrUpdateProfile(ctx context.Context, userID uuid.UUID, profile *model.UserProfile) error {
	profile.UserID = userID
	if profile.BirthDate != nil {
		if err := validateBirthDate(profile.BirthDate.Time(), time.Now().UTC()); err != nil {
			return err
		}
	}
	return s.profileRepo.CreateOrUpdate(ctx, profile)
}

// ReplaceProfile replaces every editable Profile field under optimistic
// concurrency. Clearing a field is sending it as null.
func (s *ProfileService) ReplaceProfile(ctx context.Context, userID uuid.UUID, command model.ReplaceProfileCommand) (*model.UserProfile, error) {
	if command.ExpectedRevision < 0 {
		return nil, fmt.Errorf("%w: expected_revision must not be negative", ErrInvalidProfile)
	}
	command.Normalize()
	if err := command.Validate(); err != nil {
		return nil, err
	}
	if command.BirthDate != nil {
		if err := validateBirthDate(command.BirthDate.Time(), time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	profile, err := s.profileRepo.Replace(ctx, userID, command)
	if err != nil || profile == nil {
		return profile, err
	}
	normalizeProfileForCurrentContract(profile, time.Now().UTC())
	return profile, nil
}

func normalizeProfileForCurrentContract(profile *model.UserProfile, now time.Time) {
	profile.ProjectPreferences()
	if profile.BirthDate == nil {
		return
	}
	ageYears := profile.BirthDate.AgeAt(now)
	profile.AgeYears = &ageYears
}

func validateBirthDate(birthDate time.Time, now time.Time) error {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	birthDate = birthDate.UTC()
	if birthDate.IsZero() || birthDate.After(today) || birthDate.Before(today.AddDate(-150, 0, 0)) {
		return fmt.Errorf("%w: birth_date must be within the past 150 years", ErrInvalidProfile)
	}
	return nil
}
