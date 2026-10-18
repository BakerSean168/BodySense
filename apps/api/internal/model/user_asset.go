package model

import (
	"time"

	"github.com/google/uuid"
)

// UserAssetKind is the closed asset-category vocabulary of the sole
// owner-scoped file identity. `photo` covers posture and consultation photos
// (the concrete photo view is the separate PhotoView field), `report` is the
// health-report upload that feeds the document-evidence pipeline, and
// `document` is reserved for future generic document uploads.
type UserAssetKind string

const (
	UserAssetKindPhoto    UserAssetKind = "photo"
	UserAssetKindReport   UserAssetKind = "report"
	UserAssetKindDocument UserAssetKind = "document"
)

// UserAssetAvailability keeps source removal strictly distinct from erasure
// (PC-D7): `source_removed` means the private original is unavailable while
// adopted evidence history stays readable with a source-unavailable marker.
// Physical source cleanup may still require a retry; availability is not a
// cleanup receipt. A successful removal command verifies all known sources;
// `erased` is the controlled-copy erasure lifecycle owned by AS-002.
type UserAssetAvailability string

const (
	UserAssetAvailable     UserAssetAvailability = "available"
	UserAssetSourceRemoved UserAssetAvailability = "source_removed"
	UserAssetErased        UserAssetAvailability = "erased"
)

// PostureView identifies the concrete posture photo view of a photo asset.
type PostureView string

const (
	PostureViewFront PostureView = "front"
	PostureViewSide  PostureView = "side"
	PostureViewBack  PostureView = "back"
)

// DerivedRunStatus is the shared pending -> processing -> completed | failed
// lifecycle of asset-derived AI runs (document extraction, posture analysis).
type DerivedRunStatus string

const (
	DerivedRunPending    DerivedRunStatus = "pending"
	DerivedRunProcessing DerivedRunStatus = "processing"
	DerivedRunCompleted  DerivedRunStatus = "completed"
	DerivedRunFailed     DerivedRunStatus = "failed"
)

func (s DerivedRunStatus) IsTerminal() bool {
	return s == DerivedRunCompleted || s == DerivedRunFailed
}

// UserAsset is the single owner-scoped identity of one uploaded private file.
// It owns storage identity, content hash and the file-removal lifecycle only
// (an erased tombstone has no content hash: the column is NULL and reads as "");
// OCR and posture results live in derived run tables that reference
// (user_id, id) through composite foreign keys.
type UserAsset struct {
	ID              uuid.UUID             `gorm:"type:uuid;primaryKey;default:uuidv7()" json:"id"`
	UserID          uuid.UUID             `gorm:"type:uuid;not null;index" json:"-"`
	Kind            UserAssetKind         `gorm:"type:varchar(20);not null" json:"kind"`
	PhotoView       *PostureView          `gorm:"type:varchar(10)" json:"photo_view,omitempty"`
	OriginalName    string                `gorm:"type:varchar(255);not null" json:"original_name"`
	MimeType        string                `gorm:"type:varchar(100);not null" json:"mime_type"`
	ByteSize        int64                 `gorm:"not null" json:"byte_size"`
	ContentSHA256   string                `gorm:"type:char(64)" json:"content_sha256"`
	StorageBackend  string                `gorm:"type:varchar(20);not null" json:"-"`
	StorageKey      string                `gorm:"type:varchar(500);not null" json:"-"`
	Availability    UserAssetAvailability `gorm:"type:varchar(20);not null;default:'available'" json:"availability"`
	SourceRemovedAt *time.Time            `gorm:"" json:"source_removed_at,omitempty"`
	ErasedAt        *time.Time            `gorm:"" json:"erased_at,omitempty"`
	CreatedAt       time.Time             `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time             `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the table name for GORM.
func (UserAsset) TableName() string { return "user_assets" }

// UserAssetFileType is the legacy public upload vocabulary of the /uploads
// contract. It is retained as the wire projection of (kind, photo_view) until
// the asset-facing web views land in AS-001 slice 2.
const (
	FileTypeConsultationPhoto = "consultation_photo"
	FileTypePhotoFront        = "photo_front"
	FileTypePhotoSide         = "photo_side"
	FileTypePhotoBack         = "photo_back"
	FileTypeReport            = "report"
)

var fileTypeToPhotoView = map[string]PostureView{
	FileTypePhotoFront: PostureViewFront,
	FileTypePhotoSide:  PostureViewSide,
	FileTypePhotoBack:  PostureViewBack,
}

// UserAssetKindFromFileType maps the public upload vocabulary onto the asset
// identity. The second return is the concrete photo view, empty when absent.
func UserAssetKindFromFileType(fileType string) (UserAssetKind, PostureView, bool) {
	switch fileType {
	case FileTypeConsultationPhoto:
		return UserAssetKindPhoto, "", true
	case FileTypePhotoFront, FileTypePhotoSide, FileTypePhotoBack:
		return UserAssetKindPhoto, fileTypeToPhotoView[fileType], true
	case FileTypeReport:
		return UserAssetKindReport, "", true
	default:
		return "", "", false
	}
}

// FileType projects the asset identity back onto the public upload vocabulary.
func (a UserAsset) FileType() string {
	if a.Kind == UserAssetKindPhoto && a.PhotoView != nil {
		switch *a.PhotoView {
		case PostureViewFront:
			return FileTypePhotoFront
		case PostureViewSide:
			return FileTypePhotoSide
		case PostureViewBack:
			return FileTypePhotoBack
		}
	}
	if a.Kind == UserAssetKindReport {
		return FileTypeReport
	}
	return FileTypeConsultationPhoto
}

// IsPosturePhoto reports whether this asset carries a posture-view photo.
func (a UserAsset) IsPosturePhoto() bool {
	return a.Kind == UserAssetKindPhoto && a.PhotoView != nil
}
