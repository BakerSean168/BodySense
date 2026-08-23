package model

import "errors"

// ErrConversationRunInProgress protects one conversation/thread from concurrent
// Agent executions that would compete for message sequence, checkpoint, and
// active-run ownership.
var ErrConversationRunInProgress = errors.New("conversation already has an active run")

// ErrRunLeaseExpired is returned when a stale running run is reclaimed because
// its lease expired (i.e. the owning process is assumed dead).
var ErrRunLeaseExpired = errors.New("run lease expired; execution reclaimed")
