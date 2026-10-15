//go:build ignore

// render-tiers.go renders the tier table to the two locations that
// carry it: the published document (docs/reference/ci-tiers.md) and
// the golden file (internal/ci/testdata/ci-tiers.golden.md).
//
// The script is run by `task ci:render-tiers`. It is not part of the
// package under test; the //go:build ignore directive keeps it out of
// the go build ./... and go test ./... runs.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thapelomagqazana/forge/internal/ci"
)

func main() {
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "render-tiers:", err)
		os.Exit(1)
	}

	table := ci.TiersTable()

	targets := []string{
		filepath.Join(root, "docs", "reference", "ci-tiers.md"),
		filepath.Join(root, "internal", "ci", "testdata", "ci-tiers.golden.md"),
	}
	for _, path := range targets {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "render-tiers:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, []byte(table), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "render-tiers:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", path)
	}
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
