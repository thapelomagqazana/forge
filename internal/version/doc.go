// Package version holds the canonical build metadata for Forge.
//
// # Purpose
//
// The four values in this package — Version, Commit, BuildDate, and
// Dirty — describe a single build of the Forge binary. They are domain
// data, not presentation, and they do not belong in internal/cli:
//
//   - internal/cli has a frozen single-symbol public surface
//     (WBS 4.1.1). Widening it to host a data model would violate
//     that contract.
//
//   - The values are consumed by multiple packages (the CLI root
//     command, the formatter, the binary entrypoint) and by future
//     phases (release tooling, SBOM generation, SARIF output) that
//     must not depend on the CLI layer.
//
//   - The variables are the write targets for linker injection
//     (-ldflags "-X"), which requires a stable, dedicated package
//     path. Putting them in internal/cli would couple the linker
//     prefix to a package whose name is chosen for a different
//     reason.
//
// # Contract
//
//   - Version, Commit, BuildDate, and Dirty are package-level string
//     variables. They are the ONLY write targets for -X in the
//     module. Their names are frozen.
//
//   - Every variable defaults to the empty string. This is
//     intentional: an empty value is an honest signal that the
//     binary was built without linker injection. Callers that want a
//     sentinel ("dev", "none", "unknown") apply it at the
//     presentation layer, not here.
//
//   - Get returns the four values as a 4-tuple. It is pure: no I/O,
//     no side effects, no allocation beyond the tuple itself (which
//     the compiler keeps on the stack). Callers may cache the
//     result; the values cannot change after package init.
//
//   - The package imports nothing. It is a leaf in the dependency
//     graph. The edge internal/cli → internal/version is one-way.
//
// # Boundary
//
//	internal/cli         ──imports──▶ internal/version
//	internal/app/version ──imports──▶ internal/version
//	cmd/forge            ──imports──▶ internal/version (wiring only)
//
// The reverse edges must never exist. See docs/architecture.md
// "Version Package" for the full rationale.
//
// # Linker Injection
//
// Values are injected at build time via:
//
//	go build -ldflags "\
//	  -X github.com/thapelomagqazana/forge/internal/version.Version=1.2.3 \
//	  -X github.com/thapelomagqazana/forge/internal/version.Commit=abc1234 \
//	  -X github.com/thapelomagqazana/forge/internal/version.BuildDate=2026-10-09T12:00:00Z \
//	  -X github.com/thapelomagqazana/forge/internal/version.Dirty=false"
//
// The prefix "github.com/thapelomagqazana/forge/internal/version."
// must match the module path declared in go.mod exactly. Any drift
// between the module path and the linker target silently produces a
// binary with empty values; this is caught by
// scripts/reproducible/verify-resolution.sh.
//
// # Why exported variables
//
// The -X linker flag requires exported identifiers. An earlier draft
// proposed unexported variables plus a build-tag-selected declaration
// to hide them; that adds a second declaration and a second build
// path for no benefit. The package is a leaf, so there is no public
// surface to widen.
//
// # Why empty defaults
//
// A non-empty default such as "dev" collapses two distinct states —
// "this binary was built without version information" and "this
// binary is a development build" — into one. The first state is a
// signal that the build pipeline is misconfigured; the second is
// normal during development. Keeping the default empty lets the
// formatter (internal/app/version) decide how to render each state
// distinctly.
//
// # Why Dirty is a string
//
// The -X linker flag can only set string variables. A bool field
// would require either a two-variable convention (e.g. Clean and
// Dirty) or a parse step. A single string is simpler to inject; the
// consumer compares against "true", or treats "" as "unknown" rather
// than "clean". The package does not validate the value; the build
// system is responsible for producing "true" or "false".
package version
