// Package cli contains white-box tests for the CLI package.
//
// This file tests validateArgs, the pre-parse argument validator
// defined in validate.go. The validator rejects three malformed
// invocations that Cobra would otherwise accept silently; the tests
// below cover each rejection and its accepted neighbours.
//
// # Why the tests are unit tests
//
// validateArgs reads a []string and returns an error or nil. It does
// not depend on Cobra, on Dependencies, or on the constructed command
// tree. It is therefore testable in isolation, and the tests below
// are unit tests in the strict sense: they exercise one function with
// no surrounding machinery.
//
// # Relationship to the contract tests in root_test.go
//
// root_test.go contains end-to-end tests that invoke the CLI through
// the shared runCLI helper and assert on the observable result
// (stdout, stderr, exit code). Those tests are the contract tests
// for the CLI's behaviour as a user observes it.
//
// The tests below are the unit tests for the function that produces
// the observable result. They are complementary: a failure in the
// unit tests localizes the defect to validateArgs; a failure in the
// contract tests might indicate a defect in validateArgs, in
// executeWithOptions, or in Cobra's dispatch. The two layers together
// make the failure diagnosable.
//
// # The error shape
//
// The validator returns *forgeerr.Error values (WBS 10.5.1). The
// error's Error() method returns "<code>: <message>", so the
// substring assertions below match the message portion, not the
// code. A test that wants to assert on the code reads it via
// errors.As with a *forgeerr.Error target; the tests here assert
// the message's semantic content, which is what makes them
// independent of the exact wording the validator chooses.
package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// =============================================================================
// validateArgs — top-level dispatch
// =============================================================================

// TestValidateArgs_AcceptsWellFormedInvocations verifies that
// validateArgs returns nil for the common invocations that the
// validator must not reject.
//
// The cases below are the accepted invocations from the help and
// version contracts, plus the empty invocation and the no-op
// invocations that the root command handles itself.
func TestValidateArgs_AcceptsWellFormedInvocations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
	}{
		{"no-args", nil},
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
		{"--version", []string{"--version"}},
		{"-v", []string{"-v"}},
		{"help", []string{"help"}},
		{"help version", []string{"help", "version"}},
		{"help help", []string{"help", "help"}},
		{"help completion", []string{"help", "completion"}},
		{"version", []string{"version"}},
		{"version --help", []string{"version", "--help"}},
		{"unknown-command", []string{"unknown-command"}},
		{"--format json", []string{"--format", "json"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := validateArgs(tc.args); err != nil {
				t.Errorf("validateArgs(%v) returned error %v; want nil",
					tc.args, err)
			}
		})
	}
}

// TestValidateArgs_RejectsMalformedInvocations verifies that
// validateArgs returns an error for every malformed invocation the
// validator is responsible for rejecting.
//
// The cases below name the contract section each rejection
// implements.
//
// # The expected substrings
//
// The expected substrings are the semantic phrases from the
// validator's current messages, not the full messages. Each
// phrase is stable across flag names (long or short) and is
// short enough to be robust to small wording changes. The
// specific flag name is not asserted; the test's own `args`
// field already names it, and asserting it again would only
// duplicate the test's input.
//
// The phrases:
//
//	"flag takes no argument" — the rule for a no-argument flag
//	                           followed by a positional argument.
//	"flag requires a value"  — the rule for a flag that requires
//	                           a value but is not given one.
//	"unknown help topic"     — the rule for a help topic that is
//	                           not a known command name.
func TestValidateArgs_RejectsMalformedInvocations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		args       []string
		wantSubstr string
	}{
		// Help flag with arg (contract § 4.10, rule 6).
		{
			name:       "help-flag-with-arg",
			args:       []string{"--help", "version"},
			wantSubstr: "flag takes no argument",
		},
		{
			name:       "help-flag-short-with-arg",
			args:       []string{"-h", "version"},
			wantSubstr: "flag takes no argument",
		},

		// Version flag with arg (contract § 4.11, rule 6).
		{
			name:       "version-flag-with-arg",
			args:       []string{"--version", "extra"},
			wantSubstr: "flag takes no argument",
		},
		{
			name:       "version-flag-short-with-arg",
			args:       []string{"-v", "extra"},
			wantSubstr: "flag takes no argument",
		},

		// Help topic unknown (contract § 4.10, rule 8).
		{
			name:       "help-unknown-topic",
			args:       []string{"help", "no-such-command"},
			wantSubstr: "unknown help topic",
		},
		// Config flag with no value (contract § 4.12, rule 4).
		{
			name:       "config-flag-with-no-value",
			args:       []string{"--config"},
			wantSubstr: "flag requires a value",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateArgs(tc.args)
			if err == nil {
				t.Fatalf("validateArgs(%v) returned nil; want error",
					tc.args)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q",
					err.Error(), tc.wantSubstr)
			}
		})
	}
}

// =============================================================================
// validateHelpFlagWithArgs
// =============================================================================

// TestValidateHelpFlagWithArgs verifies the help-flag-with-args
// validator in isolation.
func TestValidateHelpFlagWithArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no-args", nil, false},
		{"--help", []string{"--help"}, false},
		{"-h", []string{"-h"}, false},
		{"--help version", []string{"--help", "version"}, true},
		{"-h version", []string{"-h", "version"}, true},
		{"--help --version", []string{"--help", "--version"}, false},
		{"version --help", []string{"version", "--help"}, false},
		{"--nonexistent", []string{"--nonexistent"}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateHelpFlagWithArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateHelpFlagWithArgs(%v) error = %v; "+
					"wantErr = %v", tc.args, err, tc.wantErr)
			}
		})
	}
}

// =============================================================================
// validateVersionFlagWithArgs
// =============================================================================

// TestValidateVersionFlagWithArgs verifies the
// version-flag-with-args validator in isolation.
func TestValidateVersionFlagWithArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no-args", nil, false},
		{"--version", []string{"--version"}, false},
		{"-v", []string{"-v"}, false},
		{"--version extra", []string{"--version", "extra"}, true},
		{"-v extra", []string{"-v", "extra"}, true},
		{"--version --help", []string{"--version", "--help"}, false},
		{"--help --version", []string{"--help", "--version"}, false},
		{"version", []string{"version"}, false},
		{"--version version", []string{"--version", "version"}, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateVersionFlagWithArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateVersionFlagWithArgs(%v) error = %v; "+
					"wantErr = %v", tc.args, err, tc.wantErr)
			}
		})
	}
}

// =============================================================================
// validateHelpTopic
// =============================================================================

// TestValidateHelpTopic verifies the help-topic validator in
// isolation.
func TestValidateHelpTopic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no-args", nil, false},
		{"help", []string{"help"}, false},
		{"help version", []string{"help", "version"}, false},
		{"help help", []string{"help", "help"}, false},
		{"help completion", []string{"help", "completion"}, false},
		{"help unknown", []string{"help", "no-such-command"}, true},
		{"version", []string{"version"}, false},
		{"help --help", []string{"help", "--help"}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateHelpTopic(tc.args)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateHelpTopic(%v) error = %v; "+
					"wantErr = %v", tc.args, err, tc.wantErr)
			}
		})
	}
}

// =============================================================================
// Edge cases and non-functional properties
// =============================================================================

// TestValidateArgs_IsDeterministic verifies that two calls with
// the same args return the same result. The function is pure, so
// this is expected; the test pins the property so that a future
// change that introduces state (for example, a cache) fails loudly.
func TestValidateArgs_IsDeterministic(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{"--help", "version"},
		{"--version", "extra"},
		{"help", "no-such-command"},
		{"version"},
		{"--help"},
	}

	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()

			firstErr := validateArgs(args)
			for i := 0; i < 20; i++ {
				gotErr := validateArgs(args)
				if (firstErr == nil) != (gotErr == nil) {
					t.Fatalf("run %d: error presence differs: "+
						"first = %v, got = %v",
						i, firstErr, gotErr)
				}
				if firstErr != nil && gotErr != nil &&
					firstErr.Error() != gotErr.Error() {
					t.Fatalf("run %d: error text differs: "+
						"first = %q, got = %q",
						i, firstErr, gotErr)
				}
			}
		})
	}
}

// TestValidateArgs_DoesNotMutateArgs verifies that the validator
// does not modify the slice it receives. The check reads the slice
// twice and compares.
func TestValidateArgs_DoesNotMutateArgs(t *testing.T) {
	t.Parallel()

	original := []string{"--help", "version"}
	snapshot := make([]string, len(original))
	copy(snapshot, original)

	_ = validateArgs(original)

	for i := range original {
		if original[i] != snapshot[i] {
			t.Errorf("args[%d] changed: before = %q, after = %q",
				i, snapshot[i], original[i])
		}
	}
}

func TestValidateConfigFlagWithArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no-args", nil, false},
		{"--config path", []string{"--config", "/tmp/forge.yaml"}, false},
		{"--config=path", []string{"--config=/tmp/forge.yaml"}, false},
		{"--config with no value", []string{"--config"}, true},
		{"--config followed by flag", []string{"--config", "--verbose"}, true},
		{"--config empty string", []string{"--config", ""}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateConfigFlagWithArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateConfigFlagWithArgs(%v) error = %v; "+
					"wantErr = %v", tc.args, err, tc.wantErr)
			}
		})
	}
}

// TestValidateArgs_ErrorsAreForgeErrors asserts the WBS 11.2.2
// property that the argument validator's errors are structured.
//
// The validator's public function returns an error. The audit
// claims that every error it returns is a *forgeerr.Error (so that
// the exit-code mapping can classify it). The test asserts the
// claim directly: it invokes the validator with a malformed
// invocation and inspects the returned error's type.
//
// # Why this is the right test shape
//
// The audit's other claims (no raw errors.New in domain code, no
// os.Exit outside main.go) are source-tree properties enforced by
// the shell scripts in scripts/deps/. The claim that the
// validator's errors are structured is a runtime property, and
// this test is the runtime half.
func TestValidateArgs_ErrorsAreForgeErrors(t *testing.T) {
	t.Parallel()

	// A malformed invocation: --help takes no argument.
	err := validateArgs([]string{"--help", "extra"})
	if err == nil {
		t.Fatal("validateArgs returned nil for a malformed invocation")
	}

	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("validateArgs returned %T, not *forgeerr.Error", err)
	}
	if fe.Category() != forgeerr.CategoryCLI {
		t.Errorf("validateArgs returned category %v; want %v",
			fe.Category(), forgeerr.CategoryCLI)
	}
}
