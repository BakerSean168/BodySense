package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"testing"
	"time"
)

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

func TestApplyReviewCorrectionCannotChangeCandidateIdentity(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	runID := uuid.New()
	run := testExtractionRun(t, uploadID, userID, runID)
	svc, writer := newReviewTestService(t, run)
	req := ReviewRequest{
		ExtractionRunID: runID, IndicatorIndex: 0, IndicatorID: "hemoglobin",
		Action: string(model.ReviewActionCorrect), SourceRefs: []string{testIndicatorRef},
		ReviewedPayload: json.RawMessage(`{"indicator_id":"glucose","value":6.2,"unit":"mmol/L"}`),
		IdempotencyKey:  "cross-indicator-correction",
	}
	if _, err := svc.ApplyReview(context.Background(), userID, req); !errors.Is(err, ErrReviewCandidateMismatch) {
		t.Fatalf("err=%v want ErrReviewCandidateMismatch", err)
	}
	if len(writer.rows) != 0 {
		t.Fatalf("cross-indicator correction appended rows: %d", len(writer.rows))
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
	if err := svc.EnsureAssetOwnsRun(context.Background(), userID, uploadID, runID); err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}
	if err := svc.EnsureAssetOwnsRun(context.Background(), userID, otherUpload, runID); !errors.Is(err, ErrReviewCandidateMismatch) {
		t.Fatalf("cross-upload binding err=%v want mismatch", err)
	}
}
func TestReviewSourceRegionsAccepts4And8NumberBBoxes(t *testing.T) {
	cases := []struct {
		name    string
		bbox    []any
		want    []float64
		wantErr bool
	}{
		{
			name: "axis aligned 4-number rectangle",
			bbox: []any{float64(1), float64(2), float64(3), float64(4)},
			want: []float64{1, 2, 3, 4},
		},
		{
			name: "8-number quadrilateral preserved",
			bbox: []any{float64(0), float64(0), float64(10), float64(0), float64(10), float64(10), float64(0), float64(10)},
			want: []float64{0, 0, 10, 0, 10, 10, 0, 10},
		},
		{
			name:    "9-number bbox fails closed",
			bbox:    []any{0, 0, 1, 1, 2, 2, 3, 3, 4},
			wantErr: true,
		},
		{
			name:    "non-numeric bbox fails closed",
			bbox:    []any{"x", "y", 3, 4},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := map[string]map[string]any{
				testIndicatorRef: {"source_ref": testIndicatorRef, "page_number": float64(1), "bbox": tc.bbox},
			}
			regions, err := reviewSourceRegions(blocks, []string{testIndicatorRef})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for bbox %v", tc.bbox)
				}
				return
			}
			if err != nil {
				t.Fatalf("reviewSourceRegions: %v", err)
			}
			if len(regions) != 1 {
				t.Fatalf("regions=%d want 1", len(regions))
			}
			got := regions[0].BBox
			if len(got) != len(tc.want) {
				t.Fatalf("bbox=%v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("bbox=%v want %v", got, tc.want)
				}
			}
		})
	}
}
