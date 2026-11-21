package repository

import (
	"context"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PruneJobKey is the idempotency key of a prune job: one per pruned cursor
// position and trigger, so a new trigger (say, another settled Run) can ask
// again while the cursor has not moved. Concurrent jobs are safe: the commit
// compare-and-sets the cursor and later passes replay its receipt.
func PruneJobKey(conversationID uuid.UUID, from int, trigger string) string {
	return fmt.Sprintf("%s:%s:%d:%s", model.ChatRetentionPruneJobType, conversationID, from, trigger)
}

// citingStatementEvidence is the statement evidence that still cites chat.
// An erased statement (AS-002 statement erasure, availability = erased) no
// longer holds its source message, its Ask User interaction or its Run, so
// the ordinary prune deletes them like any other raw chat.
const citingStatementEvidence = `(SELECT * FROM statement_evidence WHERE availability <> 'erased')`

// citedInteractionsSQL lists every Ask User interaction a durable record
// cites, with the citing owner: a structured-answer statement evidence (the
// BodyFact/Observation/Hypothesis funnel), a Safety intake screen, or a Safety
// reverification. Only these interactions and their resume receipts outlive a
// prune; every other interaction of a pruned window is raw chat and goes.
const citedInteractionsSQL = `
SELECT source_interaction_id AS interaction_id, user_id FROM ` + citingStatementEvidence + ` e WHERE source_interaction_id IS NOT NULL
UNION ALL
SELECT source_interaction_id, user_id FROM safety_screens
UNION ALL
SELECT check_interaction_id, user_id FROM safety_resolutions WHERE check_interaction_id IS NOT NULL`

// protectedMessages returns live messages in (from, through] that a durable
// record still cites. The full scope and its proof are documented in
// docs/security/chat-retention-evidence-scope.md: unerased BodyFact/Observation/
// Hypothesis/Safety statement evidence (which every Diagnosis, BodyState and
// Plan citation funnels through), a Diagnosis analysis or direct-edit request,
// a Safety screen or resolution, and both messages of a cited Ask User resume.
// A reference held by another owner is an anomaly and fails the pass.
func protectedMessages(ctx context.Context, tx *gorm.DB, conversation *model.Conversation, from, through int) (map[uuid.UUID]bool, error) {
	var refs []struct {
		MessageID uuid.UUID
		RefUserID uuid.UUID
	}
	err := tx.WithContext(ctx).Raw(`
WITH window_messages AS (
  SELECT id FROM messages WHERE conversation_id = ? AND seq > ? AND seq <= ?
), cited AS (`+citedInteractionsSQL+`
)
SELECT e.source_message_id AS message_id, e.user_id AS ref_user_id
  FROM `+citingStatementEvidence+` e JOIN window_messages w ON w.id = e.source_message_id
UNION ALL
SELECT c.request_message_id, c.user_id
  FROM condition_analysis_runs c JOIN window_messages w ON w.id = c.request_message_id
UNION ALL
SELECT c.request_message_id, c.user_id
  FROM condition_direct_edits c JOIN window_messages w ON w.id = c.request_message_id
UNION ALL
SELECT s.source_message_id, s.user_id
  FROM safety_screens s JOIN window_messages w ON w.id = s.source_message_id
UNION ALL
SELECT s.source_message_id, s.user_id
  FROM safety_resolutions s JOIN window_messages w ON w.id = s.source_message_id
UNION ALL
SELECT ir.user_message_id, cited.user_id
  FROM interaction_resumes ir JOIN window_messages w ON w.id = ir.user_message_id
  JOIN cited ON cited.interaction_id = ir.interaction_id
UNION ALL
SELECT ir.assistant_message_id, cited.user_id
  FROM interaction_resumes ir JOIN window_messages w ON w.id = ir.assistant_message_id
  JOIN cited ON cited.interaction_id = ir.interaction_id`,
		conversation.ID, from, through).Scan(&refs).Error
	if err != nil {
		return nil, err
	}
	protected := map[uuid.UUID]bool{}
	for _, ref := range refs {
		if ref.RefUserID != conversation.UserID {
			return nil, fmt.Errorf("%w: message referenced by another owner", model.ErrRetentionOwnerAnomaly)
		}
		protected[ref.MessageID] = true
	}
	return protected, nil
}

// referencedRuns returns runs whose identity durable evidence still names:
// unerased statement evidence, a Safety screen, or a cited Ask User interaction.
func referencedRuns(ctx context.Context, tx *gorm.DB, owner uuid.UUID, candidates []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(candidates) == 0 {
		return out, nil
	}
	var refs []struct {
		RunID     uuid.UUID
		RefUserID uuid.UUID
	}
	err := tx.WithContext(ctx).Raw(`
SELECT source_run_id AS run_id, user_id AS ref_user_id FROM `+citingStatementEvidence+` e WHERE source_run_id IN ?
UNION ALL
SELECT source_run_id, user_id FROM safety_screens WHERE source_run_id IN ?
UNION ALL
SELECT ai.run_id, cited.user_id
  FROM agent_interactions ai JOIN (`+citedInteractionsSQL+`) cited ON cited.interaction_id = ai.id
 WHERE ai.run_id IN ?`,
		candidates, candidates, candidates).Scan(&refs).Error
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if ref.RefUserID != owner {
			return nil, fmt.Errorf("%w: run referenced by another owner", model.ErrRetentionOwnerAnomaly)
		}
		out[ref.RunID] = true
	}
	return out, nil
}
