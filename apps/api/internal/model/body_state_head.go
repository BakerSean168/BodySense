package model

import (
	"github.com/google/uuid"
)

// BodyStateHead is the aggregate head a policy pins before reading the V2
// revision ledger: the owner, the committed revision and the Safety state.
type BodyStateHead struct {
	UserID          uuid.UUID
	CurrentRevision int64
}
