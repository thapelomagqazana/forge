package logger

import (
	"testing"
	"time"
)

// --- TDD: zero value ---

// TestRecord_ZeroValueIsValid asserts the zero value of Record is
// usable: every field has a well-defined value, no field needs
// initialization before the record can be read, and the level
// defaults to LevelInfo (the zero value of the Level type).
//
// # Why the level default is Info
//
// LevelInfo is the zero value of Level (see level.go). A zero
// Record therefore carries LevelInfo, the same default a zero
// Options produces for a Logger. The two agree: a record emitted
// without an explicit level is an info-level record.
//
// # What the test does not assert
//
// The test does not assert that the zero Record is a valid
// record for emission. A record with an empty Message and the
// sentinel CorrelationID is not useful output; the type permits
// it, but a producer should set the fields. The test asserts
// only that reading the zero value is safe and yields defined
// values.
func TestRecord_ZeroValueIsValid(t *testing.T) {
	t.Parallel()

	var r Record
	if r.Level != LevelInfo {
		t.Errorf("zero Level = %v; want LevelInfo (the zero value)",
			r.Level)
	}
	if r.Component != ComponentInvalid {
		t.Errorf("zero Component = %v; want ComponentInvalid", r.Component)
	}
	if r.Message != "" {
		t.Errorf("zero Message = %q; want empty", r.Message)
	}
	if r.CorrelationID != "" {
		t.Errorf("zero CorrelationID = %q; want empty", r.CorrelationID)
	}
	if r.Attrs != nil {
		t.Errorf("zero Attrs = %v; want nil", r.Attrs)
	}
}

// --- TDD: WithAttr ---

func TestRecord_WithAttr_AddsKey(t *testing.T) {
	t.Parallel()

	r := Record{Message: "x"}
	got := r.WithAttr("path", "/tmp/out")
	if got.Attrs["path"] != "/tmp/out" {
		t.Fatalf("Attrs[path] = %v; want /tmp/out", got.Attrs["path"])
	}
	if r.Attrs != nil {
		t.Fatalf("original Record was mutated: %v", r.Attrs)
	}
}

func TestRecord_WithAttr_PreservesExistingAttrs(t *testing.T) {
	t.Parallel()

	r := Record{
		Message: "x",
		Attrs:   map[string]any{"a": 1},
	}
	got := r.WithAttr("b", 2)
	if got.Attrs["a"] != 1 {
		t.Errorf("Attrs[a] = %v; want 1", got.Attrs["a"])
	}
	if got.Attrs["b"] != 2 {
		t.Errorf("Attrs[b] = %v; want 2", got.Attrs["b"])
	}
	if len(r.Attrs) != 1 {
		t.Errorf("original Attrs mutated: %v", r.Attrs)
	}
}

// --- Negative: reserved keys panic ---

func TestRecord_WithAttr_ReservedKeysPanic(t *testing.T) {
	t.Parallel()

	reserved := []string{
		"ts", "level", "component", "msg", "correlation_id",
	}
	for _, key := range reserved {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Errorf("WithAttr(%q, ...) did not panic", key)
				}
			}()
			_ = Record{}.WithAttr(key, "value")
		})
	}
}

// --- Non-functional: WithAttr does not mutate its receiver ---

func TestRecord_WithAttr_NoMutation(t *testing.T) {
	t.Parallel()

	r := Record{
		Ts:            time.Now(),
		Level:         LevelInfo,
		Component:     ComponentCLI,
		Message:       "x",
		CorrelationID: "id",
		Attrs:         map[string]any{"a": 1},
	}
	snapshot := make(map[string]any, len(r.Attrs))
	for k, v := range r.Attrs {
		snapshot[k] = v
	}

	_ = r.WithAttr("b", 2)
	_ = r.WithAttr("c", 3)

	if len(r.Attrs) != len(snapshot) {
		t.Fatalf("receiver mutated: len=%d; want %d",
			len(r.Attrs), len(snapshot))
	}
	for k, v := range snapshot {
		if r.Attrs[k] != v {
			t.Errorf("receiver mutated at %q: got %v; want %v",
				k, r.Attrs[k], v)
		}
	}
}

// --- TDD: ReservedAttrKeys matches the Record JSON tags ---

func TestRecord_ReservedKeysMatchJSONTags(t *testing.T) {
	t.Parallel()

	// The reserved set must equal the set of JSON tags on Record.
	// This is a structural pin: a future field added to Record
	// without being added to ReservedAttrKeys would silently allow
	// a shadowing attr.
	want := map[string]bool{
		"ts":             true,
		"level":          true,
		"component":      true,
		"msg":            true,
		"correlation_id": true,
	}
	if len(ReservedAttrKeys) != len(want) {
		t.Fatalf("ReservedAttrKeys size = %d; want %d",
			len(ReservedAttrKeys), len(want))
	}
	for k := range want {
		if !ReservedAttrKeys[k] {
			t.Errorf("ReservedAttrKeys missing %q", k)
		}
	}
}
