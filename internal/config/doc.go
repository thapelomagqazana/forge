// Package config defines the CLI configuration for Forge.
//
// # Scope (frozen by WBS 8.1.1)
//
// The CLI configuration is deliberately small. It describes how the
// CLI behaves; it does not describe the project the CLI operates on.
// In Phase 2, the CLI configuration has exactly three conceptual
// fields:
//
//	log_level      Diagnostic verbosity. Consumer: WBS 12.0.
//	output_format  Default output format ("text" or "json").
//	               Consumer: commands that accept --format.
//	config_path    Metadata: where the config file was loaded from.
//	               Consumer: diagnostics, `forge config`.
//
// The first two fields are read from the config file. The third is
// not: it is a resolution result, written by the config loader after
// it resolves the file's path. `forge config` displays it.
//
// # Out of scope (Phase 3 or later)
//
// The following categories are project-level configuration. They
// belong to the project configuration subsystem (Blueprint,
// Phase 3) or to later phases:
//
//	Project name, type, language       -> Phase 3 (project config)
//	Testing, CI, database, container   -> Phase 3 (project config)
//	Policies                           -> Phase 8
//	Components                         -> Phase 12
//	Templates                          -> Phase 4
//
// The boundary between CLI configuration and project configuration
// is the boundary between this package and the project
// configuration package (Blueprint, Phase 3). The two packages are
// separate; this package does not depend on the project
// configuration package.
//
// # Explicitly rejected (ever)
//
// Three categories are forbidden from CLI configuration in every
// phase:
//
//   - Command-specific flags. Flags are invocation-scoped, not
//     persistent. A flag's value belongs on the command line, not
//     in a config file. Putting a flag in config would make
//     `forge version --format json` depend on a file the user did
//     not pass; the surprising dependency is a bug, not a feature.
//
//   - Secrets, tokens, credentials. Config files are often
//     committed. A secret in a config file is a leak. Forge never
//     stores secrets in CLI configuration; a future secret
//     mechanism (for example, an OIDC token for a registry) uses
//     the operating system's credential store, not a config file.
//
//   - Repository paths. The CLI works in any repository. The
//     repository's identity (its name, its directory, its
//     structure) belongs to the repository's own `forge.yaml`,
//     which is project configuration (Phase 3). Putting a
//     repository path in CLI config would couple the CLI to one
//     repository; the user would have to reconfigure the CLI when
//     moving between repositories.
//
// # Adding a field
//
// Adding a field to the CLI configuration requires an ADR. The ADR
// must name the field, its type, its default, its consumer, and
// the WBS item that requires it. The scope test
// (`scope_test.go`) fails if the field count changes without the
// ADR's corresponding update to the test.
//
// # Why a separate package
//
// The configuration type is a leaf: it imports nothing from Forge
// and is imported by `internal/cli` (via `Dependencies.Config`) and
// by any future command that reads the CLI's output format default.
// Keeping it a leaf means a future change to the config type does
// not ripple into the rest of the module.
//
// # No I/O
//
// This package defines the configuration's shape. It does not read
// files, does not parse YAML, and does not resolve paths. Those
// responsibilities belong to a future loader package (WBS 8.3);
// the loader constructs a Config value from whatever source it
// uses and hands the value to its caller.
//
// The separation keeps the type pure: a Config value is a value,
// and any code that holds one can reason about it without knowing
// where it came from.
package config
