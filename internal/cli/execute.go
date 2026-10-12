// Package cli implements the Forge command-line interface.
//
// This file defines the process boundary of the CLI: the options
// struct, the functions that read os.* into it, and the entry points
// that transform it into the command-boundary Dependencies struct.
//
// # The two-boundary model
//
// Forge has two structs that describe its injectable environment,
// and they serve different purposes:
//
//   - options (this file) is the process boundary. It holds raw
//     os.* values: os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
//     os.Getenv, and the working directory. It is constructed by
//     defaultOptions in production, or by a test.
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
// command testable in-process.
//
// # The single construction site
//
// buildDependencies is called exactly once in production code, from
// runWithOptions. The invariant is verified by the Taskfile
// target `verify:deps` and by the structural test
// TestExecuteWithOptions_BuildsDependenciesOnce.
//
// # The single os.* reader
//
// defaultOptions is the only function in the package that reads
// os.Args, os.Stdin, os.Stdout, os.Stderr, os.Getenv, or os.Getwd.
// Every other function receives these values through the options
// struct, and every command receives them through the Dependencies
// struct built from options. The invariant is verified by the
// Taskfile target `verify:two-boundary`.
//
// The panic recovery in Execute is a deliberate exception: it
// writes to os.Stderr directly, because a panic means the injected
// writers are not trustworthy. The exception is documented on
// Execute and in ADR-020.
//
// # The pre-parse validation stage
//
// runWithOptions calls validateArgs before constructing the
// command tree. The validator rejects malformed invocations that
// Cobra would otherwise accept silently (see validate.go and
// docs/cli-ux-spec.md § 4.9). The validation must run before Cobra
// parses the arguments: Cobra's --help interception short-circuits
// the command tree, so a malformed `--help <cmd>` invocation is
// unobservable from inside the tree.
//
// # The error boundary (WBS 10.5.1)
//
// The CLI's error handling is split across two functions, and the
// split is deliberate:
//
//   - Execute is the process's error boundary. It recovers panics
//     and converts them to FORGE_INTERNAL_PANIC errors. The
//     recovery is placed at the outermost point before os.Exit,
//     so a panic anywhere below — including inside
//     runWithOptions itself — is caught.
//
//   - runWithOptions is the rendering and exit-code boundary. It
//     ensures every error that reaches it is a *forgeerr.Error
//     (wrapping non-*Error errors as FORGE_INTERNAL_UNEXPECTED),
//     renders the error with forgeerr.Format, writes the rendered
//     string to the injected stderr (unless --quiet suppresses
//     it), writes the JSON error envelope to the injected stdout
//     when --format json is set, and returns the error's
//     Category().ExitCode() together with the error itself.
//
// The split is forced by the code's shape: the wrapping must run
// in the same function as the rendering, and the recovery must
// run at the outermost point. The two requirements put the two
// behaviours in two different functions.
//
// # The output-stream contract (WBS 11.3.2)
//
// The CLI's output streams have fixed roles:
//
//   - stdout carries the command's result. In text mode, the
//     result is human-readable. In JSON mode (--format json),
//     the result is exactly one JSON document: a success
//     envelope on success, an error envelope on failure.
//
//   - stderr carries diagnostics: the human error block on
//     failure, rendered by forgeerr.Format. --quiet suppresses
//     the diagnostic; it does not suppress the JSON envelope on
//     stdout, because the envelope is the command's result in
//     JSON mode.
//
// The contract is documented in docs/reference/output-streams.md
// and enforced by the tests in error_envelope_test.go and
// output_test.go. The envelope's schema is published at
// schemas/error-envelope.schema.json.
//
// # The exit boundary (WBS 11.3.1, ADR-020)
//
// The package exposes two public entry points:
//
//   - Run(ctx context.Context) (int, error) is the process
//     boundary. It receives the context the caller installed
//     (cmd/forge/main.go installs a signal-aware one). Run parses
//     the arguments, dispatches to the matching command, renders
//     any error, and returns the process exit code and the error.
//     Run does not call os.Exit.
//
//   - Execute() int is a thin wrapper around Run. It is preserved
//     for callers that only need the code; it calls Run with a
//     background context and discards the error.
//
// The two-value return is the seam for a future WBS that embeds
// Forge as a library. See ADR-020.
//
// # The error message format
//
// Errors are rendered through the frozen user-facing format defined
// by WBS 10.4.1 and implemented by forgeerr.Format:
//
//	Error: <message>
//	Code:  <code>
//	       <context-key-1>: <value-1>
//	       <context-key-2>: <value-2>
//
//	Suggestion:
//	  <remediation>
//
// The rules that govern the format (indentation, sorting,
// whitespace, the absence of colour) are documented in
// docs/cli-ux-spec.md "ForgeError User Format" and in ADR-016. They
// are enforced by the golden files in
// internal/forgeerr/testdata/format/.
//
// # The exit-code taxonomy (WBS 11.1.1)
//
// The exit code returned by runWithOptions is the error's
// Category().ExitCode() (WBS 10.2.1). The numeric values are
// grouped into documented ranges defined in exitcodes.go and
// published in docs/reference/exit-code-ranges.md. The two
// non-zero domains that matter at this boundary are:
//
//   - Forge findings (1–9): the command ran correctly; the subject
//     of the command is in a detectable state. The drift and
//     validation codes live here.
//   - Forge execution failures (10–19): the command could not run;
//     Forge itself encountered a problem. The config, template,
//     filesystem, and security codes live here.
//
// Three further codes are emitted by this boundary directly:
//
//   - ExitUsage (64, BSD EX_USAGE): a usage error produced by the
//     pre-parse validator or by wrapCobraError's conversion of a
//     Cobra parser error.
//   - ExitInternal (70, BSD EX_SOFTWARE): a panic recovered by
//     Execute, or a non-*forgeerr.Error wrapped by
//     wrappedUnexpectedError. Both are classified as internal
//     errors.
//   - ExitSIGINT (130) / ExitSIGTERM (143): the context the
//     caller installed was cancelled by a signal. The boundary
//     inspects the context's Cause to decide which code to
//     return.
//
// The numeric values are a public contract. They are additive only
// and are never reassigned. See ADR-018 for the decision and
// exitcodes.go for the constants.
//
// # Verbose mode
//
// When the user passes --verbose, the CLI's error path sets
// forgeerr.Options.Verbose, which enables the verbose rendering
// frozen in WBS 10.4.4: the cause chain, the stack trace (when a
// stack is available), the internal_-prefixed context keys, and
// full home-directory paths. Secret redaction is unchanged; the
// rule "Secrets are never shown" is absolute.
//
// The verbose flag is read from the command tree after Cobra has
// parsed the arguments. It is not read from the process
// environment; the flag is the only source. The JSON envelope is
// not affected by --verbose; it carries the same fields regardless.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// options captures the injectable environment of a CLI invocation.
//
// It is the boundary between the process and the CLI's behaviour.
// Every side effect a command may perform — reading arguments,
// reading stdin, writing stdout, writing stderr, reading environment
// variables, resolving paths — flows through this struct.
//
// Tests construct an options value with synthetic inputs, call
// runWithOptions, and assert on the captured output. This is what
// makes every command testable in-process, without spawning a
// subprocess.
//
// # Field semantics
//
// Every field is a value, not a pointer, except config, which is a
// pointer so that nil can represent "no injection; use the default".
// This is deliberate: the struct is small (eight fields), and
// passing it by value avoids the aliasing questions that come with
// pointers. A caller who wants to share state between two
// invocations can construct two options values that reference the
// same underlying writers.
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

	// config is a pre-built configuration to inject into the
	// command tree, bypassing the config loader.
	//
	// # When to use this field
	//
	// In production, this field is nil. The config loader (WBS
	// 8.4.x) resolves the effective configuration from the
	// discovered file, the environment, and the flags, and
	// buildDependencies constructs the Dependencies.Config from
	// the loader's output. defaultOptions does not set this
	// field, so a production invocation follows the loader path.
	//
	// In tests, this field may be set to a *config.Config value
	// that the test has constructed directly. buildDependencies
	// uses the injected value as Dependencies.Config, so the
	// command tree sees the test's configuration instead of the
	// loader's. This is how golden-file tests exercise the
	// renderers with specific configurations (with a file, with
	// non-default sources, and so on) without needing the loader
	// to run.
	//
	// # Why a test-injection seam is in options
	//
	// options is the process boundary: the struct through which
	// every input to the CLI flows. A test-injected Config is an
	// input in the same sense — it is the value the command tree
	// reads — even though production does not set it. Putting
	// the field here (rather than, say, in a parallel struct or
	// a global) keeps the injection seam on the same boundary as
	// every other input, so a reader who wants to know what the
	// CLI reads has one struct to read.
	//
	// # The nil case
	//
	// A nil config field is the normal production state.
	// buildDependencies treats nil as "no injection; construct
	// the default Config", so a nil field does not require a
	// branch at the call site.
	config *config.Config

	// verbose enables the verbose error rendering frozen in WBS
	// 10.4.4.
	//
	// # When the field is set
	//
	// In production, the field is populated from the --verbose
	// global flag (WBS 5.3.1) after Cobra parses the arguments.
	// defaultOptions does not set the field; the flag is the only
	// source.
	//
	// In tests, the field may be set directly to exercise the
	// verbose rendering through runWithOptions. A test that
	// wants the non-verbose rendering sets the field to false, or
	// constructs an options value without setting it.
	//
	// # Why the field is on options, not on Dependencies
	//
	// The verbose flag changes the rendering of the error block,
	// not the behaviour of any command. The error path is part of
	// runWithOptions, which reads from options. Putting the
	// field on Dependencies would require every command to
	// forward it to the error path, which is not how the error
	// path works: the error path is called by runWithOptions
	// after the command tree returns, not by the command itself.
	//
	// The field's place is on the process boundary, alongside the
	// other inputs the error path consults.
	verbose bool
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
//
// # Why verbose is not set here
//
// The verbose flag is read from the command tree after Cobra has
// parsed the arguments. defaultOptions runs before Cobra parses,
// so it cannot set the field. The field is populated by
// runWithOptions after the command tree returns, from the
// flag's value.
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

// Run runs the Forge CLI with the given context and returns the
// process exit code and the error it rendered.
//
// Run is the process boundary. It is called by cmd/forge/main.go
// with a context that is cancelled on SIGINT or SIGTERM. Run
// itself installs no signal handlers: the caller owns the
// context.
//
// # The two-value return
//
// The first return is the process exit code, always a documented
// value from the WBS 11.1.2 table. The second is the error the
// boundary rendered, or nil on success. main ignores the error
// (it has already been rendered to stderr); a test or a future
// library caller uses it to inspect the failure.
//
// A nil error paired with a non-zero code is impossible in the
// current design: the boundary maps the error to a code, and the
// code is non-zero if and only if the error is non-nil.
//
// # Signal handling
//
// Run expects the context to be cancelled on SIGINT or SIGTERM.
// When the cancellation arrives, the command tree's
// ExecuteContext returns the context's error; the boundary
// inspects the context's Cause to decide whether the signal was
// SIGINT (→ ExitSIGINT, 130) or SIGTERM (→ ExitSIGTERM, 143). A
// context cancelled by something other than a signal maps to
// ExitSIGINT as the more common case.
//
// # What Run does not do
//
// Run does not call os.Exit. The process terminates in main, not
// here. This is the single-exit-boundary rule (ADR-020); the CI
// check scripts/deps/verify-no-os-exit.sh enforces it.
//
// # Relationship to Execute
//
// Execute is a thin wrapper around Run. It calls Run with a
// background context and discards the error, returning only the
// code. Execute predates Run (WBS 4.1.1); it is preserved for
// backward compatibility. New code should call Run.
func Run(ctx context.Context) (int, error) {
	return runWithOptions(ctx, defaultOptions())
}

// Execute runs the Forge CLI and returns a process exit code.
//
// It is a thin wrapper around Run. It calls Run with a
// background context and discards the error, returning only the
// code. The wrapper exists for backward compatibility: callers
// that only need the code continue to work; callers that want
// the error call Run.
//
// # The signature is frozen
//
// The exported signature is exactly:
//
//	func Execute() int
//
// The named return (`code`) is a syntactic detail of the
// implementation; it is not part of the signature. Callers see
// `func Execute() int` and use the value as a process exit code.
// The name exists because the panic recovery path must set the
// return value from a deferred function; Go has no other way to
// do so.
//
// # What the function does
//
//  1. Recovers from any panic that occurs below it. The panic is
//     converted to a FORGE_INTERNAL_PANIC error (via
//     recoveredPanicError), rendered through forgeerr.Format,
//     written to os.Stderr, and returned as the process exit code
//     (ExitInternal, 70, per CategoryInternal.ExitCode()).
//
//  2. Calls Run with a background context.
//
//  3. Returns the code Run returned. The function does not
//     re-derive the code; it trusts the inner function's result.
//
// # Why panics are recovered here
//
// The alternative — letting a panic escape — terminates the
// process without a rendered error. The user sees a Go stack
// trace, not a Forge error block. The exit code is 2 (Go's panic
// exit) rather than a Forge code, which breaks scripts that
// branch on the Forge exit codes.
//
// Recovering the panic at the boundary converts it into a
// first-class error: the user sees the frozen format with the
// FORGE_INTERNAL_PANIC code, and the process exits with the
// internal-error code (ExitInternal, 70). The panic's value is
// preserved in the error's context map, so a developer can see
// what went wrong.
//
// # Why the recovery is not in runWithOptions
//
// The recovery is placed at the outermost point before os.Exit,
// so a panic anywhere below — including inside runWithOptions
// itself — is caught. Moving it into runWithOptions would
// leave a window in which a panic before the deferred recovery is
// set up (for example, in defaultOptions, or in
// buildDependencies) escapes.
//
// The trade-off is testability: the recovery path cannot be
// exercised through the test helpers, because the helpers call
// runWithOptions, not Execute. The conversion is extracted
// into recoveredPanicError so that it can be tested directly. The
// placement of the deferred recovery is what is not tested; its
// correctness is a matter of code review, not of test.
//
// # Why the panic path writes directly to os.Stderr
//
// The panic recovery runs before runWithOptions, so the
// injected stderr (opts.stderr) is not yet available. The
// boundary writes to os.Stderr directly. This is one of the
// two places in the package that touch a process stream
// directly; the other is defaultOptions, which reads them.
//
// The direct write is a deliberate exception to the two-boundary
// rule, scoped to the panic path. A panic is the failure of the
// process environment, not of the command tree; the injected
// writers are not trustworthy in that state. A future WBS item
// that wants to route the panic path through the injection layer
// would need to construct the options value before the defer,
// which is possible but adds complexity for a path that is, by
// definition, exceptional.
//
// # Why the panic path's verbose flag is a heuristic
//
// The authoritative value of --verbose is on the command tree,
// which the panic path cannot reach: a panic may occur before
// Cobra has parsed the arguments. The boundary scans os.Args
// for the literal "--verbose" token as a best-effort
// approximation.
//
// A false negative (the flag was passed but the scan did not
// find it) renders the panic in non-verbose mode; the message
// and code are still shown, and the stack trace is available
// through the log layer. A false positive (the scan finds the
// string but the user did not pass the flag) renders the panic
// in verbose mode; the additional information is visible, which
// is not a security concern because the panic path does not
// handle user data.
//
// # The named return and main.go
//
// main.go calls:
//
//	os.Exit(cli.Execute())
//
// and nothing else. The named return is invisible to main.go:
// the caller sees an int, which is the process exit code.
func Execute() (code int) {
	defer func() {
		if r := recover(); r != nil {
			e := recoveredPanicError(r)

			// Render and write. The write goes to os.Stderr
			// directly; see the docstring above for the
			// rationale.
			//
			// The verbose flag is read from os.Args as a
			// best-effort. See the docstring for the
			// rationale and the acceptable failure modes.
			fmt.Fprint(os.Stderr, forgeerr.Format(e, forgeerr.Options{
				Verbose: hasVerboseArg(os.Args),
			}))

			// The exit code is the internal-error code
			// (ExitInternal, 70). We do not call os.Exit here;
			// main.go does. The named return carries the code
			// to the caller.
			code = e.Category().ExitCode()
		}
	}()

	// Run the command tree. The call is wrapped so that a panic
	// in the tree is caught by the deferred function above.
	//
	// The error handling (wrapping, rendering, exit-code
	// derivation) is performed by runWithOptions. The
	// boundary only handles the panic path.
	code, _ = Run(context.Background())
	return code
}

// recoveredPanicError converts a recovered panic value into a
// *forgeerr.Error.
//
// The error is category CategoryInternal, code
// CodeInternalPanic, message "internal error occurred", and the
// panic's string representation in the context map under the key
// "panic".
//
// The function is called by Execute's deferred recovery. It is
// extracted so that the conversion can be tested directly, without
// invoking Execute (which would read the process's os.Args and
// write to os.Stderr, neither of which the test can control).
//
// # The context key
//
// The key "panic" is snake_case, satisfies the context-key rule
// (WBS 10.1.2), and is not in the secret-keys list (WBS 10.4.3),
// so it is not redacted. The value is the panic value's string
// representation; fmt.Sprint handles every type, including nil
// (which renders as "<nil>") and errors (which render via
// Error()).
//
// # Why the message is not the panic value
//
// The message is a fixed, non-user-authored string. The panic
// value is in the context map, where it is rendered as a context
// line by forgeerr.Format and as a structured attribute by
// LogValue. Keeping the message fixed means the message
// invariants (single line, no trailing period, <= 80 bytes) hold
// regardless of the panic value's content.
//
// # The exit code
//
// The error's category is CategoryInternal, whose ExitCode() is
// ExitInternal (70, BSD EX_SOFTWARE). The panic path therefore
// exits with the internal-error code, not with the usage code and
// not with a finding. This is the correct classification: a panic
// is a bug in Forge, not a usage error and not a subject-side
// finding.
func recoveredPanicError(panicValue any) *forgeerr.Error {
	return forgeerr.New(
		forgeerr.CategoryInternal,
		string(forgeerr.CodeInternalPanic),
		"internal error occurred",
	).WithContext("panic", fmt.Sprint(panicValue))
}

// wrappedUnexpectedError wraps a non-*forgeerr.Error in a
// FORGE_INTERNAL_UNEXPECTED error.
//
// The wrapper preserves the original error's message in its
// context under the key "cause". The original error is attached
// via WithCause, so that errors.Is and errors.As see through the
// wrapper.
//
// The function is called by runWithOptions when the command
// tree returns an error that is neither a *forgeerr.Error nor
// wraps one. It is extracted so that the wrapping can be tested
// directly, in addition to being tested through the CLI's error
// path.
//
// # Why the message is not the original error's message
//
// The wrapper's message is "unexpected error", the category is
// CategoryInternal, and the code is CodeInternalUnexpected. The
// original error's message is preserved in the context and in
// the cause, not in the wrapper's message. The separation keeps
// the wrapper's *forgeerr.Error message within the 80-byte limit
// and gives the format a stable, non-user-authored message.
//
// # Why the context key is "cause"
//
// The key "cause" is snake_case, satisfies the context-key rule,
// and is not in the secret-keys list. It is the natural name for
// the original error's message, and it matches the log format's
// own "cause" attribute (WBS 10.4.2). A reader who sees the
// context line `cause: plain error` in the user-facing output
// understands that the wrapper's cause is the source of the
// failure.
//
// # The wrapping's location
//
// The wrapping is in runWithOptions, not in Execute, because
// the rendering is in runWithOptions. If the wrapping were in
// Execute, it would run after runWithOptions had already
// rendered the un-wrapped error, and the wrapped error would
// never reach the user.
//
// # Why the error is wrapped and not replaced
//
// The original error is attached via WithCause, so the wrapper
// preserves the original error's chain. A caller that uses
// errors.Is or errors.As on the wrapper sees the original error
// through the chain. This is the property that makes the wrapper
// a wrapper and not a replacement.
//
// # The exit code
//
// The error's category is CategoryInternal, whose ExitCode() is
// ExitInternal (70, BSD EX_SOFTWARE). An unexpected error is a
// bug in Forge, not a usage error and not a subject-side finding.
// The exit code reflects that classification.
func wrappedUnexpectedError(cause error) *forgeerr.Error {
	// forgeerr.Wrap returns the cause unchanged if it is already
	// a *forgeerr.Error. The caller checks for that case before
	// calling this function; the check is a documented invariant
	// of wrappedUnexpectedError's contract.
	return forgeerr.Wrap(
		forgeerr.CategoryInternal,
		string(forgeerr.CodeInternalUnexpected),
		"unexpected error",
		cause,
	).WithContext("cause", cause.Error())
}

// hasVerboseArg reports whether the given argument vector contains
// the --verbose flag.
//
// The function is a heuristic used by the panic recovery path. It
// scans the arguments for the exact token "--verbose" and for the
// short form prefix of "--verbose=...". It does not attempt to
// parse the argument grammar, does not consult Cobra, and does
// not distinguish a flag from a positional argument that happens
// to be the string "--verbose".
//
// The heuristic is sufficient for the panic recovery path. See
// Execute's docstring for the rationale and the acceptable
// failure modes.
//
// The function takes an argument vector by value because the
// panic recovery path reads the process arguments directly; there
// is no injected argument vector available at that point.
func hasVerboseArg(args []string) bool {
	const flag = "--verbose"
	for _, a := range args {
		if a == flag {
			return true
		}
		if len(a) > len(flag)+1 && a[:len(flag)+1] == flag+"=" {
			// --verbose=true, --verbose=1, etc. The value is
			// not inspected: any value other than "false" is
			// treated as true by Cobra, and the panic path
			// does not need to distinguish.
			return true
		}
	}
	return false
}

// runWithOptions runs the CLI with the given context and options
// and returns the exit code and the rendered error.
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
// command testable in-process and what enforces the "no direct
// os.* access from commands" rule.
//
// # The validation stage
//
// Before the transformation, runWithOptions calls
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
// The validator returns a *forgeerr.Error with category
// CategoryCLI. The error path renders it with forgeerr.Format,
// writes it to the same stderr, and maps it to the CLI
// category's exit code (ExitUsage, 64). The result is
// indistinguishable from an error produced by a command: same
// stream, same message shape, same exit code.
//
// # What the function does
//
//  1. Checks the context for cancellation. A cancelled context
//     means the user or supervisor requested termination; the
//     boundary returns the signal's code without doing any work.
//  2. Validates the raw argument list via validateArgs.
//  3. Transforms options into Dependencies via buildDependencies.
//  4. Constructs the root command, passing Dependencies to it.
//  5. Binds the injectable inputs and outputs to the command tree.
//  6. Executes the command tree with the context.
//  7. Converts any Cobra-produced error (unknown flag, unknown
//     command) into a *forgeerr.Error with category CategoryCLI.
//  8. Ensures the error is a *forgeerr.Error; a non-*Error error
//     is wrapped as FORGE_INTERNAL_UNEXPECTED.
//  9. Reads the --verbose flag from the command tree.
//  10. Reads the --format flag from the command tree.
//  11. Reads the --quiet flag from the command tree.
//  12. Renders the error with forgeerr.Format, in verbose mode if
//     the --verbose flag was set.
//  13. Writes the rendered error to the injected stderr, unless
//     --quiet suppresses it.
//  14. Writes the JSON error envelope to the injected stdout when
//     --format json is set (WBS 11.3.2).
//  15. Returns the error's Category().ExitCode() and the error.
//
// # What the function does not do
//
//   - It does not read os.Args, os.Stdin, os.Stdout, os.Stderr, or
//     os.Getenv. Every input is read from the options value.
//   - It does not perform I/O of its own beyond writing the
//     rendered error to opts.stderr and the JSON envelope to
//     opts.stdout. The command tree performs the rest.
//   - It does not recover panics. The recovery is in Execute, at
//     the outermost point; see Execute's docstring for the
//     rationale.
//   - It does not call os.Exit. The process terminates in main.
//
// # The error path
//
// When the command tree returns a non-nil error, the function
// applies two wrapping steps in order:
//
//  1. wrapCobraError recognizes Cobra's parser and dispatcher
//     errors (unknown flag, unknown command) by message prefix
//     and converts each to a *forgeerr.Error with category
//     CategoryCLI and the appropriate code. An error that is
//     already a *forgeerr.Error, or that is not a recognized
//     Cobra error, passes through unchanged.
//
//  2. The boundary's own wrapping converts any error that is
//     still not a *forgeerr.Error to FORGE_INTERNAL_UNEXPECTED.
//     This step is the safety net; after the migrations of
//     Cobra, the version command, and the config package, it
//     fires only for a genuine contract violation.
//
// The two-step ordering matters. A recognized Cobra error is
// converted by the first step, so the second step does not fire
// and the error is classified as a usage error (ExitUsage, 64).
// An unrecognized error falls through the first step and is
// wrapped by the second step as internal (ExitInternal, 70).
//
// The function then renders the (possibly wrapped) error with
// forgeerr.Format, writes the rendered string to opts.stderr
// (unless --quiet suppresses it), writes the JSON envelope to
// opts.stdout when --format json is set, and returns the error's
// Category().ExitCode() together with the error itself.
//
// When the command tree returns a nil error, the function returns
// (ExitSuccess, nil) without writing anything.
//
// # Context cancellation
//
// The context is passed to root.ExecuteContext. When the context
// is cancelled (by a signal handler in main, or by a test), the
// command tree returns the context's error. The boundary treats
// a cancelled context specially: the error is not rendered (the
// cancellation is the user's intent, not a failure); the code is
// ExitSIGINT (130) or ExitSIGTERM (143) depending on the cause.
//
// # Why fmt.Fprint, not fmt.Fprintln
//
// forgeerr.Format returns a string ending with exactly one "\n"
// (forgeerr rule F10). Writing it with fmt.Fprintln would add a
// second "\n", producing a trailing blank line and violating F10
// at the output boundary. The call sites use fmt.Fprint so that
// the bytes on stderr are exactly what forgeerr.Format produced.
//
// This is the same discipline the formatter itself follows: the
// producer is responsible for the complete byte-exact output
// including its terminator, and the consumer writes it verbatim.
//
// # The output path
//
// Successful output is written by the command tree itself, to
// opts.stdout (which is deps.Stdout). The function does not write
// to opts.stdout on success; it only wires it to the command
// tree.
//
// On failure, the function may write the JSON error envelope to
// opts.stdout. The write is performed by writeFailure
// (output.go), which is a no-op in text mode. The two modes'
// behaviors are documented in docs/reference/output-streams.md.
//
// # Why buildDependencies is called exactly once
//
// The invariant requires that Dependencies is constructed exactly
// once per invocation. Constructing it here, and only here,
// satisfies the invariant. A future refactor that constructs
// Dependencies in newRootCmd, or in a subcommand constructor, or
// in a test helper outside this package, would violate it and
// break the single source of truth for command collaborators.
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
// deps.Stdout and deps.Stderr. This is safe because
// buildDependencies passes the same writers through unchanged.
// Using opts.* makes the equivalence explicit and avoids a
// spurious dependency on deps for the Cobra binding.
//
// # Why a nil error is not written
//
// When the command tree succeeds, nothing is written to stderr.
// This is the CLI UX contract: a successful invocation produces no
// diagnostic output. The function returns (ExitSuccess, nil)
// immediately, without touching opts.stderr.
//
// # Why verbose is read after the command tree runs
//
// The verbose flag is a property of the invocation. It is parsed
// by Cobra when the command tree runs, so the value is not
// available before root.ExecuteContext(). The flag's only
// consumer in this function is the error rendering, which runs
// after the command tree returns. The ordering is therefore
// natural: execute the tree, read the flag from the tree, render
// any error with the flag's value.
//
// # Why format and quiet are read after the command tree runs
//
// The --format and --quiet flags are properties of the invocation,
// parsed by Cobra during the tree's execution. The error path
// reads them after the tree returns, in the same way it reads
// --verbose. The three flags are read together so that a reader
// sees them as a group: the flags that modify the error path's
// output.
//
// # The exit-code taxonomy
//
// The code returned by this function is the error's
// Category().ExitCode(). The numeric values are grouped into
// documented ranges defined in exitcodes.go and published in
// docs/reference/exit-code-ranges.md. The ranges this function
// can produce are:
//
//   - ExitSuccess (0): the command tree returned nil.
//   - Forge findings (1–9): a command reported a finding, e.g.
//     drift detected or a validation warning promoted to error.
//     These are not tool malfunctions; the caller decides what to
//     do.
//   - Forge execution failures (10–19): a command could not run,
//     e.g. a config, template, filesystem, or security error.
//     These are tool malfunctions from the caller's perspective.
//   - ExitUsage (64): the pre-parse validator rejected the
//     invocation, or wrapCobraError classified a Cobra parser
//     error as a usage error.
//   - ExitInternal (70): the boundary wrapped a non-*forgeerr.Error
//     as FORGE_INTERNAL_UNEXPECTED, or a panic was recovered by
//     Execute (in which case Execute returns the code, not this
//     function).
//   - ExitSIGINT (130) / ExitSIGTERM (143): the context was
//     cancelled by a signal.
//
// The numeric values are a public contract. They are additive
// only and are never reassigned. See ADR-018 for the decision.
func runWithOptions(ctx context.Context, opts options) (int, error) {
	// Fast path: the context is cancelled before we start. This
	// happens when a signal arrives during process startup, before
	// the command tree runs. The boundary returns the signal's
	// code without doing any work.
	if cerr := ctx.Err(); cerr != nil {
		return signalExitCode(ctx), cerr
	}

	// Validate the raw arguments before Cobra parses them. The
	// malformed invocations are rejected here; see validate.go
	// for the rationale and docs/cli-ux-spec.md for the contract.
	//
	// The validator returns a *forgeerr.Error with category
	// CategoryCLI (the four rejections are all usage errors).
	// The error is rendered with the same format as any other
	// error; the exit code is the CLI category's code
	// (ExitUsage, 64).
	//
	// The write uses fmt.Fprint, not fmt.Fprintln, because
	// forgeerr.Format already includes the trailing newline
	// (rule F10 of the frozen format).
	if err := validateArgs(opts.args); err != nil {
		fmt.Fprint(opts.stderr, forgeerr.Format(err, forgeerr.Options{
			Verbose: opts.verbose,
		}))
		return exitCodeOf(err), err
	}

	// Transform the process-boundary options into the command-
	// boundary Dependencies. This is the only call site of
	// buildDependencies in production code.
	deps := buildDependencies(opts)

	// Construct the command tree with the resolved collaborators.
	root := newRootCmd(deps)

	// Bind the injectable inputs and outputs to the command tree.
	root.SetArgs(opts.args)
	root.SetIn(opts.stdin)
	root.SetOut(opts.stdout)
	root.SetErr(opts.stderr)

	// Execute the command tree with the context. The tree's
	// RunE handlers receive the context via cmd.Context() and
	// may return early if it is cancelled.
	err := root.ExecuteContext(ctx)
	if err == nil {
		// Success. No diagnostic output is written. The command
		// tree has already written any successful output to
		// opts.stdout. Return (ExitSuccess, nil).
		return ExitSuccess, nil
	}

	// If the context is cancelled, the command tree's error is
	// the context's error. The boundary returns the signal's
	// code and does not render the error: a cancellation is the
	// user's intent, not a failure to report.
	if cerr := ctx.Err(); cerr != nil {
		return signalExitCode(ctx), cerr
	}

	// Step 1: convert Cobra's parser and dispatcher errors into
	// *forgeerr.Error values with category CategoryCLI.
	err = wrapCobraError(err)

	// Step 2: ensure the error is a *forgeerr.Error (or wraps
	// one). If it is not, wrap it as FORGE_INTERNAL_UNEXPECTED.
	var ferr *forgeerr.Error
	if !errors.As(err, &ferr) {
		err = wrappedUnexpectedError(err)
		// Re-extract the wrapped error. wrappedUnexpectedError
		// returns a *forgeerr.Error, so the next errors.As
		// succeeds. The re-extraction is performed here so that
		// the writeFailure call below receives a non-nil
		// *forgeerr.Error regardless of which wrapping step
		// produced it.
		_ = errors.As(err, &ferr)
	}

	// Read the invocation flags from the command tree. The flags
	// are properties of the invocation; they are parsed by Cobra
	// during the tree's execution, so they are not available
	// before root.ExecuteContext().
	//
	// Only --verbose and --format are read here. --quiet is
	// deliberately NOT read: the flag suppresses informational
	// log lines (a WBS 12.0 concern), not the error block. See
	// the comment on the error write below.
	verbose := verboseRequested(root)
	format := formatRequested(root)

	// Render the error and write it to the injected stderr. The
	// write is unconditional.
	//
	// # Why --quiet does not guard this write
	//
	// WBS 7.4.3's flag-interaction contract states explicitly
	// that errors are never suppressed by --quiet. The flag's
	// semantics in Phase 2 are "suppress informational log
	// lines"; Phase 2 emits none, so the flag has no effect on
	// any Phase 2 output. The error block is not an
	// informational log line; it is the user-facing explanation
	// of a failure, and hiding it would make the CLI's failure
	// modes invisible in the very scenarios (quiet CI runs,
	// scripted invocations) where the flag is most likely to be
	// used.
	//
	// WBS 11.3.2's "suppresses diagnostics" refers to
	// informational diagnostics, not to the error block. The
	// WBS's own parenthetical — "(errors still printed to
	// stderr; ...)" — names the rule that this write follows.
	//
	// The JSON error envelope on stdout (written below) is
	// likewise not suppressed. The envelope is the command's
	// result in JSON mode; the exit code and the envelope's
	// contents are the machine consumer's data.
	//
	// The write uses fmt.Fprint, not fmt.Fprintln, because
	// forgeerr.Format produces the complete byte-exact string
	// including the trailing newline (rule F10).
	fmt.Fprint(opts.stderr, forgeerr.Format(err, forgeerr.Options{
		Verbose: verbose,
	}))

	// Write the JSON error envelope to stdout when --format json
	// is set. In text mode the call is a no-op.
	//
	// The write's error is intentionally unchecked: the exit code
	// is the primary signal, and a failed write on stdout is not
	// a reason to change the classification. The envelope's
	// encoder is deterministic and produces valid JSON for every
	// *forgeerr.Error; a write error here would be a broken pipe
	// or a closed stream, neither of which the CLI can act on.
	if ferr != nil {
		_, _ = writeFailure(opts.stdout, format, ferr)
	}

	// Map the error to an exit code. The mapping is defined by
	// the error's category; exitCodeOf extracts the
	// *forgeerr.Error from the chain and reads its
	// Category().ExitCode().
	return exitCodeOf(err), err
}

// signalExitCode returns the exit code for a cancelled context.
//
// The function inspects the context's Cause to distinguish SIGINT
// from SIGTERM. If the cause is not a signal (or the cause is not
// available), the function returns ExitSIGINT (130) as the more
// common case.
//
// # The Cause's shape
//
// signal.NotifyContext attaches an os.Signal to the context by
// passing it to context.WithCancelCause, whose cancel function
// takes an error. Since os.Signal does not satisfy error, the
// standard library wraps the signal in an unexported type that
// satisfies both interfaces. The wrapper's dynamic type is not
// syscall.Signal; a direct `cause == syscall.SIGTERM` comparison
// fails.
//
// The function therefore checks two shapes:
//
//  1. The cause is a bare syscall.Signal (the shape a caller that
//     bypasses signal.NotifyContext might produce). The direct
//     comparison `sig == syscall.SIGTERM` succeeds.
//
//  2. The cause is a wrapper whose embedded os.Signal is
//     syscall.SIGTERM. The wrapper's dynamic type is not
//     syscall.Signal, so the direct comparison fails. The
//     comparison is made via the signal's String()
//     representation instead: syscall.SIGTERM.String() is
//     "terminated", and the wrapper's promoted String() method
//     returns the same string.
//
// The two checks together cover both shapes. A future Go version
// that changes the wrapper's shape must be matched by a change to
// this function; the test TestRun_ContextCancelled_ReturnsSIGTERM
// exercises the wrapper shape, and a failure of that test is the
// signal that the function needs updating.
//
// # Why the fallback is ExitSIGINT
//
// A cancelled context whose cause is not a signal is most often a
// Ctrl-C that the signal handler delivered without a cause, or a
// context cancelled by a test. The SIGINT code is the more common
// case; the alternative would be to invent a third code for
// "cancelled by an unknown cause", which the taxonomy does not
// have.
//
// # The exit codes
//
//   - syscall.SIGTERM → ExitSIGTERM (143, 128 + 15).
//   - os.Interrupt, nil, or any other cause → ExitSIGINT
//     (130, 128 + 2).
//
// Both codes are in the signals range (129–255) and follow the
// POSIX shell convention.
func signalExitCode(ctx context.Context) int {
	cause := context.Cause(ctx)
	if cause == nil {
		return ExitSIGINT
	}

	// The cause must satisfy os.Signal for the comparison to be
	// meaningful. A cause that does not satisfy os.Signal is not
	// a signal; the fallback applies.
	sig, ok := cause.(os.Signal)
	if !ok {
		return ExitSIGINT
	}

	// Shape 1: the cause is a bare syscall.Signal. The direct
	// comparison works because the dynamic type of the interface
	// value equals the concrete type of syscall.SIGTERM.
	if sig == syscall.SIGTERM {
		return ExitSIGTERM
	}

	// Shape 2: the cause is a wrapper around an os.Signal. The
	// wrapper's dynamic type is not syscall.Signal, so the
	// direct comparison above is false. The comparison is made
	// via the signal's String() representation instead.
	//
	// syscall.SIGTERM.String() returns "terminated"; the
	// wrapper's promoted String() returns the same string.
	if sig.String() == syscall.SIGTERM.String() {
		return ExitSIGTERM
	}

	return ExitSIGINT
}

// exitCodeOf returns the process exit code for err.
//
// If err is a *forgeerr.Error or wraps one, the code is the
// error's Category().ExitCode(). Otherwise, the code is
// ExitInternal (70, BSD EX_SOFTWARE).
//
// The function is the CLI's exit-code derivation. It is
// deliberately small: the category's ExitCode method is the
// source of truth, and this function only handles the case
// where no *Error is present.
//
// # Why a local helper, not Category.ExitCode directly
//
// The category's ExitCode is a method on Category, not on error.
// The call site has an error value, and must extract the
// *forgeerr.Error before it can read the category. The helper
// performs the errors.As extraction and the nil fallback; a
// caller that already has a *forgeerr.Error calls the method
// directly.
//
// # The non-*Error branch
//
// The branch is a safety net. The wrapping in runWithOptions
// ensures a *forgeerr.Error is present in the chain when this
// function is called from the error path. A caller that
// constructs runWithOptions directly and returns a
// non-*Error error without the wrapping is responsible for the
// consequence; the helper's fallback (ExitInternal, 70) is the
// correct code for an unclassified internal error.
//
// The fallback is ExitInternal, not ExitFailure. The latter was
// the WBS 4.1.2 constant and has been superseded by the WBS
// 11.1.1 taxonomy: a bare error is a bug in Forge, not a finding,
// and its code belongs in the internal-error range (70–78), not
// in the findings range (1–9).
func exitCodeOf(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var ferr *forgeerr.Error
	if errors.As(err, &ferr) {
		return ferr.Category().ExitCode()
	}
	// The error is not a *forgeerr.Error and does not wrap one.
	// Classify it as an internal error: the boundary's safety net
	// (wrappedUnexpectedError) normally prevents this branch from
	// being reached through runWithOptions; a direct caller of
	// runWithOptions that constructs a non-*Error error is
	// responsible for the consequence.
	return ExitInternal
}

// wrapCobraError converts a Cobra-produced error into a
// *forgeerr.Error with category CategoryCLI.
//
// Cobra's parser and dispatcher produce plain errors for the
// failures they detect:
//
//   - "unknown flag: <name>" — an unrecognized long flag.
//   - "unknown shorthand flag: '<x>' in -<name>" — an unrecognized
//     short flag.
//   - "unknown command \"<name>\" for \"<parent>\"" — an
//     unrecognized subcommand.
//
// These errors are plain *errors.errorString values. Cobra does
// not export the types it uses, so the CLI cannot match on the
// concrete type. The function recognizes the errors by their
// message prefixes and maps each to a registered CLI code.
//
// # What the function returns
//
// For a recognized Cobra error, the function returns a
// *forgeerr.Error with:
//
//   - Category: CategoryCLI.
//   - Code: CodeCLIUnknownFlag or CodeCLIUnknownCommand, depending
//     on the message prefix.
//   - Message: a short fixed string ("unknown flag" or "unknown
//     command").
//   - Context: the varying data from Cobra's message (the flag
//     name or the command name).
//
// For an error that is already a *forgeerr.Error (or wraps one),
// the function returns the error unchanged. For a non-Cobra plain
// error, the function returns the error unchanged; the boundary's
// own wrapping then classifies it as FORGE_INTERNAL_UNEXPECTED,
// which is the correct code for an unrecognized failure.
//
// # Why the message is fixed and the varying data is in the context
//
// Cobra's messages include the varying data (the flag name or the
// command name). If the wrapper's message included that data, the
// wrapper would inherit Cobra's message shape, which is not the
// frozen format and does not satisfy the *forgeerr.Error message
// invariants for every possible flag or command name. Using a
// fixed message and putting the varying data in the context keeps
// the invariants trivially satisfied and produces the frozen
// format:
//
//	Error: unknown flag
//	Code:  FORGE_CLI_UNKNOWN_FLAG
//	       flag: --unknown-flag
//
// # The exit code
//
// The error's category is CategoryCLI, whose ExitCode() is
// ExitUsage (64, BSD EX_USAGE). A Cobra parser error is a usage
// error: the user invoked the command incorrectly. It is not a
// finding (the command did not run) and not an internal error
// (the CLI is working as designed).
//
// # Why the function is not inlined
//
// The function is a helper so that it can be tested directly. The
// test table in execute_test.go covers each recognized prefix and
// the unrecognized case. The table is easier to maintain than a
// set of integration tests that exercise the same shapes through
// the full CLI.
func wrapCobraError(err error) error {
	if err == nil {
		return nil
	}

	// Already a *forgeerr.Error (or wraps one): leave it alone.
	// This check runs first so that a *forgeerr.Error produced by
	// a command is not re-wrapped by the prefix matching.
	var ferr *forgeerr.Error
	if errors.As(err, &ferr) {
		return err
	}

	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "unknown flag: "):
		name := strings.TrimPrefix(msg, "unknown flag: ")
		return forgeerr.Wrap(
			forgeerr.CategoryCLI,
			string(forgeerr.CodeCLIUnknownFlag),
			"unknown flag",
			err,
		).WithContext("flag", name)

	case strings.HasPrefix(msg, "unknown shorthand flag: "):
		// Cobra's shorthand message is "unknown shorthand flag:
		// '<x>' in -<name>". The full remainder is stored; the
		// context key is "flag" for consistency with the long
		// form.
		detail := strings.TrimPrefix(msg, "unknown shorthand flag: ")
		return forgeerr.Wrap(
			forgeerr.CategoryCLI,
			string(forgeerr.CodeCLIUnknownFlag),
			"unknown flag",
			err,
		).WithContext("flag", detail)

	case strings.HasPrefix(msg, "unknown command "):
		// Cobra's command message is
		// `unknown command "<name>" for "<parent>"`. The name is
		// extracted from between the double quotes for the
		// context key "command"; the parent is stored under
		// "parent".
		name, parent := parseUnknownCommandMessage(msg)
		e := forgeerr.Wrap(
			forgeerr.CategoryCLI,
			string(forgeerr.CodeCLIUnknownCommand),
			"unknown command",
			err,
		).WithContext("command", name)
		if parent != "" {
			e = e.WithContext("parent", parent)
		}
		return e
	}

	// The error is not a recognized Cobra error. The boundary's
	// own wrapping classifies it as FORGE_INTERNAL_UNEXPECTED.
	return err
}

// parseUnknownCommandMessage extracts the command name and the
// parent name from Cobra's unknown-command message.
//
// The message's shape is:
//
//	unknown command "<name>" for "<parent>"
//
// The function returns the two quoted values. If the shape does
// not match, both are returned as the empty string; the caller
// then falls back to a wrapper without context.
//
// # Why the parser is a separate function
//
// The parser is the part of wrapCobraError that is most likely to
// need updating if Cobra's message shape changes. Isolating it
// means the update is localized to one function, and the test
// table can cover the shape variations independently of the
// wrapping logic.
func parseUnknownCommandMessage(msg string) (name, parent string) {
	// Find the first quoted segment.
	nameStart := strings.IndexByte(msg, '"')
	if nameStart < 0 {
		return "", ""
	}
	nameEnd := strings.IndexByte(msg[nameStart+1:], '"')
	if nameEnd < 0 {
		return "", ""
	}
	name = msg[nameStart+1 : nameStart+1+nameEnd]

	// Find the second quoted segment (the parent).
	rest := msg[nameStart+1+nameEnd+1:]
	parentStart := strings.IndexByte(rest, '"')
	if parentStart < 0 {
		return name, ""
	}
	parentEnd := strings.IndexByte(rest[parentStart+1:], '"')
	if parentEnd < 0 {
		return name, ""
	}
	parent = rest[parentStart+1 : parentStart+1+parentEnd]
	return name, parent
}
