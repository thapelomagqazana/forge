# Approved Dependencies

Every third-party dependency is listed here with its purpose, pinned
version, and upgrade criteria. Adding a new dependency requires an ADR
per the process in.

## github.com/spf13/cobra

| Attribute           | Value                                       |
|---------------------|---------------------------------------------|
| Module path         | `github.com/spf13/cobra`                    |
| Pinned version      | `v1.8.1`                                    |
| License             | Apache-2.0                                  |
| Maintainer          | Steve Francia and contributors              |
| First introduced    | WBS 2.4.1                                   |
| ADR                 | [ADR-006](decisions/ADR-006-cobra-cli-framework.md) |
| Purpose             | CLI command tree, flag parsing, help output |

**Why Cobra:** Forge requires a command-line framework that supports
nested subcommands, POSIX-style flags, automatic help generation, and
shell completion. Cobra provides all four and is the de facto standard
in the Go ecosystem, used by `kubectl`, `gh`, `hugo`, `helm`, and
`docker`. The alternative (`flag` from the standard library) does not
support subcommands. See ADR-006 for the full comparison.

**Transitive dependencies:**

- `github.com/spf13/pflag v1.0.5` — POSIX/GNU-style flags. Cobra's
  flag parser. Same maintainer.
- `github.com/inconshreveable/mousetrap v1.1.0` — Windows-only
  detection of double-click launches. Pulled in unconditionally but
  only compiled on Windows.

Both transitive dependencies are pinned automatically via `go.sum`
and require no manual maintenance.

**Upgrade criteria:**

- **Patch upgrades** (e.g., `v1.8.1` → `v1.8.2`): permitted if the
  upstream release notes contain only bug fixes and no API changes.
  A short note in the pull request description suffices; no ADR
  required.
- **Minor upgrades** (e.g., `v1.8.1` → `v1.9.0`): permitted if the
  release notes do not indicate breaking changes. A brief ADR is
  required, documenting what changed and why the upgrade is safe.
- **Major upgrades** (e.g., `v1.x` → `v2.x`): require a full ADR
  describing the migration plan, the code changes required, and the
  rollback strategy.

Cobra has not released a major version beyond v1.x. When v2.0 is
released, its upgrade will be treated as a Phase 4-or-later concern,
not Phase 2.

**Upgrade cadence:** Reviewed quarterly, aligned with the general
dependency review cycle described in.
