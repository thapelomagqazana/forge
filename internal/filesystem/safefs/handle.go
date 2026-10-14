package safefs

import (
	"io"
	"io/fs"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// Handle is a scoped view of a Root, bound to a sub-path.
//
// # What a Handle is
//
// A Handle returned by Root.Rel("src") is a *safefs.Handle whose
// operations are equivalent to the parent's operations with
// "src/" prepended to the name, with one additional guarantee:
// the handle rejects any name that resolves outside "src",
// regardless of whether the name would be legal under the
// parent.
//
// # What a Handle is not
//
// It is not a second Root. It shares the parent's filesystem,
// policy, bounds, and resolved-symlink cache. Constructing one
// does not re-validate the subtree, does not open any
// descriptors, and does not create any resources that need
// cleanup.
//
// # Why a Handle rather than a Root
//
// A caller that wants to sandbox a component to a subtree has
// two options: construct a new Root rooted at the subtree, or
// construct a Handle on the parent. The two differ in three
// ways:
//
//   - A new Root re-validates its root from scratch. A Handle
//     validates only the sub-path, and the sub-path's
//     validation is cheap because it uses the parent's
//     already-validated root.
//
//   - A new Root has its own resolved-symlink cache. A
//     Handle shares the parent's, so a name that was already
//     resolved under the parent is not re-resolved under the
//     handle.
//
//   - A new Root has no memory of the parent's boundary. A
//     name that escapes the subtree but stays inside the
//     parent is legal for a new Root but rejected by a
//     Handle. The Handle's guarantee is strictly stronger.
//
// The last property is why a Handle is the right tool for
// sandboxing: it enforces the sub-boundary *in addition to* the
// parent's, rather than instead of it.
//
// # Concurrency and lifetime
//
// A Handle is safe for concurrent use by multiple goroutines.
// It holds a pointer to its parent Root, which is immutable
// after construction, and an immutable sub-path. It is valid
// for as long as its parent Root is valid.
type Handle struct {
	root *Root
	sub  string
}

// newHandle constructs a Handle for the named sub-path.
//
// # Validation
//
// The sub-path is validated immediately:
//
//  1. It must satisfy the name contract (see validateName).
//
//  2. It must pass the Root's pipeline. The pipeline runs
//     validateName, the deny-list check, the allow-list check,
//     and the symlink resolution. A sub-path that matches the
//     Root's deny-list is rejected with ErrDenyList; a
//     sub-path outside the Root's allow-list is rejected with
//     ErrNotAllowed; a sub-path whose parent contains a
//     symlink that escapes is rejected with ErrSymlinkDenied.
//
// A Handle's sub-path is empty by default, which represents the
// Root itself. A Handle with an empty sub-path is
// behaviourally equivalent to its parent Root, with the
// additional (empty) sub-boundary check as a no-op.
//
// # Why the pipeline call is the right shape
//
// The sub-path is subject to the same boundary model as every
// other name the Root sees. It is not privileged by virtue of
// being used to construct a Handle. The pipeline is the single
// place the model is applied, and every caller goes through it.
//
// An earlier version of this function called validateName and
// resolveSymlinks separately. The two calls covered the name
// contract and the symlink policy but skipped the deny-list and
// allow-list checks, which live in resolveForIO. The result was
// that a sub-root could be constructed at a denied path
// (Rel(".git") succeeded). The single pipeline call closes that
// gap: the checks that were added to the pipeline by WBS 13.4.1
// apply to the sub-root construction automatically.
//
// # Why validation happens at construction
//
// A caller that constructs a Handle at a misconfigured sub-path
// (an absolute path, a traversal name, a symlink to outside, a
// path on the Root's deny-list) should learn about the
// misconfiguration at construction, not at the first operation.
// Construction-time validation means the Handle's operations
// never fail for reasons unrelated to their input.
func newHandle(r *Root, rel string) (*Handle, error) {
	// The empty sub-path and "." both name the Root itself.
	// No pipeline call is needed: the Handle is the Root, and
	// the sub-boundary check on subsequent operations is a
	// no-op.
	if rel == "" || rel == "." {
		return &Handle{root: r, sub: ""}, nil
	}

	// Run the Root's full pipeline on the sub-path. This
	// validates the name, applies the deny-list and allow-list
	// checks, and resolves the symlinks under the Root's
	// policy.
	resolved, err := r.resolveForIO(rel)
	if err != nil {
		return nil, err
	}

	return &Handle{root: r, sub: resolved}, nil
}

// Path returns the resolved sub-path, joined with the parent
// Root's path.
//
// The value is informational, like Root.Path. It is intended
// for logging and diagnostics and must not be used for I/O.
func (h *Handle) Path() string {
	if h.sub == "" {
		return h.root.Path()
	}
	return h.root.Path() + "/" + h.sub
}

// Policy returns the symlink policy in effect, inherited from
// the parent Root.
func (h *Handle) Policy() SymlinkPolicy { return h.root.policy }

// -----------------------------------------------------------------------------
// Read operations
// -----------------------------------------------------------------------------

// Open opens name for reading, scoped to the handle's sub-path.
//
// The operation runs the handle's pipeline (see
// resolveForIOHandle), which enforces both the sub-boundary and
// the root boundary.
func (h *Handle) Open(name string) (fs.File, error) {
	resolved, err := h.resolveForIOHandle(name)
	if err != nil {
		return nil, err
	}
	return h.root.fs.Open(resolved)
}

// Stat returns metadata about name, scoped to the handle.
func (h *Handle) Stat(name string) (fs.FileInfo, error) {
	resolved, err := h.resolveForIOHandle(name)
	if err != nil {
		return nil, err
	}
	return h.root.fs.Stat(resolved)
}

// ReadFile reads the whole file at name, scoped to the handle.
func (h *Handle) ReadFile(name string) ([]byte, error) {
	f, err := h.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// ReadDir lists the entries of the directory at name, scoped to
// the handle.
func (h *Handle) ReadDir(name string) ([]fs.DirEntry, error) {
	resolved, err := h.resolveForIOHandle(name)
	if err != nil {
		return nil, err
	}
	entries, err := h.root.fs.List(resolved)
	if err != nil {
		return nil, err
	}
	if h.root.collide {
		if err := detectCollisions(entries); err != nil {
			return nil, pathError(err, name)
		}
	}
	return entries, nil
}

// Walk recursively visits the tree rooted at name, scoped to
// the handle.
//
// The walker inherits the parent Root's bounds. A walk of a
// handle's sub-path is bounded by the same MaxDepth and
// MaxEntries as a walk of the parent; the sub-path is the
// starting point, not the boundary.
func (h *Handle) Walk(name string, fn WalkFunc) error {
	if fn == nil {
		return forgeerr.New(
			forgeerr.CategoryInternal,
			forgeerr.CodeInternalUnexpected,
			"safefs.Handle.Walk: nil WalkFunc",
		)
	}
	clean, err := h.resolveForIOHandle(name)
	if err != nil {
		return err
	}
	w := &walker{root: h.root, fn: fn}
	return w.walk(clean, name, 0)
}

// Rel returns a sub-handle of this handle.
//
// The returned Handle's sub-path is this handle's sub-path
// joined with rel, so the nested handle enforces both
// boundaries: the outer handle's, and its own.
//
// Nested handles are the mechanism by which a future component
// can be sandboxed to a subtree that is itself a sandbox of a
// larger project. The composition is transitive: a Handle at
// src/internal cannot name anything outside src/internal, even
// if its parent Handle at src could.
func (h *Handle) Rel(rel string) (*Handle, error) {
	if rel == "" || rel == "." {
		return h, nil
	}
	clean, err := h.root.validateName(rel)
	if err != nil {
		return nil, err
	}

	// Resolve the sub-path through the handle's pipeline so
	// that the nested handle's sub-path is expressed in terms
	// of the root's namespace, not the handle's.
	full := clean
	if h.sub != "" {
		full = h.sub + "/" + clean
	}
	resolved, err := h.root.resolveForIO(full)
	if err != nil {
		return nil, err
	}
	if !isUnder(resolved, h.sub) {
		return nil, pathError(ErrSubBoundary, rel)
	}
	return &Handle{root: h.root, sub: resolved}, nil
}
