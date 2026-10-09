package version

import (
	forgeversion "github.com/thapelomagqazana/forge/internal/version"
)

// buildInfo reads the build-time values from internal/version.
//
// # Why this file exists
//
// Before WBS 4.3.1, this accessor read the build variables from
// internal/cli. That created an import cycle: internal/cli imports
// internal/app/version (for the `forge version` handler), and
// internal/app/version would need to import internal/cli (for the
// variables). Go rejects the cycle.
//
// WBS 4.3.1 resolves the cycle by relocating the variables to
// internal/version, a leaf package that both internal/cli and
// internal/app/version can import without creating a cycle. The
// accessor now reads from that package.
//
// # Why keep the accessor at all
//
// The accessor is a two-line function. It could be inlined into Get
// in service.go. Keeping it separate has two benefits:
//
//  1. It isolates the import of internal/version in one file. A
//     reader of service.go sees a pure Get that reads no imports
//     besides the local call to buildInfo.
//
//  2. It is the seam at which a future refactor can change the
//     source of the build-time values without touching Get. For
//     example, if the values are later populated from
//     runtime/debug.ReadBuildInfo, only this file changes.
//
// # Why the import alias
//
// The imported package is named "version", and the containing package
// is also named "version". The alias "forgeversion" disambiguates.
// An alternative would be to rename the imported package to something
// like "buildvars"; the collision is a naming smell that will be
// reconsidered if a third package with the same name is ever added.
// For now, the alias is local and unambiguous: "forgeversion" names
// the Forge build metadata, not the general concept of versioning.
//
// # Purity
//
// buildInfo is pure, for the same reason forgeversion.Get is pure. It
// reads three package-level variables and returns them.
func buildInfo() (version, commit, buildDate, dirty string) {
	return forgeversion.Get()
}
