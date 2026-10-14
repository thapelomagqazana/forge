package arch_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/thapelomagqazana/forge/internal/arch"
)

// modulePath is the Go module path of the forge repository. It is read from
// go.mod at test time so that forks and renamed modules still work.
func modulePath(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	t.Fatal("go.mod has no module directive")
	return ""
}

// internalPath converts an absolute import path to a module-relative suffix
// rooted at "internal/" (or returns ok=false if the package is not under
// internal/).
func internalPath(mod, importPath string) (string, bool) {
	rel := strings.TrimPrefix(importPath, mod+"/")
	if !strings.HasPrefix(rel, "internal/") {
		return "", false
	}
	return rel, true
}

var update = flag.Bool("update", false, "update golden files")

func TestLayerMatrixGolden(t *testing.T) {
	got := arch.MatrixGolden()
	goldenPath := filepath.Join("testdata", "layer_diagram.golden.txt")

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run: go test ./internal/arch -run TestLayerMatrixGolden -update)", err)
	}
	if got != string(want) {
		t.Fatalf("matrix drift\n--- got ---\n%s\n--- want ---\n%s\n(run: go test ./internal/arch -run TestLayerMatrixGolden -update)",
			got, string(want))
	}
}

// TestEveryInternalPackageIsClassified is the "closed by default" guardrail.
//
// It loads the full package graph under ./internal/... and asserts that every
// package resolves to a known layer. A new package that is neither under a
// layer prefix nor in the grandfather allowlist fails here, forcing an
// architectural decision.
func TestEveryInternalPackageIsClassified(t *testing.T) {
	mod := modulePath(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports,
		Dir:  "..",
	}
	pkgs, err := packages.Load(cfg, "./internal/...")
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages loaded; check pattern")
	}
	var unclassified []string
	for _, p := range pkgs {
		rel, ok := internalPath(mod, p.PkgPath)
		if !ok {
			continue
		}
		if arch.Classify(rel) == arch.LayerUnknown {
			unclassified = append(unclassified, rel)
		}
	}
	if len(unclassified) > 0 {
		t.Fatalf("unclassified internal packages (add a rule or a grandfather entry):\n  %s",
			strings.Join(unclassified, "\n  "))
	}
}

// TestNoUpwardImports asserts the dependency matrix against the real import
// graph. This is the enforcement of the matrix, not just its documentation.
//
// Negative case coverage:
//   - domain importing infrastructure  -> fail
//   - domain importing application     -> fail
//   - application importing infra      -> fail
//   - shared importing anything else   -> fail
//
// Edge / boundary:
//   - presentation importing anything  -> allowed
//   - infrastructure importing domain  -> allowed
//   - shared importing shared          -> allowed
func TestNoUpwardImports(t *testing.T) {
	mod := modulePath(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports,
		Dir:  "..",
	}
	pkgs, err := packages.Load(cfg, "./internal/...")
	if err != nil {
		t.Fatalf("load packages: %v", err)
	}

	type violation struct {
		from, fromLayer string
		to, toLayer     string
	}
	var violations []violation

	for _, p := range pkgs {
		fromRel, ok := internalPath(mod, p.PkgPath)
		if !ok {
			continue
		}
		fromLayer := arch.Classify(fromRel)
		if fromLayer == arch.LayerUnknown {
			continue // covered by TestEveryInternalPackageIsClassified
		}
		for importedPath := range p.Imports {
			toRel, ok := internalPath(mod, importedPath)
			if !ok {
				continue // stdlib or external module; not classified
			}
			toLayer := arch.Classify(toRel)
			if toLayer == arch.LayerUnknown {
				continue // covered by classification test
			}
			if !arch.AllowedImportForTest(fromLayer, toLayer) {
				violations = append(violations, violation{
					from: fromRel, fromLayer: string(fromLayer),
					to: toRel, toLayer: string(toLayer),
				})
			}
		}
	}
	if len(violations) > 0 {
		var b strings.Builder
		for _, v := range violations {
			fmt.Fprintf(&b, "  %s (%s) -> %s (%s)\n",
				v.from, v.fromLayer, v.to, v.toLayer)
		}
		t.Fatalf("layer violations:\n%s", b.String())
	}
}

// TestGrandfathersHaveRemovalPhase is a policy test: no permanent exceptions.
func TestGrandfathersHaveRemovalPhase(t *testing.T) {
	for _, g := range arch.DefaultGrandfathers() {
		if g.RemovalPhase == "" {
			t.Errorf("grandfather %q has no removal phase", g.Package)
		}
		if g.Reason == "" {
			t.Errorf("grandfather %q has no reason", g.Package)
		}
		if g.Layer == arch.LayerUnknown {
			t.Errorf("grandfather %q has no target layer", g.Package)
		}
	}
}

// TestGrandfathersSorted asserts deterministic ordering for golden stability.
func TestGrandfathersSorted(t *testing.T) {
	gs := arch.SortedGrandfathers()
	for i := 1; i < len(gs); i++ {
		if gs[i-1].Package > gs[i].Package {
			t.Fatalf("grandfathers not sorted: %q before %q",
				gs[i-1].Package, gs[i].Package)
		}
	}
}

// TestClassify covers classification directly, including edge cases around
// prefix matching that strings.HasPrefix would get wrong.
func TestClassify(t *testing.T) {
	cases := []struct {
		in   string
		want arch.Layer
	}{
		{"internal/cli", arch.LayerPresentation},
		{"internal/cli/foo", arch.LayerPresentation},
		{"internal/application", arch.LayerApplication},
		{"internal/application/check", arch.LayerApplication},
		{"internal/domain/blueprint", arch.LayerDomain},
		{"internal/infrastructure/fs", arch.LayerInfrastructure},
		{"internal/shared/errors", arch.LayerShared},

		// Grandfathered exact matches.
		{"internal/logger", arch.LayerShared},
		{"internal/forgeerr", arch.LayerShared},
		{"internal/filesystem/safefs", arch.LayerInfrastructure}, // prefix, not exact

		// Edge: prefix boundary must be respected.
		{"internal/applicationfoo", arch.LayerUnknown}, // not "internal/application/..."
		{"internal/clix", arch.LayerUnknown},
		{"internal/domainless", arch.LayerUnknown},

		// Edge: empty / malformed.
		{"", arch.LayerUnknown},
		{"internal", arch.LayerUnknown},
		{"internal/", arch.LayerUnknown},
		{"other/package", arch.LayerUnknown},

		// Boundary: deepest known prefix wins.
		{"internal/application/check/foo", arch.LayerApplication},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := arch.Classify(tc.in); got != tc.want {
				t.Fatalf("Classify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
