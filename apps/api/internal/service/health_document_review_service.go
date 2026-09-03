package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ErrReviewAccessDenied is returned when the caller does not own the upload,
// extraction run or candidate referenced by a review request. Callers map it
// to a 403 so ownership leaks are avoided.
var ErrReviewAccessDenied = errors.New("review target not owned by caller")

// ErrReviewCandidateMismatch is returned when the submitted candidate identity
// or source refs disagree with the persisted extraction snapshot. Callers map
// it to a 409 so stale clients reload the run.
var ErrReviewCandidateMismatch = errors.New("review candidate does not match extraction snapshot")

// ErrReviewDuplicateConflict is returned when the same idempotency key is
// reused with different action or payload. Callers map it to 409.
var ErrReviewDuplicateConflict = errors.New("idempotency key reused with different review content")

// ErrReviewValidation indicates a malformed review request (400).
var ErrReviewValidation = errors.New("invalid review request")

// documentExtractionRunReader is the ownership-scoped read side the review
// service needs from the extraction-run repository.
type documentExtractionRunReader interface {
	GetOwnedByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.DocumentExtractionRun, error)
}

// documentIndicatorReviewWriter is the append-only write/read side for reviews.
type documentIndicatorReviewWriter interface {
	Create(ctx context.Context, review *model.DocumentIndicatorReview) error
	ByOwnerScope(ctx context.Context, userID uuid.UUID, extractionRunID uuid.UUID, indicatorIndex int, idempotencyKey string) ([]model.DocumentIndicatorReview, error)
	ListByUpload(ctx context.Context, uploadID uuid.UUID, userID uuid.UUID) ([]model.DocumentIndicatorReview, error)
	ListByExtractionRun(ctx context.Context, extractionRunID uuid.UUID) ([]model.DocumentIndicatorReview, error)
}

// HealthDocumentReviewService implements the append-only human review flow on
// top of persisted document extraction runs. It never mutates
// document_extraction_runs or user_uploads.ocr_result.
type HealthDocumentReviewService struct {
	runs    documentExtractionRunReader
	reviews documentIndicatorReviewWriter
	now     func() time.Time
}

// NewHealthDocumentReviewService creates a new review service.
func NewHealthDocumentReviewService(
	runs documentExtractionRunReader,
	reviews documentIndicatorReviewWriter,
) *HealthDocumentReviewService {
	return &HealthDocumentReviewService{runs: runs, reviews: reviews, now: time.Now}
}

// reviewCandidateSnapshot decodes one persisted machine candidate drawn from
// the extraction run indicator snapshot. raw keeps the exact immutable JSON.
type reviewCandidateSnapshot struct {
	raw         map[string]any
	IndicatorID string
	Name        string
	Value       any
	Unit        string
	SourceRefs  []string
}

// DocumentIndicatorCandidate is one machine-extracted candidate projection
// exposed to the review UI. It never includes private raw text or storage keys.
type DocumentIndicatorCandidate struct {
	IndicatorIndex int      `json:"indicator_index"`
	IndicatorID    string   `json:"indicator_id"`
	Name           string   `json:"name"`
	Value          any      `json:"value,omitempty"`
	Unit           string   `json:"unit,omitempty"`
	SourceRefs     []string `json:"source_refs,omitempty"`
}

// DocumentIndicatorReviewRecord is one immutable review row as exposed to
// clients; it never includes storage backend/key or raw OCR text.
type DocumentIndicatorReviewRecord struct {
	ReviewID        uuid.UUID       `json:"id"`
	ExtractionRunID uuid.UUID       `json:"extraction_run_id"`
	UploadID        uuid.UUID       `json:"upload_id"`
	IndicatorIndex  int             `json:"indicator_index"`
	IndicatorID     string          `json:"indicator_id"`
	Action          string          `json:"action"`
	ReviewedPayload json.RawMessage `json:"reviewed_payload,omitempty"`
	Note            string          `json:"note,omitempty"`
	ReviewerUserID  uuid.UUID       `json:"reviewer_user_id"`
	CreatedAt       time.Time       `json:"created_at"`
	IdempotencyKey  string          `json:"idempotency_key"`
}

// DocumentIndicatorReviewProjection is the effective latest review state for
// one candidate plus its full history for audit replay.
type DocumentIndicatorReviewProjection struct {
	IndicatorIndex  int                             `json:"indicator_index"`
	IndicatorID     string                          `json:"indicator_id"`
	Candidate       DocumentIndicatorCandidate      `json:"candidate"`
	EffectiveReview *DocumentIndicatorReviewRecord  `json:"effective_review,omitempty"`
	History         []DocumentIndicatorReviewRecord `json:"history,omitempty"`
}

// ListCandidates projects the machine candidates of one extraction run plus
// their effective latest review, scoped to the caller's ownership.
func (s *HealthDocumentReviewService) ListCandidates(
	ctx context.Context,
	userID uuid.UUID,
	extractionRunID uuid.UUID,
) ([]DocumentIndicatorReviewProjection, error) {
	run, err := s.runs.GetOwnedByID(ctx, extractionRunID, userID)
	if err != nil {
		return nil, ErrReviewAccessDenied
	}
	snapshot, err := decodeIndicatorSnapshot(run.IndicatorSnapshot)
	if err != nil {
		return nil, fmt.Errorf("decode indicator snapshot: %w", err)
	}
	history, err := s.reviews.ListByExtractionRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	latestByIndex := make(map[int]DocumentIndicatorReviewRecord)
	historyByIndex := make(map[int][]DocumentIndicatorReviewRecord)
	for _, row := range history {
		record := reviewRecordFromRow(row)
		historyByIndex[row.IndicatorIndex] = append(historyByIndex[row.IndicatorIndex], record)
		if existing, ok := latestByIndex[row.IndicatorIndex]; !ok || record.CreatedAt.After(existing.CreatedAt) {
			latestByIndex[row.IndicatorIndex] = record
		}
	}
	projections := make([]DocumentIndicatorReviewProjection, 0, len(snapshot))
	for index, candidate := range snapshot {
		proj := DocumentIndicatorReviewProjection{
			IndicatorIndex: index,
			IndicatorID:    candidate.IndicatorID,
			Candidate: DocumentIndicatorCandidate{
				IndicatorIndex: index,
				IndicatorID:    candidate.IndicatorID,
				Name:           candidate.Name,
				Value:          candidate.Value,
				Unit:           candidate.Unit,
				SourceRefs:     append([]string(nil), candidate.SourceRefs...),
			},
		}
		if record, ok := latestByIndex[index]; ok {
			proj.EffectiveReview = &record
		}
		if records, ok := historyByIndex[index]; ok {
			proj.History = records
		}
		projections = append(projections, proj)
	}
	return projections, nil
}

// ReviewRequest is the authenticated payload to append a review action for a
// single candidate of an exact extraction run.
type ReviewRequest struct {
	ExtractionRunID uuid.UUID       `json:"extraction_run_id"`
	IndicatorIndex  int             `json:"indicator_index"`
	IndicatorID     string          `json:"indicator_id"`
	Action          string          `json:"action"`
	ReviewedPayload json.RawMessage `json:"reviewed_payload"`
	SourceRefs      []string        `json:"source_refs"`
	IdempotencyKey  string          `json:"idempotency_key"`
	Note            string          `json:"note"`
}

// ApplyReview validates a review request against the persisted extraction
// snapshot and appends an immutable review row. It only ever inserts; it never
// updates a prior review row and never mutates the extraction run or upload
// OCR. On an idempotent replay of the same request it returns the existing row.
func (s *HealthDocumentReviewService) ApplyReview(
	ctx context.Context,
	userID uuid.UUID,
	req ReviewRequest,
) (*DocumentIndicatorReviewRecord, error) {
	if req.ExtractionRunID == uuid.Nil || req.IndicatorIndex < 0 {
		return nil, fmt.Errorf("%w: extraction run or indicator index missing", ErrReviewValidation)
	}
	action, err := normalizeReviewAction(req.Action)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, fmt.Errorf("%w: idempotency_key is required", ErrReviewValidation)
	}

	run, err := s.runs.GetOwnedByID(ctx, req.ExtractionRunID, userID)
	if err != nil {
		// Fail closed: do not leak whether the run exists.
		return nil, ErrReviewAccessDenied
	}
	snapshot, err := decodeIndicatorSnapshot(run.IndicatorSnapshot)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrReviewCandidateMismatch, err)
	}
	if req.IndicatorIndex < 0 || req.IndicatorIndex >= len(snapshot) {
		return nil, ErrReviewCandidateMismatch
	}
	candidate := snapshot[req.IndicatorIndex]
	// Candidate identity must match the persisted snapshot exactly.
	if candidate.IndicatorID != req.IndicatorID {
		return nil, fmt.Errorf("%w: indicator identity changed", ErrReviewCandidateMismatch)
	}

	payload, err := normalizeReviewedPayload(action, req.ReviewedPayload)
	if err != nil {
		return nil, err
	}
	if action == string(model.ReviewActionCorrect) {
		var corrected map[string]any
		if err := json.Unmarshal(payload, &corrected); err != nil {
			return nil, fmt.Errorf("%w: corrected payload cannot be decoded", ErrReviewValidation)
		}
		correctedID, _ := corrected["indicator_id"].(string)
		if strings.TrimSpace(correctedID) != candidate.IndicatorID {
			return nil, fmt.Errorf("%w: corrected payload changed indicator identity", ErrReviewCandidateMismatch)
		}
	}

	// Source refs must resolve against the run's persisted source snapshot.
	sourceBlocks, err := decodeSourceBlocks(run.SourceSummary)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrReviewCandidateMismatch, err)
	}
	if err := validateReviewSourceRefs(req.SourceRefs, sourceBlocks, candidate.SourceRefs); err != nil {
		return nil, err
	}

	// Idempotency: reuse of the same key in the owner scope returns the prior
	// result unchanged, or conflicts when the content differs.
	existing, err := s.reviews.ByOwnerScope(ctx, userID, run.ID, req.IndicatorIndex, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		prior := existing[len(existing)-1]
		if prior.Action == action && sameJSON(json.RawMessage(prior.ReviewedPayload), req.ReviewedPayload) {
			rec := reviewRecordFromRow(prior)
			return &rec, nil
		}
		return nil, ErrReviewDuplicateConflict
	}

	machineCandidate, err := json.Marshal(candidate.raw)
	if err != nil {
		return nil, fmt.Errorf("marshal machine candidate snapshot: %w", err)
	}
	sourceRefsJSON, err := json.Marshal(candidate.SourceRefs)
	if err != nil {
		return nil, fmt.Errorf("marshal source refs: %w", err)
	}
	pageRefJSON, err := pageRefForCandidate(sourceBlocks, candidate.SourceRefs)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	row := &model.DocumentIndicatorReview{
		UserID:           userID,
		UploadID:         run.UploadID,
		ExtractionRunID:  run.ID,
		IndicatorIndex:   req.IndicatorIndex,
		Action:           action,
		IdempotencyKey:   req.IdempotencyKey,
		ReviewedPayload:  datatypes.JSON(payload),
		MachineCandidate: datatypes.JSON(machineCandidate),
		SourceRefs:       sourceRefsJSON,
		PageRef:          datatypes.JSON(pageRefJSON),
		ReviewerUserID:   userID,
		Note:             req.Note,
		CreatedAt:        now,
	}
	if err := s.reviews.Create(ctx, row); err != nil {
		return nil, err
	}
	rec := reviewRecordFromRow(*row)
	return &rec, nil
}

// EnsureUploadOwnsRun verifies that the given extraction run belongs to the
// given user AND that the run is attached to the given upload, so agents in the
// handler cannot bind a review against a run that belongs to a different upload
// of the same owner. Returns ErrReviewCandidateMismatch when they disagree and
// ErrReviewAccessDenied when the run is not owned by the caller.
func (s *HealthDocumentReviewService) EnsureUploadOwnsRun(
	ctx context.Context,
	userID uuid.UUID,
	uploadID, runID uuid.UUID,
) error {
	run, err := s.runs.GetOwnedByID(ctx, runID, userID)
	if err != nil {
		return ErrReviewAccessDenied
	}
	if run.UploadID != uploadID {
		return fmt.Errorf("%w: extraction run does not belong to the referenced upload", ErrReviewCandidateMismatch)
	}
	return nil
}
