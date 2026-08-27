package service

import (
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
)

func TestNormalizeProfileForCurrentContract(t *testing.T) {
	birthDate, err := model.ParseDateOnly("1998-09-20")
	if err != nil {
		t.Fatalf("parse birth date: %v", err)
	}
	legacyAge := 27
	occupation := "程序员"
	sleepTime := "23:30"
	wakeTime := "07:00"
	selfDescription := "最近肩颈不舒服"
	profile := &model.UserProfile{
		BirthDate:       &birthDate,
		Age:             &legacyAge,
		Occupation:      &occupation,
		SleepTime:       &sleepTime,
		WakeTime:        &wakeTime,
		SelfDescription: &selfDescription,
	}

	normalizeProfileForCurrentContract(profile, time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC))

	if profile.AgeYears == nil || *profile.AgeYears != 27 {
		t.Fatalf("age_years = %v, want 27", profile.AgeYears)
	}
	if profile.Age != nil || profile.Occupation != nil || profile.SleepTime != nil || profile.WakeTime != nil || profile.SelfDescription != nil {
		t.Fatal("retired profile fields must not escape the current read contract")
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
