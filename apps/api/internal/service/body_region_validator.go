package service

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrUnknownBodyRegionID means a caller supplied a canonical region identity
	// that is not present in the configured BodyRegion ontology authority.
	ErrUnknownBodyRegionID = errors.New("unknown body region id")

	// ErrBodyRegionIDValidationUnavailable prevents the durable layer from
	// accepting a new canonical identity before the authoritative ontology has
	// been wired. Legacy records without body_region_id remain fully supported.
	ErrBodyRegionIDValidationUnavailable = errors.New("body region id validation unavailable")
)

// BodyRegionIDValidator is the stable Go-side seam to the canonical BodyRegion
// ontology. BODY3D-1110 deliberately does not duplicate the ontology here;
// integration must adapt Worker B's generated canonical values to this interface.
type BodyRegionIDValidator interface {
	IsValidBodyRegionID(id string) bool
}

// BodyRegionIDValidatorFunc keeps integration adapters and focused tests small.
type BodyRegionIDValidatorFunc func(id string) bool

func (f BodyRegionIDValidatorFunc) IsValidBodyRegionID(id string) bool {
	return f(id)
}

func (s *BodyStateService) normalizeBodyRegionID(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	id := strings.TrimSpace(*raw)
	if id == "" {
		return nil, nil
	}
	if s.bodyRegionIDValidator == nil {
		return nil, fmt.Errorf("%w: %q", ErrBodyRegionIDValidationUnavailable, id)
	}
	if !s.bodyRegionIDValidator.IsValidBodyRegionID(id) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownBodyRegionID, id)
	}
	return &id, nil
}
