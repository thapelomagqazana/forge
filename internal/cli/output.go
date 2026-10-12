// Package cli — output format dispatch.
//
// This file implements the format-aware writers the CLI uses to
// route output to the correct stream. It is the single place that
// knows the --format flag's semantics.
//
// # The two formats
//
//   - text (default): human-readable output.
//   - json: machine-readable output. Successes write a JSON
//     result object to stdout; failures write the JSON error
//     envelope to stdout.
//
// # The contract per stream
//
// The dispatch is driven by a single decision: what does stdout
// carry for this invocation? The answer depends on the format and
// on whether the invocation succeeded.
//
//	                 text                     json
//	success          command output           {"schemaVersion":"1","exitCode":0,"result":{...}}
//	failure          empty                    {"schemaVersion":"1","exitCode":N,"error":{...}}
//
// stderr's content is format-independent: it carries the
// human-readable diagnostic on failure and is empty on success
// (except for warnings, which are not part of WBS 11.3.2's
// contract).
//
// # Why a dedicated type
//
// The dispatch logic could live in runWithOptions as a pair of
// if-branches. A dedicated type makes the contract visible: a
// reader who wants to know "what does Forge write where?" reads
// this file's docstring and the OutputFormat type's methods.
//
// The type is small (two methods) and pure; its methods have no
// side effects and no state.
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// OutputFormat identifies the output format selected by --format.
//
// The two values are "text" and "json". The zero value is the
// empty string, which the boundary treats as text (the flag's
// default). A future format (for example, "yaml") would add a
// third value; the two-value set is the contract for Phase 2.
type OutputFormat string

const (
	// FormatText is the human-readable output format. It is the
	// default when --format is not set.
	FormatText OutputFormat = "text"

	// FormatJSON is the machine-readable output format. It is
	// selected by --format json.
	FormatJSON OutputFormat = "json"
)

// writeFailure writes err to stdout in the selected format and
// returns the exit code the process should return.
//
// # What the function writes
//
//   - FormatText: nothing to stdout. The human-readable rendering
//     of the error is written to stderr by the caller (this
//     function does not write to stderr; see below).
//   - FormatJSON: the JSON error envelope to stdout. The
//     envelope's schema is published at
//     schemas/error-envelope.schema.json.
//
// The function returns the exit code the error maps to (the
// error's Category().ExitCode()). The caller uses the return
// value as the process's exit code.
//
// # Why the function does not write to stderr
//
// The stderr rendering is owned by runWithOptions's error path,
// which renders the human error block via forgeerr.Format. The
// rendering runs unconditionally on failure (subject to --quiet;
// see below). Separating the two writes keeps each function's
// responsibility narrow: this function's job is "what does
// stdout carry?"; runWithOptions's job is "what does stderr
// carry?".
//
// # The --quiet interaction
//
// The function does not consult --quiet. --quiet suppresses
// stderr diagnostics; the caller applies the suppression when it
// writes to stderr. The JSON envelope is not suppressed, because
// it is the command's result in JSON mode; the exit code and the
// envelope's contents are the machine consumer's data.
//
// # The error return
//
// The function returns an error only if the envelope's encoding
// fails (see encodeErrorEnvelope's docstring for when that can
// happen). A nil error means the write succeeded or the format
// is text (which writes nothing to stdout). The caller may ignore
// the error in production (the exit code is the primary signal);
// tests use the error to distinguish "the write failed" from "the
// write was a no-op".
func writeFailure(stdout io.Writer, format OutputFormat, err *forgeerr.Error) (int, error) {
	if format != FormatJSON {
		return err.Category().ExitCode(), nil
	}

	data, marshalErr := encodeErrorEnvelope(err)
	if marshalErr != nil {
		return err.Category().ExitCode(), marshalErr
	}

	if _, writeErr := stdout.Write(data); writeErr != nil {
		return err.Category().ExitCode(), fmt.Errorf("write error envelope: %w", writeErr)
	}
	if _, writeErr := stdout.Write([]byte{'\n'}); writeErr != nil {
		return err.Category().ExitCode(), fmt.Errorf("write error envelope newline: %w", writeErr)
	}

	return err.Category().ExitCode(), nil
}

// writeSuccessResult writes the command's result to stdout in the
// selected format.
//
// # What the function writes
//
//   - FormatText: the result string as-is (no trailing newline is
//     added; the caller controls the bytes).
//   - FormatJSON: the result wrapped in a success object of the
//     form {"schemaVersion":"1","exitCode":0,"result":<result>}.
//
// The success object's schema is not published in WBS 11.3.2; it
// is documented in docs/reference/output-streams.md and will be
// published when the first command produces a JSON result (Phase
// 10 WBS 5.0). The object's shape is deliberately minimal: a
// schema version, the exit code (always 0), and the result.
//
// # Why the function takes a []byte and not a string
//
// The command's result may be arbitrary bytes (a serialized JSON
// document, a YAML fragment, a binary blob encoded as base64).
// The function does not interpret the bytes; it wraps them.
func writeSuccessResult(stdout io.Writer, format OutputFormat, result []byte) error {
	if format != FormatJSON {
		if _, err := stdout.Write(result); err != nil {
			return fmt.Errorf("write result: %w", err)
		}
		return nil
	}

	// Wrap the result in a success object. The result is embedded
	// verbatim; the caller is responsible for ensuring it is
	// valid JSON (a future WBS will add a validator).
	data, marshalErr := encodeSuccessEnvelope(result)
	if marshalErr != nil {
		return marshalErr
	}
	if _, err := stdout.Write(data); err != nil {
		return fmt.Errorf("write success envelope: %w", err)
	}
	if _, err := stdout.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("write success envelope newline: %w", err)
	}
	return nil
}

// successEnvelope is the JSON document written to stdout when a
// command succeeds with --format json.
//
// The shape is deliberately minimal. A future WBS that adds
// structured results (for example, a check report with per-file
// findings) replaces the Result field's type from json.RawMessage
// to the structured type; the envelope's outer shape is unchanged.
type successEnvelope struct {
	SchemaVersion string          `json:"schemaVersion"`
	ExitCode      int             `json:"exitCode"`
	Result        json.RawMessage `json:"result"`
}

// encodeSuccessEnvelope renders the success object.
//
// # The result parameter
//
// The result is embedded verbatim. The caller is responsible for
// ensuring it is valid JSON; a future WBS will add a validator.
// The current call sites pass results that are already
// JSON-encoded.
func encodeSuccessEnvelope(result []byte) ([]byte, error) {
	env := successEnvelope{
		SchemaVersion: "1",
		ExitCode:      0,
		Result:        json.RawMessage(result),
	}

	data, marshalErr := json.Marshal(env)
	if marshalErr != nil {
		return nil, fmt.Errorf("encode success envelope: %w", marshalErr)
	}
	return data, nil
}
