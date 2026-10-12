// Package cli — error-to-exit-code mapping.
//
// This file implements the mapping contract defined in
// docs/reference/error-mapping.md and decided in WBS 11.2.1. It
// exposes a single function, ExitCode, that the process boundary
// calls to convert a Go error into a process exit code.
//
// # The contract
//
// ExitCode is:
//
//   - Total. Every input — nil, a bare error, a wrapped chain, a
//     context sentinel — returns a documented exit code. There is
//     no "undefined code".
//
//   - Stable. Once a code is returned for a given error shape, it
//     is never reassigned. A change to the mapping requires an ADR.
//
//   - Compositional. The mapping unwraps the error chain with
//     errors.As, so a *forgeerr.Error buried under fmt.Errorf
//     wrapping is found and its code returned.
//
// # Relationship to execute.go
//
// The boundary's local helper exitCodeOf (in execute.go) reads the
// exit code from a *forgeerr.Error after the wrapping stage has
// guaranteed that every error reaching it is a *forgeerr.Error. It
// is a narrower function: it assumes the input is a *forgeerr.Error
// and does not handle the sentinel or fallback cases.
//
// ExitCode is the wider function: it is the public mapping contract
// that handles every shape an error can take. It is exported
// because cmd/forge/main.go and downstream consumers (IDE tooling,
// wrapper scripts) need the same mapping. exitCodeOf is unexported
// because it is an implementation detail of the boundary's error
// path.
//
// # The two functions' agreement
//
// For a *forgeerr.Error input, ExitCode and exitCodeOf return the
// same value: the error's category's exit code. The agreement is
// asserted by TestMapping_AgreesWithExitCodeOf. A future change to
// either function that broke the agreement would fail the test.
package cli

import (
	"context"
	"errors"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// ExitCode returns the process exit code for err.
//
// The mapping is defined in docs/reference/error-mapping.md. It is
// applied in order; the first rule that matches wins.
//
//	nil                               → ExitSuccess (0)
//	errors.Is(err, context.Canceled)  → ExitSIGINT (130)
//	errors.Is(err, context.DeadlineExceeded) → ExitInternal (70)
//	errors.As(err, &*forgeerr.Error)  → err.Category().ExitCode()
//	(fallback)                        → ExitUsage (64)
//
// # Why the order matters
//
// A *forgeerr.Error whose Cause is context.Canceled is a
// cancellation wrapped in a structured error. The mapping returns
// the structured error's category code, not ExitSIGINT, because the
// structured error's category is the more specific classification.
// A caller who wants the cancellation semantic should not wrap the
// sentinel inside a structured error.
//
// # Why the fallback is ExitUsage (64)
//
// WBS 11.2.1 § "Description / Scope" item (c) specifies 64 as the
// fallback. The rationale: an unrecognised error at the process
// boundary is, from the caller's perspective, a misuse of the tool.
// The invocation did not produce a result the CLI can classify, and
// the closest classification is a usage error. A future ADR may
// change the fallback to ExitInternal (70); until then, 64 is the
// contract.
//
// # Why context.DeadlineExceeded is ExitInternal (70)
//
// A deadline that fired is not a usage error (the user did not
// invoke the command incorrectly) and not a finding (the subject
// was not in a detectable state). It is an internal condition of
// the run: the tool had a deadline and the deadline elapsed. The
// internal-error range (70–78) is the correct classification.
//
// # What ExitCode does not do
//
// It does not modify the error. It does not log. It does not write
// to stdout or stderr. It returns an integer; the caller decides
// what to do with it. The process boundary (main.go) calls os.Exit
// with the result; other callers may inspect it.
//
// # Why ExitCode is exported
//
// The mapping is a public contract. cmd/forge/main.go uses it; a
// future IDE extension or wrapper script that wants to translate
// Forge errors into its own exit codes uses it; a test in a
// downstream module can assert the mapping without reaching into
// the package's unexported helpers. Exporting the function is the
// smallest surface that serves all three.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// Rule 2: context cancellation. The sentinel is checked with
	// errors.Is, so a wrapped context.Canceled (for example,
	// fmt.Errorf("...: %w", context.Canceled)) is detected.
	if errors.Is(err, context.Canceled) {
		return ExitSIGINT
	}

	// Rule 3: context deadline. Same reasoning as rule 2.
	if errors.Is(err, context.DeadlineExceeded) {
		return ExitInternal
	}

	// Rule 4: structured error. The check uses errors.As, so a
	// *forgeerr.Error buried under fmt.Errorf wrapping is found.
	var fe *forgeerr.Error
	if errors.As(err, &fe) {
		return fe.Category().ExitCode()
	}

	// Rule 5: fallback. The error is not a structured error and
	// does not match any of the sentinel checks. WBS 11.2.1 §
	// "Description / Scope" item (c) specifies 64.
	return ExitUsage
}
