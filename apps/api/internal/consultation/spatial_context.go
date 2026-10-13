package consultation

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/bodysense/api/internal/service"
	"gorm.io/datatypes"
)

var errInvalidSpatialContext = errors.New("invalid Body Explorer context")

var allowedReferenceMotions = map[string]string{
	"stand": "站立", "run": "跑步", "jump": "跳跃", "sit": "坐姿",
	"arm_raise": "抬臂观察", "calf_raise": "提踵", "hip_hinge": "髋铰链", "bridge": "臀桥",
}

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

	normalizedRegions := make([]string, 0, len(ctx.BodyRegionIDs)+1)
	seenRegions := make(map[string]struct{}, len(ctx.BodyRegionIDs)+1)
	addRegion := func(value string) error {
		regionID := strings.TrimSpace(value)
		if regionID == "" {
			return nil
		}
		if len(regionID) > 80 || !service.IsCanonicalBodyRegionID(regionID) {
			return errInvalidSpatialContext
		}
		if _, exists := seenRegions[regionID]; exists {
			return nil
		}
		seenRegions[regionID] = struct{}{}
		normalizedRegions = append(normalizedRegions, regionID)
		return nil
	}
	if err := addRegion(ctx.BodyRegionID); err != nil {
		return nil, nil, err
	}
	for _, regionID := range ctx.BodyRegionIDs {
		if err := addRegion(regionID); err != nil {
			return nil, nil, err
		}
	}
	if len(normalizedRegions) > 35 {
		return nil, nil, errInvalidSpatialContext
	}
	ctx.BodyRegionIDs = normalizedRegions
	if ctx.BodyRegionID == "" && len(normalizedRegions) > 0 {
		ctx.BodyRegionID = normalizedRegions[0]
	}

	if ctx.ReferenceMotion != nil {
		motion := *ctx.ReferenceMotion
		motion.ID = strings.TrimSpace(motion.ID)
		motion.Label = strings.TrimSpace(motion.Label)
		motion.Source = strings.TrimSpace(motion.Source)
		canonicalLabel, ok := allowedReferenceMotions[motion.ID]
		if !ok ||
			motion.Source != "reference_animation" ||
			math.IsNaN(motion.Phase) || math.IsInf(motion.Phase, 0) ||
			motion.Phase < 0 || motion.Phase > 1 {
			return nil, nil, errInvalidSpatialContext
		}
		motion.Label = canonicalLabel
		ctx.ReferenceMotion = &motion
	}

	if ctx.BodyRegionID == "" && ctx.AnatomyID == "" && ctx.ReferenceMotion == nil {
		return datatypes.JSON(`{}`), nil, nil
	}

	normalized := bodyExplorerMessageMetadata{BodyExplorerContext: &ctx}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, err
	}
	return datatypes.JSON(encoded), &ctx, nil
}
