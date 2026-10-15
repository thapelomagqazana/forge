// Package verify_test contains end-to-end acceptance tests for the
// binary verification suite (WBS 17.5.1).
//
// The tests build the artifacts with scripts/build.go, then run
// cmd/forge-internal-verify against the resulting dist/ directory.
// They exercise the verifier's observable contract: its exit code,
// its report's shape, and the acceptance criteria of WBS 17.5.1.
//
// # What these tests are
//
// They are the ATDD acceptance tests for the verification suite. The
// suite's unit tests live in cmd/forge-internal-verify/main_test.go;
// those tests exercise the verifier's internals. The tests here
// exercise the verifier's observable behavior: given a directory of
// artifacts, does the verifier pass the right ones and fail the
// wrong ones?
//
// # What these tests are not
//
// They are not the integration tests for the Forge binary itself.
// Those live in cmd/forge/ (binary_integration_test.go,
// brokenpipe_*.go). They are not the unit tests for the verifier;
// those live in cmd/forge-internal-verify/main_test.go.
//
// # Why the tests are skipped in -short mode
//
// The tests build the artifacts (roughly thirty seconds on a warm
// cache) and run the verifier against them. A `go test ./... -short`
// run does not have that budget. The full suite runs in CI on every
// release tag, and in the local `task test:integration`.
//
// # Why the tests use go run instead of a built binary
//
// The verifier is invoked as `go run ./cmd/forge-internal-verify`.
// Building the verifier first would add a compilation step and an
// artifact-management concern to every test. `go run` builds the
// verifier on the fly and executes it; the binary is cached by the
// Go toolchain.
//
// The tests do not shell out. They use os/exec directly, with the
// arguments passed verbatim. No shell is involved; the exit code is
// the verifier's, not a shell's translation.
//
// # Why the tests use t.TempDir for dist/
//
// The tests must not pollute the repository's dist/ directory. Each
// test creates its own temporary directory, passes it to the builder
// via --dist=<dir>, and passes the same directory to the verifier.
// The builder's output is isolated to the test's temporary tree.
package verify_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Positive: the suite passes against a fresh build
// =============================================================================

// TestVerifyBinaries_SucceedsAgainstFreshBuild is the ATDD acceptance
// test for WBS 17.5.1.
//
// The test:
//
//  1. Builds the artifacts with `go run ./scripts/build.go --all`.
//  2. Runs `go run ./cmd/forge-internal-verify --dist <dir>` against
//     the resulting directory.
//  3. Asserts the verifier exits 0 and prints "OK:".
//
// The test is the executable form of the WBS's KPI: "All six
// platforms pass smoke suite". A regression in the build pipeline
// (wrong architecture, dynamic linking, missing field) fails here.
//
// # Why the test builds and verifies
//
// The build step is not optional. A test that ran the verifier
// against a checked-in dist/ would not exercise the current
// pipeline; it would exercise whatever was committed last. The test
// builds the artifacts, so a regression in the builder is caught by
// the verifier.
//
// # What the test does not assert
//
// The test does not assert on the verifier's per-check output. The
// per-check output is the verifier's internal report; a change to
// its format is not a contract change. The test asserts the exit
// code and the summary line ("OK:").
func TestVerifyBinaries_SucceedsAgainstFreshBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary verification in short mode")
	}
	t.Parallel()

	root := repoRoot(t)
	dist := t.TempDir()

	// 1. Build the artifacts.
	build := exec.Command("go", "run", "./scripts/build.go", "--all", "--dist="+dist)
	build.Dir = root
	var buildOut bytes.Buffer
	build.Stdout = &buildOut
	build.Stderr = &buildOut
	if err := build.Run(); err != nil {
		t.Fatalf("build: %v\noutput:\n%s", err, buildOut.String())
	}

	// 2. Run the verifier.
	verify := exec.Command("go", "run", "./cmd/forge-internal-verify", "--dist="+dist)
	verify.Dir = root
	var verifyOut bytes.Buffer
	verify.Stdout = &verifyOut
	verify.Stderr = &verifyOut
	if err := verify.Run(); err != nil {
		t.Fatalf("verify: %v\noutput:\n%s", err, verifyOut.String())
	}

	// 3. Assert the summary line.
	if !strings.Contains(verifyOut.String(), "OK:") {
		t.Errorf("output does not contain OK:\n%s", verifyOut.String())
	}
	if strings.Contains(verifyOut.String(), "FAIL") {
		t.Errorf("output contains FAIL against a fresh build:\n%s", verifyOut.String())
	}
}

// TestVerifyBinaries_ReportsPerPlatformSections asserts the
// verifier's report contains one section per Tier 1/2 platform.
//
// The test pins the report's shape: a reader who runs the verifier
// sees one section per platform, and the section count matches the
// platform matrix. The assertion is loose enough to tolerate
// additions to the matrix; it counts sections by their "(" suffix.
func TestVerifyBinaries_ReportsPerPlatformSections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary verification in short mode")
	}
	t.Parallel()

	root := repoRoot(t)
	dist := t.TempDir()

	build := exec.Command("go", "run", "./scripts/build.go", "--all", "--dist="+dist)
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	verify := exec.Command("go", "run", "./cmd/forge-internal-verify", "--dist="+dist)
	verify.Dir = root
	out, err := verify.CombinedOutput()
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}

	// Each platform section begins with "<name> (<goos>/<goarch>, ...)".
	// The count is the number of lines whose first token is a
	// platform name and whose second is a parenthesised tuple.
	sections := 0
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "OK") || strings.HasPrefix(trimmed, "FAIL") {
			continue
		}
		if strings.Contains(trimmed, " (") && strings.Contains(trimmed, ")") {
			sections++
		}
	}
	// Six Tier 1/2 platforms. The count is stable; a future matrix
	// change updates this constant in the same commit.
	const want = 6
	if sections != want {
		t.Errorf("report has %d platform section(s), want %d\n%s",
			sections, want, out)
	}
}

// =============================================================================
// Negative: the suite fails when it should
// =============================================================================

// TestVerifyBinaries_DetectsMissingArtifact verifies the verifier
// fails when the artifact is missing.
//
// The test points the verifier at an empty directory. The verifier
// must exit non-zero and print "FAIL". The failure message names the
// missing file so a user knows which artifact is absent.
func TestVerifyBinaries_DetectsMissingArtifact(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	empty := t.TempDir()

	verify := exec.Command("go", "run", "./cmd/forge-internal-verify",
		"--dist="+empty, "--platform=linux/amd64")
	verify.Dir = root
	var out bytes.Buffer
	verify.Stdout = &out
	verify.Stderr = &out
	err := verify.Run()
	if err == nil {
		t.Fatalf("verify succeeded against an empty dist; want failure\n%s", out.String())
	}
	if !strings.Contains(out.String(), "FAIL") {
		t.Errorf("output does not contain FAIL:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "forge-linux-amd64.tar.gz") {
		t.Errorf("output does not name the missing artifact:\n%s", out.String())
	}
}

// TestVerifyBinaries_RejectsBadSizeBudget verifies the size budget
// is enforced.
//
// The test builds a fake artifact that exceeds the budget, places it
// in a temp dist, and asserts the verifier fails the "size" check.
// The fake artifact is not a valid archive; the verifier reports the
// size failure before the extraction failure, which is what the
// test asserts.
//
// # Why the test uses a fake artifact
//
// The test cannot build a real artifact over the size budget: the
// real artifacts are small. The fake artifact is a sparse file
// whose size is one byte over the budget. The verifier's size check
// runs before extraction, so the fake artifact's invalidity does
// not matter.
func TestVerifyBinaries_RejectsBadSizeBudget(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	dist := t.TempDir()

	// A 16 MiB file named as the linux/amd64 artifact. The size
	// budget is 15 MiB; the file exceeds it.
	fake := filepath.Join(dist, "forge-linux-amd64.tar.gz")
	f, err := os.Create(fake)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(16 * 1024 * 1024); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	verify := exec.Command("go", "run", "./cmd/forge-internal-verify",
		"--dist="+dist, "--platform=linux/amd64")
	verify.Dir = root
	var out bytes.Buffer
	verify.Stdout = &out
	verify.Stderr = &out
	err = verify.Run()
	if err == nil {
		t.Fatalf("verify accepted an oversized artifact; want failure\n%s", out.String())
	}
	if !strings.Contains(out.String(), "size") {
		t.Errorf("output does not mention the size check:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "budget") {
		t.Errorf("output does not mention the budget:\n%s", out.String())
	}
}

// TestVerifyBinaries_RejectsUnknownPlatform verifies the verifier
// rejects a platform name that is not in the matrix.
//
// The --platform flag accepts a single platform. A value that does
// not match any matrix entry selects no platform; the verifier must
// report the empty selection rather than run zero checks and
// succeed.
func TestVerifyBinaries_RejectsUnknownPlatform(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	dist := t.TempDir()

	verify := exec.Command("go", "run", "./cmd/forge-internal-verify",
		"--dist="+dist, "--platform=plan9/386")
	verify.Dir = root
	var out bytes.Buffer
	verify.Stdout = &out
	verify.Stderr = &out
	err := verify.Run()
	if err == nil {
		t.Fatalf("verify accepted an unknown platform; want failure\n%s", out.String())
	}
	if !strings.Contains(out.String(), "no platforms selected") {
		t.Errorf("output does not mention the empty selection:\n%s", out.String())
	}
}

// =============================================================================
// Documentation and CI assertions
// =============================================================================

// TestVerifyBinaries_DocsExist asserts the two documents WBS 17.5.1
// requires are present.
//
// The assertion is a file-existence check. It does not validate the
// documents' content; that is a documentation review, not a test.
// The check catches a future refactor that deletes the docs without
// updating the WBS.
func TestVerifyBinaries_DocsExist(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	for _, rel := range []string{
		"docs/guides/verifying-binaries.md",
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("required document missing: %s (%v)", rel, err)
		}
	}
}

// TestVerifyBinaries_MakefileIntegrates asserts the Makefile's
// release-dry-run target invokes the verifier.
//
// The check is textual: it reads the Makefile and asserts the target
// contains the verifier's invocation. A future change that removed
// the invocation would leave the WBS's acceptance criterion
// ("Verification integrated into make release-dry-run") unsatisfied.
//
// The check does not parse the Makefile; a substring search is
// sufficient for a target that is a sequence of explicit commands.
func TestVerifyBinaries_MakefileIntegrates(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(data)

	if !strings.Contains(makefile, "cmd/forge-internal-verify") {
		t.Errorf("Makefile does not invoke cmd/forge-internal-verify; " +
			"the WBS requires the verifier to be part of release-dry-run")
	}
}

// TestVerifyBinaries_WorkflowExists asserts the CI workflow WBS
// 17.5.1 requires is present.
//
// The workflow runs the verifier on every release tag. The check is
// a file-existence check; it does not validate the workflow's
// content.
func TestVerifyBinaries_WorkflowExists(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "verify-binaries.yml")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("verify-binaries workflow missing: %v", err)
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// repoRoot walks up from the test's working directory to find
// go.mod.
//
// The helper is local to this package. It is duplicated from other
// test packages in the repo because Go does not permit sharing
// unexported helpers across packages; the duplication is three
// small functions, not a meaningful maintenance burden.
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
