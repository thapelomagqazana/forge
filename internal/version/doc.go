// Package version holds the build-time version metadata for the
// running Forge binary.
//
// # What this package is
//
// This package is the single source of truth for the values that
// describe a Forge build: the semantic version, the short Git commit
// hash, and the build timestamp. Every consumer — the CLI, the
// version application service, and any future subsystem — reads them
// from here.
//
// # Why a dedicated package
//
// Before WBS 4.3.1, the three variables lived in internal/cli. That
// worked while the CLI was the only consumer. WBS 4.3.1 introduced
// internal/app/version as a second consumer, and an import cycle
// appeared:
//
//	internal/cli  ──────►  internal/app/version
//	     ▲                        │
//	     │                        │
//	     └────────────────────────┘
//	              (cycle)
//
// internal/cli imports internal/app/version for the `forge version`
// handler. internal/app/version would need to import internal/cli for
// the build variables. Go rejects the cycle.
//
// Moving the variables to this package breaks the cycle. This package
// imports nothing from Forge; both internal/cli and
// internal/app/version import it. The dependency graph becomes a
// directed acyclic graph:
//
//	internal/cli        internal/app/version
//	         \            /
//	          v          v
//	          internal/version
//
// # Why the package lives at internal/version, not internal/cli/version
//
// Go's cycle rules operate at package granularity, not directory
// granularity. A package at internal/cli/version would still be
// imported as a distinct package, but the cycle would remain if any
// package that internal/cli imports — directly or transitively — also
// imports internal/cli/version. The safe location is outside any
// package that could close the cycle. internal/version is that
// location.
//
// # Why not define the variables in internal/app/version
//
// The variables are injected at link time via the -X flag. The -X
// flag requires the fully-qualified package path of the variable:
//
//	-X github.com/thapelomagqazana/forge/internal/version.Version=<value>
//
// If the variables were defined in internal/app/version, the ldflags
// path would be:
//
//	-X github.com/thapelomagqazana/forge/internal/app/version.Version=<value>
//
// That path is longer, and it couples the build system to an
// application-layer package. The build system should not need to know
// which layer of Forge owns the version metadata. A dedicated
// internal/version package is a neutral home.
//
// # Why the variables are exported
//
// The -X flag requires the target variable to be exported. The
// variables are therefore declared as `var Version string`, not
// `const`. A `const` cannot be injected at link time.
//
// The variables must not be assigned after package initialization.
// Code that reads them should call Get, which is the stable API.
//
// # Stability
//
// The import path of this package is part of the build system's
// contract. Any change to the path must be accompanied by a change to
// every build script, CI configuration, and release pipeline that
// injects these values. After WBS 4.3.1, the path is stable; future
// relocations are not anticipated.
package version
