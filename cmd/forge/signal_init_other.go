//go:build !unix

// Package main — non-Unix signal initialization.
//
// This file provides the same function as signal_init_unix.go, as
// a no-op. It is compiled on every platform that is not Unix:
// Windows, plan9, js/wasm, and any other platform Go supports.
//
// # Why the no-op is correct
//
// The Unix implementation exists to install SIG_IGN for SIGPIPE,
// so that a write to a closed pipe returns EPIPE as an error. On
// non-Unix platforms:
//
//   - Windows does not have SIGPIPE. Its broken-pipe condition is
//     ERROR_BROKEN_PIPE, returned by WriteFile as an error. The
//     CLI's write sites classify the error via
//     streams.HandleWriteError, which on Windows matches
//     ERROR_BROKEN_PIPE. No signal disposition is needed.
//
//   - js/wasm does not have signals. The runtime's stdout is
//     usually a console or a buffer; the broken-pipe condition
//     does not arise.
//
//   - Other non-Unix platforms either have no signals or do not
//     deliver SIGPIPE in the way that would terminate the
//     process.
//
// The no-op is therefore the correct behavior on every platform
// the file is compiled on. A future platform that needs a
// different disposition adds its own tagged file; the compiler
// picks the right one.
package main

// installPlatformSignalHandlers installs the process's
// platform-specific signal dispositions.
//
// On non-Unix platforms there are no platform-specific
// dispositions to install, so the function is a no-op. See the
// file docstring for the rationale.
func installPlatformSignalHandlers() {
	// No-op on non-Unix platforms.
}
