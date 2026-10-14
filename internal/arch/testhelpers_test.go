package arch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// moduleRoot returns the absolute path to the repository root.
//
// The test binary's working directory under `go test ./internal/arch/...` is
// the package directory (internal/arch), so the module root is two levels up.
// Resolving to an absolute path removes any ambiguity about how go/packages
// interprets a relative Dir, and makes the helper work regardless of where
// `go test` is invoked from.
func moduleRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	return abs
}

// loadInternalPackages returns the module-relative paths of every package
// under internal/... in the forge module.
//
// It is the single, shared loader used by the layer tests (14.1.1) and the
// matrix tests (14.1.2). Keeping it in one place means a change to the load
// pattern, the module-relative trimming, or the build-tag handling is made
// once and observed by every consumer.
//
// The returned slice is in the order reported by go/packages, which is
// deterministic for a fixed module. Callers that require a specific order
// should sort the result themselves.
//
// Errors are reported via t.Fatalf; callers receive only a valid slice.
// The helper never returns nil for a successful load — if the pattern matches
// zero packages, or if every package is filtered out by the module-relative
// check, that is a test failure. Either outcome would hide every downstream
// assertion.
func loadInternalPackages(t *testing.T, mod string) []string {
	t.Helper()

	dir := moduleRoot(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./internal/...")
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("no packages loaded from %s; check pattern ./internal/...", dir)
	}

	out := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		rel, ok := moduleRelativeInternal(mod, p.PkgPath)
		if !ok {
			continue
		}
		out = append(out, rel)
	}
	if len(out) == 0 {
		// The loader returned packages, but none matched the module path.
		// Report the first few PkgPaths so a maintainer can see what went
		// wrong (e.g. a stale module cache, a go.work file, a renamed module).
		var sample []string
		for i, p := range pkgs {
			if i >= 3 {
				break
			}
			sample = append(sample, p.PkgPath)
		}
		t.Fatalf("no internal packages found under module %q; sample PkgPaths: %v", mod, sample)
	}
	return out
}

// loadInternalPackageGraph returns the loaded packages, for tests that need
// to walk the import graph (e.g. TestNoUpwardImports in layers_test.go).
//
// It is the richer variant of loadInternalPackages: the caller gets the full
// *packages.Package values, not just the module-relative paths. The module
// path is not returned because callers that need it already call
// modulePathFromGoMod once and pass it to moduleRelativeInternal.
func loadInternalPackageGraph(t *testing.T, mod string) []*packages.Package {
	t.Helper()

	dir := moduleRoot(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./internal/...")
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("no packages loaded from %s; check pattern ./internal/...", dir)
	}
	return pkgs
}

// moduleRelativeInternal trims the module path from an import path and
// reports whether the result is under internal/.
//
// It is the inverse of the package loader: given a package's PkgPath
// (e.g. "github.com/thapelomagqazana/forge/internal/arch") and the module
// path, it returns ("internal/arch", true). For packages outside the module
// or outside internal/, it returns ("", false).
//
// The "internal" and "internal/" cases are rejected because they name the
// internal root directory, not a package. Go's own tooling does not produce
// such package paths, but the guard prevents a malformed PkgPath from being
// silently classified.
func moduleRelativeInternal(mod, importPath string) (string, bool) {
	rel := strings.TrimPrefix(importPath, mod+"/")
	if !strings.HasPrefix(rel, "internal/") {
		return "", false
	}
	if rel == "internal/" || rel == "internal" {
		return "", false
	}
	return rel, true
}

// modulePathFromGoMod reads the module path from the repository's go.mod.
//
// The path is read at test time (rather than hard-coded) so that forks,
// renames, and vendored builds keep working without changing any test. The
// function fails the test if go.mod cannot be read or has no module directive.
//
// It assumes the test runs from a directory one level below the repository
// root (internal/arch), which is where all callers live. If a future test
// moves, the "../../go.mod" path must move with it; the failure mode is a
// clear "read go.mod" error, not a silent misread.
func modulePathFromGoMod(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if mod == "" {
				t.Fatal("go.mod module directive is empty")
			}
			return mod
		}
	}
	t.Fatal("go.mod has no module directive")
	return ""
}

// mustReadFile reads a file and fails the test with a readable message if it
// cannot. It exists to avoid repeating the os.ReadFile + t.Fatalf pattern in
// every golden test.
func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// assertContains fails the test if haystack does not contain needle. It is
// used for the string-matching acceptance tests, where full equality is
// brittle and substring containment expresses the intent.
func assertContains(t *testing.T, haystack, needle, context string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%s: expected to contain %q", context, needle)
	}
}

// formatViolations renders a slice of "from -> to" strings as a
// newline-indented block suitable for t.Fatalf. Determinism matters: the
// caller sorts the slice, and this helper preserves the order.
//
// An empty slice renders as the empty string, so a caller that unconditionally
// formats the result of a successful scan produces no trailing whitespace or
// spurious blank lines.
func formatViolations(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return "\n  " + strings.Join(vs, "\n  ")
}
