package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// S2 best-effort distillation bookkeeping. This repository reads source text
// only to hand it to the already-admitted S2-B1 proposal path; it never
// stores text, message ids or model output in the ledger. The ledger is
// content-free provenance: seq range, status, attempt count and a stable
// reason code.
const (
	// EpisodicDistillationMaxAttempts is the per-segment retry ceiling. A
	// segment is gapped (and therefore released for pruning) after this many
	// failed model/admission attempts across prune passes.
	EpisodicDistillationMaxAttempts = 3
	// EpisodicDistillationMaxSegmentsPerPass bounds model calls in one prune
	// pass. Segments beyond the bound are recorded as gaps, never silently
	// dropped.
	EpisodicDistillationMaxSegmentsPerPass = 8
	// EpisodicShadowSourceMaxRunes mirrors the private Proto string bound.
	EpisodicShadowSourceMaxRunes = 1200
	// EpisodicShadowSourceWindowRunes mirrors the S2-B1 command budget.
	EpisodicShadowSourceWindowRunes = 8192
)

// EpisodicDistillationStatus values are terminal except pending.
const (
	EpisodicDistillationPending   = "pending"
	EpisodicDistillationDistilled = "distilled"
	EpisodicDistillationSkipped   = "skipped"
	EpisodicDistillationGapped    = "gapped"
)

// EpisodicDistillationState is the cursor view the trigger needs. R is the
// physical prune boundary; D is the last final distillation outcome.
type EpisodicDistillationState struct {
	ConversationID      uuid.UUID
	ProcessedThroughSeq int
	ContextStartSeq     int
	PrunedThroughSeq    int
	DistilledThroughSeq int
}

// EpisodicDistillationMessage is one text-only source or context row. It is
// not persisted by this repository.
type EpisodicDistillationMessage struct {
	ID   uuid.UUID
	Seq  int
	Role string
	Text string
}

// EpisodicDistillationSegment is a complete interaction (exactly one user and
// one assistant message, contiguous seqs) that retention is about to remove.
// Status/Attempts come from the ledger; Eligible/Reason are a conservative
// pre-check and SaveShadowDraft remains the authoritative gate.
type EpisodicDistillationSegment struct {
	FromSeq  int
	ToSeq    int
	Messages []EpisodicDistillationMessage
	Context  []EpisodicDistillationMessage
	Eligible bool
	Reason   string
	Status   string
	Attempts int
	NoteID   *uuid.UUID
}

type episodicDistillationRow struct {
	ID                uuid.UUID
	TurnID            uuid.UUID
	Seq               int
	Role              string
	Status            string
	Parts             string
	ContentBytes      int
	ContentText       string
	RunStatus         *string
	RunUserID         *uuid.UUID
	RunConversationID *uuid.UUID
	TurnSize          int
	ClinicalRefs      int
}

type episodicDistillationLedgerRow struct {
	FromSeq  int
	ToSeq    int
	Status   string
	Attempts int
	NoteID   *uuid.UUID
}

// EpisodicDistillationRepository is the server-only bookkeeping boundary for
// the S2 trigger. It is not wired to HTTP, chat serving or clinical inputs.
type EpisodicDistillationRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewEpisodicDistillationRepository(db *gorm.DB) *EpisodicDistillationRepository {
	return &EpisodicDistillationRepository{db: db, now: time.Now}
}
