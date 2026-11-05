package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// EpisodicShadowGenerationEnv is the default-off kill switch for the S2
// best-effort trigger. Like the S6 controls flag, only an explicit true/1
// opts in; a wired env flag alone still cannot generate because no approved
// model proposer is connected in the production constructor.
const EpisodicShadowGenerationEnv = "EPISODIC_SHADOW_GENERATION_ENABLED"

const (
	episodicProposalTimeout = 5 * time.Second
	episodicPassBudget      = 20 * time.Second
)

// EpisodicShadowGenerationEnabledFromEnv reads the default-off kill switch.
func EpisodicShadowGenerationEnabledFromEnv(getenv func(string) string) bool {
	value := strings.TrimSpace(getenv(EpisodicShadowGenerationEnv))
	return strings.EqualFold(value, "true") || value == "1"
}

// EpisodicDistillationStats is content-free accounting recorded on the prune
// job. It names no source text and no note body.
type EpisodicDistillationStats struct {
	Considered int `json:"considered"`
	Distilled  int `json:"distilled"`
	Abstained  int `json:"abstained"`
	Skipped    int `json:"skipped"`
	Gaps       int `json:"gaps"`
	Failures   int `json:"failures"`
	Retries    int `json:"retries"`
}

func (s EpisodicDistillationStats) any() bool {
	return s.Considered != 0 || s.Distilled != 0 || s.Abstained != 0 ||
		s.Skipped != 0 || s.Gaps != 0 || s.Failures != 0 || s.Retries != 0
}

// EpisodicDistillationTrigger runs at the prune boundary. It must never
// return an error that aborts a prune: every failure is recorded and the raw
// rows are released on schedule. It reads source text only to build the
// already-admitted S2-B1 typed command; it never serves notes or feeds them
// into any clinical or chat context.
type EpisodicDistillationTrigger struct {
	repo     *repository.EpisodicDistillationRepository
	writer   *repository.EpisodicShadowRepository
	proposer EpisodicShadowProposer
	enabled  bool

	proposalTimeout time.Duration
	passBudget      time.Duration
	maxSegments     int
	maxAttempts     int
}

func NewEpisodicDistillationTrigger(
	repo *repository.EpisodicDistillationRepository,
	writer *repository.EpisodicShadowRepository,
	proposer EpisodicShadowProposer,
) *EpisodicDistillationTrigger {
	return &EpisodicDistillationTrigger{
		repo: repo, writer: writer, proposer: proposer,
		proposalTimeout: episodicProposalTimeout, passBudget: episodicPassBudget,
		maxSegments: repository.EpisodicDistillationMaxSegmentsPerPass,
		maxAttempts: repository.EpisodicDistillationMaxAttempts,
	}
}

// NewEpisodicDistillationTriggerFromEnv wires the flag but keeps generation
// off unless an approved proposer is explicitly supplied. Setting the env
// flag without a proposer logs once and remains inert.
func NewEpisodicDistillationTriggerFromEnv(
	getenv func(string) string,
	repo *repository.EpisodicDistillationRepository,
	writer *repository.EpisodicShadowRepository,
	proposer EpisodicShadowProposer,
) *EpisodicDistillationTrigger {
	t := NewEpisodicDistillationTrigger(repo, writer, proposer)
	requested := EpisodicShadowGenerationEnabledFromEnv(getenv)
	if requested && (proposer == nil || writer == nil) {
		log.Printf("episodic shadow generation requested but no approved proposer is wired; generation stays off")
	}
	t.enabled = requested && proposer != nil && writer != nil && repo != nil
	return t
}

func (t *EpisodicDistillationTrigger) WithEnabled(enabled bool) *EpisodicDistillationTrigger {
	if t != nil {
		t.enabled = enabled && t.repo != nil && t.writer != nil && t.proposer != nil
	}
	return t
}

func (t *EpisodicDistillationTrigger) Enabled() bool {
	return t != nil && t.enabled
}

// BeforePrune records every segment in the window, then attempts at most a
// bounded number of model proposals. A generation failure, timeout, invalid
// source or exhausted retry budget only changes the ledger and the watermark;
// the caller always continues to checkpoint cleanup and commit.
func (t *EpisodicDistillationTrigger) BeforePrune(
	ctx context.Context, owner uuid.UUID, plan *model.ChatRetentionPrunePlan,
) (stats EpisodicDistillationStats) {
	if !t.Enabled() || plan == nil {
		return stats
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("episodic distillation recovered from panic for conversation %s", plan.ConversationID)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, t.passBudget)
	defer cancel()

	state, err := t.repo.State(ctx, owner)
	if err != nil {
		log.Printf("episodic distillation state for conversation %s: %v", plan.ConversationID, err)
		return stats
	}
	if state.ConversationID != plan.ConversationID {
		return stats
	}
	if state.DistilledThroughSeq < state.PrunedThroughSeq {
		if _, err := t.repo.HealAfterPrune(ctx, owner, state.ConversationID); err != nil {
			log.Printf("episodic distillation heal for conversation %s: %v", plan.ConversationID, err)
			return stats
		}
		state.DistilledThroughSeq = state.PrunedThroughSeq
	}
	segments, err := t.repo.Segments(ctx, owner, state.ConversationID, state.DistilledThroughSeq, plan.TargetSeq)
	if err != nil {
		log.Printf("episodic distillation segments for conversation %s: %v", plan.ConversationID, err)
		return stats
	}
	if len(segments) == 0 {
		return stats
	}
	stats.Considered = len(segments)
	if err := t.repo.RecordPlan(ctx, owner, state.ConversationID, segments); err != nil {
		// Without a durable plan a crash could lose the gap record. Prune
		// still proceeds; HealAfterPrune writes the coarse fallback gap.
		log.Printf("episodic distillation plan for conversation %s: %v", plan.ConversationID, err)
		return stats
	}

	resolvedThrough := state.DistilledThroughSeq
	blocked := false
	modelCalls := 0
	for _, seg := range segments {
		switch {
		case seg.Status == repository.EpisodicDistillationDistilled:
			if !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		case seg.Status == repository.EpisodicDistillationSkipped:
			stats.Skipped++
			if !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		case seg.Status == repository.EpisodicDistillationGapped:
			if !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		case !seg.Eligible:
			// RecordPlan already persisted this skip; no model call.
			stats.Skipped++
			if !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		case seg.Attempts >= t.maxAttempts:
			if err := t.repo.RecordResolved(ctx, owner, state.ConversationID, seg.FromSeq, seg.ToSeq,
				repository.EpisodicDistillationGapped, "retry_limit", nil); err != nil {
				log.Printf("episodic distillation retry-limit for conversation %s: %v", plan.ConversationID, err)
				blocked = true
				continue
			}
			stats.Gaps++
			if !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		}
		if modelCalls >= t.maxSegments || ctx.Err() != nil {
			// The source is about to be deleted and this pass cannot ask the
			// model again. Record the bounded gap instead of dropping it.
			if err := t.repo.RecordResolved(ctx, owner, state.ConversationID, seg.FromSeq, seg.ToSeq,
				repository.EpisodicDistillationGapped, "pass_budget", nil); err != nil {
				log.Printf("episodic distillation pass-budget for conversation %s: %v", plan.ConversationID, err)
				blocked = true
				continue
			}
			stats.Gaps++
			if !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		}
		modelCalls++
		request := EpisodicShadowProposalRequest{
			Owner: owner, ConversationID: plan.ConversationID,
			FromSeq: seg.FromSeq, ToSeq: seg.ToSeq,
			Source:  make([]EpisodicShadowSourceMessage, 0, len(seg.Messages)),
			Context: make([]EpisodicShadowSourceMessage, 0, len(seg.Context)),
		}
		for _, msg := range seg.Messages {
			request.Source = append(request.Source, EpisodicShadowSourceMessage{
				ID: msg.ID, Seq: msg.Seq, Role: msg.Role, Text: msg.Text,
			})
		}
		for _, msg := range seg.Context {
			request.Context = append(request.Context, EpisodicShadowSourceMessage{
				ID: msg.ID, Seq: msg.Seq, Role: msg.Role, Text: msg.Text,
			})
		}
		callCtx, cancelCall := context.WithTimeout(ctx, t.proposalTimeout)
		note, created, generateErr := GenerateAndApplyEpisodicShadow(callCtx, t.writer, t.proposer, request)
		cancelCall()
		if generateErr != nil {
			stats.Failures++
			reason, terminal := episodicFailureReason(generateErr)
			if terminal {
				status := repository.EpisodicDistillationGapped
				if errors.Is(generateErr, repository.ErrEpisodicNoteUserDeleted) {
					status = repository.EpisodicDistillationSkipped
				}
				if err := t.repo.RecordResolved(ctx, owner, state.ConversationID, seg.FromSeq, seg.ToSeq,
					status, reason, nil); err != nil {
					log.Printf("episodic distillation terminal outcome for conversation %s: %v", plan.ConversationID, err)
					blocked = true
					continue
				}
				if status == repository.EpisodicDistillationSkipped {
					stats.Skipped++
				} else {
					stats.Gaps++
				}
				if !blocked {
					resolvedThrough = seg.ToSeq
				}
				continue
			}
			if t.recordFailure(ctx, owner, state.ConversationID, seg, reason, &stats, &blocked, plan) && !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		}
		if note == nil {
			// Abstention is a final, empty outcome: no note and no retry.
			if err := t.repo.RecordResolved(ctx, owner, state.ConversationID, seg.FromSeq, seg.ToSeq,
				repository.EpisodicDistillationDistilled, "abstained", nil); err != nil {
				log.Printf("episodic distillation abstention for conversation %s: %v", plan.ConversationID, err)
				blocked = true
				continue
			}
			stats.Abstained++
			if !blocked {
				resolvedThrough = seg.ToSeq
			}
			continue
		}
		noteID := &note.ID
		reason := "replayed"
		if created {
			reason = "persisted"
		}
		if err := t.repo.RecordResolved(ctx, owner, state.ConversationID, seg.FromSeq, seg.ToSeq,
			repository.EpisodicDistillationDistilled, reason, noteID); err != nil {
			log.Printf("episodic distillation persist for conversation %s: %v", plan.ConversationID, err)
			blocked = true
			continue
		}
		stats.Distilled++
		if !blocked {
			resolvedThrough = seg.ToSeq
		}
	}
	if resolvedThrough > state.DistilledThroughSeq {
		if err := t.repo.AdvanceWaterline(ctx, owner, state.ConversationID, resolvedThrough); err != nil {
			log.Printf("episodic distillation watermark for conversation %s: %v", plan.ConversationID, err)
		}
	}
	return stats
}

// recordFailure applies the bounded per-segment retry policy and reports
// whether the segment became resolved (gapped) in this attempt.
func (t *EpisodicDistillationTrigger) recordFailure(
	ctx context.Context, owner, conversationID uuid.UUID, seg repository.EpisodicDistillationSegment,
	reason string, stats *EpisodicDistillationStats, blocked *bool, plan *model.ChatRetentionPrunePlan,
) bool {
	status, _, err := t.repo.RecordFailure(ctx, owner, conversationID,
		seg.FromSeq, seg.ToSeq, reason, t.maxAttempts)
	if err != nil {
		log.Printf("episodic distillation failure record for conversation %s: %v", plan.ConversationID, err)
		*blocked = true
		return false
	}
	if status == repository.EpisodicDistillationGapped {
		stats.Gaps++
		return true
	}
	stats.Retries++
	*blocked = true
	return false
}

func episodicFailureReason(err error) (string, bool) {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "timeout", false
	case errors.Is(err, ErrEpisodicShadowProposalFailed):
		return "model_error", false
	case errors.Is(err, repository.ErrEpisodicNoteUserDeleted):
		return "user_deleted", true
	case errors.Is(err, repository.ErrEpisodicDraftConflict):
		return "conflict", true
	case errors.Is(err, repository.ErrEpisodicSourceInvalid):
		return "source_invalid", true
	default:
		return "model_error", false
	}
}

// AfterPrune closes the watermark after the raw rows are gone. It uses a
// detached context so a canceled job can still record the gap.
func (t *EpisodicDistillationTrigger) AfterPrune(
	ctx context.Context, owner uuid.UUID, plan *model.ChatRetentionPrunePlan,
) {
	if !t.Enabled() || plan == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	gaps, err := t.repo.HealAfterPrune(ctx, owner, plan.ConversationID)
	if err != nil {
		log.Printf("episodic distillation after-prune for conversation %s: %v", plan.ConversationID, err)
		return
	}
	if gaps > 0 {
		log.Printf("episodic distillation recorded %d pruned gap(s) for conversation %s", gaps, plan.ConversationID)
	}
}
