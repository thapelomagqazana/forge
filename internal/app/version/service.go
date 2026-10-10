package version

import (
	"strings"
)

// Info is the result of the version application service.
//
// It is a plain struct with no methods, no embedded types, and no
// dependencies. The CLI handler formats it; nothing else does.
//
// # Why a struct and not four strings
//
// A handler could receive the four fields as separate return values.
// A struct is preferable because:
//
//   - It has a name. "Info" is more legible at the call site than
//     four positional string returns.
//
//   - It is extensible. Adding a fifth field (for example, "GoVersion"
//     or "OS") is a one-line change that does not alter the signature
//     of Get.
//
//   - It is testable. A test can construct an Info literal and pass
//     it to Format without going through Get.
//
// # Zero value
//
// The zero value of Info is valid. It represents "no build
// information is available", which renders as empty strings. The
// formatter is responsible for deciding whether to omit empty fields
// or render them as "(unknown)". In Phase 2, the formatter renders
// them as empty strings.
type Info struct {
	// Version is the semantic version of the running Forge build,
	// for example "0.1.0". It is empty when the binary was built
	// without version information (for example, during local
	// development without ldflags).
	Version string

	// Commit is the short Git commit hash of the running Forge
	// build, for example "a1b2c3d". It is empty when the binary was
	// built outside a Git working tree.
	Commit string

	// BuildDate is the RFC 3339 timestamp of when the binary was
	// built, for example "2026-10-09T12:00:00Z". It is empty when
	// the binary was built without build-date information.
	//
	// The format is not enforced by this struct. The formatter
	// renders the string as-is; the build system is responsible for
	// producing a valid RFC 3339 value.
	BuildDate string

	// Dirty reports whether the running Forge build was produced from a
	// working tree with uncommitted changes. It is the string "true"
	// or "false". It is empty when the binary was built without
	// dirty-state information.
	//
	// The value is not validated by this struct. The formatter
	// renders the string as-is; the build system is responsible for
	// producing a valid value.
	Dirty string
}

// Get returns the build information for the running Forge binary.
//
// # Purity
//
// Get is pure. It reads package-level variables from
// internal/version, constructs an Info value, and returns it. It
// does not touch the filesystem, the network, the environment, or
// any other global state. Two calls with no intervening mutation
// return equal values.
//
// The purity is what makes the version command testable without a
// subprocess. A test can stub the build variables (by assigning to
// them in the test package), call Get, and assert on the result.
// See service_test.go for the pattern.
//
// # Where the build variables live
//
// The build variables (Version, Commit, BuildDate, Dirty) live in
// internal/version and are injected at link time via ldflags:
//
//	go build -ldflags "\
//	  -X github.com/thapelomagqazana/forge/internal/version.Version=0.1.0 \
//	  -X github.com/thapelomagqazana/forge/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/thapelomagqazana/forge/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
//	  -X github.com/thapelomagqazana/forge/internal/version.Dirty=false"
//
// The relocation of the variables from internal/cli to
// internal/version happened as part of WBS 4.3.1, when the version
// handler was introduced. The relocation broke the import cycle
// between internal/cli and internal/app/version that would
// otherwise have existed. See docs/architecture.md § 11.12 for the
// full account.
//
// Get reads the variables through buildInfo, a small accessor that
// isolates the import of internal/version to one file.
func Get() Info {
	version, commit, buildDate, dirty := buildInfo()
	return Info{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		Dirty:     dirty,
	}
}

// Raw returns the version string that the CLI prints for the version
// command, in the format documented in docs/cli-ux-spec.md § 4.8.
//
// # What Raw produces
//
// Raw renders the same text that `forge version` writes to stdout:
//
//	forge <version>
//	  commit:     <commit>
//	  built:      <build-date>
//	  dirty:      <dirty>
//	  go version: <go-version>
//	  platform:   <os>/<arch>
//
// The string ends with a trailing newline.
//
// # Why Raw exists
//
// Cobra has a built-in --version flag that prints a one-line string
// when root.Version is non-empty. Forge's own `forge version`
// subcommand prints a longer, structured report. WBS 5.1.2 requires
// that `forge --version` and `forge version` produce identical
// output, so the CLI needs a single canonical function that returns
// the string. Raw is that function.
//
// The alternative — that newRootCmd formats the string itself —
// would duplicate the formatter and create a maintenance hazard:
// a change to the format would have to be applied in two places.
// Raw centralises the format.
//
// # Relationship to Format
//
// Format writes the same string to an io.Writer. Raw returns it as
// a string. The two are two views of the same format. The CLI uses
// Format when writing to a stream (as in the version handler), and
// Raw when it needs the string itself (as in root.Version).
//
// # Purity
//
// Raw is pure. It reads the same values that Get reads and formats
// them. Two calls with no intervening mutation return the same
// string.
func Raw() string {
	var b strings.Builder
	if err := Format(&b, Get()); err != nil {
		// Format writes to an io.Writer; a strings.Builder cannot
		// fail. The error branch is unreachable but is kept so that
		// the function's behaviour is identical to Format's, and so
		// that a future change to Format's signature does not
		// silently break Raw.
		//
		// The fallback returns a minimal, well-formed string so that
		// the CLI's --version flag always produces output, even in
		// the impossible case.
		return "forge\n"
	}
	return b.String()
}
