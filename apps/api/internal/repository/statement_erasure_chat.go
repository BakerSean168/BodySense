package repository

import (
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// sourceMessageCitationsSQL counts the durable records other than the
// statement being erased that still cite one chat message: the same scope
// chat retention protects (protectedMessages), with erased statements left
// out. It deliberately ignores owners, so a foreign reference keeps the row.
const sourceMessageCitationsSQL = `
WITH cited AS (
  SELECT source_interaction_id AS interaction_id FROM statement_evidence
   WHERE source_interaction_id IS NOT NULL AND availability <> 'erased' AND id <> @statement
  UNION ALL
  SELECT source_interaction_id FROM safety_screens
  UNION ALL
  SELECT check_interaction_id FROM safety_resolutions WHERE check_interaction_id IS NOT NULL
)
SELECT (SELECT count(*) FROM statement_evidence
         WHERE source_message_id = @message AND availability <> 'erased' AND id <> @statement)
     + (SELECT count(*) FROM condition_analysis_runs WHERE request_message_id = @message)
     + (SELECT count(*) FROM condition_direct_edits WHERE request_message_id = @message)
     + (SELECT count(*) FROM safety_screens WHERE source_message_id = @message)
     + (SELECT count(*) FROM safety_resolutions WHERE source_message_id = @message)
     + (SELECT count(*) FROM interaction_resumes ir JOIN cited ON cited.interaction_id = ir.interaction_id
         WHERE ir.user_message_id = @message OR ir.assistant_message_id = @message)`

// eraseStatementSourceMessage deletes the chat message the statement was
// taken from when nothing else cites it. Under the Conversation lock (the
// one chat retention and message admission take) it refuses a message whose
// Run has not settled or that is the newest message (seq is allocated as
// MAX+1), keeps a message another durable record cites as a conversation
// record (retained_by_policy, chat_record_cited), and otherwise deletes the
// uncited Ask User receipts naming it and the message itself; its projection
// copy cascades. The runtime copy of the turn is chat history like any other
// turn and leaves with the ordinary prune.
func eraseStatementSourceMessage(tx *gorm.DB, userID, statementID uuid.UUID) (model.AssetErasureOutcome, error) {
	verified := model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}
	var source struct {
		ConversationID *uuid.UUID
		MessageID      *uuid.UUID
	}
	if err := tx.Raw(`SELECT source_conversation_id AS conversation_id, source_message_id AS message_id
FROM statement_evidence WHERE user_id = ? AND id = ?`, userID, statementID).Scan(&source).Error; err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if source.MessageID == nil || source.ConversationID == nil {
		return verified, nil // a direct edit has no chat source
	}
	var locked []uuid.UUID
	if err := tx.Raw(`SELECT id FROM conversations WHERE id = ? AND user_id = ? FOR NO KEY UPDATE`,
		*source.ConversationID, userID).Scan(&locked).Error; err != nil {
		return model.AssetErasureOutcome{}, err
	}
	var message []struct {
		Seq     int
		Status  string
		Settled bool
		Newest  bool
	}
	if err := tx.Raw(`SELECT m.seq, m.status,
       (m.run_id IS NULL OR r.status IN ('completed','failed','cancelled')) AND NOT EXISTS (
         SELECT 1 FROM agent_interactions ai WHERE ai.run_id = m.run_id AND ai.status = 'pending') AS settled,
       m.seq = (SELECT max(seq) FROM messages WHERE conversation_id = m.conversation_id) AS newest
FROM messages m LEFT JOIN runs r ON r.id = m.run_id
WHERE m.id = ? AND m.conversation_id = ?`, *source.MessageID, *source.ConversationID).Scan(&message).Error; err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if len(locked) == 0 || len(message) == 0 {
		return verified, nil // already pruned, or the Conversation is gone
	}
	if m := message[0]; m.Status == "streaming" || !m.Settled || m.Newest {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: 1, FailureCode: model.ErasureCodeSourceMessageBusy}, nil
	}
	args := map[string]any{"message": *source.MessageID, "statement": statementID}
	var cited int
	if err := tx.Raw(sourceMessageCitationsSQL, args).Scan(&cited).Error; err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if cited > 0 {
		return model.AssetErasureOutcome{State: model.ErasureTargetRetainedByPolicy, Outstanding: 1, RetentionReason: model.ErasureRetentionChatRecordCited}, nil
	}
	// Receipts name messages without a cascade; any left here is cited by
	// nothing but this statement (a cited one would have kept the message).
	if err := tx.Exec(`DELETE FROM interaction_resumes WHERE conversation_id = ? AND (user_message_id = ? OR assistant_message_id = ?)`,
		*source.ConversationID, *source.MessageID, *source.MessageID).Error; err != nil {
		return model.AssetErasureOutcome{}, err
	}
	deleted := tx.Exec(`DELETE FROM messages WHERE id = ? AND conversation_id = ?`, *source.MessageID, *source.ConversationID)
	if deleted.Error != nil {
		return model.AssetErasureOutcome{}, deleted.Error
	}
	var left int64
	if err := tx.Raw(`SELECT count(*) FROM messages WHERE id = ?`, *source.MessageID).Scan(&left).Error; err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if left > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: int(left), FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	verified.Erased = int(deleted.RowsAffected)
	return verified, nil
}

// episodicNotesForStatement counts narrative copies citing any chat source of
// this statement. Only the owner and the exact source-message UUID can match;
// a note that cites several statements is deleted whole when *one* is erased.
// The statement row is already locked by lockStatementPins at each call.
func episodicNotesForStatement(tx *gorm.DB, userID, statementID uuid.UUID) (int, *uuid.UUID, error) {
	var source struct{ MessageID *uuid.UUID }
	if err := tx.Raw(`SELECT source_message_id AS message_id FROM statement_evidence
WHERE user_id = ? AND id = ?`, userID, statementID).Scan(&source).Error; err != nil {
		return 0, nil, err
	}
	if source.MessageID == nil {
		return 0, nil, nil
	}
	count, err := countRows(tx, `SELECT count(*) FROM episodic_notes
WHERE user_id = ? AND source_message_ids @> ARRAY[?::uuid]`, userID, *source.MessageID)
	return count, source.MessageID, err
}

// eraseStatementEpisodicNotes is a separately receipted controlled-copy
// target. PostgreSQL's source gate takes the statement FOR SHARE on note
// INSERT/UPDATE; the worker holds its FOR UPDATE lock during deletion.
func eraseStatementEpisodicNotes(tx *gorm.DB, userID, statementID uuid.UUID) (model.AssetErasureOutcome, error) {
	_, source, err := episodicNotesForStatement(tx, userID, statementID)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if source == nil {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, nil
	}
	deleted := tx.Exec(`DELETE FROM episodic_notes WHERE user_id = ?
AND source_message_ids @> ARRAY[?::uuid]`, userID, *source)
	if deleted.Error != nil {
		return model.AssetErasureOutcome{}, deleted.Error
	}
	return verifyCount(tx, int(deleted.RowsAffected),
		`SELECT count(*) FROM episodic_notes WHERE user_id = ?
AND source_message_ids @> ARRAY[?::uuid]`, userID, *source)
}
