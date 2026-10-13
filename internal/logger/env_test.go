package logger

import (
	"os"
	"testing"
)

// --- TDD: ReadEnv reads the variables ---

func TestReadEnv_UnsetReturnsEmpty(t *testing.T) {
	// Ensure the variables are unset for this test.
	t.Setenv(EnvVar, "")
	t.Setenv(EnvVarFormat, "")

	lvl, format := ReadEnv()
	if lvl != "" {
		t.Fatalf("level = %q, want empty", lvl)
	}
	if format != "" {
		t.Fatalf("format = %q, want empty", format)
	}
}

func TestReadEnv_ReadsValues(t *testing.T) {
	t.Setenv(EnvVar, "debug")
	t.Setenv(EnvVarFormat, "json")

	lvl, format := ReadEnv()
	if lvl != "debug" {
		t.Fatalf("level = %q, want debug", lvl)
	}
	if format != "json" {
		t.Fatalf("format = %q, want json", format)
	}
}

// --- ATDD: FORGE_LOG_LEVEL feeds ResolveLevel ---

func TestReadEnv_FeedsResolveLevel(t *testing.T) {
	t.Setenv(EnvVar, "warn")

	lvl, _ := ReadEnv()
	got, err := ResolveLevel(Inputs{EnvLevel: lvl})
	if err != nil {
		t.Fatalf("ResolveLevel: %v", err)
	}
	if got.Level != LevelWarn {
		t.Fatalf("Level = %v, want WARN", got.Level)
	}
	if got.Source != SourceEnv {
		t.Fatalf("Source = %v, want env", got.Source)
	}
}

// --- Corner: whitespace and case in the env var ---

func TestReadEnv_PaddedValue(t *testing.T) {
	t.Setenv(EnvVar, "  DEBUG  ")

	lvl, _ := ReadEnv()
	got, err := ResolveLevel(Inputs{EnvLevel: lvl})
	if err != nil {
		t.Fatalf("ResolveLevel: %v", err)
	}
	if got.Level != LevelDebug {
		t.Fatalf("Level = %v, want DEBUG", got.Level)
	}
}

// --- Negative: invalid env value surfaces as an error ---

func TestReadEnv_InvalidValue(t *testing.T) {
	t.Setenv(EnvVar, "loud")

	lvl, _ := ReadEnv()
	_, err := ResolveLevel(Inputs{EnvLevel: lvl})
	if err == nil {
		t.Fatal("expected error for invalid FORGE_LOG_LEVEL")
	}
}

// --- Safety: ReadEnv does not mutate the environment ---

func TestReadEnv_DoesNotMutateEnv(t *testing.T) {
	t.Setenv(EnvVar, "info")
	before := os.Getenv(EnvVar)
	_, _ = ReadEnv()
	after := os.Getenv(EnvVar)
	if before != after {
		t.Fatalf("ReadEnv mutated env: %q -> %q", before, after)
	}
}
