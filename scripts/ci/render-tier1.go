//go:build ignore

// render-tier1.go writes the Tier 1 model's golden JSON to
// internal/ci/testdata/tier1.golden.json.
//
// The script is run by `task ci:render-tier1`. The //go:build ignore
// directive keeps it out of the go build ./... and go test ./...
// runs.
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
		fmt.Fprintln(os.Stderr, "render-tier1:", err)
		os.Exit(1)
	}

	path := filepath.Join(root, "internal", "ci", "testdata", "tier1.golden.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "render-tier1:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, []byte(ci.Tier1Golden()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "render-tier1:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", path)
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
