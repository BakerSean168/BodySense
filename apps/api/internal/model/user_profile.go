package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidProfile          = errors.New("invalid profile")
	ErrProfileRevisionConflict = errors.New("profile revision conflict")
	// ErrProfileAvatarUnavailable masks a foreign, missing, non-photo or
	// no-longer-available avatar asset behind one answer (owner isolation).
	ErrProfileAvatarUnavailable = errors.New("profile avatar asset unavailable")
)

const (
	ProfileGenderMale   = "male"
	ProfileGenderFemale = "female"

	UnitPreferenceMetric   = "metric"
	UnitPreferenceImperial = "imperial"

	maxProfileDisplayNameRunes = 40
	maxProfileTimeZoneRunes    = 64
	maxProfileLocaleRunes      = 35
)

var profileLocalePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// UserProfile is the minimal optional Profile (PC-D1): display identity and
// user preferences only. Time-varying health state (height, weight,
// lifestyle, injuries, symptoms) lives in BodyState and is never stored here.
// Gender is the user's self-reported answer, not a physiological or medical
// determination. Every editable field is optional; Revision is the internal
// concurrency token and never a user-facing value.
type UserProfile struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID         uuid.UUID          `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	DisplayName    *string            `gorm:"type:text" json:"display_name,omitempty"`
	AvatarAssetID  *uuid.UUID         `gorm:"type:uuid" json:"avatar_asset_id,omitempty"`
	Gender         *string            `gorm:"type:varchar(20)" json:"gender,omitempty"`
	BirthDate      *DateOnly          `gorm:"type:date" json:"birth_date,omitempty"`
	AgeYears       *int               `gorm:"-" json:"age_years,omitempty"`
	Locale         *string            `gorm:"type:text" json:"-"`
	TimeZone       *string            `gorm:"type:text" json:"-"`
	UnitPreference *string            `gorm:"type:text" json:"-"`
	Preferences    ProfilePreferences `gorm:"-" json:"preferences"`
	Revision       int64              `gorm:"not null;default:1" json:"revision"`
	CreatedAt      time.Time          `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time          `gorm:"not null;default:now()" json:"updated_at"`
}

func (UserProfile) TableName() string { return "user_profiles" }

// ProfilePreferences is the logical preference sub-structure. Changing it only
// changes presentation and the interpretation of new input; it never rewrites
// stored Observation values/units, training dates or historical occurrences.
type ProfilePreferences struct {
	Locale         *string `json:"locale,omitempty"`
	TimeZone       *string `json:"time_zone,omitempty"`
	UnitPreference *string `json:"unit_preference,omitempty"`
}

// ProjectPreferences fills the logical preference view from the stored columns.
func (p *UserProfile) ProjectPreferences() {
	p.Preferences = ProfilePreferences{Locale: p.Locale, TimeZone: p.TimeZone, UnitPreference: p.UnitPreference}
}

// ProfileAgentContext is the only Profile content an Agent/replay input may
// carry: self-reported identity context with its semantics spelled out. Display
// name, avatar and preferences are presentation data and are not forwarded.
type ProfileAgentContext struct {
	Gender    *string   `json:"gender,omitempty"`
	BirthDate *DateOnly `json:"birth_date,omitempty"`
	AgeYears  *int      `json:"age_years,omitempty"`
}

// AgentContext returns the allowlisted Agent view of this profile.
func (p *UserProfile) AgentContext() ProfileAgentContext {
	if p == nil {
		return ProfileAgentContext{}
	}
	return ProfileAgentContext{Gender: p.Gender, BirthDate: p.BirthDate, AgeYears: p.AgeYears}
}

// ProfileFields are the user-editable Profile values. A nil field is "not
// provided" and is stored as NULL: the update is a full replacement.
type ProfileFields struct {
	DisplayName    *string    `json:"display_name"`
	AvatarAssetID  *uuid.UUID `json:"avatar_asset_id"`
	Gender         *string    `json:"gender"`
	BirthDate      *DateOnly  `json:"birth_date"`
	Locale         *string    `json:"locale"`
	TimeZone       *string    `json:"time_zone"`
	UnitPreference *string    `json:"unit_preference"`
}

// ReplaceProfileCommand replaces every editable field under optimistic
// concurrency. ExpectedRevision is 0 when the user has no profile yet.
type ReplaceProfileCommand struct {
	ExpectedRevision int64
	ProfileFields
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// Normalize trims text and turns blank values into "not provided".
func (f *ProfileFields) Normalize() {
	f.DisplayName = trimOptional(f.DisplayName)
	f.Gender = trimOptional(f.Gender)
	f.Locale = trimOptional(f.Locale)
	f.TimeZone = trimOptional(f.TimeZone)
	f.UnitPreference = trimOptional(f.UnitPreference)
	if f.AvatarAssetID != nil && *f.AvatarAssetID == uuid.Nil {
		f.AvatarAssetID = nil
	}
}

// Validate checks field shapes. Birth-date range is checked by the service
// against the current clock; avatar ownership by the repository.
func (f ProfileFields) Validate() error {
	if f.DisplayName != nil && utf8.RuneCountInString(*f.DisplayName) > maxProfileDisplayNameRunes {
		return fmt.Errorf("%w: display_name must be at most %d characters", ErrInvalidProfile, maxProfileDisplayNameRunes)
	}
	if f.Gender != nil && *f.Gender != ProfileGenderMale && *f.Gender != ProfileGenderFemale {
		return fmt.Errorf("%w: unknown gender", ErrInvalidProfile)
	}
	if f.Locale != nil && (utf8.RuneCountInString(*f.Locale) > maxProfileLocaleRunes || !profileLocalePattern.MatchString(*f.Locale)) {
		return fmt.Errorf("%w: locale must be a BCP 47 language tag", ErrInvalidProfile)
	}
	if f.TimeZone != nil {
		if utf8.RuneCountInString(*f.TimeZone) > maxProfileTimeZoneRunes || !ValidIANATimeZone(*f.TimeZone) {
			return fmt.Errorf("%w: time_zone must be an IANA identifier", ErrInvalidProfile)
		}
	}
	if f.UnitPreference != nil && *f.UnitPreference != UnitPreferenceMetric && *f.UnitPreference != UnitPreferenceImperial {
		return fmt.Errorf("%w: unknown unit_preference", ErrInvalidProfile)
	}
	return nil
}

// ValidIANATimeZone accepts only a loadable IANA zone; "Local" would silently
// depend on the server locale.
func ValidIANATimeZone(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// SameFields reports whether the stored profile already holds exactly these
// values (used to recognise an exact replay of the update that produced it).
func (p UserProfile) SameFields(f ProfileFields) bool {
	return equalOptionalString(p.DisplayName, f.DisplayName) && equalOptionalUUID(p.AvatarAssetID, f.AvatarAssetID) &&
		equalOptionalString(p.Gender, f.Gender) && equalOptionalDate(p.BirthDate, f.BirthDate) &&
		equalOptionalString(p.Locale, f.Locale) && equalOptionalString(p.TimeZone, f.TimeZone) &&
		equalOptionalString(p.UnitPreference, f.UnitPreference)
}

func equalOptionalUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalOptionalDate(a, b *DateOnly) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.String() == b.String()
}
