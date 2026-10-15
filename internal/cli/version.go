// Package cli — the `forge version` command.
//
// This file is the reference implementation of the handler /
// service pattern defined by WBS 4.3.1 and documented in
// docs/architecture.md "Handler / Service Boundary".
//
// The handler:
//
//   - Parses flags.
//   - Collects arguments (there are none; the command is arg-free).
//   - Calls the application service.
//   - Writes the result to deps.Streams.Out, never to a
//     process-global.
//   - Returns an error on failure; never calls os.Exit.
//
// The handler does not:
//
//   - Read or write files.
//   - Access the network.
//   - Contain business rules or formatting logic.
//   - Write to os.Stdout or os.Stderr directly.
//   - Print formatted output with fmt.Fprintln or its variants.
//
// The handler body is under twenty lines of executable code. The
// `--format` flag is a local flag; the `--verbose` flag is the
// root's persistent flag, read through verboseRequested. The
// dispatch happens in the switch below.
//
// # The --format flag
//
// The flag has two supported values, "text" and "json". The
// default is "text", which produces the human-readable output
// frozen by WBS 6.4.1. The "json" value produces the machine-
// readable output frozen by WBS 6.4.2 and extended by WBS 17.4.1.
//
// An unknown value produces a *forgeerr.Error with category
// CategoryCLI and code CodeCLIUnknownFormat. The handler
// constructs the error; runWithOptions renders it and maps its
// category to the exit code (ExitUsage, 64).
//
// # The --verbose flag
//
// --verbose selects the extended output. For --format text it
// renders the three-section human format from WBS 17.4.1
// (metadata, provenance, dependencies and settings). For --format
// json it adds a "provenance" field to the JSON object.
//
// --verbose is independent of --format; any combination is valid.
// A user who wants the extended human output runs `forge version
// --verbose`; a user who wants the extended JSON runs `forge
// version --format json --verbose`.
//
// # Why --verbose is not declared locally
//
// The version command does NOT declare its own --verbose flag. It
// reads the root's persistent --verbose flag via verboseRequested.
//
// # The flag-collision problem
//
// Cobra's flag set for a subcommand is the merge of the
// subcommand's own flags and the ancestors' persistent flags. When
// two flags share a name, cobra keeps one and shadows the other;
// which one wins depends on the cobra version and on the order of
// registration. The two flags cannot be made independent by
// declaring both: the merged set has one --verbose, and its value
// is the same regardless of where the user wrote it.
//
// # The single persistent flag
//
// The design that works is a single persistent --verbose flag on
// the root. Its value is set when the user writes --verbose in
// either position:
//
//	forge --verbose version     --verbose is parsed by the root
//	forge version --verbose     --verbose is parsed by the merged
//	                            set inherited from the root
//
// Both invocations set the same flag to the same value. The
// version handler reads it via verboseRequested(cmd), which calls
// cmd.Flags().GetBool(FlagVerbose). The merged set resolves the
// name to the persistent flag.
//
// # Consequences
//
//   - `forge --verbose version` and `forge version --verbose` are
//     equivalent: both produce the verbose output.
//
//   - There is no way to select the verbose output for the
//     version command without also enabling debug logging. The
//     two concerns share a flag because they share a name.
//
//   - The test TestFlagInteraction_VerboseVersionPrintsVersion
//     asserts both positions and their equivalence. A future
//     design that wants to separate the concerns must rename one
//     of the flags (for example, --provenance for the version
//     command's output) and file an ADR.
//
// # Why the error is a *forgeerr.Error
//
// The WBS 10.5.1 invariant is that every error reaching the CLI
// boundary is a *forgeerr.Error. Before the migration, the
// handler returned a plain error wrapped around the service's
// sentinel; the boundary classified the plain error as
// FORGE_INTERNAL_UNEXPECTED, and the exit code was
// ExitInternal (70) instead of ExitUsage (64).
//
// The handler now constructs a *forgeerr.Error directly. The
// category is CategoryCLI, because the failure is a usage error
// (the user passed a value the command does not accept). The
// code is CodeCLIUnknownFormat, which is a member of the
// registry (WBS 10.3.1). The offending value is in the error's
// context under the key "format", and the remediation names the
// accepted values.
//
// # What about the service's sentinel
//
// The service still exports appversion.ErrUnknownFormat. The
// handler does not use it in the new implementation: the
// *forgeerr.Error carries the same information in a structured
// form (the code), and the CLI's consumer can match on the code
// via errors.As + Code() (the pattern documented in ADR-015).
// The sentinel remains exported for callers outside the CLI that
// need it; a future WBS item may deprecate it.
//
// # Error path
//
// Two error paths exist:
//
//  1. Unknown format — the handler constructs a *forgeerr.Error
//     with code CodeCLIUnknownFormat. Exit code ExitUsage (64).
//  2. Write error — the service returns the writer's error,
//     wrapped by the handler as FORGE_INTERNAL_UNEXPECTED. Exit
//     code is the internal-error code (ExitInternal, 70).
//
// Both are returned by the handler; neither is written to stderr
// by the handler. The error-to-message and error-to-exit-code
// mapping is done one layer up, in runWithOptions.
//
// # The Example field (WBS 7.1.3)
//
// The per-command help contract (WBS 7.1.3) requires an Examples
// block for any command that takes arguments, has command-specific
// flags, or has subcommands. The version command has one
// command-specific flag (`--format`) and reads the root's
// `--verbose`; it declares an Example.
//
// The four examples below are the command's four primary
// invocations: text, JSON, verbose text, and verbose JSON. The
// block is deliberately minimal; a user who needs more detail
// reads the Long description or the CLI UX spec.
//
// The golden file internal/cli/testdata/help/version.golden.txt
// records the rendered output, including this Examples block. A
// change to the field's content changes the rendered output and
// fails TestHelpGolden_Subcommands/version until the golden file
// is regenerated.
//
// # The output stream (WBS 12.4.1)
//
// The handler writes product output to deps.Streams.Out. Under
// the stream contract (ADR-022), product output goes to stdout;
// diagnostics go to stderr. The version command produces no
// diagnostics of its own; a failure is returned as an error and
// the boundary renders it on stderr.
//
// # DEBUG records (WBS 12.5.1)
//
// The command emits two DEBUG records, both to the process
// logger (which writes to stderr):
//
//  1. "version: resolved build info" — after appversion.Get()
//     returns, with the five linker fields (version, commit,
//     build_date, dirty, build_type) and the Go version from
//     the provenance.
//  2. "version: selected format" — after the format switch
//     resolves, with the requested format, the verbose flag, and
//     whether the format was accepted.
//
// The two records are the version subsystem's contribution to
// docs/reference/verbose-mode.md §3. They are emitted through the
// package-level logger, which the process boundary configures
// from --verbose / FORGE_LOG_LEVEL before the command tree runs
// (see internal/cli/execute.go).
//
// # Why --verbose does not imply DEBUG records (in principle)
//
// The --verbose flag selects the extended *output*, not the
// extended logging. In a design with two independent flags, a
// user who runs `forge version --verbose` would see more output
// on stdout without extra logging; a user who sets --log-level
// debug would see DEBUG records on stderr without verbose output.
//
// In the current single-flag design, the flag controls both: it
// sets the log level to DEBUG and selects the verbose output.
// The two effects happen together because they share a flag. A
// future design that separates them must rename one flag and
// file an ADR.
//
// The records are subject to the same rules as every other DEBUG
// record: they never carry secrets, they never write to stdout,
// and they are subject to the redaction policy from ADR-016. The
// version command's fields (version, commit, build_date, dirty,
// build_type, format, verbose, go_version) are not secrets; a
// future field that is a secret must be added in the same change
// as a redaction test.
//
// # Type-level contract
//
// forgeerr.New and forgeerr.Wrap take their code parameter as
// forgeerr.Code. The two call sites below pass the named
// constants from internal/forgeerr/codes.go directly. A
// string(...) cast in a constructor call is the diagnostic that
// the migration documented in safefs/errors.go was rolled back.
package cli

import (
	"context"

	"github.com/spf13/cobra"

	appversion "github.com/thapelomagqazana/forge/internal/app/version"
	"github.com/thapelomagqazana/forge/internal/forgeerr"
	"github.com/thapelomagqazana/forge/internal/logger"
)

// newVersionCmd constructs the `forge version` command.
//
// The constructor's signature is fixed by the command registry
// (registry.go): every constructor takes a Dependencies value and
// returns a *cobra.Command. The handler body is under twenty lines
// of executable code; the flags, arguments, and formatting are
// delegated.
//
// # Flags
//
// One local flag is registered: --format. Its value set is
// {text, json}, frozen by WBS 6.4.1. The handler reads its value
// from the local `format` variable.
//
// The --verbose flag is NOT registered here. It is the root's
// persistent flag (registered by registerGlobalFlags in
// flags.go). The handler reads it via verboseRequested(cmd),
// which resolves the flag through the merged flag set the
// subcommand inherits. The docstring on the handler explains the
// single-flag design in detail.
func newVersionCmd(deps Dependencies) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print Forge version information",
		Long: `Print the version, commit, build date, Go version, and
platform of the running Forge binary.

The default output is a stable, line-oriented format suitable for
humans. The --format flag selects an alternative representation:

  --format text   the human-readable format (default)
  --format json   a single-line JSON object suitable for machines

The --verbose flag adds build provenance: the Go toolchain
version, the module graph, and the VCS revision. It may be
combined with either --format value.

The --verbose flag is the root's persistent flag; it may be
written before or after the subcommand name:

  forge --verbose version
  forge version --verbose

Both invocations produce the same output. See the CLI UX
specification for the frozen format of each.`,
		Example: `  forge version
  forge version --format json
  forge version --verbose
  forge version --format json --verbose`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// The context is threaded through for the DEBUG
			// records (WBS 12.5.1). The records carry the
			// invocation's correlation ID if one is bound
			// (see internal/logger's FromContext). When the
			// command runs outside an invocation — a unit
			// test that calls RunE directly, for example —
			// the records carry the zero ID, which a
			// consumer can filter out.
			ctx := cmd.Context()
			if ctx == nil {
				// Defensive: cobra guarantees a non-nil
				// context when RunE is invoked through
				// ExecuteContext, but a caller that invokes
				// RunE directly may pass nil. A Background
				// context is the correct fallback; the
				// logger tolerates it.
				ctx = context.Background()
			}

			// Read the --verbose flag from the merged flag set.
			//
			// # Why verboseRequested and not a local bool
			//
			// The version command does not declare its own
			// --verbose; the flag is the root's persistent
			// flag. verboseRequested(cmd) reads it through
			// the merged set the subcommand inherits. The
			// docstring on the handler explains the design.
			//
			// # Positions
			//
			// The value is the same regardless of where the
			// user wrote the flag:
			//
			//	forge --verbose version
			//	forge version --verbose
			//
			// Both set the flag. The two invocations are
			// equivalent.
			verbose := verboseRequested(cmd)

			// Resolve the build info once. The result is used
			// both by the DEBUG record and by the format
			// dispatch, so a test that asserts on the record's
			// fields and a test that asserts on the output
			// observe the same values.
			//
			// WBS 17.4.1: the Info carries the provenance read
			// from runtime/debug.ReadBuildInfo. The provenance
			// is included in the DEBUG record's go_version
			// field so a reader who is diagnosing a build can
			// see the toolchain version without a second
			// command.
			info := appversion.Get()

			// WBS 12.5.1: the build-info resolution record.
			//
			// # Why these fields
			//
			// version, commit, build_date, dirty, and build_type
			// are the five fields the version service produces
			// (see the Info struct's docstring). They are the
			// values a developer needs to know when diagnosing
			// "which build is this?". The five fields together
			// are the minimum for reproducing a release build.
			//
			// go_version is included because the Go toolchain
			// version is a build input: a behavior difference
			// between two builds of the same commit is often a
			// toolchain difference. Reading it from the DEBUG
			// record saves a `forge version --verbose` round
			// trip.
			//
			// # Why no "source" field
			//
			// The Info struct does not carry a source indicator.
			// An earlier draft of this record included a
			// "source" field (ldflags / debug.ReadBuildInfo /
			// fallback), but that information is not exposed by
			// the version service. Adding it would be a change
			// to the service's public API, which is outside
			// WBS 12.5.1's scope.
			//
			// # Why not the full Info struct
			//
			// Logging the whole struct would produce a record
			// whose shape depends on the struct's field set. A
			// future field addition would silently change the
			// record's shape. Naming the fields explicitly
			// keeps the record's shape stable and documents the
			// fields a consumer can rely on.
			//
			// The provenance's Dependencies and Settings slices
			// are deliberately not logged: they are large, and
			// a reader who needs them runs
			// `forge version --format json --verbose`.
			logger.Debug(ctx, "version: resolved build info",
				"version", info.Version,
				"commit", info.Commit,
				"build_date", info.BuildDate,
				"dirty", info.Dirty,
				"build_type", info.BuildType,
				"go_version", info.Provenance.GoVersion,
			)

			// WBS 12.5.1: the format-selection record.
			//
			// # What the record reports
			//
			// The record names the requested format, the
			// verbose flag, and whether the format was
			// accepted. A future contributor who reads the
			// DEBUG output for a failing invocation sees the
			// format value that caused the failure, without
			// re-running the command.
			//
			// # Why the record is emitted before the error is
			// # constructed
			//
			// The unknown-format branch returns an error. The
			// DEBUG record is the diagnostic surface for that
			// branch; emitting it before the error means a user
			// running with --log-level debug sees both the
			// DEBUG record and the rendered error, and can
			// correlate them by timestamp.
			//
			// # Type-level contract
			//
			// The forgeerr.New call passes CodeCLIUnknownFormat
			// directly. The constructor's parameter is
			// forgeerr.Code after the WBS 13.1.1 migration.
			switch format {
			case appversion.FormatText, appversion.FormatJSON:
				logger.Debug(ctx, "version: selected format",
					"format", format,
					"verbose", verbose,
					"accepted", true,
				)
			default:
				logger.Debug(ctx, "version: selected format",
					"format", format,
					"verbose", verbose,
					"accepted", false,
				)
				return forgeerr.New(
					forgeerr.CategoryCLI,
					forgeerr.CodeCLIUnknownFormat,
					"unknown output format",
				).
					WithContext("format", format).
					WithRemediation("Use '--format text' or '--format json'.")
			}

			// Dispatch to the writer. The switch has four
			// branches: text, text+verbose, json, json+verbose.
			// Each branch is a single call to the service; the
			// service owns the format. This is why the handler
			// stays thin: the branches name the four outputs,
			// they do not construct them.
			var err error
			switch {
			case format == appversion.FormatText && !verbose:
				err = appversion.FormatAs(deps.Streams.Out, info, format)
			case format == appversion.FormatText && verbose:
				err = appversion.WriteVerbose(deps.Streams.Out, info)
			case format == appversion.FormatJSON && !verbose:
				err = appversion.FormatAs(deps.Streams.Out, info, format)
			case format == appversion.FormatJSON && verbose:
				err = appversion.WriteFullJSON(deps.Streams.Out, info)
			}
			if err != nil {
				// The service returned a writer error. Wrap
				// it as an INTERNAL error, so the invariant
				// "every error reaching the boundary is a
				// *forgeerr.Error" holds without the
				// boundary's safety-net wrapping firing.
				//
				// The wrapper is the FORGE_INTERNAL_UNEXPECTED
				// shape: the failure is a write to the
				// injected writer, which is an internal
				// concern (the caller supplied the writer,
				// and its failure is not a user input
				// problem).
				//
				// # Type-level contract
				//
				// The forgeerr.Wrap call passes
				// CodeInternalUnexpected directly. Same
				// rationale as above.
				return forgeerr.Wrap(
					forgeerr.CategoryInternal,
					forgeerr.CodeInternalUnexpected,
					"failed to write version output",
					err,
				)
			}
			return nil
		},
	}

	// The only local flag. --verbose is not registered here; it is
	// the root's persistent flag, read via verboseRequested. The
	// docstring on the handler explains why.
	cmd.Flags().StringVar(&format, "format", appversion.FormatText,
		"Output format: text|json")

	return cmd
}
