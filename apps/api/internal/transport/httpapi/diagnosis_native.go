package httpapi

import (
	"context"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type nativeDiagnosisReadApplication interface {
	Read(context.Context, uuid.UUID) (*service.NativeDiagnosisView, error)
}

// WithNativeDiagnosis wires only Go-owned committed Diagnosis read models.
// Legacy per-run analysis routes must never be used as this authority.
func (s *PublicServer) WithNativeDiagnosis(reader nativeDiagnosisReadApplication) *PublicServer {
	s.nativeDiagnosis = reader
	return s
}

func (s *PublicServer) GetNativeDiagnosis(ctx context.Context, _ openapiv1.GetNativeDiagnosisRequestObject) (openapiv1.GetNativeDiagnosisResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if s.nativeDiagnosis == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis read unavailable"}, nil
	}
	view, err := s.nativeDiagnosis.Read(applicationRequestContext(ctx), owner)
	if err != nil || view == nil || view.UserID != owner || view.CurrentRevision < 0 {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis read unavailable"}, nil
	}
	for _, condition := range view.ConditionEntries {
		if condition.ConditionID == uuid.Nil || condition.CommittedRevision <= 0 || condition.CommittedRevision > view.CurrentRevision ||
			condition.LatestDecisionID == uuid.Nil || condition.MedicalName == "" {
			return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis projection inconsistent"}, nil
		}
	}
	converted, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisProjection]("NativeDiagnosisProjection", view)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis projection unavailable"}, nil
	}
	return openapiv1.GetNativeDiagnosis200JSONResponse(converted), nil
}
