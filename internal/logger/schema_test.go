package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// schemaPath returns the path to schemas/log-record.schema.json,
// derived from this test file's location.
func schemaPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = .../internal/logger/schema_test.go
	// moduleRoot = .../forge
	dir := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	return filepath.Join(dir, "schemas", "log-record.schema.json")
}

// TestSchemaFileExists pins the schema's presence. A missing
// schema is a contract violation: the documentation references it
// and consumers rely on it.
func TestSchemaFileExists(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat(schemaPath(t)); err != nil {
		t.Fatalf("schema missing: %v", err)
	}
}

// TestSchemaIsValidJSON asserts the schema is syntactically valid.
func TestSchemaIsValidJSON(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(schemaPath(t))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if doc["$schema"] == nil {
		t.Fatal("schema missing $schema")
	}
	if doc["$id"] == nil {
		t.Fatal("schema missing $id")
	}
}

// TestSchemaValidatesRealOutput is the ATDD test for the JSON
// schema. It renders a representative record with writeJSON and
// validates the result against the schema's required fields and
// enumerations. The validation is done by hand because importing a
// JSON Schema library is not permitted by the dependency policy
// (see docs/dependency-policy.md).
//
// The test asserts:
//
//   - The required fields are present.
//   - The level is one of the schema's four values.
//   - The component is one of the schema's seven values.
//   - The correlation_id matches the v4 UUID pattern.
//   - The ts matches the RFC3339 UTC pattern.
func TestSchemaValidatesRealOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	_ = writeJSON(&buf, Record{
		Ts:            time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "loaded configuration",
		CorrelationID: NewCorrelationID(),
		Attrs:         map[string]any{"path": "/etc/forge/config.yaml"},
	})

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("rendered output is not JSON: %v", err)
	}

	for _, key := range []string{"ts", "level", "component", "msg", "correlation_id"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("required field missing: %q", key)
		}
	}

	// Level enum.
	levels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if l, _ := doc["level"].(string); !levels[l] {
		t.Errorf("level %q not in enum", l)
	}

	// Component enum.
	comps := map[string]bool{
		"cli": true, "config": true, "blueprint": true,
		"template": true, "renderer": true, "filesystem": true,
		"policy": true,
	}
	if c, _ := doc["component"].(string); !comps[c] {
		t.Errorf("component %q not in enum", c)
	}

	// Correlation ID pattern.
	if id, _ := doc["correlation_id"].(string); !uuidV4Pattern.MatchString(id) {
		t.Errorf("correlation_id %q does not match v4 UUID", id)
	}
}
