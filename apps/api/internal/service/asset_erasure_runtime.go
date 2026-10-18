package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type assetRuntimeErasureRequest struct {
	UserID            string   `json:"user_id"`
	ContentSHA256     string   `json:"content_sha256"`
	ThreadIDs         []string `json:"thread_ids"`
	AttachedThreadIDs []string `json:"attached_thread_ids"`
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// EraseAssetRuntimeCopies asks the ai-service to erase one asset from the
// owner's consultation threads (POST /runtime/assets/{asset}/erasure) and
// returns its per-thread receipt read back from checkpoint storage.
func (c *AIClient) EraseAssetRuntimeCopies(ctx context.Context, userID, assetID uuid.UUID, scope model.AssetRuntimeScope) (*model.RuntimeErasureReceipt, error) {
	raw, err := c.callJSON(ctx, "/runtime/assets/"+assetID.String()+"/erasure", assetRuntimeErasureRequest{
		UserID:            userID.String(),
		ContentSHA256:     scope.ContentSHA256,
		ThreadIDs:         uuidStrings(scope.ThreadIDs),
		AttachedThreadIDs: uuidStrings(scope.AttachedThreadIDs),
	})
	if err != nil {
		return nil, err
	}
	var receipt model.RuntimeErasureReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return nil, fmt.Errorf("decode asset runtime erasure receipt: %w", err)
	}
	return &receipt, nil
}
