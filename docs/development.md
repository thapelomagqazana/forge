# Development Guide

This document describes how to set up, build, test, and contribute to
Forge. Every contributor should be able to move from a fresh clone to a
passing test run in under ten minutes.

---

## Table of Contents

- [Supported Development Environment](#supported-development-environment)
- [Supported Go Versions](#supported-go-versions)
- [Reproducible Builds](#reproducible-builds)
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

The invariant is verified in CI. Any pull request that breaks it fails
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

CI runners must set these explicitly. The workflow file
`.github/workflows/ci.yml` (implemented in WBS 18.0) does so at the
job level:

```yaml
env:
  GOFLAGS: -mod=readonly
  GOTOOLCHAIN: auto
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

To verify that your local environment satisfies the invariant:

```bash
task verify:reproducible
```

This runs four checks:

1. `GOFLAGS` and `GOTOOLCHAIN` are set as required.
2. `go.mod` and `go.sum` are unchanged after `go mod tidy`.
3. `go list -m all` produces the same output on two consecutive runs.
4. The negative test: a tampered `go.mod` fails to build under
   `-mod=readonly`.

If any check fails, the output explains what to fix.

### The negative test

To prove that `-mod=readonly` actually prevents mutation:

```bash
task verify:reproducible:negative
```

This task:

1. Copies `go.mod`, `go.sum`, `cmd/`, and `internal/` to a temporary
   directory.
2. Appends a deliberate inconsistency to the copy's `go.mod`.
3. Runs `go build ./...` with `GOFLAGS=-mod=readonly` in the temporary
   directory.
4. Asserts that the build **fails**.
5. Removes the temporary directory.

The working tree is never modified.

The negative test is the strongest evidence that the policy is
enforced. It runs in CI on every pull request.

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
task module:verify           # confirm the module is correctly configured
task goversion:verify        # confirm the version matrix is documented
task verify:reproducible     # confirm the reproducibility invariant
task check                   # run the full quality gate
task build                   # produce ./forge
```

### Run the binary

```bash
./forge version
./forge --help
```

---

## Task Runner

Every routine operation is exposed as a `task`. Do not run `go` commands
directly except for debugging or one-off experiments.

Run `task --list` to see all available tasks. The most important ones:

| Task                          | Purpose                                                       |
|-------------------------------|---------------------------------------------------------------|
| `task module:verify`          | Verify the Go module configuration (WBS 2.1.1).               |
| `task goversion:verify`       | Verify the Go version matrix (WBS 2.1.2).                     |
| `task verify:reproducible`    | Verify the reproducibility invariant (WBS 2.3.1).             |
| `task tidy`                   | Run `go mod tidy` (maintainers only; clean branch required).  |
| `task build`                  | Build the binary with injected metadata.                      |
| `task build:debug`            | Build without `-trimpath` for debugger support.               |
| `task test`                   | Run tests with the race detector.                             |
| `task fmt`                    | Format all Go source files.                                   |
| `task fmt:check`              | Verify formatting (fails if not formatted).                   |
| `task vet`                    | Run `go vet` across all packages.                             |
| `task lint`                   | Run all linters (currently Markdown).                         |
| `task lint:md`                | Lint Markdown files.                                          |
| `task lint:md:fix`            | Auto-fix Markdown violations where possible.                  |
| `task check`                  | Run the full quality gate.                                    |
| `task verify`                 | Run all verification tasks.                                   |
| `task clean`                  | Remove build artifacts.                                       |
| `task adr:new -- "<title>"`   | Create a new Architecture Decision Record.                    |
| `task adr:list`               | List all ADRs.                                                |
| `task adr:lint`               | Check all ADRs for format completeness.                       |

### Running tasks from a subdirectory

All tasks set `dir: "{{.USER_WORKING_DIR}}"`, so they work from any
subdirectory of the repository. You do not need to `cd` to the root.

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

All tests run with `-race` by default. This is non-negotiable: Forge
is a concurrent CLI, and unsynchronised state is a bug waiting to
happen.

### Test fixtures

Test fixtures live in `internal/<package>/testdata/`. Each fixture
directory is self-contained and named after the test it supports.

### Integration tests

Some tests spawn the compiled binary as a subprocess. These are gated
behind the `integration` build tag and are not run by `task test`.
Run them explicitly:

```bash
go test -race -tags=integration ./...
```

---

## Formatting

```bash
task fmt        # format all files
task fmt:check  # verify formatting (CI uses this)
```

The `fmt:check` task runs `gofmt -l .` and fails if any file is not
formatted. This is enforced in CI.

Do not manually adjust indentation, alignment, or whitespace beyond
what `gofmt` produces. If you disagree with `gofmt`'s output, disagree
with the Go community, not with your teammate's pull request.

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
- **Products:** `GitHub`, `GitLab`, `Docker`, `Kubernetes`, `PostgreSQL`,
  `SQLite`.
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

You are not inside the Forge repository, or `go.mod` is missing. Check
with `pwd` and `ls go.mod`.

### `go: downloading go1.23.4`

This is expected on first build. Go's automatic toolchain selection is
fetching the recommended toolchain. Subsequent builds reuse the cached
toolchain.

### `go: module path mismatch`

The module path in `go.mod` does not match `Taskfile.yml`. Run
`task module:verify` to see the exact mismatch. Fix by editing both
files in the same commit.

### `go: go.mod requires go >= 1.23.0`

You are running an older Go toolchain and have `GOTOOLCHAIN=local` set.
Either unset `GOTOOLCHAIN` to allow auto-selection, or install Go 1.23
or later.

### `go build: updates to go.mod needed, disabled by -mod=readonly`

Your `go.mod` or `go.sum` is stale relative to the source tree. This is
expected: `-mod=readonly` is doing its job.

To fix: run `task tidy` on a clean branch, review the diff, and commit
the updated lockfiles. See
[How `go mod tidy` should be run](#how-go-mod-tidy-should-be-run).

### `gofmt` reports files you did not modify

Another contributor committed unformatted code. Run `task fmt` to fix
locally; the fix appears in your next pull request. Consider filing an
issue in parallel.

### `task lint:md` reports violations you cannot see

The linter checks rules that are invisible in a rendered preview:

- Blank lines around headings, lists, and code fences.
- Trailing whitespace.
- Fenced code block language identifiers.

Open the file in a raw text editor and look at the reported line
numbers. `task lint:md:fix` resolves most violations automatically.

### `task` not found

Install Task from <https://taskfile.dev/installation/>. The repository
does not have a `Makefile`.

### `markdownlint-cli2` not found

Install with one of:

```bash
npm install -g markdownlint-cli2
brew install markdownlint-cli2
go install github.com/DavidAnson/markdownlint-cli2@latest
```

### `go mod tidy` produces unexpected changes

If `go mod tidy` removes a dependency you expected, the dependency is
not imported by any file in the module. Verify with:

```bash
grep -r "<import-path>" --include='*.go' .
```

If the dependency is genuinely needed, ensure at least one file
imports it, even a blank import for side effects.

---

## See Also

- [`docs/architecture.md`](./architecture.md) — module structure and
  package boundaries.
- [`docs/cli-ux-spec.md`](./cli-ux-spec.md) — CLI behaviour contracts.
- [`docs/dependency-policy.md`](./dependency-policy.md) — dependency
  addition criteria and versioning policy.
- [`docs/phase-2-requirements.md`](./phase-2-requirements.md) —
  the authoritative requirement register.
- [`docs/decisions/`](./decisions/) — Architecture Decision Records.
- [`Taskfile.yml`](../Taskfile.yml) — task definitions.
- [`.editorconfig`](../.editorconfig) — cross-editor formatting.
- [`.markdownlint.yaml`](../.markdownlint.yaml) — Markdown lint rules.
