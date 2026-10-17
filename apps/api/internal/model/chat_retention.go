package model

import (
	"errors"

	"github.com/google/uuid"
)

// Chat retention (PC-D6, PC-RET-001). The singleton Conversation carries three
// cursors over its Go-assigned message seq numbers. Seqs are never renumbered
// by pruning, so a cursor stays meaningful after the rows it passed are gone.
//
//   - ProcessedThroughSeq P: every message with seq <= P has finished semantic
//     processing (its Run is terminal and any domain commit it made already
//     holds a receipt). Messages after P are never treated as processed.
//   - ContextStartSeq C: the first seq the AI context may still read as raw
//     message text. Go context assembly ignores raw messages before C.
//   - PrunedThroughSeq R: the physical-deletion boundary. Raw messages and
//     their controlled runtime copies at or before R are gone, except rows a
//     durable reference still holds (they are retained, never dangled).
//
// Ordering, enforced in Go and by conversations_retention_cursor_order_check:
//
//	0 <= R < C <= P + 1   (so R <= P and nothing unprocessed leaves context)
//
// Every cursor is monotonic and every move bumps RetentionRevision.
const ChatRetentionPruneJobType = "chat.retention.prune"

var (
	// ErrRetentionNotFound masks a missing or foreign Conversation.
	ErrRetentionNotFound = errors.New("conversation not found")
	// ErrRetentionRevisionConflict is a stale expected revision.
	ErrRetentionRevisionConflict = errors.New("retention revision conflict")
	// ErrRetentionCursorRegression rejects any move backwards.
	ErrRetentionCursorRegression = errors.New("retention cursor cannot move backwards")
	// ErrRetentionCursorInvalid rejects a target that would skip unprocessed
	// messages, point past the transcript or split an interaction.
	ErrRetentionCursorInvalid = errors.New("invalid retention cursor target")
	// ErrRetentionOwnerAnomaly reports rows in the owner's Conversation that
	// belong to, or are referenced by, another owner. Retention fails closed.
	ErrRetentionOwnerAnomaly = errors.New("retention owner anomaly")
)

// ChatRetentionCursor is the current retention state of one Conversation.
type ChatRetentionCursor struct {
	ConversationID      uuid.UUID `json:"conversationId"`
	ProcessedThroughSeq int       `json:"processedThroughSeq"`
	ContextStartSeq     int       `json:"contextStartSeq"`
	PrunedThroughSeq    int       `json:"prunedThroughSeq"`
	RetentionRevision   int64     `json:"retentionRevision"`
}

// ValidRetentionOrder reports whether cursors satisfy 0 <= R < C <= P + 1.
func ValidRetentionOrder(processed, contextStart, pruned int) bool {
	return pruned >= 0 && processed >= pruned && contextStart > pruned && contextStart <= processed+1
}

// AdvanceRetentionCursorCommand moves one cursor forward under an expected
// revision. Repeating an already-applied target is an idempotent no-op.
type AdvanceRetentionCursorCommand struct {
	ExpectedRevision int64
	ThroughSeq       int
}

// ChatRetentionPruneReceipt is the durable result of one prune pass, stored as
// the result of a completed `chat.retention.prune` job.
type ChatRetentionPruneReceipt struct {
	JobID                uuid.UUID `json:"jobId"`
	ConversationID       uuid.UUID `json:"conversationId"`
	FromPrunedSeq        int       `json:"fromPrunedSeq"`
	PrunedThroughSeq     int       `json:"prunedThroughSeq"`
	RetentionRevision    int64     `json:"retentionRevision"`
	DeletedMessages      int64     `json:"deletedMessages"`
	RetainedMessages     int       `json:"retainedMessages"`
	PrunedRuns           int       `json:"prunedRuns"`
	DeletedRuntimeEvents int64     `json:"deletedRuntimeEvents"`
	DeletedToolCalls     int64     `json:"deletedToolCalls"`
	// CheckpointCleanup is "not_run" until the ai-service cleanup receipt
	// exists (PC-RET-001 slice 2); this pass touched no LangGraph state.
	CheckpointCleanup string `json:"checkpointCleanup"`
	// Replayed is true when the receipt came from an earlier identical pass.
	Replayed bool `json:"replayed"`
}
