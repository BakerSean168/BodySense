package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// strictJSONConvert is a boundary presenter helper for application read models
// whose public JSON shape is intentionally identical to a generated OpenAPI type.
// Unknown application fields fail closed instead of being silently dropped.
func strictJSONConvert[T any](source any) (T, error) {
	var target T
	encoded, err := json.Marshal(source)
	if err != nil {
		return target, err
	}
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
