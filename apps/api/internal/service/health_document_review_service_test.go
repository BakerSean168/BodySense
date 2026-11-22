package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"testing"
	"time"
)

const testIndicatorRef = "page:1:ocr-block:5"

func strPtr(value string) *string { return &value }

func mustReviewJSON(t *testing.T, v any) datatypes.JSON {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return datatypes.JSON(b)
}

func testExtractionRun(t *testing.T, uploadID, userID, runID uuid.UUID) *model.DocumentExtractionRun {
	t.Helper()
	indicator := map[string]any{
		"indicator_id": "hemoglobin", "name": "hb", "value": float64(142), "unit": "g/L", "reference_range": "120-160",
		"parser_revision": "parser-v1", "source_refs": []any{testIndicatorRef},
		"evidence_admissibility": map[string]any{"status": "needs_review", "policy_revision": "pol-v1", "reason_codes": []any{"manual_confirmation_required"}},
		"evidence_verification":  map[string]any{"status": "verified_consensus", "revision": "ver-v1"},
	}
	sourceSummary := map[string]any{
		"source_blocks": []any{map[string]any{
			"source_ref": testIndicatorRef, "page_number": 1, "bbox": []any{float64(1), float64(2), float64(3), float64(4)},
		}},
		"indicator_count": 1, "source_block_count": 1,
	}
	return &model.DocumentExtractionRun{
		ID:                runID,
		AssetID:           uploadID,
		UserID:            userID,
		ConfigurationID:   "c1",
		MechanismRevision: strPtr("m1"),
		DocumentSHA256:    strPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		ResultSHA256:      strPtr("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		RawTextSHA256:     strPtr("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
		IndicatorSnapshot: mustReviewJSON(t, []any{indicator}),
		SourceSummary:     mustReviewJSON(t, sourceSummary),
		CreatedAt:         time.Now(),
	}
}

type fakeReviewRunReader struct {
	runs map[uuid.UUID]*model.DocumentExtractionRun
}

func (f *fakeReviewRunReader) GetOwnedByID(_ context.Context, id, userID uuid.UUID) (*model.DocumentExtractionRun, error) {
	run, ok := f.runs[id]
	if !ok || run.UserID != userID {
		return nil, errReviewNotFound
	}
	return run, nil
}

func (f *fakeReviewRunReader) GetLatestOwnedByAsset(_ context.Context, assetID, userID uuid.UUID) (*model.DocumentExtractionRun, error) {
	var latest *model.DocumentExtractionRun
	for _, run := range f.runs {
		if run.AssetID != assetID || run.UserID != userID {
			continue
		}
		if latest == nil || run.CreatedAt.After(latest.CreatedAt) || (run.CreatedAt.Equal(latest.CreatedAt) && run.ID.String() > latest.ID.String()) {
			latest = run
		}
	}
	if latest == nil {
		return nil, errReviewNotFound
	}
	return latest, nil
}

var errReviewNotFound = errors.New("not found")

type fakeReviewWriter struct {
	rows []model.DocumentIndicatorReview
}

func (f *fakeReviewWriter) Create(_ context.Context, review *model.DocumentIndicatorReview) error {
	if review.ID == uuid.Nil {
		review.ID = uuid.New()
	}
	if review.CreatedAt.IsZero() {
		review.CreatedAt = time.Now()
	}
	f.rows = append(f.rows, *review)
	return nil
}
func (f *fakeReviewWriter) ByOwnerScope(_ context.Context, userID uuid.UUID, runID uuid.UUID, index int, key string) ([]model.DocumentIndicatorReview, error) {
	var out []model.DocumentIndicatorReview
	for _, r := range f.rows {
		if r.UserID == userID && r.ExtractionRunID == runID && r.IndicatorIndex == index && r.IdempotencyKey == key {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeReviewWriter) ListByExtractionRun(_ context.Context, runID uuid.UUID) ([]model.DocumentIndicatorReview, error) {
	var out []model.DocumentIndicatorReview
	for _, r := range f.rows {
		if r.ExtractionRunID == runID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeReviewWriter) ListByAsset(_ context.Context, assetID, userID uuid.UUID) ([]model.DocumentIndicatorReview, error) {
	var out []model.DocumentIndicatorReview
	for _, r := range f.rows {
		if r.AssetID == assetID && r.UserID == userID {
			out = append(out, r)
		}
	}
	return out, nil
}

// fakeReviewAssetReader backs the review service with owner-scoped asset
// availability so CurrentContext can mark evidence source_unavailable.
type fakeReviewAssetReader struct {
	assets map[uuid.UUID]model.UserAsset
}

func (f *fakeReviewAssetReader) GetOwnedByID(_ context.Context, id, userID uuid.UUID) (*model.UserAsset, error) {
	asset, ok := f.assets[id]
	if !ok || asset.UserID != userID {
		return nil, nil
	}
	return &asset, nil
}

func reviewAssetForRun(run *model.DocumentExtractionRun, availability model.UserAssetAvailability) *fakeReviewAssetReader {
	if run == nil {
		return &fakeReviewAssetReader{assets: map[uuid.UUID]model.UserAsset{}}
	}
	return &fakeReviewAssetReader{assets: map[uuid.UUID]model.UserAsset{
		run.AssetID: {ID: run.AssetID, UserID: run.UserID, Availability: availability},
	}}
}

func newReviewTestService(t *testing.T, run *model.DocumentExtractionRun) (*HealthDocumentReviewService, *fakeReviewWriter) {
	t.Helper()
	reader := &fakeReviewRunReader{runs: map[uuid.UUID]*model.DocumentExtractionRun{}}
	if run != nil {
		reader.runs[run.ID] = run
	}
	writer := &fakeReviewWriter{}
	svc := NewHealthDocumentReviewService(reader, writer, reviewAssetForRun(run, model.UserAssetAvailable))
	return svc, writer
}

func TestCurrentReviewContextResolvesLatestOwnedRunAndProjectsSourceRegion(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	older := testExtractionRun(t, uploadID, userID, uuid.New())
	older.CreatedAt = time.Now().Add(-time.Minute)
	latest := testExtractionRun(t, uploadID, userID, uuid.New())
	latest.CreatedAt = time.Now()
	reader := &fakeReviewRunReader{runs: map[uuid.UUID]*model.DocumentExtractionRun{older.ID: older, latest.ID: latest}}
	assets := &fakeReviewAssetReader{assets: map[uuid.UUID]model.UserAsset{
		uploadID: {ID: uploadID, UserID: userID, Availability: model.UserAssetSourceRemoved},
	}}
	svc := NewHealthDocumentReviewService(reader, &fakeReviewWriter{}, assets)

	context, err := svc.CurrentContext(context.Background(), userID, uploadID)
	if err != nil {
		t.Fatalf("CurrentContext: %v", err)
	}
	if context.ExtractionRunID != latest.ID || context.AssetID != uploadID {
		t.Fatalf("context ids=%s/%s want %s/%s", context.ExtractionRunID, context.AssetID, latest.ID, uploadID)
	}
	if context.SourceAvailability != "source_unavailable" {
		t.Fatalf("context source availability=%q want source_unavailable", context.SourceAvailability)
	}
	if len(context.ReviewCandidates) != 1 {
		t.Fatalf("candidates=%d want 1", len(context.ReviewCandidates))
	}
	candidate := context.ReviewCandidates[0].Candidate
	if candidate.EvidenceAdmissibility.Status != "needs_review" || candidate.ReferenceRange != "120-160" {
		t.Fatalf("candidate governance=%+v range=%q", candidate.EvidenceAdmissibility, candidate.ReferenceRange)
	}
	regions := candidate.SourceRegions
	if len(regions) != 1 || regions[0].PageNumber == nil || *regions[0].PageNumber != 1 {
		t.Fatalf("source regions=%+v want page 1", regions)
	}
	if got := regions[0].BBox; len(got) != 4 || got[0] != 1 || got[1] != 2 || got[2] != 3 || got[3] != 4 {
		t.Fatalf("bbox=%v want [1 2 3 4]", got)
	}
}

func TestCurrentReviewContextFailsClosedForMissingOrForeignUpload(t *testing.T) {
	owner := uuid.New()
	foreign := uuid.New()
	uploadID := uuid.New()
	run := testExtractionRun(t, uploadID, owner, uuid.New())
	reader := &fakeReviewRunReader{runs: map[uuid.UUID]*model.DocumentExtractionRun{run.ID: run}}
	svc := NewHealthDocumentReviewService(reader, &fakeReviewWriter{}, reviewAssetForRun(run, model.UserAssetAvailable))

	if _, err := svc.CurrentContext(context.Background(), foreign, uploadID); !errors.Is(err, ErrReviewContextUnavailable) {
		t.Fatalf("foreign err=%v want ErrReviewContextUnavailable", err)
	}
	if _, err := svc.CurrentContext(context.Background(), owner, uuid.New()); !errors.Is(err, ErrReviewContextUnavailable) {
		t.Fatalf("missing err=%v want ErrReviewContextUnavailable", err)
	}
}

func TestApplyReviewConfirmAppendsImmutableSnapshotAndNeverMutatesRun(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, writer := newReviewTestService(t, run)
	req := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionConfirm), SourceRefs: []string{testIndicatorRef},
		IdempotencyKey: "idem-confirm-1",
	}
	record, err := svc.ApplyReview(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("ApplyReview: %v", err)
	}
	if record.Action != string(model.ReviewActionConfirm) {
		t.Fatalf("action=%q", record.Action)
	}
	if record.ReviewerUserID != userID {
		t.Fatalf("reviewer=%s want %s", record.ReviewerUserID, userID)
	}
	if record.IndicatorID != "hemoglobin" {
		t.Fatalf("indicator id=%q", record.IndicatorID)
	}
	// A confirm review always stores the machine candidate snapshot untouched.
	row := writer.rows[0]
	var candidate map[string]any
	if err := json.Unmarshal(row.MachineCandidate, &candidate); err != nil {
		t.Fatalf("machine candidate: %v", err)
	}
	if candidate["indicator_id"] != "hemoglobin" || candidate["value"] != float64(142) {
		t.Fatalf("machine candidate changed: %v", candidate)
	}
	if string(row.SourceRefs) == "" {
		t.Fatal("source refs snapshot missing")
	}
	// The immutable extraction run must remain byte-identical.
	snapshotBefore, _ := json.Marshal(run.IndicatorSnapshot)
	snapshotAfter, _ := json.Marshal(run.IndicatorSnapshot)
	if string(snapshotBefore) != string(snapshotAfter) {
		t.Fatal("extraction run snapshot mutated")
	}
}

func TestApplyReviewIdempotentReplayReturnsExistingRow(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, writer := newReviewTestService(t, run)
	req := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionConfirm), SourceRefs: []string{testIndicatorRef},
		IdempotencyKey: "idem-same",
	}
	first, err := svc.ApplyReview(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("first ApplyReview: %v", err)
	}
	second, err := svc.ApplyReview(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("replay ApplyReview: %v", err)
	}
	if first.ReviewID != second.ReviewID {
		t.Fatal("idempotent replay returned a new row")
	}
	if len(writer.rows) != 1 {
		t.Fatalf("rows=%d want 1", len(writer.rows))
	}
}

func TestApplyReviewDuplicateKeyDifferentContentConflicts(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, _ := newReviewTestService(t, run)
	confirm := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionConfirm), SourceRefs: []string{testIndicatorRef},
		IdempotencyKey: "shared-key",
	}
	if _, err := svc.ApplyReview(context.Background(), userID, confirm); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	reject := confirm
	reject.Action = string(model.ReviewActionReject)
	if _, err := svc.ApplyReview(context.Background(), userID, reject); !errors.Is(err, ErrReviewDuplicateConflict) {
		t.Fatalf("err=%v want ErrReviewDuplicateConflict", err)
	}
}

func TestApplyReviewRejectThenCorrectAppendsBothRows(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, writer := newReviewTestService(t, run)
	reject := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionReject), SourceRefs: []string{testIndicatorRef},
		IdempotencyKey: "idem-reject",
	}
	correct := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionCorrect), SourceRefs: []string{testIndicatorRef},
		IdempotencyKey:  "idem-correct",
		ReviewedPayload: json.RawMessage(`{"indicator_id":"hemoglobin","value":141,"unit":"g/L","name":"corrected"}`),
	}
	if _, err := svc.ApplyReview(context.Background(), userID, reject); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := svc.ApplyReview(context.Background(), userID, correct); err != nil {
		t.Fatalf("correct: %v", err)
	}
	if len(writer.rows) != 2 {
		t.Fatalf("rows=%d want 2 append-only rows", len(writer.rows))
	}
	if writer.rows[0].Action != string(model.ReviewActionReject) || writer.rows[1].Action != string(model.ReviewActionCorrect) {
		t.Fatal("review order lost")
	}
}
