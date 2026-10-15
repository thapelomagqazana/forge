<!--
  Building Forge
  ==============
  This guide is the contract for building Forge. It documents every
  supported build entry point, what each one does, and which invariants
  each one enforces.

  Normative references:
    - Platform matrix:   docs/reference/platforms.yaml
    - Versioning:        docs/reference/versioning.md
    - Release channels:  docs/reference/release-channels.md
    - ADR:               docs/decisions/ADR-044-makefile-and-cross-compile.md

  Editing rules:
    1. Every target listed here MUST exist in the Makefile or Taskfile.
    2. Every invariant stated here MUST be enforced by a test or a
       precondition. Prose alone is not a contract.
    3. If this file and the Makefile disagree, the Makefile wins and
       this file is corrected in the same commit.
-->

# Building Forge

This guide is the contract for building Forge. It documents every
supported build entry point, what each one does, and which invariants
each one enforces.

For the version string semantics, see
[`docs/reference/versioning.md`](../reference/versioning.md).

## Prerequisites

| Tool | Version | Purpose | Required for |
|---|---|---|---|
| Go | the version in `go.mod`'s `toolchain` directive | Compiling Forge | Every target |
| GNU Make or BSD Make | any recent version | Running the Makefile | Every target invoked as `make ...` |
| Task | v3.x | The development workflow | Development targets that delegate to `task` |
| cosign | v2.x | Signing release artifacts | `make sign`, `make release` |
| `gh` | v2.x | Publishing a GitHub Release | `make release` |
| `git` | any recent version | Version metadata (`git describe`, `git rev-parse`) | Every target that injects metadata |

Development targets require only Go and Make. Release targets require
cosign and `gh`; they fail with a legible error if either is missing.

`go.mod`'s `toolchain` directive is authoritative for the Go version.
The Makefile's `toolchain` target prints the version that will be
used; run it first if a build fails with an unexpected compiler error.

## The Makefile is a shim

Forge's `Makefile` is a **compatibility shim**. Development targets
delegate to `Taskfile.yml`; release targets delegate to
`scripts/build.go`. No build logic lives in the Makefile.

Specifically, the Makefile contains no `-X` linker flags, no `MODULE`
variable, and no `git` command in code. This is enforced by
`verify:injection:single-authority` in `Taskfile.yml` and by
`TestMakefile_IsZeroLogicShim` in `scripts/build_test.go`.

If `task` is not installed and you only need a development build:

```sh
go build ./cmd/forge
```

The Makefile's `build` target checks for `task` and prints the same
suggestion if it is missing.

## Development targets

Development targets are fast, host-only, and unsigned. They work on a
fresh clone with no tags and no signing credentials. They may read the
working tree; they must not mutate it.

| Target | Delegates to | Purpose |
|---|---|---|
| `make build` | `task build` | Build the host binary with injected metadata. Reproducible (`-trimpath`). |
| `make build-debug` | `task build:debug` | Build the host binary without `-trimpath`. **Not reproducible. Do not ship.** |
| `make run` | `make build && ./forge --help` | Build then run. |
| `make test` | `task test` | Unit tests with the race detector. |
| `make test-integration` | `task test:integration` | Process-boundary tests. |
| `make check` | `task check` | Full quality gate: format, lint, vet, version matrix, reproducibility, resolution, cobra, dependency policy, two-boundary, handler-boundary, injection, test, integration. |
| `make fmt` | `task fmt` | Format Go source. |
| `make fmt-check` | `task fmt:check` | Verify Go source is formatted. |
| `make vet` | `task vet` | Static analysis. |
| `make lint` | `task lint` | Markdown and configuration linting. |
| `make clean` | `task clean` + inline | Remove `forge`, `forge.exe`, and `dist/`. |

Every development target prints the toolchain version and fails on
`go.mod`/`go.sum` drift before doing its work.

### Reproducible builds

`make build` produces a reproducible binary: two builds of the same
commit on different hosts produce identical outputs. This is achieved by:

- `-trimpath` removes absolute host paths from the binary.
- `-buildvcs=true` embeds the VCS revision, so `go version -m forge`
  reports the commit.
- The four version variables are injected via `-ldflags -X` from
  `Taskfile.yml`. The source is deterministic: `git describe`,
  `git rev-parse`, and `date -u`.

`make build-debug` omits `-trimpath`, so it embeds absolute paths and
is **not reproducible**. It exists for local debugging where source
paths in stack traces are useful. Do not ship a debug binary.

## Release targets

Release targets are cross-platform, version-injected, and (for signed
flows) signed. They require a clean, tagged tree.

| Target | Delegates to | Purpose |
|---|---|---|
| `make build-all` | `scripts/build.go --all` | Cross-compile every Tier 1 and Tier 2 platform. |
| `make build-linux` | `scripts/build.go --os=linux` | Cross-compile linux/amd64 and linux/arm64. |
| `make build-darwin` | `scripts/build.go --os=darwin` | Cross-compile darwin/amd64 and darwin/arm64. |
| `make build-windows` | `scripts/build.go --os=windows` | Cross-compile windows/amd64 and windows/arm64. |
| `make checksums` | `scripts/build.go --checksums-only` | Produce `dist/SHA256SUMS`. |
| `make sign` | `scripts/build.go --sign-only` | Sign artifacts with cosign keyless. |
| `make sbom` | `scripts/build.go --sbom-only` | Produce a CycloneDX SBOM at `dist/forge.sbom.json`. |
| `make release-dry-run` | `build-all && checksums && sbom && sign` | The full pipeline except publishing. |
| `make release` | `release-dry-run && scripts/build.go --publish` | The full pipeline, then upload to the GitHub Release. |

### Preconditions

Every release target depends on four preconditions. They run before
the driver, so a release from a dirty or untagged tree fails before
any binary is produced.

| Precondition | Purpose | Where |
|---|---|---|
| `toolchain` | Prints `go version`. | Makefile (one line) |
| `verify-mod` | Fails on `go.mod`/`go.sum` drift. | `task module:verify:tidy` |
| `verify-cgo` | Fails if `CGO_ENABLED` is not `0`. | Makefile (one line) |
| `verify-tagged` | Fails if `HEAD` is not an exact tag. | `task verify:tagged` |

`verify-tagged` delegates to `task` because the `git` command it runs
must live in a file that `verify:injection:single-authority` permits.
The Makefile is a shim: it contains no `git` commands in code.

### Build flags

Every release build applies:

| Flag | Why |
|---|---|
| `-trimpath` | Removes absolute host paths from the binary. Required for reproducible builds. |
| `-buildvcs=true` | Embeds VCS information, so `go version -m forge` reports the commit. |
| `-ldflags "-s -w"` | Strips the symbol table and DWARF data. Reduces binary size by ~30%. |
| `-ldflags "-X <pkg>.Version=…"` | Injects the version string. |
| `-ldflags "-X <pkg>.Commit=…"` | Injects the commit SHA. |
| `-ldflags "-X <pkg>.BuildDate=…"` | Injects the UTC build timestamp. |
| `-ldflags "-X <pkg>.Dirty=false"` | Marks the binary as built from a clean tree. |
| `CGO_ENABLED=0` | Static linking. The hard rule from `docs/reference/platforms.yaml`. |

The four `-X` flags are built in exactly one place:
`scripts/build.go`'s `buildOne` function. A rename of any variable in
`internal/version/version.go` is a one-line change to `scripts/build.go`.

### Output layout

Every release target writes to `dist/`:

```text
dist/
  forge-vX.Y.Z-linux-amd64.tar.gz
  forge-vX.Y.Z-linux-arm64.tar.gz
  forge-vX.Y.Z-darwin-amd64.tar.gz
  forge-vX.Y.Z-darwin-arm64.tar.gz
  forge-vX.Y.Z-windows-amd64.zip
  forge-vX.Y.Z-windows-arm64.zip
  SHA256SUMS
  forge.sbom.json
  forge-vX.Y.Z-*.tar.gz.sig
  forge-vX.Y.Z-*.tar.gz.pem
```

`dist/` is removed and recreated at the start of every build. A
partial previous run cannot mask a failure.

### Distribution per channel

The channel determines which of the artifacts above are published.

| Channel | Distribution |
|---|---|
| stable | All: direct, Homebrew, Scoop, winget, apt, Docker |
| prerelease | Direct, Docker only |
| nightly | Direct only; unsigned |

### Verifying a release

```sh
# Verify the checksum.
cd dist
sha256sum -c SHA256SUMS

# Verify the signature (requires cosign).
cosign verify-blob \
  --certificate forge-vX.Y.Z-linux-amd64.tar.gz.pem \
  --signature   forge-vX.Y.Z-linux-amd64.tar.gz.sig \
  forge-vX.Y.Z-linux-amd64.tar.gz
```

## Why a Go script instead of shell loops

The Makefile delegates cross-compilation to `scripts/build.go` rather
than a shell loop. The reasons are structural:

1. **Windows compatibility.** `make release` on Windows with `nmake`
   cannot run a `for` loop; a Go program can. The Makefile must work
   on all three platforms without a POSIX shell.

2. **YAML parsing.** The platform matrix lives in
   `docs/reference/platforms.yaml`. A Go program reads it via
   `internal/platform`; a shell script would need `yq` or a
   hand-rolled parser.

3. **Type safety.** The four linker variable names are declarations in
   `internal/version`. A Go program that names them in one place
   cannot typo them across six invocations; a shell script can.

4. **Testability.** `scripts/build_test.go` drives the driver's
   functions directly. A shell script would need a subprocess and a
   temp directory for every test.

The Makefile's `verify:injection:single-authority` target enforces
that the Makefile contains no `-X`, no `MODULE`, and no `git` in
code. Injection logic lives in exactly two places: `Taskfile.yml` (for
development builds) and `scripts/build.go` (for release builds). Both
target the same variable names in `internal/version`; a rename is a
three-file change caught by `task verify:injection`.

## Build metadata injection

The four variables the binary embeds are declared in
`internal/version/version.go`:

```go
var (
    Version   string
    Commit    string
    BuildDate string
    Dirty     string
)
```

They are populated at link time via `-ldflags -X`. There is no other
mechanism: no environment variables, no generated files, no ad-hoc
scripts.

### The silent-failure trap

If the module path in the `-X` target does not match the module
directive in `go.mod`, the linker **silently ignores** the flag. The
binary is produced, but `forge version` reports empty strings. This is
the trap the contract warns about.

The driver catches it in two ways:

1. `checkModulePath` reads `go.mod` and asserts the module path
   matches the driver's `modulePath` constant. A mismatch fails before
   any build runs.
2. `verify:injection:prefix` in `Taskfile.yml` performs the same check
   for development builds.

Both are redundant with `TestAC6_ModulePathMatchesGoMod` in
`internal/version/injection_test.go`, which is the source of truth.
The redundancy is deliberate: catching the trap early is cheaper than
debugging an empty `forge version` output.

### Renaming a linker variable

Renaming any of the four variables is a coordinated change across
three files, all in the same commit:

1. `internal/version/version.go` — the declaration.
2. `Taskfile.yml` — the `LDFLAGS` block.
3. `scripts/build.go` — the four `-X` format strings in `buildOne`.

The `verify:injection:ldflags` task in `Taskfile.yml` detects a
mismatch between (1) and (2); `checkModulePath` and the driver's own
tests detect a mismatch between (1) and (3).

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `FAIL: CGO_ENABLED must be 0` | Your shell has `CGO_ENABLED=1`. | `export CGO_ENABLED=0`. |
| `FAIL: HEAD is not an exact tag` | You are on a commit, not a tag. | `git tag v0.1.0 && git push origin v0.1.0`. |
| `go mod tidy -mod=readonly failed` | `go.mod`/`go.sum` have drifted. | `task tidy`, then commit. |
| `cosign not found on PATH` | You ran `make sign` without cosign. | Install cosign, or run `make release-dry-run` without signing. |
| `gh not found on PATH` | You ran `make release` without `gh`. | Install `gh`, or run `make release-dry-run`. |
| `no platforms selected` | You passed a filter that matched nothing. | Check `--os`/`--arch` values against `docs/reference/platforms.yaml`. |
| `module path mismatch` | `go.mod`'s `module` directive differs from the driver's `modulePath`. | Update both in the same commit. |
| `forge version` reports empty strings | The `-X` target's module path is wrong. | Run `make verify:injection-prefix`; the check names the mismatch. |
| `error: task is not installed` | You ran `make build` without Task. | Install Task from <https://taskfile.dev>, or run `go build ./cmd/forge` directly. |
| `unformatted files` in `make fmt-check` | A Go file is not gofmt-clean. | Run `make fmt` and commit. |

### Diagnosing a cross-compile failure

If `make build-all` fails on one platform, isolate it:

```sh
go run ./scripts/build.go --os=linux --arch=arm64
```

The driver prints `==> <platform>` before each build, so the failing
platform is the last one printed.

If the build fails with `undefined: <symbol>`, the failure is in the
source, not the cross-compile. Run `go build ./...` on the host to
reproduce with better error messages.

If the build fails with `cannot find package`, the module cache is
stale. Run `go clean -modcache` and retry.

### Inspecting a built binary

```sh
# The embedded version metadata.
./forge version --format json

# The VCS revision and the build flags.
go version -m ./forge

# Confirm -trimpath was used: the output has no /home/... paths.
go version -m ./forge | grep -c "/home/"   # should print 0
```

## Related documents

| Document | Role |
|---|---|
| [`docs/reference/platforms.yaml`](../reference/platforms.yaml) | The platform matrix the driver reads |
| [`docs/reference/versioning.md`](../reference/versioning.md) | The version string semantics the driver embeds |
| [`Taskfile.yml`](../../Taskfile.yml) | The development task definitions |
| [`scripts/build.go`](../../scripts/build.go) | The cross-compile driver |
| [`internal/version/version.go`](../../internal/version/version.go) | The four linker variables |
