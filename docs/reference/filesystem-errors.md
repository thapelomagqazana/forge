# Filesystem Errors

- **Status:** Frozen
- **WBS:** 13.5.1
- **ADR:** ADR-011, ADR-012, ADR-013, ADR-035
- **Traces:** FR-FS-024, SEC-FS-016, NFR-ERROR-001
- **Last reviewed:** 2025-01-15

## Purpose

This document specifies the error taxonomy that `safefs`
produces and the existing-file policy that its write operations
follow. It exists so that:

1. A caller can predict the error for any operation and any
   target state.
2. A contributor adding a new operation knows which sentinel to
   return.
3. A reviewer auditing the error surface can check the mapping
   by reading one document and comparing it to one file
   (`internal/filesystem/safefs/errors.go`).

## The Taxonomy

`safefs` produces eleven sentinel errors, divided into two
families.

### Boundary sentinels

Produced by the pipeline before any filesystem access.

| Sentinel | Condition | Exit |
|---|---|---|
| `ErrNotLocal` | The name is not a local, forward-slash, cleaned relative path. | 12 |
| `ErrPathTraversal` | The name attempts to lexically escape. | 13 |
| `ErrSymlinkDenied` | The symlink policy refuses a link. | 13 |
| `ErrDenyList` | The name matches the Root's deny-list. | 13 |
| `ErrNotAllowed` | The name does not match the Root's allow-list. | 13 |
| `ErrSubBoundary` | The name escapes a Handle's sub-boundary. | 13 |
| `ErrBoundaryViolation` | Umbrella sentinel for boundary violations. | 13 |

### OS sentinels

Produced by `classify` when the underlying FS returns an
OS-level error.

| Sentinel | Condition | Exit |
|---|---|---|
| `ErrNotFound` | The target does not exist. | 12 |
| `ErrExists` | The target exists and the operation is fail-closed. | 12 |
| `ErrPermissionDenied` | The process lacks permission. | 12 |
| `ErrReadOnly` | The filesystem is mounted read-only. | 12 |
| `ErrIsDirectory` | The operation expected a file, found a directory. | 12 |
| `ErrNotDirectory` | The operation expected a directory, found a file. | 12 |
| `ErrSymlinkLoop` | Symlink resolution exceeded the bound. | 12 |
| `ErrReservedName` | The name is reserved by the OS (Windows). | 12 |
| `ErrPathTooLong` | The path exceeds the OS's maximum length. | 12 |
| `ErrNameCollision` | Two names collide under Unicode case folding. | 12 |

### The four fields

Every `safefs` error carries four fields, attached via
`forgeerr.Error`'s context, cause, and remediation mechanisms:

- **operation** — the name of the `Root` or `Handle` method
  that produced the error. Example: `"Open"`.
- **path** — the cleaned relative name the operation acted on.
  Example: `"src/main.go"`.
- **cause** — the underlying error, if any. Example:
  `syscall.ENOENT`.
- **remediation** — a single-line hint. Example: `"Check the
  path and try again."`.

A caller that logs the error sees all four. A caller that
branches on the sentinel uses `errors.Is`. A caller that maps
to an exit code uses `errors.Is(err, ErrPathTraversal)` (exit
13) or the sentinel's category (exit 12 for filesystem
sentinels).

## The Existing-File Policy Matrix

The following matrix specifies what each write operation does
when the target exists.

| Operation | Target is file | Target is directory | Target missing |
|---|---|---|---|
| `Write` | `ErrExists` | `ErrExists` | create |
| `WriteOverwrite` | truncate | `ErrIsDirectory` | create |
| `Mkdir` | `ErrExists` | no-op | create |
| `MkdirAll` | `ErrExists` | no-op | create parents |
| `Remove` | delete | delete if empty | `ErrNotFound` |
| `RemoveIfExists` | delete | delete if empty | no-op |
| `RemoveAll` | delete | recurse | no-op |
| `Rename` | `ErrExists` | `ErrExists` | `ErrNotFound` for src |
| `RenameIfNotExists` | `ErrExists` | `ErrExists` | rename |
| `RenameOverwrite` | replace | replace | rename |

### The fail-closed default

Every operation that could destroy content the caller did not
ask to destroy is fail-closed:

- `Write` fails if the target exists.
- `Rename` fails if the destination exists.
- `Mkdir` fails if the target exists as a file.

The fail-closed default is not negotiable at the call site.
The only way to overwrite is to use the explicit variant
(`WriteOverwrite`, `RenameOverwrite`) or to remove the target
first (with `Remove` or `RemoveIfExists`).

### Why `Mkdir`/`MkdirAll` are no-ops for existing directories

The distinction is between "the target exists and is the right
kind of thing" and "the target exists and is the wrong kind of
thing."

- `Mkdir("src")` where `src` is a directory: the caller asked
  for `src` to exist as a directory, and it does. No-op.
- `Mkdir("src")` where `src` is a file: the caller asked for
  `src` to exist as a directory, and it does not. `ErrExists`.

This matches `os.MkdirAll`'s behaviour and the expectation a
caller who uses `MkdirAll` has.

### Why `Remove` is not fail-closed for existing targets

The rule is not "always fail when the target exists"; it is
"never destroy content the caller did not ask to destroy."
`Remove` is the caller asking. `Remove` on a missing target
returns `ErrNotFound` because the caller's intent was for the
file to be gone and it is not (it was never there); the
diagnostic is honest.

A caller that wants "remove if present" uses
`RemoveIfExists`, which is a no-op for a missing target.

### Why `Rename` is fail-closed for existing destinations

A `Rename` that replaces an existing destination destroys the
destination's content. That is the same data-loss condition as
a silent `Write` overwrite. The fail-closed default makes the
loss explicit; the caller either chooses a different
destination or uses `RenameOverwrite`.

`RenameIfNotExists` is a synonym for `Rename` that makes the
fail-closed semantics visible at the call site. It exists
because a reviewer reading `Rename(src, dst)` cannot tell
whether the fail-closed behaviour is intended or accidental;
`RenameIfNotExists(src, dst)` says so.

### Overwrite-with-confirmation is not this layer's concern

The WBS is explicit: "overwrite-with-confirmation is a
render-plan concern (Phase 4), not a filesystem-layer
concern — the filesystem layer always fails-closed; the
Render Plan engine decides whether to prompt."

The filesystem layer has exactly two behaviours for an
existing target: fail (`Write`) or overwrite (`WriteOverwrite`).
A caller that wants to prompt the user before overwriting
implements the prompt in the caller's layer:

1. Call `Exists` (or `Stat`) to determine whether the target
   exists.
2. If it exists, prompt the user.
3. If the user confirms, call `WriteOverwrite`.
4. If the user declines, do nothing.

The four steps live in the caller (the render-plan engine),
not in `safefs`. `safefs` provides the primitives; the
caller composes them.

## Cross-Platform Mapping

The `classify` function translates OS-level errors to the
taxonomy. The mapping is:

| POSIX | Windows | Sentinel |
|---|---|---|
| `ENOENT` | `ERROR_FILE_NOT_FOUND`, `ERROR_PATH_NOT_FOUND` | `ErrNotFound` |
| `EEXIST` | `ERROR_FILE_EXISTS`, `ERROR_ALREADY_EXISTS` | `ErrExists` |
| `EACCES`, `EPERM` | `ERROR_ACCESS_DENIED` | `ErrPermissionDenied` |
| `EROFS` | `ERROR_WRITE_PROTECT` | `ErrReadOnly` |
| `EISDIR` | `ERROR_ACCESS_DENIED` (directory targeted) | `ErrIsDirectory` |
| `ENOTDIR` | `ERROR_DIRECTORY` | `ErrNotDirectory` |
| `ELOOP` | `ERROR_CANT_RESOLVE_FILENAME`, `ERROR_TOO_MANY_LINKS` | `ErrSymlinkLoop` |
| `ENAMETOOLONG` | `ERROR_FILENAME_EXCED_RANGE`, `ERROR_PATH_NOT_FOUND` (long) | `ErrPathTooLong` |
| — | reserved name (`CON`, `NUL`, ...) | `ErrReservedName` |
| — | two names collide by case | `ErrNameCollision` |

The two rows marked "—" are Windows-only: the reserved-name
set and the case-insensitive collision detection do not exist
on POSIX. On POSIX, the corresponding sentinels are
unreachable.

### Errors without a sentinel

Some OS-level errors have no sentinel in the taxonomy. `classify`
wraps them as `ErrInternal` (the umbrella internal-error
sentinel from `forgeerr`) with the underlying error preserved.
The `ErrInternal` sentinel maps to exit 70 (`ExitInternal`).

A caller that receives `ErrInternal` has found a case the
taxonomy does not cover. The error is still well-formed (four
fields, stable code), so the CLI can display it, but the
sentinel is less specific. Filing the case as a follow-up
review is appropriate.

## Change History

| Date | Change | Author |
|------|--------|--------|
| 2025-01-15 | Initial version; ratifies WBS 13.5.1 | Core Runtime Owner |

## See Also

- [ADR-011 — Closed Error Category Taxonomy](../decisions/ADR-011-closed-error-category-taxonomy.md)
- [ADR-012 — Error Code Registry and Stability Policy](../decisions/ADR-012-error-code-registry-and-stability-policy.md)
- [ADR-013 — Error Wrapping Contract](../decisions/ADR-013-error-wrapping-contract.md)
- [Filesystem API Reference](./filesystem-api.md)
- [Boundary Model](./boundary-model.md)
- [Path Normalization & Canonicalization](./path-normalization.md)
