package service

import (
	"context"
	"errors"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// ProfileService owns stable identity context only. Health facts and
// observations are deliberately excluded and belong to BodyState.
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

func (s *ProfileService) CreateOrUpdateProfile(ctx context.Context, userID uuid.UUID, profile *model.UserProfile) error {
	profile.UserID = userID
	if profile.BirthDate != nil {
		if err := validateBirthDate(profile.BirthDate.Time(), time.Now().UTC()); err != nil {
			return err
		}
	}
	return s.profileRepo.CreateOrUpdate(ctx, profile)
}

func normalizeProfileForCurrentContract(profile *model.UserProfile, now time.Time) {
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
		return errors.New("birth_date must be within the past 150 years")
	}
	return nil
}
