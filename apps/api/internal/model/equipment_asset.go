package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidEquipmentAsset             = errors.New("invalid equipment asset")
	ErrEquipmentAssetNotFound            = errors.New("equipment asset not found")
	ErrEquipmentAssetRevisionConflict    = errors.New("equipment asset revision conflict")
	ErrEquipmentAssetRetired             = errors.New("equipment asset is retired")
	ErrEquipmentAssetDuplicateName       = errors.New("equipment asset name already in use")
	ErrEquipmentAssetIdempotencyConflict = errors.New("equipment asset idempotency conflict")
)

const (
	EquipmentAccessOwned          = "owned"
	EquipmentAccessBorrowed       = "borrowed"
	EquipmentAccessFacilityAccess = "facility_access"

	EquipmentAvailable   = "available"
	EquipmentUnavailable = "unavailable"
	EquipmentRetired     = "retired"

	maxEquipmentNameRunes  = 80
	maxEquipmentNotesRunes = 240
	maxEquipmentKeyRunes   = 128
)

// EquipmentAsset is a user-owned resource the user can (or currently cannot)
// use for training (PC-D2). It is NOT BodyState: it states access to a
// resource, never a habit, symptom or clinical fact, and it carries no BodyState
// revision. Changing it never rewrites an accepted plan; Treatment generation
// only reads the currently available assets as a user constraint.
type EquipmentAsset struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID               uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	DisplayName          string     `gorm:"not null" json:"display_name"`
	Notes                *string    `json:"notes"`
	AccessKind           string     `gorm:"not null" json:"access_kind"`
	Availability         string     `gorm:"not null" json:"availability"`
	ValidFrom            *time.Time `json:"valid_from"`
	ValidUntil           *time.Time `json:"valid_until"`
	Revision             int64      `gorm:"not null" json:"revision"`
	CreateIdempotencyKey string     `gorm:"not null" json:"-"`
	CreateRequestDigest  string     `gorm:"not null" json:"-"`
	CreatedAt            time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"not null" json:"updated_at"`
}

func (EquipmentAsset) TableName() string { return "equipment_assets" }

// EquipmentAssetFields are the user-editable fields shared by create/update.
type EquipmentAssetFields struct {
	DisplayName  string     `json:"display_name"`
	Notes        *string    `json:"notes,omitempty"`
	AccessKind   string     `json:"access_kind"`
	Availability string     `json:"availability"`
	ValidFrom    *time.Time `json:"valid_from,omitempty"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
}

type CreateEquipmentAssetCommand struct {
	IdempotencyKey string `json:"idempotency_key"`
	EquipmentAssetFields
}

type UpdateEquipmentAssetCommand struct {
	ExpectedRevision int64 `json:"expected_revision"`
	EquipmentAssetFields
}

// Normalize trims user text and turns blank notes into "no notes".
func (f *EquipmentAssetFields) Normalize() {
	f.DisplayName = strings.TrimSpace(f.DisplayName)
	if f.Notes != nil {
		notes := strings.TrimSpace(*f.Notes)
		if notes == "" {
			f.Notes = nil
		} else {
			f.Notes = &notes
		}
	}
	if f.ValidFrom != nil {
		value := f.ValidFrom.UTC().Truncate(time.Microsecond)
		f.ValidFrom = &value
	}
	if f.ValidUntil != nil {
		value := f.ValidUntil.UTC().Truncate(time.Microsecond)
		f.ValidUntil = &value
	}
}

func (f EquipmentAssetFields) validate(allowRetired bool) error {
	if f.DisplayName == "" || utf8.RuneCountInString(f.DisplayName) > maxEquipmentNameRunes {
		return fmt.Errorf("%w: display_name must be 1-%d characters", ErrInvalidEquipmentAsset, maxEquipmentNameRunes)
	}
	if f.Notes != nil && utf8.RuneCountInString(*f.Notes) > maxEquipmentNotesRunes {
		return fmt.Errorf("%w: notes must be at most %d characters", ErrInvalidEquipmentAsset, maxEquipmentNotesRunes)
	}
	switch f.AccessKind {
	case EquipmentAccessOwned, EquipmentAccessBorrowed, EquipmentAccessFacilityAccess:
	default:
		return fmt.Errorf("%w: unknown access_kind", ErrInvalidEquipmentAsset)
	}
	switch f.Availability {
	case EquipmentAvailable, EquipmentUnavailable:
	case EquipmentRetired:
		if !allowRetired {
			return fmt.Errorf("%w: a new asset cannot start retired", ErrInvalidEquipmentAsset)
		}
	default:
		return fmt.Errorf("%w: unknown availability", ErrInvalidEquipmentAsset)
	}
	if f.ValidFrom != nil && f.ValidUntil != nil && f.ValidUntil.Before(*f.ValidFrom) {
		return fmt.Errorf("%w: valid_until precedes valid_from", ErrInvalidEquipmentAsset)
	}
	return nil
}

func (c CreateEquipmentAssetCommand) Validate() error {
	key := strings.TrimSpace(c.IdempotencyKey)
	if key == "" || key != c.IdempotencyKey || utf8.RuneCountInString(key) > maxEquipmentKeyRunes {
		return fmt.Errorf("%w: idempotency_key must be 1-%d trimmed characters", ErrInvalidEquipmentAsset, maxEquipmentKeyRunes)
	}
	return c.validate(false)
}

func (c UpdateEquipmentAssetCommand) Validate() error {
	if c.ExpectedRevision < 1 {
		return fmt.Errorf("%w: expected_revision must be positive", ErrInvalidEquipmentAsset)
	}
	return c.validate(true)
}

// SameFields reports whether the stored asset already holds exactly these
// user-editable values (used to recognise an exact update replay).
func (a EquipmentAsset) SameFields(f EquipmentAssetFields) bool {
	return a.DisplayName == f.DisplayName && equalOptionalString(a.Notes, f.Notes) &&
		a.AccessKind == f.AccessKind && a.Availability == f.Availability &&
		equalOptionalTime(a.ValidFrom, f.ValidFrom) && equalOptionalTime(a.ValidUntil, f.ValidUntil)
}

// UsableAt reports whether the asset is available and inside its validity window.
func (a EquipmentAsset) UsableAt(now time.Time) bool {
	if a.Availability != EquipmentAvailable {
		return false
	}
	if a.ValidFrom != nil && now.Before(*a.ValidFrom) {
		return false
	}
	return a.ValidUntil == nil || !now.After(*a.ValidUntil)
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalOptionalTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
