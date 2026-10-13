package logger

import (
	"fmt"
	"strings"
)

// Level is the severity of a log record.
//
// # Ordering
//
// Levels are ordered by severity: Debug < Info < Warn < Error. A
// configured level (the "threshold") enables that level and every
// higher (more severe) level. For example, a threshold of Warn
// enables Warn and Error, and suppresses Debug and Info.
//
// # The zero value is LevelInfo
//
// LevelInfo is the zero value of the Level type. The choice is
// deliberate: the default severity of a diagnostic record is
// "progress milestone", not "internal detail". A caller who
// constructs a logger with a zero Options gets a logger at
// LevelInfo, matching the documented default and matching the
// CLI's default (see docs/reference/log-levels.md §2).
//
// The severity ordering is preserved by assigning explicit
// integer values rather than using iota. Under iota, the first
// constant is always 0; since Debug is the least severe level and
// must sort below Info, the only way to make Info the zero value
// while keeping the ordering is to number the constants by hand:
//
//	Debug  = -1
//	Info   =  0
//	Warn   =  1
//	Error  =  2
//
// The offsets leave room for a future level between any two
// existing levels (for example, a "Trace" below Debug at -2, or a
// "Fatal" above Error at 3) without renumbering. Adding a level
// still requires an ADR; the spacing just makes the change
// additive.
//
// # Why not reorder with iota
//
// A naive "reorder so Info is first" breaks the ordering: with
// iota, Debug would become 1 and Info would become 0, and
// Enabled's `record >= threshold` comparison would treat Debug as
// more severe than Info. The explicit numbering avoids the
// inversion.
//
// # Semantics
//
// Each level has a defined purpose, documented on its constant.
// The semantics are normative and are published in
// docs/reference/log-levels.md. A contributor adding a log call
// must choose the level whose semantics match the event; "closest
// level" is not a valid rationale.
//
// # Operational vs. policy
//
// LevelWarn is an *operational* warning: a recoverable anomaly in
// how Forge ran. It is distinct from a *policy* warning produced
// by forge check or forge validate, which is product output on
// stdout. See docs/reference/log-levels.md §3.
//
// # Stability
//
// The set of levels is part of the public contract. Adding,
// removing, or renaming a level requires an ADR. The numeric
// values are an implementation detail: callers must compare
// levels by their named constants, not by their integer values.
type Level int

const (
	// LevelDebug is fine-grained internal state for
	// troubleshooting: values resolved, branches taken, caches
	// hit or missed, timing.
	//
	// Emitted only when the threshold is Debug. Off by default.
	//
	// Audience: developers diagnosing a bug. Not useful to a
	// normal user during a successful run.
	//
	// Value -1 (not 0) so that LevelInfo can be the zero value
	// while the ordering Debug < Info < Warn < Error holds.
	LevelDebug Level = -1

	// LevelInfo reports progress milestones and lifecycle events:
	// starting, finishing, loading, writing, connecting.
	//
	// Emitted when the threshold is Debug or Info. On by default.
	//
	// Audience: users who want to see what the command did,
	// without diagnostic noise. Suppressed by --quiet.
	//
	// Value 0: LevelInfo is the zero value of the Level type. A
	// zero Options or a zero Level field means "Info", matching
	// the package's documented default.
	LevelInfo Level = 0

	// LevelWarn reports a recoverable anomaly worth the user's
	// attention: a fallback was used, a deprecated form was
	// accepted, a value was coerced, a retry succeeded.
	//
	// Emitted when the threshold is Debug, Info, or Warn. On by
	// default.
	//
	// Distinct from a *policy* WARN finding produced by
	// forge check or forge validate, which is product output on
	// stdout. See docs/reference/log-levels.md §3.
	LevelWarn Level = 1

	// LevelError reports that Forge could not complete an
	// operation. The command failed or a sub-operation failed and
	// was surfaced.
	//
	// Always emitted, regardless of threshold. Cannot be
	// suppressed by any flag, env var, or config value: a logged
	// error is a real problem, and if it should be silent, the
	// code should not log it.
	//
	// Distinct from the ERROR *error envelope* on stdout under
	// --format json (ADR-021). The envelope is product output;
	// this is a diagnostic. Both may be emitted for the same
	// failure.
	LevelError Level = 2
)

// AllLevels returns every level in ascending severity.
//
// The returned slice is a fresh copy; callers may sort or filter
// it without affecting the package's level set. This is the
// canonical enumeration used by help text, --log-level
// validation, and tests.
func AllLevels() []Level {
	return []Level{LevelDebug, LevelInfo, LevelWarn, LevelError}
}

// String returns the canonical lowercase name of the level.
//
// The names match --log-level and FORGE_LOG_LEVEL values and are
// part of the public contract. Do not change them without an
// ADR.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return fmt.Sprintf("Level(%d)", int(l))
	}
}

// Describe returns a one-line human-readable description of the
// level.
//
// It is used by --help output and by error messages that need to
// explain what a level means. The descriptions mirror the
// semantics in docs/reference/log-levels.md and must stay in
// sync with them.
func (l Level) Describe() string {
	switch l {
	case LevelDebug:
		return "internal state for troubleshooting (off by default)"
	case LevelInfo:
		return "progress milestones (on by default)"
	case LevelWarn:
		return "operational warnings: recoverable anomalies (on by default)"
	case LevelError:
		return "failures: Forge could not complete an operation (always on)"
	default:
		return "unknown level"
	}
}

// ParseLevel converts a flag or env value into a Level.
//
// It accepts the four canonical names case-insensitively and the
// alias "warning" for "warn". Surrounding whitespace is trimmed.
// Empty input is treated as "unset" and returns LevelInfo with a
// nil error; callers that need to distinguish "unset" from
// "info" must check the input themselves (see ResolveLevel).
//
// Any other value returns an error suitable for a usage error
// (exit 64).
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf(
			"unknown log level %q (want debug, info, warn, error)", s)
	}
}

// Enabled reports whether a record at level record should be
// emitted when the configured threshold is the receiver.
//
// The rule is "that level and every higher (more severe) level":
//
//	Debug.Enabled(Debug) == true
//	Info.Enabled(Debug)  == false
//	Info.Enabled(Error)  == true
//
// The rule relies on the ordering Debug < Info < Warn < Error,
// which the explicit numeric values in the const block preserve.
// This is the only place the ordering rule is encoded; all
// filtering decisions go through Enabled.
func (threshold Level) Enabled(record Level) bool {
	return record >= threshold
}
