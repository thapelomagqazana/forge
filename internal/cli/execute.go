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
// variables — flows through this struct.
//
// Tests construct an options value with synthetic inputs, call
// executeWithOptions, and assert on the captured output.
type options struct {
	// args are the command-line arguments, excluding the program
	// name.
	args []string

	// stdin is the reader from which the CLI may read interactive
	// input.
	stdin io.Reader

	// stdout is the writer to which the CLI writes successful
	// output.
	stdout io.Writer

	// stderr is the writer to which the CLI writes diagnostics and
	// errors.
	stderr io.Writer

	// env reads environment variables by name.
	env func(string) string
}

// defaultOptions returns an options value bound to the current
// process.
//
// It is the only place in the package that reads os.Args, os.Stdin,
// os.Stdout, os.Stderr, or os.Getenv directly.
func defaultOptions() options {
	return options{
		args:   os.Args[1:],
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		env:    os.Getenv,
	}
}

// Execute runs the Forge CLI and returns a process exit code.
//
// It is the only exported symbol of this package. It exists so that
// cmd/forge/main.go can delegate the entire process lifecycle to a
// single, testable function.
//
// The signature is frozen:
//
//	func Execute() int
//
// Execute is a thin wrapper over executeWithOptions.
func Execute() int {
	return executeWithOptions(defaultOptions())
}

// executeWithOptions runs the CLI with the given options and returns
// the process exit code.
//
// It is unexported because it is an implementation detail. Tests
// within the package call it directly.
//
// The function:
//
//  1. Constructs the root command tree.
//  2. Binds the injectable environment to the command tree.
//  3. Executes the tree.
//  4. Maps any returned error to an exit code.
//  5. Prints the error to the injected stderr, if any.
//
// The exit code is derived by exitCodeFromError, which is the single
// point in the package where errors are classified.
func executeWithOptions(opts options) int {
	root := newRootCmd()
	root.SetArgs(opts.args)
	root.SetIn(opts.stdin)
	root.SetOut(opts.stdout)
	root.SetErr(opts.stderr)

	err := root.Execute()
	if err != nil {
		// Print the error to the injected stderr. Format: the
		// error's own message.
		fmt.Fprintln(opts.stderr, err)
	}

	return exitCodeFromError(err)
}
