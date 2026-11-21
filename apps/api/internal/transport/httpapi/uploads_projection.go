package httpapi

import (
	"encoding/json"
	"errors"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
)

func strictUserUpload(upload *model.UserUploadView) (openapiv1.UserUpload, error) {
	if upload == nil {
		return openapiv1.UserUpload{}, errors.New("upload is nil")
	}
	encoded, err := json.Marshal(upload)
	if err != nil {
		return openapiv1.UserUpload{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.UserUpload{}, err
	}
	// The view already hides user_id, availability and storage identity;
	// the deletes keep the guard explicit against future field additions.
	delete(projected, "user_id")
	delete(projected, "availability")
	delete(projected, "agent_configuration_id")
	delete(projected, "storage_backend")
	delete(projected, "storage_key")
	// The owner sees only whether the private source is still there (CD-D4),
	// never the lifecycle state behind it.
	projected["source_availability"] = uploadSourceAvailability(upload.Availability)
	return strictOpenAPIConvert[openapiv1.UserUpload]("UserUpload", projected)
}

// uploadSourceAvailability is the public CD-D4 marker of an asset: a source
// removal or an erasure leaves the source unavailable.
func uploadSourceAvailability(availability model.UserAssetAvailability) string {
	switch availability {
	case model.UserAssetSourceRemoved, model.UserAssetErased:
		return "source_unavailable"
	default:
		return "available"
	}
}
