// Package arch encodes Forge's clean architecture layer model as an executable
// contract.
//
// The package has two responsibilities:
//
//  1. Classify every package under internal/ into one of five layers
//     (Presentation, Application, Domain, Infrastructure, Shared).
//  2. Assert that the import graph between layers matches the allowed
//     dependency matrix published in docs/architecture/clean-architecture.md.
//
// The classification is "closed by default": a package that is neither matched
// by a layer prefix nor listed in the grandfather allowlist is a test failure.
// This forces every new package to make an explicit architectural choice.
//
// # Why this is a Go test and not a linter
//
// Go's go/packages gives us the real, resolved import graph — including build
// tags, module replacements, and test-only imports — with no separate toolchain.
// The test runs in the normal `go test ./...` loop, so it cannot be skipped by
// forgetting to run a linter. A pre-commit wrapper (scripts/arch/verify-layers.sh)
// makes the fast path even faster.
//
// # What this package does NOT do
//
//   - It does not enforce intra-layer rules (e.g. "no os in domain"). Those are
//     enforced by the domain's own package tests and by code review.
//   - It does not analyze the standard library import graph. Only packages under
//     the module's own internal/ tree are classified.
//
// # Stability
//
// This package is internal. Its API may change as the model evolves; the model
// itself is versioned by ADR-040.
package arch
