package logger

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// writeHuman renders r in the human format to w.
//
// The format is:
//
//	LEVEL [component] message key=value key=value ...
//
// The line ends in exactly one newline. The rendering is
// deterministic: the keys of Attrs are sorted before printing, so
// two records with the same Attrs render identically regardless of
// Go's map iteration order.
//
// writeHuman is deliberately not part of the public API. It is
// called by StderrSink. The human format is not stable; see
// docs/reference/log-record-format.md.
func writeHuman(w io.Writer, r Record) error {
	level := humanLevel(r.Level)
	component := r.Component.String()
	msg := r.Message

	// Compose the line in a single buffer to avoid partial writes.
	var b strings.Builder
	b.Grow(len(level) + len(component) + len(msg) + 16)

	// Level, padded to a fixed width so the component column
	// aligns across records.
	b.WriteString(level)
	b.WriteByte(' ')

	// Component in brackets.
	b.WriteByte('[')
	b.WriteString(component)
	b.WriteString("] ")

	// Message verbatim.
	b.WriteString(msg)

	// Attrs, sorted by key for determinism.
	if len(r.Attrs) > 0 {
		keys := make([]string, 0, len(r.Attrs))
		for k := range r.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteByte(' ')
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(humanValue(r.Attrs[k]))
		}
	}
	b.WriteByte('\n')

	_, err := io.WriteString(w, b.String())
	return err
}

// humanLevel returns the level name padded to the width of the
// longest level name ("DEBUG", 5 characters).
//
// The padding aligns the component column across lines of
// different levels. Callers that print the padded name in a
// non-monospace terminal will see the alignment fail; that is
// accepted, because the human format is explicitly not stable.
func humanLevel(l Level) string {
	name := strings.ToUpper(l.String())
	for len(name) < 5 {
		name += " "
	}
	return name
}

// humanValue formats an attr value for the human rendering.
//
// The rules are documented in
// docs/reference/log-record-format.md §"Value formatting rules".
// The function is deliberately simple: it covers the primitive
// types that make up the bulk of Forge's attrs, and falls back to
// fmt.Sprintf for everything else.
func humanValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return quoteIfNeeded(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case time.Time:
		return x.UTC().Format(time.RFC3339)
	case error:
		return quoteIfNeeded(x.Error())
	default:
		return quoteIfNeeded(fmt.Sprintf("%v", v))
	}
}

// quoteIfNeeded returns s verbatim if it contains no whitespace
// and no quote character; otherwise it returns s double-quoted
// with Go string-escaping.
//
// The function does not quote an empty string. An empty string
// renders as `key=`, which is unambiguous in the grammar.
func quoteIfNeeded(s string) string {
	if s == "" {
		return s
	}
	if !strings.ContainsAny(s, " \t\n\r\"") {
		return s
	}
	return strconv.Quote(s)
}
