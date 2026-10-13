# Forge Decision Log

This index is auto-generated. Do not edit it manually.

## Accepted

| ID | Title | Date |
|----|-------|------|
| [ADR-021](./ADR-021-json-error-envelope-for-format-json.md) | ADR-021: JSON Error Envelope for `--format json` | 2026-10-12 |
| [ADR-020](./ADR-020-stdout-stderr-contract-per-exit-code.md) | ADR-020: stdout/stderr Contract per Exit Code | 2026-10-12 |
| [ADR-019](./ADR-019-single-exit-boundary-in-main.md) | ADR-019: Single Exit Boundary in main | 2026-10-12 |
| [ADR-018](./ADR-018-exit-code-migration-and-stability-policy.md) | ADR-018: Exit Code Migration and Stability Policy | 2026-10-12 |
| [ADR-017](./ADR-017-error-verbose-modes.md) | ADR-017: Error verbose modes | 2026-10-12 |
| [ADR-016](./ADR-016-error-redaction-policy.md) | ADR-016: Error redaction policy | 2026-10-12 |
| [ADR-015](./ADR-015-programmer-facing-error-format.md) | ADR-015: Programmer-facing error format | 2026-10-12 |
| [ADR-014](./ADR-014-user-facing-error-format.md) | ADR-014: User-facing error format | 2026-10-12 |
| [ADR-013](./ADR-013-error-wrapping-contract.md) | ADR-013: Error wrapping contract | 2026-10-12 |
| [ADR-012](./ADR-012-error-code-registry-and-stability-policy.md) | ADR-012: Error code registry and stability policy | 2026-10-12 |
| [ADR-011](./ADR-011-closed-error-category-taxonomy.md) | ADR-011: Closed Error Category Taxonomy | 2026-10-12 |
| [ADR-002](./ADR-002-choose-cobra-as-the-cli-framework.md) | ADR-002: Choose Cobra as the CLI framework | 2026-10-09 |

## Proposed

| ID | Title | Date |
|----|-------|------|
| [ADR-034](./ADR-034-the-redaction-policy-is-closed-no-log-field-is-exempt-and-every-mechanism-has-a-test.md) | ADR-034: 'The redaction policy is closed: no log field is exempt, and every mechanism has a test' | 2026-10-13 |
| [ADR-033](./ADR-033-sensitive-data-is-redacted-at-three-layers-type-pattern-and-config.md) | ADR-033: 'Sensitive data is redacted at three layers: type, pattern, and config' | 2026-10-13 |
| [ADR-032](./ADR-032-quiet-and-verbose-are-mutually-exclusive.md) | ADR-032: '--quiet and --verbose are mutually exclusive' | 2026-10-13 |
| [ADR-031](./ADR-031-quiet-suppresses-diagnostics-but-never-suppresses-errors.md) | ADR-031: '--quiet suppresses diagnostics but never suppresses errors' | 2026-10-13 |
| [ADR-030](./ADR-030-verbose-is-long-only-v-is-reserved-for-version.md) | ADR-030: '--verbose is long-only; -v is reserved for --version' | 2026-10-13 |
| [ADR-029](./ADR-029-broken-pipes-exit-cleanly-with-code-141-unix-141-windows-equivalent-and-never-print-a-stack-trace.md) | ADR-029: 'Broken pipes exit cleanly with code 141 Unix 141 Windows-equivalent and never print a stack trace' | 2026-10-13 |
| [ADR-028](./ADR-028-the-stream-contract-is-enforced-at-the-boundary-output-stdout-logger-stderr-errors-stderr.md) | ADR-028: 'The stream contract is enforced at the boundary: output→stdout, logger→stderr, errors→stderr' | 2026-10-13 |
| [ADR-027](./ADR-027-domain-packages-emit-diagnostics-only-through-the-logger-interface.md) | ADR-027: 'Domain packages emit diagnostics only through the Logger interface' | 2026-10-13 |
| [ADR-026](./ADR-026-the-logger-interface-has-four-level-methods-and-a-with-combinator-the-implementation-is-slog-backed-and-hidden.md) | ADR-026: 'The Logger interface has four level methods and a With combinator; the implementation is slog-backed and hidden' | 2026-10-13 |
| [ADR-025](./ADR-025-log-records-are-structured-with-a-schema-versioned-json-form-and-an-unstable-human-form.md) | ADR-025: 'Log records are structured with a schema-versioned JSON form and an unstable human form' | 2026-10-13 |
| [ADR-024](./ADR-024-log-level-precedence-cli-flag-env-var-config-file-default.md) | ADR-024: 'Log level precedence: CLI flag > env var > config file > default' | 2026-10-13 |
| [ADR-023](./ADR-023-four-log-levels-error-warn-info-debug-with-operational-warn-distinct-from-policy-warn.md) | ADR-023: 'Four log levels: ERROR, WARN, INFO, DEBUG — with operational WARN distinct from policy WARN' | 2026-10-13 |
| [ADR-022](./ADR-022-output-and-logging-are-separate-architectural-layers.md) | ADR-022: 'Output and logging are separate architectural layers' | 2026-10-13 |
| [ADR-010](./ADR-010-forge-config-interaction-with-the-global-flags.md) | ADR-010: `forge config` Interaction with the Global Flags | 2026-10-12 |
| [ADR-009](./ADR-009-forge-config-output-format.md) | ADR-009: `forge config` Output Format | 2026-10-12 |
| [ADR-008](./ADR-008-forge-config-command-scope.md) | ADR-008: `forge config` Command Scope | 2026-10-12 |
| [ADR-007](./ADR-007-configuration-foundation-precedence-discovery-and-error-taxonomy.md) | ADR-007: Configuration Foundation — Precedence, Discovery, and Error Taxonomy | 2026-10-11 |
| [ADR-006](./ADR-006-help-and-cli-ux-contracts-invocation-matrix-output-formats-stream-boundary-and-phase-2-presentation-policy.md) | ADR-006: Help and CLI UX Contracts — Invocation Matrix, Output Formats, Stream Boundary, and Phase 2 Presentation Policy | 2026-10-11 |
| [ADR-005](./ADR-005-version-command-structure-output-formats-and-stability-guarantees.md) | ADR-005: Version Command Structure, Output Formats, and Stability Guarantees | 2026-10-10 |
| [ADR-004](./ADR-004-root-command-identity-and-behavior-contracts.md) | ADR-004: Root Command Identity and Behavior Contracts | 2026-10-10 |
| [ADR-003](./ADR-003-cli-framework-foundation-package-execution-injection-registry-and-contract.md) | ADR-003: CLI Framework Foundation — Package, Execution, Injection, Registry, and Contract | 2026-10-10 |
| [ADR-001](./ADR-001-forge-is-a-foundation-manager.md) | ADR-001: Forge is a Foundation Manager | 2026-10-09 |

## Superseded

| ID | Title | Date |
|----|-------|------|

## Deprecated

| ID | Title | Date |
|----|-------|------|
