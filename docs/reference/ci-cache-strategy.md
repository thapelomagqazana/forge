# CI Cache Strategy

> **Status:** Accepted
> **Owner:** Release Engineering
> **Related:** [CI Tiers](ci-tiers.md), [CI Architecture](../architecture/ci-architecture.md)

This document describes the caching strategy the CI tier
workflows use. It is rendered from `internal/ci/cache.go`; a
golden-file test asserts the two agree.

## Cache Types

The CI workflows use three caches. Each is a directory the Go
toolchain (or the linter) writes to; `actions/cache` restores the
directory before the tier's steps run and saves it after the run
completes.

| Cache | Paths | Key | Restore key | Hit-rate target |
|---|---|---|---|---|
| module | ~/go/pkg/mod<br>${{ env.HOME }}/go/pkg/mod | `${{ runner.os }}-go-${{ env.CACHE_VERSION }}-${{ hashFiles('**/go.sum') }}` | `${{ runner.os }}-go-${{ env.CACHE_VERSION }}-` | 90% |
| build | ~/.cache/go-build<br>${{ env.HOME }}/.cache/go-build<br>${{ env.LOCALAPPDATA }}/go-build | `${{ runner.os }}-${{ inputs.go-version }}-build-${{ env.CACHE_VERSION }}-${{ hashFiles('**/*.go', '**/go.sum') }}` | `${{ runner.os }}-${{ inputs.go-version }}-build-${{ env.CACHE_VERSION }}-` | 60% |
| lint | ~/go/bin/golangci-lint<br>${{ env.HOME }}/go/bin/golangci-lint | `${{ runner.os }}-golangci-lint-${{ env.CACHE_VERSION }}-${{ env.GOLANGCI_LINT_VERSION }}` | `${{ runner.os }}-golangci-lint-${{ env.CACHE_VERSION }}-` | 90% |

Each cache's note:

- **module:** Stores the extracted sources of every module the repository depends on; keyed on go.sum's hash so that a dependency change invalidates the cache.
- **build:** Stores compiled object files and intermediate artifacts; keyed on the repository's Go sources and go.sum so that a code change invalidates the cache.
- **lint:** Stores the golangci-lint binary; keyed on the pinned linter version so that a version bump invalidates the cache.

## Invalidation

Every cache key includes a `CACHE_VERSION` prefix. To force-clear
every cache, bump the prefix in `.github/workflows/cache.yml`:

```yaml
env:
  CACHE_VERSION: v1  # bump to v2 to invalidate
```

The next CI run sees a cache miss on every key and rebuilds from
scratch. The change is one line and appears in the diff; a reviewer
sees the reason in the commit message.

The current version is **v1**.

## Branch Scope

GitHub Actions scopes caches by branch: a branch reads its ancestors' caches but writes only its own. A PR's first push starts cold; subsequent pushes to the same PR hit the PR's branch's cache. A PR never writes to main's cache.

The consequence: a PR's first push is a miss; subsequent pushes
to the same PR hit the PR's branch's cache. The hit-rate targets
below describe the steady-state rate across all runs, not the
rate for a single PR.

## Cache Safety

The caches store **derived artifacts only**:

- the module cache stores extracted module sources;
- the build cache stores compiled object files;
- the linter cache stores the golangci-lint binary.

None of the three contains a secret. The workflow's `env:` block
declares no secret. The cache keys do not incorporate any secret.

The repository's `go.mod` and `go.sum` are **not** cached: they are
source files, not derived artifacts. A cache restore must never
overwrite them. The workflow caches the directories the Go
toolchain derives from them, not the files themselves.

The `CACHE_VERSION` prefix is the mechanism that recovers from an
accidentally poisoned cache. A cache whose contents are suspect
is not deleted; the version is bumped and the next run rebuilds.

## Hit-Rate Targets

The repository targets the following steady-state hit rates.
They are the targets the strategy document publishes; a future
metric collector (a follow-up task) reads the workflow runs'
`cache-hit` outputs and reports the actual rate.

| Cache | Target |
|---|---|
| module | 90% |
| build | 60% |
| lint | 90% |

The module cache's target is higher than the build cache's because
the module cache changes only when `go.sum` changes, while the
build cache changes whenever any Go source file changes.
