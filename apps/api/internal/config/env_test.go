package config

import "testing"

func TestEnvOr(t *testing.T) {
	t.Run("unset returns fallback", func(t *testing.T) {
		if got := EnvOr("BODYSENSE_TEST_ENVOR_UNSET", "fallback"); got != "fallback" {
			t.Fatalf("EnvOr unset = %q, want %q", got, "fallback")
		}
	})
	t.Run("blank returns fallback", func(t *testing.T) {
		t.Setenv("BODYSENSE_TEST_ENVOR_BLANK", "  ")
		if got := EnvOr("BODYSENSE_TEST_ENVOR_BLANK", "fallback"); got != "fallback" {
			t.Fatalf("EnvOr blank = %q, want %q", got, "fallback")
		}
	})
	t.Run("set value is trimmed", func(t *testing.T) {
		t.Setenv("BODYSENSE_TEST_ENVOR_SET", "  value  ")
		if got := EnvOr("BODYSENSE_TEST_ENVOR_SET", "fallback"); got != "value" {
			t.Fatalf("EnvOr set = %q, want %q", got, "value")
		}
	})
}
