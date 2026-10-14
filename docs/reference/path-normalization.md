# Path Normalization & Canonicalization Semantics

- **Status:** Frozen
- **WBS:** 13.3.1
- **ADR:** ADR-035
- **Traces:** SEC-FS-011, FR-FS-023
- **Last reviewed:** 2025-01-15
- **Review cadence:** per platform-matrix change
- **Supersedes:** —

## Purpose

This document specifies exactly how `safefs` transforms a
caller-supplied name before using it to touch the filesystem. It
exists so that:

1. A reader can predict, from the documentation alone, what
   `safefs` will do with any input.
2. A contributor adding a name-handling feature consults the
   specification rather than reverse-engineering the code.
3. A security reviewer audits the enforcement by reading one
   document and comparing it to one function.

The specification is **normative**. Where the code and this
document disagree, the code is wrong and the document is the
target. Where the document is silent, the code is free.

## Audience and Scope

This document is written for two audiences:

- **Contributors** who need to know what a name means before
  they use it, extend the pipeline, or write a test.
- **Security reviewers** who need to verify that the enforcement
  matches the specification without reading every source file.

It covers the four layers of name handling, the platform-specific
rules that Layer 1 applies, and the decision table that the test
suite enforces.

It does **not** cover:

- The `FS` interface's boundary semantics (see WBS 13.1.1 and
  the `internal/filesystem` package doc). The `FS` boundary is
  the directory the FS was constructed with; the `Root` boundary
  is the directory the `Root` was constructed with. They are
  conceptually distinct. In production they coincide, because
  `safefs.New` constructs the FS with the `Root`'s absolute
  path. A test that constructs the two independently may have
  different boundaries, and that is a deliberate property of
  Layer 1.
- The `Walk` depth and entry bounds (see the `Walk` method's
  doc comment).
- The symlink policy's specific algorithms (see
  `internal/filesystem/safefs/symlink.go` and ADR-035). This
  document summarises their role in the pipeline.

## The Four Layers

Every name that enters `safefs` traverses four layers, in order.
Each layer's guarantee depends on the previous layer's. The
order is deliberate: it is cheap-first, so a name that is going
to fail never reaches the layers that cost syscalls.

```text
caller name
    │
    ▼
┌───────────────────────────────────────────────────────────┐
│ Layer 1 — Lexical normalization                           │
│ Pure string work. No syscalls.                            │
│ Output: cleaned, forward-slash, local relative name.      │
└───────────────────────────────────────────────────────────┘
    │
    ▼
┌───────────────────────────────────────────────────────────┐
│ Layer 2 — Boundary check                                  │
│ Folded into Layer 1 via filepath.IsLocal.                 │
│ Output: accept or reject.                                 │
└───────────────────────────────────────────────────────────┘
    │
    ▼
┌───────────────────────────────────────────────────────────┐
│ Layer 3 — Symlink resolution on the parent directory      │
│ Syscalls (Lstat, Readlink). Bounded by MaxSymlinkChain.   │
│ Output: resolved parent inside the root, or an error.     │
└───────────────────────────────────────────────────────────┘
    │
    ▼
┌───────────────────────────────────────────────────────────┐
│ Layer 4 — Filesystem canonicalization                     │
│ NOT USED by safefs. Documented for completeness.          │
└───────────────────────────────────────────────────────────┘
    │
    ▼
resolved name handed to the underlying FS
```

### Layer 1 — Lexical normalization

**Cost:** pure string work. No syscalls.

**Input:** a caller-supplied string.

**Output:** a cleaned, forward-slash, local relative name, or an
error.

**Algorithm:**

```text
1. If the input is the empty string, return ErrNotLocal.

2. Replace every backslash (\) with a forward slash (/).
   Rationale: Windows callers use backslashes; the FS
   interface documents forward-slash names. The replacement
   makes the same call site work on both platforms.

3. Clean with path.Clean.
   Rationale: path.Clean is platform-independent. It always
   produces forward slashes, collapses "." and repeated "/",
   and resolves ".." lexically. It does NOT consult the
   filesystem.

4. If the cleaned name is ".", return "." (the root itself).

5. If filepath.IsLocal(cleaned) is false:
   a. If the cleaned name contains a ".." segment, return
      ErrPathTraversal.
   b. Otherwise, return ErrNotLocal.

6. Return the cleaned name.
```

**Guarantees:**

- The output is forward-slash.
- The output contains no `.` segments (except the single name
  `.`).
- The output contains no repeated `/`.
- The output does not begin with `/`.
- The output does not contain a `..` segment.
- The output does not contain a Windows drive letter prefix.
- The output does not contain a UNC prefix.
- The output is deterministic: the same input produces the same
  output on every call, on every platform.

**Non-guarantees:**

- It does not touch the filesystem.
- It does not resolve symlinks.
- It does not check the name against the root's physical path.
- It does not canonicalize case.
- It does not normalize Unicode.
- It does not strip trailing dots or spaces.

**Why `path.Clean` and not `filepath.Clean`:** `filepath.Clean`
uses the host's separator and can introduce a drive letter on
Windows. `path.Clean` is platform-independent. The output is
always the forward-slash form the FS interface documents.

**Why backslash replacement before cleaning:** `path.Clean` treats
a backslash as a literal character, not a separator. On Windows a
caller writing `a\b.txt` expects the path to have two components.
The replacement makes the two components visible to the cleaner.
On Unix the replacement is a no-op for well-formed inputs; for
an input that legitimately contains a backslash in a filename
(rare, but possible), the abstraction converts it to a separator,
which is the documented behaviour.

### Layer 2 — Boundary check

**Cost:** pure string work on the layer-1 output. No syscalls.

**Input:** a cleaned relative name.

**Output:** a decision — proceed or reject.

**Algorithm:**

```text
1. If the name is ".", it refers to the root itself. Proceed.
2. The name is local (guaranteed by Layer 1). Proceed.
```

**Guarantees:**

- A name that passed Layer 1 is inside the root at the lexical
  level.
- The lexical form does not escape.

**Non-guarantees:**

- It does not resolve symlinks. A name that is lexically inside
  the root can still resolve outside via a symlink on the path.
  Layer 3 handles that.
- It does not check the name against the physical filesystem.
  A name can be lexically inside the root and still point at a
  file outside via a symlink at an intermediate component.

**Why this layer is folded into Layer 1:** `filepath.IsLocal`
performs both checks in one predicate: it rejects `..` segments
and rejects absolute paths. The two are conceptually distinct —
one is "not traversing up," the other is "not starting from the
root" — but they share an implementation and a single error
site. The distinction matters for a future policy that wants to
allow `..` up to a sub-boundary but not past the root; the
implementation would have to split the check.

### Layer 3 — Symlink resolution

**Cost:** one or more `Lstat` syscalls per component of the
parent path, plus a `Readlink` per symlink component. Bounded by
`MaxSymlinkChain`.

**Input:** a cleaned relative name that passed Layer 2.

**Output:** a resolved relative name whose parent's symlinks
have been resolved under the Root's policy, or an error.

**Algorithm:**

```text
1. Split the name into parent and base:
   - parent: everything before the last "/"
   - base:   the last component
   A name with no "/" has an empty parent.

2. If the parent is empty or ".", the parent is the root itself.
   No resolution is needed. Return the name unchanged.

3. Apply the Root's symlink policy to the parent:
   a. Under NoFollow, verify that no component of the parent is
      a symlink. If any is, return ErrSymlinkDenied.
   b. Under FollowInRoot, resolve each symlink component and
      verify that the resolved parent is inside the root. If
      any resolution escapes, return ErrSymlinkDenied. Bound
      the chain by MaxSymlinkChain.

4. Return the resolved parent joined with the base.
```

**Guarantees:**

- The parent's symlinks have been resolved under the policy.
- The resolved parent is inside the root.
- The base (final component) is unchanged.

**Non-guarantees:**

- It does not resolve the base. The base may not exist (write
  operations create it), and resolving it would require a
  syscall whose result the operation would then re-check.
- It does not use `filepath.EvalSymlinks`. That function
  resolves the entire path including the base, uses the
  process's CWD for relative inputs, and cannot express
  NoFollow.
- It does not close the TOCTOU window between the parent's
  resolution and the operation's syscall. See "TOCTOU Posture"
  below.

**Why the parent and not the whole path:** resolving the whole
path would require every component to exist. A write operation
creating `a/b/c.txt` where `a/b` exists but `c.txt` does not
cannot resolve the whole path. Resolving the parent (`a/b`) is
enough to defend against a symlink-swap on the containing
directory, which is the attack vector. The final component is
created or opened by the operation itself; if it is a symlink,
the operation's resolution (for reads) or the OS's behaviour
(for creates) handles it.

### Layer 4 — Filesystem canonicalization

**Cost:** OS-dependent. `filepath.EvalSymlinks` on Unix;
`GetFinalPathNameByHandle` on Windows.

**Input:** a resolved relative name.

**Output:** the OS-provided real path of the file the name
refers to.

**Where it is used:** nowhere in `safefs`'s enforcement. The
layer exists in this specification because it is the layer a
reader might expect to find, and the specification must say
explicitly that `safefs` does not use it.

**Why not used:**

- `filepath.EvalSymlinks` on Unix resolves every component
  including the base, which is wrong for writes and which the
  policy cannot control. It is also documented to be
  non-deterministic in the presence of concurrent filesystem
  changes.
- `GetFinalPathNameByHandle` on Windows requires an open
  handle, which means the path is already resolved and the
  enforcement has already happened.
- Both are substantially more expensive than Layers 1–3 and
  provide no additional guarantee that those layers do not.

**Where a caller might use it:** a caller that wants to display
the real path of a file (for example, a diagnostic that says
"this file is actually at /tmp/xyz") can call
`filepath.EvalSymlinks` itself. That is a display concern, not
an enforcement concern, and it is outside the abstraction.

## The Algorithm End to End

For a name `n` and a Root `r`:

```text
1. c := lexicalNormalize(n)              // Layer 1
   If error, return error.

2. // Layer 2 is folded into Layer 1; the check is
   // filepath.IsLocal, invoked from lexicalNormalize.

3. p := resolveParent(c, r.policy)       // Layer 3
   If error, return error.

4. result := p.parent + "/" + p.base

5. return result                          // Layer 4 not used
```

The layers are cheap-first. A name that fails Layer 1 or Layer 2
never triggers a syscall. A name that reaches Layer 3 costs at
least one syscall per component of the parent.

## TOCTOU Posture

`safefs` is **best-effort** against an attacker with concurrent
filesystem access. The layers narrow the TOCTOU (Time-of-Check
to Time-of-Use) window but do not close it.

**What is guaranteed:**

- No lexical traversal reaches the filesystem. A name that
  contains `..` after cleaning is refused before any syscall.
- A symlink-swap on the parent path is detected and refused.
  The parent is resolved between the lexical check and the
  operation's syscall; if the resolution escapes the root, the
  operation is refused.
- The bounds (`MaxDepth`, `MaxEntries`, `MaxSymlinkChain`) are
  enforced.

**What is not guaranteed:**

- The final component cannot be swapped between the parent's
  resolution and the operation's syscall. Closing that window
  would require an atomic "resolve and open relative to a
  directory descriptor" primitive. Linux provides `openat2(2)`
  with `RESOLVE_BENEATH`; macOS and Windows do not provide an
  equivalent. A cross-platform abstraction cannot rely on it.
- A file created by the operation is not guaranteed to survive
  a concurrent `remove`. The operation may fail with
  `ENOENT`; the failure is reported honestly.

**What a caller can do:**

- A caller that needs a stronger guarantee holds the parent
  directory open and operates relative to the descriptor. This
  is outside `safefs`'s abstraction and is not required by any
  Forge use case.
- A caller that operates on a tree under its own exclusive
  control (a project directory the user owns) does not need the
  stronger guarantee.

## Platform-Specific Rules

The rules below are enforced by Layer 1's `filepath.IsLocal` or
by the OSFS implementation in `internal/filesystem`. They are
listed here so a reader knows what to expect without reading the
`os` package's documentation.

### Windows

Windows has the largest number of quirks. Every rule below is
enforced by Layer 1 or by the OSFS implementation.

#### Case insensitivity

Windows filesystems are case-insensitive by default. Two names
that differ only by case refer to the same file. `safefs`
enforces case-insensitivity by:

- Not adding case-folding to `validateName`. The FS layer
  handles it.
- Offering `WithCaseCollisionDetection(true)` on the Root,
  which makes `ReadDir` reject directories that contain two
  names differing only by case.

Together, the two give the caller a predictable answer on a
case-insensitive filesystem: the FS layer resolves names
case-insensitively, and the collision detection reports when a
directory contains a name that the FS would silently shadow.

#### Drive letters

Windows paths may be absolute (`C:\...`), drive-relative
(`C:foo`), or drive-absolute without a root (`C:\foo`).
`filepath.IsLocal` rejects all three:

- `C:\foo` is absolute. Rejected with `ErrNotLocal`.
- `C:foo` is drive-relative. Rejected with `ErrNotLocal`.
- `C:\` is a root directory. Rejected with `ErrNotLocal`.

A caller that needs to operate on a specific drive constructs
the Root with that drive as the boundary.

#### UNC paths

`\\server\share\path` is an absolute path on Windows.
`filepath.IsLocal` rejects it with `ErrNotLocal`. A caller that
needs to operate on a UNC path constructs the Root with that
UNC path as the boundary.

#### Reserved names

Windows reserves a set of device names that cannot be used as
file or directory names, regardless of extension:

```text
CON, PRN, AUX, NUL,
COM1, COM2, ..., COM9,
LPT1, LPT2, ..., LPT9
```

The names are reserved in every directory, with or without an
extension. `CON.txt` is reserved; `CON` is reserved; `con` is
reserved (case-insensitive).

`filepath.IsLocal` rejects these names. The rejection is
`ErrNotLocal`, because the name is malformed rather than a
traversal attempt.

`COM10` and higher are **not** reserved. The reserved set is
exactly the nine COM ports and the nine LPT ports.

#### Trailing dots and spaces

Windows strips trailing dots and spaces from filenames at the
API boundary. A file created as `foo.` is stored as `foo`. A
file created as `foo` is stored as `foo`.

`filepath.IsLocal` does not reject trailing dots and spaces.
The OS handles them. `safefs` documents the behaviour here so
a caller is not surprised: a name that ends with a dot or a
space may refer to a different file than the caller intended.

A caller that wants the stricter behaviour (reject trailing
dots and spaces) can construct the Root with a policy that adds
the check. `safefs` does not add it by default because the
behaviour is platform-specific and rejecting the names on Unix
would be surprising.

#### Long paths

Windows paths longer than 260 characters require the `\\?\`
prefix. The prefix is applied by `internal/filesystem`'s
`prepareForOS` (see WBS 13.2.1). It is transparent to `safefs`:
the caller passes a short name, the FS layer joins it with the
boundary, the long-path handler adds the prefix if needed.

The threshold used by `prepareForOS` is 240 characters, which
is below `MAX_PATH` (260) to leave room for suffixes the OS and
third-party tools append. See `internal/filesystem/
longpath_windows.go` for the implementation.

### macOS

macOS has two quirks that affect path handling.

#### Case insensitivity (APFS default)

APFS, the default filesystem on modern macOS, is case-insensitive
by default. The same behaviour as Windows applies: `safefs`
relies on the FS layer for case-insensitive resolution and
offers `WithCaseCollisionDetection(true)` for detecting shadowed
names.

A case-sensitive APFS volume is available as an option, and the
case-collision check is a no-op on such a volume (there is no
collision to detect).

#### NFD vs NFC Unicode normalization

HFS+ normalizes filenames to NFD (canonical decomposition)
before storing them. APFS preserves the caller's form but
compares using NFC (canonical composition). The result:

- A caller that creates a file named `café` (NFC) and later
  looks it up as `café` (NFD) may or may not find it, depending
  on the filesystem and the API.
- Two names that are canonically equivalent (`café` NFC vs NFD)
  are treated as the same file on macOS.

`safefs` does not normalize Unicode. A caller that needs
portable behaviour uses NFC consistently. The specification
documents the behaviour here so a caller is not surprised.

A caller that wants to detect canonically-equivalent names
constructs the Root with a policy that adds the check. `safefs`
does not add it by default because the normalization forms are
platform-specific and rejecting NFD names on Linux would be
surprising.

### Linux

Linux filesystems are byte-oriented. Two names that differ in
any byte are different files. `safefs` does not add any
normalization:

- Case is significant.
- Unicode normalization is not performed.
- Trailing dots and spaces are significant.
- Reserved names do not exist (a file named `CON` is legal).

The `WithCaseCollisionDetection(true)` option is a no-op on
Linux; there is no collision to detect.

## The Decision Table

The table below enumerates the documented decisions. Every row
has a corresponding test in `internal/filesystem/safefs/
normalize_test.go` (Layer 1 rows), `normalize_unix_test.go`
(Unix rows), `normalize_windows_test.go` (Windows rows), or
`normalize_darwin_test.go` (macOS rows). The "Layer" column
names the layer that makes the decision; the "Platform" column
names the platform on which the row is meaningful (`.` for any).

| #  | Input                            | Layer | Platform | Decision |
|----|----------------------------------|-------|----------|----------|
| 1  | `""`                             | 1     | .        | ErrNotLocal |
| 2  | `"."`                            | 1     | .        | Accept, cleaned to `.` |
| 3  | `"a"`                            | 1     | .        | Accept, cleaned to `a` |
| 4  | `"a/b"`                          | 1     | .        | Accept, cleaned to `a/b` |
| 5  | `"a//b"`                         | 1     | .        | Accept, cleaned to `a/b` |
| 6  | `"a/./b"`                        | 1     | .        | Accept, cleaned to `a/b` |
| 7  | `"a/../b"`                       | 1     | .        | Accept, cleaned to `b` |
| 8  | `"a/b/.."`                       | 1     | .        | Accept, cleaned to `a` |
| 9  | `"a/.."`                         | 1     | .        | Accept, cleaned to `.` |
| 10 | `".."`                           | 1     | .        | ErrPathTraversal |
| 11 | `"../a"`                         | 1     | .        | ErrPathTraversal |
| 12 | `"a/../../b"`                    | 1     | .        | ErrPathTraversal |
| 13 | `"/a"`                           | 1     | .        | ErrNotLocal |
| 14 | `"/"`                            | 1     | .        | ErrNotLocal |
| 15 | `"./a"`                          | 1     | .        | Accept, cleaned to `a` |
| 16 | `"./../a"`                       | 1     | .        | ErrPathTraversal |
| 17 | `"..foo"`                        | 1     | .        | Accept, cleaned to `..foo` |
| 18 | `"foo.."`                        | 1     | .        | Accept, cleaned to `foo..` |
| 19 | `"foo."`                         | 1     | Windows  | Accept; OS strips trailing dot |
| 20 | `"foo "`                         | 1     | Windows  | Accept; OS strips trailing space |
| 21 | `"a\\b"`                         | 1     | .        | Accept, cleaned to `a/b` |
| 22 | `"a\\\\b"`                       | 1     | .        | Accept, cleaned to `a/b` |
| 23 | `"C:\\foo"`                      | 1     | Windows  | ErrNotLocal |
| 24 | `"C:foo"`                        | 1     | Windows  | ErrNotLocal |
| 25 | `"C:\\"`                         | 1     | Windows  | ErrNotLocal |
| 26 | `"\\\\server\\share\\a"`         | 1     | Windows  | ErrNotLocal |
| 27 | `"CON"`                          | 1     | Windows  | ErrNotLocal |
| 28 | `"con.txt"`                      | 1     | Windows  | ErrNotLocal |
| 29 | `"NUL"`                          | 1     | Windows  | ErrNotLocal |
| 30 | `"AUX"`                          | 1     | Windows  | ErrNotLocal |
| 31 | `"COM1"`                         | 1     | Windows  | ErrNotLocal |
| 32 | `"COM9"`                         | 1     | Windows  | ErrNotLocal |
| 33 | `"LPT1"`                         | 1     | Windows  | ErrNotLocal |
| 34 | `"COM10"`                        | 1     | Windows  | Accept (`COM10` is not reserved) |
| 35 | `"a/b"` where `a` is an in-root symlink | 3 | .     | FollowInRoot: resolve, accept |
| 36 | `"a/b"` where `a` is an out-of-root symlink | 3 | . | FollowInRoot: ErrSymlinkDenied |
| 37 | `"a/b"` where `a` is a symlink (any target) | 3 | . | NoFollow: ErrSymlinkDenied |
| 38 | `"a"` where `a` is an in-root symlink to a file | 3 | . | FollowInRoot: resolve, accept |
| 39 | `"a"` where `a` is an out-of-root symlink to a file | 3 | . | FollowInRoot: ErrSymlinkDenied |
| 40 | `"café"` (NFC) vs `"café"` (NFD) | 4 | macOS | Same file on APFS; distinct on Linux |

**Row count: 40.** The WBS requires at least 30; the table
exceeds that.

### Notes on individual rows

- **Rows 7–9** (`a/../b`, `a/b/..`, `a/..`). The `..` resolves
  lexically before the boundary check, so the cleaned form is
  what is checked. A reader who expects `..` to be rejected
  anywhere in the input should note that the specification's
  position is that the *cleaned* form is what matters, not the
  raw input. A caller who writes `a/../b` intends `b`; rejecting
  it would be rejecting a spelling, not a behaviour.

- **Rows 10–12** (`..`, `../a`, `a/../../b`). The cleaned form
  begins with `..` and is therefore outside the root.
  `filepath.IsLocal` rejects it; `containsTraversal` classifies
  it as traversal rather than malformed.

- **Rows 13–14** (`/a`, `/`). Absolute paths. `filepath.IsLocal`
  rejects them. The classification is `ErrNotLocal`, not
  `ErrPathTraversal`, because they are malformed rather than
  traversal attempts.

- **Row 15** (`./a`). The leading `./` is a no-op after
  cleaning.

- **Rows 17–18** (`..foo`, `foo..`). A file whose name begins
  or ends with two dots is legal on every supported platform.
  The check is a segment comparison, not a substring search.

- **Rows 19–20** (`foo.`, `foo`). The OS strips trailing dots
  and spaces on Windows; elsewhere they are significant. The
  row documents the behaviour without asserting a specific
  outcome.

- **Rows 21–22** (`a\b`, `a\\b`). Backslashes are normalized to
  forward slashes before cleaning. On Unix the normalization
  is a no-op for well-formed inputs; on Windows it is required.

- **Rows 23–26** (the four Windows absolute forms). All are
  rejected with `ErrNotLocal`, not `ErrPathTraversal`, because
  they are malformed rather than traversal attempts.

- **Rows 27–34** (Windows reserved names). The reserved set is
  `CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`.
  The names are reserved with or without an extension.
  `COM10` is not reserved.

- **Rows 35–39** (symlink policy). The policy is applied to the
  parent by Layer 3 and to the final component by the operation
  itself. The rows document the composition.

- **Row 40** (NFD/NFC). The only row that depends on the
  filesystem's Unicode behaviour, not on `safefs`. The row
  documents the behaviour without asserting a `safefs` decision.

## Enforcement and Tests

Every row of the decision table has a corresponding test. The
tests are structured to mirror the table:

| Layer | Test file |
|-------|-----------|
| 1     | `internal/filesystem/safefs/normalize_test.go` |
| 1 (Windows) | `internal/filesystem/safefs/normalize_windows_test.go` |
| 1 (macOS)   | `internal/filesystem/safefs/normalize_darwin_test.go` |
| 1 (Unix)    | `internal/filesystem/safefs/normalize_unix_test.go` |
| 3     | `internal/filesystem/safefs/symlink_unix_test.go`, `symlink_windows_test.go` |
| 4     | Not used; no test. |

A change to the specification without a corresponding change to
the tests, or vice versa, is a drift. The test file's header
comment names the decision table row for each test, so a
reviewer can trace either direction.

## Change History

| Date | Change | Author |
|------|--------|--------|
| 2025-01-15 | Initial version; ratifies WBS 13.3.1 | Security + Architecture Owners |

## See Also

- [ADR-035 — Filesystem Enforcement by Construction](../decisions/ADR-035-filesystem-enforcement-by-construction.md)
- [Filesystem Layering](./../architecture/filesystem-layering.md)
- [Filesystem API Reference](./filesystem-api.md)
- [internal/filesystem/safefs package doc](../../internal/filesystem/safefs/doc.go)
- [internal/filesystem package doc](../../internal/filesystem/filesystem.go)
- Go stdlib: [`path.Clean`](https://pkg.go.dev/path#Clean),
  [`filepath.IsLocal`](https://pkg.go.dev/path/filepath#IsLocal),
  [`filepath.EvalSymlinks`](https://pkg.go.dev/path/filepath#EvalSymlinks)
- POSIX: [`realpath(3)`](https://pubs.opengroup.org/onlinepubs/9699919799/functions/realpath.html)
- CWE-22, CWE-59, CWE-61
