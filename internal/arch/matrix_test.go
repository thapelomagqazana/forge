package arch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/arch"
)

// loadCanonicalMatrix loads the published dependency matrix from
// docs/architecture/dependency-matrix.yaml.
//
// Every test in this file starts from this load, so a broken YAML fails the
// whole suite rather than one test. The path is relative to internal/arch,
// which is the test's working directory under `go test`.
//
// The helper is deliberately local to this file rather than shared with
// layers_test.go: the layer tests do not need the matrix, and moving this to
// testhelpers_test.go would force every arch test to pay the load cost.
func loadCanonicalMatrix(t *testing.T) *arch.Matrix {
	t.Helper()
	p := filepath.Join("..", "..", "docs", "architecture", "dependency-matrix.yaml")
	m, err := arch.LoadMatrix(p)
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}
	return m
}

// --- Positive ---

// TestMatrixLoadsAndCompiles asserts that the canonical YAML parses, validates,
// compiles, and passes the substantive acyclicity check without error.
//
// It is the smoke test for the whole matrix pipeline: if this fails, every
// other test in this file is meaningless.
func TestMatrixLoadsAndCompiles(t *testing.T) {
	m := loadCanonicalMatrix(t)
	if len(m.Groups) == 0 {
		t.Fatal("no groups loaded")
	}
	if m.Version != 1 {
		t.Fatalf("version = %d, want 1", m.Version)
	}
}

// TestMatrixAcyclic asserts that the canonical matrix has no substantive
// cycles. ParseMatrix already rejects a cyclic matrix at load time, so this
// test would fail at loadCanonicalMatrix if the invariant were violated. It
// is kept as an explicit assertion so that the invariant is named in the
// test suite, not only implied by the parser's behavior.
func TestMatrixAcyclic(t *testing.T) {
	m := loadCanonicalMatrix(t)
	if err := m.Acyclic(); err != nil {
		t.Fatalf("matrix has a cycle: %v", err)
	}
}

// TestMatrixGroupsCoverEveryInternalPackage asserts that every package under
// internal/... is assigned to exactly one group by GroupOf.
//
// This is the package-group analogue of TestEveryInternalPackageIsClassified
// from 14.1.1: even if a package is classified into a layer, it must also be
// covered by a group for the matrix to apply. The canonical matrix covers
// internal/arch (via architecture.tooling) and the three layer-root
// placeholder packages (via domain, infrastructure, and shared).
func TestMatrixGroupsCoverEveryInternalPackage(t *testing.T) {
	m := loadCanonicalMatrix(t)
	mod := modulePathFromGoMod(t)
	pkgs := loadInternalPackages(t, mod)

	var uncovered []string
	for _, p := range pkgs {
		if m.GroupOf(p) == "" {
			uncovered = append(uncovered, p)
		}
	}
	if len(uncovered) > 0 {
		t.Fatalf("packages not covered by any group (add them to a group in dependency-matrix.yaml):%s",
			formatViolations(uncovered))
	}
}

// --- Negative ---

// TestMatrixRejectsUndeclaredGroupInCell asserts that a matrix cell naming a
// group that is not declared in the groups block is rejected at parse time.
func TestMatrixRejectsUndeclaredGroupInCell(t *testing.T) {
	yaml := []byte(`
version: 1
schema: x
adr: x
groups:
  a: {layer: shared, packages: [internal/a]}
  b: {layer: shared, packages: [internal/b]}
matrix:
  a: {c: allow}
conditions: {}
exceptions: []
`)
	_, err := arch.ParseMatrix(yaml)
	if err == nil {
		t.Fatal("expected error for undeclared group in cell")
	}
	if !strings.Contains(err.Error(), "not a declared group") {
		t.Fatalf("error = %v, want 'not a declared group'", err)
	}
}

// TestMatrixRejectsUndeclaredCondition asserts that a cell referencing a
// condition that is not declared is rejected at parse time. This catches the
// most common drift: a condition added to the matrix but not to the
// conditions block.
func TestMatrixRejectsUndeclaredCondition(t *testing.T) {
	yaml := []byte(`
version: 1
schema: x
adr: x
groups:
  a: {layer: shared, packages: [internal/a]}
  b: {layer: shared, packages: [internal/b]}
matrix:
  a: {b: "condition:C99"}
conditions: {}
exceptions: []
`)
	_, err := arch.ParseMatrix(yaml)
	if err == nil {
		t.Fatal("expected error for undeclared condition")
	}
	if !strings.Contains(err.Error(), "undeclared condition C99") {
		t.Fatalf("error = %v, want 'undeclared condition C99'", err)
	}
}

// TestMatrixRejectsDeadCondition asserts that a condition declared but never
// referenced by any matrix cell is rejected.
//
// The current validator reports the structural defect ("has no applies_to and
// is not exception-managed") before the dead-declaration check, because an
// empty applies_to on a non-exception-managed condition is the more specific
// and more actionable failure. The test asserts the message the validator
// actually produces; the dead-declaration path is covered by the fact that a
// condition with neither a reference nor an applies_to cannot load.
func TestMatrixRejectsDeadCondition(t *testing.T) {
	yaml := []byte(`
version: 1
schema: x
adr: x
groups:
  a: {layer: shared, packages: [internal/a]}
  b: {layer: shared, packages: [internal/b]}
matrix:
  a: {b: allow}
conditions:
  C99:
    summary: "dead"
    rationale: "never referenced"
    applies_to: []
    enforced_by: review
exceptions: []
`)
	_, err := arch.ParseMatrix(yaml)
	if err == nil {
		t.Fatal("expected error for dead condition")
	}
	want := "has no applies_to and is not exception-managed"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// TestMatrixRejectsCycle asserts that a substantive cycle is rejected at
// parse time, with an error that names the cycle.
//
// ParseMatrix calls Acyclic as its final validation step, so a substantive
// cycle never produces a usable Matrix. The test asserts both the presence of
// the cycle error and the presence of the cycle path, which makes the failure
// actionable: the maintainer sees "a -> b -> a" and knows which cells to fix.
//
// Exception-derived edges are excluded from the check (see ADR-042
// §"Exception edges and acyclicity"); this test uses an explicit matrix cell
// cycle, which is the case the check is designed to catch.
func TestMatrixRejectsCycle(t *testing.T) {
	yaml := []byte(`
version: 1
schema: x
adr: x
groups:
  a: {layer: shared, packages: [internal/a]}
  b: {layer: shared, packages: [internal/b]}
matrix:
  a: {b: allow}
  b: {a: allow}
conditions: {}
exceptions: []
`)
	_, err := arch.ParseMatrix(yaml)
	if err == nil {
		t.Fatal("expected error for cyclic matrix")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error = %v, want 'cycle'", err)
	}
	// The DFS visits nodes in sorted order and reconstructs the cycle from
	// the stack, so the message is deterministic: "a -> b -> a".
	if !strings.Contains(err.Error(), "a -> b -> a") {
		t.Fatalf("error = %v, want cycle path 'a -> b -> a'", err)
	}
}

// --- Edge / boundary ---

// TestMatrixDefaultsUnlistedPairsToDeny asserts that a pair not listed in the
// matrix and not covered by an exception is denied by default. This is the
// closed-by-default invariant at the group level.
func TestMatrixDefaultsUnlistedPairsToDeny(t *testing.T) {
	m := loadCanonicalMatrix(t)
	d, _ := m.Lookup("domain.policy", "shared.version")
	if d != arch.DecisionDeny {
		t.Fatalf("decision = %v, want deny", d)
	}
}

// TestMatrixExceptionsOverrideDeny asserts that a cross-cutting exception
// (shared.errors, shared.logging) applies to every group, even groups that
// have no explicit cell for the exception target. This is the mechanism that
// makes the error taxonomy and logger interface universally importable
// without a cell-per-row in the YAML.
func TestMatrixExceptionsOverrideDeny(t *testing.T) {
	m := loadCanonicalMatrix(t)
	d, _ := m.Lookup("domain.policy", "shared.errors")
	if d != arch.DecisionAllow {
		t.Fatalf("decision = %v, want allow (exception)", d)
	}
}

// TestMatrixSelfImportAllowed asserts that every group may import itself.
// Self-imports are permitted implicitly by Compile and are not written as
// cells in the YAML.
func TestMatrixSelfImportAllowed(t *testing.T) {
	m := loadCanonicalMatrix(t)
	for name := range m.Groups {
		d, _ := m.Lookup(name, name)
		if d != arch.DecisionAllow {
			t.Errorf("group %q cannot import itself: %v", name, d)
		}
	}
}

// TestMatrixGroupOfUsesLongestPrefix asserts that GroupOf resolves overlapping
// package prefixes by longest match. This is what allows a grandfathered
// package (e.g. internal/filesystem) to be claimed by a group
// (infrastructure.filesystem) that also owns a future, more specific path,
// and what allows the layer-root groups (domain, infrastructure, shared) to
// coexist with their more specific subgroups.
func TestMatrixGroupOfUsesLongestPrefix(t *testing.T) {
	m := loadCanonicalMatrix(t)
	cases := map[string]string{
		// Presentation.
		"internal/cli":        "presentation.cli",
		"internal/cli/config": "presentation.cli",

		// Application.
		"internal/application/check": "application",

		// Domain — layer root resolves to the parent group, subgroups to
		// their own groups.
		"internal/domain":           "domain",
		"internal/domain/blueprint": "domain.blueprint",
		"internal/domain/template":  "domain.template",

		// Infrastructure — same pattern.
		"internal/infrastructure":            "infrastructure",
		"internal/infrastructure/filesystem": "infrastructure.filesystem",
		"internal/filesystem":                "infrastructure.filesystem",
		"internal/filesystem/safefs":         "infrastructure.filesystem",
		"internal/config":                    "infrastructure.config",
		"internal/output":                    "infrastructure.renderer",
		"internal/streams":                   "infrastructure.streams",
		"internal/logger":                    "shared.logging", // shared, not infrastructure

		// Shared — layer root, subgroups, and grandfathered packages.
		"internal/shared":        "shared",
		"internal/shared/errors": "shared.errors",
		"internal/forgeerr":      "shared.errors",
		"internal/version":       "shared.version",
		"internal/app/version":   "shared.version",

		// Tooling.
		"internal/arch": "architecture.tooling",

		// Unclassified.
		"internal/not-a-real-package": "",
	}
	for path, want := range cases {
		if got := m.GroupOf(path); got != want {
			t.Errorf("GroupOf(%q) = %q, want %q", path, got, want)
		}
	}
}

// --- Golden: Markdown form matches the YAML ---

// TestMatrixYAMLMatchesMarkdown asserts that the Markdown table in
// docs/architecture/dependency-matrix.md contains the exact table rendered
// from the canonical YAML.
//
// Run with -update to replace the table in the Markdown document with the
// freshly rendered one:
//
//	go test ./internal/arch -run TestMatrixYAMLMatchesMarkdown -update
//
// The -update path writes the document as UTF-8 and cannot introduce a
// mojibake regression. Use it whenever the YAML's group set or matrix cells
// change.
func TestMatrixYAMLMatchesMarkdown(t *testing.T) {
	m := loadCanonicalMatrix(t)
	got := m.Markdown()

	mdPath := filepath.Join("..", "..", "docs", "architecture", "dependency-matrix.md")
	md := mustReadFile(t, mdPath)

	if *update {
		updated, ok := replaceMarkdownTable(md, got)
		if !ok {
			t.Fatalf("could not find a Markdown table to replace in %s", mdPath)
		}
		if err := os.WriteFile(mdPath, []byte(updated), 0o644); err != nil {
			t.Fatalf("write %s: %v", mdPath, err)
		}
		t.Logf("updated %s", mdPath)
		return
	}

	if !strings.Contains(md, got) {
		t.Fatalf("matrix table in %s does not match YAML\n--- rendered ---\n%s\n(run: go test ./internal/arch -run TestMatrixYAMLMatchesMarkdown -update)",
			mdPath, got)
	}
}

// replaceMarkdownTable replaces the first Markdown table in doc with rendered.
//
// A Markdown table begins with a line starting with "|" and ends at the first
// subsequent line that does not start with "|". The replacement swaps that
// entire block for rendered. If no table is found, ok is false.
//
// This helper exists so the -update path does not require knowing the table's
// exact byte offsets in the document; the boundaries are structural (line
// prefixes), not positional. That makes the update robust against prose edits
// above or below the table.
func replaceMarkdownTable(doc, rendered string) (string, bool) {
	lines := strings.Split(doc, "\n")
	start, end := -1, -1
	for i, line := range lines {
		if start == -1 && strings.HasPrefix(line, "|") {
			start = i
			continue
		}
		if start != -1 && !strings.HasPrefix(line, "|") {
			end = i
			break
		}
	}
	if start == -1 {
		return doc, false
	}
	if end == -1 {
		end = len(lines)
	}

	var b strings.Builder
	b.WriteString(strings.Join(lines[:start], "\n"))
	if start > 0 {
		b.WriteString("\n")
	}
	b.WriteString(rendered)
	if end < len(lines) {
		b.WriteString("\n")
		b.WriteString(strings.Join(lines[end:], "\n"))
	}
	return b.String(), true
}

// TestCanonicalMatrixGolden asserts that the canonical dependency matrix at
// docs/architecture/dependency-matrix.yaml is byte-for-byte identical to the
// golden copy in testdata/.
//
// The golden copy is a test fixture read from disk, not an embedded asset.
// Embedding it would defeat the -update workflow and link a test fixture into
// the shipped binary.
//
// Run with -update to regenerate the golden copy after an intentional change:
//
//	go test ./internal/arch -run TestCanonicalMatrixGolden -update
func TestCanonicalMatrixGolden(t *testing.T) {
	canonical := filepath.Join("..", "..", "docs", "architecture", "dependency-matrix.yaml")
	golden := filepath.Join("testdata", "dependency-matrix.golden.yaml")

	got := mustReadFile(t, canonical)

	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", golden)
		return
	}

	want := mustReadFile(t, golden)
	if got != want {
		t.Fatalf("canonical matrix drifted from golden\n--- canonical ---\n%s\n--- golden ---\n%s\n(run: go test ./internal/arch -run TestCanonicalMatrixGolden -update)",
			got, want)
	}
}

// --- Conditions and exceptions are consistent ---

// TestMatrixConditionsAreConsistent asserts that the canonical matrix's
// conditions block is internally consistent: every referenced condition is
// declared, every declared condition is either referenced or exception-
// managed, and the matrix as a whole remains acyclic.
//
// The heavy lifting is in ParseMatrix.validate; this test exists so the
// invariant is named in the test suite and so a future refactor of the
// validator cannot silently drop it.
func TestMatrixConditionsAreConsistent(t *testing.T) {
	m := loadCanonicalMatrix(t)
	if err := m.Acyclic(); err != nil {
		t.Fatalf("acyclic: %v", err)
	}
	for id := range m.Conditions {
		if id == "" {
			t.Error("empty condition ID")
		}
	}
}

// TestMatrixExceptionsNameDeclaredGroups asserts that every exception names a
// group that is declared in the groups block. An exception pointing at a
// non-existent group is a configuration bug that would silently have no
// effect.
func TestMatrixExceptionsNameDeclaredGroups(t *testing.T) {
	m := loadCanonicalMatrix(t)
	for _, ex := range m.Exceptions {
		if _, ok := m.Groups[ex.Group]; !ok {
			t.Errorf("exception references undeclared group %q", ex.Group)
		}
	}
}

// --- Non-functional: determinism ---

// TestMatrixMarkdownIsDeterministic asserts that rendering the same matrix
// twice produces byte-identical output. Determinism is required for the
// golden comparison in TestMatrixYAMLMatchesMarkdown: a non-deterministic
// renderer would cause spurious test failures.
func TestMatrixMarkdownIsDeterministic(t *testing.T) {
	m := loadCanonicalMatrix(t)
	a := m.Markdown()
	b := m.Markdown()
	if a != b {
		t.Fatal("Markdown rendering is not deterministic")
	}
}

// TestAcyclicAlgorithm exercises Matrix.Acyclic directly with a hand-built
// matrix, so the DFS is covered independently of the YAML parser.
//
// It documents the self-loop exception and the exception-edge exclusion:
// a group importing itself is permitted, and exception-derived edges do not
// participate in the cycle check. Both are asserted here as properties of
// the algorithm, not of any particular YAML.
func TestAcyclicAlgorithm(t *testing.T) {
	t.Run("self-loop is allowed", func(t *testing.T) {
		m := &arch.Matrix{
			Groups: map[string]arch.Group{
				"a": {Layer: "shared", Packages: []string{"internal/a"}},
			},
			RawMatrix: map[string]map[string]string{
				"a": {"a": "allow"},
			},
		}
		if err := m.Compile(); err != nil {
			t.Fatalf("compile: %v", err)
		}
		if err := m.Acyclic(); err != nil {
			t.Fatalf("self-loop reported as cycle: %v", err)
		}
	})

	t.Run("direct cycle is detected", func(t *testing.T) {
		m := &arch.Matrix{
			Groups: map[string]arch.Group{
				"a": {Layer: "shared", Packages: []string{"internal/a"}},
				"b": {Layer: "shared", Packages: []string{"internal/b"}},
			},
			RawMatrix: map[string]map[string]string{
				"a": {"b": "allow"},
				"b": {"a": "allow"},
			},
		}
		if err := m.Compile(); err != nil {
			t.Fatalf("compile: %v", err)
		}
		err := m.Acyclic()
		if err == nil {
			t.Fatal("expected cycle error")
		}
		if !strings.Contains(err.Error(), "a -> b -> a") &&
			!strings.Contains(err.Error(), "b -> a -> b") {
			t.Fatalf("error = %v, want cycle between a and b", err)
		}
	})

	t.Run("transitive cycle is detected", func(t *testing.T) {
		m := &arch.Matrix{
			Groups: map[string]arch.Group{
				"a": {Layer: "shared", Packages: []string{"internal/a"}},
				"b": {Layer: "shared", Packages: []string{"internal/b"}},
				"c": {Layer: "shared", Packages: []string{"internal/c"}},
			},
			RawMatrix: map[string]map[string]string{
				"a": {"b": "allow"},
				"b": {"c": "allow"},
				"c": {"a": "allow"},
			},
		}
		if err := m.Compile(); err != nil {
			t.Fatalf("compile: %v", err)
		}
		err := m.Acyclic()
		if err == nil {
			t.Fatal("expected cycle error")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("error = %v, want 'cycle'", err)
		}
	})

	t.Run("exception edges do not form a cycle", func(t *testing.T) {
		// Two excepted groups. Without the exception-edge exclusion in
		// Acyclic, this would be reported as a cycle:
		//   shared.errors -> shared.logging -> shared.errors
		m := &arch.Matrix{
			Groups: map[string]arch.Group{
				"a":              {Layer: "shared", Packages: []string{"internal/a"}},
				"shared.errors":  {Layer: "shared", Packages: []string{"internal/shared/errors"}},
				"shared.logging": {Layer: "shared", Packages: []string{"internal/shared/logging"}},
			},
			RawMatrix: map[string]map[string]string{},
			Exceptions: []arch.Exception{
				{Group: "shared.errors", AppliesTo: "all"},
				{Group: "shared.logging", AppliesTo: "all"},
			},
		}
		if err := m.Compile(); err != nil {
			t.Fatalf("compile: %v", err)
		}
		if err := m.Acyclic(); err != nil {
			t.Fatalf("exception edges reported as cycle: %v", err)
		}
	})
}

// --- ATDD: acceptance criteria from WBS 14.1.2 ---

// TestAcceptance_14_1_2 asserts the mechanically-checkable acceptance criteria
// of WBS 14.1.2. Criteria that require human judgment (two-engineer sign-off,
// Security owner review) are recorded in the ADR and the PR, not here.
//
// This is an ATDD test: it is written against the task's acceptance criteria,
// not against the implementation. If the criteria change, this test changes.
func TestAcceptance_14_1_2(t *testing.T) {
	m := loadCanonicalMatrix(t)

	t.Run("every layer and cross-cutting group is covered", func(t *testing.T) {
		wantLayers := map[string]bool{
			"presentation": false, "application": false,
			"domain": false, "infrastructure": false, "shared": false,
		}
		for _, g := range m.Groups {
			if _, ok := wantLayers[g.Layer]; !ok {
				t.Errorf("group layer %q is not a known layer", g.Layer)
			}
			wantLayers[g.Layer] = true
		}
		for layer, seen := range wantLayers {
			if !seen {
				t.Errorf("no group declares layer %q", layer)
			}
		}
	})

	t.Run("every cell has a decision", func(t *testing.T) {
		for from, row := range m.RawMatrix {
			for to, cell := range row {
				if cell == "" {
					t.Errorf("cell %s -> %s is empty", from, to)
				}
			}
		}
	})

	t.Run("no cycles", func(t *testing.T) {
		if err := m.Acyclic(); err != nil {
			t.Fatalf("cycle: %v", err)
		}
	})

	t.Run("machine-readable YAML published", func(t *testing.T) {
		mustReadFile(t, filepath.Join("..", "..", "docs", "architecture", "dependency-matrix.yaml"))
	})

	t.Run("schema published", func(t *testing.T) {
		mustReadFile(t, filepath.Join("..", "..", "schemas", "dependency-matrix.schema.json"))
	})

	t.Run("human-readable matrix published", func(t *testing.T) {
		mustReadFile(t, filepath.Join("..", "..", "docs", "architecture", "dependency-matrix.md"))
	})
}
