// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the configuration precedence order (WBS 8.1.2).
// The order is a linear order over four sources: flag, env, file,
// default. Each test asserts one pairwise relation; a test for
// the transitive case pins the order's coherence.
//
// # How the tests work
//
// The tests use the Source type and its Beats method. The tests
// do not read a real config file, a real environment variable, or
// a real flag; the loader that does so is a future WBS item. The
// tests exercise the order itself, which is the contract.
package config

import (
	"reflect"
	"testing"
)

// =============================================================================
// The order
// =============================================================================

// TestPrecedence_SourceOrder verifies that the sources' integer
// values match the documented precedence order.
//
// The order, from lowest to highest priority, is:
//
//	SourceUnknown < SourceDefault < SourceFile < SourceEnv < SourceFlag
//
// SourceUnknown is not a real source; it is the zero value. A
// real source's integer value is greater than SourceUnknown's.
func TestPrecedence_SourceOrder(t *testing.T) {
	t.Parallel()

	if !(SourceUnknown < SourceDefault &&
		SourceDefault < SourceFile &&
		SourceFile < SourceEnv &&
		SourceEnv < SourceFlag) {
		t.Errorf("the sources' integer values do not match the "+
			"documented precedence order:\n"+
			"SourceUnknown=%d SourceDefault=%d SourceFile=%d "+
			"SourceEnv=%d SourceFlag=%d",
			SourceUnknown, SourceDefault, SourceFile, SourceEnv, SourceFlag)
	}
}

// TestPrecedence_FlagBeatsEnv verifies that a CLI flag has higher
// priority than an environment variable.
func TestPrecedence_FlagBeatsEnv(t *testing.T) {
	t.Parallel()

	if !SourceFlag.Beats(SourceEnv) {
		t.Errorf("SourceFlag does not beat SourceEnv; " +
			"the documented order is flag > env")
	}
}

// TestPrecedence_EnvBeatsFile verifies that an environment
// variable has higher priority than the config file.
func TestPrecedence_EnvBeatsFile(t *testing.T) {
	t.Parallel()

	if !SourceEnv.Beats(SourceFile) {
		t.Errorf("SourceEnv does not beat SourceFile; " +
			"the documented order is env > file")
	}
}

// TestPrecedence_FileBeatsDefault verifies that the config file
// has higher priority than the built-in default.
func TestPrecedence_FileBeatsDefault(t *testing.T) {
	t.Parallel()

	if !SourceFile.Beats(SourceDefault) {
		t.Errorf("SourceFile does not beat SourceDefault; " +
			"the documented order is file > default")
	}
}

// TestPrecedence_FlagBeatsDefault verifies the transitive case:
// a flag has higher priority than the default.
//
// The relation is implied by the pairwise relations
// (flag > env, env > file, file > default), but the test asserts
// it directly as a sanity check.
func TestPrecedence_FlagBeatsDefault(t *testing.T) {
	t.Parallel()

	if !SourceFlag.Beats(SourceDefault) {
		t.Errorf("SourceFlag does not beat SourceDefault; " +
			"the transitive relation should hold")
	}
}

// =============================================================================
// The unknown source
// =============================================================================

// TestPrecedence_SourceUnknownIsNotValid verifies that
// SourceUnknown has the lowest integer value and beats nothing.
func TestPrecedence_SourceUnknownIsNotValid(t *testing.T) {
	t.Parallel()

	for _, other := range AllSources() {
		if SourceUnknown.Beats(other) {
			t.Errorf("SourceUnknown beats %s; "+
				"SourceUnknown must beat nothing", other)
		}
	}
}

// =============================================================================
// The AllSources function
// =============================================================================

// TestPrecedence_AllSourcesReturnsFour verifies that
// AllSources returns exactly four sources.
func TestPrecedence_AllSourcesReturnsFour(t *testing.T) {
	t.Parallel()

	sources := AllSources()
	if len(sources) != 4 {
		t.Errorf("AllSources returned %d sources; want 4", len(sources))
	}
}

// TestPrecedence_AllSourcesAreDescending verifies that
// AllSources returns the sources in descending order of
// precedence (highest first).
func TestPrecedence_AllSourcesAreDescending(t *testing.T) {
	t.Parallel()

	sources := AllSources()
	for i := 1; i < len(sources); i++ {
		if !sources[i-1].Beats(sources[i]) {
			t.Errorf("AllSources is not descending: "+
				"sources[%d]=%s does not beat sources[%d]=%s",
				i-1, sources[i-1], i, sources[i])
		}
	}
}

// =============================================================================
// The pickByPrecedence function
// =============================================================================

// TestPrecedence_PickByPrecedence verifies that pickByPrecedence
// returns the higher-priority source of its two arguments.
func TestPrecedence_PickByPrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a    Source
		b    Source
		want Source
	}{
		{SourceFlag, SourceEnv, SourceFlag},
		{SourceEnv, SourceFlag, SourceFlag},
		{SourceEnv, SourceFile, SourceEnv},
		{SourceFile, SourceEnv, SourceEnv},
		{SourceFile, SourceDefault, SourceFile},
		{SourceDefault, SourceFile, SourceFile},
		{SourceDefault, SourceDefault, SourceDefault},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.a.String()+" vs "+tc.b.String(), func(t *testing.T) {
			t.Parallel()

			got := pickByPrecedence(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("pickByPrecedence(%s, %s) = %s; want %s",
					tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// =============================================================================
// The String method
// =============================================================================

// TestPrecedence_StringNames verifies that each source's String
// method returns the expected name.
func TestPrecedence_StringNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		src  Source
		want string
	}{
		{SourceUnknown, "unknown"},
		{SourceDefault, "default"},
		{SourceFile, "file"},
		{SourceEnv, "environment"},
		{SourceFlag, "flag"},
		{Source(999), "invalid"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			if got := tc.src.String(); got != tc.want {
				t.Errorf("Source(%d).String() = %q; want %q",
					tc.src, got, tc.want)
			}
		})
	}
}

// =============================================================================
// The Source type
// =============================================================================

// TestPrecedence_SourceIsComparable verifies that Source values
// can be compared with ==.
func TestPrecedence_SourceIsComparable(t *testing.T) {
	t.Parallel()

	if SourceFlag == SourceEnv {
		t.Error("SourceFlag and SourceEnv compared equal")
	}
	if SourceFlag != SourceFlag {
		t.Error("SourceFlag compared unequal to itself")
	}
}

// TestPrecedence_SourceTypeIsInt verifies that Source is an
// integer-typed type. The test pins the type's representation;
// a future change to a string-typed enum would fail the test and
// would require updating the precedence implementation.
func TestPrecedence_SourceTypeIsInt(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Source(0))
	if typ.Kind() != reflect.Int {
		t.Errorf("Source's underlying type is %s; want int", typ.Kind())
	}
}
