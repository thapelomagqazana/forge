<!--
  Forge Platform Tier Definitions
  ================================
  Normative. Referenced by docs/reference/supported-platforms.md.
  If this document and supported-platforms.md disagree, this document wins.
-->

# Forge Platform Tiers

A **tier** is a promise about how much verification a platform receives
before a Forge release is published. Tiers are per-platform, not per-arch.

## Tier 1 — Fully Tested

**Promise.** Every pull request and every release tag is built, unit-tested,
and integration-tested on this platform in CI.

**CI requirements.**

- A GitHub Actions runner (hosted or self-hosted) exists for this platform.
- `task test:unit` and `task test:integration` pass on every PR.
- The binary produced by CI is byte-for-byte the binary that ships
  (modulo signing on macOS/Windows).

**Failure handling.** A Tier 1 failure blocks the release.

**Current Tier 1 platforms.**

- `linux/amd64`
- `linux/arm64`
- `darwin/arm64`
- `windows/amd64`

## Tier 2 — Built and Unit-Tested

**Promise.** Every release tag is built and unit-tested on this platform in
CI. Integration tests are run manually before the release and the result is
recorded in the release notes.

**CI requirements.**

- A GitHub Actions runner exists for this platform.
- `task test:unit` passes on every release tag.
- `task test:integration` is run by the release engineer and the output is
  attached to the release PR.

**Failure handling.** A Tier 2 failure blocks the release until either the
platform is fixed or it is demoted to Tier 3 with an ADR.

**Current Tier 2 platforms.**

- `darwin/amd64`
- `windows/arm64`

## Tier 3 — Best-Effort

**Promise.** No CI. The platform may be built on request via
`task release:tier3 GOOS=<goos> GOARCH=<goarch>`. No SLA. No signing. No
distribution channel.

**CI requirements.** None.

**Failure handling.** A Tier 3 failure is not a release blocker.

**Current Tier 3 platforms.** None.

## Tier Transitions

| From → To | Change class | Requirements |
|---|---|---|
| None → 3 | Minor | Add to `platforms.yaml` and `supported-platforms.md` |
| 3 → 2 | Minor | Add a CI runner; document the manual integration procedure |
| 2 → 1 | Minor | Add integration tests to CI; ensure the runner is available on PRs |
| 1 → 2 | Major | Deprecation notice in a prior minor release |
| 2 → 3 | Major | Deprecation notice in a prior minor release |
| 3 → None | Major | Deprecation notice in a prior minor release |

## Tier and Artifact Naming

Artifacts are named `<name>-<version>-<goos>-<goarch>[.exe|.tar.gz|.zip]`.
The tier does not appear in the artifact name. The tier is published in
`SHA256SUMS` metadata and in `platforms.yaml`.
