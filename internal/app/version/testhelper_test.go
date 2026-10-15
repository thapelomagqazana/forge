package version

import (
	"flag"
	"os"
	"testing"
)

// update is the -update flag shared by every golden-file test in
// this package. When set, a golden test writes its actual output to
// the golden file instead of comparing against it.
//
// Usage:
//
//	go test ./internal/app/version/ -update
//	go test ./internal/app/version/ -run TestWriteJSON_Golden -update
//
// The flag is package-level because Go registers flags per test
// binary, not per test function. Every golden test in the package
// reads the same flag.
var update = flag.Bool("update", false, "update golden files")

// readGolden reads a golden file. When -update is set, the caller
// is expected to have written the file first; this function still
// reads it so the comparison path is exercised.
func readGolden(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return string(b)
}

// writeGolden writes content to path. It is called by a golden test
// when -update is set.
func writeGolden(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write golden %s: %v", path, err)
	}
}
