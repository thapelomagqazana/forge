package version

// Version is the semantic version of the running Forge build, for
// example "0.1.0" or "1.2.3-rc1".
//
// It is set at link time via:
//
//	-X github.com/thapelomagqazana/forge/internal/version.Version=<value>
//
// When the binary is built without ldflags (for example, during
// `go test` or a local development build), Version is the empty
// string. This is intentional: an empty version is an honest signal
// that the binary carries no version information. A non-empty default
// such as "dev" would hide the fact.
//
// The value is not validated by this package. Any string is accepted;
// the formatter and any future consumer decide how to render it.
var Version string

// Commit is the short Git commit hash of the running Forge build, for
// example "a1b2c3d" or "a1b2c3d4e5f6g7h8".
//
// It is set at link time via:
//
//	-X github.com/thapelomagqazana/forge/internal/version.Commit=<value>
//
// When the binary is built outside a Git working tree, or without
// ldflags, Commit is the empty string.
//
// The value is not validated. The build system is responsible for
// producing a value that identifies the source revision.
var Commit string

// BuildDate is the RFC 3339 timestamp of when the binary was built,
// for example "2026-10-09T12:00:00Z".
//
// It is set at link time via:
//
//	-X github.com/thapelomagqazana/forge/internal/version.BuildDate=<value>
//
// The value is not validated by this package. The build system is
// responsible for producing a valid RFC 3339 timestamp. When the
// binary is built without ldflags, BuildDate is the empty string.
var BuildDate string

// Dirty reports whether the running Forge build was produced from a
// working tree with uncommitted changes.
//
// It is set at link time via:
//
//	-X github.com/thapelomagqazana/forge/internal/version.Dirty=<value>
//
// The value is the string "true" or "false". When the binary is
// built without ldflags, Dirty is the empty string.
//
// # Why a string and not a bool
//
// The -X linker flag can only set string variables. A bool field
// would require either a two-variable convention (for example,
// Clean and Dirty) or a parse step in the consumer. A single string
// is simpler to inject and to read; callers that need a boolean
// compare against "true" (or, more leniently, against the empty
// string, treating empty as "unknown" rather than "clean").
//
// The value is not validated by this package. The build system is
// responsible for producing "true" or "false". A build that passes
// "yes" or "1" produces a binary whose Dirty value is "yes" or "1";
// the package does not correct it.
var Dirty string

// Get returns the four build-time values as a single tuple.
//
// # Why an accessor
//
// Callers could read the four variables directly. The accessor
// exists so that:
//
//   - The variables can be unexported in a future refactor without
//     touching any caller. The -X flag requires exported variables,
//     but a build tag could select between an exported and an
//     unexported declaration; the accessor would then be the only
//     stable API.
//
//   - A future implementation can populate the values from a
//     different source — for example, runtime/debug.ReadBuildInfo,
//     which reads the module version from the binary's build info —
//     without changing any call site.
//
//   - The tuple return reads naturally at the call site:
//     `v, c, d, dirty := version.Get()`.
//
// # Purity
//
// Get is pure. It reads four package-level variables and returns
// them. It does not touch the filesystem, the network, the
// environment, or any other global state. Two consecutive calls with
// no intervening mutation return equal values.
//
// # Concurrency
//
// The four variables are read-only after package initialization.
// Concurrent calls to Get are safe. A caller that wants to mutate
// the variables for testing (see buildinfo_test.go in
// internal/app/version) must serialize access with its own
// synchronization; the standard pattern is to avoid t.Parallel in
// such tests.
func Get() (version, commit, buildDate, dirty string) {
	return Version, Commit, BuildDate, Dirty
}
