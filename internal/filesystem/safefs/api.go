package safefs

// This file is an index of the Root surface, not a definition.
// Every method named below is declared in root.go, walk.go, or
// write.go. This file exists so that a reader can audit the
// entire Layer 2 surface — every operation, its policy, its
// consumer — without reading five files.
//
// # Purpose
//
// The WBS 13.1.2 acceptance criterion "no operation exists just
// in case" is enforced by this file: every method below names a
// WBS item that consumes it, and a method whose row here lacks a
// consumer is a method that should be removed.
//
// # Why the list is closed
//
// A new Root method is not a private implementation detail. It
// is a promise to every caller that the operation is safe under
// every policy the Root supports, that it enforces the name
// contract, that it composes with the resolved-symlink cache,
// and that it participates in the depth and entry bounds where
// applicable. Adding one requires answering those questions, and
// this file is the checklist a reviewer uses to verify that they
// were answered.
//
// # What this file does NOT do
//
//   - It does not declare the methods. The declarations in
//     root.go, walk.go, and write.go are authoritative.
//   - It does not change the surface. A future addition to Root
//     adds a row here in the same commit as the method, and the
//     row must name a consumer.
//   - It does not re-document the behavioral contract. Each
//     method's doc comment is the contract; this file summarises
//     it, and a reviewer comparing the two catches drift.

// Root — Layer 2 operations.
//
// The Root type exposes the following operations. Every operation
// runs the three-step pipeline: validateName, resolveSymlinks,
// and the underlying FS call. Operations marked (atomic) run a
// fourth step (write-temp-rename) after the FS call.
//
// Open(name) (fs.File, error)
//   Opens a file for streaming reads. The returned value is an
//   io/fs.File whose concrete type the policy layer controls.
//   Policy: enforced.
//   Missing: error wraps fs.ErrNotExist.
//   Consumer: WBS 13.2.1.
//
// Stat(name) (fs.FileInfo, error)
//   Returns metadata, following the Root's symlink policy.
//   Policy: enforced.
//   Consumer: WBS 13.6.1.
//
// ReadFile(name) ([]byte, error)
//   Reads a whole file. Convenience over Open + io.ReadAll.
//   Policy: enforced.
//   Consumer: WBS 13.2.1.
//
// ReadDir(name) ([]fs.DirEntry, error)
//   Lists immediate children, sorted. Non-recursive.
//   Policy: enforced. Case-collision check when enabled.
//   Consumer: WBS 13.5.1, WBS 13.6.1.
//
// Walk(name, fn) error
//   Recursive traversal with depth and entry bounds.
//   Policy: enforced. Directory symlinks not descended.
//   Consumer: Phase 7 (repository walk), Phase 14 (base-state).
//
// Write(name, data, mode) error
//   Writes data, creating parents as needed.
//   Policy: enforced. Parent mode 0o755.
//   Consumer: WBS 13.2.3.
//
// WriteAtomic(name, data, mode) error
//   Write + fsync + rename. The rename is the atomic step.
//   Policy: enforced.
//   Consumer: WBS 13.2.3 (Phase 4 render plan).
//
// Mkdir(name, mode) error
//   Creates a single directory. Fails if the parent is missing.
//   Policy: enforced.
//   Consumer: WBS 13.2.3.
//
// MkdirAll(name, mode) error
//   Creates the directory and any missing parents.
//   Policy: enforced.
//   Consumer: WBS 13.2.3.
//
// Remove(name) error
//   Removes a file or an empty directory.
//   Policy: enforced.
//   Consumer: WBS 13.2.3.
//
// RemoveAll(name) error
//   Recursively removes a subtree.
//   Policy: enforced. Refuses to remove the Root itself.
//   Consumer: WBS 13.2.3 (cleanup of a stale render tree).
//
// RenameIfNotExists(src, dst) error
//   Renames src to dst, failing if dst exists.
//   Policy: enforced. Both names run the full pipeline.
//   Consumer: WBS 13.2.3 (avoid clobbering user edits).
//
// Exists(name) (bool, error)
//   Reports whether name exists.
//   Policy: enforced.
//   Consumer: WBS 13.6.1.
//
// Hash(name, algo) ([]byte, error)
//   Returns the digest of a file's content under algo.
//   Policy: enforced. The FS call is streaming, not Read-all.
//   Consumer: Phase 14 (base-state hashing).
//
// Symlink(target, link) error
//   Creates a symlink at link whose target is target.
//   Policy: restricted. The target must be a local relative
//   name; absolute targets are refused. This is the only Root
//   operation that creates a link, and it exists because the
//   template engine needs to reproduce symlinks found in a
//   source tree.
//   Consumer: WBS 13.2.2 (template materialisation).
//
// Readlink(name) (string, error)
//   Returns a symlink's stored target.
//   Policy: enforced. This is the read side of Symlink, exposed
//   so that a caller can reproduce a link's target verbatim.
//   Consumer: WBS 13.2.2.
//
// # Rejected operations
//
// The following operations are deliberately NOT on Root. A future
// requirement that needs one of them must open an ADR before
// adding a method, because each of them has a reason to be absent
// that is not obvious from the outside.
//
// Chmod — folded into Write. Mode is set at creation. Changing
//   the mode of an existing file has no consumer in Phase 4, 7,
//   14, or 15. A future caller that needs it uses WriteAtomic
//   with the desired mode, which rewrites the file and applies
//   the new mode as a side effect.
//
// Chown — never. A foundation manager does not own files, and
//   changing ownership requires privileges that a user-level CLI
//   should not request.
//
// Truncate — never. Write with an empty byte slice expresses
//   the same intent, and truncation is a data-loss operation
//   that has no consumer in any phase.
//
// Watch — out of scope. Forge is a CLI, not a daemon. File
//   watching would require a long-running process and a
//   different concurrency model.
//
// Chroot — never. Process-level isolation is orthogonal to the
//   file abstraction; see the "Boundaries" section of the package
//   doc comment.
