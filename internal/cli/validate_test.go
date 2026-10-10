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
package cli

import (
	"strings"
	"testing"
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
			wantSubstr: "--help takes no arguments",
		},
		{
			name:       "help-flag-short-with-arg",
			args:       []string{"-h", "version"},
			wantSubstr: "-h takes no arguments",
		},

		// Version flag with arg (contract § 4.11, rule 6).
		{
			name:       "version-flag-with-arg",
			args:       []string{"--version", "extra"},
			wantSubstr: "--version takes no arguments",
		},
		{
			name:       "version-flag-short-with-arg",
			args:       []string{"-v", "extra"},
			wantSubstr: "-v takes no arguments",
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
			wantSubstr: "--config requires a value",
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
