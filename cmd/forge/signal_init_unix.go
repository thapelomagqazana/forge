//go:build unix

// Package main — Unix signal initialization.
//
// This file installs the process's signal dispositions that are
// specific to Unix. It is compiled only on Unix platforms; the
// sibling file signal_init_other.go provides the same function
// as a no-op for every other platform.
//
// # Why a separate file
//
// The disposition for SIGPIPE is set by signal.Ignore. The
// syscall.SIGPIPE constant is declared on Unix platforms but not
// on every platform Go supports (for example, js/wasm). A
// platform-agnostic caller that referenced syscall.SIGPIPE would
// fail to compile on those platforms.
//
// The split — a Unix implementation and a no-op fallback —
// confines the platform-specific symbol to the file that is
// compiled on the platforms where it exists. The caller in
// main.go calls installPlatformSignalHandlers(), which is
// available on every platform.
package main

import (
	"os/signal"
	"syscall"
)

// installPlatformSignalHandlers installs the process's
// platform-specific signal dispositions.
//
// On Unix, the function installs SIG_IGN as the disposition for
// SIGPIPE. The effect is that a write to a pipe whose read end has
// closed returns EPIPE as an error, instead of the kernel
// terminating the process with the signal. The error is what the
// CLI's write sites classify via streams.HandleWriteError, and
// what the exit boundary maps to code 141.
//
// The function has no corresponding "stop": a signal disposition
// is process-global and persists until the process exits.
func installPlatformSignalHandlers() {
	signal.Ignore(syscall.SIGPIPE)
}
