package service

import "errors"

var (
	// ErrUnknownBodyRegionID means a caller supplied a canonical region identity
	// that is not present in the configured BodyRegion ontology authority.
	ErrUnknownBodyRegionID = errors.New("unknown body region id")

	// ErrBodyRegionIDRequired means an anatomically localized durable value cannot
	// be resolved to one canonical BodyRegion identity.
	ErrBodyRegionIDRequired = errors.New("canonical body region id is required")

	// ErrBodyRegionIDValidationUnavailable prevents the durable layer from
	// accepting a new canonical identity before the authoritative ontology has
	// been wired.
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
