# Symlink Policy

- **Status:** Frozen
- **WBS:** 13.5.2
- **ADR:** ADR-035
- **Traces:** SEC-FS-017 through SEC-FS-022
- **Last reviewed:** 2025-01-15
- **Review cadence:** per-phase

## Purpose

This document specifies the symlink policy that `safefs`
enforces and the TOCTOU mitigations the package applies. It
exists so that:

1. A caller can predict the behaviour of an operation on a
   path that contains a symlink.
2. A contributor adding an operation knows how to handle
   symlinks.
3. A security reviewer can audit the policy by reading one
   document and comparing it to one file
   (`internal/filesystem/safefs/symlink.go`).

## The Four Rules

### Rule 1 — The target directory may be a symlink

A caller can construct a Root on a path that is a symlink. For
example, on macOS, `~/projects` might be a symlink to
`/Volumes/external/projects`.

**Behaviour:** `safefs.New` resolves the target directory's
symlinks once, at construction. The Root's `Path()` returns the
resolved absolute path. Subsequent operations are relative to
the resolved path, and the OS resolves the target directory
when the first operation touches the disk.

**Rationale:** a caller who names a file inside a project
expects the operation to act on the file, not on the symlink
that names the project. Resolving the target once is cheaper
than resolving it per operation and produces a more useful
`Path()` for logging.

**Failure mode:** if the target does not exist, `New` falls
back to the unresolved absolute path. If the target is a
symlink chain that exceeds the OS's limit, `New` returns an
error wrapping `ErrSymlinkLoop`.

### Rule 2 — Symlinks inside the target are refused on the write path

A write to `a/b/c.txt` where any component of `a/b` is a
symlink is refused with `ErrSymlinkDenied`. The final component
of a write is never a symlink: the write primitive opens the
target with `O_NOFOLLOW` (Unix) or
`FILE_FLAG_OPEN_REPARSE_POINT` (Windows), which refuses to
follow a symlink at the final component.

**Rationale:** a symlink inside the target can point outside
it. A write that follows the symlink writes to the target, not
to the location the caller named. That is a CWE-59 vector and
the source of the classic "I edited a file and the change went
somewhere else" bug.

**Failure mode:** the failure is `ErrSymlinkDenied` (category
`CategorySecurity`, exit code 13).

### Rule 3 — Symlinks during Walk are reported but not followed

A directory symlink is delivered to the walk callback as a
non-directory entry (`Type() == fs.ModeSymlink`); the walker
does not descend into it. A symlink to a file is delivered
normally. The callback can inspect the entry's type to decide
whether to follow the link explicitly.

**Rationale:** following directory symlinks during a walk turns
a tree into a graph. A single link can create a cycle that no
depth bound catches, or a fan-out that no entry bound predicts.
The walker reports the link so the caller can decide; the
caller's decision is explicit and visible.

**Failure mode:** none. The link is an entry, not an error.

### Rule 4 — Broken symlinks are reported, not errored

A symlink whose target does not exist is a valid directory
entry. `ReadDir` returns it; `Walk` delivers it to the
callback; `Stat` (which follows) reports the OS's error
(typically `ErrNotFound`); `Lstat` (which does not follow)
reports the link's own metadata.

**Rationale:** a broken symlink is a fact about the filesystem,
not a policy violation. Erroring on it would make a directory
that contains a broken link unlistable, which is not what the
caller expects. The caller can inspect the entry and decide.

**Failure mode:** the caller's decision. A callback that wants
to error on a broken link can do so; a callback that wants to
skip it can do so.

## TOCTOU Posture

`safefs` is **best-effort** against an attacker with concurrent
filesystem access. The policy narrows the TOCTOU window but does
not close it.

### What is guaranteed

- No lexical traversal reaches the filesystem. A name that
  contains `..` after cleaning is refused before any syscall.
- A symlink placed on the parent path *before* the operation
  is detected and refused.
- The final component of a write is never a symlink. The write
  primitive's open flags (`O_NOFOLLOW` or
  `FILE_FLAG_OPEN_REPARSE_POINT`) make the kernel refuse a
  symlink at the final component atomically.
- The atomic-write primitive's rename is atomic on POSIX and on
  Windows. A reader sees either the old content or the new
  content.

### What is not guaranteed

- A symlink placed on the parent path *between* the pipeline's
  resolution and the write primitive's open is not detected.
  The window between the two is one syscall wide; an attacker
  who wins the race can change the meaning of the name.
  Closing the window would require an atomic
  "resolve-and-open" primitive that the operating system does
  not provide cross-platform.
- A symlink placed on the parent path *after* the write
  primitive's open and *before* the write is not detected.
  This is the same window, seen from the other side.
- A concurrent writer that creates the target between the
  fail-closed check and the rename is silently overwritten by
  the rename. `rename(2)` does not support "rename only if the
  target does not exist" atomically.

### What a caller can do

- A caller that needs a stronger guarantee holds the parent
  directory open and operates relative to the descriptor. This
  is outside `safefs`'s abstraction.
- A caller that operates on a tree under its own exclusive
  control does not need the stronger guarantee.

## The TOCTOU Mitigations

The four mitigations are applied in order. Each narrows the
window; together they make the window one syscall wide.

**(a) `O_NOFOLLOW` where the OS supports it.** On Unix,
`open(2)` with `O_NOFOLLOW` refuses to open a symlink. On
Windows, `CreateFile` with `FILE_FLAG_OPEN_REPARSE_POINT`
refuses to follow a symlink at the final component.

**(b) Resolve the parent immediately before the write.** The
pipeline's `resolveForIO` resolves the parent and returns the
resolved name. The write operation uses that name immediately.

**(c) Write to a temp file in the same directory, then
rename.** The atomic-write primitive writes to
`<name>.tmp.<random>` in the same directory as `<name>`, then
renames. The rename is atomic on POSIX.

**(d) On Windows, `FILE_FLAG_OPEN_REPARSE_POINT`.** The
Windows equivalent of `O_NOFOLLOW`. See (a).

## Cross-Platform Summary

| Mitigation | Unix | Windows |
|---|---|---|
| Refuse symlink at final component of write | `O_NOFOLLOW` | `FILE_FLAG_OPEN_REPARSE_POINT` |
| Refuse symlink at final component of read | `O_NOFOLLOW` on open | `FILE_FLAG_OPEN_REPARSE_POINT` |
| Atomic replace | `rename(2)` | `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` |
| Durable write | `fsync` on file and parent dir | `FlushFileBuffers`; `MOVEFILE_WRITE_THROUGH` for the rename |
| Symlink-loop detection | `ELOOP` from `Lstat`/`Readlink` | `ERROR_CANT_RESOLVE_FILENAME` |

## Change History

| Date | Change | Author |
|------|--------|--------|
| 2025-01-15 | Initial version; ratifies WBS 13.5.2 | Security Owner |

## See Also

- [Path Normalization & Canonicalization](../reference/path-normalization.md)
- [Boundary Model](../reference/boundary-model.md)
- [Filesystem Errors](../reference/filesystem-errors.md)
- [internal/filesystem/safefs/symlink.go](../../internal/filesystem/safefs/symlink.go)
- [internal/filesystem/safefs/atomic.go](../../internal/filesystem/safefs/atomic.go)
