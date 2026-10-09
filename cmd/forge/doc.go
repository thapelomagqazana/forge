// Package main implements the Forge command-line interface.
//
// # What this package is
//
// This package contains exactly one file of consequence: main.go. That
// file is the process entry point for the `forge` binary. It delegates
// immediately to internal/cli.Execute() and returns its result as the
// process exit code.
//
// # What this package is not
//
// This package does not implement the CLI. It does not parse flags,
// read environment variables, print to any stream, format errors, or
// interact with the filesystem. All of that lives in internal/cli and
// its dependencies.
//
// # Why this package exists at all
//
// Go requires a `main` package for an executable. There is no way to
// build a runnable binary without one. This package is the smallest
// possible main package that satisfies that requirement while keeping
// the actual CLI testable.
//
// # Testing
//
// This package is not tested directly. Its sole function, main, is
// untestable in isolation by design: invoking it means replacing the
// current process, which makes assertions impossible.
//
// Instead, the behaviour of the forge binary is tested by calling
// internal/cli.Execute() with injectable inputs, from tests inside
// internal/cli. See internal/cli/execute_test.go and
// internal/cli/root_test.go.
//
// # Adding code here
//
// Please don't. If you have logic to add, add it to internal/cli or a
// deeper package, where it can be tested. The package documentation in
// main.go enumerates the forbidden categories of logic explicitly.
package main
