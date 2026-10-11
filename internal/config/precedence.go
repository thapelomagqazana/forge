// Package config defines the CLI configuration for Forge.
//
// This file holds the configuration precedence contract (WBS
// 8.1.2). The contract names the four sources of configuration
// values and the order in which they override one another.
//
// # The precedence order
//
// Highest priority first:
//
//  1. CLI flags                --log-level=debug
//  2. Environment variables    FORGE_LOG_LEVEL=debug
//  3. Config file              log_level: debug
//  4. Built-in defaults        log_level: info
//
// A higher-numbered source does not override a lower-numbered
// one. A flag that is explicitly set wins over an environment
// variable; an environment variable wins over the config file; a
// config file wins over the default.
//
// # Why the order is what it is
//
// The order matches the user's mental model and the operating
// system's conventions:
//
//   - A flag is the most explicit statement. The user typed it
//     for this invocation; it wins.
//
//   - An environment variable is a per-session or per-process
//     statement. It is more specific than a config file, which
//     is per-user, and less specific than a flag, which is
//     per-invocation.
//
//   - A config file is a per-user statement. It is the user's
//     persistent preference; it is less specific than a
//     per-session environment variable.
//
//   - A built-in default is the fallback when no source provides
//     a value. It is the least specific.
//
// # Where the contract is enforced
//
// The order is documented in docs/cli-ux-spec.md "Configuration
// Precedence" and enforced by the tests in precedence_test.go.
// The tests assert the order's pairwise relations: flag beats
// env, env beats file, file beats default. The transitivity of
// the order follows; a test pins the transitive case as a
// sanity check.
//
// The loader that applies the order is a future WBS item
// (8.4.1). This file defines the order; the loader reads the
// sources and produces a Config value whose fields are the
// winners.
//
// # No package-level mutable state
//
// This file declares a type and functions. It declares no
// package-level variables.
package config

// Source identifies where a configuration value came from.
//
// # Why the type exists
//
// The precedence rule is a linear order over four sources. The
// type names the four sources and provides the comparison that
// the loader uses to decide which source wins. A future
// contributor who reads a Config value can ask "which source
// provided this field?" and get an answer.
//
// # The zero value
//
// The zero value is SourceUnknown. It is not a valid source; a
// caller that reads a zero-valued Source has a bug. The type's
// first constant is SourceUnknown so that the zero value is
// distinguishable from any real source.
//
// # Why an integer-typed enum
//
// The type is an integer. Go's convention for an enum that
// supports ordering is an integer type with named constants.
// The alternative — a string-typed enum — would require a
// separate ordering function; the integer type has the ordering
// built in via the comparison operator.
//
// # Why the constants are exported
//
// The type is exported so that future callers (the loader, a
// diagnostic command, a test) can name a source. The constants
// are exported so that the same callers can compare against a
// named source without knowing its integer value.
type Source int

const (
	// SourceUnknown is the zero value. It is not a valid source.
	//
	// A caller that reads a zero-valued Source has a bug: the
	// value was never set. The constant is first so that the
	// zero value is distinguishable from any real source.
	SourceUnknown Source = iota

	// SourceDefault is the built-in default. It is the lowest
	// priority source.
	SourceDefault

	// SourceFile is the config file. It overrides the default.
	SourceFile

	// SourceEnv is an environment variable. It overrides the
	// config file.
	SourceEnv

	// SourceFlag is a CLI flag. It is the highest priority
	// source; it overrides every other source.
	SourceFlag
)

// String returns a human-readable name for the source.
//
// # Why a String method
//
// The method makes the source's name available to a caller that
// wants to display it (for example, a diagnostic command that
// shows "log_level=debug (source: flag)"). The method is also
// used by the tests' failure messages.
//
// # The unknown source
//
// SourceUnknown's String returns "unknown". A caller that
// displays an unknown source sees the string; the caller can
// decide whether to treat it as an error.
func (s Source) String() string {
	switch s {
	case SourceUnknown:
		return "unknown"
	case SourceDefault:
		return "default"
	case SourceFile:
		return "file"
	case SourceEnv:
		return "environment"
	case SourceFlag:
		return "flag"
	default:
		return "invalid"
	}
}

// Beats reports whether s has higher priority than other.
//
// # What the function does
//
// The function compares the two sources' integer values. A
// higher integer is a higher priority. The function returns
// true if s's integer is greater than other's.
//
// # The unknown source
//
// SourceUnknown has the lowest integer value (zero). Any real
// source beats SourceUnknown; SourceUnknown beats nothing. A
// caller that compares a real source to SourceUnknown gets the
// expected result. A caller that compares SourceUnknown to
// SourceUnknown gets false, which is the correct answer (a
// source does not beat itself).
//
// # Why the function is a method
//
// The comparison is a property of the source type. A method
// reads naturally at the call site: `flagSource.Beats(envSource)`.
// A free function (`Beats(flagSource, envSource)`) would work but
// would be less legible.
func (s Source) Beats(other Source) bool {
	return s > other
}

// pickByPrecedence returns the higher-priority of two sources.
//
// # Why the function exists
//
// A future loader that reads two sources for the same field
// calls this function to decide which source wins. The function
// is the mechanical implementation of the precedence rule; the
// rule itself is documented in this file's package docstring.
//
// # Ties
//
// The function returns the first argument if the two sources
// are equal. A tie is impossible in practice: the two sources
// are the ones a value came from, and a value comes from exactly
// one source. The function's tie behaviour is defined for
// completeness; a caller that passes equal sources gets a
// deterministic result.
func pickByPrecedence(a, b Source) Source {
	if a.Beats(b) {
		return a
	}
	return b
}

// AllSources returns the four real sources in descending order of
// precedence (highest first).
//
// # Why the function exists
//
// A test that wants to iterate the sources reads them from this
// function. The order matches the precedence rule; a caller that
// wants to know "what are the sources?" calls the function and
// iterates.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns a fresh slice on each call
// has no package-level state.
func AllSources() []Source {
	return []Source{
		SourceFlag,
		SourceEnv,
		SourceFile,
		SourceDefault,
	}
}
