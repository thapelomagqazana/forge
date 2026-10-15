package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestSignAllWithCosign_NoCosign asserts the blocker is returned
// when cosign is not on PATH.
//
// The context is context.Background(): the driver passes it to
// exec.CommandContext as the parent, and the test's command runs to
// its natural completion before the test returns. The test does not
// start a goroutine that would observe the test's cancellation, so
// the t.Context() convenience (Go 1.24+) is not needed.
func TestSignAllWithCosign_NoCosign(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("cosign"); err == nil {
		t.Skip("cosign is on PATH; the no-cosign path cannot be tested")
	}
	var out, errOut bytes.Buffer
	blockers := signAllWithCosign(context.Background(), t.TempDir(), nil, &out, &errOut)
	if len(blockers) != 1 {
		t.Fatalf("got %d blockers, want 1", len(blockers))
	}
	if !strings.Contains(blockers[0].Reason, "cosign") {
		t.Errorf("blocker reason = %q, want it to name cosign", blockers[0].Reason)
	}
}

// TestSignDarwin_NoCodesign asserts the blocker is returned when
// codesign is not on PATH.
//
// The context is context.Background(), for the same reason as
// TestSignAllWithCosign_NoCosign.
func TestSignDarwin_NoCodesign(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("codesign"); err == nil {
		t.Skip("codesign is on PATH; the no-codesign path cannot be tested")
	}
	var out, errOut bytes.Buffer
	blockers := signDarwin(context.Background(), t.TempDir(), platformForTest("darwin", "amd64"), &out, &errOut)
	if len(blockers) != 1 {
		t.Fatalf("got %d blockers, want 1", len(blockers))
	}
	if !strings.Contains(blockers[0].Reason, "codesign") {
		t.Errorf("blocker reason = %q, want it to name codesign", blockers[0].Reason)
	}
}

// TestSignWindows_NoSigntool asserts the blocker is returned when
// signtool is not on PATH.
//
// The context is context.Background(), for the same reason as
// TestSignAllWithCosign_NoCosign.
func TestSignWindows_NoSigntool(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("signtool"); err == nil {
		t.Skip("signtool is on PATH; the no-signtool path cannot be tested")
	}
	var out, errOut bytes.Buffer
	blockers := signWindows(context.Background(), t.TempDir(), platformForTest("windows", "amd64"), &out, &errOut)
	if len(blockers) != 1 {
		t.Fatalf("got %d blockers, want 1", len(blockers))
	}
	if !strings.Contains(blockers[0].Reason, "signtool") {
		t.Errorf("blocker reason = %q, want it to name signtool", blockers[0].Reason)
	}
}

// TestSignWindows_NoSubject asserts the blocker names the missing
// environment variable when signtool is present but no subject is
// configured.
//
// The test unsets the two environment variables and restores them on
// return. The restore is what makes the test safe to run in parallel
// with other tests that read the variables; a test that left them
// unset would affect a sibling.
//
// The context is context.Background(), for the same reason as
// TestSignAllWithCosign_NoCosign.
func TestSignWindows_NoSubject(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("signtool"); err != nil {
		t.Skip("signtool is not on PATH; the no-subject path cannot be tested")
	}
	// Clear the two env vars.
	old1, had1 := os.LookupEnv("FORGE_AUTHENTICODE_SUBJECT")
	old2, had2 := os.LookupEnv("FORGE_AUTHENTICODE_SHA1")
	defer func() {
		if had1 {
			os.Setenv("FORGE_AUTHENTICODE_SUBJECT", old1)
		} else {
			os.Unsetenv("FORGE_AUTHENTICODE_SUBJECT")
		}
		if had2 {
			os.Setenv("FORGE_AUTHENTICODE_SHA1", old2)
		} else {
			os.Unsetenv("FORGE_AUTHENTICODE_SHA1")
		}
	}()
	os.Unsetenv("FORGE_AUTHENTICODE_SUBJECT")
	os.Unsetenv("FORGE_AUTHENTICODE_SHA1")

	var out, errOut bytes.Buffer
	blockers := signWindows(context.Background(), t.TempDir(), platformForTest("windows", "amd64"), &out, &errOut)
	if len(blockers) != 1 {
		t.Fatalf("got %d blockers, want 1", len(blockers))
	}
	if !strings.Contains(blockers[0].Reason, "FORGE_AUTHENTICODE") {
		t.Errorf("blocker reason = %q, want it to name the env var", blockers[0].Reason)
	}
}
