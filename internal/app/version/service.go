package version

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/logger"
)

// Info is the result of the version application service.
//
// It is a plain struct with no methods, no embedded types, and no
// dependencies. The CLI handler formats it; nothing else does.
//
// # Why a struct and not five strings
//
// A handler could receive the fields as separate return values. A
// struct is preferable because:
//
//   - It has a name. "Info" is more legible at the call site than
//     five positional string returns.
//
//   - It is extensible. Adding a field (for example, "BuildType" or
//     a "Provenance" is a one-line change that does not alter the
//     signature of Get.
//
//   - It is testable. A test can construct an Info literal and pass
//     it to the formatters without going through Get.
//
// # Zero value
//
// The zero value of Info is valid. It represents "no build
// information is available", which renders as empty strings. The
// formatters are responsible for deciding whether to omit empty
// fields or render them as "-". In WBS 17.4.1, the verbose
// formatter renders empty values as "-"; the JSON formatter emits
// them as empty strings; the human one-line formatter omits them.
type Info struct {
	// Version is the semantic version of the running Forge build,
	// for example "v1.4.2" or "dev". It is empty when the binary was
	// built without version information (for example, during local
	// development without ldflags).
	Version string

	// Commit is the short Git commit hash of the running Forge
	// build, for example "a1b2c3d" or "none". It is empty when the
	// binary was built outside a Git working tree.
	Commit string

	// BuildDate is the RFC 3339 timestamp of when the binary was
	// built, for example "2026-10-09T12:00:00Z". It is empty when
	// the binary was built without build-date information.
	//
	// The format is not enforced by this struct. The formatters
	// render the string as-is; the build system is responsible for
	// producing a valid RFC 3339 value. The schema and the release
	// pipeline's preconditions enforce the format at the boundary.
	BuildDate string

	// Dirty reports whether the running Forge build was produced
	// from a working tree with uncommitted changes. It is the
	// string "true" or "false". It is empty when the binary was
	// built without dirty-state information.
	//
	// # Why a string and not a bool
	//
	// The linker injects a string. Keeping the string here means
	// the service does not need to parse it; the formatter renders
	// it as-is. The JSON formatter and the Metadata type convert
	// to bool at their boundary; a caller that wants a bool uses
	// MetadataOf.
	Dirty string

	// BuildType is the environment the running Forge build was
	// produced in. It is one of "release", "ci", or "local", or the
	// empty string when the binary was built without build-type
	// information.
	//
	// The empty string is meaningful: a binary that carries no
	// build-type information was almost certainly built outside CI.
	// The Metadata type normalizes it to "local"; this field
	// preserves the raw value so a caller that wants to detect an
	// uninstrumented build can.
	BuildType string

	// Provenance is the Go toolchain's own record of the build: the
	// Go version, the module graph, and the VCS revision. It is
	// populated by readProvenance (which delegates to
	// internal/version's ReadProvenance, itself a wrapper over
	// runtime/debug.ReadBuildInfo). Its zero value is valid.
	//
	// The field is a value, not a pointer, so Info is copyable. A
	// caller that wants a snapshot retains the whole Info value.
	//
	// The type is the service's Provenance alias (declared in
	// buildinfo.go), which is the same type as the library's
	// Provenance. A caller of the service does not need to import
	// the library to name it.
	Provenance Provenance
}

// resolve returns the build information for the running Forge
// binary, without emitting any DEBUG records.
//
// # Purity
//
// resolve reads the five linker variables from internal/version,
// reads the provenance from runtime/debug.ReadBuildInfo (via
// readProvenance), constructs an Info value, and returns it. It
// does not touch the filesystem, the network, the environment, or
// any other global state. Two calls with no intervening mutation
// return equal values.
//
// The purity is what makes the version command testable without a
// subprocess. A test can stub the build variables (by assigning to
// them in the test package), call resolve (or Get, which calls it),
// and assert on the result. See service_test.go for the pattern.
//
// # Where the build variables live
//
// The five build variables (Version, Commit, BuildDate, Dirty,
// BuildType) live in internal/version and are injected at link time
// via ldflags:
//
//	go build -ldflags "\
//	  -X github.com/thapelomagqazana/forge/internal/version.Version=0.1.0 \
//	  -X github.com/thapelomagqazana/forge/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/thapelomagqazana/forge/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
//	  -X github.com/thapelomagqazana/forge/internal/version.Dirty=false \
//	  -X github.com/thapelomagqazana/forge/internal/version.BuildType=local"
//
// The relocation of the variables from internal/cli to
// internal/version happened as part of WBS 4.3.1, when the version
// handler was introduced. The relocation broke the import cycle
// between internal/cli and internal/app/version that would
// otherwise have existed. See docs/architecture.md § 11.11 for the
// full account.
//
// # Why this function is separate from GetContext
//
// GetContext emits two DEBUG records and measures their duration.
// The records' emission calls logger.Debug, which allocates; the
// duration measurement calls time.Now, which also allocates. The
// allocations would fail the allocation test, which asserts that
// the resolution's allocation is bounded by a small constant.
//
// Splitting the two concerns keeps both properties:
//
//   - resolve is pure and has bounded allocation (the property the
//     non-functional test measures).
//   - GetContext wraps resolve with the DEBUG records (the
//     property WBS 12.5.1 requires).
//
// # Provenance and allocation
//
// readProvenance reads runtime/debug.ReadBuildInfo, which returns
// a pointer to a value the runtime computed at startup. The reader
// copies the value into a new Provenance, which allocates. The
// allocation is bounded: one Provenance value and two slices sized
// to the module graph. It is not zero, but it is small and does
// not grow with the number of calls.
//
// The non-functional test measures Get (which calls resolve) and
// asserts the count is at most a small constant. See the test file
// for the exact bound.
func resolve() Info {
	v, c, d, dirty, buildType := buildInfo()
	return Info{
		Version:    v,
		Commit:     c,
		BuildDate:  d,
		Dirty:      dirty,
		BuildType:  buildType,
		Provenance: readProvenance(),
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
//   - The non-functional test calls Get repeatedly to measure its
//     allocations. A context parameter would be irrelevant to that
//     measurement, and its presence would tempt a future change to
//     emit a record (which would reintroduce the allocation).
//
//   - Raw, which is called by newRootCmd during command-tree
//     construction, before any invocation's context exists.
//
// Callers that have an invocation's context use GetContext, which
// emits the DEBUG records and returns the same Info.
//
// # Allocation and provenance
//
// The bounded-allocation property the non-functional test asserts
// covers both the linker-variable read (which does not allocate)
// and the provenance read (which allocates a small, bounded
// amount). A caller that needs a strict zero-alloc path does not
// call Get.
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
// that GetContext emits the records and measures the duration; Get
// does neither. The CLI handler (internal/cli/version.go) uses
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
//     that the service was invoked.
//
//  2. "version service: resolved" — after resolve returns, with
//     the five linker fields (version, commit, build_date, dirty,
//     build_type), the Go version from provenance, and a duration
//     in milliseconds. The duration is the wall time between the
//     two records.
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
		"build_type", info.BuildType,
		"go_version", info.Provenance.GoVersion,
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
// # Relationship to Format
//
// Format writes the same string to an io.Writer. Raw returns it as
// a string. The two are two views of the same format.
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
// which runs before the CLI's per-invocation context exists.
func Raw() string {
	var b strings.Builder
	if err := Format(&b, Get()); err != nil {
		// Format writes to an io.Writer; a strings.Builder cannot
		// fail. The error branch is unreachable but is kept so that
		// the function's behaviour is identical to Format's.
		return "forge\n"
	}
	return b.String()
}

// Metadata returns the Info's five linker fields as a Metadata
// value.
//
// # Why a conversion
//
// The service's Info type carries the raw linker strings and the
// provenance. The Metadata type (re-exported from the library in
// buildinfo.go) carries the JSON shape: a bool for Dirty, a
// normalized BuildType. The conversion is performed once, at the
// boundary, so the writers below do not repeat it.
//
// The conversion normalizes an unrecognized BuildType to "local",
// matching the library's MetadataOf behavior. The normalization is
// performed inside the library; this method delegates to it via
// the metadataOf accessor.
func (i Info) Metadata() Metadata {
	return metadataOf(i.Version, i.Commit, i.BuildDate, i.Dirty, i.BuildType)
}

// WriteJSON writes the five-field JSON form of i to w.
//
// It is the shape emitted by `forge version --format json`. The
// output validates against schemas/version.schema.json.
func WriteJSON(w io.Writer, i Info) error {
	return writeMetadataJSON(w, i.Metadata())
}

// WriteFullJSON writes the extended JSON form of i to w, including
// the provenance.
//
// It is the shape emitted by `forge version --format json --verbose`.
// The output validates against schemas/version-full.schema.json.
func WriteFullJSON(w io.Writer, i Info) error {
	return writeMetadataFullJSON(w, i.Metadata(), i.Provenance)
}

// WriteVerbose writes the verbose human-readable form of i to w.
//
// It is the shape emitted by `forge version --verbose`. The format
// is documented in docs/reference/version-output.md §"Verbose
// format".
func WriteVerbose(w io.Writer, i Info) error {
	return writeMetadataVerbose(w, i.Metadata(), i.Provenance)
}
