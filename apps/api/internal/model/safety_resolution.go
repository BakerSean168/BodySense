package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidSafetyResolution             = errors.New("invalid safety resolution")
	ErrSafetyRestrictionNotFound           = errors.New("safety restriction not found")
	ErrSafetyRestrictionAlreadyResolved    = errors.New("safety restriction already resolved")
	ErrSafetyResolutionSourceNotFound      = errors.New("safety resolution source not found")
	ErrSafetyResolutionIdempotencyConflict = errors.New("safety resolution idempotency conflict")
)

// ProfessionalClearanceStatement is the explicit confirmation supported by the
// first user-reported clearance flow. It must occur verbatim in the user's own
// message; arbitrary positive prose and model interpretations are not clearance.
const ProfessionalClearanceStatement = "专业人员已针对这条限制及全部训练放行"

const SafetyResolutionPolicyV1 = "restriction-lift-v2"
const SafetyReverificationPurpose = "safety_reverification"

type ResolveSafetyRestrictionCommand struct {
	IdempotencyKey             string    `json:"idempotency_key"`
	ExpectedRevision           int64     `json:"expected_revision"`
	SourceMessageID            uuid.UUID `json:"source_message_id,omitzero"`
	Statement                  string    `json:"statement,omitempty"`
	ExpectedRestrictionVersion int64     `json:"expected_restriction_version"`
	BasisKind                  string    `json:"basis_kind"`
	SourceVersion              int64     `json:"source_version,omitempty"`
	CheckInteractionID         uuid.UUID `json:"check_interaction_id,omitzero"`
}

func (c ResolveSafetyRestrictionCommand) Validate() error {
	invalid := fmt.Errorf("%w: exact basis, target version and source required", ErrInvalidSafetyResolution)
	if len(c.IdempotencyKey) < 1 || len(c.IdempotencyKey) > 128 || c.IdempotencyKey != strings.TrimSpace(c.IdempotencyKey) || c.ExpectedRevision < 0 || c.ExpectedRestrictionVersion < 1 {
		return invalid
	}
	switch c.BasisKind {
	case "expiry":
		if c.SourceMessageID != uuid.Nil || c.CheckInteractionID != uuid.Nil || c.SourceVersion != 0 || c.Statement != "" {
			return invalid
		}
	case "user_reverification":
		if c.SourceMessageID == uuid.Nil || c.CheckInteractionID == uuid.Nil || c.SourceVersion != 1 || c.Statement != "" {
			return invalid
		}
	case "professional_clearance":
		if c.SourceMessageID == uuid.Nil || c.CheckInteractionID != uuid.Nil || c.SourceVersion != 1 || !strings.HasPrefix(c.Statement, ProfessionalClearanceStatement) || len(c.Statement) > 12000 {
			return invalid
		}
	default:
		return invalid
	}
	return nil
}

// SafetyProfessionalClearanceStatement names any additional reviewed conditions
// verbatim. The existing declaration covers all training, including scoped risks.
func SafetyProfessionalClearanceStatement(r SafetyRestriction) string {
	statement := ProfessionalClearanceStatement
	if len(r.LiftConditions) > 0 {
		statement += "；解除条件：" + strings.Join(r.LiftConditions, "；")
	}
	return statement
}

// SafetyReverificationQuestion is constructed by Go from a reviewed restriction.
// Every explicit lift condition must be affirmed; no medical text is interpreted.
func SafetyReverificationQuestion(r SafetyRestriction) ([]byte, error) {
	if r.ID == uuid.Nil || r.CreatedRevision < 1 || r.LiftPolicy != "user_reverification" || len(r.LiftConditions) == 0 || len(r.LiftConditions) > 20 {
		return nil, ErrInvalidSafetyResolution
	}
	fields := make([]map[string]any, 0, len(r.LiftConditions))
	for i, condition := range r.LiftConditions {
		if strings.TrimSpace(condition) == "" || len([]rune(condition)) > 1000 {
			return nil, ErrInvalidSafetyResolution
		}
		fields = append(fields, map[string]any{"key": fmt.Sprintf("condition_%d", i), "label": condition, "answer_type": "single_choice", "options": []string{"是", "否", "不确定"}, "required": true})
	}
	return json.Marshal(map[string]any{"purpose": SafetyReverificationPurpose, "restriction_id": r.ID, "restriction_version": r.CreatedRevision, "scope_kind": r.ScopeKind, "plan_id": r.PlanID, "exercise_ids": append([]string{}, r.ExerciseIDs...), "question": fmt.Sprintf("复核限制 %s（%s）：%s。请逐项确认解除条件；不确定请选择不确定。", r.ID, r.ScopeKind, r.Reason), "answer_type": "text", "fields": fields})
}

func SafetyReverificationSatisfied(r SafetyRestriction, question, answer []byte) bool {
	canonical, err := SafetyReverificationQuestion(r)
	if err != nil {
		return false
	}
	expected, err := InteractionJSONFingerprint(canonical)
	if err != nil {
		return false
	}
	actual, err := InteractionJSONFingerprint(question)
	if err != nil || expected != actual {
		return false
	}
	normalized, _, err := ValidateInteractionAnswer(question, answer)
	if err != nil {
		return false
	}
	var parsed struct {
		Fields map[string]string `json:"fields"`
	}
	if json.Unmarshal(normalized, &parsed) != nil || len(parsed.Fields) != len(r.LiftConditions) {
		return false
	}
	for i := range r.LiftConditions {
		if parsed.Fields[fmt.Sprintf("condition_%d", i)] != "是" {
			return false
		}
	}
	return true
}

// SafetyResolution is both the immutable per-restriction lift audit and the
// idempotent receipt. Original text lives only in owner-bound StatementEvidence.
// User-reported professional clearance never claims clinical-document admission.
type SafetyResolution struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"user_id"`
	RestrictionID      uuid.UUID  `gorm:"type:uuid;not null" json:"restriction_id"`
	SourceEvidenceID   *uuid.UUID `gorm:"type:uuid" json:"source_evidence_id"`
	SourceMessageID    *uuid.UUID `gorm:"type:uuid" json:"source_message_id"`
	SourceVersion      int64      `gorm:"not null" json:"source_version"`
	RestrictionVersion int64      `gorm:"not null" json:"restriction_version"`
	CheckInteractionID *uuid.UUID `gorm:"type:uuid" json:"check_interaction_id"`
	BasisKind          string     `gorm:"not null" json:"basis_kind"`
	SourceLevel        string     `gorm:"not null" json:"source_level"`
	ScopeKind          string     `gorm:"not null" json:"scope_kind"`
	PolicyRevision     string     `gorm:"not null" json:"policy_revision"`
	IdempotencyKey     string     `gorm:"not null" json:"-"`
	Fingerprint        string     `gorm:"not null" json:"-"`
	Revision           int64      `gorm:"not null" json:"revision"`
	RevisionID         uuid.UUID  `gorm:"type:uuid;not null" json:"revision_id"`
	EventID            uuid.UUID  `gorm:"type:uuid;not null" json:"event_id"`
	Actor              string     `gorm:"not null" json:"actor"`
	LiftedAt           time.Time  `gorm:"not null" json:"lifted_at"`
}

func (SafetyResolution) TableName() string { return "safety_resolutions" }

// SafetyRestrictionRecord is a read projection. Source invalidation makes an
// earlier resolution ineffective without erasing its immutable audit.
type SafetyRestrictionRecord struct {
	SafetyRestriction
	Resolution         *SafetyResolution `json:"resolution"`
	ClearanceAvailable bool              `json:"clearance_available"`
	Active             bool              `json:"active"`
}

type SafetyRestrictionList struct {
	UserID          uuid.UUID                 `json:"user_id"`
	CurrentRevision int64                     `json:"current_revision"`
	Restrictions    []SafetyRestrictionRecord `json:"restrictions"`
}
