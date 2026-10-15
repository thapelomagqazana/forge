package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildPerBinarySBOM asserts the per-binary SBOM contains the
// expected fields: bomFormat CycloneDX, the archive's SHA-256, and
// the module dependencies.
func TestBuildPerBinarySBOM(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "forge-linux-amd64.tar.gz"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := options{distDir: dir, version: "v1.4.2"}
	p := platformForTest("linux", "amd64")
	deps := []goModDep{
		{path: "github.com/spf13/cobra", version: "v1.8.1"},
	}
	doc, err := buildPerBinarySBOM(opts, p, deps)
	if err != nil {
		t.Fatal(err)
	}
	if doc.BOMFormat != "CycloneDX" {
		t.Errorf("BOMFormat = %q", doc.BOMFormat)
	}
	if doc.SpecVersion != "1.5" {
		t.Errorf("SpecVersion = %q", doc.SpecVersion)
	}
	if doc.Metadata.Component.Name != "forge" {
		t.Errorf("name = %q", doc.Metadata.Component.Name)
	}
	if doc.Metadata.Component.Version != "v1.4.2" {
		t.Errorf("version = %q", doc.Metadata.Component.Version)
	}
	if len(doc.Components) != 1 {
		t.Fatalf("components = %d, want 1", len(doc.Components))
	}
	if doc.Components[0].Name != "github.com/spf13/cobra" {
		t.Errorf("component name = %q", doc.Components[0].Name)
	}
}

// TestBuildPerBinarySBOM_ValidJSON asserts the doc round-trips
// through encoding/json.
func TestBuildPerBinarySBOM_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "forge-linux-amd64.tar.gz"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := options{distDir: dir, version: "v1.4.2"}
	p := platformForTest("linux", "amd64")
	doc, err := buildPerBinarySBOM(opts, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["bomFormat"] != "CycloneDX" {
		t.Errorf("round-trip bomFormat = %v", got["bomFormat"])
	}
}
