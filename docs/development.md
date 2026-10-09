# Development Guide

This document describes how to set up, build, test, and contribute to
Forge. Every contributor should be able to move from a fresh clone to a
passing test run in under ten minutes.

---

## Table of Contents

- [Supported Development Environment](#supported-development-environment)
- [Supported Go Versions](#supported-go-versions)
- [Reproducible Builds](#reproducible-builds)
- [Dependency Integrity](#dependency-integrity)
- [Reproducible Dependency Resolution](#reproducible-dependency-resolution)
- [Module Resolution Policy](#module-resolution-policy)
- [Detecting Dependency Drift](#detecting-dependency-drift)
- [Getting Started](#getting-started)
- [Task Runner](#task-runner)
- [Testing](#testing)
- [Formatting](#formatting)
- [Static Analysis](#static-analysis)
- [Markdown Linting](#markdown-linting)
- [Editor Configuration](#editor-configuration)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)

---

## Supported Development Environment

Forge is developed and tested on the following environments. Other
environments may work but are not officially supported until validated
in CI.

| Component     | Minimum              | Recommended             | Notes                                              |
|---------------|----------------------|-------------------------|----------------------------------------------------|
| Operating system | —                 | —                       | Linux, macOS, and Windows are all supported.       |
| Go toolchain  | 1.23.0               | 1.23.4                  | See [Supported Go Versions](#supported-go-versions). |
| Shell         | POSIX-compatible     | `bash` 5+               | Windows contributors should use Git Bash or WSL.   |
| Task runner   | [Task](https://taskfile.dev/) 3.x | 3.x        | All workflow commands are defined in `Taskfile.yml`. |
| Git           | 2.30+                | latest                  | Required for reproducible builds.                  |
| Node.js       | 18+                  | LTS                     | Required only for `markdownlint-cli2`.             |
| Editor        | —                    | VS Code, GoLand, Neovim | See [Editor Configuration](#editor-configuration). |

### Windows notes

- **Recommended:** use [Git Bash](https://gitforwindows.org/) or
  [WSL 2](https://learn.microsoft.com/windows/wsl/). Both provide a
  POSIX-compatible shell.
- **Command Prompt and PowerShell** are not supported for the shell
  tasks in `Taskfile.yml`. They may work for direct `go` commands.
- **Line endings:** `.editorconfig` enforces LF endings for source
  files, so Windows contributors do not produce spurious CRLF diffs.

---

## Supported Go Versions

Forge maintains a **three-tier** Go version policy. Each tier has a
different purpose and a different audience.

| Role                    | Version | EOL                  | Rationale                                                                                                                            |
|-------------------------|---------|----------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| **Minimum supported**   | Go 1.23.0 | TBD (follows upstream) | The lowest Go version Forge guarantees to build and run on. Every language feature Forge relies on is available at this version.   |
| **Current development** | Go 1.23.4 | TBD                  | The patch version contributors should use locally. Receives security updates within the minimum-supported line.                     |
| **Latest supported**    | Go 1.24.x | TBD                  | The current stable line. Tested in CI to detect forward-compatibility regressions early.                                            |
| **Recommended toolchain** | Go 1.23.4 | TBD                | Pinned via the `toolchain` directive in `go.mod`. Go's automatic toolchain selection uses this version when available.              |

### Why three tiers

- **Minimum supported** is a *contract*: everything Forge claims to
  support must build on this version. It is the lowest common
  denominator.
- **Current development** is a *convenience*: it is what contributors
  should use locally to benefit from the latest patch fixes.
- **Latest supported** is a *preview*: it lets CI detect language or
  standard-library changes that break Forge before the change reaches
  end users.

### Why the minimum is not the latest

Making the latest release the minimum would exclude contributors on
older, still-supported toolchains without providing Forge any benefit.
Forge does not use features introduced after Go 1.23.

### Why the minimum is not `latest`

The `go` directive does not accept `latest`. Additionally, a moving
target destroys reproducibility: two contributors who run `go build`
six months apart would produce different behaviour without any source
change.

### Policy: N-1 rule

The **minimum supported version must be at least N-1** where N is the
current stable Go release. When a new Go version is released:

1. The **latest supported** version is bumped to N.
2. The **minimum supported** version remains unchanged until the
   previous minimum is one release behind N-1.
3. The bump of the minimum is recorded in an ADR that supersedes
   ADR-004.

This rule prevents the minimum from stagnating while avoiding churn.

### How to check which Go version you have

```bash
go version
```

Expected output when using the recommended toolchain:

```text
go version go1.23.4 <os>/<arch>
```

### How to build with the minimum supported version

```bash
GOTOOLCHAIN=go1.23.0 go build ./...
```

### How to build with the latest supported version

```bash
GOTOOLCHAIN=go1.24.x go build ./...
```

Replace `x` with the current patch of Go 1.24.

### How Go's automatic toolchain selection works

When a contributor with Go 1.22 runs `go build` in a module declaring
`go 1.23.0`, Go will:

1. Detect that the module requires a newer language version.
2. Read the `toolchain` directive in `go.mod`.
3. Download and use `go1.23.4` automatically, provided `GOTOOLCHAIN`
   is set to `auto` or `path` (the default).
4. Produce a clear error if the toolchain cannot be downloaded.

This is why contributors do not need to manually install Go 1.23. Go
handles it. The `go` and `toolchain` directives are the only thing
that matters.

---

## Reproducible Builds

Reproducibility is a core property of Forge. Two contributors running
`task build` on the same commit must produce binaries that differ only
in build metadata.

This section defines the environment variables, build flags, and
workflows that make that guarantee possible.

### The reproducibility invariant

> Given the same commit of the Forge repository, two independent clean
> environments — regardless of operating system, Go toolchain patch, or
> working directory — produce **identical module resolution**.

"Identical module resolution" means:

- `go.mod` is byte-identical after any build or test.
- `go.sum` is byte-identical after any build or test.
- `go list -m all` produces the same sorted list of
  `(module, version)` pairs.

The invariant is verified in CI by the `verify` job in
`.github/workflows/ci.yml`. Any pull request that breaks it fails
to merge.

### Environment variables

#### Required

These must be set on every developer machine and every CI runner.

| Variable      | Value           | Purpose                                       |
|---------------|-----------------|-----------------------------------------------|
| `GOFLAGS`     | `-mod=readonly` | Prevents silent `go.mod` mutation.            |
| `GOTOOLCHAIN` | `auto`          | Allows Go to auto-select the pinned toolchain. |

#### Optional

Set only when the default behaviour is not suitable for the
environment.

| Variable     | Default                           | Purpose                                              |
|--------------|-----------------------------------|------------------------------------------------------|
| `GOPROXY`    | `https://proxy.golang.org,direct` | Module download source. Override for private mirrors. |
| `GOSUMDB`    | `sum.golang.org`                  | Checksum database. Override only when air-gapped.     |
| `GOPRIVATE`  | (empty)                           | Module path prefixes that bypass `GOPROXY`/`GOSUMDB`. |
| `GONOSUMDB`  | (empty)                           | Alternative to `GOPRIVATE` for checksum bypass.       |
| `GOCACHE`    | OS default                        | Build cache. Override only for isolated CI runners.   |
| `GOMODCACHE` | `$GOPATH/pkg/mod`                 | Module cache. Override only for isolated CI runners.  |

#### Prohibited

Never set these. Their presence silently changes behaviour and breaks
the reproducibility invariant.

| Variable              | Why prohibited                                              |
|-----------------------|-------------------------------------------------------------|
| `GO111MODULE`         | Deprecated; Go 1.16+ always uses modules.                   |
| `GOFLAGS=-mod=mod`    | Allows silent `go.mod` mutation.                            |
| `GOFLAGS=-mod=vendor` | Requires vendoring, which is not adopted.                   |
| `GONOSUMCHECK`        | Deprecated; replaced by `GONOSUMDB`.                        |
| `GOINSECURE`          | Bypasses TLS for module downloads. Never acceptable.        |

The full rationale for each decision is in
[`docs/dependency-policy.md`](dependency-policy.md), section
"Module resolution policy".

### How to set the environment

#### On Linux and macOS

Add to your shell profile (`~/.bashrc`, `~/.zshrc`, or equivalent):

```bash
export GOFLAGS=-mod=readonly
export GOTOOLCHAIN=auto
```

Reload your shell:

```bash
source ~/.bashrc   # or ~/.zshrc
```

Verify:

```bash
go env GOFLAGS GOTOOLCHAIN
```

Expected output:

```text
-mod=readonly
auto
```

#### On Windows (PowerShell)

Add to your PowerShell profile (`$PROFILE`):

```powershell
$env:GOFLAGS = "-mod=readonly"
$env:GOTOOLCHAIN = "auto"
```

Reload:

```powershell
. $PROFILE
```

Verify:

```powershell
go env GOFLAGS GOTOOLCHAIN
```

#### CI runners

CI runners set these explicitly. The workflow file
`.github/workflows/ci.yml` does so at the workflow level, so every
job inherits the same environment:

```yaml
env:
  GOFLAGS: -mod=readonly
  GOTOOLCHAIN: auto
  GOPROXY: https://proxy.golang.org,direct
  GOSUMDB: sum.golang.org
```

### Why `-mod=readonly` is the CI default

`-mod=readonly` instructs the Go toolchain to **fail** rather than
**mutate** when `go.mod` or `go.sum` would change.

Without it:

1. A CI build that sees a stale `go.sum` silently updates it in the
   ephemeral workspace.
2. The build succeeds.
3. The contributor's pull request appears correct, but the committed
   `go.sum` is wrong.
4. The error surfaces only after merge, when another contributor pulls
   the branch.

With `-mod=readonly`:

1. The CI build fails immediately.
2. The error message points at the stale file.
3. The contributor runs `task tidy` locally and commits the updated
   `go.sum`.
4. The build succeeds with a correct lockfile.

The policy shifts an invisible runtime failure into a visible
build-time failure. That is the entire point.

### How `go mod tidy` should be run

**Only maintainers run `go mod tidy`**, and only on a clean branch.

The workflow:

1. Create a branch from `main`:

    ```bash
    git checkout main
    git pull
    git checkout -b chore/tidy-dependencies
    ```

2. Verify the working tree is clean:

    ```bash
    git status
    ```

    Expected: `nothing to commit, working tree clean`.

3. Run tidy:

    ```bash
    task tidy
    ```

4. Inspect the diff:

    ```bash
    git diff go.mod go.sum
    ```

5. If the diff is empty, no work is needed. Delete the branch:

    ```bash
    git checkout main
    git branch -D chore/tidy-dependencies
    ```

6. If the diff is non-empty, review each change:

    - Additions of `require` lines: verify they are intentional.
    - Removals: verify the module is truly unused.
    - Version bumps: verify they correspond to `go get` operations.

7. Commit and open a pull request:

    ```bash
    git add go.mod go.sum
    git commit -m "chore(deps): tidy module dependencies"
    git push -u origin chore/tidy-dependencies
    ```

8. Note in the pull request description why tidy was run.

### Why only maintainers run tidy

`go mod tidy` is not idempotent in the presence of uncommitted
changes. If a contributor runs tidy with a partially edited source
tree, tidy may add or remove dependencies based on incomplete
information.

The policy: **tidy is a deliberate operation, not a build step**.

### Verifying reproducibility locally

The reproducibility invariant is verified by three separate checks,
each with a distinct scope:

```bash
task verify:reproducible     # environment-level reproducibility
task verify:resolution       # cross-environment reproducibility
task verify:deps:integrity   # lockfile integrity
```

Each check is described in detail in its own section below.

### Cross-platform notes

The reproducibility invariant holds across Linux, macOS, and Windows.
Two nuances:

- **Line endings.** `.gitattributes` (added in WBS 3.1) enforces LF
  endings for `go.mod` and `go.sum`, so Windows checkouts do not
  introduce CRLF noise.
- **Symlinks.** `go build` follows symlinks. If your `GOPATH` or
  workspace contains symlinks, the invariant still holds, but the
  resolved paths may differ in `go env`. The invariant concerns
  `go.mod` and `go.sum` content, not resolved paths.

---

## Dependency Integrity

Integrity is a local property: is the committed `go.sum` consistent
with the committed `go.mod`, in *this* environment?

This section explains what `go.sum` is, why it is committed, why
manual edits are forbidden, and how to regenerate it legitimately.

### What `go.sum` is

`go.sum` is a lockfile. It records the cryptographic checksum of every
module in the build, including transitive dependencies. When the Go
toolchain resolves a dependency, it compares the checksum in `go.sum`
against the checksum of the downloaded module. If they differ, the
build fails.

`go.sum` is generated by the Go toolchain. It is not authored by hand.
The toolchain writes it whenever a command that resolves modules runs:
`go get`, `go mod tidy`, `go build`, `go test`, `go mod download`.

### Why `go.sum` is committed

Committing `go.sum` provides three guarantees:

1. **Integrity.** Every contributor downloads the same module
   versions. A compromised module cannot silently replace an
   approved one.
2. **Reproducibility.** Two contributors on different machines
   resolve identical dependency trees.
3. **Auditability.** The exact version of every dependency is
   recorded in version control. A change to a dependency version
   appears in the git history.

Without `go.sum`, dependency resolution would depend on the state
of the local module cache and the state of the upstream repository
at the moment of the build. Two builds of the same commit could
produce different results.

### Why manual edits are forbidden

`go.sum` has a strict format: each line contains two or three
whitespace-separated fields:

```text
<module-path> <version> <hash>
<module-path> <version>/go.mod <hash>
```

The hash is a base64-encoded cryptographic digest. A single
character change to the hash produces a value that does not match
the actual module, and the build fails with a checksum mismatch. A
comment line, a blank line, or any other non-conforming line causes
a parse error.

Manual edits are forbidden because:

1. **They cannot produce a valid hash.** Computing the hash requires
   downloading the module and hashing its contents. This is the
   toolchain's job.
2. **They cannot reason about the transitive tree.** Adding a
   checksum for one module requires adding checksums for every
   module that module depends on. The toolchain computes the
   closure; a human cannot.
3. **They cannot detect conflicts.** Two modules may declare
   incompatible dependencies on a third. The toolchain detects the
   conflict and fails. A manual edit would silently produce an
   inconsistent lockfile.

### How to regenerate `go.sum` legitimately

When `go.sum` is genuinely stale — for example, after a dependency
was added or removed — regenerate it with `task tidy` on a clean
branch. See [How `go mod tidy` should be
run](#how-go-mod-tidy-should-be-run).

The regeneration procedure is:

1. Create a clean branch from `main`.
2. Run `task tidy`.
3. Inspect the diff.
4. Commit both `go.mod` and `go.sum` together.

Never edit `go.sum` by hand. Never commit a partially regenerated
`go.sum`.

### The integrity check

The integrity check is implemented in
`scripts/deps/verify-integrity.sh` and invoked by
`task verify:deps:integrity`. It verifies:

1. `go.sum` exists at the repository root.
2. `go.sum` is tracked by git.
3. `go.mod` and `go.sum` have no uncommitted changes.
4. `go mod tidy` produces no diff.
5. A corrupted `go.sum` causes a checksum failure under
   `-mod=readonly`.

The last check is a negative test: it proves that the integrity
guarantee is enforced by the toolchain, not just asserted by the
script.

Run it:

```bash
task verify:deps:integrity
```

---

## Reproducible Dependency Resolution

Integrity is a local property. **Reproducibility** is a global
property: do two independent clean environments resolve the same
module tree from the same commit?

This section defines the invariant, defines "clean environment",
documents the verification procedure, and lists the known
non-reproducibility sources.

### The invariant

> Given the same commit and the same `go.mod`, `go.sum`, and pinned
> toolchain, two independent clean environments must resolve to the
> identical module tree.

"Identical module tree" means:

- The same set of modules, with the same versions.
- The same ordering when sorted lexically.
- The same set of transitive dependencies.

The invariant is verified by
`scripts/reproducible/verify-resolution.sh`, which is invoked by
`task verify:resolution`. It runs in CI on every pull request.

### What "clean environment" means

A clean environment is one that satisfies all of the following:

1. **A fresh Go module cache.** `GOMODCACHE` points to a directory
   that either does not yet exist or is empty. The toolchain must
   download and populate the cache from scratch.
2. **A fresh `GOPATH`.** `GOPATH` points to a directory that is
   unrelated to any existing workspace. This prevents the toolchain
   from reusing state from previous resolutions.
3. **No reuse of prior build artifacts.** The `GOCACHE` directory
   may be warm, but it must not contain entries that influence
   module resolution. In practice, the resolution step
   (`go list -m all`) does not consult `GOCACHE`, so a warm cache
   does not violate the invariant.
4. **The pinned toolchain.** The Go binary on `PATH` must be the
   version declared by the `toolchain` directive in `go.mod`, or a
   version that Go's automatic toolchain selection would use.
   `GOTOOLCHAIN=auto` (the default) satisfies this condition.

The verification script constructs two such environments in
temporary directories and compares their resolved trees.

### The verification procedure

The script performs the following steps:

1. **Check preconditions.** Verify that `go.mod` and `go.sum` exist,
   are committed, and have no uncommitted changes.
2. **Capture the committed tree.** Run `go list -m all` in the
   current environment. This is the reference tree.
3. **Verify the reference tree is complete.** Count the `require`
   directives in `go.mod` and confirm that the reference tree
   contains at least as many modules. This catches the case where
   the module cache is empty and the toolchain resolved nothing.
4. **Resolve in environment A.** Run `go list -m all` with a
   temporary `GOMODCACHE` and `GOPATH`. Capture the sorted output.
5. **Resolve in environment B.** Repeat with a different temporary
   `GOMODCACHE` and `GOPATH`. Capture the sorted output.
6. **Compare environments A and B.** If the trees differ, fail with
   a diff.
7. **Compare environment A against the committed tree.** If the
   trees differ, fail with a diff. This catches the case where
   `go.sum` is stale relative to `go.mod`.

If all checks pass, the invariant holds.

### What the procedure does not verify

The verification procedure is deliberately scoped. It does **not**
verify:

- **Build output byte-for-byte reproducibility.** The procedure
  compares module trees, not compiled binaries. Build
  reproducibility is a separate concern handled by WBS 17.x.
- **Toolchain reproducibility across Go versions.** The procedure
  assumes both environments use the pinned toolchain. Verifying
  cross-version behaviour is the domain of the version matrix
  defined in [Supported Go Versions](#supported-go-versions).
- **Network-dependent behaviour.** Both environments use the same
  `GOPROXY` and `GOSUMDB` settings. If those settings differ, the
  procedure may pass or fail depending on the specific
  configuration.
- **Correctness of the resolved modules.** A malicious or
  compromised module resolves identically in both environments.
  The procedure verifies consistency, not correctness.

### Known non-reproducibility sources

The following conditions can cause the invariant to fail. Each is
listed with the mitigation.

#### `GOFLAGS` differences between environments

If one environment has `GOFLAGS=-mod=readonly` and another has
`GOFLAGS=-mod=mod`, the two environments may resolve differently.

**Mitigation:** `GOFLAGS` is set consistently by the pre-commit
hook and by CI. Contributors set it in their shell profile. The
check in `verify:reproducible:env` verifies that
`GOFLAGS=-mod=readonly` is set.

#### `GOPROXY` differences

If one environment uses `GOPROXY=https://proxy.golang.org` and
another uses a private mirror, the two environments may resolve to
different module versions if the mirror is stale.

**Mitigation:** `GOPROXY` is documented as an environment-specific
setting in [Module Resolution Policy](#module-resolution-policy).
Contributors behind restrictive networks set it explicitly. The
invariant is verified against the current environment's `GOPROXY`;
if a contributor uses a different `GOPROXY` than CI, the two may
diverge.

#### `GONOSUMDB` / `GOSUMDB` configurations

If one environment disables checksum verification and another does
not, the two environments may accept different modules.

**Mitigation:** `GOSUMDB=sum.golang.org` is the default and is never
overridden. `GONOSUMDB` and `GOPRIVATE` are empty in Phase 2.
Contributors who need to override them do so with explicit
documentation in their environment.

#### Proxy cache staleness

If a module is republished with a different hash under the same
version (which is not supposed to happen but can occur if the proxy
is compromised), one environment may resolve to the new hash and
another to the old one.

**Mitigation:** `go.sum` pins the exact checksum. Any mismatch
causes a build failure. The pre-commit hook and CI both verify that
the committed `go.sum` is consistent with `go.mod`.

#### Network partition

If one environment can reach the proxy and another cannot, the
second environment may resolve to a subset of the tree (using the
local cache only) or fail outright.

**Mitigation:** The verification script requires the resolved trees
to be complete. If either environment resolves fewer modules than
`go.mod` declares, the script fails.

### Running the check locally

```bash
task verify:resolution
```

Expected output:

```text
Forge reproducible dependency resolution
─────────────────────────────────────────
✓ preconditions satisfied
✓ committed module tree contains N modules
→ resolving in environment A
✓ environment A resolved N modules
→ resolving in environment B
✓ environment B resolved N modules
✓ environment A and environment B resolved identically
✓ resolved tree matches the committed tree

✓ reproducibility invariant holds
```

The check runs in under 30 seconds on a warm Go module cache. On a
cold cache, it may take longer if modules need to be downloaded.

---

## Module Resolution Policy

Forge's module resolution decisions are documented in
[`docs/dependency-policy.md`](dependency-policy.md), section
"Module resolution policy". The policy specifies:

- **`GOPROXY`** — default (`https://proxy.golang.org,direct`). No
  override.
- **`GOSUMDB`** — default (`sum.golang.org`). Never disabled.
- **`GOPRIVATE`** — empty. No private modules in Phase 2.
- **`GOMODCACHE`** — default (`$GOPATH/pkg/mod`). Overrides only in
  CI runners that require isolation.
- **Vendoring** — not adopted.

The policy is enforced by
`scripts/deps/verify-env-policy.sh`, which is invoked by
`task verify:deps:env-policy`. The check runs in CI and in the
pre-commit hook.

### Running the policy check

```bash
task verify:deps:env-policy
```

Expected output:

```text
OK: GOFLAGS=-mod=readonly
OK: GOTOOLCHAIN=auto
OK: GOSUMDB=sum.golang.org (default)
OK: GONOSUMDB is empty
OK: GONOSUMCHECK is not set
OK: GO111MODULE is not set
OK: GOINSECURE is not set
OK: GOPROXY= (default: https://proxy.golang.org,direct)
OK: GOMODCACHE= (default: $GOPATH/pkg/mod)
OK: no vendor/ directory

OK: environment conforms to the module resolution policy
```

If the check fails, the failure message identifies which variable
is misconfigured and how to fix it.

### Environment template

Contributors behind restrictive networks or internal proxies should
use the template file [`docs/env.example`](env.example) as a
starting point. The file documents the environment variables that
can be overridden, with comments explaining the tradeoffs.

The template is not loaded automatically. It is a reference, not a
configuration.

---

## Detecting Dependency Drift

Drift is any condition in which `go.mod` or `go.sum` does not
accurately reflect the module's actual dependency requirements.
Drift is insidious: it does not break the current build, but it
breaks future builds on other machines, in CI, or after a
dependency update.

Drift is detected in three places:

1. **Locally, before committing** — the pre-commit hook runs
   `verify:deps`, which includes the drift check.
2. **Locally, before pushing** — the pre-push hook runs
   `verify`, which includes the drift check.
3. **In CI, on every push and pull request** — the `drift` job in
   `.github/workflows/ci.yml` runs the check as the first stage.

CI reports drift with one of five identifiers: D1, D2, D3, D4, or
D5. Each identifier corresponds to a specific condition and a
specific fix.

### The five drift types

#### D1 — `go.sum` is missing entries required by `go.mod`

A dependency was added to `go.mod` without regenerating `go.sum`.
The next build on a fresh clone fails with a missing-checksum
error.

**Detection:** `go mod verify`.

**Fix:**

```bash
go mod tidy
git diff go.mod go.sum
git add go.mod go.sum
git commit -m "chore(deps): regenerate go.sum"
```

#### D2 — `go.sum` has extra entries not referenced by `go.mod`

A dependency was removed from `go.mod` without regenerating
`go.sum`. The stale entries cause no immediate failure, but they
accumulate over time and confuse dependency audits.

**Detection:** `go mod tidy` produces a diff that removes lines
from `go.sum`.

**Fix:**

```bash
go mod tidy
git diff go.sum
git add go.mod go.sum
git commit -m "chore(deps): remove stale go.sum entries"
```

#### D3 — `go.mod` is not tidy

A source file was edited to add or remove an import, but `go.mod`
was not regenerated. The `require` block no longer matches the
source tree.

**Detection:** `go mod tidy` produces a diff that changes `go.mod`.

**Fix:**

```bash
go mod tidy
git diff go.mod
git add go.mod go.sum
git commit -m "chore(deps): regenerate go.mod"
```

#### D4 — `go.mod` / `go.sum` inconsistent with the pinned toolchain

A `require` directive declares a Go version newer than the pinned
toolchain, or the source tree uses a language feature the toolchain
does not support.

**Detection:** `go build ./...` fails with an error mentioning
`requires go >=` or `go.mod requires`.

**Fix:**

Review the Go version matrix in this document (see [Supported Go
Versions](#supported-go-versions)). Either:

- **Downgrade the requirement.** If the source tree does not
  actually need the newer version, edit the offending source file
  to avoid the newer feature.
- **Upgrade the toolchain.** If the source tree genuinely needs the
  newer version, update the `go` directive in `go.mod` and the
  `GO_MIN` variable in `Taskfile.yml`. This is an ADR-level change:
  see [`docs/decisions/`](decisions/) for the process.

```bash
# Downgrade case: edit the source, then regenerate the lockfiles.
go mod tidy

# Upgrade case: update the version matrix, then regenerate.
# (Requires an ADR that supersedes ADR-004.)
```

#### D5 — Unpinned dependency version

A dependency was added with `go get @master` or `go get` without
an explicit version. The resolved version is a pseudo-version, a
`+incompatible` marker, or a branch name, none of which are
reproducible.

**Detection:** regex match against `go.mod`.

**Fix:**

```bash
# Re-pin the dependency to an explicit semantic version.
go get <module>@<version>

# Verify the pin.
task verify:deps:pin
```

### The CI stage

The drift check runs as the first CI stage on every push to `main`
and every pull request targeting `main`. It is defined in
`.github/workflows/ci.yml`:

```yaml
drift:
  name: Dependency drift
  runs-on: ubuntu-latest
  timeout-minutes: 5
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version-file: go.mod
        cache: true
    - run: sh scripts/deps/detect-drift.sh
```

The job runs only on `ubuntu-latest` because drift is
deterministic across operating systems. Running it on the full
matrix would waste CI compute without providing additional
coverage.

### Branch protection

The `drift` job is a **required status check** for merging to
`main`. The branch protection rule is configured on the GitHub
repository settings page:

- **Settings → Branches → Branch protection rules → `main` → Require status checks to pass before merging**.
- Required checks: `Dependency drift`.
- **Require branches to be up to date before merging** is enabled.

The rule is not enforced by this document. It is enforced by the
GitHub repository settings. The document specifies what the rule
must be so that the intent is preserved if the settings are ever
recreated.

### Fixing drift

The fix procedure for each drift type is documented above. In
summary:

| Drift type | Fix                                                                |
|------------|---------------------------------------------------------------------|
| D1         | `go mod tidy`, review diff, commit.                                |
| D2         | `go mod tidy`, review diff, commit.                                |
| D3         | `go mod tidy`, review diff, commit.                                |
| D4         | Align the toolchain with `go.mod`, per the version matrix.         |
| D5         | Re-pin the dependency with `go get <module>@<version>`.            |

After applying the fix, run:

```bash
task verify:deps:drift
```

The check must report no drift before the fix is considered
complete.

---

## Getting Started

### Prerequisites

1. Install Git 2.30 or later.
2. Install [Task](https://taskfile.dev/) 3.x.
3. Install Node.js 18 or later (only for Markdown linting).
4. Install Go 1.23.4, or any version at or above 1.23.0. Go will
   auto-select the pinned toolchain.
5. Install `markdownlint-cli2`:

    ```bash
    npm install -g markdownlint-cli2
    ```

    Alternatives:

    ```bash
    # Homebrew (macOS, Linux)
    brew install markdownlint-cli2

    # Go
    go install github.com/DavidAnson/markdownlint-cli2@latest
    ```

### Clone and build

```bash
git clone git@github.com:thapelomagqazana/forge.git
cd forge
task module:verify              # confirm the module is correctly configured
task goversion:verify           # confirm the version matrix is documented
task verify:reproducible        # confirm the environment-level invariant
task verify:resolution          # confirm the cross-environment invariant
task verify:deps:integrity      # confirm the lockfile integrity
task verify:deps:env-policy     # confirm the environment policy
task verify:deps:drift          # confirm no dependency drift
task check                      # run the full quality gate
task build                      # produce ./forge
```

### Run the binary

```bash
./forge version
./forge --help
```

### Verify CI locally

The CI workflow runs on GitHub Actions. To simulate it locally:

```bash
task check
```

`task check` runs the same checks in the same order as CI. If it
passes locally, CI will likely pass as well.

---

## Task Runner

Every routine operation is exposed as a `task`. Do not run `go`
commands directly except for debugging or one-off experiments.

Run `task --list` to see all available tasks. The most important
ones:

| Task                              | Purpose                                                       |
|-----------------------------------|---------------------------------------------------------------|
| `task module:verify`              | Verify the Go module configuration (WBS 2.1.1).               |
| `task goversion:verify`           | Verify the Go version matrix (WBS 2.1.2).                     |
| `task verify:reproducible`        | Verify the environment-level reproducibility invariant (WBS 2.3.1). |
| `task verify:resolution`          | Verify the cross-environment reproducibility invariant (WBS 3.2.1). |
| `task verify:cobra`               | Verify the Cobra pin and wiring (WBS 2.4.1).                  |
| `task verify:deps:env-policy`     | Verify the module resolution policy (WBS 3.2.2).              |
| `task verify:deps:integrity`      | Verify the lockfile integrity (WBS 3.1.1).                    |
| `task verify:deps:drift`          | Detect go.mod / go.sum drift (WBS 3.3.1).                     |
| `task verify:deps`                | Verify every dependency policy check (WBS 2.5.1, 3.1.1, 3.2.2, 3.3.1). |
| `task verify`                     | Run every verification check.                                 |
| `task tidy`                       | Run `go mod tidy` (maintainers only; clean branch required).  |
| `task build`                      | Build the binary with injected metadata.                      |
| `task build:debug`                | Build without `-trimpath` for debugger support.               |
| `task test`                       | Run unit tests with the race detector.                        |
| `task test:integration`           | Run integration tests (compiles and invokes the binary).      |
| `task test:scripts`               | Run shell test harnesses for the scripts/ directory.          |
| `task fmt`                        | Format all Go source files.                                   |
| `task fmt:check`                  | Verify formatting (fails if not formatted).                   |
| `task vet`                        | Run `go vet` across all packages.                             |
| `task lint`                       | Run all linters (currently Markdown).                         |
| `task lint:md`                    | Lint Markdown files.                                          |
| `task lint:md:fix`                | Auto-fix Markdown violations where possible.                  |
| `task check`                      | Run the full quality gate.                                    |
| `task clean`                      | Remove build artifacts.                                       |
| `task hooks:install`              | Install the local Git hooks (run once).                       |
| `task hooks:status`               | Report whether the Git hooks are installed.                   |
| `task adr:new -- "<title>"`       | Create a new Architecture Decision Record.                    |
| `task adr:list`                   | List all ADRs.                                                |
| `task adr:lint`                   | Check all ADRs for format completeness.                       |

### Running tasks from a subdirectory

All tasks set `dir: "{{.ROOT}}"`, so they work from any
subdirectory of the repository. You do not need to `cd` to the root.

The `ROOT` variable resolves to the repository root regardless of
the current directory, computed via `git rev-parse --show-toplevel`.
This is deliberate: verification tasks must operate on the
repository's own files, not on the user's current directory.

---

## Testing

### Run all tests

```bash
task test
```

This runs:

```bash
go test -race -count=1 ./...
```

### Run tests for a single package

```bash
go test -race -count=1 ./internal/config/...
```

### Run a single test by name

```bash
go test -race -run TestConfigCommand_DefaultOutput ./internal/cli/...
```

### Race detector

All tests run with `-race` by default. This is non-negotiable:
Forge is a concurrent CLI, and unsynchronised state is a bug
waiting to happen.

### Test fixtures

Test fixtures live in `internal/<package>/testdata/`. Each fixture
directory is self-contained and named after the test it supports.

### Integration tests

Some tests spawn the compiled binary as a subprocess. These are
gated behind the `integration` build tag and are not run by
`task test`. Run them explicitly:

```bash
task test:integration
```

### Script tests

The shell scripts in `scripts/` are tested by shell harnesses in
`tests/scripts/`. Run them with:

```bash
task test:scripts
```

Each harness creates synthetic repositories in temporary
directories and exercises the script against them.

---

## Formatting

```bash
task fmt        # format all files
task fmt:check  # verify formatting (CI uses this)
```

The `fmt:check` task runs `gofmt -l .` and fails if any file is not
formatted. This is enforced in CI.

Do not manually adjust indentation, alignment, or whitespace beyond
what `gofmt` produces. If you disagree with `gofmt`'s output,
disagree with the Go community, not with your teammate's pull
request.

---

## Static Analysis

```bash
task vet
```

Runs `go vet ./...`. This catches a class of bugs that the compiler
misses, including format string mismatches, unreachable code, and
suspicious pointer usage.

Additional linters may be introduced in WBS 16.3.

---

## Markdown Linting

Documentation is part of the product. Markdown files are linted with
[markdownlint-cli2](https://github.com/DavidAnson/markdownlint-cli2)
against the rules defined in `.markdownlint.yaml`.

### Check

```bash
task lint:md
```

Fails the build on any violation.

### Auto-fix

```bash
task lint:md:fix
```

Modifies files in place where the violation can be fixed
automatically. This task is **not** run in CI; it is a contributor
convenience. After running, review the diff and commit intentional
changes.

### The most common violations

| Rule   | Meaning                                                             |
|--------|---------------------------------------------------------------------|
| MD022  | Headings must be surrounded by exactly one blank line.              |
| MD032  | Lists must be surrounded by blank lines.                            |
| MD031  | Fenced code blocks must be surrounded by blank lines.               |
| MD040  | Fenced code blocks must specify a language.                         |
| MD044  | Proper names must use canonical capitalisation (`CLI`, `Linux`, ...). |

If `task lint:md` fails, read the error carefully; the message
identifies the exact rule and location.

### The proper-names list

The following names must always be capitalised as shown. The linter
enforces this:

- **Acronyms:** `CLI`, `API`, `SDK`, `JSON`, `TOML`, `SARIF`, `OIDC`,
  `SAML`, `SCIM`, `RBAC`, `SSO`, `ADR`.
- **Products:** `GitHub`, `GitLab`, `Docker`, `Kubernetes`,
  `PostgreSQL`, `SQLite`.
- **Tools:** `Cobra`, `Viper`, `Makefile`.
- **Operating systems:** `Linux`, `macOS`, `Windows`.
- **Brand:** `Forge`.

---

## Editor Configuration

The repository includes a `.editorconfig` file that standardises
indentation, line endings, and whitespace across editors.

### Editors that support `.editorconfig` natively

- VS Code (built-in)
- GoLand and IntelliJ IDEA (built-in)
- Neovim (via plugin)
- Vim (via plugin)

### Editors that require a plugin

- Sublime Text
- Atom
- Emacs

Install the `.editorconfig` plugin for your editor to benefit.

### Recommended Go-specific configuration

For VS Code, add to `.vscode/settings.json`:

```json
{
  "go.useLanguageServer": true,
  "go.formatTool": "gofmt",
  "go.lintTool": "golangci-lint",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.insertSpaces": false
  }
}
```

For GoLand, no additional configuration is required. It respects
`gofmt` by default.

---

## Troubleshooting

### `go: cannot find main module`

You are not inside the Forge repository, or `go.mod` is missing.
Check with `pwd` and `ls go.mod`.

### `go: downloading go1.23.4`

This is expected on first build. Go's automatic toolchain selection
is fetching the recommended toolchain. Subsequent builds reuse the
cached toolchain.

### `go: module path mismatch`

The module path in `go.mod` does not match `Taskfile.yml`. Run
`task module:verify` to see the exact mismatch. Fix by editing both
files in the same commit.

### `go: go.mod requires go >= 1.23.0`

You are running an older Go toolchain and have `GOTOOLCHAIN=local`
set. Either unset `GOTOOLCHAIN` to allow auto-selection, or install
Go 1.23 or later.

### `go build: updates to go.mod needed, disabled by -mod=readonly`

Your `go.mod` or `go.sum` is stale relative to the source tree. This
is expected: `-mod=readonly` is doing its job.

To fix: run `task tidy` on a clean branch, review the diff, and
commit the updated lockfiles. See [How `go mod tidy` should be
run](#how-go-mod-tidy-should-be-run).

### `FAIL: go.sum is missing entries required by go.mod` (D1)

The `verify:deps:drift` check reports D1. A dependency was added to
`go.mod` without regenerating `go.sum`.

To fix:

```bash
task tidy
git diff go.mod go.sum
git add go.mod go.sum
git commit -m "chore(deps): regenerate go.sum"
```

See [D1 — `go.sum` is missing entries required by
`go.mod`](#d1--gosum-is-missing-entries-required-by-gomod).

### `FAIL: go.mod is not tidy` (D3)

The `verify:deps:drift` check reports D3. A source file was edited
to add or remove an import, but `go.mod` was not regenerated.

To fix:

```bash
task tidy
git diff go.mod go.sum
git add go.mod go.sum
git commit -m "chore(deps): regenerate go.mod"
```

See [D3 — `go.mod` is not
tidy](#d3--gomod-is-not-tidy).

### `FAIL: unpinned dependency version` (D5)

The `verify:deps:drift` check reports D5. A dependency was added
without an explicit version.

To fix:

```bash
go get <module>@<version>
task verify:deps:pin
git add go.mod go.sum
git commit -m "chore(deps): pin <module> to <version>"
```

See [D5 — Unpinned dependency
version](#d5--unpinned-dependency-version).

### `FAIL: GOFLAGS does not include -mod=readonly`

The environment policy check reports a `GOFLAGS` violation. Set
`GOFLAGS=-mod=readonly` in your shell profile. See [How to set the
environment](#how-to-set-the-environment).

### `FAIL: GOTOOLCHAIN has an unexpected value`

The environment policy check reports a `GOTOOLCHAIN` violation.
Set `GOTOOLCHAIN=auto` in your shell profile.

### `FAIL: GOSUMDB has a non-default value`

The environment policy check reports a `GOSUMDB` violation. Unset
`GOSUMDB` (it defaults to `sum.golang.org`). Do not override it.

### `FAIL: GONOSUMCHECK is set`

The environment policy check reports a `GONOSUMCHECK` violation.
Unset `GONOSUMCHECK`. It is deprecated and its presence breaks
checksum verification.

To fix:

```bash
unset GONOSUMCHECK
```

Add the `unset` to your shell profile so it persists.

### `FAIL: no vendor/ directory` (or the opposite)

The environment policy check reports a vendoring violation. If a
`vendor/` directory exists, remove it:

```bash
rm -rf vendor
```

See [Module Resolution Policy](#module-resolution-policy).

### `gofmt` reports files you did not modify

Another contributor committed unformatted code. Run `task fmt` to
fix locally; the fix appears in your next pull request. Consider
filing an issue in parallel.

### `task lint:md` reports violations you cannot see

The linter checks rules that are invisible in a rendered preview:

- Blank lines around headings, lists, and code fences.
- Trailing whitespace.
- Fenced code block language identifiers.

Open the file in a raw text editor and look at the reported line
numbers. `task lint:md:fix` resolves most violations automatically.

### `task` not found

Install Task from <https://taskfile.dev/installation/>. The
repository does not have a `Makefile`.

### `markdownlint-cli2` not found

Install with one of:

```bash
npm install -g markdownlint-cli2
brew install markdownlint-cli2
go install github.com/DavidAnson/markdownlint-cli2@latest
```

### `go mod tidy` produces unexpected changes

If `go mod tidy` removes a dependency you expected, the dependency
is not imported by any file in the module. Verify with:

```bash
grep -r "<import-path>" --include='*.go' .
```

If the dependency is genuinely needed, ensure at least one file
imports it, even a blank import for side effects.

---

## See Also

### Core documentation

- [`docs/architecture.md`](architecture.md) — module structure and
  package boundaries.
- [`docs/cli-ux-spec.md`](cli-ux-spec.md) — CLI behaviour contracts.
- [`docs/dependency-policy.md`](dependency-policy.md) — dependency
  addition criteria, versioning policy, and module resolution policy.
- [`docs/phase-2-requirements.md`](phase-2-requirements.md) — the
  authoritative requirement register.
- [`docs/decisions/`](decisions/) — Architecture Decision Records.

### Scripts and tooling

- [`scripts/deps/`](../scripts/deps/) — dependency integrity and
  policy checks.
- [`scripts/deps/README.md`](../scripts/deps/README.md) —
  documentation for the dependency scripts.
- [`scripts/reproducible/`](../scripts/reproducible/) —
  reproducibility checks.
- [`scripts/reproducible/README.md`](../scripts/reproducible/README.md)
  — documentation for the reproducibility scripts.
- [`scripts/adr/`](../scripts/adr/) — Architecture Decision Record
  tooling.
- [`tests/scripts/`](../tests/scripts/) — shell test harnesses.

### Configuration

- [`Taskfile.yml`](../Taskfile.yml) — task definitions.
- [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) — CI
  workflow.
- [`.editorconfig`](../.editorconfig) — cross-editor formatting.
- [`.markdownlint.yaml`](../.markdownlint.yaml) — Markdown lint
  rules.
- [`docs/env.example`](env.example) — environment template for
  contributors behind restrictive networks.
- [`docs/dependency-policy.md`](dependency-policy.md) — the full
  dependency policy.
