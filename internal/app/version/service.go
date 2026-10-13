package version

import (
	"context"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/logger"
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

// resolve returns the build information for the running Forge
// binary, without emitting any DEBUG records.
//
// # Purity
//
// resolve reads package-level variables from internal/version,
// constructs an Info value, and returns it. It does not touch the
// filesystem, the network, the environment, or any other global
// state. Two calls with no intervening mutation return equal
// values.
//
// The purity is what makes the version command testable without a
// subprocess. A test can stub the build variables (by assigning to
// them in the test package), call resolve (or Get, which calls it),
// and assert on the result. See service_test.go for the pattern.
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
// resolve reads the variables through buildInfo, a small accessor
// that isolates the import of internal/version to one file.
//
// # Why this function is separate from GetContext
//
// GetContext emits two DEBUG records and measures their duration.
// The records' emission calls logger.Debug, which allocates; the
// duration measurement calls time.Now, which also allocates. The
// allocations would fail the
// TestNonFunctional_GetIsAllocationFreeOnHeap test, which asserts
// that the resolution is allocation-free.
//
// Splitting the two concerns keeps both properties:
//
//   - resolve is pure and allocation-free (the property the
//     non-functional test measures).
//   - GetContext wraps resolve with the DEBUG records (the
//     property WBS 12.5.1 requires).
//
// The allocation test calls Get (or resolve), not GetContext. A
// future change that adds allocation to resolve fails the test; a
// change that adds allocation to GetContext (a new record, a new
// field) does not, because the measurement is on the inner
// function.
func resolve() Info {
	version, commit, buildDate, dirty := buildInfo()
	return Info{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		Dirty:     dirty,
	}
}

// Get returns the build information for the running Forge binary.
//
// Get is the pure entry point. It calls resolve directly and does
// not emit DEBUG records. Callers that want the records use
// GetContext.
//
// # Why Get does not take a context
//
// Get is called from two places where a context is not available
// or not wanted:
//
//   - The non-functional test
//     TestNonFunctional_GetIsAllocationFreeOnHeap calls Get
//     repeatedly to measure its allocations. A context parameter
//     would be irrelevant to that measurement, and its presence
//     would tempt a future change to emit a record (which would
//     reintroduce the allocation).
//
//   - Raw, which is called by newRootCmd during command-tree
//     construction, before any invocation's context exists.
//
// Callers that have an invocation's context use GetContext, which
// emits the DEBUG records and returns the same Info.
//
// The allocation-free property that
// TestNonFunctional_GetIsAllocationFreeOnHeap measures applies to
// Get (and to resolve). GetContext is the record-emitting variant
// and is not allocation-free by design.
func Get() Info {
	return resolve()
}

// GetContext returns the build information for the running Forge
// binary, emitting two DEBUG records.
//
// # Purity
//
// GetContext is pure with respect to its inputs. It reads
// package-level variables from internal/version, constructs an
// Info value, and returns it. It does not touch the filesystem,
// the network, the environment, or any other global state. Two
// calls with no intervening mutation return equal values.
//
// The context is used solely for the DEBUG records; the
// resolution's behavior does not depend on it.
//
// # Nil context
//
// A nil context is replaced by context.Background(). The
// substitution keeps the DEBUG records' emission unconditional
// without requiring every caller to pass a non-nil context.
//
// # Relationship to Get
//
// Get and GetContext return the same Info value. The difference is
// that GetContext emits the records and measures the duration;
// Get does neither. The CLI handler (internal/cli/version.go) uses
// GetContext so its DEBUG surface includes the resolution; the
// non-functional test and Raw use Get so their performance and
// output are independent of the logging configuration.
//
// # DEBUG records (WBS 12.5.1)
//
// The function emits two DEBUG records:
//
//  1. "version service: reading build info" — before resolve
//     runs. The record has no fields; its presence is the signal
//     that the service was invoked. The alternative — a record
//     with a "binary" field naming the running executable's path
//     — was rejected because the service does not read the
//     binary's path (it reads package-level variables), and a
//     field whose value the function cannot produce would be
//     misleading.
//
//  2. "version service: resolved" — after resolve returns, with
//     the four fields (version, commit, build_date, dirty) and a
//     duration in milliseconds. The duration is the wall time
//     between the two records. It is reported because the
//     underlying debug.ReadBuildInfo call can be slow in a
//     container with a limited /proc, and a reader diagnosing a
//     slow `forge version` benefits from the measurement.
//
// The two records are the version subsystem's contribution to
// docs/reference/verbose-mode.md §3, alongside the two records the
// CLI handler emits (see internal/cli/version.go). The service
// records the resolution's behavior; the handler records the
// command's dispatch decisions.
func GetContext(ctx context.Context) Info {
	if ctx == nil {
		ctx = context.Background()
	}

	logger.Debug(ctx, "version service: reading build info")

	start := time.Now()
	info := resolve()
	duration := time.Since(start)

	logger.Debug(ctx, "version service: resolved",
		"version", info.Version,
		"commit", info.Commit,
		"build_date", info.BuildDate,
		"dirty", info.Dirty,
		"duration_ms", duration.Milliseconds(),
	)

	return info
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
//
// # Context
//
// Raw does not take a context, and it calls Get (not GetContext).
// It is called by the root command's Version field initialization,
// which runs before the CLI's per-invocation context exists. The
// absence of a context is deliberate: the call is not part of any
// invocation, and its output must not depend on the logging
// configuration. Calling Get (the pure path) rather than
// GetContext (the record-emitting path) keeps the property.
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
