package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/getkin/kin-openapi/openapi3"
)

var (
	publicContractOnce sync.Once
	publicContractSpec *openapi3.T
	publicContractErr  error
)

// strictJSONConvert is a boundary presenter helper for application read models
// whose public JSON shape is intentionally identical to a generated OpenAPI type.
// Unknown application fields fail closed instead of being silently dropped.
func strictJSONConvert[T any](source any) (T, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		var target T
		return target, err
	}
	return strictJSONDecode[T](encoded)
}

// strictOpenAPIConvert adds full schema validation (required fields, enums,
// bounds, array cardinality, etc.) before decoding into a generated Go type.
// Use it for application read models that contain Raw JSON or unions, where Go
// decoding alone can otherwise fill missing nested fields with zero values.
func strictOpenAPIConvert[T any](component string, source any) (T, error) {
	var target T
	encoded, err := json.Marshal(source)
	if err != nil {
		return target, err
	}
	if err := validateOpenAPIComponentJSON(component, encoded); err != nil {
		return target, err
	}
	return strictJSONDecode[T](encoded)
}

func strictJSONDecode[T any](encoded []byte) (T, error) {
	var target T
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&target); err != nil {
		return target, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return target, fmt.Errorf("unexpected trailing JSON value")
		}
		return target, err
	}
	return target, nil
}

func validateOpenAPIComponentJSON(component string, encoded []byte) error {
	publicContractOnce.Do(func() {
		publicContractSpec, publicContractErr = openapiv1.GetSwagger()
	})
	if publicContractErr != nil {
		return fmt.Errorf("load public OpenAPI contract: %w", publicContractErr)
	}
	if publicContractSpec == nil {
		return fmt.Errorf("public OpenAPI contract is unavailable")
	}
	schemaRef, ok := publicContractSpec.Components.Schemas[component]
	if !ok || schemaRef == nil || schemaRef.Value == nil {
		return fmt.Errorf("OpenAPI component %q is unavailable", component)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return fmt.Errorf("decode %s for schema validation: %w", component, err)
	}
	if err := schemaRef.Value.VisitJSON(value); err != nil {
		return fmt.Errorf("%s schema validation failed: %w", component, err)
	}
	return nil
}
