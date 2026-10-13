package logger

import (
	"fmt"
	"os"
	"strings"
)

// Format identifies the rendering of a log record.
//
// The two formats are the human rendering (default) and the JSON
// rendering (one JSON object per line). The choice is a
// presentation concern: it does not affect which records are
// emitted, only how they are serialized.
//
// Format is distinct from the CLI's OutputFormat (which selects
// product output on stdout). The two are independent: a user may
// request JSON product output with human logs, or vice versa.
type Format int

const (
	// FormatHuman is the human-readable rendering. It is the
	// default. Its grammar is documented in
	// docs/reference/log-record-format.md §"Human format". The
	// human format is NOT stable: consumers must not parse it.
	FormatHuman Format = iota

	// FormatJSON is the machine-readable rendering: one JSON
	// object per line, conforming to
	// schemas/log-record.schema.json. Field names are stable
	// within a major version.
	FormatJSON
)

// String returns the canonical lowercase name of the format.
//
// The value matches the FORGE_LOG_FORMAT env var's accepted
// values. It is used in error messages and tests.
func (f Format) String() string {
	switch f {
	case FormatHuman:
		return "human"
	case FormatJSON:
		return "json"
	default:
		return fmt.Sprintf("Format(%d)", int(f))
	}
}

// ParseFormat converts a string into a Format.
//
// It accepts "human" and "json" case-insensitively and trims
// surrounding whitespace. An empty string returns FormatHuman with
// a nil error, matching the default. Any other value returns an
// error suitable for a usage error (exit 64).
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "human":
		return FormatHuman, nil
	case "json":
		return FormatJSON, nil
	default:
		return FormatHuman, fmt.Errorf(
			"unknown log format %q (want human or json)", s)
	}
}

// ReadEnvFormat returns the value of FORGE_LOG_FORMAT, or the
// empty string if the variable is unset.
//
// ReadEnvFormat exists so tests can call it directly rather than
// touching os.Setenv. Callers that need the typed Format should
// call ParseFormat on the result.
func ReadEnvFormat() string {
	return os.Getenv(EnvVarFormat)
}
