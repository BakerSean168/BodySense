package consultation

import (
	"github.com/bodysense/api/internal/service"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeNoLongerOwnsPerRunPendingAgentConfiguration(t *testing.T) {
	typeOfRuntime := reflect.TypeOf(Runtime{})
	for _, fieldName := range []string{
		"pendingAgentConfigurationID",
		"pendingAgentConfiguration",
		"pendingExecutionProvenance",
	} {
		if _, found := typeOfRuntime.FieldByName(fieldName); found {
			t.Fatalf("service-global per-run field %s must not exist on Runtime", fieldName)
		}
	}
}

func TestValidateConsultationExecutionIdentity(t *testing.T) {
	state := testStreamState()
	state.ExpectedConfigurationID = "consult-config-7feb8ca2d5bfad5a"
	event := validConsultationHandshake(t, state)

	identity, err := validateConsultationExecutionIdentity(event, state.ExpectedConfigurationID)
	if err != nil {
		t.Fatalf("valid handshake rejected: %v", err)
	}
	if identity.ConfigurationID != state.ExpectedConfigurationID {
		t.Fatalf("unexpected configuration id: %s", identity.ConfigurationID)
	}
}

func TestValidateConsultationExecutionIdentityRejectsMismatch(t *testing.T) {
	state := testStreamState()
	state.ExpectedConfigurationID = "consult-config-7feb8ca2d5bfad5a"

	tests := []struct {
		name       string
		mutate     func(map[string]any, map[string]any)
		wantErrSub string
	}{
		{
			name: "configuration id",
			mutate: func(config, provenance map[string]any) {
				config["id"] = "consult-config-wrong"
			},
			wantErrSub: "configuration id",
		},
		{
			name: "role",
			mutate: func(config, provenance map[string]any) {
				config["role"] = "diagnosis"
			},
			wantErrSub: "role",
		},
		{
			name: "decision policy",
			mutate: func(config, provenance map[string]any) {
				config["decision_policy_revision"] = "wrong-policy"
			},
			wantErrSub: "decision policy",
		},
		{
			name: "configuration logical model",
			mutate: func(config, provenance map[string]any) {
				config["logical_model"] = "wrong-model"
			},
			wantErrSub: "logical model",
		},
		{
			name: "execution logical model",
			mutate: func(config, provenance map[string]any) {
				provenance["logical_model"] = "wrong-model"
			},
			wantErrSub: "execution logical model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configuration := map[string]any{
				"id":                       state.ExpectedConfigurationID,
				"role":                     "consultation",
				"decision_policy_revision": service.ConsultationDecisionPolicyV2,
				"logical_model":            "bodysense-consultation",
			}
			provenance := map[string]any{
				"runtime":       "langgraph",
				"logical_model": "bodysense-consultation",
			}
			tt.mutate(configuration, provenance)
			event := privateRuntimeEvent(t, 1, service.ConsultationRuntimeAgentConfiguration, state, map[string]any{
				"agent_configuration":  configuration,
				"execution_provenance": provenance,
			})
			_, err := validateConsultationExecutionIdentity(event, state.ExpectedConfigurationID)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Fatalf("expected %q error, got %v", tt.wantErrSub, err)
			}
		})
	}
}
