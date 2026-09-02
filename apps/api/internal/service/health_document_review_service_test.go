package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const testIndicatorRef = "page:1:ocr-block:5"

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
		"indicator_id": "hemoglobin", "name": "hb", "value": float64(142), "unit": "g/L",
		"parser_revision": "parser-v1", "source_refs": []any{testIndicatorRef},
		"evidence_admissibility": map[string]any{"status": "admissible", "policy_revision": "pol-v1"},
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
		UploadID:          uploadID,
		UserID:            userID,
		ConfigurationID:   "c1",
		MechanismRevision: "m1",
		DocumentSHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ResultSHA256:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RawTextSHA256:     "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
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
func (f *fakeReviewWriter) ListByUpload(_ context.Context, uploadID, userID uuid.UUID) ([]model.DocumentIndicatorReview, error) {
	var out []model.DocumentIndicatorReview
	for _, r := range f.rows {
		if r.UploadID == uploadID && r.UserID == userID {
			out = append(out, r)
		}
	}
	return out, nil
}

func newReviewTestService(t *testing.T, run *model.DocumentExtractionRun) (*HealthDocumentReviewService, *fakeReviewWriter) {
	t.Helper()
	reader := &fakeReviewRunReader{runs: map[uuid.UUID]*model.DocumentExtractionRun{}}
	if run != nil {
		reader.runs[run.ID] = run
	}
	writer := &fakeReviewWriter{}
	svc := NewHealthDocumentReviewService(reader, writer)
	return svc, writer
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

func TestApplyReviewMalformedCorrectionFailsClosed(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, writer := newReviewTestService(t, run)
	cases := []ReviewRequest{
		{ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin", Action: string(model.ReviewActionCorrect),
			IdempotencyKey: "bad-1"},
		{ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin", Action: string(model.ReviewActionCorrect),
			ReviewedPayload: json.RawMessage(`not-json`), IdempotencyKey: "bad-2"},
		{ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin", Action: string(model.ReviewActionCorrect),
			ReviewedPayload: json.RawMessage(`{"indicator_id":""}`), IdempotencyKey: "bad-3"},
	}
	for i, req := range cases {
		if _, err := svc.ApplyReview(context.Background(), userID, req); !errors.Is(err, ErrReviewValidation) {
			t.Fatalf("case %d err=%v want ErrReviewValidation", i, err)
		}
	}
	if len(writer.rows) != 0 {
		t.Fatalf("malformed correction appended a row: %d", len(writer.rows))
	}
}

func TestApplyReviewFabricatedSourceRefOrIdentityFailsClosed(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, writer := newReviewTestService(t, run)
	identity := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "glucose",
		Action: string(model.ReviewActionConfirm), SourceRefs: []string{testIndicatorRef}, IdempotencyKey: "stale-identity",
	}
	if _, err := svc.ApplyReview(context.Background(), userID, identity); !errors.Is(err, ErrReviewCandidateMismatch) {
		t.Fatalf("identity err=%v want mismatch", err)
	}
	fabricated := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionConfirm), SourceRefs: []string{"page:9:ocr-block:999"}, IdempotencyKey: "fabricated-ref",
	}
	if _, err := svc.ApplyReview(context.Background(), userID, fabricated); !errors.Is(err, ErrReviewCandidateMismatch) {
		t.Fatalf("fabricated ref err=%v want mismatch", err)
	}
	if len(writer.rows) != 0 {
		t.Fatalf("stale/fabricated submissions appended rows: %d", len(writer.rows))
	}
}

func TestApplyReviewEnforcesOwnership(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, writer := newReviewTestService(t, run)
	other := uuid.New()
	req := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionConfirm), SourceRefs: []string{testIndicatorRef}, IdempotencyKey: "other-user",
	}
	if _, err := svc.ApplyReview(context.Background(), other, req); !errors.Is(err, ErrReviewAccessDenied) {
		t.Fatalf("err=%v want access denied", err)
	}
	if len(writer.rows) != 0 {
		t.Fatal("non-owner appended a review")
	}
}

func TestListCandidatesProjectsEffectiveLatestReview(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, _ := newReviewTestService(t, run)
	base := time.Now()
	svc.now = func() time.Time { return base }
	reject := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionReject), SourceRefs: []string{testIndicatorRef}, IdempotencyKey: "proj-reject",
	}
	correct := reject
	correct.Action = string(model.ReviewActionCorrect)
	correct.IdempotencyKey = "proj-correct"
	correct.ReviewedPayload = json.RawMessage(`{"indicator_id":"hemoglobin","value":130,"name":"fixed","unit":"g/L"}`)
	if _, err := svc.ApplyReview(context.Background(), userID, reject); err != nil {
		t.Fatalf("reject: %v", err)
	}
	svc.now = func() time.Time { return base.Add(time.Minute) }
	if _, err := svc.ApplyReview(context.Background(), userID, correct); err != nil {
		t.Fatalf("correct: %v", err)
	}
	projection, err := svc.ListCandidates(context.Background(), userID, runID)
	if err != nil {
		t.Fatalf("ListCandidates: %v", err)
	}
	if len(projection) != 1 {
		t.Fatalf("candidates=%d want 1", len(projection))
	}
	if projection[0].EffectiveReview == nil || projection[0].EffectiveReview.Action != string(model.ReviewActionCorrect) {
		t.Fatalf("effective review=%+v want correct", projection[0].EffectiveReview)
	}
	if len(projection[0].History) != 2 {
		t.Fatalf("history=%d want 2", len(projection[0].History))
	}
	if projection[0].Candidate.SourceRefs[0] != testIndicatorRef {
		t.Fatalf("candidate refs=%v", projection[0].Candidate.SourceRefs)
	}
	if projection[0].Candidate.Value != float64(142) {
		t.Fatalf("candidate value mutated: %v", projection[0].Candidate.Value)
	}
}

func TestEnsureUploadOwnsRunRejectsCrossUploadBinding(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	otherUpload := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, _ := newReviewTestService(t, run)
	if err := svc.EnsureUploadOwnsRun(context.Background(), userID, uploadID, runID); err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}
	if err := svc.EnsureUploadOwnsRun(context.Background(), userID, otherUpload, runID); !errors.Is(err, ErrReviewCandidateMismatch) {
		t.Fatalf("cross-upload binding err=%v want mismatch", err)
	}
}
