//go:build ignore

// render-tier3.go writes the Tier 3 model's golden JSON to
// internal/ci/testdata/tier3.golden.json.
//
// The script is run by `task ci:render-tier3`. The //go:build ignore
// directive keeps it out of the module's normal build (go build ./...
// and go test ./...) so that it is not compiled into the repository's
// packages; it is only compiled when invoked explicitly with
// `go run`.
//
// # Why a script rather than a test-time generator
//
// The golden file is committed to the repository. It is not produced
// by the test suite; the test suite compares the committed file
// against the model. The generator must therefore be runnable
// explicitly by a contributor who changes the model, and it must not
// run during `go test`. A //go:build ignore program satisfies both
// requirements: it is excluded from the build, and it is invocable
// with `go run`.
//
// # What the script does
//
//  1. Finds the module root by walking up from the working directory
//     until it finds go.mod. The walk is the same pattern the test
//     files use (see repoRoot in workflows_contract_test.go); the
//     script cannot use that helper because it is in package ci_test,
//     not in package main.
//  2. Computes the golden bytes by calling ci.Tier3Golden().
//  3. Writes the bytes to internal/ci/testdata/tier3.golden.json,
//     creating the testdata directory if it does not exist.
//
// # What the script does not do
//
//   - It does not validate the golden bytes against the model. The
//     contract test TestTier3_GoldenMatchesModel does that; the script
//     is a generator, not a verifier. If the script wrote a stale
//     golden file, the test would fail on the next `go test` run.
//
//   - It does not update docs/reference/ci-tiers.md. That file is
//     rendered by `task ci:render-tiers` (the cross-tier renderer,
//     which is a different script). The two renderers are separate
//     because the tier table and the Tier 3 golden file change at
//     different cadences: the golden file changes whenever the Tier 3
//     model changes; the tier table changes whenever any tier
//     changes.
//
//   - It does not commit the change. A contributor who runs the script
//     reviews the diff and commits it; the script's job ends when the
//     file is written.
//
// # Exit codes
//
//	0 — the golden file was written.
//	1 — the module root could not be found, or the file could not be
//	    written. The error is printed to stderr with the
//	    "render-tier3:" prefix so that a caller can distinguish the
//	    script's output from the model's output.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// goldenRelPath is the golden file's path relative to the module
// root.
//
// The path is a constant rather than a flag because the script has
// exactly one output. A future change that needed a second output
// (for example, a second golden file for a different rendering)
// would add a flag, but the current script's single purpose is to
// write one file.
const goldenRelPath = "internal/ci/testdata/tier3.golden.json"

// main is the script's entry point.
//
// The function delegates to run and exits with run's return code. The
// split between main and run is the same pattern the repository's
// other scripts use (see scripts/build.go): it lets a test call run
// without spawning a subprocess, and it keeps main's body small.
func main() {
	os.Exit(run())
}

// run is the script's body. It returns the process exit code: 0 on
// success, 1 on any error.
//
// The function has no parameters because the script takes no
// arguments. A future change that needed arguments would add them
// here; the current form keeps the signature minimal.
func run() int {
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "render-tier3:", err)
		return 1
	}

	path := filepath.Join(root, filepath.FromSlash(goldenRelPath))

	// Create the testdata directory if it does not exist. The
	// directory is committed, so the call is a no-op in practice;
	// the check is defensive against a fresh clone where a
	// contributor deleted the directory to test the script.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "render-tier3:", err)
		return 1
	}

	// Compute the golden bytes.
	//
	// The call is the only dependency on the package under test. A
	// failure inside Tier3Golden (for example, a panic from a
	// malformed model) would surface here as a panic in the
	// script, which is the correct failure mode: the model's
	// invariants are validated at package init, so a panic here
	// means the model is inconsistent and the script's output
	// would be wrong.
	golden := ci.Tier3Golden()

	// Write the file with 0o644 so that the repository's file mode
	// is not changed by a contributor whose umask is unusual. The
	// mode is the same as the other golden files in the
	// repository.
	if err := os.WriteFile(path, []byte(golden), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "render-tier3:", err)
		return 1
	}

	fmt.Println("wrote", path)
	return 0
}

// findModuleRoot walks up from the current working directory until it
// finds a go.mod file, and returns the directory that contains it.
//
// The function is the same pattern the test files use (see repoRoot
// in workflows_contract_test.go). The duplication is deliberate: the
// script is package main, and the test helper is package ci_test;
// neither can import the other. The two-line walk is small enough
// that the duplication is not worth a shared package.
//
// The function returns an error rather than calling os.Exit so that
// run can decide the exit code. The error's message names the
// starting directory and the reason, which is the most useful
// information a contributor sees when the script fails.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"go.mod not found walking up from %s",
				start,
			)
		}
		dir = parent
	}
}
