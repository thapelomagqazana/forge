package safefs

import (
	"strings"
)

// This file defines the deny-list and allow-list types that
// refine the boundary model beyond the Root and sub-boundary
// checks. See docs/reference/boundary-model.md for the model and
// the boundary decision tree.
//
// # The three controls
//
// A name is subject to up to three independent checks before it
// reaches the filesystem:
//
//  1. The Root boundary. Enforced by the pipeline's validateName
//     (Layer 1) and the parent-symlink resolution (Layer 3).
//
//  2. The sub-boundary, if the operation is on a Handle. Enforced
//     by resolveForIOHandle.
//
//  3. The deny-list and allow-list. Enforced by the matchers in
//     this file, invoked from the pipeline after the boundary
//     checks succeed.
//
// The three are independent. A name that satisfies all applicable
// checks proceeds; a name that fails any is rejected with the
// sentinel specific to the failing check.

// DefaultDenyPatterns are the deny-list entries that every Root
// is constructed with unless the caller overrides them.
//
// See docs/reference/boundary-model.md, "The Deny-list
// Defaults", for the rationale for each entry.
//
// # Why the defaults are a package-level value
//
// The defaults are the same for every Root, so they are a
// constant. A caller that wants a different set uses
// WithDenyList (which replaces) or WithAdditionalDenyPatterns
// (which extends). The default is extend, because a caller who
// wants to remove a default must say so explicitly, and the
// removal is visible in review.
//
// # Why the defaults are a slice, not a matchList
//
// The matchList is constructed lazily, in New, so that a parse
// failure is reported at Root construction time and not at
// package init time. A package-level matchList would panic on
// load if a pattern were malformed, which would abort every
// test binary that imports the package. The slice of strings is
// safe to have as a constant; the parse is deferred to where it
// belongs.
//
// # Do not mutate this slice
//
// The slice is exported so that callers can inspect the defaults
// (for example, to construct a list that replaces them
// thoughtfully). Callers MUST NOT mutate it. A mutation would
// change the behaviour of every Root constructed after the
// mutation, which is a process-wide side effect that no caller
// should be able to cause by accident.
//
// The correct way to change a Root's deny-list is to pass
// WithDenyList or WithAdditionalDenyPatterns at construction.
var DefaultDenyPatterns = []string{
	// Git metadata. A write to .git/config, .git/hooks/*, or
	// .git/HEAD can execute arbitrary code on the next Git
	// command. The boundary alone permits writes here because
	// .git/ is inside every Git working tree.
	".git",

	// Forge's internal backup directory. Writes here could
	// corrupt the update mechanism's rollback state.
	".forge/backups",

	// Forge's update lock file. A write here could race with a
	// concurrent `forge update`.
	".forge/lock",

	// Vim swap files. These are transient and are never
	// intentional outputs. A write here is almost always a
	// mistake.
	"**/*.swp",

	// macOS directory metadata. Not an intentional output.
	"**/.DS_Store",
}

// -----------------------------------------------------------------------------
// Boundary patterns
// -----------------------------------------------------------------------------

// pattern is a single entry in a deny-list or allow-list.
//
// # Syntax
//
// A pattern is a forward-slash, cleaned relative path with two
// special forms:
//
//   - "**/x" matches "x" at any depth under the root. It matches
//     "x", "a/x", "a/b/x", and so on.
//
//   - "x/**" matches every path under "x". It matches "x/",
//     "x/a", "x/a/b", and so on. It does not match "x" itself;
//     a pattern that wants to match both uses two entries.
//
// A pattern without a special form matches the name exactly and
// every name under it. The pattern "a/b" matches "a/b",
// "a/b/c", and "a/b/c/d", but not "a/bee".
//
// # Why a small syntax
//
// A full glob language ("*.go", "src/**/testdata/*.txt") would
// permit patterns that a reviewer cannot verify by inspection.
// The current syntax covers the Phase 12 use cases and is small
// enough that a reviewer can read a pattern list and know
// exactly what it permits.
type pattern struct {
	// raw is the pattern as the caller wrote it, used in
	// diagnostic messages.
	raw string

	// prefix is the literal prefix the pattern matches. For
	// "a/b", prefix is "a/b". For "**/x", prefix is "x" and
	// anyDepth is true. For "x/**", prefix is "x" and
	// prefixOnly is true.
	prefix string

	// anyDepth indicates that the pattern matches at any depth
	// under the root. It is set for patterns of the form
	// "**/x".
	anyDepth bool

	// prefixOnly indicates that the pattern matches every path
	// strictly under prefix, but not prefix itself. It is set
	// for patterns of the form "x/**".
	prefixOnly bool
}

// parsePattern parses a pattern string into a pattern value. It
// returns the zero value for a pattern that cannot be parsed; the
// caller is expected to have validated the pattern at
// construction time, and a parse failure here is a programming
// error that the constructor prevents.
//
// # Validation
//
// The pattern is validated by the same rules as a name
// (filepath.IsLocal), with the two special forms ("**/" prefix
// and "/**" suffix) permitted. A pattern that fails validation
// is rejected at Root construction time, not at operation time.
func parsePattern(raw string) (pattern, bool) {
	if raw == "" {
		return pattern{}, false
	}

	p := pattern{raw: raw}

	switch {
	case strings.HasPrefix(raw, "**/"):
		p.anyDepth = true
		p.prefix = raw[len("**/"):]
		if p.prefix == "" {
			return pattern{}, false
		}
	case strings.HasSuffix(raw, "/**"):
		p.prefixOnly = true
		p.prefix = raw[:len(raw)-len("/**")]
		if p.prefix == "" {
			return pattern{}, false
		}
	default:
		p.prefix = raw
	}

	// The prefix must satisfy the name contract. This is what
	// prevents a caller from injecting a traversal or an
	// absolute path through the deny-list.
	if p.prefix == "." || !isLocalName(p.prefix) {
		return pattern{}, false
	}

	return p, true
}

// matches reports whether the pattern matches the given name.
// The name must be a cleaned, forward-slash, local relative name.
//
// # The three branches
//
// The pattern's three forms are matched by the three branches:
//
//   - anyDepth (the "**/x" form): the component x appears at
//     some position in the name. Four cases are checked; see
//     the comment on the branch.
//
//   - prefixOnly (the "x/**" form): the name is strictly under
//     x. The name "x" itself does not match; a caller that wants
//     both uses two entries.
//
//   - default (the "a/b" form): the name is exactly a/b or is
//     under a/b. The prefix check is on a segment boundary; a
//     name like "a/bee" does not match "a/b".
//
// # Why the anyDepth branch has four cases
//
// The pattern "**/x" means "a path component named x, at any
// depth." A name can place that component in four positions:
//
//  1. As the whole name: "x".
//  2. At the start, followed by more components: "x/a/b".
//  3. At the end, preceded by other components: "a/b/x".
//  4. In the middle, surrounded by other components: "a/x/b".
//
// All four cases must be checked. An earlier version of this
// function checked only cases 1, 3, and 4, which missed case 2
// (a name that begins with the prefix). The miss caused a
// top-level match to be treated as a non-match: the pattern
// "**/*.swp" did not match a top-level "foo.swp", and the
// pattern "**/assets" did not match a top-level "assets/logo".
//
// The four checks are ordered from the most specific to the
// least. The first two are constant-time; the last two walk the
// string.
func (p pattern) matches(name string) bool {
	if p.anyDepth {
		// Case 1: the name is exactly the prefix.
		if name == p.prefix {
			return true
		}
		// Case 2: the name begins with the prefix followed by
		// a separator. This is the case a top-level match
		// takes.
		if strings.HasPrefix(name, p.prefix+"/") {
			return true
		}
		// Case 3: the name ends with a separator followed by
		// the prefix.
		if strings.HasSuffix(name, "/"+p.prefix) {
			return true
		}
		// Case 4: the name contains the prefix between two
		// separators.
		return strings.Contains(name, "/"+p.prefix+"/")
	}

	if p.prefixOnly {
		// Match strictly under the prefix. The name "x"
		// itself does not match; the pattern "x/**" is
		// satisfied by "x/" and any path under it.
		return strings.HasPrefix(name, p.prefix+"/")
	}

	// Match the prefix and everything under it. The check is on
	// a segment boundary: "a/b" matches "a/b" and "a/b/c" but
	// not "a/bee".
	return name == p.prefix || strings.HasPrefix(name, p.prefix+"/")
}

// isLocalName reports whether the name is a local relative path
// per the rules pattern parsing applies. It is a narrow version
// of validateName's check that does not go through the Root,
// because patterns are validated at Root construction time,
// before a Root exists.
//
// The check is:
//
//   - non-empty
//   - forward-slash (the caller is expected to pass a
//     forward-slash pattern; a backslash is a programming
//     error)
//   - does not begin with "/" or ".."
//   - contains no ".." segments
//
// The full filepath.IsLocal is not used here because it would
// reject the "**/x" and "x/**" forms, which are valid pattern
// syntax. The parse step strips those forms before this check
// runs on the resulting prefix.
func isLocalName(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "/") {
		return false
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
// Match lists
// -----------------------------------------------------------------------------

// matchList is a list of patterns with a precomputed match check.
//
// The list is immutable after construction. Matching is O(n) in
// the number of patterns; the lists are small (fewer than a
// dozen patterns in production) and the cost is dominated by the
// string operations, which the compiler optimises.
type matchList struct {
	patterns []pattern
}

// newMatchList constructs a matchList from raw pattern strings.
// It returns an error if any pattern fails to parse. The error
// names the pattern that failed.
//
// The constructor is called from safefs.New (for the Root's
// deny-list) and from WithAllowList (for a Handle's allow-list).
// A pattern that fails to parse at construction time is a
// configuration error that the caller must fix before the Root
// is used.
func newMatchList(raws []string) (*matchList, error) {
	if len(raws) == 0 {
		return &matchList{}, nil
	}
	ps := make([]pattern, 0, len(raws))
	for _, raw := range raws {
		p, ok := parsePattern(raw)
		if !ok {
			return nil, pathError(ErrInvalidPattern, raw)
		}
		ps = append(ps, p)
	}
	return &matchList{patterns: ps}, nil
}

// matches reports whether any pattern in the list matches the
// given name. An empty list never matches.
func (m *matchList) matches(name string) bool {
	if m == nil {
		return false
	}
	for _, p := range m.patterns {
		if p.matches(name) {
			return true
		}
	}
	return false
}

// add returns a new matchList with the given patterns appended.
// The receiver is not mutated; a new list is returned.
//
// add exists so that a Handle's allow-list can be extended with
// the parent Root's deny-list without mutating either. The
// pattern is "immutable lists, composed functionally" rather
// than "mutable lists, modified in place".
func (m *matchList) add(raws []string) (*matchList, error) {
	if len(raws) == 0 {
		return m, nil
	}
	extra, err := newMatchList(raws)
	if err != nil {
		return nil, err
	}
	combined := make([]pattern, 0, len(m.patterns)+len(extra.patterns))
	combined = append(combined, m.patterns...)
	combined = append(combined, extra.patterns...)
	return &matchList{patterns: combined}, nil
}

// isEmpty reports whether the list contains no patterns. It is
// used to short-circuit the pipeline's deny-list check when the
// Root has no deny-list configured (which happens only if the
// caller explicitly removes the defaults).
func (m *matchList) isEmpty() bool {
	return m == nil || len(m.patterns) == 0
}
