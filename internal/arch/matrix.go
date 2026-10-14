package arch

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Matrix is the parsed, validated dependency matrix.
//
// It is loaded from docs/architecture/dependency-matrix.yaml, which is the
// authoritative, machine-readable form of ADR-042. The human-readable form is
// docs/architecture/dependency-matrix.md; the two are asserted equal by
// TestMatrixYAMLMatchesMarkdown.
//
// A Matrix is immutable once loaded. Concurrent reads are safe: the compiled
// lookup tables are built once by Compile and never mutated afterward.
//
// Loading is a two-phase process:
//
//  1. ParseMatrix (or LoadMatrix) decodes the YAML and runs semantic
//     validation, rejecting malformed matrices with a message that names the
//     offending field.
//  2. Compile builds the fast-lookup tables used by Lookup and GroupOf. It is
//     called automatically by ParseMatrix; callers using the zero Matrix must
//     call it explicitly before Lookup.
type Matrix struct {
	// Version is the matrix schema version. Currently 1. The parser rejects
	// any other value so that silent schema evolution cannot happen.
	Version int `yaml:"version"`

	// Schema is the path to the JSON Schema that validates this file. It is
	// recorded for traceability; the parser does not fetch it.
	Schema string `yaml:"schema"`

	// ADR is the path to the Architecture Decision Record that governs this
	// matrix (ADR-042). It is recorded for traceability.
	ADR string `yaml:"adr"`

	// Traceability lists the requirement IDs this matrix satisfies, e.g.
	// "FR-ARCH-002". It is recorded for audit purposes.
	Traceability []string `yaml:"traceability"`

	// Groups maps a group name (e.g. "infrastructure.filesystem") to its
	// definition. A group is a named set of packages that share import rules.
	// Every group belongs to exactly one layer.
	Groups map[string]Group `yaml:"groups"`

	// RawMatrix is the literal YAML matrix: RawMatrix[from][to] is one of
	// "allow", "deny", or "condition:<id>". It is preserved alongside the
	// compiled form so that error messages and the Acyclic check can name the
	// original cell value rather than the compiled Decision.
	//
	// Unexported code should prefer Lookup over reading RawMatrix directly.
	RawMatrix map[string]map[string]string `yaml:"matrix"`

	// Conditions maps a condition ID (e.g. "C1") to its definition. A
	// condition is a named, conditional permission in the matrix.
	Conditions map[string]Condition `yaml:"conditions"`

	// Exceptions lists cross-cutting permissions that apply to every group.
	// The exception list is closed: adding to it requires a superseding ADR.
	Exceptions []Exception `yaml:"exceptions"`

	// compiled is the fast-lookup form built by Compile. It maps a group pair
	// to its Decision. Unlisted pairs default to DecisionDeny at lookup time;
	// they are not materialized here.
	compiled map[groupPair]Decision

	// exceptionEdges records which pairs in `compiled` were added by the
	// cross-cutting exception registry rather than by an explicit matrix cell.
	//
	// Acyclic ignores these pairs. The rationale: an exception edge represents
	// a universal permission ("every group may import shared.errors"), not a
	// substantive dependency between two groups. When two groups are both
	// excepted (shared.errors and shared.logging), each exception grants an
	// edge from one to the other, producing a cycle that has no semantic
	// content. Excluding exception edges from the acyclicity check makes the
	// check ask the right question — "do two groups substantively require
	// each other?" — without weakening the check for explicit matrix cells.
	//
	// Lookup and GroupOf do NOT consult this set: the exception registry must
	// continue to grant the imports it always granted.
	exceptionEdges map[groupPair]bool

	// byPath maps a package-path prefix to the group that owns it. Lookups use
	// longest-prefix match with a "/" boundary. It is built by Compile.
	byPath map[string]string
}

// Group is a named set of packages that share import rules.
//
// A group is the unit of the matrix. It is finer-grained than a layer: the
// Infrastructure layer contains multiple groups (infrastructure.filesystem,
// infrastructure.config, and so on), each with its own row and column.
type Group struct {
	// Layer is the architectural layer this group belongs to. One of:
	// "presentation", "application", "domain", "infrastructure", "shared".
	Layer string `yaml:"layer"`

	// Packages lists the import-path prefixes owned by this group. A prefix
	// matches the exact path and every sub-package of it, subject to
	// longest-prefix resolution when two groups claim overlapping subtrees.
	// Paths are module-relative and start with "internal/".
	Packages []string `yaml:"packages"`
}

// Condition is a named, conditional permission in the matrix.
//
// A condition exists when a permission is real but bounded: the import is
// allowed, but only under a stated constraint that a reviewer (or, later, a
// type-checker) must verify. Naming conditions (rather than describing them
// inline) lets the YAML cell, the Markdown footnote, the ADR table, and the
// future lint rule's error message all refer to the same thing by the same
// identifier.
type Condition struct {
	// Summary is a one-line description of the condition.
	Summary string `yaml:"summary"`

	// Rationale explains why the condition exists and what would go wrong if
	// it were relaxed to an unconditional allow.
	Rationale string `yaml:"rationale"`

	// AppliesTo lists the "from -> to" group pairs this condition governs.
	// It is the mechanical cross-check between the conditions block and the
	// matrix block: every entry must correspond to a cell whose value is
	// exactly "condition:<this id>".
	//
	// A condition may have an empty AppliesTo list only if it is managed by
	// the exception registry (currently C4, which applies via shared.logging
	// being an exception rather than via explicit matrix cells).
	AppliesTo []string `yaml:"applies_to"`

	// EnforcedBy records how the condition is enforced today. One of:
	//
	//   - "layer":  the layer-level check (14.1.1) already covers it.
	//   - "review": enforcement is by code review; no mechanical check yet.
	//   - "type-checker": enforced by a type-checker rule (planned for 14.6.1).
	//
	// Recording this explicitly prevents the common drift where a condition
	// is assumed to be enforced but is not.
	EnforcedBy string `yaml:"enforced_by"`
}

// Exception is a cross-cutting permission that applies to every group.
//
// Exceptions exist so that the matrix does not need a cell for every
// (importer, shared.errors) and (importer, shared.logging) pair. They are
// applied uniformly to every group and cannot be overridden by a deny in the
// matrix.
type Exception struct {
	// Group is the group being excepted, e.g. "shared.errors".
	Group string `yaml:"group"`

	// AppliesTo is the scope of the exception. Currently only "all" is
	// supported, meaning every group in the matrix may import Group.
	AppliesTo string `yaml:"applies_to"`

	// Rationale explains why the exception exists and cites the ADRs that
	// motivate it.
	Rationale string `yaml:"rationale"`
}

// Decision is the outcome of looking up a (from, to) group pair.
type Decision int

const (
	// DecisionDeny is the default for any pair not explicitly allowed.
	// It is also the value returned when either group is unclassified.
	DecisionDeny Decision = iota

	// DecisionAllow is an unconditional permission.
	DecisionAllow

	// DecisionConditional is a permission that applies only under a named
	// condition. The condition ID is returned alongside the Decision by
	// Lookup.
	DecisionConditional
)

// groupPair is the key for the compiled decision map.
type groupPair struct {
	from, to string
}

// LoadMatrix reads and validates a dependency matrix from a YAML file.
//
// It returns an error if the file is missing, unreadable, malformed, fails
// semantic validation, contains a cycle, or references undeclared groups or
// conditions. Every error message names the offending field so a maintainer
// can fix it without re-reading the schema.
//
// LoadMatrix is the entry point used by tests and by the future lint rule
// (14.6.1). Callers that already have the bytes in memory should use
// ParseMatrix.
func LoadMatrix(path string) (*Matrix, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("matrix: read %s: %w", path, err)
	}
	m, err := ParseMatrix(raw)
	if err != nil {
		return nil, fmt.Errorf("matrix: %s: %w", path, err)
	}
	return m, nil
}

// ParseMatrix parses and validates a dependency matrix from bytes.
//
// It is used by LoadMatrix and by tests. The validation steps are:
//
//  1. Parse YAML with unknown fields rejected. This catches typos in field
//     names that would otherwise be silently ignored and produce a matrix
//     that is missing rules the author intended to add.
//  2. Validate that every matrix row and every cell names a declared group.
//  3. Validate that every condition referenced by a cell is declared, and
//     that every condition's AppliesTo entry corresponds to a matrix cell
//     whose value references that condition.
//  4. Validate that every condition is either referenced by at least one
//     cell or explicitly exempted from that requirement (currently only C4).
//  5. Validate that every exception names a declared group.
//  6. Compile the fast-lookup tables.
//  7. Reject a matrix whose explicit cells form a cycle.
//
// Steps 3 and 4 together catch the two most common drift bugs: a condition
// added to the matrix but not to the conditions block (dangling reference),
// and a condition declared but never used (dead condition). Both are silent
// rot in a purely textual matrix.
func ParseMatrix(raw []byte) (*Matrix, error) {
	var m Matrix
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("matrix: parse YAML: %w", err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	if err := m.Compile(); err != nil {
		return nil, err
	}
	if err := m.Acyclic(); err != nil {
		return nil, err
	}
	return &m, nil
}

// validate runs the semantic checks described on ParseMatrix. It does not
// build the compiled form; that is Compile's job.
func (m *Matrix) validate() error {
	if m.Version != 1 {
		return fmt.Errorf("matrix: unsupported version %d (want 1)", m.Version)
	}
	if len(m.Groups) == 0 {
		return fmt.Errorf("matrix: no groups declared")
	}

	// Every group must have a layer and at least one package path, and every
	// path must be under internal/ (the matrix only governs this module's own
	// packages; stdlib and external modules are out of scope).
	for name, g := range m.Groups {
		if g.Layer == "" {
			return fmt.Errorf("matrix: group %q has no layer", name)
		}
		switch g.Layer {
		case "presentation", "application", "domain", "infrastructure", "shared":
			// ok
		default:
			return fmt.Errorf("matrix: group %q has unknown layer %q", name, g.Layer)
		}
		if len(g.Packages) == 0 {
			return fmt.Errorf("matrix: group %q has no packages", name)
		}
		for _, p := range g.Packages {
			if !strings.HasPrefix(p, "internal/") {
				return fmt.Errorf("matrix: group %q package %q is not under internal/", name, p)
			}
		}
	}

	// Every matrix row and every cell must name a declared group.
	for from, row := range m.RawMatrix {
		if _, ok := m.Groups[from]; !ok {
			return fmt.Errorf("matrix: matrix row %q is not a declared group", from)
		}
		for to, cell := range row {
			if _, ok := m.Groups[to]; !ok {
				return fmt.Errorf("matrix: matrix cell %s -> %s: %q is not a declared group", from, to, to)
			}
			if err := validateCell(from, to, cell, m.Conditions); err != nil {
				return err
			}
		}
	}

	// Build the set of condition IDs actually referenced by the matrix, so
	// the dead-condition check below does not depend on AppliesTo being
	// correctly populated (a stale AppliesTo is itself a drift the check
	// should not mask).
	referenced := map[string]bool{}
	for _, row := range m.RawMatrix {
		for _, cell := range row {
			if id, ok := parseConditionRef(cell); ok {
				referenced[id] = true
			}
		}
	}

	// Every condition's AppliesTo entry must correspond to a matrix cell,
	// and that cell's value must reference the same condition. This is the
	// strongest cross-check between the conditions block and the matrix block:
	// it forbids a condition whose AppliesTo claims coverage that the matrix
	// does not actually grant.
	for id, cond := range m.Conditions {
		// C4 is managed by the exception registry, not by explicit matrix
		// cells, and legitimately has an empty AppliesTo.
		if id == "C4" {
			continue
		}
		if len(cond.AppliesTo) == 0 {
			return fmt.Errorf("matrix: condition %s has no applies_to and is not exception-managed", id)
		}
		for _, pair := range cond.AppliesTo {
			from, to, err := parsePair(pair)
			if err != nil {
				return fmt.Errorf("matrix: condition %s: %w", id, err)
			}
			row, ok := m.RawMatrix[from]
			if !ok {
				return fmt.Errorf("matrix: condition %s applies_to %q: row %q not in matrix", id, pair, from)
			}
			cell, ok := row[to]
			if !ok {
				return fmt.Errorf("matrix: condition %s applies_to %q: cell not in matrix", id, pair)
			}
			if !isConditionRef(cell, id) {
				return fmt.Errorf("matrix: condition %s applies_to %q: matrix says %q", id, pair, cell)
			}
		}
	}

	// Dead condition check: a condition that is referenced by no cell and is
	// not exception-managed is a leftover from an earlier design.
	for id := range m.Conditions {
		if id == "C4" {
			continue
		}
		if !referenced[id] {
			return fmt.Errorf("matrix: condition %s is declared but never referenced", id)
		}
	}

	// Every exception's group must be declared.
	for _, ex := range m.Exceptions {
		if _, ok := m.Groups[ex.Group]; !ok {
			return fmt.Errorf("matrix: exception references undeclared group %q", ex.Group)
		}
	}
	return nil
}

// Compile builds the fast-lookup form of the matrix: a map from group pair
// to decision, and a map from package-path prefix to group name.
//
// Compile is idempotent and safe to call more than once. It is called by
// ParseMatrix; callers constructing a Matrix by hand must call it explicitly
// before Lookup or GroupOf.
//
// The compiled form has three layers of resolution, in order:
//
//  1. Group self-imports are allowed implicitly. A group may import itself;
//     the matrix does not need a cell for every diagonal.
//  2. Explicit matrix cells: "allow", "deny", or "condition:<id>".
//  3. Cross-cutting exceptions. These override any deny, so a deny cell for
//     (group, shared.errors) cannot suppress the shared.errors exception.
//     This is deliberate: the exception list is the closed registry of
//     universally permitted imports, and the matrix must not be able to
//     accidentally forbid them.
//
// Exception-derived pairs are recorded in m.exceptionEdges so that Acyclic
// can exclude them from the cycle check. See the field comment on
// exceptionEdges for the rationale.
func (m *Matrix) Compile() error {
	m.compiled = make(map[groupPair]Decision, len(m.RawMatrix)*4)
	m.exceptionEdges = make(map[groupPair]bool, len(m.Groups)*len(m.Exceptions))
	m.byPath = make(map[string]string, len(m.Groups)*2)

	// Layer 1: self-imports.
	for name := range m.Groups {
		m.compiled[groupPair{from: name, to: name}] = DecisionAllow
	}

	// Layer 2: explicit matrix cells.
	for from, row := range m.RawMatrix {
		for to, cell := range row {
			switch {
			case cell == "allow":
				m.compiled[groupPair{from: from, to: to}] = DecisionAllow
			case cell == "deny":
				m.compiled[groupPair{from: from, to: to}] = DecisionDeny
			default:
				if _, ok := parseConditionRef(cell); ok {
					m.compiled[groupPair{from: from, to: to}] = DecisionConditional
				} else {
					return fmt.Errorf("matrix: invalid cell %s -> %s: %q", from, to, cell)
				}
			}
		}
	}

	// Layer 3: cross-cutting exceptions. Only "all" scope is supported today;
	// a future scope (e.g. "layers:domain,application") would require a
	// superseding ADR.
	for _, ex := range m.Exceptions {
		if ex.AppliesTo != "all" {
			return fmt.Errorf("matrix: exception %q: unsupported applies_to %q", ex.Group, ex.AppliesTo)
		}
		for from := range m.Groups {
			if from == ex.Group {
				continue
			}
			pair := groupPair{from: from, to: ex.Group}
			m.compiled[pair] = DecisionAllow
			m.exceptionEdges[pair] = true
		}
	}

	// Build the path-prefix index. Two groups must not claim the same path
	// prefix; overlapping prefixes within a group are allowed and deduplicated
	// by the map.
	for name, g := range m.Groups {
		for _, p := range g.Packages {
			if existing, ok := m.byPath[p]; ok && existing != name {
				return fmt.Errorf("matrix: package %q is claimed by groups %q and %q", p, existing, name)
			}
			m.byPath[p] = name
		}
	}
	return nil
}

// GroupOf returns the group name that owns the given internal package path,
// or "" if the path is not covered by any group.
//
// Matching is by longest prefix with a "/" boundary. Longest-prefix is what
// makes it safe for a more specific group to override a broader one: if a
// future "infrastructure.filesystem.safefs" group were introduced, its
// packages would take precedence over "infrastructure.filesystem" for the
// subtree they name.
//
// The returned name is a key into Matrix.Groups.
func (m *Matrix) GroupOf(internalPath string) string {
	var bestPrefix string
	for prefix := range m.byPath {
		if !hasPrefix(internalPath, prefix) {
			continue
		}
		if len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
		}
	}
	if bestPrefix == "" {
		return ""
	}
	return m.byPath[bestPrefix]
}

// Lookup returns the decision for an import from group `from` to group `to`.
//
// The second return value is the condition ID when the decision is
// DecisionConditional, and the empty string otherwise. Callers that only need
// the decision can ignore it.
//
// Unlisted pairs default to DecisionDeny. A pair is "listed" if it appears in
// the compiled map, which includes self-imports and exception-derived cells
// in addition to the explicit matrix cells.
func (m *Matrix) Lookup(from, to string) (Decision, string) {
	d, ok := m.compiled[groupPair{from: from, to: to}]
	if !ok {
		return DecisionDeny, ""
	}
	if d == DecisionConditional {
		// The compiled map records "conditional" but not which condition;
		// recover the ID from the raw matrix. This keeps the compiled map
		// small (one Decision per pair) while allowing the ID to be returned
		// at lookup time.
		if row, ok := m.RawMatrix[from]; ok {
			if cell, ok := row[to]; ok {
				if id, ok := parseConditionRef(cell); ok {
					return DecisionConditional, id
				}
			}
		}
	}
	return d, ""
}

// LookupPath is a convenience wrapper around GroupOf + Lookup.
//
// It returns the Decision, the condition ID (if any), and a non-empty reason
// string when the lookup could not be performed because one of the paths is
// unclassified. Callers that need to distinguish "unclassified" from "denied"
// should call GroupOf directly.
//
// The three return values are:
//
//	decision  the Decision for the (from, to) group pair, or DecisionDeny
//	condition the condition ID when decision is DecisionConditional
//	reason    a human-readable explanation when decision could not be
//	          computed; empty when the decision is meaningful
func (m *Matrix) LookupPath(fromPath, toPath string) (Decision, string, string) {
	fromGroup := m.GroupOf(fromPath)
	toGroup := m.GroupOf(toPath)
	if fromGroup == "" {
		return DecisionDeny, "", fmt.Sprintf("importer %q is not classified by the matrix", fromPath)
	}
	if toGroup == "" {
		return DecisionDeny, "", fmt.Sprintf("imported %q is not classified by the matrix", toPath)
	}
	d, cond := m.Lookup(fromGroup, toGroup)
	return d, cond, ""
}

// validateCell checks that a cell value is well-formed and that any condition
// it references is declared.
func validateCell(from, to, cell string, conditions map[string]Condition) error {
	switch cell {
	case "allow", "deny":
		return nil
	}
	id, ok := parseConditionRef(cell)
	if !ok {
		return fmt.Errorf("matrix: cell %s -> %s: invalid value %q (want allow, deny, or condition:<id>)", from, to, cell)
	}
	if _, ok := conditions[id]; !ok {
		return fmt.Errorf("matrix: cell %s -> %s: references undeclared condition %s", from, to, id)
	}
	return nil
}

// parseConditionRef parses "condition:C1" and returns ("C1", true).
// It returns ("", false) for any other input, including "condition:" with no
// ID. It does not validate that the ID is declared; that is the caller's job.
func parseConditionRef(cell string) (string, bool) {
	const prefix = "condition:"
	if !strings.HasPrefix(cell, prefix) {
		return "", false
	}
	id := strings.TrimPrefix(cell, prefix)
	if id == "" {
		return "", false
	}
	return id, true
}

// isConditionRef reports whether cell is exactly "condition:<id>".
// It is the strict form used by the AppliesTo cross-check, which must reject
// "condition:C10" when the condition ID is "C1".
func isConditionRef(cell, id string) bool {
	got, ok := parseConditionRef(cell)
	return ok && got == id
}

// parsePair parses "a -> b" into ("a", "b"). It trims surrounding whitespace
// on both sides, so "a->b", "a ->b", and "a -> b" all parse the same way.
func parsePair(s string) (string, string, error) {
	parts := strings.Split(s, "->")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid pair %q (want \"from -> to\")", s)
	}
	from := strings.TrimSpace(parts[0])
	to := strings.TrimSpace(parts[1])
	if from == "" || to == "" {
		return "", "", fmt.Errorf("invalid pair %q: empty side", s)
	}
	return from, to, nil
}

// Acyclic reports whether the matrix's substantive edges form a DAG.
//
// "Substantive" means explicit matrix cells: unconditional allow and
// conditional cells. Exception-derived edges (from the cross-cutting
// exception registry) are excluded. See the comment on Matrix.exceptionEdges
// for the rationale: exception edges represent universal permissions, not
// dependencies, and including them produces a spurious cycle whenever two
// exception groups exist.
//
// Self-loops (a group importing itself) are also excluded: the matrix
// explicitly allows them, and the self-import edge is not a dependency in
// the sense that matters here (it does not create a mutual allowance between
// two distinct groups).
//
// It returns nil if acyclic, or an error naming one cycle if not. The check
// uses a standard three-color depth-first search (white = unvisited,
// gray = on the current path, black = fully explored). A gray node reached
// again is a back edge, i.e. a cycle. The cycle is reconstructed from the
// DFS stack so the error message names the actual path, not just "a cycle
// exists".
func (m *Matrix) Acyclic() error {
	// Build adjacency from the compiled map, excluding self-loops and
	// exception-derived edges. See the doc comment above for why each is
	// excluded.
	adj := map[string][]string{}
	for pair, d := range m.compiled {
		if pair.from == pair.to {
			continue
		}
		if m.exceptionEdges[pair] {
			continue
		}
		if d == DecisionAllow || d == DecisionConditional {
			adj[pair.from] = append(adj[pair.from], pair.to)
		}
	}
	// Deterministic iteration for stable error messages and reproducible test
	// output. Map iteration order in Go is randomized; sorting the adjacency
	// lists and the node list makes the DFS visit nodes in a fixed order.
	for k := range adj {
		sort.Strings(adj[k])
	}
	nodes := make([]string, 0, len(m.Groups))
	for name := range m.Groups {
		nodes = append(nodes, name)
	}
	sort.Strings(nodes)

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string

	var visit func(n string) error
	visit = func(n string) error {
		color[n] = gray
		stack = append(stack, n)
		for _, next := range adj[n] {
			switch color[next] {
			case gray:
				// Back edge: reconstruct the cycle from the DFS stack.
				idx := 0
				for i, s := range stack {
					if s == next {
						idx = i
						break
					}
				}
				cycle := append(append([]string{}, stack[idx:]...), next)
				return fmt.Errorf("matrix: cycle detected: %s", strings.Join(cycle, " -> "))
			case white:
				if err := visit(next); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return nil
	}

	for _, n := range nodes {
		if color[n] == white {
			if err := visit(n); err != nil {
				return err
			}
		}
	}
	return nil
}

// Markdown renders the matrix as a Markdown table.
//
// The output is byte-stable: same input, same output. It is compared against
// the table in docs/architecture/dependency-matrix.md by
// TestMatrixYAMLMatchesMarkdown, and against the golden file
// testdata/dependency-matrix.golden.yaml (via TestCanonicalMatrixGolden).
//
// The rendering uses ✓, ✗, and ⚠ exactly as the human-readable form does, so
// that a diff in the docs is readable. Group order is layer order first, then
// alphabetical within a layer, and is defined by sortedGroups.
func (m *Matrix) Markdown() string {
	groups := m.sortedGroups()
	var b strings.Builder

	b.WriteString("| importer \\ imported |")
	for _, g := range groups {
		fmt.Fprintf(&b, " %s |", g)
	}
	b.WriteString("\n")

	b.WriteString("|")
	for range groups {
		b.WriteString(":---:|")
	}
	b.WriteString("\n")

	for _, from := range groups {
		fmt.Fprintf(&b, "| **%s** |", from)
		for _, to := range groups {
			d, _ := m.Lookup(from, to)
			switch d {
			case DecisionAllow:
				b.WriteString(" ✓ |")
			case DecisionConditional:
				b.WriteString(" ⚠ |")
			default:
				b.WriteString(" ✗ |")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// sortedGroups returns the group names in the order used by the published
// matrix: layer order (presentation, application, domain, infrastructure,
// shared), then alphabetical within a layer. Stable ordering is required for
// golden comparison.
func (m *Matrix) sortedGroups() []string {
	layerOrder := map[string]int{
		"presentation":   0,
		"application":    1,
		"domain":         2,
		"infrastructure": 3,
		"shared":         4,
	}
	names := make([]string, 0, len(m.Groups))
	for name := range m.Groups {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		li := layerOrder[m.Groups[names[i]].Layer]
		lj := layerOrder[m.Groups[names[j]].Layer]
		if li != lj {
			return li < lj
		}
		return names[i] < names[j]
	})
	return names
}
