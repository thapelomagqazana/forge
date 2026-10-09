// Package config defines the Forge configuration model.
//
// # What this package is
//
// This package defines the data types that represent a resolved Forge
// configuration. A "resolved" configuration is the result of loading
// a config file, applying environment-variable overrides, applying
// CLI flag overrides, and falling back to defaults. The resolution
// logic itself lives in internal/config once WBS 8.0 implements it;
// for now, the package contains only the empty Config struct.
//
// # What this package is not
//
// This package does not load files, does not parse YAML, and does not
// read environment variables. Those operations belong to the loader
// that WBS 8.4 introduces. Keeping this package pure means it can be
// imported by every other package, including the domain layer,
// without creating a dependency on I/O.
//
// # Relationship to other packages
//
// This package is imported by:
//
//   - internal/cli, which reads the resolved configuration from the
//     Dependencies struct.
//   - internal/blueprint, which consumes configuration fields when
//     constructing a Blueprint (Phase 3).
//
// This package imports nothing from the Forge module. It is a leaf.
package config
