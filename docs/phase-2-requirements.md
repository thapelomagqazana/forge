# Phase 2 — Forge Core Foundation: Requirement Register

**Status:** Active
**Owner:** Forge Core Team
**Phase:** 2 — Forge Core Foundation
**Last updated:** <YYYY-MM-DD>

---

## 1. Purpose

This document is the authoritative register of Phase 2 requirements.

Phase 2 is **infrastructure, not feature development**. It establishes the
reliable, cross-platform, testable technical skeleton on which the Blueprint,
Template, Validation, and Update engines will be built in later phases.

A requirement belongs in Phase 2 **only if** it satisfies one of:

1. It establishes a boundary (error, exit code, filesystem, config).
2. It establishes an abstraction used by a future phase.
3. It establishes the build/CI/distribution foundation.
4. It is required to verify another Phase 2 requirement.

Anything that could plausibly belong to Phase 3+ is explicitly rejected in
[Section 9 — Non-Requirements](#9-non-requirements).

---

## 2. ID Convention

| Prefix      | Category                        |
|-------------|---------------------------------|
| `FR-CLI-*`  | Functional — CLI surface        |
| `FR-CFG-*`  | Functional — Configuration      |
| `FR-FS-*`   | Functional — Filesystem         |
| `NFR-*`     | Non-functional                  |
| `ERR-*`     | Error model                     |
| `EXIT-*`    | Exit codes                      |
| `LOG-*`     | Logging                         |
| `SEC-*`     | Security                        |
| `BUILD-*`   | Build & distribution            |
| `CI-*`      | Continuous integration          |
| `TEST-*`    | Testing                         |
| `DOC-*`     | Documentation                   |
| `MKT-*`     | Market / build-in-public        |

Every requirement has a stable ID. IDs are never reused, even if a
requirement is later retired.

---

## 3. Priority Definitions

| Priority | Meaning                                                      |
|----------|--------------------------------------------------------------|
| **P0**   | Required for Phase 2 exit gate. Cannot be deferred.          |
| **P1**   | Required for Phase 2 completeness. Deferrable only with an ADR. |
| **P2**   | Nice-to-have. Should not block Phase 2 exit.                 |

---

## 4. Verification Definitions

| Method | Meaning                                                |
|--------|--------------------------------------------------------|
| **U**  | Unit test                                              |
| **I**  | Integration test (subprocess / real filesystem)        |
| **C**  | CI check (workflow passes on supported matrix)         |
| **M**  | Manual verification documented in a checklist          |
| **D**  | Documentation review                                   |

---

## 5. Functional Requirements — CLI (`FR-CLI-*`)

### FR-CLI-001 — Forge executable starts successfully

- **Priority:** P0
- **Statement:** Running `forge` with no arguments executes without panic and
  exits with a defined exit code.
- **Acceptance:** `forge` runs; exit code is `0` or `2` per CLI UX spec.
- **Verify:** U, I
- **Traces to:** WBS 2.0, 5.0

### FR-CLI-002 — `forge help` displays command information

- **Priority:** P0
- **Statement:** `forge --help` and `forge -h` print root help to stdout.
- **Acceptance:** Output contains `Usage:`, `Available Commands:`, and `Flags:`.
- **Verify:** U, I
- **Traces to:** WBS 5.2, 7.1

### FR-CLI-003 — `forge version` displays version information

- **Priority:** P0
- **Statement:** `forge version` prints version, commit, and build date.
- **Acceptance:** Output matches the frozen version contract in the
  [CLI UX spec](./cli-ux-spec.md#version-output).
- **Verify:** U, I
- **Traces to:** WBS 6.0

### FR-CLI-004 — Unknown command produces a clear error

- **Priority:** P0
- **Statement:** `forge <unknown>` writes a human-readable error to stderr
  and exits with the designated usage exit code.
- **Acceptance:** stderr contains the unknown token and a suggestion;
  stdout is empty; exit code = `2`.
- **Verify:** U, I
- **Traces to:** WBS 7.3, WBS 11.0

### FR-CLI-005 — Command output streams are separated

- **Priority:** P0
- **Statement:** Successful output → stdout. Errors/diagnostics → stderr.
- **Acceptance:** Redirecting stdout to `/dev/null` does not hide errors.
- **Verify:** I
- **Traces to:** WBS 7.4

### FR-CLI-006 — `forge config` displays resolved configuration

- **Priority:** P0
- **Statement:** `forge config` prints the effective configuration.
- **Acceptance:** Output includes config source path, log level, output format.
- **Verify:** U, I
- **Traces to:** WBS 9.0

### FR-CLI-007 — `forge config --path` prints config file path

- **Priority:** P1
- **Statement:** Prints the resolved config file location.
- **Acceptance:** Output is a single line; exit code `0` when found, `3` when
  missing.
- **Verify:** U, I
- **Traces to:** WBS 9.3

### FR-CLI-008 — Global flags have architectural purpose only

- **Priority:** P0
- **Statement:** Only `--verbose`, `--quiet`, and `--config` are added in
  Phase 2. No speculative flags.
- **Acceptance:** `forge --help` lists exactly these global flags.
- **Verify:** I
- **Traces to:** WBS 5.3

### FR-CLI-009 — Command registration is centralised

- **Priority:** P0
- **Statement:** All commands are registered through one mechanism in
  `internal/cli`.
- **Acceptance:** Adding a new command requires editing exactly one file.
- **Verify:** D
- **Traces to:** WBS 4.4

### FR-CLI-010 — Commands are testable without spawning a process

- **Priority:** P0
- **Statement:** Command construction is separated from execution.
- **Acceptance:** `newRootCmd()` returns a `*cobra.Command` that can be
  executed in-process with a custom `io.Writer`.
- **Verify:** U
- **Traces to:** WBS 4.2, 4.3

---

## 6. Functional Requirements — Configuration (`FR-CFG-*`)

### FR-CFG-001 — Configuration can be loaded

- **Priority:** P0
- **Statement:** `internal/config` loads a YAML config file if present.
- **Acceptance:** A valid YAML file produces a populated `Config` struct.
- **Verify:** U
- **Traces to:** WBS 8.4

### FR-CFG-002 — Configuration precedence is deterministic

- **Priority:** P0
- **Statement:** Precedence order is:
  CLI flags → environment → config file → defaults.
- **Acceptance:** A test asserts each layer overrides the one below it.
- **Verify:** U
- **Traces to:** WBS 8.1, 9.4

### FR-CFG-003 — Default configuration is safe

- **Priority:** P0
- **Statement:** Defaults are documented and non-destructive.
- **Acceptance:** No default causes file writes, network access, or
  process execution.
- **Verify:** D, U
- **Traces to:** WBS 8.3

### FR-CFG-004 — Missing config file is not an error

- **Priority:** P0
- **Statement:** Absence of a config file falls back to defaults.
- **Acceptance:** `forge config` succeeds when no config file exists.
- **Verify:** U, I
- **Traces to:** WBS 8.5

### FR-CFG-005 — Malformed YAML produces a structured error

- **Priority:** P0
- **Statement:** Invalid YAML returns a `ForgeError` with code
  `FORGE_CONFIG_INVALID` and the file location.
- **Acceptance:** Error includes line number where possible.
- **Verify:** U
- **Traces to:** WBS 8.5, WBS 10.0

### FR-CFG-006 — Unknown configuration fields are rejected

- **Priority:** P1
- **Statement:** Unknown keys produce a validation error, not silent
  acceptance.
- **Acceptance:** `unknown_field: true` triggers `FORGE_CONFIG_UNKNOWN_FIELD`.
- **Verify:** U
- **Traces to:** WBS 8.5

### FR-CFG-007 — Config struct is deliberately minimal

- **Priority:** P0
- **Statement:** Phase 2 config contains only `LogLevel`, `OutputFormat`,
  `ConfigPath`. No future `forge.yaml` fields.
- **Acceptance:** Struct has exactly these fields.
- **Verify:** D
- **Traces to:** WBS 8.2, Non-Requirements §9

---

## 7. Functional Requirements — Filesystem (`FR-FS-*`)

### FR-FS-001 — Filesystem access is abstracted behind an interface

- **Priority:** P0
- **Statement:** Domain code depends on `internal/filesystem` interface, not
  `os` directly.
- **Acceptance:** No domain package imports `os` for file operations.
- **Verify:** D, U
- **Traces to:** WBS 13.1

### FR-FS-002 — Filesystem interface covers minimum operations

- **Priority:** P0
- **Statement:** Interface exposes `Read`, `Write`, `Exists`, `Mkdir`,
  `Remove`, `List`.
- **Acceptance:** No additional methods added without justification.
- **Verify:** D
- **Traces to:** WBS 13.1

### FR-FS-003 — Path normalisation is cross-platform

- **Priority:** P0
- **Statement:** Paths are normalised before boundary checks.
- **Acceptance:** Windows and POSIX paths resolve consistently.
- **Verify:** U, C
- **Traces to:** WBS 13.3

### FR-FS-004 — Target-directory boundary enforcement is prepared

- **Priority:** P0
- **Statement:** Interface supports a "target root" so future writes cannot
  escape it.
- **Acceptance:** Attempting `../` outside root returns an error.
- **Verify:** U
- **Traces to:** WBS 13.4, SEC-001

### FR-FS-005 — Existing-file handling is explicit

- **Priority:** P0
- **Statement:** Behaviour for `not found`, `exists`, `permission denied`,
  `read-only` is defined and tested.
- **Acceptance:** Each case has a named error.
- **Verify:** U
- **Traces to:** WBS 13.5

### FR-FS-006 — Symlink behaviour is defined

- **Priority:** P1
- **Statement:** Symlinks do not allow boundary escape.
- **Acceptance:** Symlink pointing outside root is rejected.
- **Verify:** U
- **Traces to:** WBS 13.4, SEC-002

---

## 8. Error, Exit, Logging, Security, Build, CI Requirements

### Error Model

#### ERR-001 — All errors are structured

- **Priority:** P0
- **Statement:** Every user-facing error is a `ForgeError` with code,
  message, cause, context, remediation.
- **Verify:** U
- **Traces to:** WBS 10.1

#### ERR-002 — Error categories are defined

- **Priority:** P0
- **Statement:** Categories: `CLI`, `CONFIG`, `FILESYSTEM`, `VALIDATION`,
  `TEMPLATE`, `SECURITY`, `INTERNAL`.
- **Verify:** D, U
- **Traces to:** WBS 10.2

#### ERR-003 — Error wrapping preserves root cause

- **Priority:** P0
- **Statement:** `errors.Is` / `errors.As` work through the chain.
- **Verify:** U
- **Traces to:** WBS 10.3

#### ERR-004 — User-facing error format is consistent

- **Priority:** P0
- **Statement:** Output format:

  ```text
  Error: <message>
  File: <path>
  Reason: <cause>
  Suggestion: <remediation>
  ```

- **Verify:** U, I
- **Traces to:** WBS 10.4

#### ERR-005 — Errors never expose secrets or stack traces by default

- **Priority:** P0
- **Statement:** No stack traces unless `--verbose`. No secret values.
- **Verify:** U
- **Traces to:** WBS 10.5, SEC-003

### Exit Codes

#### EXIT-001 — Exit code constants are defined

- **Priority:** P0
- **Statement:** Constants exist for `0`, `1`, `2`, `3`, `4`, `5`, `6` with
  documented meaning.
- **Verify:** D
- **Traces to:** WBS 11.1

#### EXIT-002 — Errors map deterministically to exit codes

- **Priority:** P0
- **Statement:** `ForgeError` → `ErrorCode` → `ExitCode`.
- **Verify:** U
- **Traces to:** WBS 11.2

#### EXIT-003 — Cobra does not bypass the exit model

- **Priority:** P0
- **Statement:** Process termination happens only in `main` via
  `cli.Execute() int`.
- **Verify:** D, U
- **Traces to:** WBS 11.3

#### EXIT-004 — Every defined exit code is tested

- **Priority:** P0
- **Statement:** Table-driven test asserts each code under its condition.
- **Verify:** U
- **Traces to:** WBS 11.4

### Logging

#### LOG-001 — Logging supports defined levels

- **Priority:** P0
- **Statement:** Levels: `ERROR`, `WARN`, `INFO`, `DEBUG`.
- **Verify:** U
- **Traces to:** WBS 12.1

#### LOG-002 — Default level is sensible

- **Priority:** P0
- **Statement:** Default is `INFO`, not `DEBUG`.
- **Verify:** D
- **Traces to:** WBS 12.2

#### LOG-003 — Logger is abstracted

- **Priority:** P0
- **Statement:** Domain packages use a logger interface, not `log.Println`.
- **Verify:** D
- **Traces to:** WBS 12.3

#### LOG-004 — `--verbose` enables debug output

- **Priority:** P1
- **Statement:** `forge --verbose version` emits debug lines.
- **Verify:** I
- **Traces to:** WBS 12.5

#### LOG-005 — `--quiet` suppresses non-error output

- **Priority:** P1
- **Statement:** Errors still visible.
- **Verify:** I
- **Traces to:** WBS 12.6

#### LOG-006 — Logs never contain secrets

- **Priority:** P0
- **Statement:** Tests confirm config values and env vars are not logged at
  `INFO` or below.
- **Verify:** U
- **Traces to:** WBS 12.7, SEC-003

### Security

#### SEC-001 — No generated path escapes the target directory

- **Priority:** P0
- **Statement:** Filesystem writes are validated against a root boundary.
- **Verify:** U
- **Traces to:** WBS 13.4

#### SEC-002 — Symlink escape is prevented

- **Priority:** P0
- **Statement:** Symlinks cannot redirect writes outside root.
- **Verify:** U
- **Traces to:** WBS 13.6

#### SEC-003 — Secrets are not logged or printed

- **Priority:** P0
- **Statement:** No path prints secret-bearing values.
- **Verify:** U
- **Traces to:** LOG-006, ERR-005

#### SEC-004 — Forge never silently overwrites user files

- **Priority:** P0 (forward-looking invariant)
- **Statement:** The filesystem abstraction refuses overwrites unless
  explicitly authorised. Full enforcement arrives in Phase 4, but the
  interface must support it.
- **Verify:** D, U
- **Traces to:** WBS 13.5

### Build & Distribution

#### BUILD-001 — Local build succeeds

- **Priority:** P0
- **Statement:** `go build ./...` completes with no errors.
- **Verify:** C
- **Traces to:** WBS 2.1

#### BUILD-002 — `make build` produces a runnable binary

- **Priority:** P0
- **Statement:** `make build` yields `./forge` and `./forge version` works.
- **Verify:** M
- **Traces to:** WBS 17.2

#### BUILD-003 — Version metadata is injectable

- **Priority:** P0
- **Statement:** `-ldflags` overrides `Version`, `Commit`, `BuildDate`.
- **Verify:** M
- **Traces to:** WBS 17.4

#### BUILD-004 — Cross-compilation succeeds

- **Priority:** P0
- **Statement:** Builds succeed for the platform matrix:
  Linux/amd64, Linux/arm64, macOS/amd64, macOS/arm64, Windows/amd64.
- **Verify:** C
- **Traces to:** WBS 17.1

#### BUILD-005 — Artifact naming convention is defined

- **Priority:** P0
- **Statement:** `forge-<os>-<arch>[.exe]`.
- **Verify:** D
- **Traces to:** WBS 19.1

#### BUILD-006 — Checksums are generated

- **Priority:** P1
- **Statement:** `SHA256SUMS` produced for release artifacts.
- **Verify:** M
- **Traces to:** WBS 19.3

### CI

#### CI-001 — CI executes automated tests

- **Priority:** P0
- **Statement:** `.github/workflows/ci.yml` runs unit and integration tests.
- **Verify:** C
- **Traces to:** WBS 18.0

#### CI-002 — CI fails on format violations

- **Priority:** P0
- **Statement:** `gofmt -l` non-empty fails the build.
- **Verify:** C
- **Traces to:** WBS 16.2, 18.2

#### CI-003 — CI fails on `go vet` failures

- **Priority:** P0
- **Statement:** `go vet ./...` errors fail the build.
- **Verify:** C
- **Traces to:** WBS 16.1

#### CI-004 — CI matrix covers three operating systems

- **Priority:** P0
- **Statement:** Matrix runs on `ubuntu-latest`, `macos-latest`,
  `windows-latest`.
- **Verify:** C
- **Traces to:** WBS 18.3

#### CI-005 — CI matrix covers two Go versions

- **Priority:** P0
- **Statement:** Matrix covers minimum and current supported Go.
- **Verify:** C
- **Traces to:** WBS 18.4

#### CI-006 — CI detects `go.mod` / `go.sum` drift

- **Priority:** P0
- **Statement:** CI fails if `go mod tidy` produces a diff.
- **Verify:** C
- **Traces to:** WBS 3.3

#### CI-007 — Branch protection readiness is documented

- **Priority:** P1
- **Statement:** `docs/development.md` lists required checks.
- **Verify:** D
- **Traces to:** WBS 18.6

### Testing

#### TEST-001 — Unit tests exist for every core package

- **Priority:** P0
- **Statement:** `internal/cli`, `internal/config`, `internal/filesystem`
  each have `*_test.go`.
- **Verify:** D
- **Traces to:** WBS 15.1

#### TEST-002 — Table-driven tests used where applicable

- **Priority:** P1
- **Statement:** Multi-case tests use table-driven pattern.
- **Verify:** D
- **Traces to:** WBS 15.3

#### TEST-003 — CLI integration tests execute the binary

- **Priority:** P0
- **Statement:** Tests spawn the built binary and assert stdout/stderr/exit
  code for `forge help`, `forge version`, `forge config`, unknown command.
- **Verify:** I
- **Traces to:** WBS 15.4

#### TEST-004 — Filesystem safety tests cover required cases

- **Priority:** P0
- **Statement:** Cases: valid path, nested path, `../`, absolute path,
  existing file, permission failure, symlink escape.
- **Verify:** U
- **Traces to:** WBS 15.5

#### TEST-005 — Coverage baseline established

- **Priority:** P2
- **Statement:** Coverage target documented (e.g., 70% for `internal/`).
  Not enforced as a hard gate in Phase 2.
- **Verify:** D
- **Traces to:** WBS 15.6

### Documentation

#### DOC-001 — README is updated

- **Priority:** P0
- **Statement:** Includes what Forge is, current status, build, run.
- **Verify:** D
- **Traces to:** WBS 20.1

#### DOC-002 — Development guide exists

- **Priority:** P0
- **Statement:** `docs/development.md` covers setup, build, test, vet.
- **Verify:** D
- **Traces to:** WBS 20.2

#### DOC-003 — Architecture guide exists

- **Priority:** P0
- **Statement:** `docs/architecture.md` documents module boundaries.
- **Verify:** D
- **Traces to:** WBS 20.3

#### DOC-004 — Contribution guide exists

- **Priority:** P1
- **Statement:** `CONTRIBUTING.md` documents conventions, PR expectations.
- **Verify:** D
- **Traces to:** WBS 20.4

#### DOC-005 — Dependency policy is documented

- **Priority:** P0
- **Statement:** `docs/dependency-policy.md` defines addition criteria,
  versioning, upgrade policy, stdlib-first principle.
- **Verify:** D
- **Traces to:** WBS 2.5

#### DOC-006 — Phase 2 Definition of Done is published

- **Priority:** P0
- **Statement:** `docs/phase-2-definition-of-done.md` lists measurable
  exit criteria.
- **Verify:** D
- **Traces to:** WBS 1.3

---

## 9. Non-Requirements

The following are explicitly **out of scope** for Phase 2. Adding them
constitutes a scope violation.

| Item                                          | Belongs to  |
|-----------------------------------------------|-------------|
| Blueprint parsing / schema / defaults         | Phase 3     |
| `forge.yaml` full domain model                | Phase 3     |
| Template loading, rendering, resolution       | Phase 4     |
| Component system                              | Phase 12    |
| Policy engine                                 | Phase 8     |
| `forge new` (project generation)              | Phase 4/5   |
| `forge check` / `forge diff` / `forge update` | Phase 7–14  |
| Template registry / publishing                | Phase 16    |
| Team or organisation features                 | Phase 17–18 |
| Interactive wizard                            | Phase 5     |
| AI-assisted generation                        | Not planned |
| Plugin system                                 | Phase 21    |
| Dashboard / web UI                            | Phase 19+   |
| Remote API                                    | Phase 18+   |

The only Phase 3-adjacent thing permitted is:

- `internal/blueprint/` package **directory** may exist with a documented
  interface placeholder.
- No parsing, no schema, no validation logic.

---

## 10. Phase 2 Exit Gate Mapping

For each acceptance gate in the WBS, the responsible requirements are:

| WBS Gate                     | Requirements                                 |
|------------------------------|----------------------------------------------|
| Reliable executable          | FR-CLI-001..010, EXIT-001..004, ERR-001..005 |
| Testable structure           | TEST-001..005, FR-CLI-010                    |
| Cross-platform behaviour     | FR-FS-003, BUILD-004, CI-004                 |
| Safe filesystem boundary     | FR-FS-004..006, SEC-001..004                 |
| Reproducible builds          | BUILD-001..006, CI-006                       |
| Configuration foundation     | FR-CFG-001..007                              |
| Logging foundation           | LOG-001..006                                 |
| Documentation                | DOC-001..006                                 |
| CI enforcement               | CI-001..007                                  |
| Build-in-public readiness    | MKT-001..003 (below)                         |

---

## 11. Market Requirements (`MKT-*`)

Phase 2's market action is **awareness, not acquisition**.

### MKT-001 — Build-in-public announcement is published

- **Priority:** P1
- **Statement:** A short public post explains what Forge is being built to
  become, without asking for adoption.
- **Verify:** D
- **Traces to:** WBS 22.1

### MKT-002 — Changelog of engineering progress exists

- **Priority:** P2
- **Statement:** `docs/changelog.md` records weekly engineering progress.
- **Verify:** D
- **Traces to:** WBS 22.3

### MKT-003 — No premature adoption call-to-action

- **Priority:** P0
- **Statement:** No public surface asks users to install or adopt Forge
  before Phase 5 exit.
- **Verify:** D
- **Traces to:** WBS 22.4

---

## 12. Requirement Traceability Matrix

Every requirement traces to one or more WBS items:

| Requirement      | WBS Items                    |
|------------------|------------------------------|
| FR-CLI-001..010  | 2.0, 4.0, 5.0, 6.0, 7.0, 9.0 |
| FR-CFG-001..007  | 8.0, 9.0                     |
| FR-FS-001..006   | 13.0                         |
| ERR-001..005     | 10.0                         |
| EXIT-001..004    | 11.0                         |
| LOG-001..006     | 12.0                         |
| SEC-001..004     | 13.0, 10.5, 12.7             |
| BUILD-001..006   | 17.0, 19.0                   |
| CI-001..007      | 16.0, 18.0                   |
| TEST-001..005    | 15.0                         |
| DOC-001..006     | 1.3, 2.5, 20.0               |
| MKT-001..003     | 22.0                         |

---

## 13. Change Control

Any new Phase 2 requirement must:

1. Have a unique ID following the convention in §2.
2. Trace to a WBS item or an approved ADR.
3. Include acceptance criteria and a verification method.
4. Be reviewed against §9 (Non-Requirements) to confirm it does not
   introduce Phase 3+ scope.

Any requirement that violates §9 must be rejected or deferred with an ADR.

---

## 14. Summary Counts

| Category  | P0   | P1   | P2  | Total |
|-----------|------|------|-----|-------|
| FR-CLI    | 9    | 1    | 0   | 10    |
| FR-CFG    | 5    | 2    | 0   | 7     |
| FR-FS     | 5    | 1    | 0   | 6     |
| ERR       | 5    | 0    | 0   | 5     |
| EXIT      | 4    | 0    | 0   | 4     |
| LOG       | 5    | 2    | 0   | 7     |
| SEC       | 4    | 0    | 0   | 4     |
| BUILD     | 5    | 1    | 0   | 6     |
| CI        | 6    | 1    | 0   | 7     |
| TEST      | 3    | 1    | 1   | 5     |
| DOC       | 5    | 1    | 0   | 6     |
| MKT       | 1    | 1    | 1   | 3     |
| **Total** | **57** | **11** | **2** | **70** |

---

## 15. Related Documents

- [architecture.md](./architecture.md)
- [cli-ux-spec.md](./cli-ux-spec.md)
- [security-model.md](./security-model.md)
- [dependency-policy.md](./dependency-policy.md)
- [development.md](./development.md)
- [phase-2-definition-of-done.md](./phase-2-definition-of-done.md)

---

## 16. Status Legend

- ☐ Not started
- ◐ In progress
- ✓ Complete
- ✗ Blocked
- ⊘ Retired

*(A separate tracking file — `docs/phase-2-status.md` — will mirror this
register with live status. This register remains the frozen specification.)*
