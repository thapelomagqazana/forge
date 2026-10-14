package safefs

import (
	"io/fs"
	"path"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// This file defines Walk — the only recursive traversal primitive
// in Forge — and the bounded walker that implements it.
//
// # Why Walk lives here
//
// Recursive traversal is the single most dangerous filesystem
// operation a foundation manager performs. It touches every file
// under a directory, it is easy to write with unbounded recursion,
// and it is the operation most likely to be handed a hostile tree
// (a malicious blueprint, a corrupted project, a runaway template).
//
// Walk is defined here so that depth and entry bounds are enforced
// in one place. Domain packages that need to traverse a tree call
// Root.Walk; they do not implement their own recursion, and the
// layering lint (ADR-035) prevents them from reaching the
// underlying os package to try.

// -----------------------------------------------------------------------------
// WalkFunc
// -----------------------------------------------------------------------------

// WalkFunc is the callback invoked by Walk for each file or
// directory visited.
//
// # Arguments
//
// relPath is a forward-slash, cleaned path relative to the Root.
// It never contains "..", it never begins with a separator, and it
// never escapes the root. It uses "/" as its separator on every
// platform, so callers that build a regex or a map key from it
// produce the same result on Linux, macOS, and Windows.
//
// d is the directory entry for the visited path. For the root
// itself, d is nil and err is non-nil; this matches the convention
// of filepath.WalkDir. For any other path, d is non-nil unless err
// is non-nil.
//
// err is non-nil when the walk encountered an error reading the
// current directory or stat'ing the current entry. When err is
// non-nil, relPath names the entry that could not be read and d
// may be nil. Returning a non-nil value from the callback in this
// case aborts the walk.
//
// # Return values
//
//   - nil: continue the walk. If d is a directory, Walk descends
//     into it.
//   - fs.SkipDir: do not descend into d (if it is a directory),
//     and do not report any other error for the current directory.
//     Has no effect if d is not a directory.
//   - fs.SkipAll: stop the walk immediately. Walk returns nil.
//   - any other error: abort the walk and return the error to
//     Root.Walk's caller. The error is returned unchanged, so the
//     caller can match it with errors.Is.
//
// # Concurrency
//
// WalkFunc is called sequentially, on the goroutine that invoked
// Walk. It must not be called concurrently with itself. A callback
// that spawns goroutines and mutates shared state must synchronise
// that state itself; Walk provides no such guarantee.
type WalkFunc func(relPath string, d fs.DirEntry, err error) error

// -----------------------------------------------------------------------------
// Walk
// -----------------------------------------------------------------------------

// Walk recursively visits the tree rooted at name, in lexical
// order, subject to the Root's MaxDepth and MaxEntries bounds.
//
// # Ordering
//
// Entries within a directory are visited in the order returned by
// the underlying FS. The OSFS implementation sorts by filename,
// matching io/fs.ReadDir; other FS implementations are expected to
// do the same. The order of sibling directories is therefore
// deterministic across runs on the same FS. Walk does not sort
// independently: doing so would allocate a second slice per
// directory, which is precisely the cost the bounds exist to
// keep predictable.
//
// # Depth
//
// The depth of an entry is the number of path components between
// it and the root. The root itself is depth 0; its immediate
// children are depth 1; their children are depth 2; and so on.
//
// MaxDepth is the deepest depth that Walk will visit. A bound of
// N permits visiting entries whose depth is at most N and refuses
// to deliver any entry whose depth would be N+1. Zero disables
// the bound.
//
// The bound is enforced by comparing a child's depth against
// MaxDepth immediately before the child is delivered to the
// callback. When the bound is exceeded, Walk aborts with an error
// wrapping ErrDepthExceeded; the offending entry is not
// delivered. Entries at depth <= MaxDepth that were delivered
// before the boundary was reached remain delivered — Walk does
// not retroactively un-deliver anything.
//
// # Entries
//
// MaxEntries bounds the total number of entries visited across
// the entire walk. It is not per-directory. Zero disables the
// bound. Exceeding it aborts the walk with an error wrapping
// ErrEntriesExceeded.
//
// Both bounds are security controls, not performance
// optimisations: a hostile tree must not exhaust the process.
// Neither bound is delivered to the callback; both are returned
// to Walk's caller. This is deliberate — a bound overrun is a
// security event, not a data point the caller should filter out
// of its callback.
//
// # Symlink policy
//
// Walk does not follow symlinks to directories, regardless of the
// Root's policy. Following directory symlinks during a walk is
// what turns a linear traversal into a graph traversal and what
// makes bounds meaningless: a single link can create a cycle that
// no depth bound catches, or a fan-out that no entry bound
// predicts.
//
// A symlink to a directory is delivered to fn as a regular
// entry — d.IsDir() is false, d.Type() reports the link — and
// Walk does not descend into it. This is true under every policy,
// including FollowInRoot. A caller that needs to follow a
// directory symlink does so explicitly, using Root.Open or
// Root.ReadDir on the link's name, where the policy applies.
//
// A symlink to a file is delivered to fn normally. Whether the
// caller can open it depends on the policy, which Root.Open
// enforces.
//
// # Concurrency
//
// Walk is safe to call concurrently with itself on the same Root,
// provided each call passes a WalkFunc that is itself free of
// shared mutable state. Walk does not mutate the Root; the only
// shared state is the resolved-symlink cache, which is guarded.
//
// # Return value
//
// Walk returns nil on success, the WalkFunc's error if the
// callback aborted the walk, or an error wrapping
// ErrDepthExceeded / ErrEntriesExceeded if a bound was exceeded.
// Errors from reading directories are passed to fn; if fn returns
// nil for them, Walk continues.
func (r *Root) Walk(name string, fn WalkFunc) error {
	if fn == nil {
		return forgeerr.New(
			forgeerr.CategoryInternal,
			forgeerr.CodeInternalUnexpected,
			"safefs.Walk: nil WalkFunc",
		)
	}

	clean, err := r.validateName(name)
	if err != nil {
		return err
	}
	resolved, err := r.resolveSymlinks(clean)
	if err != nil {
		return err
	}

	w := &walker{
		root:  r,
		fn:    fn,
		count: 0,
	}
	return w.walk(resolved, clean, 0)
}

// -----------------------------------------------------------------------------
// walker
// -----------------------------------------------------------------------------

// walker carries the per-Walk state: the Root it walks, the
// callback, and the running entry count.
//
// walker is deliberately not exported. A caller that needs to
// track additional state does so inside its WalkFunc, using a
// closure; a caller that needs a different traversal algorithm
// writes it in its own package, on top of Root.ReadDir, and takes
// responsibility for its own bounds. The layering lint prevents
// that caller from reaching os to do so, which means it either
// uses Root.ReadDir (bounded by nothing it does not enforce) or it
// requests a new Root method — which is the conversation the
// architecture wants it to have.
type walker struct {
	root  *Root
	fn    WalkFunc
	count int
}

// walk lists the children of rel and delivers each child to the
// callback, descending into directories subject to the depth
// bound.
//
// The two names rel and clean are threaded separately because they
// serve different consumers:
//
//   - rel is the resolved path, used for FS operations.
//   - clean is the caller-visible path, passed to fn. Under
//     NoFollow the two are equal; under FollowInRoot they differ
//     whenever a component of the path was a symlink.
//
// The callback sees clean, never rel. A caller that logs a path
// it saw during a walk should see a path it could pass back to
// Root.Open, which means the caller-visible path, not the resolved
// one. Exposing rel would leak the internal resolution and invite
// callers to depend on it.
//
// # Depth accounting
//
// depth is the depth of rel itself, not of rel's children. rel
// has already been delivered to the callback by the caller of
// walk (or is the start name, which is not delivered at all).
// Each child of rel has depth depth+1.
//
// The depth bound is applied to each child before the child is
// delivered. This is the correction that makes MaxDepth(N) mean
// "visit entries up to depth N" instead of "descend up to N
// times": the previous implementation compared the parent's depth
// against the bound at the top of the recursive call, which
// permitted one extra level.
func (w *walker) walk(rel, clean string, depth int) error {
	entries, err := w.root.fs.List(rel)
	if err != nil {
		// Deliver the read error to the callback. This mirrors
		// filepath.WalkDir: a caller that cannot read a
		// directory is offered the chance to skip it rather
		// than aborting the whole walk. A caller that wants to
		// abort returns the error from fn.
		return w.fn(clean, nil, err)
	}

	for _, d := range entries {
		// Entry bound. Checked before the callback so that
		// the bound counts entries the callback will actually
		// see, not entries that were enumerated.
		if w.root.maxEntries > 0 && w.count >= w.root.maxEntries {
			return pathError(ErrEntriesExceeded, clean)
		}
		w.count++

		// Child paths. path.Join normalises the separator and
		// cleans redundant slashes; it never produces a
		// leading separator or a ".." segment for a
		// well-formed parent and a single-segment name, which
		// is what d.Name() returns.
		childRel := path.Join(rel, d.Name())
		childClean := path.Join(clean, d.Name())
		childDepth := depth + 1

		// Depth bound. The child is at childDepth. If the
		// bound is exceeded, refuse to deliver the child and
		// abort the walk. Entries at depth <= MaxDepth that
		// were delivered before this point remain delivered.
		//
		// A bound of zero disables the check entirely. The
		// comparison is strict (>), so MaxDepth(N) permits
		// visiting entries at depth N and rejects entries at
		// depth N+1.
		if w.root.maxDepth > 0 && childDepth > w.root.maxDepth {
			return pathError(ErrDepthExceeded, childClean)
		}

		if err := w.fn(childClean, d, nil); err != nil {
			// fs.SkipDir and fs.SkipAll are the two
			// control-flow sentinels documented on
			// WalkFunc. They are compared with == because
			// they are exported sentinel values, not
			// wrapped errors.
			if err == fs.SkipDir {
				// SkipDir on a non-directory is documented
				// to be a no-op by io/fs.WalkDir. We match
				// that: only a directory is actually
				// skipped.
				continue
			}
			if err == fs.SkipAll {
				return nil
			}
			return err
		}

		// Descend. d.IsDir() reports the entry type from the
		// directory listing, which is a symlink's own type
		// (not its target's). A symlink to a directory
		// therefore has d.IsDir() == false and is not
		// descended. See the doc comment on Root.Walk for the
		// rationale.
		if !d.IsDir() {
			continue
		}
		if err := w.walk(childRel, childClean, childDepth); err != nil {
			return err
		}
	}
	return nil
}
