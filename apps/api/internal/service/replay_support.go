package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func replayAgentExecuted(raw json.RawMessage) bool {
	var execution struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(raw, &execution)
	return execution.Status == "executed"
}

// sanitizeRegressionReplayJSON pseudonymizes a frozen replay input before it is
// exported as a regression case: user ids are replaced and direct profile
// identifiers are dropped.
func sanitizeRegressionReplayJSON(raw json.RawMessage, profileRoot bool) json.RawMessage {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return raw
	}
	var walk func(any, bool) any
	walk = func(current any, isProfileRoot bool) any {
		switch typed := current.(type) {
		case map[string]any:
			result := make(map[string]any, len(typed))
			for key, item := range typed {
				normalized := strings.ToLower(strings.TrimSpace(key))
				if normalized == "user_id" {
					result[key] = "historical-regression"
					continue
				}
				if isProfileRoot {
					switch normalized {
					case "id", "email", "phone", "phone_number", "full_name", "display_name", "nickname", "avatar_url":
						continue
					}
				}
				result[key] = walk(item, false)
			}
			return result
		case []any:
			result := make([]any, 0, len(typed))
			for _, item := range typed {
				result = append(result, walk(item, false))
			}
			return result
		default:
			return current
		}
	}
	sanitized, _ := json.Marshal(walk(value, profileRoot))
	return json.RawMessage(sanitized)
}
