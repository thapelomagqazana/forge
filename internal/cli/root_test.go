// Package cli_test contains tests for the CLI package.
//
// In WBS 2.4.1, these tests verify the wiring between the module and
// Cobra. The tests are intentionally minimal: the goal is to prove the
// dependency is correctly integrated, not to test the full command tree
// (which does not yet exist).
//
// Full command tests arrive in WBS 4.x through WBS 9.x.
package cli_test

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/thapelomagqazana/forge/internal/cli"
)

// TestExecute_IsExported verifies that Execute has the required public
// signature. This is a compile-time test: if Execute's signature changes,
// the test fails to compile.
//
// The assertion is expressed as a typed assignment rather than a nil
// check. Function values are not comparable to nil in Go, so
// `cli.Execute == nil` is a compile-time error. The typed assignment
// proves the signature without needing a runtime check.
func TestExecute_IsExported(t *testing.T) {
	t.Parallel()

	// Assigning to a typed variable forces the compiler to verify the
	// signature. If Execute returned something other than int, or took
	// any parameters, this line would not compile.
	var _ func() int = cli.Execute
}

// TestExecute_NoPanic verifies that invoking Execute with an empty
// environment does not panic. The command tree is minimal in this WBS,
// so Execute should return cleanly.
//
// Note: this test calls Execute with whatever os.Args the test binary
// was invoked with. In `go test`, os.Args[0] is the test binary itself,
// and Cobra treats that as an unknown command, causing Execute to
// return 1. The test asserts the *absence of a panic*, not the exit
// code, which is what WBS 2.4.1 can meaningfully prove.
func TestExecute_NoPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Execute panicked: %v", r)
		}
	}()

	_ = cli.Execute()
}

// TestCobraImported is a compile-time check that Cobra is importable
// from the test package. It proves the module is correctly resolved
// and that Cobra's API is accessible.
func TestCobraImported(t *testing.T) {
	t.Parallel()

	// Construct a Cobra command directly. This does not use any Forge
	// code; it simply proves Cobra is importable and functional.
	cmd := &cobra.Command{
		Use:   "test",
		Short: "test command",
	}

	if cmd.Name() != "test" {
		t.Fatalf("expected command name 'test', got %q", cmd.Name())
	}
}