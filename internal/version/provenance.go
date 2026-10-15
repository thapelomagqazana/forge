// provenance.go reads Go's own build metadata via
// runtime/debug.ReadBuildInfo.
//
// # What Go embeds
//
// A Go binary built with -buildvcs=true embeds a debug.BuildInfo
// value containing:
//
//   - the Go toolchain version (e.g. "go1.23.4")
//   - the main module path and version
//   - the list of module dependencies with their versions
//   - VCS information: the revision (a full SHA), the commit time,
//     and a boolean indicating whether the tree was modified
//   - the build settings: -trimpath, -buildmode, CGO_ENABLED, and
//     so on
//
// runtime/debug.ReadBuildInfo returns that value, or (nil, false) if
// the binary was built without module support (rare; the module is
// required for modern Go).
//
// # Why a separate reader
//
// The five linker variables in version.go are injected by the build
// pipeline. Provenance is read from the binary itself. They have
// different sources, different lifetimes, and different failure
// modes. Keeping them in separate files makes the boundary clear:
// version.go is "what the build pipeline says"; provenance.go is
// "what the toolchain says".
//
// The two are complementary:
//
//   - The linker variables are authoritative for the release
//     identity: which tag, which build date, which channel.
//
//   - Provenance is authoritative for the build inputs: which Go
//     toolchain, which modules, which VCS revision.
//
// A user who is debugging a suspicious binary reads both. A user
// who is verifying a release reads the linker variables. A user who
// is reporting a compiler bug reads the provenance.
//
// # Relationship to `go version -m`
//
// `go version -m ./forge` prints the same information this file
// reads. The reader exists so the CLI can present it in a stable,
// documented format without depending on the `go` toolchain being
// installed on the user's machine.
//
// # Purity
//
// ReadProvenance is pure: it reads the runtime's build info, which
// the linker populated at startup, and returns a value. It does not
// touch the filesystem, the network, the environment, or any other
// mutable global state.
//
// # Concurrency
//
// ReadProvenance is safe for concurrent use. The runtime's build
// info is read-only after package initialization.
//
// # Testability
//
// ReadBuildInfo is a package-level function in runtime/debug that
// cannot be stubbed without reflection or link-time hooks. The
// reader is written to tolerate a nil result: a binary built without
// module support returns a zero Provenance and no error. Tests use
// the real ReadBuildInfo from the test binary, which always has it,
// or call provenanceFromBuildInfo directly with a synthetic
// debug.BuildInfo value (see export_test.go).
package version

import (
	"runtime/debug"
	"time"
)

// Provenance is the subset of runtime/debug.BuildInfo that Forge
// reports. It is a value type with no pointers to the runtime's
// internal state, so it is safe to retain and to serialize.
//
// The zero value is valid: it is what an uninstrumented build
// reports, and what ReadProvenance returns when the binary has no
// build info.
type Provenance struct {
	// GoVersion is the Go toolchain version, e.g. "go1.23.4".
	//
	// It is the version of the toolchain that compiled the binary,
	// not the version installed on the user's machine. It is what
	// `go version -m ./forge` prints on the first line.
	GoVersion string

	// Module is the main module's path, e.g.
	// "github.com/thapelomagqazana/forge".
	//
	// It is the value of the `module` directive in the go.mod of the
	// binary's own module.
	Module string

	// ModuleVersion is the main module's version. For a binary built
	// from a working tree (not via `go install pkg@version`), this is
	// typically "(devel)".
	//
	// The empty string and "(devel)" are both valid; a consumer that
	// renders the value should treat both as "unknown".
	ModuleVersion string

	// VCSRevision is the full VCS revision SHA, e.g.
	// "0123456789abcdef0123456789abcdef01234567".
	//
	// It is empty if the binary was built without -buildvcs=true or
	// outside a VCS working tree.
	//
	// Note: this is the FULL SHA. The linker-injected Commit in
	// version.go is the short SHA. They identify the same commit but
	// are not interchangeable strings. A consumer that compares them
	// must normalize.
	VCSRevision string

	// VCSTime is the commit time reported by the VCS.
	//
	// It is the zero Time if the binary was built without
	// -buildvcs=true or if the setting was absent. A consumer that
	// renders the value should treat the zero Time as "unknown".
	//
	// The value is always in UTC.
	VCSTime time.Time

	// VCSModified is true if the VCS working tree had uncommitted
	// changes when the binary was built.
	//
	// It is the provenance reader's analogue of the linker's Dirty
	// variable. The two are computed differently:
	//
	//   - version.Dirty is set by the build pipeline from
	//     `git status --porcelain`.
	//   - version.Provenance.VCSModified is set by the Go toolchain
	//     from its own inspection of the working tree.
	//
	// They will agree for a Forge release build. They may disagree
	// for a binary built by a third party whose pipeline computes
	// dirty differently. The disagreement is informative, not a
	// defect.
	VCSModified bool

	// Dependencies lists the modules the binary links against,
	// including their versions. Indirect dependencies are included;
	// see Dependency.Indirect.
	//
	// The order matches runtime/debug.BuildInfo.Deps, which is
	// deterministic for a given binary. A consumer that wants a
	// stable ordering sorts by Path.
	Dependencies []Dependency

	// Settings lists the Go build settings, e.g. -trimpath=true,
	// CGO_ENABLED=0, vcs.revision=..., vcs.time=....
	//
	// The order matches runtime/debug.BuildInfo.Settings, which is
	// sorted by key. A consumer that wants a stable ordering does not
	// need to sort.
	Settings []Setting
}

// Dependency is one module the binary links against.
type Dependency struct {
	// Path is the module path, e.g. "example.com/module".
	Path string

	// Version is the module version, e.g. "v1.8.1".
	Version string

	// Sum is the module's go.sum hash, or empty if not present.
	//
	// The value is the "h1:" line from go.sum, not the full file
	// entry. It is the hash the toolchain verified at build time.
	Sum string

	// Replace is the module this one is replaced by, or empty.
	//
	// Format: "<path>@<version>" for a versioned replace,
	// "<path>" for a filesystem or local replace. The distinction
	// matters: a versioned replace is a fork published to a
	// module proxy; a filesystem replace is a local edit that will
	// not be reproducible on another host.
	Replace string

	// Indirect is true if the module is not a direct dependency of
	// the main module. A direct dependency is one the main module
	// imports; an indirect dependency is one pulled in transitively.
	//
	// The field is informational. A consumer that lists "what Forge
	// depends on" filters Indirect == false.
	Indirect bool
}

// Setting is one Go build setting.
type Setting struct {
	// Key is the setting name, e.g. "-trimpath", "CGO_ENABLED",
	// "vcs.revision".
	Key string

	// Value is the setting value, e.g. "true", "0", or a SHA.
	//
	// The value is the toolchain's string representation. A boolean
	// setting renders as "true" or "false"; an integer setting
	// renders as a decimal string.
	Value string
}

// ReadProvenance returns the binary's provenance information.
//
// It reads runtime/debug.ReadBuildInfo, which the Go runtime
// populates at startup from the module and VCS information embedded
// in the binary. It returns a zero Provenance (not an error) if the
// binary was built without module support, which is the case for a
// binary built with GO111MODULE=off.
//
// The function is pure: it does not modify the runtime's state and
// it always returns the same value for the same binary. It is safe
// for concurrent use.
//
// The result is a value, not a pointer. A caller that retains it
// retains a snapshot; a subsequent change to the runtime's build
// info (which does not happen in practice) would not affect the
// snapshot.
func ReadProvenance() Provenance {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Provenance{}
	}
	return provenanceFromBuildInfo(info)
}

// provenanceFromBuildInfo converts a debug.BuildInfo into a
// Provenance.
//
// It is factored out of ReadProvenance so a test can exercise the
// conversion with a synthetic debug.BuildInfo. The conversion is
// where the two APIs could diverge; a test against the real
// ReadBuildInfo only exercises whatever the test binary happened to
// embed.
//
// The function copies every string and creates a new slice for
// Dependencies and Settings. It does not retain a pointer into the
// runtime's internal state, so the returned Provenance is safe to
// retain.
func provenanceFromBuildInfo(info *debug.BuildInfo) Provenance {
	if info == nil {
		return Provenance{}
	}
	p := Provenance{
		GoVersion:     info.GoVersion,
		Module:        info.Main.Path,
		ModuleVersion: info.Main.Version,
	}
	if len(info.Deps) > 0 {
		p.Dependencies = make([]Dependency, 0, len(info.Deps))
		for _, d := range info.Deps {
			p.Dependencies = append(p.Dependencies, dependencyFromModule(d))
		}
	}
	if len(info.Settings) > 0 {
		p.Settings = make([]Setting, 0, len(info.Settings))
		for _, s := range info.Settings {
			p.Settings = append(p.Settings, Setting{Key: s.Key, Value: s.Value})
			switch s.Key {
			case "vcs.revision":
				p.VCSRevision = s.Value
			case "vcs.time":
				// The toolchain writes vcs.time as RFC 3339. A
				// failure to parse means the toolchain produced an
				// unexpected format; we treat it as "no time" rather
				// than fail the whole reader.
				if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
					p.VCSTime = t.UTC()
				}
			case "vcs.modified":
				// The toolchain writes "true" or "false". A different
				// value is treated as false, matching the lenient
				// parse rule used for the linker's Dirty variable.
				p.VCSModified = s.Value == "true"
			}
		}
	}
	return p
}

// dependencyFromModule converts a *debug.Module into a Dependency.
//
// It tolerates a nil argument: a caller that iterates a slice with a
// nil element receives a zero Dependency rather than a panic. The
// runtime does not produce nil elements, but the function is the
// boundary between the runtime's representation and Forge's, and a
// nil check at the boundary is cheap insurance.
func dependencyFromModule(m *debug.Module) Dependency {
	if m == nil {
		return Dependency{}
	}
	d := Dependency{
		Path:    m.Path,
		Version: m.Version,
		Sum:     m.Sum,
	}
	if m.Replace != nil {
		if m.Replace.Version != "" {
			d.Replace = m.Replace.Path + "@" + m.Replace.Version
		} else {
			d.Replace = m.Replace.Path
		}
	}
	return d
}

// Setting returns the value of the named build setting, or the empty
// string if the setting is absent.
//
// The lookup is linear. The settings list is small (typically fewer
// than twenty entries) and is read once per process. A map would add
// an allocation per call; a linear scan does not.
//
// The function is a method on Provenance so a consumer that has the
// value in hand does not need to import a helper.
func (p Provenance) Setting(key string) string {
	for _, s := range p.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

// Trimpath reports whether the binary was built with -trimpath.
//
// It is a convenience over Setting("-trimpath"). The value is "true"
// for every binary the release pipeline produces; a value of "false"
// indicates a binary built outside the driver, which is not
// reproducible.
func (p Provenance) Trimpath() bool {
	return p.Setting("-trimpath") == "true"
}

// CGOEnabled reports whether the binary was built with CGO enabled.
//
// It is a convenience over Setting("CGO_ENABLED"). The value is
// "0" for every binary the release pipeline produces. A value of
// "1" indicates a binary built outside the driver.
func (p Provenance) CGOEnabled() bool {
	return p.Setting("CGO_ENABLED") == "1"
}
