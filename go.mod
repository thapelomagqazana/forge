// Module: forge
//
// Forge is a cross-platform CLI for defining, generating, validating, and
// evolving software project foundations as code.
//
// Invariants (see WBS 2.1.1):
//
//   - The module path MUST match the canonical GitHub URL exactly.
//   - The module path MUST contain no uppercase characters (Go convention).
//   - The module path MUST remain stable for the lifetime of the project,
//     because every import statement in every package depends on it.
//   - The `go` directive MUST reflect the minimum supported Go version.
//   - The `toolchain` directive MUST pin the current development toolchain,
//     so that all contributors and CI resolve identical standard-library
//     behaviour.
//
// Changing the module path is a project-wide breaking change and requires
// an ADR (see docs/decisions/).
//
// See:
//   - WBS 2.1.2  (Go version matrix)
//   - WBS 2.3.1  (reproducible module & toolchain configuration)
//   - docs/dependency-policy.md
//   - docs/development.md

module github.com/thapelomagqazana/forge

go 1.23

toolchain go1.23.4

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)
