// Package safefs is Forge's policy-enforcing filesystem boundary.
//
// # What this package is
//
// safefs is the only package in the module permitted to combine a
// caller-supplied name with a directory tree. It owns every
// filesystem security policy that Forge applies:
//
//   - symlink resolution and policy (NoFollow, FollowInRoot,
//     FollowAnywhereForbidden);
//   - path-traversal rejection (".." segments are refused before
//     any resolution is attempted);
//   - root confinement (a name that would escape the root is
//     refused, using filepath.IsLocal rather than string-prefix
//     comparison);
//   - bounded walks (MaxDepth, MaxEntries);
//   - optional case-collision detection for case-insensitive
//     filesystems.
//
// Domain packages — blueprint, template, renderer, update, check,
// drift — import safefs and MUST NOT import internal/filesystem
// directly. The rule is enforced by
// scripts/deps/verify-fs-layering.sh (ADR-035).
//
// # What this package is not
//
// safefs is not a general-purpose filesystem library. It is not a
// drop-in replacement for io/fs. It does not attempt to be
// portable beyond what Forge needs. It does not parse blueprints,
// render templates, or know what a "project" is. It does not import
// os. Every disk operation it performs goes through the FS
// interface defined in internal/filesystem, which is the only
// package in the module — apart from cmd/forge — that holds an
// *os.File.
//
// # Enforcement by construction
//
// The architectural claim of this package is stronger than "we
// validate paths." A domain package cannot perform an unchecked
// filesystem operation, because it has no legal way to name one.
//
// This is achieved by four properties:
//
//  1. Root is the only exported constructor that produces file
//     handles. There is no exported function in safefs that
//     returns an *os.File or accepts an absolute path.
//
//  2. Root never returns an *os.File. It returns io/fs.File, whose
//     Stat has been filtered through the symlink policy.
//
//  3. The only constructor is New, which validates the root once
//     and encodes the confinement invariant in the Root's
//     lifetime. Methods rely on that invariant; they never
//     re-derive it from an untrusted path.
//
//  4. The CI lint forbids the alternative. A domain package that
//     tries to import os or internal/filesystem fails the build.
//
// The result is that filesystem security is a property of the type
// system, not of reviewer diligence. A contributor who has never
// read this comment cannot accidentally introduce CWE-22, because
// the compiler and CI prevent the shape of code that would contain
// it.
//
// # The name contract
//
// Every Root method accepts a name that satisfies four rules:
//
//   - non-empty;
//   - local, per filepath.IsLocal (this is the single source of
//     truth: it rejects absolute paths, Windows drive letters, UNC
//     prefixes, ".." segments, and reserved names);
//   - forward-slash (backslashes are normalised on entry, so the
//     same call site works on Windows);
//   - cleaned, per path.Clean (which is platform-independent).
//
// Names that violate the contract produce an error wrapping either
// ErrPathTraversal or ErrNotLocal. Both wrap a forgeerr category,
// so the CLI can map them to exit codes without string matching
// (ADR-011, ADR-013).
//
// # Example
//
// Construct a Root once, reuse it for the lifetime of the command:
//
//	root, err := safefs.New(projectDir,
//	    safefs.WithSymlinkPolicy(safefs.NoFollow),
//	    safefs.WithMaxDepth(64),
//	    safefs.WithMaxEntries(1<<20),
//	)
//	if err != nil {
//	    return fmt.Errorf("construct safefs root: %w", err)
//	}
//
//	f, err := root.Open("blueprint/go-api.yaml")
//	if err != nil {
//	    return err
//	}
//	defer f.Close()
//
//	data, err := io.ReadAll(f)
//	if err != nil {
//	    return err
//	}
//
// The domain code never sees an *os.File. It never sees an
// absolute path. It cannot construct a name that escapes the root,
// because validateName refuses such names before any I/O is
// attempted.
//
// # Why a handle, not a validator
//
// It is tempting to expose a ValidatePath function and call it
// before each os.Open. That pattern is vulnerable to TOCTOU races:
// the path can change between validation and the open. The policy
// must be applied by the operation that performs the I/O, which is
// why Root.Open — not Root.Validate — is the enforcement point.
//
// # Testing
//
// safefs depends on the filesystem.FS interface, not on os. Tests
// inject an in-memory implementation via WithFS, so the suite never
// touches the real disk. This keeps tests fast, deterministic, and
// parallel-safe:
//
//	mem := fstest.MapFS{
//	    "a.txt": &fstest.MapFile{Data: []byte("hello")},
//	}
//	root, err := safefs.New("/root", safefs.WithFS(mem))
//	if err != nil {
//	    t.Fatal(err)
//	}
//	b, err := root.ReadFile("a.txt")
//
// Symlink behaviour is the one exception: it requires a real
// filesystem, so its tests are isolated in symlink_unix_test.go and
// symlink_windows_test.go behind build tags.
//
// Every policy added to this package must ship with positive,
// negative, edge, corner, boundary, and non-functional tests. The
// conformance test in verify-fs-layering_test.sh proves that a
// violating package is rejected by the layering lint; that test is
// as much a part of this package's suite as the unit tests are.
//
// # Concurrency
//
// A *Root is safe for concurrent use by multiple goroutines. It is
// constructed once and is immutable thereafter, apart from the
// resolved-symlink cache, which is guarded by a mutex. The
// underlying filesystem.FS implementation is not required to be
// concurrency-safe; Root serialises calls to it.
//
// # Boundaries
//
// safefs does not attempt to be a chroot. A process with
// filesystem access outside the Root can still reach outside the
// Root; safefs only prevents the code that uses a Root from doing
// so by accident or by a crafted name. Process-level isolation is a
// separate concern, tracked under the security model, and is
// orthogonal to this package.
//
// # Stability
//
// The exported surface of this package — Root, New, Option, the
// With* functions, SymlinkPolicy, WalkFunc, and the sentinel
// errors — is covered by ADR-035 and is subject to the same
// stability policy as the rest of the module's foundation
// packages. Adding a method to Root is a compatible change;
// removing or renaming one is not.
//
// # See also
//
//   - ADR-035 (Filesystem Enforcement by Construction)
//   - ADR-011 (Closed Error Category Taxonomy)
//   - ADR-013 (Error Wrapping Contract)
//   - docs/architecture/filesystem-layering.md
//   - internal/filesystem (Layer 1 primitives)
package safefs
