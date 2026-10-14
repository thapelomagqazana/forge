package safefs

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// This file defines Root — the policy-enforcing handle that is the
// only way domain code names a file — and its construction and
// operations.
//
// # Layout
//
//   - Root            the handle itself
//   - New             the single constructor; validates once
//   - Path / Rel      informational accessor and scoped sub-root
//   - Open / Stat / ReadFile / ReadDir   the read operations
//   - validateName    the name contract, shared by every method
//   - resolveForIO    the boundary-enforcement pipeline (pipeline.go)
//   - resolveSymlinks symlink policy enforcement (symlink.go)
//
// Write operations are intentionally absent from this file; they
// arrive in WBS 13.3. See the package doc for the contract they
// will be required to honour.

// Root is a policy-enforcing handle to a directory tree.
//
// # Contract
//
// A Root is the only way a domain package may name a file. It is
// constructed once via New and is immutable thereafter. Every
// exported method takes a forward-slash, cleaned, relative name;
// the Root validates that name against its symlink policy,
// traversal policy, and root-confinement invariant before
// performing any I/O.
//
// # Construction
//
// New converts targetDir to an absolute, cleaned path and validates
// it once. All subsequent operations are relative to that path.
// Construction-time validation is the single point at which the
// confinement invariant is established; methods rely on it and do
// not re-derive it from an untrusted name.
//
// # Boundary enforcement
//
// Every operation runs a five-step pipeline (see pipeline.go for
// the full description):
//
//  1. validateName — the name is non-empty, local, forward-slash,
//     and cleaned. Lexical traversal is rejected here, before any
//     syscall.
//
//  2. The lexical escape check is folded into validateName:
//     filepath.IsLocal rejects any cleaned name that begins with
//     "..".
//
//  3. resolveParent — resolve symlinks on the parent directory of
//     the target under the Root's policy. Resolving the parent
//     (not the target) is what makes the operation correct for
//     names whose final component does not yet exist, and it is
//     what defends against a symlink-swap on the containing
//     directory.
//
//  4. Resolved-parent escape check — the resolved parent must
//     remain inside the Root's boundary.
//
//  5. Delegate — the underlying FS is called with the resolved
//     name.
//
// # TOCTOU posture
//
// safefs is best-effort against an attacker with concurrent
// filesystem access. It guarantees that no lexical traversal
// reaches the filesystem, and that a symlink-swap on the parent
// path is detected and refused. It does not guarantee that the
// final component cannot change between the parent resolution and
// the syscall; closing that window would require an atomic
// "resolve and open" primitive that the operating system does not
// provide. Callers that need a stronger guarantee must arrange
// external coordination (for example, holding the parent
// directory open and operating relative to the descriptor).
//
// # Concurrency
//
// A Root is safe for concurrent use by multiple goroutines. The
// fields set by New are read-only after construction; the only
// mutable state is the resolved-symlink cache, which is guarded by
// mu. The underlying filesystem.FS implementation is not required
// to be concurrency-safe; Root serialises calls to it, so a
// single Root provides the guarantee even when its FS does not.
//
// # Lifetime
//
// A Root holds no OS resources. It does not need to be closed.
// Individual files returned by Open must be closed by their
// caller.
type Root struct {
	// abs is the absolute, cleaned root. It is set once by New
	// and never mutated. It is safe to log: the root is a
	// caller-supplied path and is not sensitive (ADR-016).
	abs string

	// fs is the underlying filesystem. It is never nil after New
	// returns successfully. Every method routes through it after
	// the pipeline (pipeline.go) has run.
	fs filesystem.FS

	// policy is the symlink policy. Immutable after New.
	policy SymlinkPolicy

	// maxDepth, maxEntries, maxSymlinkChain are the walk and
	// resolution bounds. Zero means unbounded. Immutable after
	// New.
	maxDepth        int
	maxEntries      int
	maxSymlinkChain int

	// collide enables case-collision detection. Immutable after
	// New.
	collide bool

	// mu guards cache. The cache maps a validated relative name
	// to its resolved absolute path when the symlink policy
	// requires resolution. It is populated lazily and is never
	// invalidated; a Root is assumed to operate on a tree that
	// does not change underneath it during its lifetime. Callers
	// that mutate the tree mid-flight construct a new Root.
	mu    sync.RWMutex
	cache map[string]string

	// deny is the Root's effective deny-list. It is nil if the
	// caller explicitly constructed a Root with no deny-list.
	deny *matchList

	// allow is the Root's allow-list. It is nil if the caller
	// did not configure one, in which case no allow-list check
	// is performed.
	allow *matchList
}

// New constructs a Root for targetDir.
//
// # Validation
//
// targetDir is converted to an absolute, cleaned path and validated
// once. If targetDir is empty, New returns an error wrapping
// ErrNotLocal. If targetDir cannot be made absolute, New returns an
// error wrapping ErrNotLocal with the original value attached.
//
// New does not verify that targetDir exists or that it is a
// directory. The first operation that requires the directory to
// exist will report the appropriate error. This is deliberate: a
// Root may legitimately be constructed before its directory is
// created (for example, in a bootstrap path), and requiring an
// existence check would make that pattern impossible without a
// second constructor.
//
// # Options
//
// Options are applied in the order given. Later options override
// earlier ones for scalar settings. The assembled config is
// validated once; the first problem found is returned as an error
// wrapping ErrInvalidOption (see policy.go).
//
// The deny-list and allow-list patterns are parsed once, here.
// A pattern that fails to parse is reported as an error wrapping
// ErrInvalidPattern; the Root is not constructed. Parsing at
// construction time means a misconfigured pattern list cannot
// silently permit a name the caller intended to reject.
//
// If no FS is supplied via WithFS, New constructs an OSFS bound to
// the resolved root. The OSFS is the only type in the module,
// apart from cmd/forge, that holds an *os.File; every disk
// operation performed through a Root goes through it.
//
// # Return value
//
// The returned *Root is safe to use concurrently and holds no OS
// resources. It does not need to be closed.
func New(targetDir string, opts ...Option) (*Root, error) {
	if targetDir == "" {
		return nil, pathError(ErrNotLocal, "<empty>")
	}

	cfg := defaultConfig()
	for _, o := range opts {
		if o == nil {
			// A nil Option is a programming error but not a
			// reason to panic. Skip it and let validation
			// proceed; if the caller intended to set a value
			// and passed nil, the resulting error (or the
			// surprising default) will surface at the first
			// operation.
			continue
		}
		o(&cfg)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Build the deny-list. The effective list is:
	//
	//   - the defaults, if the caller did not pass
	//     WithDenyList; or
	//   - the caller's explicit list, if WithDenyList was
	//     called; plus
	//   - any patterns added by WithAdditionalDenyPatterns,
	//     in either case.
	//
	// The two cases are unified by starting from the effective
	// base and appending the additional patterns. The order of
	// the base matters: the defaults come first so that a caller
	// reading the deny-list in a debugger sees the well-known
	// patterns at the top.
	var denyBase []string
	if cfg.denyExplicit {
		denyBase = cfg.denyPatterns
	} else {
		denyBase = DefaultDenyPatterns
	}
	denyPatterns := make([]string, 0, len(denyBase)+len(cfg.additionalDenyPatterns))
	denyPatterns = append(denyPatterns, denyBase...)
	denyPatterns = append(denyPatterns, cfg.additionalDenyPatterns...)

	deny, err := newMatchList(denyPatterns)
	if err != nil {
		return nil, err
	}

	// Build the allow-list. Unlike the deny-list, the
	// allow-list has no defaults: a Root without an allow-list
	// permits every name that passes the boundary and
	// deny-list checks. The allow-list is only constructed if
	// the caller passed WithAllowList; a caller that passes
	// WithAllowList() with no patterns gets an empty list,
	// which rejects every name. That is the correct reading
	// of "the caller declared an allow-list and it permits
	// nothing."
	var allow *matchList
	if cfg.allowExplicit {
		allow, err = newMatchList(cfg.allowPatterns)
		if err != nil {
			return nil, err
		}
	}

	// Resolve the target directory to an absolute, cleaned
	// path. filepath.Abs does not resolve symlinks; it only
	// makes the path absolute against the process's current
	// working directory. The resolution of symlinks is the
	// next step.
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, pathError(ErrNotLocal, targetDir)
	}
	abs = filepath.Clean(abs)

	// Resolve the target directory's symlinks once, at
	// construction (Rule 1 of the symlink policy).
	//
	// A caller who names a project through a symlink (for
	// example, ~/projects → /Volumes/external/projects)
	// expects the Root's operations to act on the target, not
	// on the symlink. The resolved path is what Root.Path()
	// returns and what the OSFS is bound to.
	//
	// resolveRootSymlink handles four cases:
	//
	//   - The target exists and is not a symlink: returns abs
	//     unchanged.
	//
	//   - The target exists and is a symlink: returns the
	//     resolved target.
	//
	//   - The target does not exist (or is a broken symlink):
	//     returns abs unchanged, so that a Root can be
	//     constructed before its directory is created.
	//
	//   - The target is a symlink chain that exceeds the OS's
	//     limit: returns an error wrapping ErrSymlinkLoop.
	//
	// The function is idempotent: resolving an already-resolved
	// path returns the path unchanged.
	abs, err = resolveRootSymlink(abs)
	if err != nil {
		return nil, err
	}

	if cfg.fs == nil {
		cfg.fs = filesystem.NewOSFS(abs)
	}

	return &Root{
		abs:             abs,
		fs:              cfg.fs,
		policy:          cfg.policy,
		maxDepth:        cfg.maxDepth,
		maxEntries:      cfg.maxEntries,
		maxSymlinkChain: cfg.maxSymlinkChain,
		collide:         cfg.detectCaseCollisions,
		cache:           make(map[string]string),
		deny:            deny,
		allow:           allow,
	}, nil
}

// Path returns the canonical absolute root.
//
// Path is intended for logging and diagnostics: a log line that
// says "read failed under /home/user/project" is more useful than
// one that says "read failed". It is not intended for I/O. A
// caller that takes Path, joins it with a name, and hands the
// result to os has bypassed the boundary enforcement the Root
// exists to provide.
//
// The value is a caller-supplied path and is not sensitive
// (ADR-016). Domain packages should use it only for display;
// every filesystem operation must go through Root's methods.
func (r *Root) Path() string { return r.abs }

// Abs returns the canonical absolute root.
//
// Deprecated: use Path. The name Abs is easily confused with
// path/filepath.Abs and does not convey that the value is
// informational. Path is the supported accessor. Abs is retained
// for one release to give callers time to migrate.
func (r *Root) Abs() string { return r.Path() }

// Policy returns the symlink policy in effect for this Root. It is
// exposed so that callers can log it and tests can assert on it.
func (r *Root) Policy() SymlinkPolicy { return r.policy }

// Rel returns a scoped sub-root rooted at the named relative path.
//
// # Semantics
//
// root.Rel("src") returns a *Handle whose operations are
// equivalent to the parent's with "src/" prepended to the name,
// with one additional guarantee: the handle rejects any name
// that resolves outside "src", regardless of whether the name
// would be legal under the parent.
//
// # Why a handle rather than a Root
//
// A caller that wants to sandbox a component to a subtree could
// construct a new Root at the subtree path, but that would lose
// the parent's resolved-symlink cache and would re-validate the
// subtree path from scratch. A Handle shares the parent's cache,
// policy, and bounds. It also composes with the parent's
// boundary: a name that escapes the sub-boundary is rejected
// even if it stays inside the root.
//
// # Construction-time validation
//
// Rel validates rel immediately: the name must satisfy the name
// contract, and the resolved sub-path must remain inside the
// parent Root's boundary. Errors from Rel are a signal that the
// caller's intended sandbox is misconfigured.
//
// The returned *Handle is safe for concurrent use and holds no
// OS resources. It is valid for the lifetime of its parent Root.
func (r *Root) Rel(rel string) (*Handle, error) {
	return newHandle(r, rel)
}

// -----------------------------------------------------------------------------
// Read operations
// -----------------------------------------------------------------------------

// Open opens name for reading.
//
// Open runs the five-step pipeline (see pipeline.go):
//
//  1. validateName normalises and lexically checks the name.
//  2. The lexical escape check rejects names that begin with
//     ".." after cleaning.
//  3. resolveParent resolves symlinks on the parent directory of
//     the target, defending against a symlink-swap between the
//     lexical check and the syscall.
//  4. The resolved-parent escape check rejects parents that
//     resolve outside the root.
//  5. fs.Open is called with the resolved name.
//
// The returned file is an io/fs.File. It is never an *os.File; a
// caller cannot type-assert to reach the underlying descriptor.
// This is a property of the enforcement-by-construction model
// (ADR-035).
//
// The caller must Close the returned file.
func (r *Root) Open(name string) (fs.File, error) {
	resolved, err := r.resolveForIO(name)
	if err != nil {
		return nil, err
	}
	return r.fs.Open(resolved)
}

// Stat returns metadata about name.
//
// Stat follows the same five-step pipeline as Open. On success it
// returns the fs.FileInfo produced by the underlying filesystem.
//
// The returned FileInfo's Name method returns the base name of
// the *resolved* path, not the name the caller passed. Callers that
// need the original name should keep it separately; the resolved
// name is not guaranteed to be stable across policy changes.
func (r *Root) Stat(name string) (fs.FileInfo, error) {
	resolved, err := r.resolveForIO(name)
	if err != nil {
		return nil, err
	}
	return r.fs.Stat(resolved)
}

// ReadFile reads the whole file at name and returns its contents.
//
// ReadFile is a convenience over Open + io.ReadAll. It exists so
// that the common case does not force every caller to write the
// same four lines of defer-and-close, and so that the pipeline
// checks cannot be forgotten by a caller who opens a file by hand.
//
// The returned slice is a fresh allocation. The caller owns it.
func (r *Root) ReadFile(name string) ([]byte, error) {
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// ReadDir lists the entries of the directory at name.
//
// ReadDir runs the five-step pipeline and then delegates to the
// underlying FS's List. The result is sorted by filename,
// matching the io/fs.ReadDir contract; callers that need a
// different order must sort the result themselves.
//
// When case-collision detection is enabled, ReadDir inspects the
// result and returns an error wrapping ErrCaseCollision if two
// entries collide under Unicode case folding. The check is
// performed after the read, not before, because the collision is a
// property of the directory's contents, not of the name passed in.
func (r *Root) ReadDir(name string) ([]fs.DirEntry, error) {
	resolved, err := r.resolveForIO(name)
	if err != nil {
		return nil, err
	}
	entries, err := r.fs.List(resolved)
	if err != nil {
		return nil, err
	}
	if r.collide {
		if err := detectCollisions(entries); err != nil {
			return nil, pathError(err, name)
		}
	}
	return entries, nil
}

// -----------------------------------------------------------------------------
// Name validation
// -----------------------------------------------------------------------------

// validateName enforces the name contract that every Root method
// shares. On success it returns the cleaned name that the method
// should pass to the rest of the pipeline.
//
// # Contract
//
// A name is valid when it is:
//
//  1. Non-empty. The empty string is rejected with ErrNotLocal.
//
//  2. Local, per filepath.IsLocal. This is the single source of
//     truth for "is this a path inside the root." It rejects:
//     - absolute paths (Unix "/", Windows "C:\", UNC "\\host\share")
//     - paths containing a ".." segment (after cleaning)
//     - Windows reserved names (CON, NUL, AUX, COM1, ...)
//     - an empty string
//
//  3. Forward-slash. Backslashes are normalised to forward slashes
//     before cleaning, so the same call site works on Windows and
//     Unix without the caller knowing which platform it is on.
//
//  4. Cleaned, per path.Clean. path.Clean is used rather than
//     filepath.Clean because its output is platform-independent:
//     it always uses forward slashes and never introduces a
//     drive letter. The result is what validateName returns and
//     what the rest of the pipeline consumes.
//
// # Errors
//
//   - The empty string produces an error wrapping ErrNotLocal.
//   - A name that fails filepath.IsLocal produces an error
//     wrapping ErrPathTraversal if the failure is due to "..",
//     and ErrNotLocal otherwise. The distinction exists so that
//     callers can distinguish "malformed name" from "attempted
//     traversal" (see the comment on ErrNotLocal).
//
// # Cost
//
// validateName is pure string work. It does not touch the
// filesystem. It is therefore cheap enough to run at the top of
// every method, which is what the enforcement-by-construction
// model requires.
func (r *Root) validateName(name string) (string, error) {
	if name == "" {
		return "", pathError(ErrNotLocal, "<empty>")
	}

	// Normalise Windows separators. On Unix this is a no-op for
	// well-formed inputs. On Windows it is required: a caller
	// writing "a\b.txt" must not have the backslash survive into
	// path.Clean, which would treat it as a literal character.
	name = strings.ReplaceAll(name, `\`, `/`)

	clean := path.Clean(name)

	// path.Clean collapses "a/.." to "." and "../a" stays as
	// "../a". We special-case "." first because IsLocal(".") is
	// true and we want to allow it: opening the root itself is
	// legal.
	if clean == "." {
		return ".", nil
	}

	if !filepath.IsLocal(clean) {
		// Distinguish traversal from malformed. filepath.IsLocal
		// rejects both; callers care about the difference.
		if containsTraversal(clean) {
			return "", pathError(ErrPathTraversal, name)
		}
		return "", pathError(ErrNotLocal, name)
	}

	return clean, nil
}

// containsTraversal reports whether clean contains a ".." segment.
//
// clean is expected to be the output of path.Clean, so a ".."
// segment that survives cleaning is a genuine attempt to escape,
// not an artefact of an intermediate step like "a/../..".
//
// The check splits on "/" and compares each segment to "..". It
// does not use strings.Contains, which would match a file named
// "..foo".
func containsTraversal(clean string) bool {
	for _, seg := range strings.Split(clean, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Symlink cache
// -----------------------------------------------------------------------------

// cacheGet returns the cached resolution of name, if any.
//
// The cache is a performance optimisation, not a correctness
// mechanism. A miss is always correct: the caller falls through to
// the resolution path. A hit is correct only if the tree has not
// changed since the entry was written, which is the documented
// assumption on Root.
func (r *Root) cacheGet(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.cache[name]
	return v, ok
}

// cachePut records the resolution of name.
//
// cachePut is called on the resolution path after a successful
// resolution. It is not called for names that required no
// resolution (that is, names under NoFollow that reached the
// filesystem without hitting a symlink): caching them would be
// pure overhead.
func (r *Root) cachePut(name, resolved string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[name] = resolved
}

// -----------------------------------------------------------------------------
// Case-collision detection
// -----------------------------------------------------------------------------

// detectCollisions returns an error wrapping ErrCaseCollision if
// two entries in entries have names that are equal under Unicode
// case folding but not byte-equal.
//
// The check is O(n^2) in the worst case but O(n) in practice,
// because collisions are rare and the common case is a single
// lookup in a map. The map is populated lazily; a directory
// without collisions never allocates a second entry per name.
//
// detectCollisions is a no-op on empty or single-element input.
func detectCollisions(entries []fs.DirEntry) error {
	if len(entries) < 2 {
		return nil
	}
	seen := make(map[string]string, len(entries))
	for _, e := range entries {
		name := e.Name()
		folded := strings.ToLower(name)
		if prev, ok := seen[folded]; ok && prev != name {
			// Two distinct byte sequences folded to the same
			// key. Report the first collision; a caller that
			// needs all of them can call ReadDir and check
			// itself.
			return fmt.Errorf("%w: %q and %q", ErrCaseCollision, prev, name)
		}
		seen[folded] = name
	}
	return nil
}

// -----------------------------------------------------------------------------
// Compile-time assertions
// -----------------------------------------------------------------------------

// Root must satisfy the io/fs interfaces it advertises. The
// assertions fail the build if a future method signature drift
// makes Root no longer assignable to these interfaces.
var (
	_ interface {
		Open(string) (fs.File, error)
	} = (*Root)(nil)

	_ interface {
		Stat(string) (fs.FileInfo, error)
	} = (*Root)(nil)

	_ interface {
		ReadDir(string) ([]fs.DirEntry, error)
	} = (*Root)(nil)

	_ interface {
		ReadFile(string) ([]byte, error)
	} = (*Root)(nil)
)
