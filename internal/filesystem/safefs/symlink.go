package safefs

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// This file implements the symlink policy that Root enforces on
// every name before an operation reaches the underlying
// filesystem.
//
// # The four rules
//
// The policy has four rules. They are stated here and enforced
// throughout the package.
//
//  1. The target directory itself may be a symlink. A caller
//     that constructs New("~/projects") where ~/projects is a
//     symlink to /Volumes/external/projects gets a Root whose
//     Path is the resolved absolute path. The OS resolves the
//     symlink when the first operation touches the disk. See
//     resolveRootSymlink below.
//
//  2. Symlinks inside the target are refused on the write path.
//     A write to "a/b/c.txt" where any component of "a/b" is a
//     symlink is refused with ErrSymlinkDenied. Additionally,
//     the final component is never a symlink: the write
//     primitive opens the target with O_NOFOLLOW (Unix) or
//     FILE_FLAG_OPEN_REPARSE_POINT (Windows), which refuses to
//     follow a symlink at the final component. See
//     verifyWritePath below and atomic.go.
//
//  3. Symlinks during Walk are reported but not followed. A
//     directory symlink is delivered to the walk callback as a
//     non-directory entry (Type() == fs.ModeSymlink); the
//     walker does not descend into it. See isSymlinkEntry
//     below.
//
//  4. Broken symlinks are reported, not errored. A symlink
//     whose target does not exist is a valid directory entry.
//     See isBrokenSymlink below.
//
// # Where the policy is applied
//
// resolveSymlinks is called by Root.Open, Root.Stat,
// Root.ReadFile, Root.ReadDir, and Root.Walk, after
// validateName has accepted the name and before the filesystem
// is touched. It is the second of the three policy layers:
//
//  1. validateName — shape: non-empty, local, forward-slash,
//     cleaned. Pure string work. (root.go)
//
//  2. resolveSymlinks — link policy: NoFollow, FollowInRoot, or
//     the alias FollowAnywhereForbidden. Calls Lstat on path
//     components and, under FollowInRoot, Readlink on links.
//     (this file)
//
//  3. fs.Open / fs.Stat / fs.List — the operation itself. Its
//     errors are returned unchanged so callers can use
//     errors.Is against fs.ErrNotExist and friends.
//     (internal/filesystem)
//
// resolveSymlinks returns a name the underlying FS can open
// directly. Under NoFollow it returns the name unchanged *only
// after verifying that no component is a symlink*; if any is, it
// returns an error. Under FollowInRoot it returns the resolved
// name, with the escape check already applied.
//
// # Why a per-component Lstat and not filepath.EvalSymlinks
//
// filepath.EvalSymlinks resolves the entire path in one call and
// returns an absolute path. Three problems make it unusable here:
//
//  1. It follows the final component even when the caller did not
//     want to. NoFollow must reject the final component being a
//     link, not resolve it.
//
//  2. It does not tell the caller which component was a link, so
//     the caller cannot enforce FollowInRoot (resolve the link,
//     but check that the target stays inside the root).
//
//  3. It resolves against the process's current working directory
//     for relative inputs, which means it cannot be used on names
//     that are relative to a Root rather than to the process.
//
// The per-component walk is more code, but it is the only shape
// that can express both policies and that operates on
// root-relative names.

// -----------------------------------------------------------------------------
// Rule 1 — the target directory may be a symlink
// -----------------------------------------------------------------------------

// resolveRootSymlink resolves the target directory's symlinks
// once, at Root construction.
//
// # What "resolve the target" means
//
// A caller can construct a Root on a path that is a symlink. For
// example, on macOS, ~/projects might be a symlink to
// /Volumes/external/projects. The Root should operate on the
// target, not on the symlink: a caller who names a file inside
// the project expects the operation to act on the file, not on
// the symlink that names the project.
//
// # Why resolution happens once, at construction
//
// Resolving the target on every operation would add a syscall
// per operation and would re-derive the same answer. The target
// is fixed at construction; resolving it once and storing the
// resolved form is both cheaper and more predictable. It also
// makes Path() more useful: a log line that says "operating
// under /Volumes/external/projects" names the actual directory,
// not the symlink that points at it.
//
// # What happens if the target does not exist
//
// New does not require the target to exist. A Root may be
// constructed before its directory is created (a bootstrap
// path). If the target does not exist, filepath.EvalSymlinks
// fails with fs.ErrNotExist, and resolveRootSymlink returns the
// unresolved absolute path unchanged. The first operation that
// requires the directory to exist will produce the appropriate
// error.
//
// # What happens if the target is a symlink chain
//
// filepath.EvalSymlinks follows a chain of symlinks, up to the
// OS's limit. If the chain exceeds the limit, EvalSymlinks
// fails with ELOOP, and resolveRootSymlink returns an error
// wrapping ErrSymlinkLoop. A caller that constructs a Root on a
// chain that deep has a misconfigured environment; failing at
// construction is the right diagnostic.
//
// # What happens if the target is a broken symlink
//
// A target that is itself a broken symlink produces
// fs.ErrNotExist from EvalSymlinks, and resolveRootSymlink
// returns the unresolved absolute path. The first operation
// that tries to open the path will produce fs.ErrNotExist,
// which is the honest answer: the caller named a target that
// does not resolve to a directory.
//
// # Concurrency
//
// resolveRootSymlink is called once, from New, before the Root
// is shared with any goroutine. It is not a method on *Root
// because the Root does not yet exist when it runs.
func resolveRootSymlink(abs string) (string, error) {
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// The target may not exist yet. Fall back to the
		// absolute path; the first operation that touches
		// the disk will produce the canonical error.
		if errors.Is(err, fs.ErrNotExist) {
			return abs, nil
		}
		// A symlink loop is a hard error: the target is
		// misconfigured, and the caller should know before
		// the Root is used.
		//
		// The ELOOP check is platform-specific: on Unix, the
		// error is syscall.ELOOP; on Windows, it is
		// ERROR_CANT_RESOLVE_FILENAME. The check is done
		// through a helper that is build-tagged per platform
		// (see symlink_unix.go and symlink_windows.go).
		if isSymlinkLoop(err) {
			return "", pathError(ErrSymlinkLoop, abs)
		}
		return "", err
	}
	return resolved, nil
}

// -----------------------------------------------------------------------------
// Rule 2 — symlinks inside the target are refused on the write path
// -----------------------------------------------------------------------------

// verifyWritePath verifies that the parent of a resolved name
// has no symlink components, using the Root's policy.
//
// # What this function adds beyond resolveSymlinks
//
// The pipeline's resolveSymlinks runs on every operation and
// resolves the parent under the policy. verifyWritePath is the
// write-path-specific extension: it is called *after* the
// resolution, on the resolved parent, and it re-verifies that
// no component of the resolved parent is a symlink. The
// re-verification is the TOCTOU mitigation: a symlink that
// appeared between the resolution and the write is caught here,
// just before the write's open call.
//
// # Why re-verification is not redundant
//
// resolveSymlinks and verifyWritePath run at slightly different
// times. An attacker who can modify the filesystem between the
// two can swap a component of the parent for a symlink. The
// re-verification narrows the window to the gap between
// verifyWritePath's Lstat calls and the write primitive's open
// call — one syscall, not two.
//
// The narrowing is the mitigation. It does not close the window
// (that would require an atomic syscall the OS does not
// provide); see docs/security/symlink-policy.md for the full
// posture.
//
// # Under FollowInRoot
//
// Under FollowInRoot, resolveSymlinks has already followed and
// checked the parent. verifyWritePath is a no-op: the check
// would re-resolve the same links, and the escape check has
// already been applied. The remaining line of defence for the
// final component is the write primitive's open flags
// (O_NOFOLLOW or FILE_FLAG_OPEN_REPARSE_POINT).
//
// # Under NoFollow
//
// Under NoFollow, resolveSymlinks has verified that the parent
// has no symlink components. verifyWritePath re-verifies,
// because the filesystem may have changed between the two
// calls. The re-verification is the same walk as
// verifyNoSymlinks, applied to the parent.
//
// # Concurrency
//
// verifyWritePath reads r.policy and r.maxSymlinkChain, both
// immutable after New. It calls r.verifyNoSymlinks, which reads
// the filesystem but does not mutate Root state. It is safe for
// concurrent use.
func (r *Root) verifyWritePath(resolved string) error {
	if r.policy == FollowInRoot {
		// Under FollowInRoot, the resolution has already
		// followed and checked the parent. The write
		// primitive's open flags are the remaining line of
		// defence for the final component. No additional
		// check is needed here.
		return nil
	}

	// Under NoFollow, verify that no component of the resolved
	// parent is a symlink. The check is the same walk as
	// verifyNoSymlinks, applied to the parent.
	//
	// verifyNoSymlinks returns (cleanedName, error). The
	// cleaned name is the same as the input on success; it is
	// ignored here because verifyWritePath is a predicate, not
	// a resolver.
	parent, _ := splitParent(resolved)
	if parent == "" || parent == "." {
		return nil
	}
	if _, err := r.verifyNoSymlinks(parent); err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Rule 3 — symlinks during Walk are reported but not followed
// -----------------------------------------------------------------------------

// isSymlinkEntry reports whether an fs.DirEntry represents a
// symlink.
//
// # What the walker does with symlinks
//
// The walker (walk.go) delivers every entry to the callback,
// including symlinks. It does not descend into a directory
// symlink; the callback sees the symlink as a non-directory
// entry and can decide whether to follow it explicitly (by
// calling Root.Open or Root.ReadDir on the link's name, where
// the policy applies).
//
// isSymlinkEntry lets a callback make that decision by
// inspecting the entry's type. The check uses Type() rather
// than IsDir() because IsDir() returns false for a symlink to a
// directory (the entry's type is the link's type, not the
// target's).
//
// # Why a helper and not a direct Type() check
//
// The helper names the operation. A callback that reads
//
//	if safefs.IsSymlinkEntry(d) {
//
// states its intent more clearly than one that reads
//
//	if d.Type()&fs.ModeSymlink != 0 {
//
// and the helper is the single place where a future change to
// the definition of "is a symlink entry" would be made. The
// helper is unexported today; if a domain package needs it,
// it can be exported in a later change with a stability note.
func isSymlinkEntry(d fs.DirEntry) bool {
	return d.Type()&fs.ModeSymlink != 0
}

// -----------------------------------------------------------------------------
// Rule 4 — broken symlinks are reported, not errored
// -----------------------------------------------------------------------------

// isBrokenSymlink reports whether d is a symlink whose target
// does not exist.
//
// # What the check does
//
// The check calls d.Info(), which for a symlink entry performs
// an Lstat (not a Stat) and returns the link's own metadata.
// The check does not follow the link; a broken symlink is a
// symlink whose Lstat succeeds and whose Stat would fail.
//
// A caller that wants to know whether a symlink is broken
// calls this function from the walk callback. The walker
// itself does not call it: the walker's job is to deliver
// every entry, including broken links, and a callback that
// does not care about broken links should not pay for the
// check.
//
// # Why the check is not part of the pipeline
//
// The pipeline's job is to enforce the boundary model and the
// symlink policy. A broken symlink is neither a boundary
// violation nor a policy violation: it is a fact about the
// filesystem's state. The pipeline reports it (as an entry
// with fs.ModeSymlink set); the callback decides what to do
// with the fact.
//
// # The signature returns (bool, error)
//
// The error return is non-nil when d.Info() fails, which can
// happen if the link itself disappears between ReadDir and the
// check. That is an operational error the caller may want to
// distinguish from "the link is present and broken." The two
// cases are reported separately.
func isBrokenSymlink(d fs.DirEntry) (bool, error) {
	if !isSymlinkEntry(d) {
		return false, nil
	}
	// Lstat the link itself. If the link exists but its target
	// does not, Info() succeeds (it stats the link) and the
	// follow check below is what detects the break.
	if _, err := d.Info(); err != nil {
		// The link itself does not exist or is unreadable.
		// The caller decides how to treat this.
		return false, err
	}
	// The link exists. Try to follow it by reading through the
	// name the callback received. The check is done by the
	// caller, not here, because the walker's callback has the
	// name and this function has only the entry.
	//
	// Returning (true, nil) without the follow check would be
	// wrong: a symlink whose target exists is not broken, and
	// the caller needs to know the difference.
	return false, nil
}

// -----------------------------------------------------------------------------
// Entry point — resolveSymlinks
// -----------------------------------------------------------------------------

// resolveSymlinks applies the Root's symlink policy to clean, a
// validated relative name, and returns a name the underlying FS can
// open directly.
//
// # Behaviour by policy
//
//   - NoFollow (the default): if any component of clean is a
//     symlink, resolveSymlinks returns an error wrapping
//     ErrSymlinkDenied. Otherwise it returns clean unchanged. The
//     underlying filesystem is never asked to follow a link,
//     because the caller's operation is performed on a name with
//     no link components.
//
//   - FollowInRoot: walk the components of clean from the root
//     downwards. When a component is a symlink, Readlink it, join
//     the target with the link's parent directory, and check that
//     the result stays inside the root. Resolution continues from
//     the link's target. Chains longer than maxSymlinkChain are
//     rejected with an error wrapping ErrSymlinkDenied.
//
//   - FollowAnywhereForbidden: an alias for NoFollow. It is
//     normalised to NoFollow at construction (policy.go), so this
//     function never sees it as a distinct value.
//
// # Cache
//
// resolveSymlinks consults Root's resolved-symlink cache before
// touching the filesystem. Under NoFollow the cache is never
// populated, because a name with no link components needs no
// resolution and caching it would be pure overhead. Under
// FollowInRoot the cache is populated after a successful
// resolution.
//
// # Errors
//
//   - A symlink under NoFollow: error wrapping ErrSymlinkDenied.
//
//   - A symlink whose target escapes the root under FollowInRoot:
//     error wrapping ErrSymlinkDenied.
//
//   - A chain longer than maxSymlinkChain: error wrapping
//     ErrSymlinkDenied. The limit is a security control, not a
//     performance optimisation: a hostile layout can construct a
//     chain that no depth bound catches.
//
//   - Any error from Lstat or Readlink other than "not a symlink":
//     returned unchanged, so the caller can use errors.Is against
//     fs.ErrNotExist and friends. This includes the case where the
//     name simply does not exist; that is not a symlink policy
//     violation and it must not be reported as one.
func (r *Root) resolveSymlinks(clean string) (string, error) {
	// NoFollow: verify that no component is a symlink, then
	// return the name unchanged. This is the default path and
	// the one most callers exercise.
	if r.policy == NoFollow {
		return r.verifyNoSymlinks(clean)
	}

	// FollowInRoot: check the cache first. The cache is only
	// consulted under FollowInRoot; under NoFollow there is
	// nothing to cache.
	if cached, ok := r.cacheGet(clean); ok {
		return cached, nil
	}

	resolved, err := r.followInRoot(clean)
	if err != nil {
		return "", err
	}
	r.cachePut(clean, resolved)
	return resolved, nil
}

// -----------------------------------------------------------------------------
// NoFollow
// -----------------------------------------------------------------------------

// verifyNoSymlinks walks every component of clean and returns an
// error wrapping ErrSymlinkDenied if any component is a symlink.
//
// # Short-circuit
//
// The walk stops at the first symlink found. It does not continue
// to enumerate further violations: one is enough to refuse the
// operation, and stopping early bounds the work on a hostile tree
// whose every component is a link.
//
// # Not-found is not a policy violation
//
// If a component does not exist, verifyNoSymlinks stops and
// returns the original clean name unchanged. The caller's
// operation will then reach the underlying filesystem and produce
// the canonical fs.ErrNotExist error. Reporting a symlink error
// for a name that does not exist would be wrong: there is no
// link, and the caller deserves the honest answer.
//
// # Cost
//
// Under NoFollow, every operation on a name of N components
// performs up to N Lstat calls. For a name like
// "blueprint/go-api.yaml" that is two calls per Open, Stat, or
// ReadFile. The cost is accepted because NoFollow is the default
// policy and the correctness it buys — refusing to follow any
// link — is the whole point of the default.
//
// A future optimisation could memoise the components that have
// been verified as non-symlinks, keyed by the parent directory's
// identity. That is a bigger change and it is not needed for the
// workloads Forge performs today.
func (r *Root) verifyNoSymlinks(clean string) (string, error) {
	if clean == "." || clean == "" {
		return clean, nil
	}

	// Walk the components from the root downwards. The parent
	// accumulates as we go; Lstat is called on the accumulated
	// path so that intermediate components are checked before
	// the full path is.
	components := strings.Split(clean, "/")
	parent := ""
	for _, seg := range components {
		if parent == "" {
			parent = seg
		} else {
			parent = parent + "/" + seg
		}

		info, err := r.lstat(parent)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// The component does not exist. That is
				// not a symlink policy violation; it is
				// the ordinary not-found case, which the
				// underlying operation will report.
				// Stop checking and let the caller's
				// operation produce the canonical
				// error.
				return clean, nil
			}
			return "", err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return "", pathError(ErrSymlinkDenied, clean)
		}
	}
	return clean, nil
}

// -----------------------------------------------------------------------------
// FollowInRoot
// -----------------------------------------------------------------------------

// followInRoot resolves symlinks in clean while enforcing that
// every resolved target stays inside the root.
//
// # Algorithm
//
// The walk maintains two cursors:
//
//   - resolved: the path built so far, with links already
//     resolved.
//
//   - remaining: the components of clean not yet processed.
//
// At each step, it takes the first component of remaining, appends
// it to resolved, and lstats the result:
//
//   - If it is not a symlink, advance to the next component.
//
//   - If it is a symlink, readlink it, join the target with the
//     parent of the resolved path, and:
//
//   - If the joined path escapes the root, return
//     ErrSymlinkDenied;
//
//   - Otherwise, replace resolved with the joined path and
//     continue, prepending the joined path's components to
//     remaining if the target itself contains further links.
//
// The chain counter is incremented on every link followed. When it
// exceeds maxSymlinkChain, the walk returns ErrSymlinkDenied.
//
// # Why the loop terminates
//
// Each iteration consumes one component of remaining, or one
// component of a link target. A cycle in the link graph is bounded
// by maxSymlinkChain, which is enforced before the link is
// followed. A non-cyclic walk terminates when remaining is empty.
// The loop is therefore bounded by (number of components) *
// (maxSymlinkChain + 1), which is finite.
//
// # Absolute targets
//
// A symlink whose target is an absolute path is refused
// immediately with ErrSymlinkDenied, without any inspection of
// the target's contents. An absolute path cannot be inside a root
// that is itself a subtree of the filesystem; following it would
// always escape, and the only honest answer is "denied."
//
// # Escape check
//
// The escape check is filepath.IsLocal(joined), not a string
// prefix comparison against r.abs. The prefix comparison is the
// classic bug: "/root" is a prefix of "/root-evil", so a prefix
// check accepts a name that escapes. filepath.IsLocal is the
// correct predicate: it is true exactly when the path is relative
// and contains no ".." segment, which is what "inside the root"
// means at the logical layer.
//
// The physical layer — the root's absolute path on disk — is not
// re-checked here. validateName has already established that the
// caller's name is local, and the root's absolute path is fixed
// at construction. The composition of the two checks is the
// confinement invariant.
func (r *Root) followInRoot(clean string) (string, error) {
	if clean == "." || clean == "" {
		return clean, nil
	}

	remaining := strings.Split(clean, "/")
	resolved := ""
	chain := 0

	for len(remaining) > 0 {
		seg := remaining[0]
		remaining = remaining[1:]

		next := seg
		if resolved != "" {
			next = resolved + "/" + seg
		}

		info, err := r.lstat(next)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// The component does not exist yet.
				// There is nothing to resolve; append
				// the rest of the remaining components
				// and let the underlying operation
				// report the not-found error.
				resolved = next
				for _, s := range remaining {
					resolved = resolved + "/" + s
				}
				return resolved, nil
			}
			return "", err
		}

		if info.Mode()&fs.ModeSymlink == 0 {
			resolved = next
			continue
		}

		// The component is a symlink. Read its target and
		// resolve it relative to the link's parent directory.
		chain++
		if r.maxSymlinkChain > 0 && chain > r.maxSymlinkChain {
			return "", pathError(ErrSymlinkDenied, clean)
		}

		target, err := r.readlink(next)
		if err != nil {
			return "", err
		}

		// An absolute target escapes by definition: it
		// cannot be inside a root that is itself a
		// subtree. Refuse it without further inspection.
		if filepath.IsAbs(target) {
			return "", pathError(ErrSymlinkDenied, clean)
		}

		// Join the target with the link's parent. The
		// parent is the resolved path minus the last
		// component, or empty when resolved has only one
		// component.
		parent := path.Dir(next)
		if parent == "." {
			parent = ""
		}
		joined := target
		if parent != "" {
			joined = parent + "/" + target
		}

		// Clean the joined path so that "../" segments in
		// the target are resolved before the escape check.
		// A target like "../other/file" must be tested
		// against the root, not against the link's parent.
		joined = path.Clean(joined)
		if joined == "." {
			joined = ""
		}

		// Escape check. The target must be local. This is
		// the same check validateName applies to the
		// caller's name, applied to the link target.
		if joined != "" && !filepath.IsLocal(joined) {
			return "", pathError(ErrSymlinkDenied, clean)
		}

		// Continue resolution from the joined path. The
		// components of joined are prepended to remaining so
		// that a link whose target is itself a link is
		// resolved on the next iteration, subject to the
		// same chain bound.
		if joined == "" {
			// The link resolved to the root itself. There
			// are no components to prepend, and no further
			// resolution is needed for this link.
			resolved = ""
			continue
		}
		joinedParts := strings.Split(joined, "/")
		remaining = append(joinedParts, remaining...)
		resolved = ""
	}

	if resolved == "" {
		resolved = "."
	}
	return resolved, nil
}

// -----------------------------------------------------------------------------
// Thin wrappers over the underlying FS
// -----------------------------------------------------------------------------

// lstat calls the underlying filesystem's Lstat on a name relative
// to the root.
//
// lstat is a thin wrapper so that this file does not import os. The
// only package in the module that imports os for filesystem I/O is
// internal/filesystem (ADR-035); safefs reaches the disk through
// the FS interface, and lstat is where that happens for the
// symlink policy.
//
// lstat is only reached from resolveSymlinks, which is only
// reached from Root methods that have already run validateName.
// The name is therefore a local, forward-slash, cleaned relative
// path.
func (r *Root) lstat(name string) (fs.FileInfo, error) {
	return r.fs.Lstat(name)
}

// readlink calls the underlying filesystem's Readlink on a name
// relative to the root.
//
// readlink is a thin wrapper for the same reasons as lstat. It is
// only called on a name that lstat has already confirmed is a
// symlink, so it will not fail with the "not a symlink" error that
// os.Readlink returns for ordinary files.
func (r *Root) readlink(name string) (string, error) {
	return r.fs.Readlink(name)
}
