package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// PostureAnalysisRun is the derived posture-analysis execution record of one
// posture photo asset. It references the asset through the composite
// (user_id, asset_id) owner-scoped foreign key; exactly one run exists per
// photo asset (UNIQUE (user_id, asset_id)). The result payload is the
// validated posture agent output; the immutable Agent configuration that
// produced it is pinned at completion.
type PostureAnalysisRun struct {
	ID                   uuid.UUID        `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	UserID               uuid.UUID        `gorm:"type:uuid;not null;index" json:"user_id"`
	AssetID              uuid.UUID        `gorm:"type:uuid;not null;index" json:"asset_id"`
	View                 string           `gorm:"type:varchar(10);not null" json:"view"`
	Status               DerivedRunStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	JobID                *uuid.UUID       `gorm:"type:uuid;index" json:"job_id,omitempty"`
	AgentConfigurationID string           `gorm:"type:varchar(80);not null;default:''" json:"agent_configuration_id"`
	ResultPayload        datatypes.JSON   `gorm:"type:jsonb" json:"result_payload,omitempty"`
	CreatedAt            time.Time        `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt            time.Time        `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (PostureAnalysisRun) TableName() string { return "posture_analysis_runs" }

// FileType projects the run's view onto the legacy public upload vocabulary.
func (r PostureAnalysisRun) FileType() string {
	switch r.View {
	case string(PostureViewFront):
		return FileTypePhotoFront
	case string(PostureViewSide):
		return FileTypePhotoSide
	case string(PostureViewBack):
		return FileTypePhotoBack
	default:
		return FileTypeConsultationPhoto
	}
}
