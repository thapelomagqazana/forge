<!--
  Reproducible Builds
  ===================
  Public contract. Versioned with the Forge release. Additive changes
  only.

  Normative references:
    - Platform matrix:   docs/reference/platforms.yaml
    - Versioning:        docs/reference/versioning.md
    - Release channels:  docs/reference/release-channels.md
    - Building:          docs/guides/building.md
    - ADR:               docs/decisions/ADR-045-reproducibility-posture.md

  Editing rules:
    1. Every claim in this document MUST be verified by a test or by
       the CI workflow named in the "How reproducibility is verified"
       section. Prose alone is not a contract.
    2. Every source of nondeterminism MUST be listed in the
       "Residual nondeterminism" table and in ADR-045, even if it is
       currently neutralized.
    3. If this document and the driver (scripts/build.go) disagree,
       the driver wins and this document is corrected in the same
       commit.
-->

# Reproducible Builds

Forge builds are reproducible: two builds of the same commit, on the
same host, with the same Go toolchain, produce byte-identical archives.
This document states what "reproducible" means for Forge, how the
property is achieved, how it is verified, and what residual
nondeterminism is accepted.

For the build entry points, see [`building.md`](./building.md). For the
release pipeline as a whole, see
[`release-channels.md`](../reference/release-channels.md).

## Definition

A build is **reproducible** when the following holds:

> Given the same source commit, the same Go toolchain version, and the
> same `SOURCE_DATE_EPOCH`, the build produces a byte-identical output
> on any host with the same OS family and architecture.

Forge verifies the strongest form it can verify in CI: two builds on
the same host produce identical output. Cross-host reproducibility
(one build on Linux, one on macOS, identical bytes) is a stronger
claim that requires the Go toolchain itself to be cross-host
deterministic; the Go project aims for this, and Forge relies on it
without asserting it in CI.

## Artifact naming

The artifact naming convention is **frozen**:

```text
forge-<goos>-<goarch>.<ext>
```

| Platform | Artifact |
|---|---|
| linux/amd64 | `forge-linux-amd64.tar.gz` |
| linux/arm64 | `forge-linux-arm64.tar.gz` |
| darwin/amd64 | `forge-darwin-amd64.tar.gz` |
| darwin/arm64 | `forge-darwin-arm64.tar.gz` |
| windows/amd64 | `forge-windows-amd64.zip` |
| windows/arm64 | `forge-windows-arm64.zip` |

The version does **not** appear in the archive name. It is embedded in
the binary via `-X` and readable with `forge version`. The
version-independent name means a package manager can pin the URL and
only the SHA-256 changes between releases.

The inner binary is named `forge` on Unix and `forge.exe` on Windows.

**Changing the naming convention is a MAJOR change**: it breaks every
script that parses the archive name. A new naming convention requires a
new ADR and a deprecation notice in a prior minor release.

The convention is implemented in `archiveName` in
`scripts/build.go`. A second copy lives in
`cmd/forge-internal-repro/main.go`; the two must change together.
`TestArchiveName` in each package asserts the same six names.

## How reproducibility is achieved

Six properties combine to make the build reproducible.

### 1. `-trimpath`

Strips absolute host paths from the binary. Without it, the binary
embeds the module cache path (`/home/user/go/pkg/mod/...` on Linux,
`/Users/user/go/pkg/mod/...` on macOS), which differs per host.

Applied by `buildOne` in `scripts/build.go`.

### 2. `-buildid=` (empty)

Clears the Go build ID. The Go toolchain embeds a build ID in the
binary; the ID is derived from the build inputs but varies across
otherwise-identical invocations in some toolchain versions. Setting
`-buildid=` overrides it with the empty string.

Applied by `buildOne`.

### 3. `CGO_ENABLED=0`

Disables CGO. Without it, the binary links against the host's libc
(glibc on Linux, libSystem on macOS), and the exact version is
recorded in the binary. Two hosts with different glibc versions
produce different binaries.

Enforced by `checkCGOEnabled` in `scripts/build.go` before any build
runs, and by the Makefile's `verify-cgo` precondition.

### 4. `-buildvcs=true` with commit-derived timestamp

Embeds VCS information: the revision and the commit time. The commit
time is read from the commit, not from the wall clock, so it is
deterministic given the same commit.

Applied by `buildOne`.

### 5. `SOURCE_DATE_EPOCH`

The [reproducible-builds standard](https://reproducible-builds.org/specs/source-date-epoch/)
for a deterministic build timestamp. The release pipeline sets it to
the committer timestamp of `HEAD`:

```sh
export SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)
```

The driver reads it and:

- converts it to RFC 3339 UTC and injects it as `BuildDate`;
- passes it to the `go build` subprocess's environment;
- uses it as the archive entry's `ModTime`.

Without `SOURCE_DATE_EPOCH`, the driver derives the same value from
`git log -1 --format=%ct`. If neither is available, the driver fails
rather than falling back to `time.Now()`.

Implemented by `sourceDateEpoch`, `sourceDateUnix`, and
`resolveBuildMetadata` in `scripts/build.go`.

### 6. Fixed archive timestamps

The `.tar.gz` and `.zip` archives embed a `ModTime` per entry. The
driver derives it from `SOURCE_DATE_EPOCH`, not from the binary's
`mtime`. Without this, two builds of the same commit produce archives
with different timestamps.

Implemented by `archiveModTime` and passed to `writeTarGz` and
`writeZip`.

## How reproducibility is verified

Three layers, in order of cost.

### 1. On every pull request — the driver's unit tests

`scripts/build_test.go` and `cmd/forge-internal-repro/main_test.go`
exercise the deterministic helpers directly:

| Test | Asserts |
|---|---|
| `TestArchiveName` | The naming convention has not drifted |
| `TestArchiveModTime` | The archive timestamp is derived from `SOURCE_DATE_EPOCH`, not from the binary's mtime |
| `TestSourceDateEpoch_EnvWins` | `SOURCE_DATE_EPOCH` takes priority over `git log` |
| `TestSourceDateEpoch_RejectsNonInteger` | A malformed `SOURCE_DATE_EPOCH` fails loudly, not silently |
| `TestSourceDateUnix_RoundTrips` | The Unix conversion is exact |
| `TestBuildOne_NoWallClock` | `buildOne` does not call `time.Now()` |
| `TestResolveBuildMetadata_NoWallClock` | `resolveBuildMetadata` does not call `time.Now()` |

A regression that reintroduces `time.Now()` fails a unit test in
milliseconds. The two "no wall clock" tests inspect the source of
`scripts/build.go` directly.

### 2. On every pull request — the reproducibility workflow

`.github/workflows/reproducible.yml` runs
`cmd/forge-internal-repro` for `linux/amd64` on every PR and for all
six platforms on every release tag. The verifier builds each platform
twice and compares SHA-256:

```sh
go run ./cmd/forge-internal-repro --os=linux --arch=amd64 --verbose
```

The verifier:

1. Resolves `SOURCE_DATE_EPOCH` once, from the environment or from
   `git log -1 --format=%ct`.
2. For each requested platform, builds the archive twice into two
   fresh temp directories using
   `go run ./scripts/build.go --os=... --arch=... --dist=<tempdir>`.
3. Computes the SHA-256 of each archive and compares.
4. Exits 0 if every pair matches, 1 otherwise.

A mismatch prints both sums:

```text
FAIL linux-amd64: archives differ:
  build 1: 3f1a...
  build 2: 8b2c...
```

### 3. Weekly — the full matrix

A scheduled workflow runs the verifier for all six platforms every
Monday at 02:00 UTC against `main`. This catches a regression that the
PR-time check missed, such as a toolchain update that changes one
platform's output.

## Residual nondeterminism

The following sources of nondeterminism are accepted and documented:

| Source | Status | Impact |
|---|---|---|
| Go toolchain version | Pinned by `go.mod`'s `toolchain` directive | A toolchain upgrade may change every binary; the toolchain version is recorded in `go version -m` |
| Host OS family | Same-family builds are reproducible; cross-family is not asserted | A Linux binary and a macOS binary never match; each platform's output matches its own rebuilds |
| Go module cache path | Removed by `-trimpath` | None |
| C library version | Removed by `CGO_ENABLED=0` | None |
| Wall-clock time | Removed by `SOURCE_DATE_EPOCH` | None |
| Go build ID | Removed by `-buildid=` | None |
| Archive entry mtime | Fixed to `SOURCE_DATE_EPOCH` | None |
| `-buildvcs` revision | Fixed to the commit's revision | None |
| Build user/group | Not embedded (tar `Uname`/`Gname` empty) | None |
| Filesystem iteration order | Not used; platform order comes from `platforms.yaml` | None |

The only accepted residual nondeterminism is the Go toolchain version.
Two different Go versions may produce different binaries from the same
source; that is by design, and the toolchain version is recorded so the
difference is attributable.

## Verifying a Forge release

```sh
# Verify the checksum.
cd dist
sha256sum -c SHA256SUMS

# Verify the signature (requires cosign).
cosign verify-blob \
  --certificate forge-linux-amd64.tar.gz.pem \
  --signature   forge-linux-amd64.tar.gz.sig \
  forge-linux-amd64.tar.gz

# Confirm the binary is static (Linux only).
tar -xzf forge-linux-amd64.tar.gz
ldd ./forge
# Expected: "not a dynamic executable"

# Confirm the embedded metadata.
./forge version --format json

# Confirm -trimpath: no host paths in the binary.
go version -m ./forge | grep -c "/home/"   # prints 0
```

## Reproducing a Forge release locally

To reproduce a released binary, you need:

- the same commit (check `go version -m ./forge` for the revision);
- the same Go toolchain (check the `toolchain` directive in `go.mod`
  at that commit);
- the same `SOURCE_DATE_EPOCH` (the committer timestamp of the commit).

```sh
git checkout <commit>
export SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)
go run ./scripts/build.go --os=linux --arch=amd64
sha256sum dist/forge-linux-amd64.tar.gz
```

Compare the printed SHA-256 with the release's `SHA256SUMS`. They must
match.

If they do not match, the difference is one of:

| Symptom | Cause |
|---|---|
| The SHA-256 differs, but `forge version` reports the same values | The Go toolchain differs. Check `go version -m ./forge` against the toolchain directive in `go.mod`. |
| `forge version` reports empty strings | The `-X` linker target did not match the module path. Run `go run ./scripts/build.go --os=linux --arch=amd64` and read the error. |
| `ldd` reports a dynamic executable | CGO was enabled. Set `CGO_ENABLED=0` and rebuild. |
| `go version -m` reports `/home/...` paths | `-trimpath` was not applied. The release binary always has it; a locally-built binary might not if built without the driver. |

## Running the verifier in CI

The verifier is the entry point for CI. Add this to a workflow:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0            # required for git log -1 --format=%ct
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
    check-latest: false
    cache: true
- name: Set SOURCE_DATE_EPOCH from commit
  run: echo "SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)" >> "$GITHUB_ENV"
- name: Verify reproducibility
  run: go run ./cmd/forge-internal-repro --os=linux --arch=amd64 --verbose
```

`fetch-depth: 0` is required: a shallow checkout has no commit history,
and `git log -1 --format=%ct` still works on the single commit, but a
future change that derives `SOURCE_DATE_EPOCH` from the tag's commit
would need the full history. Keeping `fetch-depth: 0` is the safe
default.

## Related documents

| Document | Role |
|---|---|
| [`docs/reference/platforms.yaml`](../reference/platforms.yaml) | The platform matrix the driver reads |
| [`docs/reference/versioning.md`](../reference/versioning.md) | The version metadata the driver embeds |
| [`docs/reference/release-channels.md`](../reference/release-channels.md) | The channels and their distribution rules |
| [`docs/guides/building.md`](./building.md) | The build entry points and the preconditions |
| [`scripts/build.go`](../../scripts/build.go) | The cross-compile driver |
| [`scripts/build_test.go`](../../scripts/build_test.go) | The driver's tests |
| [`cmd/forge-internal-repro/main.go`](../../cmd/forge-internal-repro/main.go) | The reproducibility verifier |
| [`cmd/forge-internal-repro/main_test.go`](../../cmd/forge-internal-repro/main_test.go) | The verifier's tests |
| [`.github/workflows/reproducible.yml`](../../.github/workflows/reproducible.yml) | The CI workflow |
| [reproducible-builds.org](https://reproducible-builds.org/) | The standard Forge follows |
