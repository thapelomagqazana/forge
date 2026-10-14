package filesystem

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// This file enforces the WBS 13.1.2 API freeze for Layer 1.
//
// The freeze is a documentation artefact: api.go names every
// operation on the FS interface, with a "Consumer:" annotation
// on each. This test verifies two properties:
//
//  1. The set of methods on the FS interface is exactly the set
//     of operations named in api.go.
//  2. Every operation named in api.go has a "Consumer:"
//     annotation.
//
// A failure of (1) means the interface grew (or shrank) without
// the freeze document being updated. A failure of (2) means an
// operation was added to the freeze without a traceable consumer,
// which violates the WBS 13.1.2 acceptance criterion "no
// operation exists just in case."
//
// # What this file does not do
//
// It does not parse the interface's method signatures, only its
// method names. A signature change (Read(path string) → Read(p
// string)) is not caught here; a rename or an addition is. The
// stronger check would require parsing the api.go doc comment
// into signatures, which is a maintenance burden for a check
// whose failure mode is "someone forgot to update one file."

// TestAPI_FSInterfaceIsFrozen asserts that every method on the
// FS interface is named in api.go and vice versa.
func TestAPI_FSInterfaceIsFrozen(t *testing.T) {
	ifaceMethods := parseInterfaceMethods(t, "filesystem.go", "FS")
	apiMethods := parseAPIMethods(t, "api.go")

	if diff := diffSets(ifaceMethods, apiMethods); diff != "" {
		t.Fatalf("FS surface and api.go disagree:\n%s", diff)
	}
}

// TestAPI_EveryOperationHasConsumer asserts that every operation
// named in api.go carries a "Consumer:" annotation.
func TestAPI_EveryOperationHasConsumer(t *testing.T) {
	rows := parseAPIRows(t, "api.go")
	if len(rows) == 0 {
		t.Fatal("api.go contains no operation rows; check the format")
	}
	for _, r := range rows {
		if !strings.Contains(r.body, "Consumer:") {
			t.Errorf("operation %q has no Consumer annotation", r.name)
		}
	}
}

// -----------------------------------------------------------------------------
// Parsing helpers
// -----------------------------------------------------------------------------

// apiRow is one operation entry in api.go. The body accumulates
// every comment line that follows the operation's name line, up
// to the next operation. "Consumer:" appears in the body of a
// well-formed row.
type apiRow struct {
	name string
	body string
}

// parseInterfaceMethods returns the names of the methods declared
// on the named interface in the named file. It uses go/parser so
// that the interface definition is the source of truth, not a
// hardcoded list.
func parseInterfaceMethods(t *testing.T, file, iface string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != iface {
			return true
		}
		it, ok := ts.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}
		for _, m := range it.Methods.List {
			if len(m.Names) > 0 {
				out = append(out, m.Names[0].Name)
			}
		}
		return false
	})
	return out
}

// parseAPIMethods returns the operation names extracted from
// api.go. It is the set the freeze document declares.
func parseAPIMethods(t *testing.T, file string) []string {
	t.Helper()
	rows := parseAPIRows(t, file)
	var out []string
	for _, r := range rows {
		out = append(out, r.name)
	}
	return out
}

// parseAPIRows reads file as text and extracts the operation
// rows. The api.go convention is:
//
//	// Name(args) (returns)
//	//   description...
//	//   Boundary: ...
//	//   Consumer: WBS ...
//
// The extraction is line-oriented rather than AST-based because
// the rows live in a doc comment, and go/parser does not descend
// into the interior of comments.
func parseAPIRows(t *testing.T, file string) []apiRow {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return extractRows(string(data))
}

// extractRows walks the file line by line. A line that begins an
// operation (see isOperationLine) starts a new row; subsequent
// lines accumulate into the current row until the next operation
// line.
func extractRows(text string) []apiRow {
	var rows []apiRow
	var current *apiRow
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		if isOperationLine(body) {
			if current != nil {
				rows = append(rows, *current)
			}
			current = &apiRow{name: nameFromLine(body)}
		}
		if current != nil {
			current.body += body + "\n"
		}
	}
	if current != nil {
		rows = append(rows, *current)
	}
	return rows
}

// isOperationLine reports whether a comment body begins a new
// operation row.
//
// The convention is a line of the form "Name(args) (returns)":
// an opening paren, no leading "//", and not one of the known
// sub-field prefixes ("Boundary:", "Consumer:"). The sub-field
// prefixes are what distinguishes an operation header from a
// description line that happens to contain parentheses.
func isOperationLine(s string) bool {
	if !strings.Contains(s, "(") {
		return false
	}
	switch {
	case strings.HasPrefix(s, "Boundary:"),
		strings.HasPrefix(s, "Consumer:"),
		strings.HasPrefix(s, "Missing:"),
		strings.HasPrefix(s, "Denied:"),
		strings.HasPrefix(s, "Directory:"),
		strings.HasPrefix(s, "Parents:"),
		strings.HasPrefix(s, "Mode:"),
		strings.HasPrefix(s, "Non-atomic:"),
		strings.HasPrefix(s, "Return type:"),
		strings.HasPrefix(s, "Caller must:"),
		strings.HasPrefix(s, "Existing directory:"),
		strings.HasPrefix(s, "Existing non-directory:"),
		strings.HasPrefix(s, "Non-empty directory:"):
		return false
	}
	// A heuristic: an operation line's name is a single
	// identifier followed immediately by "(". A description
	// line that happens to contain "(" will usually have
	// whitespace before the paren.
	paren := strings.Index(s, "(")
	if paren == 0 {
		return false
	}
	name := s[:paren]
	return !strings.ContainsAny(name, " \t")
}

// nameFromLine extracts the operation name from an operation
// line, up to the first "(".
func nameFromLine(s string) string {
	paren := strings.Index(s, "(")
	if paren < 0 {
		return s
	}
	return strings.TrimSpace(s[:paren])
}

// -----------------------------------------------------------------------------
// Set diff
// -----------------------------------------------------------------------------

// diffSets returns a human-readable description of the
// differences between two string slices, or "" if they are
// equal as sets. Order is ignored; the sets are compared for
// membership.
func diffSets(a, b []string) string {
	setA := toSet(a)
	setB := toSet(b)

	var missingFromB, missingFromA []string
	for k := range setA {
		if !setB[k] {
			missingFromB = append(missingFromB, k)
		}
	}
	for k := range setB {
		if !setA[k] {
			missingFromA = append(missingFromA, k)
		}
	}
	if len(missingFromB) == 0 && len(missingFromA) == 0 {
		return ""
	}

	var sb strings.Builder
	if len(missingFromB) > 0 {
		fmt.Fprintf(&sb,
			"in interface but not api.go: %v\n", missingFromB)
	}
	if len(missingFromA) > 0 {
		fmt.Fprintf(&sb,
			"in api.go but not interface: %v\n", missingFromA)
	}
	return sb.String()
}

func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}
