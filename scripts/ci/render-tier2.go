//go:build ignore

// render-tier2.go writes internal/ci/testdata/tier2.golden.json
// from ci.Tier2Golden(). Run from the repository root:
//
//     go run ./scripts/ci/render-tier2.go
//
// The script is the source of the golden file; the contract test
// TestTier2_GoldenMatchesModel compares the file byte-for-byte
// against ci.Tier2Golden(), so the file must always be regenerated
// from the model, never edited by hand.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thapelomagqazana/forge/internal/ci"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "render-tier2: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := findModuleRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "internal", "ci", "testdata", "tier2.golden.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data := ci.Tier2Golden()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", path, len(data))
	return nil
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find the module root")
		}
		dir = parent
	}
}
