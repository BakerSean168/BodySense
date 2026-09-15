package service

import (
	"bytes"
	"encoding/json"
	"fmt"

	protovalidate "buf.build/go/protovalidate"
	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var runtimeProtoJSON = protojson.MarshalOptions{UseProtoNames: true}
var runtimeProtoPayloadJSON = protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}

func marshalStartTurnCommand(threadID string, req StartConsultationTurnRequest) ([]byte, error) {
	businessContext, err := consultationBusinessContextToProto(req.BusinessContext)
	if err != nil {
		return nil, err
	}
	images := make([]*runtimev1.ConsultationImageRef, 0, len(req.Input.Images))
	for _, image := range req.Input.Images {
		item := &runtimev1.ConsultationImageRef{DataUrl: image.DataURL}
		if image.UploadID != "" {
			item.UploadId = &image.UploadID
		}
		if image.MimeType != "" {
			item.MimeType = &image.MimeType
		}
		images = append(images, item)
	}
	command := &runtimev1.StartTurnCommand{
		ThreadId:        threadID,
		RunId:           req.RunID,
		ConversationId:  req.ConversationID,
		UserId:          req.UserID,
		ConfigurationId: req.ConfigurationID,
		Input: &runtimev1.ConsultationUserInput{
			Type:   req.Input.Type,
			Text:   req.Input.Text,
			Images: images,
		},
		BusinessContext: businessContext,
	}
	return validateAndMarshalRuntimeCommand(command)
}

func marshalResumeInterruptCommand(
	threadID string,
	interruptID string,
	req ResumeConsultationInterruptRequest,
) ([]byte, error) {
	businessContext, err := consultationBusinessContextToProto(req.BusinessContext)
	if err != nil {
		return nil, err
	}
	answer, err := rawObjectToProto(req.Answer, "answer")
	if err != nil {
		return nil, err
	}
	command := &runtimev1.ResumeInterruptCommand{
		ThreadId:        threadID,
		RunId:           req.RunID,
		ConversationId:  req.ConversationID,
		UserId:          req.UserID,
		ConfigurationId: req.ConfigurationID,
		InterruptId:     interruptID,
		Answer:          answer,
		BusinessContext: businessContext,
	}
	return validateAndMarshalRuntimeCommand(command)
}

func validateAndMarshalRuntimeCommand(message proto.Message) ([]byte, error) {
	if err := protovalidate.Validate(message); err != nil {
		return nil, fmt.Errorf("invalid runtime command: %w", err)
	}
	body, err := runtimeProtoJSON.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("marshal runtime command: %w", err)
	}
	return body, nil
}

func consultationBusinessContextToProto(input ConsultationBusinessContext) (*runtimev1.ConsultationBusinessContext, error) {
	profile, err := rawObjectToProto(input.Profile, "profile")
	if err != nil {
		return nil, err
	}
	bodyState, err := rawObjectToProto(input.BodyState, "body_state")
	if err != nil {
		return nil, err
	}
	extractedInfo, err := rawObjectListToProto(input.RuntimeState.ExtractedInfo, "runtime_state.extracted_info")
	if err != nil {
		return nil, err
	}
	relevantHistory, err := valueListToProto(input.RelevantHistory, "relevant_history")
	if err != nil {
		return nil, err
	}
	currentDiagnosis, err := rawObjectToProto(input.CurrentDiagnosis, "current_diagnosis")
	if err != nil {
		return nil, err
	}
	currentTreatment, err := rawObjectToProto(input.CurrentTreatment, "current_treatment")
	if err != nil {
		return nil, err
	}
	recentOutcomes, err := rawObjectListToProto(input.RecentOutcomes, "recent_outcomes")
	if err != nil {
		return nil, err
	}
	postureAnalysis, err := rawObjectToProto(input.PostureAnalysis, "posture_analysis")
	if err != nil {
		return nil, err
	}

	output := &runtimev1.ConsultationBusinessContext{
		Profile:          profile,
		BodyState:        bodyState,
		RuntimeState:     &runtimev1.ConsultationRuntimeState{Phase: input.RuntimeState.Phase, ExtractedInfo: extractedInfo},
		RelevantHistory:  relevantHistory,
		CurrentDiagnosis: currentDiagnosis,
		CurrentTreatment: currentTreatment,
		RecentOutcomes:   recentOutcomes,
		PostureAnalysis:  postureAnalysis,
	}
	if input.SpatialContext != nil {
		spatial := &runtimev1.ConsultationSpatialContext{}
		if input.SpatialContext.BodyRegionID != "" {
			spatial.BodyRegionId = &input.SpatialContext.BodyRegionID
		}
		if input.SpatialContext.BodyRegionLabel != "" {
			spatial.BodyRegionLabel = &input.SpatialContext.BodyRegionLabel
		}
		if input.SpatialContext.AnatomyID != "" {
			spatial.AnatomyId = &input.SpatialContext.AnatomyID
		}
		if input.SpatialContext.AnatomyName != "" {
			spatial.AnatomyName = &input.SpatialContext.AnatomyName
		}
		output.SpatialContext = spatial
	}
	return output, nil
}

func rawObjectToProto(raw json.RawMessage, field string) (*structpb.Struct, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return &structpb.Struct{}, nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("runtime command %s must be a JSON object: %w", field, err)
	}
	output, err := structpb.NewStruct(value)
	if err != nil {
		return nil, fmt.Errorf("runtime command %s: %w", field, err)
	}
	return output, nil
}

func rawObjectListToProto(raw json.RawMessage, field string) ([]*structpb.Struct, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var values []map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("runtime command %s must be a JSON object array: %w", field, err)
	}
	return mapsToProto(values, field)
}

func valueListToProto[T any](values []T, field string) ([]*structpb.Struct, error) {
	if len(values) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("runtime command %s: %w", field, err)
	}
	return rawObjectListToProto(encoded, field)
}

func mapsToProto(values []map[string]any, field string) ([]*structpb.Struct, error) {
	output := make([]*structpb.Struct, 0, len(values))
	for index, value := range values {
		item, err := structpb.NewStruct(value)
		if err != nil {
			return nil, fmt.Errorf("runtime command %s[%d]: %w", field, index, err)
		}
		output = append(output, item)
	}
	return output, nil
}
