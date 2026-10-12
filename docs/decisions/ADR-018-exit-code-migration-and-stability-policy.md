# ADR-018: Exit Code Migration and Stability Policy

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

The Forge CLI's exit codes began as a flat, hand-assigned sequence in WBS 4.1.2: `ExitSuccess = 0`, `ExitFailure = 1`, `ExitUsage = 2`, `ExitConfig = 3`, `ExitFilesystem = 4`, `ExitValidation = 5`, `ExitSecurity = 6`, `ExitConflict = 7`. The assignment served the CLI's first commands and was adequate while the CLI had only a handful of failure modes. It stopped being adequate once the CLI's consumers — CI pipelines, IDE extensions, wrapper scripts — needed to distinguish *kinds* of failure from *specific* failures. The flat sequence encoded neither distinction: a config error and a finding were both non-zero, and nothing in the numbers told a consumer whether to remediate the subject or fix the invocation.

WBS 11.1.1 replaced the flat sequence with a range-based taxonomy: findings (1–9), execution failures (10–19), usage (64), internal (70–78), signals (128 + N). The taxonomy is the exit-code system's structural decision, recorded separately. This ADR records the *migration* decision — the transition from the flat assignment to the taxonomy — and the *stability policy* that governs the transition and every change after it.

The transition is not additive. `ExitUsage` moves from 2 to 64. `ExitFailure` (1) is removed and its meaning splits into two successors (`ExitDriftDetected`, `ExitInternal`). `ExitConfig` moves from 3 to 10; `ExitFilesystem` from 4 to 12; `ExitValidation` from 5 to 2; `ExitSecurity` from 6 to 13; `ExitConflict` from 7 to 14. Every one of these changes breaks a consumer that hard-coded the integer. The CLI is pre-1.0 and no consumer is on a release, so the break is permissible now; the policy this ADR establishes is that it will not be permissible later.

The policy matters because exit codes are a *public contract*. Scripts, CI configurations, and IDE integrations observe them as numbers. The numbers are not advisory: a shell that branches on `$? == 2` behaves differently if 2 stops meaning "usage error". Once a code ships in a tagged release, its meaning is fixed. This ADR records the rule and the transition, so that a future contributor who wants to change a code finds the rule and the current transition rather than inventing a policy in isolation.

---

## 2. Options Considered

### Option A — Flat assignment with a stability policy only (no migration)

Keep the WBS 4.1.2 flat assignment (0, 1, 2, 3, 4, 5, 6, 7) and add a rule that the numbers are stable going forward. New codes are appended at the next integer. No range structure.

**Pros:**

- Zero migration. No consumer is broken.
- The existing constants keep their values.
- The policy is the minimum rule the situation requires.

**Cons:**

- The flat sequence does not encode the two failure domains. A consumer cannot tell "the tool failed" from "the subject is in a detectable state" without a lookup table.
- New codes extend the flat sequence indefinitely (8, 9, 10, …), colliding with the sysexits.h ranges that the CLI intends to align with (`64` for `EX_USAGE`, `70–78` for internal errors). The collision is not resolvable without a range structure.
- The reserved space for future features (registry, network, enterprise) has no home in the flat sequence; each new feature's codes would be adjacent to the CLI's findings and would be indistinguishable.
- The problem the CLI's consumers raised (distinguishing kinds of failure) is not solved. The flat sequence is exactly the shape the CLI is trying to leave behind.

### Option B — Range-based taxonomy with a breaking migration and an additive-only stability policy

Adopt the range taxonomy (findings 1–9, execution failures 10–19, usage 64, internal 70–78, signals 128 + N) and move the existing codes into it. Record the migration explicitly. Establish the policy that once a code ships, it is never reassigned; new codes are added within existing ranges or reserved ranges; a code that must change meaning is deprecated and a new code is added.

**Pros:**

- The two failure domains are encoded structurally. A consumer branches on `code < 10` vs. `code >= 10` without a lookup table.
- The usage and internal codes align with BSD sysexits.h (`EX_USAGE = 64`, `EX_SOFTWARE = 70`, `EX_OSERR = 71`, …), which is the convention systems engineers already know.
- Reserved ranges (20–29, 30–63, 65–69, 79–127) give future features a home that does not collide with the CLI's current codes.
- The migration table (below) is a single artifact; a consumer that reads it once can update its hard-coded integers.
- The stability policy is enforceable: the structural tests in `structure_test.go` and the CI check `scripts/deps/verify-exit-codes.sh` fail if a code's value changes.

**Cons:**

- The migration is a breaking change. Every consumer that hard-coded an integer must update.
- The stability policy restricts the CLI's own flexibility: a future change that wants to move a code cannot, because the code is frozen. The change must add a new code and deprecate the old one, which accumulates deprecated codes.
- The range structure imposes a coupling: every new code must find a home in an existing or reserved range, and a code whose natural home is taken must go in a reserved range and be documented as such.

### Option C — Range-based taxonomy without a compatibility break (new codes only, old codes deprecated)

Adopt the range taxonomy for *new* codes, leave the old flat codes at their WBS 4.1.2 values, and mark the old codes deprecated. New commands use the new codes; old commands keep the old ones.

**Pros:**

- No consumer is broken. The old integers keep their meanings.
- The new codes benefit from the range structure.
- The migration is gradual.

**Cons:**

- The exit-code namespace contains two disjoint vocabularies indefinitely: the old flat values (0–7) and the new range values (1–9, 10–19, 64, 70–78, 128 + N). The two overlap numerically: old `ExitUsage = 2` coexists with new `ExitValidation = 2`. A consumer cannot know which vocabulary a given code belongs to without knowing which command emitted it.
- The "two failure domains" distinction is not reliable: a code < 10 might be an old flat code (usage, config) or a new finding. The structural distinction the taxonomy exists to provide is unavailable.
- The deprecated codes must be maintained indefinitely. The CLI cannot remove them without breaking a consumer, and it cannot reassign them without the same.
- The reserved ranges are partially consumed by the old flat codes' "home" ambiguity; a future contributor assigning a new code must check two namespaces for collisions.

### Option D — String-based exit codes

Replace the integer codes with symbolic strings (`EXIT_DRIFT_DETECTED`, `EXIT_CONFIG_ERROR`) and translate them to integers at the process boundary using a fixed mapping.

**Pros:**

- The codes are self-describing. A log line containing `EXIT_DRIFT_DETECTED` does not require a lookup.
- The mapping to integers is one file; the mapping's stability is the integers' stability.
- The symbolic names can be reorganized without changing the integers.

**Cons:**

- POSIX exit codes are integers. Shells, CI systems, and process APIs expect integers. A string-based code requires a translation layer at every boundary, and the translation's output is the integers the CLI already has.
- The translation is a source of drift: a string not in the mapping has no exit code, and a string in the mapping must be kept in sync with the CLI's code constants.
- The benefit (self-describing codes) is already provided by the error codes (WBS 10.3.1), which are strings (`FORGE_CONFIG_MALFORMED`) and are logged alongside the exit code. The exit code's role is to classify the failure for a shell, not to describe it for a log.
- Standard tooling does not support string exit codes. The feature would be unusable from the CLI's own consumers.

---

## 3. Decision

Adopt Option B: the range-based taxonomy with a breaking migration and an additive-only stability policy.

The ranges are: 0 for success; 1–9 for findings; 10–19 for execution failures; 20–29 reserved for registry/network/enterprise; 30–63 reserved for future Forge use; 64 for usage errors; 65–69 reserved for sysexits.h; 70–78 for internal errors; 79–127 reserved for future use; 128 for invalid exit argument; 129–143 for signals (130 = SIGINT, 143 = SIGTERM); 144–255 reserved for signals and high codes. The full table is published in `docs/reference/exit-code-ranges.md`.

The migration is recorded in the table below. Each row names the WBS 4.1.2 constant, its old value, its successor (which may be a new constant with a different name, or the same constant with a new value), and the new value.

| WBS 4.1.2 constant | Old value | Successor | New value |
|--------------------|-----------|-----------|-----------|
| `ExitSuccess` | 0 | `ExitSuccess` | 0 |
| `ExitFailure` | 1 | `ExitDriftDetected` (finding) | 1 |
| `ExitFailure` | 1 | `ExitInternal` (internal error) | 70 |
| `ExitUsage` | 2 | `ExitUsage` | 64 |
| `ExitConfig` | 3 | `ExitConfig` | 10 |
| `ExitFilesystem` | 4 | `ExitFilesystem` | 12 |
| `ExitValidation` | 5 | `ExitValidation` | 2 |
| `ExitSecurity` | 6 | `ExitSecurity` | 13 |
| `ExitConflict` | 7 | `ExitConflict` | 14 |

The policy is: **once an exit code ships in a tagged release, it is frozen. It is never reassigned, never removed, and never renumbered.** A code that must change meaning is deprecated (documented as such in the registry) and a new code is added within the appropriate range. A new code is added at the next free integer in its range; if the range is full, the code goes in a reserved range and the range's stability class is updated.

The policy applies to the CLI's codes as a public contract, not to the CLI's internal constants as implementation details. A future refactor that renames a Go constant is permitted as long as the code's string form (in `docs/reference/exit-codes.md`) and its numeric value are unchanged.

The policy is enforced by: the range table's `init` invariants (`internal/cli/exitcodes.go`), the structural tests (`internal/cli/exitcodes_test.go`, `internal/cli/structure_test.go`), the compatibility test (`internal/cli/exitcodes_compat_test.go`), and the CI check (`scripts/deps/verify-exit-codes.sh`).

---

## 4. Consequences

### 4.1 Positive

- **The two failure domains are structurally encoded.** A consumer branches on `code < 10` vs. `code >= 10` without a lookup table. This is the primary benefit and the reason Option B was chosen over Option A.
- **The usage and internal codes align with BSD sysexits.h.** `EX_USAGE = 64`, `EX_SOFTWARE = 70`, `EX_OSERR = 71`, `EX_OSFILE = 72`, `EX_CANTCREAT = 73`, `EX_IOERR = 74`, `EX_TEMPFAIL = 75`, `EX_PROTOCOL = 76`, `EX_NOPERM = 77`, `EX_CONFIG = 78`. A systems engineer who has written sysexits.h-aware code recognizes the values.
- **The reserved ranges give future features a home.** The registry/network/enterprise range (20–29) accommodates Phase 2+'s blueprint registry, network timeouts, and enterprise policy errors without colliding with the CLI's current codes. The future-Forge-use range (30–63) accommodates new top-level categories.
- **The migration is a single artifact.** A consumer that reads the migration table updates its hard-coded integers once. The table is in this ADR and in `docs/reference/exit-code-ranges.md`; the two must agree, and the CI check asserts it.
- **The stability policy is enforceable.** A future change that silently renumbers a code fails the structural tests and the CI check. The failure is loud and the fix is explicit.
- **The constants are `const` in Go.** The compiler enforces that no code is assigned at runtime. The values are compile-time constants; a future code is a `const` addition, not a mutable variable.

### 4.2 Negative

- **The migration is a breaking change.** Every consumer that hard-coded a WBS 4.1.2 integer must update. The change is documented in this ADR and in the commit that introduces it, but the update is the consumer's responsibility. The CLI is pre-1.0, so the break is permissible; a post-1.0 break of the same shape would not be.
- **`ExitUsage`'s move from 2 to 64 is the sharpest edge.** A consumer that branches on `2` expecting a usage error now sees `ExitValidation` (a finding). The two are semantically different: a usage error means "the invocation was wrong"; a validation finding means "the invocation was correct and the subject has an issue". A consumer that does not update its `2` branch misclassifies findings as usage errors. The compatibility test (`TestCompat_ExitUsageMovedFromTwoToSixtyFour`) makes the transition visible.
- **`ExitFailure`'s removal is ambiguous for consumers.** The constant's meaning was "a generic failure". Its successors are `ExitDriftDetected` (a finding) and `ExitInternal` (an internal error). A consumer that used `ExitFailure` for a generic failure must decide which successor matches its use case. The migration table names both; the choice is the consumer's.
- **The reserved ranges restrict the CLI's own flexibility.** A future code whose natural home is in a reserved range must wait for the range's activation. A future code whose natural home is outside any range must go in a reserved range and be documented as such, which is less clear than a code in a purpose-built range.
- **The stability policy accumulates deprecated codes.** A code that must change meaning is deprecated, not removed. Over time, the registry may accumulate deprecated codes that no current command emits, each retained for historical log resolution. The cost is bounded (the codes are strings in a registry), but the registry grows monotonically.
- **The range boundaries are frozen, which constrains future taxonomy work.** A future change that wants to split the findings range (1–9) into two sub-ranges cannot, because the boundary is a public contract. New sub-ranges must be added in the reserved space (30–63 or 79–127), which may not be semantically adjacent to the codes they are meant to group.
- **The migration table's two-source nature is a drift hazard.** The table exists in this ADR and in `docs/reference/exit-code-ranges.md`. The two must agree. The CI check `scripts/deps/verify-exit-codes.sh` asserts the agreement by reading both; a change to one without the other fails the check. The cost of the check is the cost of the two sources; the benefit is that a reader finds the table in whichever document they reach first.

### 4.3 Neutral

- **The Go constants are duplicated across the two packages.** `internal/cli` defines the constants; `internal/forgeerr` defines the categories whose `ExitCode()` methods return the same values. The duplication is deliberate: `internal/forgeerr` must not depend on `internal/cli`. The two definitions are kept in sync by `TestAC4_ExitCodesMatchCLIConstants`. The duplication is a neutral consequence of the package boundary; it could be eliminated by moving the constants into a third package, but that would invert the dependency or add a package for a small vocabulary.
- **The compatibility test file exists for one release cycle.** `internal/cli/exitcodes_compat_test.go` records the migration table and is removed in the first major release after the taxonomy ships. Its presence is a transitional cost; its removal is a deliberate act.
- **The CLI now emits exit codes outside the flat range it started with.** A consumer that assumed "Forge emits 0–7" now sees 64 and 70. The assumption was never a contract; the contract is "0–255", and the CLI has always reserved the right to emit any value in that range.

---

## 5. Related Requirements

- `FR-CLI-EXIT-001` — the range table's publication and invariants. Established by this ADR's companion decision (the taxonomy) and constrained by the stability policy here.
- `FR-CLI-EXIT-002` through `FR-CLI-EXIT-020` — the per-code enumeration. Each code's value is frozen by this ADR's policy.
- `FR-CLI-EXIT-021` — the error-to-code mapping. The mapping's outputs are constrained to the values this ADR freezes.
- `FR-CLI-EXIT-022` — the audit of error paths. The audit's findings are subject to the policy here (no `os.Exit` outside the boundary; no raw `errors.New` in domain code).
- `FR-CLI-EXIT-023` — the single exit boundary. The boundary's return values are the codes this ADR freezes.
- `FR-CLI-EXIT-024` — the stdout/stderr contract per exit code. The contract is a function of the code; the code's value is frozen here.
- `NFR-QUALITY-005` — the CI-enforceable policy. The policy's enforcement mechanisms are named in § 3 and § 4.

> The following requirements must be added to the register when
> WBS 1.4 is complete: `FR-CLI-EXIT-025` (the migration table's
> canonical form), `NFR-QUALITY-006` (the compatibility window's
> duration), and `NFR-QUALITY-007` (the reserved-range activation
> policy).

---

## 6. Related ADRs

- `ADR-019` — Exit Code Semantics. Sibling decision: the severity classification per code. This ADR freezes the numeric values; ADR-019 classifies them. The two are independent: a code's value never changes, and its severity never changes; a change of either requires a new code.
- `ADR-011` — Closed Error Category Taxonomy. The category-to-code prefix rule. The categories are the layer between the error codes and the exit codes; this ADR freezes the exit codes, and the category taxonomy's `ExitCode()` method produces them.
- `ADR-012` — Error Code Registry and Stability Policy. The sibling policy for error codes (the strings). The two policies are parallel: error codes are strings that are added but never renamed; exit codes are integers that are added but never reassigned. A future contributor who changes one should consider whether the other needs the same change.
- `ADR-013` — Error Wrapping Contract. The wrapping rules that determine which error's category produces the exit code. This ADR assumes the wrapping contract; a change to it would change which code is emitted, not the code's value.
- `ADR-014` — User-Facing Error Format. The format that accompanies the exit code on stderr. The format's contract is a function of the code's category; the code's value is frozen here.
- `ADR-016` — Error Redaction Policy. The redaction rules that apply to the error's context. The rules are unchanged by the exit code's value.
- `ADR-018` (this ADR) — cited by the code and the docs that implement the migration and the policy.

---

## 7. Notes

### Open questions deferred by this ADR

- **The compatibility window's duration.** The migration is a breaking change; a consumer that hard-coded a WBS 4.1.2 integer must update. The window in which the CLI documents the migration (this ADR, the compatibility test, the migration table) is not time-bounded here. A future ADR should set a date or a release after which the compatibility test and the migration table are removed. The candidate is the first major release after the taxonomy ships.
- **The reserved-range activation policy.** The reserved ranges (20–29, 30–63, 65–69, 79–127, 144–255) are inactive. A future feature that needs a code in one of them must "activate" the range: change its `Stability` class from `Reserved` to `Additive`, document the first code assigned, and update the range table's `Purpose` and `CallerGuide`. The activation procedure is not documented in this ADR; a future ADR or a WBS item should record it.
- **The reserved-range collision with sysexits.h.** The ranges 64 and 70–78 align with sysexits.h, and 65–69 are reserved for sysexits codes the CLI does not currently use. A future code that wants to use one of 65–69 must decide whether to align with sysexits.h's meaning for the value or to repurpose the range. The decision is deferred; the current state is that 65–69 are reserved and unused.
- **The migration's visibility to consumers.** The migration table is in this ADR and in `docs/reference/exit-code-ranges.md`. A consumer who only reads the CLI's `--help` or its error messages will not see the table. A future WBS item could add a `forge version --format json` field that names the exit-code taxonomy's version, so a consumer can detect the migration programmatically. The mechanism is deferred.

### Follow-up decisions anticipated

- **WBS 11.1.2** (per-code enumeration and severity). This ADR's policy applies to the codes WBS 11.1.2 enumerates; the enumeration does not change any code's value.
- **WBS 11.2.1** (error-to-exit-code mapping). The mapping's outputs are the codes this ADR freezes. The mapping's totality is a property the taxonomy provides; this ADR's policy is orthogonal.
- **WBS 11.3.1** (single exit boundary). The boundary returns the codes this ADR freezes. The boundary's shape is the subject of a separate ADR; the codes' values are this ADR's subject.
- **WBS 11.3.2** (stdout/stderr contract per exit code). The contract is a function of the code; the code's value is frozen here.

### Links to external discussion

- WBS 4.1.2 — the original flat assignment, superseded by this ADR's migration.
- WBS 11.1.1 — the range taxonomy, the sibling decision to this ADR.
- BSD `sysexits.h` — the convention the usage and internal codes align with.
- POSIX.1-2017 § 2.8.2 — the exit status convention the signals codes follow.

---

*End of ADR-018.*
