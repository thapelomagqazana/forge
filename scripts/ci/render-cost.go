//go:build ignore

// render-cost.go writes the cost model's two rendered artifacts:
//
//   - docs/reference/ci-cost-budget.md
//     from ci.CostMarkdown()
//   - internal/ci/testdata/cost.golden.json
//     from ci.CostGolden()
//
// The script is run by `task ci:render-cost`. The //go:build ignore
// directive keeps it out of the module's normal build (go build ./...
// and go test ./...) so that it is not compiled into the repository's
// packages; it is only compiled when invoked explicitly with
// `go run`.
//
// # Why a script rather than a test-time generator
//
// The two files are committed to the repository. They are not
// produced by the test suite; the test suite compares the committed
// files against the model. The generator must therefore be runnable
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
//  2. Writes docs/reference/ci-cost-budget.md from
//     ci.CostMarkdown().
//  3. Writes internal/ci/testdata/cost.golden.json from
//     ci.CostGolden().
//  4. Prints each written path to stdout.
//
// # What the script does not do
//
//   - It does not validate the written files against the model. The
//     contract tests (TestCost_DocMatchesModel and
//     TestCost_GoldenMatchesModel) do that; the script is a
//     generator, not a verifier. If the script wrote stale files,
//     the tests would fail on the next `go test` run.
//
//   - It does not update the ADR. The ADR is hand-written; its
//     content does not derive from the model, only its facts do.
//
//   - It does not modify .github/workflows/ci.yml. The paths-ignore
//     filter is hand-written; its presence is asserted by the
//     contract test TestCost_CIYmlHasPathsIgnore. A future change
//     that wanted the filter rendered from a model would add a
//     second renderer.
//
//   - It does not commit the changes. A contributor who runs the
//     script reviews the diff and commits; the script's job ends
//     when the files are written.
//
// # Exit codes
//
//	0 — both files were written.
//	1 — the module root could not be found, or a file could not be
//	    written. The error is printed to stderr with the
//	    "render-cost:" prefix so that a caller can distinguish the
//	    script's output from the model's output.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// outputs is the list of files the script writes.
//
// Each entry names the file's path (relative to the module root) and
// the function that produces its content. The slice is ordered: the
// cost-budget document is written first, then the golden JSON. The
// order does not matter for correctness (each file is independent),
// but it makes the script's stdout deterministic, which is useful
// when a caller diffs two runs' output.
var outputs = []struct {
	// RelPath is the file's path, relative to the module root. The
	// path uses forward slashes; the script converts to the OS's
	// separator when building the absolute path.
	RelPath string

	// Content returns the file's content.
	Content func() string

	// What describes the rendering for the script's stderr messages.
	What string
}{
	{
		RelPath: "docs/reference/ci-cost-budget.md",
		Content: ci.CostMarkdown,
		What:    "cost-budget document",
	},
	{
		RelPath: "internal/ci/testdata/cost.golden.json",
		Content: ci.CostGolden,
		What:    "golden JSON",
	},
}

// main is the script's entry point.
//
// The function delegates to run and exits with run's return code. The
// split between main and run is the same pattern the repository's
// other scripts use (see scripts/build.go and
// scripts/ci/render-cache.go): it lets a test call run without
// spawning a subprocess, and it keeps main's body small.
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
		fmt.Fprintln(os.Stderr, "render-cost:", err)
		return 1
	}

	for _, o := range outputs {
		path := filepath.Join(root, filepath.FromSlash(o.RelPath))

		// Create the file's directory if it does not exist. The
		// directory is committed, so the call is a no-op in
		// practice; the check is defensive against a fresh clone
		// where a contributor deleted the directory to test the
		// script.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "render-cost: %s: %v\n", o.What, err)
			return 1
		}

		// Compute the content. The call is the only dependency on
		// the package under test. A panic inside Content (for
		// example, a malformed model) would surface here as a
		// panic in the script, which is the correct failure mode:
		// the model's invariants are validated at package init, so
		// a panic here means the model is inconsistent and the
		// script's output would be wrong.
		content := o.Content()

		// Write the file with 0o644 so that the repository's file
		// mode is not changed by a contributor whose umask is
		// unusual. The mode is the same as the other golden files
		// in the repository.
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "render-cost: %s: %v\n", o.What, err)
			return 1
		}

		fmt.Println("wrote", path)
	}

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
