package consultation

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/bodysense/api/internal/service"
	"gorm.io/datatypes"
)

var errInvalidSpatialContext = errors.New("invalid Body Explorer context")

type bodyExplorerMessageMetadata struct {
	BodyExplorerContext *service.ConsultationSpatialContext `json:"body_explorer_context,omitempty"`
}

func normalizeSpatialContextMetadata(raw json.RawMessage) (datatypes.JSON, *service.ConsultationSpatialContext, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return datatypes.JSON(`{}`), nil, nil
	}

	var envelope bodyExplorerMessageMetadata
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, nil, errInvalidSpatialContext
	}
	if envelope.BodyExplorerContext == nil {
		return datatypes.JSON(`{}`), nil, nil
	}

	ctx := *envelope.BodyExplorerContext
	ctx.BodyRegionID = strings.TrimSpace(ctx.BodyRegionID)
	ctx.BodyRegionLabel = strings.TrimSpace(ctx.BodyRegionLabel)
	ctx.AnatomyID = strings.TrimSpace(ctx.AnatomyID)
	ctx.AnatomyName = strings.TrimSpace(ctx.AnatomyName)

	if len(ctx.BodyRegionID) > 80 || len(ctx.BodyRegionLabel) > 120 || len(ctx.AnatomyID) > 240 || len(ctx.AnatomyName) > 240 {
		return nil, nil, errInvalidSpatialContext
	}
	if ctx.BodyRegionID != "" && !service.IsCanonicalBodyRegionID(ctx.BodyRegionID) {
		return nil, nil, errInvalidSpatialContext
	}
	if ctx.BodyRegionID == "" && ctx.AnatomyID == "" {
		return datatypes.JSON(`{}`), nil, nil
	}

	normalized := bodyExplorerMessageMetadata{BodyExplorerContext: &ctx}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, err
	}
	return datatypes.JSON(encoded), &ctx, nil
}
