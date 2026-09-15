package httpapi

import (
	"net/http"
	"testing"
)

func TestLifestyleOpenAPIRequiresExpectedRevisionBeforeAdapter(t *testing.T) {
	svc := &fakeBodyStateFactService{}
	rec := performJSONAt(
		newOpenAPITestRouter(t, svc),
		http.MethodPut,
		"/api/v1/lifestyle",
		`{"activity":{"summary":"walks daily"}}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestBodyMetricsOpenAPIRejectsOutOfRangeHeightBeforeAdapter(t *testing.T) {
	svc := &fakeBodyStateFactService{}
	rec := performJSONAt(
		newOpenAPITestRouter(t, svc),
		http.MethodPut,
		"/api/v1/body-metrics",
		`{"expected_revision":4,"height_cm":300}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}

func TestOnboardingOpenAPIRequiresInitialBodyStateRevision(t *testing.T) {
	svc := &fakeBodyStateFactService{}
	rec := performJSONAt(
		newOpenAPITestRouter(t, svc),
		http.MethodPut,
		"/api/v1/onboarding/context",
		`{"profile":{"gender":"male","birth_date":"2004-01-01"},"body_metrics":{"height_cm":175,"weight_kg":65},"lifestyle":{"activity":{"summary":""},"sleep":{"summary":""},"exercise":{"summary":""},"nutrition":{"summary":""},"substances":{"summary":""},"recovery":{"summary":""}},"injury_history":""}`,
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=400 body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_REQUEST")
}
