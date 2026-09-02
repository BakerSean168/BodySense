package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func buildDocumentExtractionRun(
	validatedBody []byte,
	documentSHA256 string,
	uploadID uuid.UUID,
	userID uuid.UUID,
	jobID uuid.UUID,
	expectedConfigurationID string,
) (*model.DocumentExtractionRun, error) {
	if len(documentSHA256) != 64 {
		return nil, errors.New("document extraction run requires a SHA256 document identity")
	}
	var envelope struct {
		Status string         `json:"status"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(validatedBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode validated health-document response: %w", err)
	}
	if envelope.Status != "completed" || envelope.Result == nil {
		return nil, errors.New("validated health-document response is incomplete")
	}
	rawText, ok := envelope.Result["raw_text"].(string)
	if !ok {
		return nil, errors.New("validated health-document response missing raw_text")
	}
	mechanism, ok := envelope.Result["mechanism_provenance"].(map[string]any)
	if !ok {
		return nil, errors.New("validated health-document response missing mechanism provenance")
	}
	configurationID, _ := mechanism["configuration_id"].(string)
	if configurationID != expectedConfigurationID {
		return nil, fmt.Errorf("extraction run configuration mismatch: got %q want %q", configurationID, expectedConfigurationID)
	}
	mechanismRevision, _ := mechanism["mechanism_revision"].(string)
	if mechanismRevision == "" {
		return nil, errors.New("extraction run mechanism revision is empty")
	}
	indicators, ok := envelope.Result["indicators"].([]any)
	if !ok {
		return nil, errors.New("validated health-document response missing indicators")
	}
	indicatorSnapshot, err := json.Marshal(indicators)
	if err != nil {
		return nil, fmt.Errorf("encode document indicator snapshot: %w", err)
	}
	sourceSummary, err := json.Marshal(map[string]any{
		"pages":              envelope.Result["pages"],
		"source_blocks":      envelope.Result["source_blocks"],
		"indicator_count":    len(indicators),
		"source_block_count": collectionLength(envelope.Result["source_blocks"]),
	})
	if err != nil {
		return nil, fmt.Errorf("encode document source summary: %w", err)
	}
	mechanismJSON, err := json.Marshal(mechanism)
	if err != nil {
		return nil, fmt.Errorf("encode document mechanism provenance: %w", err)
	}
	resultHash := sha256.Sum256(validatedBody)
	rawHash := sha256.Sum256([]byte(rawText))
	jobCopy := jobID
	return &model.DocumentExtractionRun{
		UploadID:            uploadID,
		UserID:              userID,
		JobID:               &jobCopy,
		ConfigurationID:     configurationID,
		MechanismRevision:   mechanismRevision,
		DocumentSHA256:      documentSHA256,
		ResultSHA256:        hex.EncodeToString(resultHash[:]),
		RawTextSHA256:       hex.EncodeToString(rawHash[:]),
		IndicatorSnapshot:   datatypes.JSON(indicatorSnapshot),
		SourceSummary:       datatypes.JSON(sourceSummary),
		MechanismProvenance: datatypes.JSON(mechanismJSON),
	}, nil
}

func collectionLength(value any) int {
	if items, ok := value.([]any); ok {
		return len(items)
	}
	return 0
}
