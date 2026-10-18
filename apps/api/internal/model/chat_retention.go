package model

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

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
//     durable reference still holds (they are retained, never dangled). R
//     moves only after the ai-service proved the LangGraph checkpoint copies
//     of the pruned Runs are gone (CheckpointCleanupReceipt).
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
	// ErrRetentionBusy: a Run is active, waiting on Ask User or still
	// streaming. Its LangGraph thread is mid-step, so no prune pass runs until
	// it settles. Retry later.
	ErrRetentionBusy = errors.New("conversation has an unsettled run")
	// ErrRetentionPlanStale: the Conversation changed between planning a pass
	// and committing it. Nothing was deleted; plan again.
	ErrRetentionPlanStale = errors.New("retention prune plan is stale")
	// ErrRetentionCleanupUnverified: the ai-service checkpoint cleanup receipt
	// does not prove the planned Runs are gone. The cursor does not move.
	ErrRetentionCleanupUnverified = errors.New("checkpoint cleanup receipt not verified")
)

// Context window policy (PC-RET-001 slice 3, PC-D9). C advances by a token
// budget, not a turn count: the raw messages from C onward are kept at or
// below ChatContextTokenBudget estimated tokens, cut only at a complete
// interaction boundary, never past an unsettled Run and never past P + 1. An
// unanswered Ask User keeps its Run unsettled, so that thread always stays in
// context (and unpruned) until it is answered, cancelled or expires.
const (
	// ChatContextTokenBudgetEnv overrides the default budget.
	ChatContextTokenBudgetEnv = "CHAT_CONTEXT_TOKEN_BUDGET"
	// DefaultChatContextTokenBudget is the raw-history share of the model
	// context. The consultation route has no tokenizer and its physical model
	// is swappable behind the gateway, so the default is sized for the
	// smallest window the route is expected to see (32K): 16K of raw history
	// plus the system prompt, BodyState/longitudinal context, tool schemas
	// and the 2048-token reply still leave headroom. It also covers the old
	// MAX_CONTEXT_TURNS=10 window for ordinary turns.
	DefaultChatContextTokenBudget = 16000
	// MinChatContextTokenBudget rejects budgets too small to hold one turn.
	MinChatContextTokenBudget = 1024
	// ChatContextTokenOverheadPerMessage covers role and framing tokens.
	ChatContextTokenOverheadPerMessage = 4
	// ChatContextBytesPerToken is the estimator divisor: one token per three
	// UTF-8 bytes of the larger of content_text and the parts JSON. A CJK
	// character (3 bytes) counts as one token and ASCII as one per three
	// characters, both above what BPE tokenizers of the routed models emit,
	// so the estimate errs on the side of a smaller context.
	ChatContextBytesPerToken = 3
)

// ChatContextTokenBudget returns the configured budget, falling back to the
// default when the variable is unset or not an integer >= the minimum.
func ChatContextTokenBudget(lookup func(string) string) int {
	raw := strings.TrimSpace(lookup(ChatContextTokenBudgetEnv))
	if raw == "" {
		return DefaultChatContextTokenBudget
	}
	budget, err := strconv.Atoi(raw)
	if err != nil || budget < MinChatContextTokenBudget {
		return DefaultChatContextTokenBudget
	}
	return budget
}

// EstimateChatTokens is the Go form of the estimator the retention query
// applies in SQL (see liveMessages); both must stay identical.
func EstimateChatTokens(contentBytes, partsBytes int) int {
	size := max(contentBytes, partsBytes, 0)
	return ChatContextTokenOverheadPerMessage + (size+ChatContextBytesPerToken-1)/ChatContextBytesPerToken
}

// ChatContextWindow is what the AI runtime may read as raw history: every
// message from ContextStartSeq on, named by the Runs that own them (in seq
// order). LangGraph keeps runtime_messages by Run, not by Go seq, so the
// ai-service drops every checkpoint segment before the first of these Runs.
type ChatContextWindow struct {
	ContextStartSeq int
	RunIDs          []uuid.UUID
}

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

// ChatRetentionPrunePlan is a prune pass computed under the Conversation lock
// but not applied. The ai-service cleans the checkpoint copies of RunIDs first;
// the commit then re-checks that nothing changed and applies the plan.
type ChatRetentionPrunePlan struct {
	ConversationID   uuid.UUID
	FromPrunedSeq    int
	TargetSeq        int
	LastSeq          int
	ExpectedRevision int64
	// RunIDs are every Run whose messages all sit at or before TargetSeq,
	// sorted. Their LangGraph segments are what the cleanup removes.
	RunIDs []uuid.UUID
}

// CheckpointCleanupReceipt is the ai-service answer to a checkpoint cleanup
// (POST /runtime/threads/{conversation}/checkpoints/cleanup).
type CheckpointCleanupReceipt struct {
	Status             string   `json:"status"`
	ThreadID           string   `json:"thread_id"`
	PrunedRunIDs       []string `json:"pruned_run_ids"`
	CheckpointID       *string  `json:"checkpoint_id"`
	RemovedMessages    int      `json:"removed_messages"`
	RetainedMessages   int      `json:"retained_messages"`
	DeletedCheckpoints int      `json:"deleted_checkpoints"`
	RemainingRunIDs    []string `json:"remaining_run_ids"`
}

// VerifyCleanup checks that a cleanup receipt covers exactly this plan: the
// same thread, the same Runs, and none of them left in the thread.
func (p *ChatRetentionPrunePlan) VerifyCleanup(receipt *CheckpointCleanupReceipt) error {
	if receipt == nil || receipt.Status != "cleaned" {
		return fmt.Errorf("%w: no cleaned receipt", ErrRetentionCleanupUnverified)
	}
	if receipt.ThreadID != p.ConversationID.String() {
		return fmt.Errorf("%w: receipt names another thread", ErrRetentionCleanupUnverified)
	}
	want := make([]string, len(p.RunIDs))
	for i, id := range p.RunIDs {
		want[i] = id.String()
	}
	got := slices.Clone(receipt.PrunedRunIDs)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		return fmt.Errorf("%w: receipt covers other runs", ErrRetentionCleanupUnverified)
	}
	for _, remaining := range receipt.RemainingRunIDs {
		if slices.Contains(want, remaining) {
			return fmt.Errorf("%w: run %s still in the thread", ErrRetentionCleanupUnverified, remaining)
		}
	}
	return nil
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
	// DeletedInteractions counts uncited Ask User interactions (with their
	// resume receipts) removed with the pruned window.
	DeletedInteractions int64 `json:"deletedInteractions"`
	// CheckpointCleanup is "verified" once the ai-service receipt proved the
	// LangGraph copies of the pruned Runs are gone; R never moves without it.
	CheckpointCleanup         string `json:"checkpointCleanup"`
	CheckpointID              string `json:"checkpointId,omitempty"`
	CheckpointMessagesRemoved int    `json:"checkpointMessagesRemoved"`
	CheckpointsDeleted        int    `json:"checkpointsDeleted"`
	// Replayed is true when the receipt came from an earlier identical pass.
	Replayed bool `json:"replayed"`
}
