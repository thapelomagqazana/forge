---
title: Filesystem API Reference
status: Frozen
wbs: 13.1.2
adr: ADR-035
traces:
  - FR-FS-002
  - FR-FS-003
  - FR-FS-004
  - FR-FS-005
  - FR-FS-006
  - FR-FS-007
  - FR-FS-008
  - FR-FS-009
  - FR-FS-010
  - FR-FS-011
  - FR-FS-012
  - FR-FS-013
  - FR-FS-014
  - FR-FS-015
  - FR-FS-016
  - FR-FS-017
  - FR-FS-018
  - FR-FS-019
  - FR-FS-020
  - NFR-API-001
last_reviewed: 2025-01-15
review_cadence: per-phase
---

# Filesystem API Reference

> **Status — Frozen.** Every operation listed here is part of the
> Forge filesystem contract. A new operation requires an ADR. The
> list is closed; the rationale for each rejection is as
> load-bearing as the rationale for each inclusion.

This document is the reference for Forge's two-layer filesystem
architecture. It exists so that:

1. **Contributors know the surface before writing code.** Every
   operation a caller might need is named here, with its
   contract, its boundary behaviour, and the WBS item that
   motivates it.
2. **Reviewers can audit "no operation exists just in case."**
   The acceptance criterion is enforced by reading one file.
3. **Future proposals have a starting point.** The
   rejected-operations section names the operations that were
   considered and why they are absent.

For the *why* behind the layering, see
[`docs/architecture/filesystem-layering.md`](../architecture/filesystem-layering.md)
and [ADR-035](../decisions/ADR-035-filesystem-enforcement-by-construction.md).

---

## 1. Layer model

```text
   domain packages (blueprint, template, renderer, update, check, drift)
                              │
                              │ may import ONLY safefs
                              ▼
   internal/filesystem/safefs   ← policy: name contract, symlink policy,
                              │     root confinement, walk bounds,
                              │     case-collision detection
                              │
                              │ may import
                              ▼
   internal/filesystem          ← primitives: mirrors os / io/fs
                              │     no policy, no name contract,
                              │     no symlink resolution
                              │
                              ▼
                          os / io/fs
```

**Layer 1** (`internal/filesystem`) is policy-free. Every method
routes to `os` or `io/fs` with no inspection of the path beyond
joining it with `Boundary()`. It is the only package in the
module — apart from `cmd/forge` — that imports `os` for
filesystem I/O.

**Layer 2** (`internal/filesystem/safefs`) is the only package
domain code may import. Every operation runs the same three-step
pipeline: `validateName`, `resolveSymlinks`, then the underlying
FS call. Operations marked **(atomic)** run a fourth step after
the FS call.

The three-step pipeline is not a convention; it is enforced by
the fact that `Root` has no method that bypasses it. A reader
who wants to confirm that a specific operation is safe reads its
row below, follows the enforcement point, and finds the
corresponding `validateName` / `resolveSymlinks` call in the
source.

---

## 2. How to read the tables below

Each operation is documented as:

- **Signature.** The exact Go signature. Layers are marked
  `L1` or `L2`.
- **Purpose.** One sentence.
- **Boundary behaviour.** What happens on missing, denied, or
  existing inputs. The notation `fs.ErrNotExist`,
  `fs.ErrPermission`, and `fs.ErrExist` refers to the stdlib
  sentinels, which the caller can match with `errors.Is`.
- **Enforcement point.** For Layer 2 operations, the method that
  runs the policy pipeline. For Layer 1, "none — policy-free."
- **Consumer.** The WBS item that motivates the operation. An
  operation without a consumer is a bug in this document.

The notation `(atomic)` marks the operations that run
write-temp-then-rename.

---

## 3. Layer 1 — `internal/filesystem`

Layer 1's exported surface is the `FS` interface and its single
production implementation, `OSFS`. The interface is the contract;
`OSFS` is the only implementation that touches the real disk.

### 3.1 Read operations

#### `Read(path string) ([]byte, error)` — L1

Reads a whole file into memory.

- **Boundary:** `path` is joined with `Boundary()` and handed to
  `os.ReadFile`.
- **Missing:** error wraps `fs.ErrNotExist`.
- **Denied:** error wraps `fs.ErrPermission`.
- **Directory:** error (platform-specific).
- **Non-atomic:** a concurrent writer may see a partially
  written file. Callers that need a consistent snapshot
  coordinate externally.
- **Consumer:** WBS 13.2.1 (blueprint loader).

#### `Open(path string) (fs.File, error)` — L1

Opens a file for streaming reads.

- **Return type:** `io/fs.File`. The concrete type is
  `*osFileHandle`, an unexported wrapper defined in `osfs.go`.
  A caller cannot type-assert the returned value to `*os.File`
  and reach the underlying descriptor.
- **Boundary:** `path` is joined with `Boundary()` and handed to
  `os.Open`.
- **Missing:** error wraps `fs.ErrNotExist`.
- **Caller must:** `Close` the returned file.
- **Consumer:** WBS 13.2.1 (template loader), WBS 13.5.1
  (scanner).

### 3.2 Write operations

#### `Write(path string, data []byte, mode fs.FileMode) error` — L1

Writes `data` to `path`, creating parents as needed.

- **Boundary:** `path` is joined with `Boundary()`.
- **Parent directory:** created with mode `0o755` if missing.
  The parent mode is fixed rather than derived from `mode`
  because directories require the execute bit to be traversable.
- **Mode:** applied at creation. If the file already exists,
  the mode is not changed.
- **Non-atomic:** a partial write leaves a partially written
  file. Use `safefs.Root.WriteAtomic` for atomic replacement.
- **Consumer:** WBS 13.2.3 (renderer output).

### 3.3 Metadata and existence

#### `Exists(path string) (bool, error)` — L1

Reports whether `path` exists.

- **Missing:** `(false, nil)`. Not an error.
- **Denied:** `(false, err)` wrapping `fs.ErrPermission`. A
  caller can distinguish "does not exist" from "cannot tell."
- **Consumer:** WBS 13.6.1 (drift detection).

#### `Stat(path string) (fs.FileInfo, error)` — L1

Returns metadata, following a terminal symlink per the OS's
semantics.

- **Missing:** error wraps `fs.ErrNotExist`.
- **Symlink:** the terminal symlink is resolved; the returned
  `FileInfo` describes the target.
- **Callers that need the link itself:** use `Lstat`.
- **Consumer:** WBS 13.6.1 (drift mtime compare).

#### `Lstat(path string) (fs.FileInfo, error)` — L1

Like `Stat`, but does not follow a terminal symlink.

- **Symlink:** the returned `FileInfo` has
  `Mode()&fs.ModeSymlink` set.
- **Not for general use.** Domain code uses `safefs.Root.Stat`,
  which applies the symlink policy. `Lstat` exists for the
  policy layer to call.
- **Consumer:** `safefs.resolveSymlinks` (WBS 13.2.2).

#### `Readlink(path string) (string, error)` — L1

Returns the stored target of a symlink.

- **Return value:** the link's stored target, not its resolved
  path. It may be relative, absolute, or contain `..` segments.
- **Not a symlink:** platform-specific error (`EINVAL` on
  POSIX).
- **Not for general use.** `safefs.followInRoot` calls it after
  `Lstat` has confirmed the path is a link.
- **Consumer:** `safefs.followInRoot` (WBS 13.2.2).

#### `Boundary() string` — L1

Returns the directory against which all paths are resolved.

- **Pure accessor.** No I/O, never returns an error.
- **Consumer:** `safefs.New` (WBS 13.1.1).

### 3.4 Directory operations

#### `MkdirAll(path string, mode fs.FileMode) error` — L1

Creates `path` and any missing parents.

- **Existing directory:** returns `nil` (idempotent).
- **Existing non-directory:** error wraps `fs.ErrExist`.
  `os.MkdirAll` returns `ENOTDIR` on POSIX for this case;
  `OSFS` normalises it to `fs.ErrExist` so that callers can
  branch on the sentinel without platform detection.
- **Consumer:** WBS 13.2.3 (renderer scaffold).

#### `Remove(path string) error` — L1

Removes a file or an empty directory.

- **Missing:** error wraps `fs.ErrNotExist`.
- **Non-empty directory:** error (`ENOTEMPTY` on POSIX,
  `ERROR_DIRECTORY_NOT_EMPTY` on Windows).
- **Consumer:** WBS 13.2.3 (cleanup).

#### `List(path string) ([]fs.DirEntry, error)` — L1

Lists the immediate children of `path`, sorted by name.

- **Non-recursive.** A caller that needs a recursive walk uses
  `safefs.Root.Walk`, which enforces depth and entry bounds.
- **Missing:** error wraps `fs.ErrNotExist`.
- **Consumer:** WBS 13.5.1 (scan), WBS 13.6.1 (drift).

---

## 4. Layer 2 — `internal/filesystem/safefs`

Layer 2's exported surface is the `Root` type and the sentinel
errors in `errors.go`. Every `Root` method runs the three-step
pipeline before any I/O:

1. **`validateName`** — the name is non-empty, local per
   `filepath.IsLocal`, forward-slash, and cleaned per
   `path.Clean`.
2. **`resolveSymlinks`** — the name is resolved against the
   Root's `SymlinkPolicy`. Under `NoFollow` (the default), any
   component that is a symlink is refused. Under `FollowInRoot`,
   resolution proceeds up to `MaxSymlinkChain` hops, and any
   escape of the root is refused.
3. **FS call** — the underlying `filesystem.FS` method is
   invoked on the resolved name.

Operations marked **(atomic)** then run a fourth step:
write-temp-then-rename.

### 4.1 Read operations

#### `Open(name string) (fs.File, error)` — L2

Opens a file for streaming reads.

- **Policy:** enforced (all three steps).
- **Return type:** `io/fs.File`. The concrete type is the one
  `filesystem.FS.Open` returns; under `OSFS`, that is an
  `*osFileHandle`. A caller cannot reach the underlying
  descriptor.
- **Missing:** error wraps `fs.ErrNotExist`.
- **Symlink:** under `NoFollow`, an error wrapping
  `ErrSymlinkDenied`. Under `FollowInRoot`, resolution proceeds
  to the terminal target.
- **Consumer:** WBS 13.2.1.

#### `Stat(name string) (fs.FileInfo, error)` — L2

Returns metadata.

- **Policy:** enforced.
- **Symlink:** as `Open`.
- **Consumer:** WBS 13.6.1.

#### `ReadFile(name string) ([]byte, error)` — L2

Reads a whole file. Convenience over `Open` + `io.ReadAll`.

- **Policy:** enforced. The `Open` call inside runs the
  pipeline; `ReadFile` does not bypass it.
- **Missing:** error wraps `fs.ErrNotExist`.
- **Consumer:** WBS 13.2.1.

#### `ReadDir(name string) ([]fs.DirEntry, error)` — L2

Lists the immediate children of `name`, sorted.

- **Policy:** enforced. When `WithCaseCollisionDetection(true)`
  is set, the result is inspected and a collision produces an
  error wrapping `ErrCaseCollision`.
- **Non-recursive.** Use `Walk` for recursion.
- **Missing:** error wraps `fs.ErrNotExist`.
- **Consumer:** WBS 13.5.1, WBS 13.6.1.

#### `Walk(name string, fn WalkFunc) error` — L2

Recursive traversal with bounds.

- **Policy:** enforced.
- **Bounds:** `MaxDepth` and `MaxEntries`, both configured at
  construction. Zero disables. A bound overrun aborts the walk
  with an error wrapping `ErrDepthExceeded` or
  `ErrEntriesExceeded`; the error is returned to the caller, not
  delivered to the callback.
- **Symlinks:** directory symlinks are delivered to the callback
  as non-directory entries and are **not** descended, under any
  policy. This is a deliberate choice: descending would turn the
  tree into a graph and defeat the bounds.
- **Ordering:** FS-provided order, which for `OSFS` is sorted by
  filename.
- **Consumer:** Phase 7 (repository walk), Phase 14 (base-state
  hashing).

### 4.2 Write operations

#### `Write(name string, data []byte, mode fs.FileMode) error` — L2

Writes `data` to `name`, creating parents as needed.

- **Policy:** enforced.
- **Parent directory:** created with mode `0o755`.
- **Mode:** applied at creation; preserved on overwrite.
- **Non-atomic.** Use `WriteAtomic` for atomic replacement.
- **Consumer:** WBS 13.2.3.

#### `WriteAtomic(name string, data []byte, mode fs.FileMode) error` — L2 (atomic)

Atomic replacement.

- **Mechanism:** write to `<name>.tmp.<random>` in the same
  directory as `<name>`, `fsync` the temp file, `rename` it to
  `<name>`, `fsync` the parent directory.
- **Guarantee:** a reader sees either the old content or the new
  content, never a partial write. The guarantee holds for
  concurrent readers on the same filesystem.
- **Cleanup:** the temp file is removed if any step fails.
  A crash mid-operation may leave a temp file behind; the name
  pattern includes a random suffix so the leftovers are
  identifiable.
- **Cross-device:** the rename is same-directory by
  construction, so `EXDEV` is not reachable.
- **Consumer:** WBS 13.2.3 (Phase 4 render plan).

#### `Mkdir(name string, mode fs.FileMode) error` — L2

Creates a single directory.

- **Policy:** enforced.
- **Parent missing:** error wraps `fs.ErrNotExist`.
- **Existing directory:** error wraps `fs.ErrExist`. To create
  a directory idempotently, use `MkdirAll`.
- **Consumer:** WBS 13.2.3.

#### `MkdirAll(name string, mode fs.FileMode) error` — L2

Creates `name` and any missing parents.

- **Policy:** enforced.
- **Existing directory:** returns `nil`.
- **Existing non-directory:** error wraps `fs.ErrExist`.
- **Consumer:** WBS 13.2.3.

#### `Remove(name string) error` — L2

Removes a file or an empty directory.

- **Policy:** enforced.
- **Missing:** error wraps `fs.ErrNotExist`.
- **Non-empty directory:** error.
- **Consumer:** WBS 13.2.3.

#### `RemoveAll(name string) error` — L2 (atomic)

Recursively removes a subtree.

- **Policy:** enforced.
- **Root itself:** the operation refuses to remove the root.
  A name that resolves to `Root.Abs()` returns an error wrapping
  `ErrPathTraversal`. This is the only `Root` operation that
  treats the root as a forbidden target; every other operation
  treats it as a legal subject.
- **Missing:** returns `nil` (matching `os.RemoveAll`).
- **Consumer:** WBS 13.2.3 (cleanup of a stale render tree).

#### `RenameIfNotExists(src, dst string) error` — L2

Renames `src` to `dst`, failing if `dst` exists.

- **Policy:** enforced on both `src` and `dst`.
- **`dst` exists:** error wraps `fs.ErrExist`. The check and the
  rename happen in the same critical section, using
  `os.Link`+`os.Remove` or the platform equivalent, so the
  "if not exists" clause cannot be raced.
- **Cross-device:** fails with `EXDEV` on POSIX. The `Root`
  does not attempt to fall back to copy-then-delete, because
  that would break atomicity.
- **Consumer:** WBS 13.2.3 (avoid clobbering user edits).

### 4.3 Existence

#### `Exists(name string) (bool, error)` — L2

Reports whether `name` exists.

- **Policy:** enforced.
- **Missing:** `(false, nil)`.
- **Denied:** `(false, err)`.
- **Consumer:** WBS 13.6.1.

### 4.4 Hashing

#### `Hash(name string, algo HashAlgo) ([]byte, error)` — L2

Returns the digest of the file's content.

- **Supported algos:**
  - `HashSHA256` — the minimum required by WBS 13.1.2. This is
    the algorithm used by Phase 14 base-state hashing.
  - `HashSHA512` — available for callers that need a larger
    digest. Not used by any current consumer.
- **Streaming:** the file is opened with `Open` and read in
  64 KiB chunks. It is not loaded into memory whole.
- **Policy:** enforced by the `Open` call inside.
- **Unknown algo:** error wrapping `ErrInvalidOption`.
- **Consumer:** Phase 14 (base-state hashing).

### 4.5 Symlinks

#### `Symlink(target, link string) error` — L2 (restricted)

Creates a symlink at `link` whose target is `target`.

- **Policy:** enforced on `link`. **Additionally**, `target` is
  validated with `filepath.IsLocal` before any I/O. An absolute
  target, a target containing `..`, or a target that is
  otherwise non-local is refused with an error wrapping
  `ErrSymlinkDenied`.
- **Why the restriction:** an absolute target cannot be
  confined by relative-path reasoning; a `..` target can escape
  the root without the Root's policy noticing. Refusing both at
  creation time makes the symlink invariant hold for the
  Root's lifetime.
- **Why the operation exists:** the template engine reproduces
  symlinks found in a source tree, and a reproduced link must
  have the same target as the original. Refusing to create links
  would force the caller to shell out.
- **Consumer:** WBS 13.2.2 (template materialisation).

#### `Readlink(name string) (string, error)` — L2

Returns the stored target of a symlink.

- **Policy:** enforced.
- **Return value:** the link's stored target, exactly as
  `filesystem.FS.Readlink` returns it. The `Root` does not
  resolve it; the caller interprets the target in context.
- **Not a symlink:** platform-specific error.
- **Consumer:** WBS 13.2.2.

---

## 5. Rejected operations

Each operation below was considered and rejected. A future
proposal to add one must open an ADR that explains what changed.

### 5.1 `Chmod` — folded into `Write`

**Proposal:** add `Root.Chmod(name string, mode fs.FileMode) error`
so callers can change the mode of an existing file.

**Rejected because:** the mode is set at creation, and no
consumer in Phase 4, 7, 14, or 15 needs to change it afterward.
A caller that needs a file with a different mode rewrites it
with `WriteAtomic(name, data, newMode)`, which applies the new
mode as a side effect of the write.

**If this changes:** a future caller with a legitimate need to
change a mode without rewriting the content opens an ADR that
names the consumer and the security implications of changing a
file's permissions outside of its creation context.

### 5.2 `Chown` — never

**Proposal:** add `Root.Chown(name string, uid, gid int) error`.

**Rejected because:** a foundation manager does not own files.
Changing ownership requires privileges that a user-level CLI
should not request, and the operation has no consumer in any
phase.

**If this changes:** the change is not a matter of adding an
operation; it is a matter of redefining the trust model. That
is an ADR for a different WBS.

### 5.3 `Truncate` — never

**Proposal:** add `Root.Truncate(name string, size int64) error`.

**Rejected because:** `Write(name, []byte{}, mode)` expresses
the same intent for the only truncation a caller needs (to
zero), and truncating to a non-zero size is a data-loss
operation with no consumer.

**If this changes:** a caller with a concrete need for partial
truncation — for example, a log-rotation tool — opens an ADR
that names the consumer and the failure modes of a partial
truncation.

### 5.4 `Watch` — out of scope

**Proposal:** add `Root.Watch(name string, fn WatchFunc) error`
so callers can react to filesystem events.

**Rejected because:** Forge is a CLI, not a daemon. File
watching requires a long-running process, a different
concurrency model, and a different failure mode (the watcher
can miss events, the process can be interrupted mid-event, and
the caller's callback runs on a goroutine it does not own).

**If this changes:** a future subcommand that runs as a daemon
opens an ADR that names the daemon's lifecycle, its signal
handling, and its relationship to the CLI's exit codes.

### 5.5 `Chroot` — never

**Proposal:** add `Root.Chroot() error` so that the process can
be isolated to the Root's directory.

**Rejected because:** process-level isolation is orthogonal to
the file abstraction. See ADR-035, "Boundaries": a Root does
not attempt to be a chroot, and process isolation is a separate
concern tracked under the security model.

**If this changes:** a container-runtime integration is an ADR
in its own right, not an operation on `Root`.

### 5.6 `Copy` — not yet

**Proposal:** add `Root.Copy(src, dst string) error` for
recursive copies.

**Rejected because:** no consumer in Phase 4, 7, 14, or 15
needs a general-purpose copy. The renderer's output is written
with `Write` and `WriteAtomic`; the template engine materialises
templates with `Write`. A future consumer with a legitimate
recursive-copy need — for example, a `forge clone` subcommand —
opens an ADR that names the source, the destination, the
symlink policy, and the behaviour on partial failure.

**If this changes:** the ADR must also specify whether the copy
preserves modes, whether it follows symlinks, and whether it
is atomic.

### 5.7 `MkdirTemp` — rejected in favour of `WriteAtomic`

**Proposal:** add `Root.MkdirTemp(prefix string) (string, error)`
so callers can create temporary directories.

**Rejected because:** `WriteAtomic` is the only atomic-write
primitive a caller needs, and it manages its own temp file
internally. A public `MkdirTemp` would invite callers to create
temp directories outside the write path, with the cleanup
responsibility falling on the caller. The atomic write hides
that responsibility inside the one operation that needs it.

**If this changes:** a caller with a legitimate need for a
user-visible temporary directory opens an ADR that names the
consumer, the cleanup contract, and the interaction with the
`Root`'s symlink policy.

---

## 6. Traceability

Every operation above names a consumer. The table below is the
authoritative mapping; a row without a consumer is a bug in this
document.

| Operation | Layer | Consumer | Requirement |
|---|---|---|---|
| `Read` / `Open` / `ReadFile` | 1, 2 | WBS 13.2.1 | FR-FS-002 |
| `Write` | 1, 2 | WBS 13.2.3 | FR-FS-003 |
| `WriteAtomic` | 2 | WBS 13.2.3 (Phase 4) | FR-FS-004 |
| `Exists` | 1, 2 | WBS 13.6.1 | FR-FS-005 |
| `Mkdir` / `MkdirAll` | 1, 2 | WBS 13.2.3 | FR-FS-006 |
| `Remove` | 1, 2 | WBS 13.2.3 | FR-FS-007 |
| `RemoveAll` | 2 | WBS 13.2.3 | FR-FS-008 |
| `List` / `ReadDir` | 1, 2 | WBS 13.5.1, 13.6.1 | FR-FS-009 |
| `Stat` | 1, 2 | WBS 13.6.1 | FR-FS-010 |
| `Lstat` | 1 | WBS 13.2.2 | FR-FS-011 |
| `Readlink` | 1, 2 | WBS 13.2.2 | FR-FS-012 |
| `Walk` | 2 | Phase 7, Phase 14 | FR-FS-013 |
| `RenameIfNotExists` | 2 | WBS 13.2.3 | FR-FS-014 |
| `Hash` | 2 | Phase 14 | FR-FS-015 |
| `Symlink` | 2 | WBS 13.2.2 | FR-FS-016 |
| `Boundary` | 1 | WBS 13.1.1 | FR-FS-017 |
| `validateName` (internal) | 2 | every L2 method | FR-FS-018 |
| `resolveSymlinks` (internal) | 2 | every L2 method | FR-FS-019 |
| `detectCollisions` (internal) | 2 | `ReadDir` | FR-FS-020 |
| (all of the above) | — | NFR-API-001 | stability |

---

## 7. Stability policy

The surface is frozen. The following changes are compatible and
require only a normal PR:

- **Adding a new operation** that names a consumer and traces to
  a requirement in a later phase. The new operation adds a row
  to this document, a row to the corresponding `api.go` file, a
  test row to the corresponding `api_test.go`, and the
  implementation. The ADR-035 review is the only gate.
- **Changing an implementation** of an existing operation, as
  long as the operation's documented contract does not change.
  A change that alters the behaviour on a documented input is
  not compatible.
- **Adding a parameter** that is optional. This is only possible
  in Go with a variadic or an options struct; the surface does
  not currently have either, so adding a parameter is a
  breaking change in practice.

The following changes are **incompatible** and require an ADR:

- **Removing an operation.**
- **Renaming an operation.**
- **Changing an operation's signature.**
- **Changing an operation's documented behaviour** on any input
  named in its row.
- **Adding an operation** that does not name a consumer, or that
  names a consumer in a phase that does not yet exist.

The stability policy exists for the same reason the log format's
does (see ADR-025): a caller who writes code against the
documented surface must be able to rely on it without reading
the implementation.

---

## 8. See also

- [`docs/architecture/filesystem-layering.md`](../architecture/filesystem-layering.md)
  — the two-layer model, the enforcement-by-construction
  argument, and the CI lint that enforces the layering.
- [`docs/decisions/ADR-035-filesystem-enforcement-by-construction.md`](../decisions/ADR-035-filesystem-enforcement-by-construction.md)
  — the decision record that ratifies the two-layer model.
- [`docs/decisions/ADR-011-closed-error-category-taxonomy.md`](../decisions/ADR-011-closed-error-category-taxonomy.md)
  — the closure argument for the error taxonomy, which the
  operation surface mirrors.
- [`docs/decisions/ADR-013-error-wrapping-contract.md`](../decisions/ADR-013-error-wrapping-contract.md)
  — the wrapping contract that makes `errors.Is(err, fs.ErrNotExist)`
  work through every layer.
- [`docs/reference/output-streams.md`](output-streams.md) —
  the stream contract that governs how filesystem errors are
  reported to the user.
- [`docs/reference/error-codes.md`](error-codes.md) — the
  error codes that the filesystem sentinels map onto.

---

## 9. Change log

| Date | Change | Author |
|------|--------|--------|
| 2025-01-15 | Initial freeze; ratifies WBS 13.1.2 | Architecture Owner |
