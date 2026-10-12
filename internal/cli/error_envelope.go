// Package cli — JSON error envelope.
//
// This file implements the JSON error envelope defined in
// docs/reference/output-streams.md and decided in WBS 11.3.2. The
// envelope is what a failing command writes to stdout when
// --format json is set.
//
// # The envelope's shape
//
// The envelope is a single JSON object:
//
//	{
//	  "schemaVersion": "1",
//	  "exitCode": 10,
//	  "error": {
//	    "code": "FORGE_CONFIG_MALFORMED",
//	    "message": "invalid YAML in config.yaml",
//	    "category": "CONFIG",
//	    "context": {"path": "config.yaml"},
//	    "remediation": "Fix the YAML syntax error."
//	  }
//	}
//
// The schema is published at schemas/error-envelope.schema.json.
// The envelope's fields are additive: a future release may add
// fields but never removes or renames them. The schemaVersion
// field is a stability marker for the envelope's shape.
//
// # Why the envelope is a distinct type
//
// The envelope could be built inline as a map[string]any at the
// call site. A dedicated type makes the shape visible in one
// place, makes the JSON marshaling deterministic (Go's encoding
// preserves the struct's field order, whereas a map's iteration
// order is randomized), and gives the envelope a natural home
// for its docstring.
//
// # Why the encoding is explicit
//
// The envelope is encoded with encoding/json's Marshal, not with
// a hand-written encoder. The standard library's encoder produces
// the canonical JSON representation (single-line object, sorted
// keys within maps, RFC 8259-compliant escaping). A hand-written
// encoder would be a maintenance burden and a source of drift.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// errorEnvelope is the JSON document written to stdout when a
// command fails with --format json.
//
// # Field order
//
// The struct's field order determines the JSON object's key order.
// Go's encoding/json marshals struct fields in declaration order,
// so the emitted object has the keys in the order:
//
//	schemaVersion, exitCode, error
//
// That order is the envelope's canonical form; a golden file in
// error_envelope_test.go asserts it.
//
// # Field semantics
//
// The three top-level fields are documented on their struct tags;
// see the package docstring for the envelope's contract.
type errorEnvelope struct {
	// SchemaVersion is the envelope's stability marker. Always
	// "1" for the current shape. A future breaking change to the
	// envelope increments this value.
	SchemaVersion string `json:"schemaVersion"`

	// ExitCode is the process exit code. It mirrors the value the
	// process returns to the operating system, so a consumer can
	// read the code from the envelope without reading the
	// process's exit status.
	ExitCode int `json:"exitCode"`

	// Error is the error's structured data.
	Error errorEnvelopeError `json:"error"`
}

// errorEnvelopeError is the envelope's inner error object.
//
// # Field order
//
// The fields are declared in the order the envelope's canonical
// form emits them: code, message, category, context, remediation.
// The context and remediation fields are omitted when empty
// (the omitempty tag).
type errorEnvelopeError struct {
	// Code is the registered error code, e.g. FORGE_CONFIG_MALFORMED.
	Code string `json:"code"`

	// Message is the error's short user-facing description.
	Message string `json:"message"`

	// Category is the error's category, e.g. "CONFIG".
	Category string `json:"category"`

	// Context carries error-specific key-value pairs. Omitted when
	// empty so that the envelope does not carry an empty object.
	Context map[string]string `json:"context,omitempty"`

	// Remediation is the error's user-facing guidance. Omitted
	// when empty.
	Remediation string `json:"remediation,omitempty"`
}

// encodeErrorEnvelope renders err as the JSON error envelope.
//
// The function's input is a *forgeerr.Error; the caller's contract
// is that err is the fully-wrapped, fully-classified error the
// boundary produced. The function does not unwrap err, does not
// consult the error's Cause, and does not apply any redaction
// beyond what forgeerr.Error already applied.
//
// # Why the function takes a *forgeerr.Error and not an error
//
// The envelope is built from the error's observable API
// (Category, Code, Message, Context, Remediation). The API is
// defined on *forgeerr.Error, not on the error interface. The
// caller (runWithOptions) has already ensured err is a
// *forgeerr.Error by the time it calls this function; the
// signature reflects that invariant.
//
// # The message field
//
// The envelope's message field carries err.Message(), the same
// string the human rendering places after "Error:". A consumer
// that parses the envelope and a consumer that reads the human
// rendering see the same message.
//
// # The context field
//
// The envelope's context field carries err.Context() verbatim.
// The map's iteration order is not significant; encoding/json
// sorts the keys when marshaling, so the output is
// deterministic.
//
// # The remediation field
//
// The envelope's remediation field carries err.Remediation(). If
// the error has no remediation, the field is omitted from the
// envelope (the omitempty tag). A consumer that expects a
// remediation for a specific code finds it in the code's
// documentation; the field's presence is not part of the
// envelope's contract.
//
// # The error return
//
// The function returns an error if json.Marshal fails. Marshal
// fails only for values it cannot represent (channels, complex
// numbers, cyclic data). The envelope's fields are all strings,
// integers, and a map of strings, so Marshal cannot fail in
// practice. The error return is preserved so that a future change
// to the envelope's shape that introduced an unrepresentable
// field would surface the failure rather than panicking.
func encodeErrorEnvelope(err *forgeerr.Error) ([]byte, error) {
	env := errorEnvelope{
		SchemaVersion: "1",
		ExitCode:      err.Category().ExitCode(),
		Error: errorEnvelopeError{
			Code:        err.Code(),
			Message:     err.Message(),
			Category:    string(err.Category()),
			Context:     err.Context(),
			Remediation: err.Remediation(),
		},
	}

	data, marshalErr := json.Marshal(env)
	if marshalErr != nil {
		// Unreachable in practice: the envelope's fields are all
		// JSON-representable. The branch exists so that a future
		// field with a non-representable type produces a clean
		// error rather than a panic or a partial write.
		return nil, fmt.Errorf("encode error envelope: %w", marshalErr)
	}
	return data, nil
}
