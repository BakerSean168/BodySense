package model

import "errors"

// ErrConversationRunInProgress protects one conversation/thread from concurrent
// Agent executions that would compete for message sequence, checkpoint, and
// active-run ownership.
var ErrConversationRunInProgress = errors.New("conversation already has an active run")
