# Boundary Model

- **Status:** Frozen
- **WBS:** 13.4.1
- **ADR:** ADR-035
- **Traces:** SEC-FS-012 through SEC-FS-015
- **Last reviewed:** 2025-01-15
- **Review cadence:** per-phase
- **Supersedes:** —

## Purpose

This document specifies the boundary model that `safefs`
enforces. It answers the question "given a request to read or
write a path, what is the outcome?" for every combination of
boundary, sub-boundary, deny-list, and allow-list state.

The model is **normative**. Where the code and this document
disagree, the code is wrong and this document is the target.
Where the document is silent, the code is free.

## Audience and Scope

This document is written for three audiences:

- **Contributors** who need to know what a name means before
  they use it, extend the pipeline, or write a test.
- **Security reviewers** who need to verify that the enforcement
  matches the specification without reading every source file.
- **Component authors** (Phase 12) who need to know how to
  declare the paths their component may write and how those
  declarations compose with the Root's own boundaries.

It covers the three concepts — Root, sub-root, and deny-list —
the allow-list mechanism for components, the boundary decision
tree, and the security invariant that the model exists to
guarantee.

It does **not** cover:

- The lexical normalization and platform-specific path rules.
  See `path-normalization.md`.
- The `FS` interface's boundary semantics. See
  `internal/filesystem`'s package doc and WBS 13.1.1. The two
  boundaries are conceptually distinct; in production they
  coincide because `safefs.New` constructs the FS with the
  Root's path.
- The walk's depth and entry bounds. See the `Walk` method's
  doc comment.
- The symlink policy's specific algorithms. See
  `internal/filesystem/safefs/symlink.go` and ADR-035.

## The Three Concepts

### Root

The **Root** is the target directory passed to `safefs.New`.
All operations on the Root and on any Handle derived from it
must resolve to a path inside the Root.

The Root's boundary is enforced by the pipeline described in
`path-normalization.md`. The pipeline has six steps; the
boundary check is step 2, and it is performed by
`filepath.IsLocal` on the cleaned name.

The Root also carries:

- A **deny-list** (see below).
- An **allow-list** (optional; see below).
- A **symlink policy** (`NoFollow`, `FollowInRoot`).
- **Walk bounds** (`MaxDepth`, `MaxEntries`,
  `MaxSymlinkChain`).
- A **case-collision policy** (on or off).

Every one of these is set at construction and immutable
thereafter. A Root is a value; a change to any of its
properties means constructing a new one.

### Sub-root

A **sub-root** is a `*safefs.Handle` returned by
`Root.Rel(sub)`. Its boundary is the Root's boundary joined
with `sub`.

A sub-root enforces its own boundary in addition to the Root's.
A name that escapes the sub-root is rejected even if it is
legal under the Root. This is what makes a sub-root a sandbox
rather than a convenience.

Sub-roots compose transitively: a Handle returned by
`handle.Rel(sub2)` enforces the Root's boundary, the outer
handle's boundary, and its own. The composition is the
mechanism by which a component nested inside another
component's sandbox cannot reach the outer component's paths.

#### Why sub-roots exist

A sub-root is the mechanism the boundary model provides for
**component isolation** (Phase 12). A component that renders a
template or materialises a scaffold receives a sub-root scoped
to its declared output paths; the sub-root ensures that a bug
in the component (a template that names `../other/file`, an
expression that computes a path from user input without
sanitising it) cannot write outside the component's declared
scope.

A sub-root is cheaper than a fresh Root:

- It shares the parent's resolved-symlink cache, so a name
  that was resolved under the parent is not re-resolved under
  the handle.
- It validates only its sub-path, not the full tree.
- It composes with the parent's policy and bounds rather than
  re-declaring them.

A sub-root is also stronger than a fresh Root:

- A fresh Root at `src` would enforce only the `src` boundary,
  because its boundary is `src`.
- A sub-root at `src` enforces the `src` boundary *and* the
  parent Root's boundary. A name that escapes `src` but stays
  inside the parent is still rejected.

The second property is what makes a sub-root the right tool
for sandboxing: it does not weaken the parent's enforcement.

### Deny-list

A **deny-list** is a set of path patterns that reject names
*inside* the boundary. The deny-list is a filter applied after
the boundary and sub-boundary checks succeed; a name that
passes both but matches the deny-list is rejected with
`ErrDenyList`.

The deny-list exists because some paths are legal but
undesirable to write. `.git/config` is inside the Root on any
Git repository, so the boundary alone would permit writing to
it. The deny-list refuses.

#### Matching rules

Deny-list matching is by **prefix on segment boundaries**. A
pattern `a/b` matches `a/b`, `a/b/c`, and `a/b/`, but does not
match `a/bee` or `a-b`. The matching rule is the same as
`isUnder` in `internal/filesystem/safefs/pipeline.go`, adapted
for the pattern syntax.

Two multi-depth forms are supported:

- **`**/x`** matches `x` at any depth under the Root. It
  matches `x`, `a/x`, `a/b/x`, and so on.
- **`x/**`** matches every path strictly under `x`. It matches
  `x/`, `x/a`, `x/a/b`, and so on. It does not match `x`
  itself; a pattern that wants to match both uses two entries.

A pattern without a special form matches the name exactly and
every name under it. A pattern with neither form is a literal
path prefix.

The full syntax is documented in
`internal/filesystem/safefs/boundary.go` under the `pattern`
type's doc comment.

#### Why a deny-list rather than a policy

The alternative to a deny-list is a policy: "the renderer never
writes to `.git/`." A policy is enforced by convention.
Deny-lists are enforced by code.

The difference matters in three situations:

1. **A new component that forgets the convention.** The
   convention is remembered by the people who wrote it and by
   the reviewers who knew them. A new contributor who has not
   read the convention writes to `.git/` and no one notices
   until the next `git status` shows a modification. A
   deny-list catches the write on the first attempt.

2. **A component that re-implements the rule.** Each component
   that enforces "do not write to `.git/`" writes its own check,
   and each check is a chance to get the rule wrong. A
   deny-list centralises the rule in one place.

3. **A sub-root constructed at a denied path.** A sub-root at
   `.git/` has a boundary that permits writes to `.git/config`.
   The deny-list closes the gap: a sub-root cannot be
   constructed at a denied path, so the gap does not exist.

## The Deny-list Defaults

Every Root is constructed with a default deny-list. The
defaults are chosen to protect the invariants of the Forge
project layout.

| Pattern | Rationale |
|---|---|
| `.git` | Git's repository metadata. A write to `.git/config`, `.git/hooks/*`, or `.git/HEAD` can execute arbitrary code on the next Git command. The boundary alone permits writes here because `.git/` is inside every Git working tree. |
| `.forge/backups` | Forge's internal backup directory. Writes here could corrupt the update mechanism's rollback state. |
| `.forge/lock` | The update lock file. A write here could race with a concurrent `forge update`. |
| `**/*.swp` | Vim swap files. These are transient and are never intentional outputs. A write here is almost always a mistake. |
| `**/.DS_Store` | macOS directory metadata. Not an intentional output. |

### Why these defaults

The defaults are the minimum set that protects Forge's own
invariants. Each is a path that Forge itself may need to write
(in the case of `.forge/backups/` and `.forge/lock`) but that
*no component's output* should write. The defaults are
constructed by the Root, not by the caller, so a component
cannot accidentally omit them.

### How to change the defaults

A caller that wants a different set uses one of two options:

- **`WithDenyList(patterns...)`** replaces the defaults with the
  given patterns. This is the option for a caller that has
  considered the defaults and decided they are not appropriate.
  The call is visible in review, which is the point: removing
  a default is a decision that should be reviewed.
- **`WithAdditionalDenyPatterns(patterns...)`** extends the
  defaults with the given patterns. This is the option for a
  caller that wants the defaults plus a few more entries. The
  defaults remain in effect.

The default behaviour is **extend**. A caller who wants to
remove a default must say so explicitly, using the option whose
name reads as "I am taking responsibility for the full list."

### Why the defaults are not exhaustive

The deny-list is not a comprehensive list of "paths no one
should write." It is a small list of paths that Forge's own
mechanisms depend on. Every entry is a path where a write would
break a Forge invariant, not merely a path where a write would
be surprising.

Adding more entries would be easy; the temptation to add
`.idea`, `.vscode`, `node_modules`, and a dozen other paths
should be resisted. The boundary and sub-boundary checks handle
the general case: a component that receives a sub-root scoped
to `src` cannot write outside `src` regardless of what else
exists in the project. The deny-list is for paths that are
legal for the Root but undesirable for a specific Forge
mechanism.

## The Allow-list Mechanism

Where the deny-list is a blacklist of forbidden paths, the
allow-list is a whitelist of permitted paths. The two are not
alternatives; a Root has both.

### What the allow-list is for

The allow-list exists for **components** (Phase 12). A
component is a self-contained unit of rendering — a template, a
plugin, an extension — that declares in its manifest which
paths it needs to write. The Root's constructor accepts a
manifest and builds an allow-list from it. A component's write
request is accepted only if it matches the allow-list.

The allow-list is per-Handle, not per-Root. A component
receives a Handle constructed with its manifest; the Handle's
boundary is the intersection of its parent's boundary and the
manifest's paths.

### The composition of allow-list and deny-list

- A name that fails the boundary check is rejected regardless
  of the allow-list.
- A name that fails the sub-boundary check is rejected
  regardless of the allow-list.
- A name that matches the deny-list is rejected with
  `ErrDenyList`, even if it matches the allow-list.
- A name that does not match the allow-list is rejected with
  `ErrNotAllowed`.
- A name that passes all four checks proceeds.

The deny-list always wins over the allow-list. A component
whose manifest permits `.git/` is still rejected by the Root's
deny-list, because the deny-list protects the Root's own
invariants and the component's manifest cannot override them.

### Allow-list syntax

An allow-list entry is a path pattern with the same syntax as a
deny-list entry:

- A literal path segment: `src`.
- A prefix with `**/`: `**/assets`, which matches `assets`,
  `src/assets`, and `src/nested/assets`.
- A suffix with `/**`: `src/**`, which matches `src/`, `src/a`,
  and `src/a/b`, but not `src`.

The syntax is deliberately small. A full glob language would
permit patterns like `*.go` that a component could use to
declare "any Go file in my subtree", which sounds convenient
but makes the allow-list impossible to verify by inspection.
The current syntax is expressive enough for the Phase 12 use
cases and small enough that a reviewer can read an allow-list
and know exactly what it permits.

## The Boundary Decision Tree

Every request — read or write — follows the same decision tree.
The tree is the specification; the pipeline is the
implementation.

```text
                          Request(name)
                                │
                                ▼
                 ┌─────────────────────────────┐
                 │ 1. Lexical normalization    │
                 │    validateName(name)        │
                 └─────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
                rejects                 accepts
                    │                       │
                    ▼                       │
              ErrNotLocal or                │
              ErrPathTraversal              │
                                            ▼
                 ┌─────────────────────────────┐
                 │ 2. Root boundary check      │
                 │    filepath.IsLocal         │
                 └─────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
                rejects                 accepts
                    │                       │
                    ▼                       │
              ErrNotLocal or                │
              ErrPathTraversal              │
                                            ▼
                 ┌─────────────────────────────┐
                 │ 3. Sub-boundary check       │
                 │    (Handle only)             │
                 │    isUnder(resolved, sub)   │
                 └─────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
                rejects                 accepts
                    │                       │
                    ▼                       │
              ErrSubBoundary                │
                                            ▼
                 ┌─────────────────────────────┐
                 │ 4. Deny-list check          │
                 │    deny.matches(clean)      │
                 └─────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
                matches                 no match
                    │                       │
                    ▼                       │
              ErrDenyList                   │
                                            ▼
                 ┌─────────────────────────────┐
                 │ 4b. Allow-list check        │
                 │    allow.matches(clean)     │
                 └─────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
              not allowed               allowed
                    │                       │
                    ▼                       │
              ErrNotAllowed                 │
                                            ▼
                 ┌─────────────────────────────┐
                 │ 5. Symlink resolution       │
                 │    resolveSymlinks(parent)  │
                 └─────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
                rejects                 accepts
                    │                       │
                    ▼                       │
              ErrSymlinkDenied              │
                                            ▼
                 ┌─────────────────────────────┐
                 │ 6. Delegation to FS         │
                 │    r.fs.Open / Read / ...   │
                 └─────────────────────────────┘
                                │
                                ▼
                          Operation result
```

### Why the order is what it is

The order of the six steps is load-bearing. Each ordering
decision is justified by the diagnostic the caller receives
when multiple checks would fail.

**Steps 1 and 2 are pure string work.** They run before any
syscall. A name that fails either never triggers a syscall. The
cost of the pipeline for a rejected name is bounded by the
string operations, which are microseconds.

**Step 3 (sub-boundary) runs before step 4 (deny-list).** A
name that escapes the sub-boundary should be reported as
`ErrSubBoundary`, not as a deny-list error, even if it happens
to match a deny-list pattern. The sub-boundary is the more
specific context: the caller is inside a component sandbox, and
the failure is that the sandbox's boundary was exceeded.

**Step 4 (deny-list) runs before step 4b (allow-list).** A name
that matches the deny-list and fails the allow-list is reported
as `ErrDenyList`. The deny-list is the more specific diagnostic:
the name is not merely outside the component's declared scope,
it is on a list of paths that no component may write. The
"deny-list wins over allow-list" rule is documented in the
allow-list section above.

**Step 4 (deny-list) runs before step 5 (symlink resolution).**
A name that matches the deny-list never touches the filesystem,
even to `Lstat` a parent. This is a small performance win and a
smaller attack-surface win: the deny-list check is a pure
predicate on the cleaned name, and running it before the
syscall means a denied name cannot trigger a syscall whose
side effects (a cache miss, a stat error) the caller might
observe.

**Step 5 (symlink resolution) runs before step 6 (delegation).**
This is the TOCTOU defence: the parent is resolved between the
lexical check and the syscall, narrowing the window during
which a symlink-swap could change the meaning of the name. See
`path-normalization.md` for the full TOCTOU posture.

### What the tree does not include

The tree does not include a step for **case-collision
detection**. The collision check is applied by `ReadDir`, not
by the pipeline, because the collision is a property of the
directory's contents rather than of the name passed in. A
`ReadDir` that enables collision detection inspects its result
and returns `ErrCaseCollision` if two entries collide. The
check is separate from the pipeline because it runs on a
different input.

The tree does not include a step for **file existence**. The
pipeline does not check whether a file exists; the operation
that delegates to the FS is the one that observes
`fs.ErrNotExist`. The pipeline's job is to enforce the boundary
model, not to predict the filesystem's state.

## The Invariant

> **No write outside the Root may ever succeed, regardless of
> code path.**

This is the security invariant that the boundary model exists
to guarantee. It is stated in absolute terms because it is an
absolute property: there is no code path in `safefs` that
bypasses the pipeline, no method that hands a raw name to the
underlying FS, and no configuration that disables the check.

### How the invariant is verified

The invariant is verified by four independent mechanisms. Each
covers a different class of failure; together they cover the
ways the invariant could break.

1. **Code review.** Every method on `Root` and `Handle` routes
   through the pipeline. The pipeline's first step is
   `validateName`, which rejects any name that is not a local
   relative path. A method that bypassed the pipeline would be
   a visible deviation from the pattern; a reviewer reading
   the package's operations sees the same shape repeated.

2. **The layering lint.**
   `scripts/deps/verify-fs-layering.sh` ensures that no code
   outside `internal/filesystem` and
   `internal/filesystem/safefs` calls `os` for filesystem I/O.
   A caller that wants to bypass `safefs` must therefore reach
   for a lower-level package, which is visible in review and
   which the lint will catch if the reach is via a new package.

3. **Property-based tests.** The fuzz targets in
   `internal/filesystem/safefs/fuzz_test.go` assert that no
   accepted name escapes the Root. The adversarial test suite
   in `internal/filesystem/safefs/boundary_test.go` includes
   specific attacks (write to `.git/config`, cross-component
   write, deny-list bypass attempts) that must all fail.

4. **The decision tree.** The tree's six steps are the
   specification; the pipeline's six steps are the
   implementation. A change to the pipeline that added,
   removed, or reordered a step would fail the decision
   tree's sample tests, which assert the sentinel each step
   produces.

### What the invariant does not promise

The invariant is about writes. It does not promise:

- **Process-level isolation.** A process with filesystem access
  outside the Root can still write outside the Root through
  `os` directly. The boundary model assumes the process obeys
  the layering rule. Process isolation is a separate concern
  (see ADR-035, "Boundaries").
- **Immutability of a successfully written file.** The
  invariant is that the write *starts* inside the boundary.
  Once a file exists and is written, its subsequent state is
  the caller's responsibility. A concurrent process with write
  access to the file can change it.
- **Protection against a compromised filesystem.** The
  invariant assumes the filesystem's metadata (what a symlink
  points at, what a directory contains) is honest. A
  malicious filesystem driver can lie. The boundary model is a
  userspace control, not a kernel-level isolation primitive.

## Worked Examples

The examples below walk through the decision tree for a few
representative cases. Each example states the request, the
step at which the tree terminates, and the resulting error (if
any).

### Example 1: A read of `.git/config` through the Root

**Request:** `root.Open(".git/config")`.

**Trace:**

1. `validateName` cleans the name to `.git/config` and
   `filepath.IsLocal` returns true. Accept.
2. The Root boundary check passes (folded into step 1). Accept.
3. There is no sub-boundary (the operation is on the Root, not
   a Handle). Skip.
4. The deny-list check: the pattern `.git` matches
   `.git/config` because `.git/config` starts with `.git/`.
   Reject with `ErrDenyList`.
5. Steps 5 and 6 are not reached.

**Result:** the operation is rejected with `ErrDenyList`. The
filesystem is not touched.

### Example 2: A read of `.gitignore` through the Root

**Request:** `root.Open(".gitignore")`.

**Trace:**

1. `validateName` cleans the name to `.gitignore`. Accept.
2. The Root boundary check passes. Accept.
3. No sub-boundary. Skip.
4. The deny-list check: the pattern `.git` does not match
   `.gitignore`. The pattern matches `.git` exactly or
   `.git/...` as a prefix; `.gitignore` is neither. No match.
   Continue.
4b. No allow-list is configured. Skip.
5. The parent of `.gitignore` is empty (single component), so
   no symlink resolution is needed. Accept.
6. The operation delegates to the FS.

**Result:** the operation proceeds to the filesystem. If the
file exists, the read succeeds; otherwise it fails with
`fs.ErrNotExist`.

### Example 3: A write to `../other.go` through a Handle at `src`

**Request:** `srcHandle.Open("../other.go")` where
`srcHandle = root.Rel("src")`.

**Trace:**

1. `validateName` cleans the name to `../other.go`. The cleaned
   form begins with `..`, so `filepath.IsLocal` returns false.
   `containsTraversal` returns true. Reject with
   `ErrPathTraversal`.
2. Steps 2–6 are not reached.

**Result:** the operation is rejected with `ErrPathTraversal`.
The filesystem is not touched.

### Example 4: A write to `a.go` through a Handle at `src`

**Request:** `srcHandle.Open("a.go")` where
`srcHandle = root.Rel("src")`.

**Trace:**

1. `validateName` cleans the name to `a.go`. Accept.
2. The Root boundary check passes. Accept.
3. The sub-boundary check: `srcHandle`'s sub-path is `src`.
   The full name is `src/a.go`, which starts with `src/`.
   `isUnder("src/a.go", "src")` returns true. Accept.
4. The deny-list check: no default pattern matches `src/a.go`.
   Continue.
4b. No allow-list is configured. Skip.
5. The parent of `src/a.go` is `src`. `resolveSymlinks("src")`
   resolves the parent under the policy. Under `NoFollow`, if
   `src` is not a symlink, it passes through. Under
   `FollowInRoot`, if `src` is a symlink, it is resolved and
   checked against the Root's boundary.
6. The operation delegates to the FS with the resolved name.

**Result:** the operation proceeds to the filesystem. If `src`
is a symlink that escapes the Root, step 5 rejects with
`ErrSymlinkDenied`.

### Example 5: A sub-root constructed at `.git`

**Request:** `root.Rel(".git")`.

**Trace:**

1. `validateName` cleans `.git` and accepts it.
2. The Root boundary check passes.
3. No sub-boundary (this is a construction, not an operation).
   Skip.
4. The deny-list check runs against the sub-path `.git`. The
   pattern `.git` matches exactly. Reject with `ErrDenyList`.

**Result:** the construction is rejected with `ErrDenyList`.
The sub-root is not created. The subtree at `.git/` remains
inaccessible from any Handle constructed via `Rel`.

## What the Model Does Not Cover

### Process-level isolation

The boundary model assumes the process obeys the layering
rule. A caller that imports `os` directly and calls
`os.WriteFile` bypasses the model. The model does not prevent
that; the layering lint does, for the packages under its
scope.

Process-level isolation — a sandbox that a rogue component
cannot escape — is a different problem, solved by OS
primitives (seccomp, AppArmor, containers). The boundary model
is complementary to those; it makes the common case correct
without requiring process-level setup.

### Recursive deny-list checks

The deny-list is checked on the name the caller passes, not
on the entries a walk visits. A `Walk` of the Root visits
`.git/` if the Root's deny-list permits it (which it does not,
by default). A future version of the walk could apply the
deny-list to each visited entry; the current version does not,
because the pipeline's contract is "the name the caller
passed," and applying it per-entry would be a different
contract.

A caller that wants to skip denied entries during a walk
checks the deny-list itself in the walk callback. The deny-list
matcher is not exported; a caller that needs it either uses the
default behaviour (which visits the denied directory) or
constructs a Root whose deny-list is empty (which is a
deliberate decision).

### Deny-list enforcement on the FS layer

The `FS` interface has no deny-list. A caller that constructs
an `OSFS` directly and calls `Write(".git/config", ...)` is
not subject to the deny-list. The deny-list is a property of
the Root, not of the FS. The layering rule forbids domain
packages from reaching the FS directly; the deny-list is the
Root's contribution to the enforcement.

### Case-insensitive filesystems

On a case-insensitive filesystem (Windows, macOS by default),
the deny-list matches by the same string rules. A pattern
`.git` matches `.GIT/config` on Windows because the FS layer
resolves the case-insensitive lookup; the pattern match itself
is case-sensitive.

The asymmetry is deliberate: `safefs` does not normalise case,
because case normalisation is a filesystem property and the
correct normalisation is not portable (Unicode case folding
differs between platforms). A caller who needs case-insensitive
deny-list matching adds both cases to the list, or uses a
filesystem that is case-sensitive.

## Change History

| Date | Change | Author |
|------|--------|--------|
| 2025-01-15 | Initial version; ratifies WBS 13.4.1 | Security Owner |

## See Also

- [ADR-035 — Filesystem Enforcement by Construction](../decisions/ADR-035-filesystem-enforcement-by-construction.md)
- [Path Normalization & Canonicalization](./path-normalization.md)
- [Filesystem API Reference](./filesystem-api.md)
- [Filesystem Layering](../architecture/filesystem-layering.md)
- [internal/filesystem/safefs package doc](../../internal/filesystem/safefs/doc.go)
- [internal/filesystem package doc](../../internal/filesystem/filesystem.go)
- [internal/filesystem/safefs/boundary.go](../../internal/filesystem/safefs/boundary.go) — the pattern syntax
- [internal/filesystem/safefs/pipeline.go](../../internal/filesystem/safefs/pipeline.go) — the implementation of the decision tree
