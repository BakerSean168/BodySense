package service

import (
	"context"
	"encoding/json"
	"fmt"

	protovalidate "buf.build/go/protovalidate"
	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

// episodicShadowProposalPath is the internal ai-service transport for the
// S2-B proposal seam. It sits beside /runtime/threads, is reachable only on
// the internal service network, and is called only while the default-off
// EPISODIC_SHADOW_GENERATION_ENABLED switch is on.
const episodicShadowProposalPath = "/runtime/episodic-shadow/proposals"

// EpisodicShadowHTTPProposer is the production EpisodicShadowProposer. It
// reuses the AI runtime client's base URL, timeout and request headers, and
// only ever transports the Go-built typed command. It cannot create a note:
// GenerateAndApplyEpisodicShadow still verifies and persists the proposal.
type EpisodicShadowHTTPProposer struct {
	client *AIClient
}

func NewEpisodicShadowHTTPProposer(client *AIClient) *EpisodicShadowHTTPProposer {
	return &EpisodicShadowHTTPProposer{client: client}
}

// ProposeEpisodicShadowQuote sends the typed command and parses the typed
// proposal. Transport failure, a non-200 upstream status, an unexpected wire
// shape or a failed typed validation is reported as an
// ErrEpisodicShadowProposalFailed so the trigger can retry. It never invents
// a quote: an empty proposal stays an explicit abstention.
func (p *EpisodicShadowHTTPProposer) ProposeEpisodicShadowQuote(
	ctx context.Context,
	request EpisodicShadowProposalRequest,
) (EpisodicShadowProposalResponse, error) {
	if p == nil || p.client == nil {
		return EpisodicShadowProposalResponse{}, episodicProposalFailure(fmt.Errorf("missing AI runtime client"))
	}
	body, err := validateAndMarshalRuntimeCommand(episodicShadowGenerationCommand(request))
	if err != nil {
		return EpisodicShadowProposalResponse{}, episodicProposalFailure(err)
	}
	raw, err := p.client.callJSON(ctx, episodicShadowProposalPath, json.RawMessage(body))
	if err != nil {
		return EpisodicShadowProposalResponse{}, episodicProposalFailure(err)
	}
	proposal := &runtimev1.EpisodicShadowDraftProposal{}
	if err := protojson.Unmarshal(raw, proposal); err != nil {
		return EpisodicShadowProposalResponse{}, episodicProposalFailure(
			fmt.Errorf("invalid shadow proposal response: %w", err))
	}
	if err := protovalidate.Validate(proposal); err != nil {
		return EpisodicShadowProposalResponse{}, episodicProposalFailure(
			fmt.Errorf("invalid typed shadow proposal: %w", err))
	}
	response := EpisodicShadowProposalResponse{
		Quotes: make([]EpisodicShadowQuoteSuggestion, 0, len(proposal.GetQuotes())),
	}
	for _, quote := range proposal.GetQuotes() {
		id, err := uuid.Parse(quote.GetSourceMessageId())
		if err != nil {
			return EpisodicShadowProposalResponse{}, episodicProposalFailure(
				fmt.Errorf("invalid shadow proposal source id: %w", err))
		}
		response.Quotes = append(response.Quotes, EpisodicShadowQuoteSuggestion{
			SourceMessageID: id,
			Quote:           quote.GetQuote(),
			Preference:      quote.GetPreference(),
		})
	}
	return response, nil
}

func episodicProposalFailure(err error) error {
	return fmt.Errorf("%w: %w", ErrEpisodicShadowProposalFailed, err)
}
