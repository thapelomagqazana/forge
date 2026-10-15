package platform_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/thapelomagqazana/forge/internal/platform"
	"gopkg.in/yaml.v3"
)

// TestSchema_PlatformsYAMLValidates asserts docs/reference/platforms.yaml
// satisfies schemas/platforms.schema.json.
//
// This is the bridge between the Go validator in platforms.go and the
// JSON Schema used by non-Go tooling. If the two ever disagree, this test
// fails.
func TestSchema_PlatformsYAMLValidates(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	schemaPath := filepath.Join(root, "schemas", "platforms.schema.json")
	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	yamlPath := filepath.Join(root, "docs", "reference", "platforms.yaml")
	doc, err := loadYAMLAsJSON(yamlPath)
	if err != nil {
		t.Fatalf("load yaml: %v", err)
	}

	if err := sch.Validate(doc); err != nil {
		t.Fatalf("platforms.yaml does not validate: %v", err)
	}
}

// TestSchema_RejectsBadDocuments asserts the schema is not vacuously true.
func TestSchema_RejectsBadDocuments(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(filepath.Join(root, "schemas", "platforms.schema.json"))
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	bad := map[string]any{
		"schema_version": 1,
		"matrix_version": "1.0.0",
		"applies_to":     ">=1.0.0",
		"policy":         map[string]any{"cgo_enabled": 1, "build_tags": []any{}, "ldflags": []any{"-s"}},
		"platforms":      []any{},
		"channels":       map[string]any{"direct": map[string]any{"description": "x"}},
	}
	if err := sch.Validate(bad); err == nil {
		t.Fatal("schema accepted an invalid document")
	}
}

// TestSchema_AndGoValidatorAgree asserts the JSON Schema and the Go
// validator reach the same verdict on the canonical matrix.
func TestSchema_AndGoValidatorAgree(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	compiler := jsonschema.NewCompiler()
	sch, err := compiler.Compile(filepath.Join(root, "schemas", "platforms.schema.json"))
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	doc, err := loadYAMLAsJSON(filepath.Join(root, "docs", "reference", "platforms.yaml"))
	if err != nil {
		t.Fatalf("load yaml: %v", err)
	}
	schemaErr := sch.Validate(doc)

	_, goErr := platform.LoadDefault()

	if (schemaErr == nil) != (goErr == nil) {
		t.Fatalf("schema err = %v, go err = %v; they must agree", schemaErr, goErr)
	}
}

// --- helpers --------------------------------------------------------------

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found")
		}
		dir = parent
	}
}

// loadYAMLAsJSON reads a YAML file and returns it as a value that
// encoding/json and JSON Schema validators can consume.
//
// It uses gopkg.in/yaml.v3 — the same library the production loader in
// internal/platform uses — so a document that parses here is parsed
// identically by platform.Load. The value is round-tripped through
// encoding/json to normalise map[string]any vs map[any]any, which the
// JSON Schema library requires.
//
// YAML 1.2 semantics are preserved: yaml.v3 does not coerce "on"/"off"
// to booleans, does not silently accept duplicate keys, and does not
// resolve aliases across documents. These are the same semantics the
// production loader relies on.
func loadYAMLAsJSON(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out any
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	var normalised any
	if err := json.Unmarshal(b, &normalised); err != nil {
		return nil, err
	}
	return normalised, nil
}
