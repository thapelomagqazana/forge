# Exit Code Ranges

<!--
  Document:    exit-code-ranges.md
  Purpose:     Authoritative allocation of Forge CLI exit codes into functional ranges.
  Audience:    CLI users, script authors, CI/CD integrators, Forge maintainers.
  Stability:   Public contract. Additive changes only.
  Traceability: FR-CLI-EXIT-001, ADR-018
  Last Review: 2026-10-12
-->

## Overview

Forge CLI exit codes are a **public contract**. Scripts, CI/CD pipelines,
IDE integrations, and wrapper tools depend on these values. Once assigned,
a code is **never reassigned** to a different meaning. Codes may be
**deprecated** (documented as such, retained for compatibility) but not
removed or repurposed.

This document defines the **namespace**: which numeric ranges mean what.
Individual code assignments within each range are defined in
`internal/forgeerr/codes.go` and the error code registry.

## Two Failure Domains

Forge distinguishes between two fundamentally different outcomes when a
command does not succeed:

| Domain | Meaning | Exit Range | Caller Action |
|--------|---------|------------|---------------|
| **Forge findings** | Command ran correctly; the *thing being checked* is in a state the caller asked to detect (drift, validation warning, policy violation). | 1–9 | Inspect output; decide whether to remediate. This is **not** a tool malfunction. |
| **Forge execution failure** | The command itself could not complete (bad config, missing template, filesystem error, security denial). | 10–19 | Fix environment, configuration, or inputs; retry. This **is** a tool malfunction from the caller's perspective. |

This distinction is the cornerstone of Forge's exit code design. A CI
pipeline can differentiate "your infrastructure drifted" (exit 1) from
"forge couldn't read its own config" (exit 10) without parsing stderr.

## Range Allocation

| Range | Purpose | Stability | Notes |
|-------|---------|-----------|-------|
| `0` | Success | Frozen | Universal convention. |
| `1–9` | **Forge findings** | Additive | Command succeeded; a finding was reported. |
| `10–19` | **Forge execution failures** | Additive | Config, template, filesystem, security. |
| `20–29` | **Reserved: registry / network / enterprise** | Reserved | Not yet assigned. Phase 2+. |
| `30–63` | **Unassigned** | Reserved | Future use by Forge. |
| `64` | **Usage error** | Frozen | BSD `sysexits.h` `EX_USAGE`. |
| `65–69` | **Unassigned** | Reserved | BSD sysexits sub-range. |
| `70–78` | **Internal error** | Additive | BSD `sysexits.h` `EX_SOFTWARE`–`EX_CONFIG`. |
| `79–127` | **Unassigned** | Reserved | Future use. |
| `128` | **Invalid exit argument** | Frozen | Shell convention. |
| `129` | **SIGHUP** | Frozen | 128 + 1. |
| `130` | **SIGINT** | Frozen | 128 + 2. Ctrl-C. |
| `131–142` | **Signals** | Frozen | 128 + N. |
| `143` | **SIGTERM** | Frozen | 128 + 15. |
| `144–255` | **Signals / reserved** | Frozen | 128 + N. |

### Range 0 — Success

The command completed and no finding was raised. This is the only
"everything is fine" code. Commands that succeed with warnings still
exit 0 unless `--strict` or equivalent is set.

### Range 1–9 — Forge Findings

The command executed correctly. The *subject* of the command is in a
state the user asked to detect. Examples (to be assigned in 11.1.2):

- Drift detected between blueprint and project
- Validation completed with warnings-as-errors
- Check failed (assertion mismatch)

**Semantics:** Exit 1 is the generic "finding" code. Specific codes 2–9
will be assigned to specific finding categories as the product grows.

**Caller guidance:** Do not treat as a crash. Parse stdout/stderr for
details. Consider remediation workflow.

### Range 10–19 — Forge Execution Failures

The command could not complete because Forge itself encountered a
problem. Examples (to be assigned in 11.1.3):

- `10` — Configuration error (parse, schema, precedence)
- `11` — Template error (missing, malformed, render failure)
- `12` — Filesystem error (permission, missing path, I/O)
- `13` — Security error (policy denial, secret exposure)
- `14–19` — Reserved for future execution-failure categories

**Caller guidance:** Treat as a tool failure. Fix environment or inputs.
Retry may succeed after remediation.

### Range 20–29 — Reserved: Registry / Network / Enterprise

Reserved for Phase 2+ features:

- Blueprint registry fetch failures
- Network timeouts
- Enterprise policy server errors

**Not yet assigned.** No code in this range is emitted by current Forge
versions. Callers should not depend on specific values.

### Code 64 — Usage Error

BSD `sysexits.h` `EX_USAGE`. The command was invoked incorrectly:
unknown flag, missing required argument, invalid subcommand.

**Caller guidance:** Fix the invocation. This is a user error, not a
tool error.

### Range 70–78 — Internal Errors

BSD `sysexits.h` range. Indicates a bug in Forge or an unrecoverable
internal state. Examples (to be assigned in 11.1.4):

- `70` — `EX_SOFTWARE` — internal logic error
- `71` — `EX_OSERR` — OS error (fork, pipe, etc.)
- `72` — `EX_OSFILE` — critical OS file missing
- `73` — `EX_CANTCREAT` — cannot create output file
- `74` — `EX_IOERR` — I/O error
- `75` — `EX_TEMPFAIL` — temporary failure, retry
- `76` — `EX_PROTOCOL` — remote protocol error
- `77` — `EX_NOPERM` — permission denied
- `78` — `EX_CONFIG` — configuration error (internal)

**Caller guidance:** Report a bug. Include `--verbose` output.

### Signals: 130, 143, and the 128+N Convention

POSIX shells report a process terminated by signal `N` as exit code
`128 + N`. Forge follows this convention:

- `130` — SIGINT (Ctrl-C)
- `143` — SIGTERM

Forge installs signal handlers that:

1. Cancel the current context.
2. Run cleanup (temp files, locks).
3. Exit with the correct `128 + N` code.

**Caller guidance:** A signal exit means the user or supervisor
requested termination. Not a Forge failure.

## Platform Notes

### Windows

Windows exit codes are 32-bit (`DWORD`). However:

- The C runtime and most shells treat exit codes as unsigned 8-bit.
- Forge **only uses 0–255** to maintain cross-platform stability.
- Forge masks any internal status to `& 0xFF` before exiting.
- Negative values are never emitted.

### Unix / POSIX

- Exit codes are 8-bit unsigned (0–255).
- Values > 255 wrap modulo 256.
- Forge emits only documented values.

### Cross-Platform Guarantee

The same logical outcome produces the same exit code on all platforms.
A drift finding is exit `1` on Linux, macOS, and Windows. A config
error is exit `10` on all three.

## Compatibility Policy

1. **Additive only.** New codes are added within existing ranges or in
   reserved ranges. Existing codes never change meaning.
2. **Deprecation, not removal.** A deprecated code continues to be
   emitted (or documented as legacy) but is superseded by a new code.
3. **No reassignment.** A code freed by deprecation is not reused for
   a different purpose.
4. **Range boundaries are stable.** The purpose of each range does not
   change. New ranges are added at the top of the reserved space.
5. **Documentation is normative.** This file and the error code
   registry are the source of truth. Code comments reference them.

## Machine-Readable Schema

`schemas/exit-codes.schema.json` provides a JSON Schema for tooling
that needs to validate or introspect exit codes. CI systems may use it
to assert that observed codes fall within documented ranges.

## References

- BSD `sysexits.h` — `<sysexits.h>` on FreeBSD, macOS, Linux.
- POSIX.1-2017, §2.8.2 — Exit status.
- `docs/decisions/ADR-018-exit-code-taxonomy.md` — Decision record.
- `internal/forgeerr/codes.go` — Individual code assignments.
- `docs/reference/error-codes.md` — Error code registry.
