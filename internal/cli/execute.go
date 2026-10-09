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
// struct is small (five fields), and passing it by value avoids the
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
	// 13.0 has a single place to read the root from.
	rootPath string
}

// defaultOptions returns an options value bound to the current
// process.
//
// It is the only place in the package that reads os.Args, os.Stdin,
// os.Stdout, os.Stderr, os.Getenv, or os.Getwd directly. Every other
// function receives these values through the options struct.
//
// This is the pattern that makes the package testable in-process.
// Tests construct options directly, bypassing this function entirely.
func defaultOptions() options {
	// os.Getwd may fail in rare circumstances (for example, if the
	// current directory has been deleted). When it does, the empty
	// string is used, and commands treat the empty root as "use the
	// process's default". This matches the behaviour of the
	// underlying os package, which also falls back to relative
	// paths when Getwd fails.
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
func Execute() int {
	return executeWithOptions(defaultOptions())
}

// executeWithOptions runs the CLI with the given options and returns
// the process exit code.
//
// It is unexported because it is an implementation detail. Tests
// within the package call it directly. Downstream packages must not.
//
// # What the function does
//
//  1. Constructs the root command, passing the injectable
//     environment to it.
//  2. Binds the injectable inputs and outputs to the command tree.
//  3. Executes the command tree.
//  4. Formats any returned error.
//  5. Writes the formatted error to the injected stderr.
//  6. Maps the error to an exit code via exitCodeFromError.
//
// # What the function does not do
//
//   - It does not read os.Args, os.Stdin, os.Stdout, os.Stderr, or
//     os.Getenv. Every input is read from the options value.
//   - It does not perform I/O of its own beyond writing the
//     formatted error to opts.stderr. The command tree performs the
//     rest.
//   - It does not interpret exit codes beyond calling
//     exitCodeFromError. The mapping is defined in exitcodes.go.
//
// # The error path
//
// When the command tree returns a non-nil error, the function:
//
//   - Formats the error via formatError.
//   - Writes the formatted string to opts.stderr, followed by a
//     newline.
//   - Returns the exit code for the error's category.
//
// When the command tree returns a nil error, the function returns
// ExitSuccess without writing anything.
//
// # The output path
//
// Successful output is written by the command tree itself, to
// opts.stdout. The function does not write to opts.stdout; it only
// wires it to the command tree.
func executeWithOptions(opts options) int {
	root := newRootCmd(opts)
	root.SetArgs(opts.args)
	root.SetIn(opts.stdin)
	root.SetOut(opts.stdout)
	root.SetErr(opts.stderr)

	err := root.Execute()
	if err == nil {
		return ExitSuccess
	}

	// The command tree returned an error. Format it and write it to
	// the injected stderr. The format is defined by formatError;
	// this function does not add anything to it.
	fmt.Fprintln(opts.stderr, formatError(err))

	return exitCodeFromError(err)
}

// formatError renders an error as a user-facing string.
//
// The function is deliberately minimal in WBS 4.2.1. It returns the
// error's own message, without prefix or decoration. This is the
// behaviour the tests assert: an error's message is the observable
// output on failure.
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
// # Why not fmt.Sprintln here
//
// The caller writes the formatted string to stderr and adds a
// newline. Adding the newline inside this function would make the
// function's output suitable only for writing to a stream; a future
// caller that wants to compose the string into a larger message
// would have to strip the newline. Keeping the newline outside the
// function preserves flexibility.
func formatError(err error) string {
	if err == nil {
		// This branch is unreachable given how formatError is
		// called, but the function is defensive: a nil error
		// formats as an empty string rather than panicking.
		return ""
	}
	return err.Error()
}
