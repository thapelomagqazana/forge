package logger

import (
	"fmt"
	"time"
)

// ReservedAttrKeys lists the record's top-level field names.
//
// These names are not valid attr keys: setting one as an attr would
// shadow the envelope field of the same name in the JSON rendering,
// or collide with the human rendering's grammar. WithAttr panics on
// a reserved key so the collision is caught at the call site, not
// at the consumer.
//
// The set is derived from the schema in
// schemas/log-record.schema.json; it must stay in sync with the
// Record struct's JSON tags. record_test.go asserts the sync.
var ReservedAttrKeys = map[string]bool{
	"ts":             true,
	"level":          true,
	"component":      true,
	"msg":            true,
	"correlation_id": true,
}

// Record is a single diagnostic entry.
//
// A Record carries everything a consumer needs to interpret a log
// line without reading the source that emitted it. The fields are
// stable within a major version; see
// docs/reference/log-record-format.md §"Stability promise".
//
// # Field semantics
//
//   - Ts is the record's creation time in UTC. The JSON rendering
//     formats it as RFC3339 with nanosecond precision. The human
//     rendering does not include it (the timestamp is added by the
//     terminal or by the collector).
//
//   - Level is the severity. See Level for the semantics of each
//     value.
//
//   - Component identifies the subsystem that emitted the record.
//     See Component for the closed enumeration.
//
//   - Message is the human-readable summary. Message text is not
//     stable; consumers classify by Component and Level.
//
//   - CorrelationID identifies the invocation that produced the
//     record. Every record of one invocation shares the same value;
//     see ZeroCorrelationID for the sentinel used outside
//     invocations.
//
//   - Attrs is the structured key-value data. Keys are strings;
//     values are JSON-serializable. Attrs is omitted from the JSON
//     rendering when empty, matching the schema's `omitempty`
//     convention.
//
// # Zero value
//
// The zero Record is valid but semantically empty: zero time, zero
// level (Debug), empty component (Invalid), empty message, the zero
// correlation ID, and no attrs. A Sink should accept the zero
// Record without panicking; the rendering it produces is
// implementation-defined.
type Record struct {
	// Ts is the record's creation time. Always UTC.
	Ts time.Time `json:"ts"`

	// Level is the severity. See Level.
	Level Level `json:"level"`

	// Component identifies the emitting subsystem.
	Component Component `json:"component"`

	// Message is the human-readable summary.
	Message string `json:"msg"`

	// CorrelationID identifies the invocation. Never empty in
	// practice; ZeroCorrelationID when not part of an invocation.
	CorrelationID string `json:"correlation_id"`

	// Attrs is the structured payload. Omitted from JSON when nil
	// or empty.
	Attrs map[string]any `json:"attrs,omitempty"`
}

// WithAttr returns a copy of r with the given key-value pair added
// to its Attrs map.
//
// The copy is shallow: the Attrs map is cloned, but the values
// inside it are not. A caller that mutates a value's underlying
// data after calling WithAttr may see the mutation reflected in
// both records. This matches the semantics of slog's Record.
//
// WithAttr panics if key is one of ReservedAttrKeys. The panic is a
// programming error: a reserved key would collide with the
// envelope's own field of the same name, and silently dropping it
// would hide the collision.
func (r Record) WithAttr(key string, value any) Record {
	if ReservedAttrKeys[key] {
		panic(fmt.Sprintf(
			"logger: attr key %q is reserved (it collides with the "+
				"record's own field); see "+
				"docs/reference/log-record-format.md", key))
	}
	if r.Attrs == nil {
		r.Attrs = make(map[string]any, 1)
	} else {
		// Clone so the caller's record is not mutated.
		clone := make(map[string]any, len(r.Attrs)+1)
		for k, v := range r.Attrs {
			clone[k] = v
		}
		r.Attrs = clone
	}
	r.Attrs[key] = value
	return r
}
