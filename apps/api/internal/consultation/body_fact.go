package consultation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type runtimeBodyFactService interface {
	RecordRunFacts(context.Context, uuid.UUID, uuid.UUID, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
	ListFacts(context.Context, uuid.UUID, int64) (*model.BodyFactPage, error)
}

// AttachBodyFactService makes the ledger available without altering the stable
// legacy constructor or silently changing historical v2 run behavior.
func (r *Runtime) AttachBodyFactService(ledger runtimeBodyFactService) {
	r.bodyFacts = ledger
}

// bodyFactProposal is the finite application contract carried by the existing
// private tool-result envelope. The producer cannot submit user/message IDs,
// idempotency keys, or a success-shaped receipt.
type bodyFactProposal struct {
	Status           string                `json:"status"`
	ExpectedRevision *int64                `json:"expected_revision"`
	Facts            []model.BodyFactInput `json:"facts"`
}

func (r *Runtime) commitBodyFactProposal(ctx context.Context, state streamState, toolCallID string, raw json.RawMessage, ownedSafetyRevisions ...model.BodyStateRevision) (json.RawMessage, []service.CommittedBodyFact, error) {
	if r.bodyFacts == nil || state.Run == nil || state.UID == uuid.Nil || state.Run.ID == uuid.Nil || state.Run.UserID != state.UID || state.Run.ConversationID != state.ConversationID {
		return nil, nil, fmt.Errorf("BodyFact persistence or authenticated run is unavailable")
	}
	policy, err := service.ConsultationDecisionPolicyRevisionForConfiguration(state.ExpectedConfigurationID)
	if err != nil || policy != service.ConsultationDecisionPolicyV3 {
		return nil, nil, fmt.Errorf("BodyFact proposal requires the registered v3 runtime")
	}
	if strings.TrimSpace(toolCallID) == "" || len(toolCallID) > 256 || len(raw) > 65536 {
		return nil, nil, fmt.Errorf("invalid BodyFact tool identity or payload size")
	}
	var proposal bodyFactProposal
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proposal); err != nil {
		return nil, nil, fmt.Errorf("invalid BodyFact proposal contract")
	}
	if decoder.Decode(new(any)) != io.EOF || proposal.Status != "proposed" || proposal.ExpectedRevision == nil {
		return nil, nil, fmt.Errorf("only a proposed statement batch with an expected revision is accepted")
	}
	// Stable producer call + server Run define retry identity. No source identity
	// or retry token can be invented/rebound by the model.
	callKey := sha256.Sum256([]byte(toolCallID))
	command := model.RecordBodyFactsCommand{
		IdempotencyKey:   fmt.Sprintf("chat-facts:%s:%x", state.Run.ID, callKey[:12]),
		ExpectedRevision: *proposal.ExpectedRevision,
		Facts:            proposal.Facts,
	}
	// A safety check in this very stream may have advanced the aggregate before
	// its extraction proposal arrives. Follow only actual contiguous Go receipts.
	// Do not fetch "latest" and rebase over an unrelated user/device change;
	// direct-user writes and amendments keep their original revision checks.
	for _, revision := range ownedSafetyRevisions {
		if revision.UserID == state.UID && revision.ID != uuid.Nil &&
			revision.ChangeType == "safety.updated" && revision.Source == "consultation" &&
			revision.Revision > 0 && command.ExpectedRevision == revision.Revision-1 {
			command.ExpectedRevision = revision.Revision
		}
	}
	if err := command.Validate(); err != nil {
		return nil, nil, err
	}
	var receipt *model.BodyFactCommit
	if state.AnswerSource != nil {
		source := *state.AnswerSource
		if source.RunID != state.Run.ID {
			return nil, nil, fmt.Errorf("answer does not belong to the active Run")
		}
		ledger, ok := r.bodyFacts.(interface {
			RecordAnswerFacts(context.Context, uuid.UUID, model.AnswerFactSource, model.RecordBodyFactsCommand) (*model.BodyFactCommit, error)
		})
		if !ok {
			return nil, nil, fmt.Errorf("accepted-answer persistence unavailable")
		}
		receipt, err = ledger.RecordAnswerFacts(ctx, state.UID, source, command)
	} else {
		receipt, err = r.bodyFacts.RecordRunFacts(ctx, state.UID, state.Run.ID, command)
	}
	if err != nil {
		return nil, nil, err
	}
	if receipt == nil || receipt.UserID != state.UID || receipt.ID == uuid.Nil || receipt.EventID == uuid.Nil || receipt.RevisionID == uuid.Nil || receipt.Revision <= 0 {
		return nil, nil, fmt.Errorf("BodyFact commit receipt is missing or does not match its owner")
	}
	var ids []uuid.UUID
	if err := json.Unmarshal(receipt.FactIDs, &ids); err != nil || len(ids) != len(command.Facts) {
		return nil, nil, fmt.Errorf("BodyFact commit receipt does not account for the complete batch")
	}
	// Committed facts, addressed by producer call + index, are what a later
	// symptom intake question in this stream may bind to.
	committed := make([]service.CommittedBodyFact, 0, len(ids))
	for index, input := range command.Facts {
		var details struct {
			Presence string `json:"presence"`
		}
		_ = json.Unmarshal(input.Details, &details)
		committed = append(committed, service.CommittedBodyFact{ToolCallID: toolCallID, Index: index, FactID: ids[index], Kind: input.Kind, Presence: details.Presence})
	}
	encoded, err := json.Marshal(struct {
		Status  string                `json:"status"`
		Receipt *model.BodyFactCommit `json:"receipt"`
	}{Status: "persisted", Receipt: receipt})
	if err != nil {
		return nil, nil, err
	}
	return encoded, committed, nil
}

// The new version has exactly one fact-write boundary. A stale/malformed
// producer must not smuggle old extraction events into the legacy tables.
func validateBodyFactRuntimeEvent(configurationID string, event service.ConsultationRuntimeEvent) error {
	policy, err := service.ConsultationDecisionPolicyRevisionForConfiguration(configurationID)
	if err != nil || policy != service.ConsultationDecisionPolicyV3 {
		return nil
	}
	switch event.Payload.(type) {
	case service.ConsultationRuntimeExtractedInfoPayload, service.ConsultationRuntimeLifestyleContextPayload:
		return fmt.Errorf("legacy fact mutation event is not permitted by the v3 BodyFact contract")
	default:
		return nil
	}
}
