// Package config defines the Forge configuration model.
//
// In Phase 2, this package contains only the Config struct that the
// Dependencies injection struct references. The struct is empty.
// WBS 8.0 introduces the configuration loading and validation logic;
// WBS 8.2.1 freezes the schema.
//
// # Why the struct is empty in Phase 2
//
// Dependencies needs a field to hold the resolved configuration, but
// the configuration subsystem does not exist yet. Rather than define
// the field as `any` (which would lose type safety) or leave it out
// (which would require changing the struct later), the Config struct
// is defined now and populated later.
//
// An empty struct is a deliberate placeholder. It has zero size, it
// carries no data, and it can be replaced with a populated struct
// without changing any caller.
//
// # Import direction
//
// This package must not import any other internal package except
// other packages under internal/config. It is a leaf of the domain
// layer, and it must remain importable by every other domain package
// without creating cycles.
package config

// Config is the resolved Forge configuration for a single invocation.
//
// The struct is empty in Phase 2. WBS 8.2.1 introduces the first
// fields (LogLevel, OutputFormat, ConfigPath). The fields are added
// without changing the type's name or its import path, so existing
// callers (notably the Dependencies struct) continue to compile.
//
// # Zero value
//
// The zero value of Config is valid. It represents "no configuration
// was loaded, defaults apply". Code that reads Config must therefore
// handle the zero value gracefully.
type Config struct {
}
