// Package version contains the version application service.
//
// This file is the single point of import for the internal/version
// library package. Every other file in internal/app/version reaches
// the library through the accessors and type aliases declared here.
//
// # Why this file exists
//
// Before WBS 4.3.1, the build variables lived in internal/cli. That
// created an import cycle: internal/cli imports
// internal/app/version (for the `forge version` handler), and
// internal/app/version would need to import internal/cli (for the
// variables). Go rejects the cycle.
//
// WBS 4.3.1 resolves the cycle by relocating the variables to
// internal/version, a leaf package that both internal/cli and
// internal/app/version can import without creating a cycle. This
// file is the only import of that package within
// internal/app/version.
//
// # Why the import is isolated
//
// Isolating the import in one file has three benefits:
//
//  1. A future change to the library's API touches this file only.
//     The service's public surface (Info, Get, GetContext, Raw,
//     the writers) does not mention the library's types directly,
//     so callers of the service are insulated from the library.
//
//  2. The service's other files (service.go, format.go, and the
//     format_*_test files) read as pure Go: they reference the
//     service's own identifiers, not a foreign package.
//
//  3. The isolation is a structural rule, not a stylistic one. It
//     is asserted by TestNonFunctional_BuildInfoIsolatesTheImport,
//     which reads both files and fails if the import appears
//     anywhere but here. The rule is documented in
//     docs/architecture.md § 11.11.
//
// # Why the import alias
//
// The imported package is named "version", and the containing
// package is also named "version". The alias "forgeversion"
// disambiguates. An alternative would be to rename the imported
// package to something like "buildvars"; the collision is a naming
// smell that will be reconsidered if a third package with the same
// name is ever added. For now, the alias is local and unambiguous:
// "forgeversion" names the Forge build metadata, not the general
// concept of versioning.
//
// # Purity
//
// Every function in this file is pure, for the same reason the
// underlying library functions are pure. They read package-level
// variables or the runtime's build info and return values. They do
// not touch the filesystem, the network, the environment, or any
// other mutable global state.
package version

import (
	"io"

	forgeversion "github.com/thapelomagqazana/forge/internal/version"
)

// -----------------------------------------------------------------------------
// Type re-exports
// -----------------------------------------------------------------------------
//
// The service exposes the library's types under its own namespace so
// a caller of the service does not need to import the library to
// name a type. The re-exports are aliases, not definitions: a value
// of the service's Metadata is identical to a value of the library's
// Metadata, and the two are interchangeable in every context.
//
// # Why aliases and not definitions
//
// A named type (type Metadata libversion.Metadata) would require a
// conversion at every boundary. It would also break == comparisons
// and errors.As targets that name the library's type. The alias
// form (type Metadata = libversion.Metadata) is the correct choice
// for a facade.

// Metadata is the wire shape of the five linker fields. It is an
// alias for the library's Metadata type.
type Metadata = forgeversion.Metadata

// Provenance is the build provenance read from
// runtime/debug.ReadBuildInfo. It is an alias for the library's
// Provenance type.
type Provenance = forgeversion.Provenance

// Dependency is one module the binary links against. It is an
// alias for the library's Dependency type.
type Dependency = forgeversion.Dependency

// Setting is one Go build setting. It is an alias for the
// library's Setting type.
type Setting = forgeversion.Setting

// BuildKind names the environment a binary was built in. It is an
// alias for the library's BuildKind type.
type BuildKind = forgeversion.BuildKind

// The three build kinds, re-exported under the service's namespace.
//
// The constants are declared as typed constants in the library, not
// as untyped string literals. Re-exporting them preserves the type,
// so a caller can use appversion.BuildKindRelease in a context that
// expects forgeversion.BuildKind and vice versa.
const (
	BuildKindRelease = forgeversion.BuildKindRelease
	BuildKindCI      = forgeversion.BuildKindCI
	BuildKindLocal   = forgeversion.BuildKindLocal
)

// -----------------------------------------------------------------------------
// Accessors
// -----------------------------------------------------------------------------

// buildInfo reads the five linker variables from the library.
//
// The function is the service's only direct reader of the library's
// package variables. A future change to how the values are
// populated (for example, reading them from a different source)
// touches this function.
//
// The signature is a tuple, not a struct, because the callers in
// this package destructure the values positionally. The order is
// (version, commit, buildDate, dirty, buildType). The order matches
// the order the library's GetAll returns its fields.
//
// # Purity
//
// buildInfo is pure. It reads five package-level variables and
// returns them.
func buildInfo() (version, commit, buildDate, dirty, buildType string) {
	i := forgeversion.GetAll()
	return i.Version, i.Commit, i.BuildDate, i.Dirty, i.BuildType
}

// readProvenance reads the build provenance from the library.
//
// The function is the service's only direct reader of the library's
// provenance. A future change to how the provenance is read touches
// this function.
//
// # Allocation
//
// The library's ReadProvenance allocates a Provenance value and two
// slices (Dependencies and Settings). The allocation is bounded by
// the size of the module graph, which for Forge is a few dozen
// entries. Callers that need a strict allocation-free path call
// buildInfo (which does not allocate) rather than readProvenance.
func readProvenance() Provenance {
	return forgeversion.ReadProvenance()
}

// metadataOf constructs a Metadata from the five linker variables.
//
// It is the service's only direct caller of the library's
// MetadataOf constructor. The constructor normalizes an empty or
// unrecognized buildKind to BuildKindLocal; this accessor preserves
// that behavior.
func metadataOf(version, commit, buildDate, dirty, buildKind string) Metadata {
	return forgeversion.MetadataOf(version, commit, buildDate, dirty, buildKind)
}

// -----------------------------------------------------------------------------
// Writers
// -----------------------------------------------------------------------------

// writeMetadataJSON writes the five-field JSON form of m to w.
//
// It is the service's only direct caller of the library's
// WriteJSON function. The output validates against
// schemas/version.schema.json.
func writeMetadataJSON(w io.Writer, m Metadata) error {
	return forgeversion.WriteJSON(w, m)
}

// writeMetadataFullJSON writes the extended JSON form of m and p
// to w.
//
// It is the service's only direct caller of the library's
// WriteFullJSON function. The output validates against
// schemas/version-full.schema.json.
func writeMetadataFullJSON(w io.Writer, m Metadata, p Provenance) error {
	return forgeversion.WriteFullJSON(w, m, p)
}

// writeMetadataVerbose writes the verbose human-readable form of m
// and p to w.
//
// It is the service's only direct caller of the library's
// WriteVerbose function. The output format is documented in
// docs/reference/version-output.md §"Verbose format".
func writeMetadataVerbose(w io.Writer, m Metadata, p Provenance) error {
	return forgeversion.WriteVerbose(w, m, p)
}
