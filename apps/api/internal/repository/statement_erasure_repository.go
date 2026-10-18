package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// statementDecisionTables hold the immutable Diagnosis and plan decision
// trail. Their frozen replay inputs and audit content may embed a statement
// or a record derived from it; they are never rewritten (decision_records is
// retained_by_policy) and account erasure removes them.
var statementDecisionTables = []string{
	"condition_analysis_runs", "condition_analysis_events", "diagnosis_proposals",
	"diagnosis_decisions", "diagnosis_settlements", "diagnosis_revisions",
	"condition_entries", "condition_entry_evidence", "condition_relations",
	"condition_disposition_changes", "condition_direct_edits",
	"plan_proposals", "training_plan_revisions",
}

// StatementErasureRepository owns the per-statement erasure request, its
// target receipts and the database-resident copies of one user statement
// (statement_evidence) and the BodyState records derived from it. Runtime
// checkpoints are executed by the service through the ai-service port; the
// source chat message is deleted here when nothing else cites it.
type StatementErasureRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewStatementErasureRepository(db *gorm.DB) *StatementErasureRepository {
	return &StatementErasureRepository{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (r *StatementErasureRepository) conn(ctx context.Context) *gorm.DB {
	return database.FromContext(ctx, r.db)
}

// CreateOrGet admits one erasure per owned statement. A repeated command for
// the same statement returns the existing request whatever its key; a key
// already used for another statement is rejected. Missing and foreign
// statements are absent.
func (r *StatementErasureRepository) CreateOrGet(ctx context.Context, userID, statementID uuid.UUID, idempotencyKey string) (*model.StatementErasureReceipt, error) {
	var requestID uuid.UUID
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var statement model.StatementEvidence
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, statementID).Take(&statement).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrStatementErasureNotFound
			}
			return err
		}
		var byKey model.StatementErasureRequest
		err := tx.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).Take(&byKey).Error
		if err == nil && byKey.StatementID != statementID {
			return model.ErrStatementErasureIdempotencyUsed
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var existing model.StatementErasureRequest
		err = tx.Where("user_id = ? AND statement_id = ?", userID, statementID).Take(&existing).Error
		if err == nil {
			requestID = existing.ID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := r.now()
		request := model.StatementErasureRequest{
			ID: uuid.New(), UserID: userID, StatementID: statementID, IdempotencyKey: idempotencyKey,
			InventoryRevision: model.StatementErasureInventoryRevision, Status: model.StatementErasurePending,
			RequestedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		targets := make([]model.StatementErasureTarget, 0, len(model.StatementErasureTargets))
		for _, name := range model.StatementErasureTargets {
			targets = append(targets, model.StatementErasureTarget{
				UserID: userID, RequestID: request.ID, Target: name,
				State: model.AssetErasureTargetPending, UpdatedAt: now,
			})
		}
		if err := tx.Create(&targets).Error; err != nil {
			return err
		}
		requestID = request.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.Receipt(ctx, userID, requestID)
}

// GetByStatement returns the owner's receipt for one statement.
func (r *StatementErasureRepository) GetByStatement(ctx context.Context, userID, statementID uuid.UUID) (*model.StatementErasureReceipt, error) {
	var request model.StatementErasureRequest
	if err := r.conn(ctx).Where("user_id = ? AND statement_id = ?", userID, statementID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrStatementErasureNotFound
		}
		return nil, err
	}
	return r.Receipt(ctx, userID, request.ID)
}

// Receipt loads one request with its targets in inventory order and the
// live review signal: still-asserted derived records, the adopted conditions
// citing the statement and the plans resting on them.
func (r *StatementErasureRepository) Receipt(ctx context.Context, userID, requestID uuid.UUID) (*model.StatementErasureReceipt, error) {
	db := r.conn(ctx)
	var request model.StatementErasureRequest
	if err := db.Where("user_id = ? AND id = ?", userID, requestID).Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrStatementErasureNotFound
		}
		return nil, err
	}
	var rows []model.StatementErasureTarget
	if err := db.Where("user_id = ? AND request_id = ?", userID, requestID).Find(&rows).Error; err != nil {
		return nil, err
	}
	byName := make(map[model.StatementErasureTargetName]model.StatementErasureTarget, len(rows))
	for _, row := range rows {
		byName[row.Target] = row
	}
	receipt := &model.StatementErasureReceipt{Request: request, Targets: make([]model.StatementErasureTarget, 0, len(rows))}
	for _, name := range model.StatementErasureTargets {
		if row, ok := byName[name]; ok {
			receipt.Targets = append(receipt.Targets, row)
		}
	}
	if request.Status.Terminal() {
		receipt.Reviews = []model.ErasureReview{}
		return receipt, nil
	}
	derived, err := statementDerivedRecords(db, userID, request.StatementID, false)
	if err != nil {
		return nil, err
	}
	reviews := derived.activeReviews()
	conditions, err := statementPinningConditions(db, userID, request.StatementID, derived)
	if err != nil {
		return nil, err
	}
	dependents, err := dependentReviews(db, userID, conditions, derived.bodyRefs())
	if err != nil {
		return nil, err
	}
	receipt.Reviews = append(reviews, dependents...)
	return receipt, nil
}

// Claim leases a request for one execution pass. Terminal requests and live
// leases are never claimed, so two workers never execute the same request.
func (r *StatementErasureRepository) Claim(ctx context.Context, requestID uuid.UUID, owner string, lease time.Duration) (*model.StatementErasureRequest, bool, error) {
	now := r.now()
	var request model.StatementErasureRequest
	result := r.conn(ctx).Raw(`
UPDATE statement_erasure_requests
SET status = 'running', attempt_count = attempt_count + 1,
    lease_owner = ?, lease_expires_at = ?, updated_at = ?
WHERE id = ?
  AND (status IN ('pending','outstanding') OR (status = 'running' AND lease_expires_at < ?))
RETURNING *`, owner, now.Add(lease), now, requestID, now).Scan(&request)
	if result.Error != nil {
		return nil, false, fmt.Errorf("claim statement erasure: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &request, true, nil
}

// Release ends a pass that did not reach a terminal state.
func (r *StatementErasureRepository) Release(ctx context.Context, request *model.StatementErasureRequest) error {
	return r.conn(ctx).Exec(`UPDATE statement_erasure_requests
SET status = 'outstanding', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE user_id = ? AND id = ? AND status = 'running' AND lease_owner = ?`,
		r.now(), request.UserID, request.ID, *request.LeaseOwner).Error
}

// RecordOutcome stores one target's result.
func (r *StatementErasureRepository) RecordOutcome(ctx context.Context, request *model.StatementErasureRequest, target model.StatementErasureTargetName, outcome model.AssetErasureOutcome) error {
	return recordStatementErasureOutcome(r.conn(ctx), r.now(), request.UserID, request.ID, target, outcome)
}

func recordStatementErasureOutcome(tx *gorm.DB, now time.Time, userID, requestID uuid.UUID, target model.StatementErasureTargetName, outcome model.AssetErasureOutcome) error {
	var code, reason, verifiedAt any
	outstanding := outcome.Outstanding
	switch outcome.State {
	case model.AssetErasureTargetVerified:
		verifiedAt, outstanding = now, 0
	case model.AssetErasureTargetPending:
	case model.ErasureTargetRetainedByPolicy:
		reason = outcome.RetentionReason
	default:
		code = outcome.FailureCode
	}
	result := tx.Exec(`UPDATE statement_erasure_targets
SET state = ?, erased_count = erased_count + ?, outstanding_count = ?, failure_code = ?, retention_reason = ?,
    attempt_count = attempt_count + 1, verified_at = ?, updated_at = ?
WHERE user_id = ? AND request_id = ? AND target = ?`,
		outcome.State, outcome.Erased, outstanding, code, reason, verifiedAt, now, userID, requestID, target)
	if result.Error != nil {
		return fmt.Errorf("record statement erasure target %s: %w", target, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("record statement erasure target %s: target row missing", target)
	}
	return nil
}

// ExecuteDatabaseTarget erases (or, for families that need a review first or
// are kept by policy, inventories) one database-resident target and verifies
// it inside the same transaction, under the statement row lock.
func (r *StatementErasureRepository) ExecuteDatabaseTarget(ctx context.Context, request *model.StatementErasureRequest, target model.StatementErasureTargetName) (model.AssetErasureOutcome, error) {
	var outcome model.AssetErasureOutcome
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		if target == model.StatementErasureSourceMessage {
			// Chat rows are guarded by the Conversation lock, not the pins.
			var err error
			outcome, err = eraseStatementSourceMessage(tx, request.UserID, request.StatementID)
			return err
		}
		pins, err := lockStatementPins(tx, request.UserID, request.StatementID)
		if err != nil {
			return err
		}
		switch target {
		case model.StatementErasureDiagnosisRecords:
			outcome = blockedWhile(len(pins.conditions), model.AssetErasureCodeDiagnosisReview)
		case model.StatementErasureTrainingPlans:
			outcome = blockedWhile(len(pins.plans), model.ErasureCodePlanReview)
		case model.StatementErasureBodyRecords:
			outcome, err = eraseStatementBodyRecords(tx, request.UserID, request.StatementID, pins)
		case model.StatementErasureAssessmentReports:
			outcome, err = eraseStatementAssessmentReports(tx, request.UserID, pins)
		case model.StatementErasureDecisionRecords:
			outcome, err = inventoryStatementDecisions(tx, request.UserID, pins)
		default:
			err = fmt.Errorf("target %s is not a database target", target)
		}
		return err
	})
	return outcome, err
}

func blockedWhile(n int, code string) model.AssetErasureOutcome {
	if n == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: n, FailureCode: code}
}

// statementDerived lists the BodyState records derived from one statement.
type statementDerived struct {
	Facts, Observations, Hypotheses []statementDerivedRecord
}

type statementDerivedRecord struct {
	ID              uuid.UUID
	AssertionStatus string
	Erased          bool
}

func (d statementDerived) all() [][]statementDerivedRecord {
	return [][]statementDerivedRecord{d.Facts, d.Observations, d.Hypotheses}
}

func derivedIDs(records []statementDerivedRecord) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func (d statementDerived) bodyRefs() erasureBodyRefs {
	return erasureBodyRefs{Facts: derivedIDs(d.Facts), Observations: derivedIDs(d.Observations)}
}

// ids is every derived record id (runtime and replay needles).
func (d statementDerived) ids() []uuid.UUID {
	out := []uuid.UUID{}
	for _, records := range d.all() {
		out = append(out, derivedIDs(records)...)
	}
	return out
}

// needles is the statement id and every derived record id as text.
func (d statementDerived) needles(statementID uuid.UUID) []string {
	out := []string{statementID.String()}
	for _, id := range d.ids() {
		out = append(out, id.String())
	}
	return out
}

func (d statementDerived) active() int {
	n := 0
	for _, records := range d.all() {
		for _, record := range records {
			if record.AssertionStatus == "asserted" {
				n++
			}
		}
	}
	return n
}

// unerased counts derived records whose text is still present.
func (d statementDerived) unerased() int {
	n := 0
	for _, records := range d.all() {
		for _, record := range records {
			if !record.Erased {
				n++
			}
		}
	}
	return n
}

func (d statementDerived) activeReviews() []model.ErasureReview {
	reviews := []model.ErasureReview{}
	subjects := []model.ErasureReviewSubject{model.ErasureReviewBodyFact, model.ErasureReviewBodyObservation, model.ErasureReviewBodyHypothesis}
	for i, records := range d.all() {
		for _, record := range records {
			if record.AssertionStatus == "asserted" {
				reviews = append(reviews, model.ErasureReview{Subject: subjects[i], SubjectID: record.ID})
			}
		}
	}
	return reviews
}

// statementDerivedRecords reads (and with lock, row-locks) the records
// derived from the statement.
func statementDerivedRecords(tx *gorm.DB, userID, statementID uuid.UUID, lock bool) (statementDerived, error) {
	var out statementDerived
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	for _, family := range []struct {
		table string
		into  *[]statementDerivedRecord
	}{{"body_facts", &out.Facts}, {"body_observations", &out.Observations}, {"body_hypotheses", &out.Hypotheses}} {
		query := fmt.Sprintf(`SELECT id, assertion_status, erased_at IS NOT NULL AS erased FROM %s
WHERE user_id = ? AND source_evidence_id = ? ORDER BY id%s`, family.table, suffix)
		if err := tx.Raw(query, userID, statementID).Scan(family.into).Error; err != nil {
			return out, err
		}
	}
	return out, nil
}

// statementPinningConditionsSQL selects the adopted condition entries citing
// the statement: a current evidence link to a derived fact or observation, or
// a user claim naming the statement. Withdrawn (inactive) conditions do not pin.
const statementPinningConditionsSQL = `SELECT c.id FROM condition_entries c
WHERE c.user_id = @user AND c.disposition = 'adopted' AND (
  EXISTS (SELECT 1 FROM condition_entry_evidence e
    WHERE e.user_id = c.user_id AND e.condition_id = c.id
      AND (e.body_fact_id IN @facts OR e.body_observation_id IN @observations))
  OR EXISTS (SELECT 1 FROM jsonb_array_elements(c.source_claim_refs) ref
    WHERE ref->>'sourceKind' IN ('user_suspicion','user_reported_clinical') AND ref->>'sourceId' = @statement))
ORDER BY c.id`

func statementPinningConditions(tx *gorm.DB, userID, statementID uuid.UUID, derived statementDerived) ([]uuid.UUID, error) {
	refs := derived.bodyRefs()
	var ids []uuid.UUID
	err := tx.Raw(statementPinningConditionsSQL, map[string]any{
		"user": userID, "statement": statementID.String(),
		"facts": nonEmptyIDs(refs.Facts), "observations": nonEmptyIDs(refs.Observations),
	}).Scan(&ids).Error
	return ids, err
}

// statementPins is what holds a statement's copies in place right now.
type statementPins struct {
	statementID uuid.UUID
	derived     statementDerived
	conditions  []uuid.UUID
	plans       []model.ErasureReview
}

func (p statementPins) needles() []string { return p.derived.needles(p.statementID) }

// lockStatementPins takes the statement row lock (Diagnosis and plan
// admission qualify a statement under a share lock) and the derived record
// locks, then reads the conditions and plans that pin the statement.
func lockStatementPins(tx *gorm.DB, userID, statementID uuid.UUID) (statementPins, error) {
	pins := statementPins{statementID: statementID}
	if err := tx.Exec(`SELECT 1 FROM statement_evidence WHERE user_id = ? AND id = ? FOR UPDATE`, userID, statementID).Error; err != nil {
		return pins, err
	}
	var err error
	if pins.derived, err = statementDerivedRecords(tx, userID, statementID, true); err != nil {
		return pins, err
	}
	if pins.conditions, err = statementPinningConditions(tx, userID, statementID, pins.derived); err != nil {
		return pins, err
	}
	refs := pins.derived.bodyRefs()
	if len(pins.conditions) > 0 || !refs.empty() {
		if pins.plans, err = dependentPlans(tx, userID, pins.conditions, refs); err != nil {
			return pins, err
		}
	}
	return pins, nil
}

// eraseStatementBodyRecords clears the text of the derived records once
// none is asserted and nothing pins them. Rows are kept as tombstones
// (identity, kind and lineage) because withdrawn conditions, episodes and
// accepted plan revisions link to them; nothing that links to them changes.
func eraseStatementBodyRecords(tx *gorm.DB, userID, statementID uuid.UUID, pins statementPins) (model.AssetErasureOutcome, error) {
	if pins.derived.unerased() == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, nil
	}
	if active := pins.derived.active(); active > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: active, FailureCode: model.ErasureCodeBodyRecordActive}, nil
	}
	if len(pins.conditions) > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: pins.derived.unerased(), FailureCode: model.AssetErasureCodeDiagnosisPinned}, nil
	}
	if len(pins.plans) > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: pins.derived.unerased(), FailureCode: model.ErasureCodePlanReview}, nil
	}
	erased := int64(0)
	for _, statement := range []string{
		`UPDATE body_facts SET erased_at = now(), statement = '', details = '{}', body_region_ids = '[]',
    temporal = '{"precision": "unknown"}'
WHERE user_id = ? AND source_evidence_id = ? AND erased_at IS NULL AND assertion_status <> 'asserted'`,
		`UPDATE body_observations SET erased_at = now(), raw_value = '', raw_unit = '', method = '',
    measurement_context = '', observed_at = '{"precision": "unknown"}'
WHERE user_id = ? AND source_evidence_id = ? AND erased_at IS NULL AND assertion_status <> 'asserted'`,
		`UPDATE body_hypotheses SET erased_at = now(), statement = '', withdrawal_reason = NULL
WHERE user_id = ? AND source_evidence_id = ? AND erased_at IS NULL AND assertion_status <> 'asserted'`,
	} {
		result := tx.Exec(statement, userID, statementID)
		if result.Error != nil {
			return model.AssetErasureOutcome{}, result.Error
		}
		erased += result.RowsAffected
	}
	after, err := statementDerivedRecords(tx, userID, statementID, false)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if left := after.unerased(); left > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: left, FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: int(erased)}, nil
}

// eraseStatementAssessmentReports deletes the assessment reports naming the
// statement or a derived record once nothing pins them; a frozen report is
// deleted whole, never rewritten.
func eraseStatementAssessmentReports(tx *gorm.DB, userID uuid.UUID, pins statementPins) (model.AssetErasureOutcome, error) {
	needles := pins.needles()
	held, err := countRowsMentioning(tx, "assessment_reports", userID, needles)
	if err != nil || held == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, err
	}
	if len(pins.conditions) > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: held, FailureCode: model.AssetErasureCodeAssessmentFrozen}, nil
	}
	where, args := mentioningClause(userID, needles)
	deleted := tx.Exec(`DELETE FROM assessment_reports t WHERE `+where, args...)
	if deleted.Error != nil {
		return model.AssetErasureOutcome{}, deleted.Error
	}
	left, err := countRowsMentioning(tx, "assessment_reports", userID, needles)
	if err != nil {
		return model.AssetErasureOutcome{}, err
	}
	if left > 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: left, FailureCode: model.AssetErasureCodeCopyReappeared}, nil
	}
	return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: int(deleted.RowsAffected)}, nil
}

// inventoryStatementDecisions never mutates: decision-trail rows naming the
// statement or a derived record are frozen replay inputs, reported as a
// terminal retained_by_policy outcome rather than a failure to retry.
func inventoryStatementDecisions(tx *gorm.DB, userID uuid.UUID, pins statementPins) (model.AssetErasureOutcome, error) {
	n, err := countStatementDecisions(tx, userID, pins.needles())
	if err != nil || n == 0 {
		return model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}, err
	}
	return model.AssetErasureOutcome{State: model.ErasureTargetRetainedByPolicy, Outstanding: n, RetentionReason: model.ErasureRetentionReplayInputFrozen}, nil
}

func countStatementDecisions(tx *gorm.DB, userID uuid.UUID, needles []string) (int, error) {
	total := 0
	for _, table := range statementDecisionTables {
		n, err := countRowsMentioning(tx, table, userID, needles)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// mentioningClause matches an owner row whose JSON text names any needle.
func mentioningClause(userID uuid.UUID, needles []string) (string, []any) {
	clauses := make([]string, 0, len(needles))
	args := []any{userID}
	for _, needle := range needles {
		clauses = append(clauses, "strpos(to_jsonb(t)::text, ?) > 0")
		args = append(args, needle)
	}
	return fmt.Sprintf("t.user_id = ? AND (%s)", strings.Join(clauses, " OR ")), args
}

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

// RuntimeScope lists what the runtime_checkpoints target hands the
// ai-service: every Conversation of the owner, the one the statement was said
// in, and the derived record ids. The user's own chat message is a
// conversation record (chat retention), not a copy.
func (r *StatementErasureRepository) RuntimeScope(ctx context.Context, userID, statementID uuid.UUID) (model.StatementRuntimeScope, error) {
	db := r.conn(ctx)
	scope := model.StatementRuntimeScope{ThreadIDs: []uuid.UUID{}, AttachedThreadIDs: []uuid.UUID{}}
	var rows []struct {
		ID       uuid.UUID
		Attached bool
	}
	if err := db.Raw(`SELECT c.id, EXISTS (SELECT 1 FROM statement_evidence e
    WHERE e.user_id = c.user_id AND e.id = ? AND e.source_conversation_id = c.id) AS attached
 FROM conversations c WHERE c.user_id = ? ORDER BY c.id`, statementID, userID).Scan(&rows).Error; err != nil {
		return scope, fmt.Errorf("statement runtime scope: %w", err)
	}
	for _, row := range rows {
		scope.ThreadIDs = append(scope.ThreadIDs, row.ID)
		if row.Attached {
			scope.AttachedThreadIDs = append(scope.AttachedThreadIDs, row.ID)
		}
	}
	derived, err := statementDerivedRecords(db, userID, statementID, false)
	if err != nil {
		return scope, fmt.Errorf("statement runtime scope: %w", err)
	}
	scope.RecordIDs = derived.ids()
	return scope, nil
}

// Finalize erases the statement and ends the request in one transaction,
// only when every other target verified (decision_records and source_message
// may be retained by policy) and a re-check of every database family still holds. A copy that
// reappeared reopens its target instead. It returns whether the request is
// now terminal.
func (r *StatementErasureRepository) Finalize(ctx context.Context, request *model.StatementErasureRequest) (bool, error) {
	terminal := false
	err := r.conn(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.StatementErasureRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", request.UserID, request.ID).Take(&current).Error; err != nil {
			return err
		}
		if current.Status.Terminal() {
			terminal = true
			return nil
		}
		pins, err := lockStatementPins(tx, request.UserID, request.StatementID)
		if err != nil {
			return err
		}
		now := r.now()
		blockManifest := func() error {
			return recordStatementErasureOutcome(tx, now, request.UserID, request.ID, model.StatementErasureManifest,
				model.AssetErasureOutcome{State: model.AssetErasureTargetBlocked, Outstanding: 1, FailureCode: model.AssetErasureCodeTargetsOutstanding})
		}
		var unverified int64
		if err := tx.Model(&model.StatementErasureTarget{}).
			Where("user_id = ? AND request_id = ? AND target <> ? AND state NOT IN ?", request.UserID, request.ID, model.StatementErasureManifest,
				[]model.AssetErasureTargetState{model.AssetErasureTargetVerified, model.ErasureTargetRetainedByPolicy}).
			Count(&unverified).Error; err != nil {
			return err
		}
		if unverified > 0 {
			return blockManifest()
		}
		reappeared := map[model.StatementErasureTargetName]int{}
		if n := len(pins.conditions); n > 0 {
			reappeared[model.StatementErasureDiagnosisRecords] = n
		}
		if n := len(pins.plans); n > 0 {
			reappeared[model.StatementErasureTrainingPlans] = n
		}
		if n := pins.derived.unerased(); n > 0 {
			reappeared[model.StatementErasureBodyRecords] = n
		}
		if n, err := countRowsMentioning(tx, "assessment_reports", request.UserID, pins.needles()); err != nil {
			return err
		} else if n > 0 {
			reappeared[model.StatementErasureAssessmentReports] = n
		}
		if len(reappeared) > 0 {
			for target, n := range reappeared {
				if err := recordStatementErasureOutcome(tx, now, request.UserID, request.ID, target,
					model.AssetErasureOutcome{State: model.AssetErasureTargetFailed, Outstanding: n, FailureCode: model.AssetErasureCodeCopyReappeared}); err != nil {
					return err
				}
			}
			return blockManifest()
		}
		// A decision made since the decision_records pass is retained too;
		// the statement row lock keeps the answer stable until commit.
		decisions, err := inventoryStatementDecisions(tx, request.UserID, pins)
		if err != nil {
			return err
		}
		if err := recordStatementErasureOutcome(tx, now, request.UserID, request.ID, model.StatementErasureDecisionRecords, decisions); err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE statement_evidence SET availability = 'erased', excerpt = '', question_excerpt = NULL
WHERE user_id = ? AND id = ?`, request.UserID, request.StatementID).Error; err != nil {
			return err
		}
		if err := recordStatementErasureOutcome(tx, now, request.UserID, request.ID, model.StatementErasureManifest,
			model.AssetErasureOutcome{State: model.AssetErasureTargetVerified, Erased: 1}); err != nil {
			return err
		}
		var retained int64
		if err := tx.Model(&model.StatementErasureTarget{}).
			Where("user_id = ? AND request_id = ? AND state = ?", request.UserID, request.ID, model.ErasureTargetRetainedByPolicy).
			Count(&retained).Error; err != nil {
			return err
		}
		status := model.StatementErasureErased
		if retained > 0 {
			status = model.StatementErasureErasedWithRetention
		}
		if err := tx.Exec(`UPDATE statement_erasure_requests
SET status = ?, erased_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE user_id = ? AND id = ?`, status, now, now, request.UserID, request.ID).Error; err != nil {
			return err
		}
		terminal = true
		return nil
	})
	return terminal, err
}

// ListRecoverable returns requests a worker should run: new ones, expired
// leases, and outstanding ones with a transient failure older than
// retryAfter. Blocked targets wait for the user's review and a repeated
// command; retained_by_policy is terminal and never retried.
func (r *StatementErasureRepository) ListRecoverable(ctx context.Context, retryAfter time.Duration, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 10
	}
	now := r.now()
	var ids []uuid.UUID
	err := r.conn(ctx).Raw(`SELECT q.id FROM statement_erasure_requests q
WHERE q.status = 'pending'
   OR (q.status = 'running' AND q.lease_expires_at < ?)
   OR (q.status = 'outstanding' AND q.updated_at < ? AND EXISTS (
        SELECT 1 FROM statement_erasure_targets t
        WHERE t.user_id = q.user_id AND t.request_id = q.id AND t.state IN ('pending','failed')))
ORDER BY q.updated_at ASC
LIMIT ?`, now, now.Add(-retryAfter), limit).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("list recoverable statement erasures: %w", err)
	}
	return ids, nil
}
