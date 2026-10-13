//go:build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestBrokenPipe_CloseReaderHandle is the ATDD test for WBS 12.4.1
// on Windows. It starts forge, closes the read end of the stdout
// pipe, and asserts the process exits with code 141 without
// printing a stack trace.
//
// # Windows specifics
//
// Windows does not deliver SIGPIPE; the kernel returns
// ERROR_BROKEN_PIPE or ERROR_NO_DATA when a write to a closed
// pipe fails. The exit boundary maps the error to 141, the same
// code a Unix process exits with under SIGPIPE. The choice of
// 141 on Windows is a Forge convention, not a Windows one; it
// lets a cross-platform consumer use one exit-code check.
func TestBrokenPipe_CloseReaderHandle(t *testing.T) {
	t.Parallel()

	bin := buildForge(t)

	cmd := exec.Command(bin, "test-emit-lines", "100000")
	cmd.Env = append(os.Environ(), "FORGE_TEST=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Read one line, then close the read end.
	buf := make([]byte, 1024)
	if _, err := stdout.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	stdout.Close()

	err = cmd.Wait()
	exitCode := processExitCode(err)
	if exitCode != 141 {
		t.Fatalf("exit code = %d; want 141\nstderr:\n%s",
			exitCode, stderr.String())
	}
	if strings.Contains(stderr.String(), "panic:") {
		t.Fatalf("panic leaked to stderr:\n%s", stderr.String())
	}
}
