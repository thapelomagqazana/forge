<!--
  Verifying Forge binaries
  ========================
  Public contract. Versioned with the Forge release. Additive changes
  only.

  Normative references:
    - Platform matrix:     docs/reference/platforms.yaml
    - Versioning scheme:   docs/reference/versioning.md
    - Reproducible builds: docs/guides/reproducible-builds.md
    - Building:            docs/guides/building.md
    - Release channels:    docs/reference/release-channels.md
    - Version output:      docs/reference/version-output.md
    - Thresholds ADR:      docs/decisions/ADR-047-binary-verification-thresholds.md

  Editing rules:
    1. Every threshold in this document MUST appear as a constant in
       cmd/forge-internal-verify/main.go. The prose and the code must
       agree. A change to either is a change to both, in the same
       commit.
    2. Every check this document describes MUST be implemented. A
       check that appears here but not in the verifier is a defect.
    3. Every example in this document MUST be a command a user can
       run. An example that fails on a clean checkout is a defect in
       this document.
    4. The "What the check does not catch" sections are part of the
       contract. A reader relies on them to know the check's limits.
       Removing a limit's description without removing the limit is
       a defect.
-->

# Verifying Forge binaries

The Forge binary verification suite runs ten smoke checks against
the compiled artifacts in a `dist/` directory. It is the last line
of defense between a broken build pipeline and a published release.

The suite is implemented in
[`cmd/forge-internal-verify`](../../cmd/forge-internal-verify/main.go)
and is invoked by the Makefile's `release-dry-run` target, by the
`release:verify-binaries` Taskfile target, and by the
`verify-binaries` CI job on every `v*` tag.

## Running the suite

From the repo root, after `task release:build-all` or
`make build-all` has produced a `dist/` directory:

```sh
go run ./cmd/forge-internal-verify
```

The verifier accepts four flags:

| Flag | Default | Purpose |
|---|---|---|
| `--dist=<dir>` | `dist` | Directory containing the artifacts |
| `--platform=<goos>/<goarch>` | (all) | Verify only one platform |
| `--verbose` | `false` | Print per-check timings |
| `--root=<dir>` | (resolved) | Repo root; resolved from cwd if empty |

The `--platform` flag accepts a single platform in `goos/goarch`
form (for example, `linux/amd64`) or by its `name` field from
`platforms.yaml` (for example, `linux-amd64`). It is used by the CI
matrix, which runs one job per platform.

## The ten checks

For each Tier 1 and Tier 2 platform in
[`platforms.yaml`](../reference/platforms.yaml), the suite performs:

| # | Name | What it asserts |
|---|---|---|
| 1 | `exists` | The artifact file is present in `dist/`. |
| 2 | `arch` | The executable's header reports the expected `GOOS`/`GOARCH`. |
| 3 | `static` | The binary has no dynamic dependencies introduced by the build. |
| 4 | `version` | `forge version` reports a version string. |
| 5 | `version-json` | `forge version --format json` emits valid JSON with the five schema fields. |
| 6 | `help-exit0` | `forge --help` exits 0 and prints help. |
| 7 | `version-exit0` | `forge --version` exits 0. |
| 8 | `unknown-exit` | `forge unknown-command` exits 64 and writes a diagnostic to stderr. |
| 9 | `size` | The artifact is at most 15 MiB. |
| 10 | `startup` | `forge version` completes within 100 ms. |

### Check 2: architecture

The architecture check reads the executable's header directly. It
does not run the binary. The header is parsed by the Go standard
library:

- Linux: [`debug/elf`](https://pkg.go.dev/debug/elf)
- macOS: [`debug/macho`](https://pkg.go.dev/debug/macho)
- Windows: [`debug/pe`](https://pkg.go.dev/debug/pe)

The check catches the "right name, wrong binary" defect: a build
pipeline that produces `forge-linux-arm64.tar.gz` from an amd64
binary would pass every runtime check on an amd64 host and fail on
a real arm64 host. The header check catches it on any host.

### Check 3: static

The static check asserts the binary has no dynamic dependencies
introduced by the build. The signal differs by platform:

| Platform | Signal |
|---|---|
| Linux | No `PT_INTERP` program header and no `DT_NEEDED` entries. |
| macOS | No `LC_LOAD_DYLIB` load command other than the OS's own libraries. |
| Windows | The PE header parses and reports a machine type. |

#### Linux

On Linux, the check is a strong static-link assertion: a binary
that has a `PT_INTERP` program header or a `DT_NEEDED` entry links
against a shared library the loader must resolve at startup. A
`CGO_ENABLED=0` Go binary has neither; a `CGO_ENABLED=1` binary
links against the host's libc.

#### macOS

On macOS, every executable imports `libSystem` from the loader, and
most import `libresolv` (Go's DNS resolver uses the `resolv` API on
macOS). Both live under `/usr/lib/`, which is the platform's own
library directory. A Homebrew library lives under `/opt/homebrew/`;
a user-installed library lives under `/usr/local/`.

The check asserts "no dylibs outside the operating system's
directories". A binary that imports only from `/usr/lib/` and
`/System/Library/` passes; a binary that imports from
`/opt/homebrew/`, `/usr/local/`, or an `@rpath`-relative path
fails.

##### What the macOS check does not catch

The check does not catch a CGO-enabled macOS binary. Such a binary
imports `/usr/lib/libc++.1.dylib`, which lives under `/usr/lib/`
and is accepted by the path predicate. Catching CGO would require a
signal the Mach-O header does not expose through
`debug/macho`. The `CGO_ENABLED=0` property is enforced by the
build pipeline's `checkCGOEnabled` in
[`scripts/build.go`](../../scripts/build.go) before the binary is
produced.

#### Windows

On Windows, every binary imports from the OS. The check is a
structural assertion: it confirms the PE header parses and reports
a machine type. The underlying property (`CGO_ENABLED=0`) is
enforced by the build pipeline's `checkCGOEnabled` before the
binary is produced.

The PE format's import directory is not exposed by `debug/pe`; a
full DLL-name check would require parsing the import directory
manually. The verifier's check is deliberately lenient; the build
pipeline is the primary guard.

### Check 8: unknown-command exit

The unknown-command check asserts two properties:

1. The exit code is `64` (`ExitUsage`).
2. The diagnostic is on stderr; stdout is empty.

A successful invocation of `forge version` writes to stdout and
nothing to stderr. An unknown-command failure writes to stderr and
nothing to stdout. The check pins both sides of the stream boundary
for the error path.

### Check 9: size

The size check measures the **packaged artifact** (the `.tar.gz` or
`.zip`), not the extracted binary. The packaged artifact is what a
user downloads; budgeting it is what the acceptance criterion asks
for. The packaging adds a small overhead (the gzip and tar headers,
or the zip central directory); budgeting the packaged form catches
an unexpectedly large resource that the archive format could
compress or fail to compress.

### Check 10: startup

The startup check measures the wall-clock time from
`exec.Cmd.Start` to the process's exit. It is not a CPU-time
measurement and does not exclude scheduling latency. A user's
perception of "startup time" is wall-clock; a CPU-time measurement
would miss a regression that adds a blocking operation on a slow
device.

The check shares its measurement with check 4 (`version`). The
verifier runs `forge version` once; the version check reads its
output, and the startup check reads its duration. Running the
binary twice would double the suite's runtime for no new
information.

## Runnable vs. inspect-only

A binary built for a platform different from the host cannot be
executed. The verifier classifies each platform:

- **Runnable** — the platform's `GOOS`/`GOARCH` match the host's.
  The verifier runs all ten checks.
- **Inspect-only** — the platform differs from the host. The
  verifier runs checks 1, 2, 3, and 9 (which parse the artifact's
  bytes) and skips the runnable checks 4–8 and 10.

On a Linux x86-64 CI runner, `linux/amd64` is runnable and
`linux/arm64`, `darwin/*`, and `windows/*` are inspect-only. A CI
matrix that runs the verifier on each platform's own runner
executes all ten checks for all platforms.

The classification appears in the report so a reader knows which
checks were skipped:

```text
linux-amd64 (linux/amd64, tier 1, runnable)
  OK   exists          forge-linux-amd64.tar.gz (2085607 bytes)
  OK   size            2085607 bytes (budget 15728640)
  OK   arch            linux/amd64
  OK   static          static
  OK   version         "forge version v1.4.2 (commit abc1234, built 2026-06-01T12:00:00Z)"
  OK   version-json    valid JSON, five fields
  OK   help-exit0      exit 0, help printed
  OK   version-exit0   exit 0, version printed
  OK   unknown-exit    exit 64, diagnostic on stderr
  OK   startup         18.432ms (budget 100ms)

linux-arm64 (linux/arm64, tier 1, inspect-only)
  OK   exists          forge-linux-arm64.tar.gz (1900467 bytes)
  OK   size            1900467 bytes (budget 15728640)
  OK   arch            linux/arm64
  OK   static          static
  OK   version         SKIP: not runnable on host
  ...
```

## Thresholds

Two budgets are enforced:

| Budget | Value | Rationale |
|---|---|---|
| Binary size | 15 MiB | A released Forge binary is currently around 8 MiB. The budget catches a packaging regression that includes debug data, a vendored resource, or a test fixture. |
| Startup time | 100 ms | A released Forge binary starts in under 20 ms on a typical CI runner. The budget catches a regression that adds a blocking network call, a large file read, or an expensive package-level initialization. |

The thresholds are declared in
[`cmd/forge-internal-verify/main.go`](../../cmd/forge-internal-verify/main.go)
as `maxBinarySize` and `maxStartupDuration` and are documented.

A future release that needs a higher budget amends the ADR and this
document in the same commit. A future release that brings the sizes
down may lower the budget without an ADR; the lowering is a MINOR
change.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every check passed on every selected platform. |
| 1 | At least one check failed, or the verifier could not run. |
| 2 | Usage error. |

The verifier runs every check for every selected platform and
reports the complete picture before exiting. A run that fails one
check still reports the others; a user who reads the report sees
every problem at once.

## CI integration

The `.github/workflows/verify-binaries.yml` workflow runs on every
`v*` tag, on `workflow_dispatch`, and on a weekly schedule. It:

1. Checks out the repository with full history (for
   `SOURCE_DATE_EPOCH` and `git describe`).
2. Builds the artifacts with
   `go run ./scripts/build.go --all --checksums-only`.
3. Runs `go run ./cmd/forge-internal-verify --dist=dist --verbose`.

A failing check fails the workflow and blocks the release.

The workflow's `verify` job is a single job, not a matrix. The
verifier inspects every artifact from any host and classifies each
platform as runnable or inspect-only. A matrix would build the
artifacts six times and would not add coverage.

A summary job named `verify-binaries` provides a stable check name
for branch protection.

## `make release-dry-run`

The Makefile's `release-dry-run` target runs the verifier after
`build-all`, `checksums`, and `sbom`, and before `sign`:

```text
build-all → checksums → sbom → verify-binaries → sign
```

A failure in `verify-binaries` stops the pipeline before signing.
This is deliberate: a signature over a broken binary is worse than
no signature. The signature certifies that the artifact is what the
release engineer intended to ship; if the artifact is broken, the
signature certifies the breakage.

## Local development

A contributor who wants to check the artifacts they just built runs:

```sh
make build-all
go run ./cmd/forge-internal-verify --verbose
```

The `--verbose` flag prints per-check timings. The timings are not a
contract; they are a diagnostic for a check that is unexpectedly
slow (for example, the startup check on a loaded machine).

To verify a single platform:

```sh
go run ./cmd/forge-internal-verify --platform=linux/amd64
```

The `--platform` flag accepts `goos/goarch` or the platform's `name`
field from `platforms.yaml` (`linux-amd64`).

## Failure reference

A failing check produces a line prefixed with `FAIL` and a
one-line message. The message names the check, the expected value,
and the observed value. The reference below maps the message to the
likely cause and the fix.

| Failure | Likely cause | Fix |
|---|---|---|
| `exists: stat ... no such file or directory` | The artifact was not produced. | Run `task release:build-all` or `make build-all` first. |
| `arch: machine = ... want ...` | The build pipeline produced the wrong architecture. | Check `scripts/build.go`'s `GOOS`/`GOARCH` assignment for the platform. |
| `static: PT_INTERP present` | CGO was enabled for this build. | Check `CGO_ENABLED` in the environment and in `scripts/build.go`. |
| `static: DT_NEEDED entries: [...]` | The binary links against a system library. | Same as above; also check for a transitive CGO dependency. |
| `static: non-system dylibs: [...]` | The macOS binary imports a library outside the OS's directories. | Check for a CGO dependency on a Homebrew or user-installed library. |
| `version: run: exit N` | The binary failed to start. | Run the binary by hand to see the failure. |
| `version: first line = "...", want a forge prefix` | The version output changed shape. | Check `internal/app/version/format.go`. |
| `version-json: not valid JSON: ...` | The JSON output is malformed. | Check `internal/version/format_json.go`. |
| `version-json: missing field "..."` | The JSON shape changed. | Check the schema in `schemas/version.schema.json`. |
| `help-exit0: exit: ...` | `forge --help` exited non-zero. | Run `./forge --help` by hand; check the CLI's error path. |
| `unknown-exit: exit = N, want 64` | The unknown-command exit code changed. | Check `internal/cli/mapping.go`. |
| `unknown-exit: stderr is empty` | The error was written to stdout, not stderr. | Check the stream boundary in `internal/cli/`. |
| `size: N bytes > budget M bytes` | The artifact exceeded the budget. | Check whether `-ldflags "-s -w"` was applied; check for a vendored resource. See ADR-047 for the budget's rationale. |
| `startup: N > budget M` | The binary's startup time exceeded the budget. | Profile the binary; check for a blocking network call or a large file read in package initialization. |

## Related documents

| Document | Role |
|---|---|
| [`docs/reference/platforms.yaml`](../reference/platforms.yaml) | The platform matrix the verifier reads |
| [`docs/guides/reproducible-builds.md`](./reproducible-builds.md) | Determinism and the archive naming convention |
| [`docs/guides/building.md`](./building.md) | The build entry points and the preconditions |
| [`docs/reference/version-output.md`](../reference/version-output.md) | The version output contract the verifier checks |
| [`cmd/forge-internal-verify/main.go`](../../cmd/forge-internal-verify/main.go) | The verifier's implementation |
| [`scripts/build.go`](../../scripts/build.go) | The build driver the verifier runs against |
| [`.github/workflows/verify-binaries.yml`](../../.github/workflows/verify-binaries.yml) | The CI job |
