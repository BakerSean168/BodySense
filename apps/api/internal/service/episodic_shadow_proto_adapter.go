package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	protovalidate "buf.build/go/protovalidate"
	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
)

// EpisodicShadowQuotePolicyRevision is an internal proposal contract, NOT
// a deployment flag or a signal to generate/serve private chat history.
const EpisodicShadowQuotePolicyRevision = "episodic-shadow-quote-v1"

// ErrEpisodicShadowProposalFailed marks a failure from the injected model
// seam. The trigger treats it as retryable; admission errors remain distinct.
var ErrEpisodicShadowProposalFailed = errors.New("episodic shadow proposal failed")

// EpisodicShadowSourceMessage is the boundary-neutral source/context view.
// Generated runtime Proto never leaves this adapter file.
type EpisodicShadowSourceMessage struct {
	ID   uuid.UUID
	Seq  int
	Role string
	Text string
}

// EpisodicShadowProposalRequest is what a model implementation sees. It is
// owner-scoped, content-bounded and cannot name a note, storage command or
// clinical authority.
type EpisodicShadowProposalRequest struct {
	Owner          uuid.UUID
	ConversationID uuid.UUID
	FromSeq        int
	ToSeq          int
	Source         []EpisodicShadowSourceMessage
	Context        []EpisodicShadowSourceMessage
}

// EpisodicShadowQuoteSuggestion is the model's only output: an exact quote
// candidate for Go to verify against its own locked source row.
type EpisodicShadowQuoteSuggestion struct {
	SourceMessageID uuid.UUID
	Quote           string
	Preference      string
}

type EpisodicShadowProposalResponse struct {
	Quotes []EpisodicShadowQuoteSuggestion
}

// EpisodicShadowProposer is the only model seam S2 permits. Production has no
// implementation; tests inject a fake and never contact a provider.
type EpisodicShadowProposer interface {
	ProposeEpisodicShadowQuote(
		context.Context, EpisodicShadowProposalRequest,
	) (EpisodicShadowProposalResponse, error)
}

// GenerateAndApplyEpisodicShadow is the sole place where the trigger touches
// the typed contract. It builds the existing S2-B1 command from the plain
// request, maps the plain response to the typed proposal, and reuses the
// Go-owned validation/persistence path.
func GenerateAndApplyEpisodicShadow(
	ctx context.Context,
	writer *repository.EpisodicShadowRepository,
	proposer EpisodicShadowProposer,
	request EpisodicShadowProposalRequest,
) (*model.EpisodicNote, bool, error) {
	if writer == nil || proposer == nil {
		return nil, false, fmt.Errorf("%w: missing writer or proposer", repository.ErrEpisodicSourceInvalid)
	}
	command := &runtimev1.EpisodicShadowGenerationCommand{
		ConversationId:  request.ConversationID.String(),
		UserId:          request.Owner.String(),
		FromSeq:         uint32(request.FromSeq),
		ToSeq:           uint32(request.ToSeq),
		PolicyRevision:  EpisodicShadowQuotePolicyRevision,
		SourceMessages:  make([]*runtimev1.EpisodicShadowSourceMessage, 0, len(request.Source)),
		ContextMessages: make([]*runtimev1.EpisodicShadowSourceMessage, 0, len(request.Context)),
	}
	for _, msg := range request.Source {
		command.SourceMessages = append(command.SourceMessages, &runtimev1.EpisodicShadowSourceMessage{
			Id: msg.ID.String(), Seq: uint32(msg.Seq), Role: msg.Role, Text: msg.Text,
		})
	}
	for _, msg := range request.Context {
		command.ContextMessages = append(command.ContextMessages, &runtimev1.EpisodicShadowSourceMessage{
			Id: msg.ID.String(), Seq: uint32(msg.Seq), Role: msg.Role, Text: msg.Text,
		})
	}
	response, err := proposer.ProposeEpisodicShadowQuote(ctx, request)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %w", ErrEpisodicShadowProposalFailed, err)
	}
	proposal := &runtimev1.EpisodicShadowDraftProposal{PolicyRevision: EpisodicShadowQuotePolicyRevision}
	for _, quote := range response.Quotes {
		proposal.Quotes = append(proposal.Quotes, &runtimev1.EpisodicShadowQuote{
			SourceMessageId: quote.SourceMessageID.String(),
			Quote:           quote.Quote,
			Preference:      quote.Preference,
		})
	}
	return ApplyEpisodicShadowProposal(ctx, request.Owner, writer, command, proposal)
}

// ApplyEpisodicShadowProposal only admits a quote whose exact, entire user
// source message Go rechecks inside the repository's locked PG transaction.
// It never accepts a model-authored free-text note. No application service,
// worker, route or AI client calls this S2-B method before policy approval.
func ApplyEpisodicShadowProposal(
	ctx context.Context,
	owner uuid.UUID,
	repo *repository.EpisodicShadowRepository,
	command *runtimev1.EpisodicShadowGenerationCommand,
	proposal *runtimev1.EpisodicShadowDraftProposal,
) (*model.EpisodicNote, bool, error) {
	fail := func(detail string) (*model.EpisodicNote, bool, error) {
		return nil, false, fmt.Errorf("%w: %s", repository.ErrEpisodicSourceInvalid, detail)
	}
	if repo == nil || owner == uuid.Nil || command == nil || proposal == nil {
		return fail("missing server owner, request or store")
	}
	if err := protovalidate.Validate(command); err != nil {
		return fail("invalid typed command")
	}
	if err := protovalidate.Validate(proposal); err != nil {
		return fail("invalid typed proposal")
	}
	if command.GetUserId() != owner.String() ||
		command.GetPolicyRevision() != EpisodicShadowQuotePolicyRevision ||
		proposal.GetPolicyRevision() != EpisodicShadowQuotePolicyRevision {
		return fail("owner or policy does not match")
	}
	convID, err := uuid.Parse(command.GetConversationId())
	if err != nil {
		return fail("invalid conversation id")
	}
	sources := command.GetSourceMessages()
	fromSeq, toSeq := int(command.GetFromSeq()), int(command.GetToSeq())
	if fromSeq < 1 || toSeq < fromSeq || len(sources) != toSeq-fromSeq+1 {
		return fail("source message sequence is not contiguous")
	}
	// Untrusted lookback is bounded, immediately preceding the source and
	// never part of provenance or a quotable source. Go selected it from the
	// same owner-scoped transcript; SaveShadowDraft never sees it.
	context := command.GetContextMessages()
	if len(context) > 2 {
		return fail("context window exceeds two messages")
	}
	if len(context) > 0 && int(context[len(context)-1].GetSeq()) != fromSeq-1 {
		return fail("context must immediately precede the source")
	}
	contextIDs := make(map[uuid.UUID]bool, len(context))
	for i, src := range context {
		id, err := uuid.Parse(src.GetId())
		seq := int(src.GetSeq())
		if err != nil || id == uuid.Nil || contextIDs[id] ||
			seq < 1 || seq >= fromSeq ||
			(i > 0 && seq != int(context[i-1].GetSeq())+1) ||
			(src.GetRole() != "user" && src.GetRole() != "assistant") ||
			src.GetText() == "" || src.GetText() != strings.TrimSpace(src.GetText()) {
			return fail("invalid context message")
		}
		contextIDs[id] = true
	}
	var sourceBytes int
	ids := make([]uuid.UUID, 0, len(sources))
	seen := make(map[uuid.UUID]bool, len(sources))
	for i, src := range sources {
		id, err := uuid.Parse(src.GetId())
		if err != nil || id == uuid.Nil || seen[id] || int(src.GetSeq()) != fromSeq+i {
			return fail("source message identity or order is inconsistent")
		}
		if contextIDs[id] {
			return fail("context message reused as source")
		}
		seen[id] = true
		sourceBytes += utf8.RuneCountInString(src.GetText())
		ids = append(ids, id)
	}
	if sourceBytes > 8192 {
		return fail("source text exceeds private proposal budget")
	}
	// Abstention is not failure, and never creates an empty or fabricated
	// note. A separate kill switch still protects model generation.
	if len(proposal.GetQuotes()) == 0 {
		return nil, false, nil
	}
	claim := proposal.GetQuotes()[0]
	quoteID, err := uuid.Parse(claim.GetSourceMessageId())
	if err != nil || !seen[quoteID] {
		return fail("unrecognized quote source")
	}
	canonical, ok := repository.CanonicalEpisodicPreference(claim.GetQuote())
	if !ok || canonical.Kind != claim.GetPreference() {
		return fail("quote is outside approved non-clinical vocabulary")
	}
	found := false
	for _, src := range sources {
		if src.GetId() == quoteID.String() && src.GetRole() == "user" &&
			src.GetText() == claim.GetQuote() {
			found = true
			break
		}
	}
	if !found {
		return fail("model quote is not an entire Go-selected user message")
	}
	// Go, not Python, synthesizes both owner metadata and the exact stored
	// string. The repository then independently re-reads all source rows
	// under a Conversation lock and rejects stale/erased/foreign evidence.
	return repo.SaveShadowDraft(ctx, owner, repository.EpisodicShadowDraft{
		ConversationID: convID, FromSeq: fromSeq, ToSeq: toSeq,
		SourceMessageIDs: ids, SourceAssetIDs: nil,
		GeneratorVersion:       "episodic-shadow-quote-proposal-v1",
		RetentionPolicyVersion: EpisodicShadowQuotePolicyRevision,
		ContentText:            canonical.Text,
		QuotedMessageID:        quoteID, QuotedMessageText: claim.GetQuote(),
	})
}
