package output

import (
	"fmt"
	"strings"
)

// Format identifies the rendering format for output.
//
// The zero value is FormatText, matching the CLI default and POSIX
// convention for human-first tools.
type Format int

const (
	// FormatText renders human-readable output. Stable but not intended
	// for machine parsing.
	FormatText Format = iota

	// FormatJSON renders a single JSON envelope to stdout. Guaranteed
	// to be the only bytes on stdout when active.
	FormatJSON
)

// String returns the canonical lowercase name of the format.
//
// It implements fmt.Stringer and is used in error messages, help text,
// and flag parsing.
func (f Format) String() string {
	switch f {
	case FormatText:
		return "text"
	case FormatJSON:
		return "json"
	default:
		return fmt.Sprintf("Format(%d)", int(f))
	}
}

// ParseFormat converts a flag string into a Format.
//
// It accepts "text" and "json" case-insensitively and returns an error
// for any other value. The error is safe to surface to the user; it does
// not leak internal type names.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "text":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	default:
		return FormatText, fmt.Errorf("unknown format %q (want text or json)", s)
	}
}

// IsJSON reports whether the format is JSON.
//
// Convenience for hot-path checks; equivalent to f == FormatJSON.
func (f Format) IsJSON() bool { return f == FormatJSON }
