package main

import (
	"strings"
	"testing"
)

func TestOpenAPIAllowedInHTTPTransport(t *testing.T) {
	source := []byte(`package httpapi
import openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
var _ = openapiv1.GetSpec
`)
	violations := analyzeGoSource("apps/api/internal/transport/httpapi/example.go", source)
	if len(violations) != 0 {
		t.Fatalf("unexpected violations: %v", violations)
	}
}

func TestOpenAPIFailsWhenItLeaksIntoDomainService(t *testing.T) {
	source := []byte(`package service
import openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
var _ = openapiv1.GetSpec
`)
	violations := analyzeGoSource("apps/api/internal/service/leaky.go", source)
	if len(violations) != 1 || !strings.Contains(violations[0], "generated OpenAPI types leaked") {
		t.Fatalf("expected generated OpenAPI leak violation, got %v", violations)
	}
}

func TestRuntimeProtoAllowedInApprovedAdapter(t *testing.T) {
	source := []byte(`package service
import runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
var _ = runtimev1.RuntimeEvent{}
`)
	violations := analyzeGoSource("apps/api/internal/service/runtime_proto_adapter.go", source)
	if len(violations) != 0 {
		t.Fatalf("unexpected violations: %v", violations)
	}
}

func TestRuntimeProtoFailsWhenItLeaksIntoAnotherService(t *testing.T) {
	source := []byte(`package service
import runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
var _ = runtimev1.RuntimeEvent{}
`)
	violations := analyzeGoSource("apps/api/internal/service/leaky.go", source)
	if len(violations) != 1 || !strings.Contains(violations[0], "generated runtime Proto leaked") {
		t.Fatalf("expected runtime Proto leak violation, got %v", violations)
	}
}

func TestDomainPackageCannotImportHTTPTransport(t *testing.T) {
	source := []byte(`package model
import "github.com/bodysense/api/internal/transport/httpapi"
var _ = httpapi.RegisterRoutes
`)
	violations := analyzeGoSource("apps/api/internal/model/leaky.go", source)
	if len(violations) != 1 || !strings.Contains(violations[0], "domain package imports HTTP transport") {
		t.Fatalf("expected domain->transport violation, got %v", violations)
	}
}
