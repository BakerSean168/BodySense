package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AssessmentReplayInput freezes the durable inputs that produced one assessment
// report so historical/counterfactual replay can reproduce the report without
// substituting today's BodyState. Images are stored as sanitized descriptors
// (media-type + count), never raw base64, to keep the replay envelope private
// and lightweight.
type AssessmentReplayInput struct {
	ConfigurationID        string            `json:"configuration_id"`
	Profile                json.RawMessage   `json:"profile"`
	BodyState              json.RawMessage   `json:"body_state"`
	ReportIndicators       json.RawMessage   `json:"report_indicators"`
	ReviewedReportEvidence json.RawMessage   `json:"reviewed_report_evidence"`
	PostureAnalysis        json.RawMessage   `json:"posture_analysis"`
	Images                 []imageDescriptor `json:"images"`
}

type imageDescriptor struct {
	MediaType string `json:"media_type"`
}

func encodeAssessmentReplayInput(
	configurationID string,
	profile json.RawMessage,
	bodyState json.RawMessage,
	reportIndicators json.RawMessage,
	reviewedReportEvidence json.RawMessage,
	posture json.RawMessage,
	images []string,
) (json.RawMessage, error) {
	if len(profile) == 0 || !json.Valid(profile) {
		profile = json.RawMessage(`{}`)
	}
	if len(bodyState) == 0 || !json.Valid(bodyState) {
		bodyState = json.RawMessage(`{}`)
	}
	if len(reportIndicators) == 0 || !json.Valid(reportIndicators) {
		reportIndicators = json.RawMessage(`[]`)
	}
	if len(reviewedReportEvidence) == 0 || !json.Valid(reviewedReportEvidence) {
		reviewedReportEvidence = json.RawMessage(`[]`)
	}
	if len(posture) == 0 || !json.Valid(posture) {
		posture = json.RawMessage(`{}`)
	}
	descriptors := make([]imageDescriptor, 0, len(images))
	for _, image := range images {
		mediaType := "image/*"
		if start := strings.Index(image, "data:"); start == 0 {
			if semi := strings.Index(image[start:], ";"); semi > 0 {
				mediaType = image[start+5 : start+semi]
			}
		}
		descriptors = append(descriptors, imageDescriptor{MediaType: mediaType})
	}
	return json.Marshal(AssessmentReplayInput{
		ConfigurationID:        configurationID,
		Profile:                profile,
		BodyState:              bodyState,
		ReportIndicators:       reportIndicators,
		ReviewedReportEvidence: reviewedReportEvidence,
		PostureAnalysis:        posture,
		Images:                 descriptors,
	})
}

func decodeAssessmentReplayInput(raw json.RawMessage) (AssessmentReplayInput, error) {
	var input AssessmentReplayInput
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return input, ErrAssessmentReplayUnavailable
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return input, fmt.Errorf("decode Assessment replay input: %w", err)
	}
	if len(input.Profile) == 0 || !json.Valid(input.Profile) {
		return input, ErrAssessmentReplayUnavailable
	}
	return input, nil
}
