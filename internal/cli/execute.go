// Package cli implements the Forge command-line interface.
//
// This file defines the process boundary of the CLI: the options
// struct, the functions that read os.* into it, and the entry point
// that transforms it into the command-boundary Dependencies struct.
//
// # The two-boundary model
//
// Forge has two structs that describe its injectable environment,
// and they serve different purposes:
//
//   - options (options.go / this file) is the process boundary. It
//     holds raw os.* values: os.Args[1:], os.Stdin, os.Stdout,
//     os.Stderr, os.Getenv, and the working directory. It is
//     constructed by defaultOptions in production, or by a test.
//
//   - Dependencies (deps.go) is the command boundary. It holds
//     resolved collaborators: a *config.Config, a Logger, a
//     filesystem.FS, two io.Writers, and an env lookup function.
//     It is constructed by buildDependencies from an options value.
//
// The transformation is one-directional and pure:
//
//	process ─► options ─► buildDependencies ─► Dependencies ─► commands
//
// There is no path from Dependencies back to options, and no path
// from a command back to the process. This is what makes every
// command testable in-process (AC5) and what enforces AC3.
//
// # The single construction site
//
// buildDependencies is called exactly once in production code, from
// executeWithOptions. This satisfies AC4. The invariant is verified
// by the Taskfile target `verify:deps` and by the structural test
// TestExecuteWithOptions_BuildsDependenciesOnce.
//
// # The single os.* reader
//
// defaultOptions is the only function in the package that reads
// os.Args, os.Stdin, os.Stdout, os.Stderr, os.Getenv, or os.Getwd.
// Every other function receives these values through the options
// struct, and every command receives them through the Dependencies
// struct built from options. This is what enforces AC3, and it is
// verified by the Taskfile target `verify:two-boundary`.
//
// # The pre-parse validation stage
//
// executeWithOptions calls validateArgs before constructing the
// command tree. The validator rejects two malformed help invocations
// that Cobra would otherwise accept silently (see validate.go and
// docs/cli-ux-spec.md § 4.10). The validation must run before Cobra
// parses the arguments: Cobra's --help interception short-circuits
// the command tree, so a malformed `--help <cmd>` invocation is
// unobservable from inside the tree. The pre-parse check is the only
// stage at which both malformed cases are visible.
//
// # The error message format
//
// Errors are rendered through the frozen format defined by WBS
// 7.3.2: a first line prefixed with "Error: ", optional context
// lines, and an optional Suggestion block. The rendering is done
// by formatError (below), which delegates to formatErrorMessage
// (errors.go). The format is documented in docs/cli-ux-spec.md
// "Error Message Format" and enforced by the tests in
// format_error_test.go and errors_test.go.
package cli

import (
	"fmt"
	"io"
	"os"
)

// options captures the injectable environment of a CLI invocation.
//
// It is the boundary between the process and the CLI's behaviour.
// Every side effect a command may perform — reading arguments,
// reading stdin, writing stdout, writing stderr, reading environment
// variables, resolving paths — flows through this struct.
//
// Tests construct an options value with synthetic inputs, call
// executeWithOptions, and assert on the captured output. This is what
// makes every command testable in-process, without spawning a
// subprocess.
//
// # Field semantics
//
// Every field is a value, not a pointer. This is deliberate: the
// struct is small (six fields), and passing it by value avoids the
// aliasing questions that come with pointers. A caller who wants to
// share state between two invocations can construct two options
// values that reference the same underlying writers.
//
// # Adding a field
//
// Adding a field to this struct is a breaking change to the tests in
// execute_test.go: every test that constructs an options value must
// be updated. The change is small, but it is a change. Add fields
// only when a command genuinely needs the new input.
//
// # Relationship to Dependencies
//
// This struct is the process boundary; Dependencies (deps.go) is the
// command boundary. buildDependencies transforms one into the other.
// Commands never see an options value; they see only Dependencies.
// This separation is what keeps command constructors stable across
// phases: adding a field to options does not change any command's
// signature, because the field is resolved into Dependencies first.
type options struct {
	// args are the command-line arguments, excluding the program
	// name. In a normal process invocation, this is os.Args[1:].
	args []string

	// stdin is the reader from which the CLI may read interactive
	// input. In a normal process invocation, this is os.Stdin.
	stdin io.Reader

	// stdout is the writer to which the CLI writes successful
	// output. In a normal process invocation, this is os.Stdout.
	stdout io.Writer

	// stderr is the writer to which the CLI writes diagnostics and
	// errors. In a normal process invocation, this is os.Stderr.
	stderr io.Writer

	// env reads environment variables by name. It is a function
	// rather than a map so that tests can inject a lookup without
	// materialising the entire environment. In a normal process
	// invocation, this is os.Getenv.
	env func(string) string

	// rootPath is the directory against which relative paths are
	// resolved. It is normally the process's current working
	// directory, but tests can set it to a temporary directory so
	// that path resolution is deterministic.
	//
	// Commands that read or write files resolve their paths against
	// this directory. When rootPath is empty, commands should treat
	// the current working directory as the root.
	//
	// WBS 4.2.1 introduced this field. No command consumes it yet;
	// the field exists so that the filesystem abstraction in WBS
	// 13.0 has a single place to read the root from. buildDependencies
	// passes it to filesystem.NewOSFS.
	rootPath string
}

// defaultOptions returns an options value bound to the current
// process.
//
// It is the only place in the package that reads os.Args, os.Stdin,
// os.Stdout, os.Stderr, os.Getenv, or os.Getwd directly. Every other
// function receives these values through the options struct, and
// every command receives them through the Dependencies struct built
// from options by buildDependencies.
//
// This is the pattern that makes the package testable in-process.
// Tests construct options directly, bypassing this function entirely.
//
// # Error handling
//
// os.Getwd may fail in rare circumstances (for example, if the
// current directory has been deleted). When it does, the empty
// string is used, and commands treat the empty root as "use the
// process's default". This matches the behaviour of the underlying
// os package, which also falls back to relative paths when Getwd
// fails.
//
// The function does not return an error. A failure to determine the
// working directory is not fatal: the CLI can still run, and the
// only consequence is that paths are resolved relative to the
// process's default rather than an explicit root. This is the
// correct behaviour for a foundation manager that must work in
// constrained environments (containers, CI runners) where the
// working directory may be unusual.
//
// # Why not use os.Environ
//
// os.Environ returns the entire environment as a slice of strings.
// The CLI does not need the entire environment; it needs to look up
// individual variables by name. A function is both cheaper and more
// testable: a test can inject a lookup that returns specific values
// without constructing a full environment map.
func defaultOptions() options {
	rootPath, err := os.Getwd()
	if err != nil {
		rootPath = ""
	}

	return options{
		args:     os.Args[1:],
		stdin:    os.Stdin,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		env:      os.Getenv,
		rootPath: rootPath,
	}
}

// Execute runs the Forge CLI and returns a process exit code.
//
// It is the only exported symbol of this package. It exists so that
// cmd/forge/main.go can delegate the entire process lifecycle to a
// single, testable function.
//
// # The signature is frozen
//
// The signature is exactly:
//
//	func Execute() int
//
// Adding parameters would break the contract with cmd/forge/main.go.
// Testability is provided by the unexported executeWithOptions
// function, which accepts an injectable environment.
//
// # Execute is a thin wrapper
//
// Execute constructs the default environment from the process and
// delegates to executeWithOptions. It contains no logic of its own.
// Every behaviour a user can observe is implemented in
// executeWithOptions or below it.
//
// # Why the process exit code is returned, not set
//
// Execute returns an int rather than calling os.Exit. This is
// deliberate: calling os.Exit inside Execute would make the function
// untestable, because os.Exit terminates the test process. Returning
// the code lets main.go decide when to exit, and lets tests call
// Execute (or executeWithOptions) and assert on the returned code
// without terminating.
//
// The convention is that main.go calls:
//
//	os.Exit(cli.Execute())
//
// and nothing else. Every other decision — argument parsing, error
// formatting, exit code mapping — is made inside the cli package.
func Execute() int {
	return executeWithOptions(defaultOptions())
}

// executeWithOptions runs the CLI with the given options and returns
// the process exit code.
//
// It is unexported because it is an implementation detail. Tests
// within the package call it directly. Downstream packages must not.
//
// # The two-boundary model
//
// This function is the single transformation point between the two
// structs that define Forge's injectable environment:
//
//   - options is the process boundary. It holds raw os.* values.
//     It is constructed by defaultOptions (in production) or by a
//     test (in tests).
//
//   - Dependencies is the command boundary. It holds resolved
//     collaborators. It is constructed by buildDependencies from
//     an options value.
//
// The transformation is one-directional and pure: given the same
// options, buildDependencies always produces the same Dependencies.
// There is no path from Dependencies back to options, and no path
// from a command back to the process. This is what makes every
// command testable in-process (AC5) and what enforces AC3.
//
// # The validation stage
//
// Before the transformation, executeWithOptions calls
// validateArgs (validate.go) on the raw argument list. The
// validator rejects malformed invocations that Cobra would
// otherwise accept silently:
//
//   - `forge --help <cmd>` — the --help flag takes no argument.
//   - `forge help <unknown>` — an unknown help topic.
//   - `forge --version <arg>` — the --version flag takes no
//     argument.
//   - `forge --config` with no value — the flag requires a value.
//
// The rejection must happen before Cobra parses the arguments:
// Cobra's --help and --version interceptions are short-circuits
// that run before any hook, so the malformed invocations are
// unobservable from inside the command tree. validateArgs is the
// only stage at which all four cases are visible.
//
// The validator returns a plain error. This function renders it
// with the same formatError used for all other errors, writes it
// to the same stderr, and maps it to an exit code with the same
// exitCodeFromError. The result is indistinguishable from an error
// produced by a command: same stream, same message shape, same
// exit code.
//
// # What the function does
//
//  1. Validates the raw argument list via validateArgs.
//  2. Transforms options into Dependencies via buildDependencies.
//  3. Constructs the root command, passing Dependencies to it.
//  4. Binds the injectable inputs and outputs to the command tree.
//  5. Executes the command tree.
//  6. Renders any returned error with formatError.
//  7. Writes the rendered error to the injected stderr.
//  8. Maps the error to an exit code via exitCodeFromError.
//
// # What the function does not do
//
//   - It does not read os.Args, os.Stdin, os.Stdout, os.Stderr, or
//     os.Getenv. Every input is read from the options value.
//   - It does not perform I/O of its own beyond writing the
//     rendered error to opts.stderr. The command tree performs the
//     rest.
//   - It does not interpret exit codes beyond calling
//     exitCodeFromError. The mapping is defined in exitcodes.go.
//
// # The error path
//
// When the command tree returns a non-nil error, the function:
//
//   - Renders the error via formatError into the frozen error
//     message format (WBS 7.3.2): a first line prefixed with
//     "Error: ", optional context lines, and an optional
//     Suggestion block.
//   - Writes the rendered string to opts.stderr, followed by a
//     newline.
//   - Returns the exit code for the error's category.
//
// When the command tree returns a nil error, the function returns
// ExitSuccess without writing anything.
//
// # The output path
//
// Successful output is written by the command tree itself, to
// opts.stdout (which is deps.Stdout). The function does not write to
// opts.stdout; it only wires it to the command tree.
//
// # Why buildDependencies is called exactly once
//
// AC4 requires that Dependencies is constructed exactly once per
// invocation. Constructing it here, and only here, satisfies that
// requirement. A future refactor that constructs Dependencies in
// newRootCmd, or in a subcommand constructor, or in a test helper
// outside this package, would violate AC4 and break the single
// source of truth for command collaborators.
//
// The Taskfile target `verify:deps` checks this invariant by
// counting the call sites of buildDependencies. The structural test
// TestExecuteWithOptions_BuildsDependenciesOnce documents it in
// Go code so that a reader of the test suite sees it.
//
// # Why the writers are bound twice
//
// The writers appear in two places: deps.Stdout / deps.Stderr (for
// commands that read them from the Dependencies struct) and
// root.SetOut / root.SetErr (for Cobra's internal use, such as
// printing help text). Both must point to the same underlying
// writer, or help output would go to one stream and command output
// to another.
//
// The binding below uses opts.stdout and opts.stderr directly, not
// deps.Stdout and deps.Stderr. This is safe because buildDependencies
// passes the same writers through unchanged. Using opts.* makes the
// equivalence explicit and avoids a spurious dependency on deps for
// the Cobra binding.
//
// # Why a nil error is not written
//
// When the command tree succeeds, nothing is written to stderr. This
// is the CLI UX contract: a successful invocation produces no
// diagnostic output. The function returns ExitSuccess immediately,
// without touching opts.stderr.
func executeWithOptions(opts options) int {
	// Validate the raw arguments before Cobra parses them. The
	// malformed invocations are rejected here; see validate.go
	// for the rationale and docs/cli-ux-spec.md for the contract.
	//
	// The validator returns a plain error. The error path below is
	// the same path used for errors returned by the command tree:
	// formatError renders the message in the frozen format,
	// Fprintln writes it to opts.stderr, and exitCodeFromError maps
	// it to an exit code. No error type distinction is necessary.
	if err := validateArgs(opts.args); err != nil {
		fmt.Fprintln(opts.stderr, formatError(err))
		return exitCodeFromError(err)
	}

	// Transform the process-boundary options into the command-
	// boundary Dependencies. This is the only call site of
	// buildDependencies in production code (AC4).
	//
	// The transformation is pure: buildDependencies reads only its
	// argument, allocates a new Dependencies value, and returns it.
	// It does not mutate opts, does not touch the filesystem, and
	// does not read the environment. A caller can therefore reason
	// about the transformation by reading a single function.
	deps := buildDependencies(opts)

	// Construct the command tree with the resolved collaborators.
	// The root command does not consume deps in Phase 2, but the
	// signature is fixed so that subcommand constructors added in
	// later WBS items can receive it without changing this call.
	root := newRootCmd(deps)

	// Bind the injectable inputs and outputs to the command tree.
	// Cobra uses these for help text, usage messages, and any
	// internal writes it performs. Commands that write their own
	// output read the writers from deps, not from Cobra.
	//
	// SetArgs is the injectable equivalent of os.Args[1:]. SetIn is
	// the injectable equivalent of os.Stdin. SetOut and SetErr are
	// the injectable equivalents of os.Stdout and os.Stderr.
	//
	// The four calls below are the only places in the package that
	// bind process-boundary values to the command tree. A reader
	// who wants to know what the CLI reads or writes can read these
	// four lines.
	root.SetArgs(opts.args)
	root.SetIn(opts.stdin)
	root.SetOut(opts.stdout)
	root.SetErr(opts.stderr)

	// Execute the command tree. Cobra dispatches to the matching
	// subcommand, or to the root's RunE handler if no subcommand
	// matches. The return value is the error from the dispatched
	// handler, or nil if the handler succeeded.
	//
	// A non-nil error means the invocation failed. The error may
	// be a usage error (unknown command, invalid flag), a runtime
	// error (filesystem failure, configuration error), or a
	// user-cancelled operation. The mapping to exit codes is
	// performed by exitCodeFromError below.
	err := root.Execute()
	if err == nil {
		// Success. No diagnostic output is written. The command
		// tree has already written any successful output to
		// opts.stdout. Return ExitSuccess.
		return ExitSuccess
	}

	// The command tree returned an error. Render it in the frozen
	// format (WBS 7.3.2) and write it to the injected stderr,
	// followed by a newline. The format is applied by formatError;
	// this function adds only the trailing newline.
	//
	// The newline is added here rather than in formatError because
	// formatError is a pure string function. A future caller that
	// wants to compose the formatted error into a larger message
	// can call formatError without stripping a trailing newline.
	//
	// The write is intentionally unchecked. If opts.stderr is a
	// broken pipe or a closed file, the write fails silently. There
	// is nothing useful the CLI can do about it: writing an error
	// about the error stream failing would itself fail. The exit
	// code is still returned, so the caller (main.go) can exit with
	// the correct status even if the diagnostic could not be
	// delivered.
	fmt.Fprintln(opts.stderr, formatError(err))

	// Map the error to an exit code. The mapping is defined in
	// exitcodes.go; it distinguishes usage errors from runtime
	// errors from cancellation, so that scripts and CI runners can
	// branch on the exit code without parsing stderr.
	return exitCodeFromError(err)
}

// formatError renders an error as a user-facing string in the
// frozen error message format (WBS 7.3.2).
//
// The format is:
//
//	Error: <message>
//
//	Suggestion:
//	  <actionable remediation>
//
// Context lines are optional and appear between the message and the
// suggestion. The format is documented in docs/cli-ux-spec.md
// "Error Message Format".
//
// # How the rendering works
//
// The function builds an errorContext from the error:
//
//   - The message is err.Error().
//   - The suggestion is extracted from the error chain via
//     suggestionOf (errors.go). An error that does not carry a
//     suggestion renders without a Suggestion block.
//
// The context lines are derived from the error's own structure.
// In Phase 2, no error carries context lines; the message is the
// only content. When WBS 10.0 introduces structured errors with
// context fields, this function is where the context is extracted
// and rendered.
//
// # Why a dedicated function
//
// The error formatting policy is not the same as the error mapping
// policy. The mapping (exitCodeFromError) decides which exit code an
// error produces. The formatting (this function) decides how the
// error is rendered. The two are separate concerns, and separating
// them means a future change to error formatting does not require
// touching the mapping.
//
// # What changes in WBS 10.0
//
// When the structured error model is introduced, this function will
// render the error's Code, Message, Context, and Remediation fields
// in the format defined by the CLI UX specification. The signature
// will not change: callers continue to pass an error and receive a
// string.
//
// The current implementation extracts the message and the
// suggestion. A future implementation will additionally extract
// the code and the context. The extraction functions
// (suggestionOf, and future siblings) live in errors.go; this
// function composes their results.
//
// # Why not fmt.Sprintln here
//
// The caller writes the rendered string to stderr and adds a
// newline. Adding the newline inside this function would make the
// function's output suitable only for writing to a stream; a future
// caller that wants to compose the string into a larger message
// would have to strip the newline. Keeping the newline outside the
// function preserves flexibility.
//
// # Nil handling
//
// The function is defensive: a nil error renders as an empty string
// rather than panicking. This branch is unreachable given how
// formatError is called (executeWithOptions returns early when err
// is nil), but the function is a pure helper and should not panic on
// any input. A future caller that passes a nil error by mistake gets
// an empty string, which is a visible symptom, rather than a panic,
// which is a crash.
func formatError(err error) string {
	if err == nil {
		return ""
	}

	ctx := errorContext{
		message:    err.Error(),
		suggestion: suggestionOf(err),
	}

	// Enforce the message-length limit. The check produces a
	// context line naming the violation if the message is over
	// maxMessageLength runes. The line is appended to the
	// context, so the developer who constructed the error sees
	// the diagnostic in the output.
	if line, ok := checkMessageLength(ctx.message); !ok {
		ctx.contextLines = append(ctx.contextLines, line)
	}

	return formatErrorMessage(ctx)
}
