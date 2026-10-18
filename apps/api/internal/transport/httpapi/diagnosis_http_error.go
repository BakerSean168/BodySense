package httpapi

import (
	"encoding/json"
	"net/http"
)

// diagnosisHTTPErrorResponse is the shared error envelope of the native
// Diagnosis endpoints.
type diagnosisHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

func (r diagnosisHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}

func (r diagnosisHTTPErrorResponse) VisitGetNativeDiagnosisResponse(w http.ResponseWriter) error {
	return r.write(w)
}
