package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// KnowledgePublicationObservation is one immutable quality observation for a publication.
type KnowledgePublicationObservation struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	PublicationID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"publication_id"`
	ObservationKey    string         `gorm:"type:varchar(200);not null" json:"observation_key"`
	ObservationKind   string         `gorm:"type:varchar(32);not null;default:'predeploy_eval';index" json:"observation_kind"`
	EvaluatorRevision string         `gorm:"type:varchar(100);not null" json:"evaluator_revision"`
	CaseID            string         `gorm:"type:varchar(120);not null" json:"case_id"`
	RetrievalStatus   string         `gorm:"type:varchar(32);not null" json:"retrieval_status"`
	CitationStatus    string         `gorm:"type:varchar(32);not null" json:"citation_status"`
	GroundingStatus   string         `gorm:"type:varchar(32);not null" json:"grounding_status"`
	IdentityStatus    string         `gorm:"type:varchar(32);not null" json:"identity_status"`
	ProvenanceStatus  string         `gorm:"type:varchar(32);not null" json:"provenance_status"`
	ExecutionError    string         `gorm:"type:text" json:"execution_error,omitempty"`
	Metadata          datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt         time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (KnowledgePublicationObservation) TableName() string {
	return "knowledge_publication_observations"
}
