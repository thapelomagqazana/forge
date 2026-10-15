package version

import (
	"errors"
	"fmt"
	"io"
)

// ErrUnknownFormat is returned when a caller requests a format the
// service does not support.
//
// # Why a sentinel error
//
// The handler needs to distinguish "unknown format" from "write
// failed". The two map to different exit codes: unknown format is
// a usage error (ExitUsage, 2); write failed is a general error
// (ExitError, 1). A sentinel error is the idiomatic way to make the
// distinction available to the caller with errors.Is.
//
// The handler wraps the sentinel in a message that names the
// offending value; the wrapping preserves the sentinel so that
// errors.Is(err, ErrUnknownFormat) remains true.
//
// # Why not a ForgeError
//
// The service package does not depend on the ForgeError type. That
// type lives in a future package (docs/architecture.md § 8.2) that
// the service does not import. The sentinel approach keeps the
// service package free of the ForgeError dependency and lets the
// handler map the sentinel to whatever exit code it needs.
var ErrUnknownFormat = errors.New("version: unknown format")

// FormatText and FormatJSON are the canonical names of the two
// supported output formats.
//
// # Why constants
//
// The handler and the service must agree on the names. A constant
// in the service package is the single source of truth; the
// handler's flag registration refers to FormatText as the default,
// and the CLI UX spec refers to both by their string values.
//
// # Why these are constants, not functions
//
// The constants hold the string values that appear on the command
// line ("text", "json"). The functions that produce output are
// named with verbs: WriteText, WriteJSON, FormatAs. The
// noun/verb split is the Go convention; it also prevents the name
// collision that would occur if a constant and a function shared
// an identifier.
//
// # Adding a new format
//
// Adding a format requires:
//
//  1. A new constant here.
//  2. A new case in FormatAs.
//  3. A new Write* function.
//  4. A new section in docs/cli-ux-spec.md.
//  5. New tests.
//
// The WBS does not currently authorise a third format. Adding one
// without a WBS item is scope creep and should be rejected at
// review.
const (
	FormatText = "text"
	FormatJSON = "json"
)

// jsonInfo is the wire representation of Info for the JSON format.
//
// # Why a separate type
//
// The Info struct has four fields (Version, Commit, BuildDate,
// Dirty) with exported names. The JSON schema requires snake_case
// field names and a boolean `dirty`. The struct tags on jsonInfo
// express the wire format; the Info struct stays focused on the
// domain model. A future change to the wire format (a new field, a
// renamed field) touches jsonInfo, not Info.
//
// # Why a value type
//
// jsonInfo has no methods, no identity, and no state. It is a
// snapshot for the encoder. Returning it by value avoids pointer
// aliasing and keeps the encoder's input immutable from the
// caller's perspective.
//
// # Field tags
//
//   - `version`    — required, string.
//   - `commit`     — required, string.
//   - `build_date` — required, string (RFC 3339 UTC).
//   - `dirty`      — required, boolean.
//
// All four fields are emitted even when their value is the empty
// string or false. The schema requires the keys to be present; a
// consumer that expects `commit` to exist should find it, even when
// the build did not inject a value. The alternative — omitempty on
// the string fields — would make the keys disappear for an
// uninstrumented build, which is a different JSON shape for the
// same schema.
//
// # Why no omitempty
//
// The consumer's parsing code is simpler if the keys are always
// present. A Go consumer that unmarshals into a struct with the
// same fields sees empty strings; a JavaScript consumer that reads
// `obj.commit` sees `""` rather than `undefined`. Both are
// preferable to a conditional read.
type jsonInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	Dirty     bool   `json:"dirty"`
}

// WriteText writes the human-readable rendering of info to w.
//
// # Naming
//
// This function was named Format in WBS 6.4.1 and is retained here
// as an alias. The name WriteText is a verb and follows the Go
// convention for functions that write to an io.Writer. The alias
// Format is kept for compatibility with the earlier WBS; new code
// should call WriteText.
//
// # The frozen format
//
// The output is six lines, as documented in Format's docstring
// (internal/app/version/format.go). The format is frozen by WBS
// 6.4.1.
func WriteText(w io.Writer, info Info) error {
	return Format(w, info)
}

// FormatAs dispatches to the formatter for the requested format.
//
// # The two formats
//
//   - FormatText — the human-readable format frozen by WBS 6.4.1.
//   - FormatJSON — the machine-readable format frozen by WBS 6.4.2.
//
// Any other value returns an error that wraps ErrUnknownFormat.
// Callers can test for it with errors.Is(err, ErrUnknownFormat).
//
// # Why dispatch lives here
//
// The handler parses the --format flag and passes its value to
// this function. The service owns the set of formats it supports;
// adding a new format (for example, "json-pretty") is a change to
// this function and to the set of constants above. The handler is
// unchanged.
//
// This is the handler / service boundary from WBS 4.3.1: the
// handler does not know which formats exist; it only knows how to
// ask for one.
func FormatAs(w io.Writer, info Info, format string) error {
	switch format {
	case FormatText:
		return WriteText(w, info)
	case FormatJSON:
		return WriteJSON(w, info)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownFormat, format)
	}
}
