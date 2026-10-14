package arch

import (
	"fmt"
	"sort"
	"strings"
)

// Layer identifies one of the five architectural layers of Forge.
//
// The zero value is LayerUnknown and is intentionally invalid: an unclassified
// package must fail the architecture test rather than silently pass.
type Layer string

const (
	// LayerUnknown is the zero value. It is never a valid layer for a package
	// under internal/; encountering it in the classifier output is a test
	// failure (see layers_test.go).
	LayerUnknown Layer = ""

	// LayerPresentation contains the CLI surface: Cobra commands, flag parsing,
	// help text, human/JSON rendering decisions, and the single exit boundary.
	// It is the composition root and may import every other layer.
	LayerPresentation Layer = "presentation"

	// LayerApplication contains use-case orchestration: one service per command.
	// Services are plain structs with injected interfaces and no direct I/O.
	// May import Domain and Shared. Must not import Infrastructure or
	// Presentation.
	LayerApplication Layer = "application"

	// LayerDomain contains pure business logic, organized into bounded contexts
	// (blueprint, template, component, policy, foundation). Defines the
	// interfaces it needs (consumer-owned). May import Shared only.
	LayerDomain Layer = "domain"

	// LayerInfrastructure contains adapters implementing domain interfaces:
	// filesystem, config, renderer, registry, git, logger implementation, error
	// formatting. May import Domain and Shared. Must not import Application or
	// Presentation.
	LayerInfrastructure Layer = "infrastructure"

	// LayerShared contains cross-cutting primitives with no upward dependencies:
	// error taxonomy, logger interfaces, version metadata. Imports nothing
	// above it (i.e. nothing except itself).
	LayerShared Layer = "shared"
)

// AllLayers returns every valid layer in a stable order, suitable for golden
// output and table-driven tests.
func AllLayers() []Layer {
	return []Layer{
		LayerPresentation,
		LayerApplication,
		LayerDomain,
		LayerInfrastructure,
		LayerShared,
	}
}

// allowedImport reports whether a package in layer `from` is permitted to
// import a package in layer `to`, per the matrix in
// docs/architecture/clean-architecture.md §3.
//
// The matrix is intentionally hard-coded here (rather than parsed from the
// markdown) so that the test is self-contained and so that a docs edit cannot
// silently weaken the guardrail. When the model changes, both this function and
// the docs must change in the same PR; the golden test in layers_test.go
// asserts that the docs and this table agree.
func allowedImport(from, to Layer) bool {
	if from == LayerUnknown || to == LayerUnknown {
		return false
	}
	switch from {
	case LayerPresentation:
		// Composition root: may import everything.
		return true
	case LayerApplication:
		switch to {
		case LayerPresentation, LayerInfrastructure:
			return false
		default:
			return true
		}
	case LayerDomain:
		switch to {
		case LayerPresentation, LayerApplication, LayerInfrastructure:
			return false
		default:
			return true
		}
	case LayerInfrastructure:
		switch to {
		case LayerPresentation, LayerApplication:
			return false
		default:
			return true
		}
	case LayerShared:
		// Shared imports nothing above it. In practice it imports only stdlib
		// and other shared packages; the matrix-level rule is "only shared".
		return to == LayerShared
	default:
		return false
	}
}

// ClassificationRule maps a package import-path prefix to a layer.
//
// The classifier uses longest-prefix-match. Rules are exported for tests so
// that the golden test can assert the rule set is stable.
type ClassificationRule struct {
	// Prefix is an import-path prefix, e.g. "internal/application".
	Prefix string
	// Layer is the layer assigned to any package whose import path equals
	// Prefix or begins with Prefix+"/".
	Layer Layer
}

// Grandfather records a package that predates the layer model and is
// temporarily exempt from classification.
//
// Every entry MUST have a RemovalPhase. The arch test fails if a grandfather
// entry has no removal phase, and a separate test asserts the allowlist shrinks
// over time (no new entries without an ADR).
type Grandfather struct {
	// Package is the exact import-path suffix under internal/, e.g.
	// "internal/logger" or "internal/filesystem".
	Package string
	// Layer is the layer the package will eventually belong to.
	Layer Layer
	// Reason is a short human-readable justification.
	Reason string
	// RemovalPhase names the WBS phase that will relocate the package.
	RemovalPhase string
}

// DefaultRules returns the canonical classifier rules for the forge module.
//
// Rules are ordered by descending prefix length conceptually; the classifier
// itself sorts by length, so callers need not pre-sort.
func DefaultRules() []ClassificationRule {
	return []ClassificationRule{
		{Prefix: "internal/cli", Layer: LayerPresentation},
		{Prefix: "internal/application", Layer: LayerApplication},
		{Prefix: "internal/domain", Layer: LayerDomain},
		{Prefix: "internal/infrastructure", Layer: LayerInfrastructure},
		{Prefix: "internal/shared", Layer: LayerShared},
	}
}

// DefaultGrandfathers returns the packages that predate the layer model.
//
// This list MUST shrink, never grow, without a superseding ADR. The arch test
// asserts every entry carries a RemovalPhase.
func DefaultGrandfathers() []Grandfather {
	return []Grandfather{
		{Package: "internal/config", Layer: LayerInfrastructure,
			Reason: "config adapter; predates layer model", RemovalPhase: "14.2.x"},
		{Package: "internal/filesystem", Layer: LayerInfrastructure,
			Reason: "filesystem primitives + policy; ADR-035", RemovalPhase: "14.2.x"},
		{Package: "internal/output", Layer: LayerInfrastructure,
			Reason: "renderer adapter; predates layer model", RemovalPhase: "14.2.x"},
		{Package: "internal/streams", Layer: LayerInfrastructure,
			Reason: "stream adapters; predates layer model", RemovalPhase: "14.2.x"},
		{Package: "internal/logger", Layer: LayerShared,
			Reason: "interface + slog adapter co-located; split tracked", RemovalPhase: "14.3.x"},
		{Package: "internal/forgeerr", Layer: LayerShared,
			Reason: "error taxonomy; predates layer model", RemovalPhase: "14.3.x"},
		{Package: "internal/version", Layer: LayerShared,
			Reason: "version metadata; predates layer model", RemovalPhase: "14.3.x"},
		{Package: "internal/app/version", Layer: LayerApplication,
			Reason: "thin version service; will fold into internal/application", RemovalPhase: "14.2.x"},
	}
}

// Classify returns the layer for an internal package import path suffix
// (e.g. "internal/application/check").
//
// It applies DefaultRules and DefaultGrandfathers together, using
// longest-prefix match, with a "/" boundary. At equal prefix length a rule
// beats a grandfather: a rule is a positive statement about the target model,
// a grandfather is a debt exception, and the target model should win ties.
//
// If nothing matches, it returns LayerUnknown; callers MUST treat that as an
// error (TestEveryInternalPackageIsClassified enforces this).
//
// The input is the path suffix relative to the module root, using forward
// slashes. Callers should obtain it via module-relative path trimming (see
// layers_test.go).
func Classify(internalPath string) Layer {
	if internalPath == "" {
		return LayerUnknown
	}

	type candidate struct {
		prefix string
		layer  Layer
		isRule bool
	}

	var best *candidate

	consider := func(prefix string, layer Layer, isRule bool) {
		if !hasPrefix(internalPath, prefix) {
			return
		}
		switch {
		case best == nil:
			// first match
		case len(prefix) > len(best.prefix):
			// longer prefix wins
		case len(prefix) == len(best.prefix) && isRule && !best.isRule:
			// equal length, rule beats grandfather
		default:
			return
		}
		best = &candidate{prefix: prefix, layer: layer, isRule: isRule}
	}

	for _, r := range DefaultRules() {
		consider(r.Prefix, r.Layer, true)
	}
	for _, g := range DefaultGrandfathers() {
		consider(g.Package, g.Layer, false)
	}

	if best == nil {
		return LayerUnknown
	}
	return best.layer
}

// hasPrefix reports whether path equals prefix or begins with prefix+"/".
//
// This is stricter than strings.HasPrefix: "internal/applicationfoo" must not
// match the rule for "internal/application".
func hasPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// MatrixGolden renders the allowed-dependency matrix as a stable text block.
//
// It is the single source of truth for the golden test and for the diagram in
// docs/architecture/clean-architecture.md §3. Any change to allowedImport that
// is not accompanied by a docs update will fail the golden test.
func MatrixGolden() string {
	layers := AllLayers()
	var b strings.Builder
	b.WriteString("importer \\ imported")
	for _, l := range layers {
		fmt.Fprintf(&b, "\t%s", l)
	}
	b.WriteString("\n")
	for _, from := range layers {
		b.WriteString(string(from))
		for _, to := range layers {
			if allowedImport(from, to) {
				b.WriteString("\tallow")
			} else {
				b.WriteString("\tdeny")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// SortedGrandfathers returns grandfather entries sorted by package path.
// Stable ordering keeps golden output deterministic.
func SortedGrandfathers() []Grandfather {
	gs := append([]Grandfather(nil), DefaultGrandfathers()...)
	sort.Slice(gs, func(i, j int) bool { return gs[i].Package < gs[j].Package })
	return gs
}
