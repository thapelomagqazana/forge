---
title: Filesystem Layering
status: Accepted
owner: Architecture + Security Owners
wbs: 13.1.1
adr: ADR-035
traces:
  - FR-FS-001
  - SEC-FS-001
  - CWE-22
  - CWE-59
  - CWE-61
last_reviewed: 2025-01-15
review_cadence: per-phase
---

# Filesystem Layering

> **TL;DR** — Forge has exactly **two** filesystem packages.
> `internal/filesystem` is portable, policy-free primitives.
> `internal/filesystem/safefs` is the policy-enforcing boundary.
> Every other package that touches the disk imports `safefs` — and
> nothing else. This is enforced by CI, not by reviewer diligence.

---

## 1. Why this document exists

A foundation manager reads and writes files across three operating
systems, on user-supplied project trees, often with content fetched
from untrusted sources (blueprints, templates, remote registries).
Historically this class of code has been written with direct
`os.Open`, `filepath.Join`, and ad-hoc `strings.HasPrefix` checks.
That pattern is:

1. **Non-portable.** Windows path semantics, drive letters, UNC
   prefixes, reserved names, and case-insensitivity are easy to get
   wrong — and the failures are silent.
2. **Insecure by default.** Every call site must remember to check
   for symlinks, `..` traversal, and TOCTOU races. A single missed
   check re-opens CWE-22, CWE-59, or CWE-61 across the whole
   product.
3. **Untestable.** `os` is a package-level global. Tests must either
   hit the real filesystem (slow, flaky) or monkey-patch (fragile).
4. **Un-auditable.** There is no single place to enumerate the
   filesystem policy the product actually enforces.

This document records the architecture that fixes all four. It is
the normative reference for "where does my filesystem code go?"

This document is the *how-to-apply-it* companion.

---

## 2. The two layers at a glance

```text
┌──────────────────────────────────────────────────────────────────┐
│  Domain packages                                                 │
│  blueprint · template · renderer · update · check · drift        │
│                                                                  │
│  May import: safefs, forgeerr, logger, output                    │
│  Must NOT import: internal/filesystem, os (for I/O)              │
└───────────────────────────────┬──────────────────────────────────┘
                                │
                                │  only legal import
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│  internal/filesystem/safefs                                      │
│                                                                  │
│  Owns: symlink policy · traversal · root confinement ·           │
│        bounded walks · case-collision detection                  │
│                                                                  │
│  Exposes: *safefs.Root handle + typed errors                     │
│  Must NOT import: domain packages, internal/filesystem           │
│  directly for policy — only for the FS interface                 │
└───────────────────────────────┬──────────────────────────────────┘
                                │
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│  internal/filesystem                                             │
│                                                                  │
│  Owns: FS interface · OSFS implementation · platform quirks      │
│  Contains: NO security policy, NO root notion, NO symlink rules  │
│  Must NOT import: safefs, domain packages                        │
└───────────────────────────────┬──────────────────────────────────┘
                                │
                                ▼
                        os / io/fs (stdlib)
```

**Import direction is one-way.** Reverse imports are a build failure
(see §7).

---

## 3. Decision table — which package does my code belong in?

| If your code…                                                                 | Put it in                        |
|-------------------------------------------------------------------------------|----------------------------------|
| Defines a portable `FS`/`File` interface, or an `os`-backed implementation    | `internal/filesystem`            |
| Knows about `*os.File`, `os.Open`, `os.ReadFile`, `os.ReadDir`                 | `internal/filesystem` (only)     |
| Enforces a security policy: symlinks, traversal, root confinement, bounds     | `internal/filesystem/safefs`     |
| Resolves a blueprint, renders a template, walks a project tree, diffs a project | domain package — import `safefs` only |
| Is a CLI command that needs to read a file                                     | `internal/cli` — inject a `*safefs.Root` |
| Is a test that needs a fake filesystem                                         | any package — inject `fstest.MapFS` via `safefs.WithFS` |
| Reads a config file at startup                                                  | `internal/config/loader.go`, routed through `safefs` |

### The one-question test

> **"Am I about to write `os.` or `strings.HasPrefix(path, root)`?"**

- If **yes** → you are in the wrong package. Move the code.
- If you believe you are the exception → open an ADR. There are
  currently only two legitimate exceptions: `internal/filesystem`
  itself, and `cmd/forge` (for `os.Args` / `os.Exit`).

### Rule of thumb

Policy is a **type-level** property, not a **review-level** one.
If a reviewer has to remember to check something, the architecture
has failed. The right question is not "did you check for `..`?" but
"could you have named the file without a `Root`?" If the answer is
yes, the code is wrong regardless of what it does.

---

## 4. Layer 1 — `internal/filesystem`

**Portable, unopinionated primitives. Zero security policy.**

### Responsibility

- Define the `FS` interface: the minimal surface Forge depends on.
- Provide `OSFS`, the single production implementation.
- Abstract platform differences (separators, case sensitivity,
  reserved names, drive letters) behind the interface.

### Non-responsibility

- It does **not** know what a "project root" is.
- It does **not** follow or reject symlinks based on policy.
- It does **not** reject `..` or absolute paths.
- It does **not** bound walks.

A file handle returned by this layer is a *raw* handle. There is
nothing safe about it. That is deliberate: policy belongs one level
up.

### The interface

```go
package filesystem

import "io/fs"

// FS is the minimal filesystem interface Forge depends on.
//
// It is a strict subset of io/fs.FS plus the write operations Forge
// needs. Implementations MUST NOT enforce security policy — that is
// the responsibility of safefs.
//
// Contract:
//   - Name arguments are forward-slash, cleaned, relative to the
//     FS's notional root. Implementations MUST reject absolute
//     paths with an error.
//   - Errors are returned unwrapped. Callers wrap with forgeerr.
//   - No method applies symlink policy. Policy lives in safefs.
type FS interface {
    fs.FS
    fs.StatFS
    fs.ReadDirFS
}
```

### The single `os` holder

```go
// OSFS is the only type in package filesystem — and, transitively,
// in the entire module except cmd/forge — that holds an *os.File.
//
// Do not add symlink, traversal, or root-confinement logic here.
// Those belong in safefs. See ADR-035.
type OSFS struct {
    root string
}
```

### Testing

`internal/filesystem` is the **only** package in the module whose
tests touch the real disk. They use `t.TempDir()` and are small. Any
other package that needs a fake filesystem uses `fstest.MapFS` and
`internal/filesystem/safefs`.

---

## 5. Layer 2 — `internal/filesystem/safefs`

**Policy-enforcing boundary. The only way to name a file.**

### Responsibility

`safefs` owns **all** filesystem security policy:

- **Symlink resolution and policy.**
  `NoFollow` (default) · `FollowInRoot` · `FollowAnywhereForbidden`.
- **Traversal prevention.** `..` segments rejected before resolution.
- **Root confinement.** A resolved absolute path must satisfy
  `filepath.IsLocal(filepath.Rel(root, resolved))` — compared
  per-segment, never by string prefix (defeating the `/root` vs
  `/root-evil` class of bugs).
- **Bounded walks.** `MaxDepth`, `MaxEntries`. These are a security
  control, not a performance optimisation: a hostile deep tree must
  not OOM the process.
- **Case-collision detection.** Optional, enabled on case-insensitive
  filesystems (macOS, Windows).

### Non-responsibility

- It does **not** parse blueprints, render templates, or know what a
  "project" is.
- It does **not** import any domain package.
- It does **not** import `os`. Ever. (Enforced by CI — §7.)

### The `Root` handle

A `*safefs.Root` is the **only** way a domain package may name a
file. It is constructed once, validated once, and immutable
thereafter. Every method takes a **relative, forward-slash, cleaned**
path and performs the confinement check before any I/O.

```go
package safefs

// Root is a policy-enforcing handle to a directory tree.
//
// Constructed once via New. Immutable thereafter. Safe for
// concurrent use.
type Root struct { /* ... */ }

// New constructs a Root for targetDir, converting it to an absolute,
// cleaned path and validating it once.
func New(targetDir string, opts ...Option) (*Root, error)

// Open opens name for reading. name must be a local, forward-slash
// relative path.
func (r *Root) Open(name string) (fs.File, error)

// Stat returns FileInfo for name.
func (r *Root) Stat(name string) (fs.FileInfo, error)

// ReadFile reads the whole file at name.
func (r *Root) ReadFile(name string) ([]byte, error)

// ReadDir lists the entries of the directory at name, sorted.
func (r *Root) ReadDir(name string) ([]fs.DirEntry, error)

// Walk recursively visits the tree rooted at name, subject to the
// Root's MaxDepth and MaxEntries bounds.
func (r *Root) Walk(name string, fn WalkFunc) error
```

### Canonical usage

```go
root, err := safefs.New("/path/to/project",
    safefs.WithSymlinkPolicy(safefs.NoFollow),
    safefs.WithMaxDepth(64),
    safefs.WithMaxEntries(1 << 20),
)
if err != nil {
    return fmt.Errorf("construct safefs root: %w", err)
}

f, err := root.Open("blueprint/go-api.yaml")
if err != nil {
    return err
}
defer f.Close()

data, err := io.ReadAll(f)
if err != nil {
    return err
}
```

The domain package never sees an `*os.File`. It never sees an
absolute path. It cannot construct a path that escapes the root,
because the only constructor — `validateName` — rejects them.

### Why a handle, not a validator

It is tempting to expose `ValidatePath(name)` and call it before
each `os.Open`. **Do not.** That pattern is vulnerable to TOCTOU
races: the path can change between validation and open. The policy
must be applied *by the operation that performs the I/O*. This is
why `Root.Open` — not `Root.Validate` — is the enforcement point.

### The name contract

Every `Root` method accepts a name that satisfies:

1. **Non-empty.**
2. **Local** — `filepath.IsLocal` returns true. This is the single
   source of truth. It correctly rejects absolute paths, Windows
   drive letters (`C:\`), UNC prefixes (`\\server\share`), `..`
   segments, and reserved names (`CON`, `NUL`, `AUX`, …).
3. **Forward-slash.** Backslashes are normalised to forward slashes
   on entry, so the same call site works on Windows.
4. **Cleaned** — `path.Clean`, which is platform-independent.

`validateName` is pure string work: it does not touch the
filesystem, so it is cheap and cannot itself race.

---

## 6. The "Enforcement by Construction" model

The architectural claim of this document is stronger than "we
validate paths." It is:

> **A domain package cannot perform an unchecked filesystem
> operation, because it has no legal way to name one.**

This is achieved by four properties:

1. **`safefs.Root` is the only constructor of file handles.** There
   is no exported function in `safefs` that returns an `*os.File`
   or takes an absolute path.
2. **`safefs.Root` never returns an `*os.File`.** It returns
   `io/fs.File`, whose `Stat` has been filtered through the symlink
   policy.
3. **The only constructor is `safefs.New`,** which validates the
   root once. Methods rely on that invariant and never re-derive it
   from an untrusted path.
4. **The CI lint forbids the alternative.** A domain package that
   tries to import `os` or `internal/filesystem` fails the build.

The result: security is a property of the *type system*, not of
*reviewer diligence*. A new contributor who has never read this
document cannot accidentally introduce CWE-22, because the compiler
and CI prevent the shape of code that would contain it.

### Worked example — adding a new domain operation

Suppose `internal/check` needs to read every `*.yaml` under
`blueprints/` in a project.

**Wrong:**

```go
// internal/check/check.go
package check

import (
    "os"
    "path/filepath"
    "strings"
)

func FindBlueprints(projectDir string) ([]string, error) {
    var out []string
    err := filepath.Walk(projectDir, func(p string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if strings.HasPrefix(p, filepath.Join(projectDir, "blueprints")) &&
            strings.HasSuffix(p, ".yaml") {
            out = append(out, p)
        }
        return nil
    })
    return out, err
}
```

Problems: imports `os`; relies on `strings.HasPrefix` (the
`/root` vs `/root-evil` bug); follows symlinks silently; unbounded;
no error taxonomy. This file will not compile in a layering-correct
module — CI rejects the `os` import.

**Right:**

```go
// internal/check/check.go
package check

import (
    "io/fs"
    "strings"

    "github.com/forge/internal/filesystem/safefs"
)

func FindBlueprints(root *safefs.Root) ([]string, error) {
    var out []string
    err := root.Walk("blueprints", func(rel string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }
        if strings.HasSuffix(d.Name(), ".yaml") {
            out = append(out, rel)
        }
        return nil
    })
    return out, err
}
```

The domain package no longer names a path or imports `os`. Depth,
entry count, symlink policy, and traversal are all enforced by the
`Root` it was handed. The function signature documents the
dependency: *"I need a filesystem, and I get it as a handle."*

---

## 7. Enforcement — the CI lint

The architecture is only real if it is checked. The rule lives in
`scripts/deps/verify-fs-layering.sh` and runs in CI via
`task verify:fs-layering`.

### Rules

1. **`os` import confinement.** Only the following packages may
   import `"os"`:
   - `internal/filesystem`
   - `internal/filesystem/safefs`
   - `internal/version` (reads `runtime/debug`, not the filesystem)
   - `cmd/forge` (process entrypoint: `os.Args`, `os.Exit`)

2. **Domain packages import `safefs`, never `internal/filesystem`.**

### Why the rule is coarse

The script does **not** try to detect "`os` for filesystem I/O" —
that is undecidable with static analysis. It forbids `"os"`
everywhere except an explicit allow-list. This is coarse but
**correct and auditable**: a reviewer can read the allow-list in ten
seconds and challenge additions.

### How to add an exception

Adding a package to the allow-list requires:

1. An ADR explaining why the package cannot route through `safefs`.
2. Sign-off from the Architecture and Security owners.
3. An update to the allow-list in `verify-fs-layering.sh` with a
   comment naming the ADR.

The current allow-list has two entries, both structural
(`cmd/forge` and `internal/version`). There are no domain
exceptions.

### Running locally

```sh
task verify:fs-layering
```

Output on success:

```text
OK: filesystem layering verified.
```

Output on failure names the offending package and links here.

---

## 8. Testing strategy

| Layer | Test style | Seam |
|-------|-----------|------|
| `internal/filesystem` | Real disk via `t.TempDir()` | none — it *is* the os layer |
| `internal/filesystem/safefs` | `fstest.MapFS` injected via `safefs.WithFS` | `filesystem.FS` |
| Domain packages | `fstest.MapFS` injected through `safefs.WithFS` | `*safefs.Root` |
| CLI | End-to-end via `binary_integration_test.go` | compiled binary |

### The test seam

`safefs` depends on `filesystem.FS`, not `os`. This single decision
means:

- The entire `safefs` suite is pure, fast, and deterministic.
- Symlink behaviour is the one exception: it requires a real
  filesystem, so it is isolated in `symlink_unix_test.go` and
  `symlink_windows_test.go` behind build tags.
- Domain packages never need `t.TempDir()` for filesystem logic —
  they inject a `MapFS`-backed `Root`.

Example:

```go
mem := fstest.MapFS{
    "blueprint/go-api.yaml": &fstest.MapFile{Data: []byte("kind: blueprint")},
}
root, err := safefs.New("/root", safefs.WithFS(mem))
if err != nil {
    t.Fatal(err)
}
f, err := root.Open("blueprint/go-api.yaml")
// ...
```

### Test categories required for `safefs`

Every new policy must ship with all of:

- **Positive** — the operation succeeds on a valid name.
- **Negative** — traversal, absolute paths, empty names, symlink
  escapes, depth/entry overruns.
- **Edge** — backslash names, `.`, repeated slashes, trailing
  slashes.
- **Corner** — symlink policy matrix on Unix *and* Windows.
- **Boundary** — `MaxDepth` exactly at the limit; `MaxEntries`
  exactly at the limit; zero means unbounded.
- **Non-functional** — concurrent use; fuzz on `validateName`.

The fixture test `verify-fs-layering_test.sh` is itself part of the
test suite: it proves the lint rejects a violating package.

---

## 9. Frequently asked questions

### "I need to read a file that is not under the project root. What do I do?"

Open an ADR. There is currently no supported way to read outside
the root. If you need to read, for example, `$HOME/.config/forge/…`,
that is a separate `Root` constructed at that directory — never a
path escape from the project root.

### "I need to write a file. `safefs` has no `WriteFile`."

Correct — write support is WBS 13.3. It will be added as
`Root.WriteFile` / `Root.Create` with the same name contract. Do not
implement it in a domain package.

### "My package needs `os.Getenv`, not filesystem I/O."

`os.Getenv` is not filesystem I/O, but the CI rule forbids the `os`
import wholesale. Use `internal/config` (which owns environment
lookup) or request an allow-list entry with an ADR. The coarse rule
is intentional; the friction is the point.

### "The symlink policy I need isn't listed."

Open an ADR proposing the new policy. The set is **closed** by
design — see ADR-034 (redaction policy is closed) for the analogous
argument.

### "Can I construct a `Root` inside a domain package?"

Yes, if the domain package *owns* the directory being rooted (for
example, a cache directory). Construction is cheap and validates
once. What you may not do is construct a `Root` from an untrusted
path and then pass an absolute path to `Open`.

### "What about `io/fs.Glob`, `io/fs.Sub`, and friends?"

`safefs` exposes the operations Forge actually needs. If you need a
new one, add it to `Root` with the same name contract — do not
call `io/fs` directly from a domain package.

---

## 10. Migration status

Packages that currently import `internal/filesystem` or `os`
directly and must be migrated to `safefs`:

| Package | Status | Owner | Tracking |
|---------|--------|-------|----------|
| `internal/blueprint` | pending | TBD | WBS 13.2 |
| `internal/template` | pending | TBD | WBS 13.2 |
| `internal/renderer` | pending | TBD | WBS 13.2 |
| `internal/update` | pending | TBD | WBS 13.2 |
| `internal/check` | pending | TBD | WBS 13.2 |
| `internal/drift` | pending | TBD | WBS 13.2 |
| `internal/config/loader.go` | pending | TBD | WBS 13.2 |
| `internal/config/discover.go` | pending | TBD | WBS 13.2 |

Each row is a follow-up WBS item. Until migration lands, the
corresponding package is listed in the **temporary** allow-list at
the top of `verify-fs-layering.sh` with a `// TODO(WBS 13.2)`
marker. The allow-list must trend to empty.

---

## 11. References

### Forge documents

- [ADR-011 — Closed Error Category Taxonomy](../decisions/ADR-011-closed-error-category-taxonomy.md)
- [ADR-012 — Error Code Registry and Stability Policy](../decisions/ADR-012-error-code-registry-and-stability-policy.md)
- [ADR-013 — Error Wrapping Contract](../decisions/ADR-013-error-wrapping-contract.md)
- [ADR-016 — Error Redaction Policy](../decisions/ADR-016-error-redaction-policy.md)
- [ADR-022 — Output and Logging Are Separate Architectural Layers](../decisions/ADR-022-output-and-logging-are-separate-architectural-layers.md)
- [ADR-028 — The Stream Contract](../decisions/ADR-028-the-stream-contract-is-enforced-at-the-boundary-output-stdout-logger-stderr-errors-stderr.md)
- [ADR-034 — The Redaction Policy Is Closed](../decisions/ADR-034-the-redaction-policy-is-closed-no-log-field-is-exempt-and-every-mechanism-has-a-test.md)
- [security-model.md](../security-model.md)
- [security/logging-redaction.md](../security/logging-redaction.md)
- [dependency-policy.md](../dependency-policy.md)
- [requirements-register.md](../requirements-register.md)

### External

- CWE-22: Improper Limitation of a Pathname to a Restricted Directory
- CWE-59: Improper Link Resolution Before File Access
- CWE-61: UNIX Symbolic Link (Symlink) Following
- OWASP: Path Traversal
- Go stdlib: [`io/fs`](https://pkg.go.dev/io/fs),
  [`os`](https://pkg.go.dev/os),
  [`path/filepath`](https://pkg.go.dev/path/filepath),
  [`filepath.IsLocal`](https://pkg.go.dev/path/filepath#IsLocal)

---

## 12. Change log

| Date | Change | Author |
|------|--------|--------|
| 2025-01-15 | Initial version; ratifies WBS 13.1.1 and ADR-035 | Architecture + Security Owners |
