package logger

import (
	"encoding/json"
	"io"
)

// jsonRecord is the wire shape of a Record in the JSON rendering.
//
// It exists because Record's Go field order does not match the
// documented field order. The schema and the documentation fix the
// order as ts, level, component, msg, correlation_id, attrs.
// encoding/json emits struct fields in declaration order, so
// jsonRecord's fields are declared in that order.
//
// A separate type (rather than a reordering of Record) keeps
// Record's Go API stable and its field names idiomatic. The
// conversion cost is one struct copy per record, which is
// negligible next to the JSON encoding.
type jsonRecord struct {
	Ts            string         `json:"ts"`
	Level         string         `json:"level"`
	Component     string         `json:"component"`
	Msg           string         `json:"msg"`
	CorrelationID string         `json:"correlation_id"`
	Attrs         map[string]any `json:"attrs,omitempty"`
}

// writeJSON renders r as one JSON object followed by a newline.
//
// The rendering conforms to schemas/log-record.schema.json. The
// encoding uses no indentation: one record per line is the
// contract.
//
// HTML escaping is disabled so that user data containing <, >, or
// & is preserved verbatim, matching the behavior of the success
// and error envelopes on stdout (see internal/output/writer.go).
//
// writeJSON is deliberately not part of the public API. It is
// called by StderrSink.
func writeJSON(w io.Writer, r Record) error {
	jr := jsonRecord{
		Ts:            r.Ts.UTC().Format("2006-01-02T15:04:05.000000000Z"),
		Level:         r.Level.String(),
		Component:     r.Component.String(),
		Msg:           r.Message,
		CorrelationID: r.CorrelationID,
		Attrs:         r.Attrs,
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(jr)
}
