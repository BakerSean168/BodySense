package service

import (
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
)

func TestNormalizeProfileDerivesAgeOnlyFromBirthDate(t *testing.T) {
	birthDate, err := model.ParseDateOnly("1998-09-20")
	if err != nil {
		t.Fatalf("parse birth date: %v", err)
	}
	profile := &model.UserProfile{BirthDate: &birthDate}

	normalizeProfileForCurrentContract(profile, time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC))

	if profile.AgeYears == nil || *profile.AgeYears != 27 {
		t.Fatalf("age_years = %v, want 27", profile.AgeYears)
	}
}

func TestValidateBirthDate(t *testing.T) {
	now := time.Date(2026, time.August, 27, 14, 40, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	tests := []struct {
		name      string
		birthDate time.Time
		wantErr   bool
	}{
		{name: "ordinary date", birthDate: time.Date(1998, time.May, 20, 0, 0, 0, 0, time.UTC)},
		{name: "today", birthDate: time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC)},
		{name: "exactly 150 years", birthDate: time.Date(1876, time.August, 27, 0, 0, 0, 0, time.UTC)},
		{name: "future date", birthDate: time.Date(2026, time.August, 28, 0, 0, 0, 0, time.UTC), wantErr: true},
		{name: "older than 150 years", birthDate: time.Date(1876, time.August, 26, 0, 0, 0, 0, time.UTC), wantErr: true},
		{name: "zero date", birthDate: time.Time{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBirthDate(tt.birthDate, now)
			if tt.wantErr && err == nil {
				t.Fatalf("validateBirthDate(%s) expected error", tt.birthDate)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateBirthDate(%s) unexpected error: %v", tt.birthDate, err)
			}
		})
	}
}
