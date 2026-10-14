package safefs

import (
	"strings"
)

// This file implements the boundary-enforcement pipeline that
// every Root and Handle operation runs before touching the
// filesystem. It is the security-relevant core of the safefs
// package: every other method delegates to it, and every other
// file assumes its invariants hold on entry.
//
// # The pipeline
//
// The pipeline has six steps, in this order. The order is
// load-bearing and is documented in
// docs/reference/boundary-model.md, "The Boundary Decision
// Tree". A change to the order is a change to which sentinel a
// caller receives when a name fails multiple checks, so it is a
// deliberate change that the sample tests catch.
//
//  1. validateName(name) — normalise the name (backslash to
//     forward slash, path.Clean), reject empty names, reject
//     names that are not local per filepath.IsLocal, and
//     classify the rejection as ErrNotLocal (malformed) or
//     ErrPathTraversal (attempted escape).
//
//  2. Lexical escape check — this is folded into step 1:
//     filepath.IsLocal rejects a name whose cleaned form begins
//     with "..". The check is pure string work, performed before
//     any syscall.
//
//  3. Deny-list check — reject any name that matches the Root's
//     effective deny-list. The deny-list is a set of path
//     patterns that identify paths no component may write to
//     (for example, .git/). See boundary.go for the pattern
//     syntax and DefaultDenyPatterns for the defaults.
//
//  4. Allow-list check — if the Root was constructed with an
//     allow-list, reject any name that does not match it. The
//     deny-list takes precedence: a name that matches both is
//     reported as ErrDenyList, not ErrNotAllowed.
//
//  5. resolveSymlinks(name) — apply the Root's symlink policy to
//     every component of the name. This is the TOCTOU defence: a
//     lexical check followed by a syscall is racy against an
//     attacker who can swap a directory for a symlink between
//     the two. Resolving the name's components before the
//     syscall narrows the window.
//
//  6. Delegate — the resolved name is returned to the caller,
//     which passes it to the underlying FS.
//
// # Why the deny-list and allow-list checks come before the
// # symlink resolution
//
// Both checks are pure string predicates on the cleaned name.
// They run before the symlink resolution so that a name they
// reject never triggers a syscall. The saving is small (one
// Lstat per component of the parent) but the property is worth
// stating: a name that fails a purely-lexical check never
// reaches the filesystem.
//
// # Composition with the symlink policy
//
// The pipeline's step 5 resolves every component of the name
// under the Root's policy. The Root's policy is also applied to
// the final component by the operation that reads or writes it
// (via the same resolveSymlinks, called from the operation).
// The two checks compose:
//
//   - Under NoFollow, no component of the name may be a
//     symlink.
//   - Under FollowInRoot, a component that is a symlink is
//     resolved and its target is checked against the root. A
//     chain that escapes is denied.
//
// The composition is what makes the enforcement uniform: every
// component of the path is subject to the same policy, and a
// single missed component would be a hole.

// resolveForIO runs the boundary-enforcement pipeline on name
// and returns the name the caller should hand to the underlying
// FS.
//
// # The full pipeline
//
//  1. validateName normalises the name and rejects lexical
//     escape. Pure string work.
//
//  2. The lexical escape check (folded into step 1) rejects
//     names that begin with ".." after cleaning.
//
//  3. The deny-list check rejects names that match the Root's
//     effective deny-list.
//
//  4. The allow-list check rejects names that do not match the
//     Root's allow-list, if the Root has one. The deny-list
//     takes precedence: a name that fails both is reported as
//     ErrDenyList.
//
//  5. resolveSymlinks applies the Root's symlink policy to every
//     component of the name.
//
//  6. Delegate — the resolved name is returned to the caller.
//
// # Errors
//
// The pipeline returns errors wrapping one of:
//
//   - ErrNotLocal — the name is malformed (empty, absolute,
//     contains a Windows drive letter, or is otherwise rejected
//     by filepath.IsLocal without a ".." segment).
//
//   - ErrPathTraversal — the name attempts to lexically escape.
//
//   - ErrDenyList — the name matches the Root's deny-list.
//
//   - ErrNotAllowed — the Root has an allow-list and the name
//     does not match it.
//
//   - ErrSymlinkDenied — a symlink on the name is refused by
//     the policy, or a resolved target escapes the root.
//
// # Concurrency
//
// resolveForIO is a method on *Root and is safe for concurrent
// use. The only mutable state it touches is the resolved-symlink
// cache, which is guarded by Root.mu. The deny-list and
// allow-list match lists are immutable after Root construction
// and are read without synchronisation.
func (r *Root) resolveForIO(name string) (string, error) {
	// Steps 1 & 2: validateName normalises the name and rejects
	// lexical escape. The rejection is fast: pure string work,
	// no syscall.
	clean, err := r.validateName(name)
	if err != nil {
		return "", err
	}

	// Step 3: deny-list. Runs before the allow-list so that a
	// name matching both is reported as ErrDenyList, which is
	// the more specific diagnostic. Runs before the symlink
	// resolution so that a denied name never triggers a
	// syscall.
	//
	// r.deny is never nil after New returns successfully; an
	// empty deny-list is represented as a matchList whose
	// patterns slice is empty, and matches returns false for
	// every name.
	if r.deny.matches(clean) {
		return "", pathError(ErrDenyList, clean)
	}

	// Step 4: allow-list. Runs only if the Root has one. A
	// Root without an allow-list permits every name that
	// passes the boundary and deny-list checks.
	//
	// The nil check is deliberate and is the distinction
	// between "no allow-list configured" and "an empty
	// allow-list configured" (the latter rejects every name).
	if r.allow != nil && !r.allow.matches(clean) {
		return "", pathError(ErrNotAllowed, clean)
	}

	// Step 5: apply the Root's symlink policy to every
	// component of the name. This is what makes NoFollow refuse
	// a name whose final component is a link, and what makes
	// FollowInRoot resolve the name under the root.
	//
	// resolveSymlinks consults the resolved-symlink cache for
	// the FollowInRoot path, so repeated operations on the same
	// name do not re-resolve.
	resolved, err := r.resolveSymlinks(clean)
	if err != nil {
		return "", err
	}

	// Belt-and-braces. resolveSymlinks already enforces that
	// the resolved name is inside the root; the assertion here
	// catches a future change that weakened the invariant
	// without updating this function.
	if !isInside(resolved) {
		return "", pathError(ErrSymlinkDenied, clean)
	}

	// Step 6: return the resolved name. The caller delegates to
	// the underlying FS.
	return resolved, nil
}

// resolveForIOHandle runs the pipeline for a Handle operation.
//
// # Order of operations
//
// The order matters:
//
//  1. Validate the caller's name *before* prepending the
//     sub-path. Validating after prepending would let an
//     absolute name be absorbed by the concatenation: the name
//     "/etc/passwd" prepended with "src" becomes
//     "src//etc/passwd", which path.Clean normalises to
//     "src/etc/passwd" — a legal local name. The leading slash
//     is what makes the original name absolute, and the
//     concatenation destroys it before any check can see it.
//
//  2. Prepend the sub-path to the *validated* name.
//
//  3. Run the parent Root's pipeline on the result. The
//     sub-path was validated at Handle construction, and the
//     caller's name was validated in step 1, so the joined
//     result is guaranteed to be a local name under the root's
//     boundary by construction. resolveForIO still runs the
//     full pipeline because the deny-list and allow-list
//     checks are on the joined name (the deny-list is defined
//     relative to the Root, not to the Handle), and because
//     the parent resolution step is not a lexical check — it
//     resolves symlinks on the path and must run on every
//     operation.
//
//  4. Enforce the sub-boundary on the resolved name.
//
// # Why the deny-list applies to the joined name
//
// The deny-list is a property of the Root, not of the Handle.
// A Handle constructed at "src" inherits the Root's deny-list;
// a name like "src/.git/config" is rejected because the pattern
// ".git" matches the ".git" segment of the joined name. A
// pattern that the Root's deny-list would reject is rejected
// whether the caller reaches it through the Root or through a
// Handle.
//
// # Why the allow-list applies to the joined name
//
// The allow-list is likewise defined relative to the Root. A
// Handle constructed at "src" with an allow-list of "src/**"
// permits "src/a.go" and rejects "other/a.go". The joining is
// what makes the allow-list's patterns meaningful: they are
// written in the Root's namespace, not in the Handle's.
func (h *Handle) resolveForIOHandle(name string) (string, error) {
	// Step 1: validate the caller's name in isolation.
	//
	// The empty-name case is handled by validateName itself.
	// The "." case is treated specially because it names the
	// handle's own directory and must not be prepended with
	// the sub-path twice.
	if name != "." {
		clean, err := h.root.validateName(name)
		if err != nil {
			return "", err
		}
		name = clean
	}

	// Step 2: prepend the sub-path. The sub-path was validated
	// at construction and is stored in cleaned form.
	full := h.sub
	switch {
	case full == "":
		full = name
	case name == ".":
		// The caller named the handle's own directory.
		// The sub-path is the full name.
	default:
		full = full + "/" + name
	}

	// Step 3: run the parent Root's pipeline. This includes
	// the deny-list and allow-list checks on the joined name.
	resolved, err := h.root.resolveForIO(full)
	if err != nil {
		return "", err
	}

	// Step 4: enforce the sub-boundary on the resolved name.
	if !isUnder(resolved, h.sub) {
		return "", pathError(ErrSubBoundary, name)
	}

	return resolved, nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// splitParent splits a cleaned name into its parent and base
// components. The parent is everything before the last "/", and
// the base is the last component. A name with no "/" has an
// empty parent and the whole name as its base.
func splitParent(name string) (parent, base string) {
	i := strings.LastIndex(name, "/")
	if i < 0 {
		return "", name
	}
	return name[:i], name[i+1:]
}

// joinClean joins a parent and a base with a single separator.
// The result is not re-cleaned; the caller is expected to have
// passed cleaned inputs, and a re-clean would silently accept a
// name that the pipeline should have rejected.
func joinClean(parent, base string) string {
	if parent == "" {
		return base
	}
	return parent + "/" + base
}

// isInside reports whether the resolved name is a legal name
// relative to the root.
//
// The predicate is filepath.IsLocal, applied to the cleaned
// name. The name produced by resolveSymlinks is already in the
// canonical form, so IsLocal is the right check.
//
// # Why not a prefix comparison
//
// "/root" is a prefix of "/root-evil". A string-prefix check on
// the resolved absolute path is the classic CWE-22 mistake. The
// predicate here is on the *relative* form, which is what the
// pipeline operates on.
//
// # Why not filepath.IsAbs
//
// IsAbs rejects absolute paths, but the pipeline's names are
// already relative. The dangerous cases are relative names that
// begin with "..", which IsLocal rejects and IsAbs does not.
func isInside(resolved string) bool {
	if resolved == "." || resolved == "" {
		return true
	}
	for _, seg := range strings.Split(resolved, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// isUnder reports whether resolved is under sub, where both are
// cleaned forward-slash relative names.
//
// # Why a segment-prefix check and not a string-prefix check
//
// "src-evil" has "src" as a string prefix but is not under
// "src". The segment check splits both names and compares the
// segments, so "src-evil/x" is not under "src" and "src/x" is.
//
// The check is on the *resolved* form, so a symlink that
// resolves out of the sub-boundary is rejected.
func isUnder(resolved, sub string) bool {
	if sub == "" || sub == "." {
		return true
	}
	if resolved == sub {
		return true
	}
	return strings.HasPrefix(resolved, sub+"/")
}
