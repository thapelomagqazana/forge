# Log Levels

<!--
  Document:     log-levels.md
  Purpose:      Normative semantics for Forge's four log levels, the
                precedence rules that select a level, and the flag/env
                mapping that resolves to one. Public contract for
                operators, CI authors, and log aggregators.
  Audience:     CLI users, script authors, CI/CD integrators, operators,
                log aggregators, monitoring systems.
  Stability:    Public contract. Additive only.
  Traceability: FR-CLI-LOG-002, FR-CLI-LOG-003, ADR-022, WBS 12.1.2
  See also:     ../architecture/output-vs-logging.md
                output-streams.md
  Last Review:  2026-10-12
-->

## Overview

Forge logs to **stderr** at one of four levels. The level is selected by
a precedence chain that resolves to exactly one value per invocation.
This document defines:

1. The four levels and their exact semantics.
2. The distinction between **operational WARN** (a log level) and
   **policy WARN** (a finding severity in `forge check` output).
3. The precedence chain: CLI flag > env var > config file > default.
4. The flag/env mapping: `--quiet`, `--verbose`, `--log-level`,
   `FORGE_LOG_LEVEL`.
5. The mutual-exclusion rule for `--quiet` and `--verbose`.

It is normative. Where code and this document disagree, the document
wins and the code must be corrected.

---

## 1. The four levels

Levels are ordered, least severe to most severe:

```text
DEBUG < INFO < WARN < ERROR
```

Selecting a level enables that level **and every higher level**. For
example, `--log-level=warn` enables WARN and ERROR, suppresses DEBUG and
INFO.

| Level | Name | Stream | Default visibility | Purpose |
|-------|------|--------|--------------------|---------|
| `DEBUG` | Debug | stderr | Off | Internal state for troubleshooting. |
| `INFO` | Info | stderr | On | Progress milestones and lifecycle events. |
| `WARN` | Warn | stderr | On | Recoverable anomalies worth attention. |
| `ERROR` | Error | stderr | Always on | Forge could not complete an operation. |

### 1.1 DEBUG

**Semantics:** internal state for troubleshooting. Values resolved,
branches taken, caches hit or missed, timing measurements, retries.

**When to emit:** the code has information that would help a developer
diagnose a bug, but that is not useful to a normal user.

**When NOT to emit:** for anything the user needs to understand a
successful run.

**Examples:**

- `debug: resolved version from build info` (with `version=1.2.3`)
- `debug: config discovered at /etc/forge/config.yaml`
- `debug: cache miss for key=blueprint:go-cli`

**Default:** off. Enabled only by `--verbose`, `--log-level=debug`, or
`FORGE_LOG_LEVEL=debug`.

### 1.2 INFO

**Semantics:** progress milestones and lifecycle events. Things a user
would want to know happened during a successful run, without being
diagnostic noise.

**When to emit:** the user's mental model of "what the command did"
would be incomplete without the line. Starting, finishing, loading,
writing, connecting.

**When NOT to emit:** for anything on the hot path (per-file, per-byte)
or that is only useful for debugging.

**Examples:**

- `info: loading config from /etc/forge/config.yaml`
- `info: generated 3 blueprints`
- `info: updated dependency github.com/foo/bar to v1.2.4`

**Default:** on. Suppressed by `--quiet`, `--log-level=warn|error`, or
`FORGE_LOG_LEVEL=warn|error`.

### 1.3 WARN (operational)

**Semantics:** a recoverable anomaly worth the user's attention. The
operation succeeded, but something was not ideal — a fallback was used,
a deprecated form was accepted, a value was coerced, a retry succeeded.

**When to emit:** the user should probably act, but the command
succeeded (exit 0) or continued past the anomaly.

**When NOT to emit:** for expected fallbacks that are not worth the
user's attention (use INFO); for failures (use ERROR).

**Examples:**

- `warn: config file not found, using defaults` (with `path=...`)
- `warn: blueprint field 'language' is deprecated, use 'runtime'`
- `warn: retrying request after transient failure` (with `attempt=1`)

**Default:** on. Suppressed only by `--log-level=error` or
`FORGE_LOG_LEVEL=error`.

> **⚠ Operational WARN ≠ policy WARN.**
> `forge check` and `forge validate` produce *findings* with severities
> (`INFO`, `WARN`, `ERROR`) that are **product output**, printed to
> **stdout**. Those are policy findings, not log records. A policy WARN
> is not a logger call; it does not appear on stderr unless the operator
> also logs it. The two share a vocabulary but not a stream, a package,
> or a contract. See §3.

### 1.4 ERROR

**Semantics:** Forge could not complete an operation. The command failed
or a sub-operation failed and was surfaced.

**When to emit:** an error is about to be returned, or an error was
handled and its occurrence must be recorded.

**When NOT to emit:** for warnings, fallbacks, or anything the user
should treat as success.

**Examples:**

- `error: failed to write blueprint` (with `path=...`, `err=...`)
- `error: config validation failed` (with `code=FORGE_CONFIG_INVALID`)

**Default:** always on. Cannot be suppressed by any flag, env var, or
config value. The only way to silence an ERROR log is to not produce
one, which the code should do only when the error is expected and
handled.

> **Note:** the ERROR *log record* is distinct from the ERROR *error
> envelope*. The envelope (ADR-021) is product output on stdout under
> `--format json`; the log record is a diagnostic on stderr. A command
> may emit both: an error envelope on stdout (so a machine consumer can
> parse the failure) and an ERROR log record on stderr (so an operator
> sees it in logs). This is correct and expected. See
> [`output-streams.md`](output-streams.md).

---

## 2. The precedence chain

When multiple sources specify a log level, they are resolved in this
order (highest precedence first):

```text
1. --log-level=<level>   (explicit CLI flag, highest)
2. --verbose             (CLI flag, implies DEBUG)
3. --quiet               (CLI flag, implies ERROR)
4. FORGE_LOG_LEVEL       (environment variable)
5. log.level             (config file)
6. (default)             (INFO)
```

Once a source provides a value, lower-precedence sources are ignored
for that invocation. This matches the CLI's general precedence model
(ADR-007) and Twelve-Factor App §III (config in the environment).

### 2.1 Explicit flag beats implicating flags

`--log-level=<level>` wins over `--verbose` and `--quiet`. This allows
a user to keep a `--verbose` in a shell alias while overriding the level
for one invocation:

```bash
alias forge='forge --verbose'           # DEBUG by default
forge --log-level=error version         # ERROR for this call
```

### 2.2 `--verbose` beats `--quiet`

If both are passed, `--verbose` wins. But see §4: passing both is an
error. The rule is stated here so the resolver is total, not because
users should rely on it.

### 2.3 Env var beats config file

`FORGE_LOG_LEVEL` (from the environment) beats `log.level` (from the
config file). This is the standard Twelve-Factor rule: environment
overrides committed defaults, so that a container can be reconfigured
without editing files.

### 2.4 Precedence table

| Source | Value | Effective level | Notes |
|--------|-------|-----------------|-------|
| `--log-level=debug` | explicit | DEBUG | Highest precedence. |
| `--log-level=warn` + `--verbose` | explicit wins | WARN | Explicit beats implicating. |
| `--verbose` | implicating | DEBUG | |
| `--quiet` | implicating | ERROR | |
| `FORGE_LOG_LEVEL=debug` + config `warn` | env wins | DEBUG | |
| `FORGE_LOG_LEVEL=warn` + default | env wins | WARN | |
| config `log.level: debug` + default | config wins | DEBUG | |
| (nothing set) | default | INFO | |

---

## 3. Operational WARN vs. policy WARN

The word "WARN" appears in two unrelated contracts. This table makes the
distinction unambiguous.

| Aspect | Operational WARN | Policy WARN |
|--------|------------------|-------------|
| **Package** | `internal/logger` | `internal/findings` (per-command) |
| **Stream** | stderr | stdout |
| **Format** | governed by `--log-format` | governed by `--format` |
| **Selected by** | `--log-level` | command flags |
| **Suppressible by** | `--log-level=error`, `--quiet` | command-specific |
| **Affects exit code** | No | Yes (per findings policy) |
| **Part of product output** | No | Yes |
| **Example** | `warn: config file not found, using defaults` | `WARN: blueprint 'go-cli' uses deprecated field 'language'` |
| **Audience** | Operator, developer | End user, CI consumer |

A command may emit both: an operational WARN on stderr and a policy
WARN on stdout. They are not the same finding, not the same channel, and
not correlated unless the command's logic makes them so.

> **Rule for contributors:** if you are writing a `logger.Warn(...)`
> call, you are producing a diagnostic. If you are producing a finding
> for `forge check` output, you are producing product output; use the
> findings package, not the logger.

---

## 4. Flag and env mapping

### 4.1 Mapping table

| Input | Effective level | Notes |
|-------|-----------------|-------|
| (nothing set) | INFO | Default. |
| `--quiet` | ERROR | Suppresses diagnostics; see §4.3. |
| `--verbose` | DEBUG | Enables full diagnostics. |
| `--log-level=debug` | DEBUG | Explicit override. |
| `--log-level=info` | INFO | Explicit override. |
| `--log-level=warn` | WARN | Explicit override. |
| `--log-level=error` | ERROR | Explicit override. |
| `FORGE_LOG_LEVEL=debug` | DEBUG | Used only if no CLI flag. |
| `FORGE_LOG_LEVEL=warn` | WARN | Used only if no CLI flag. |
| config `log.level: debug` | DEBUG | Used only if no env var or flag. |

### 4.2 Accepted values

`--log-level` and `FORGE_LOG_LEVEL` accept:

- `debug`
- `info`
- `warn` (alias: `warning`)
- `error`

Case-insensitive. Surrounding whitespace is trimmed. Empty string is
treated as unset (falls through to the next source).

Any other value is a **usage error** (exit 64): the CLI rejects it with
a clear message before any work begins.

### 4.3 `--quiet` semantics

`--quiet` sets the effective level to ERROR. It also suppresses:

- The human-rendered error block on stderr (the error still sets the
  exit code and, in `--format json`, the error envelope on stdout).
- The finding report on stdout in text mode (the exit code still
  signals the finding).

It does **not** suppress:

- The JSON error envelope on stdout (ADR-021).
- ERROR log records.
- The exit code.

See [`output-streams.md`](output-streams.md) § "Interaction with
`--quiet`" for the per-exit-code table.

### 4.4 `--verbose` semantics

`--verbose` sets the effective level to DEBUG. It also:

- Adds the cause chain, stack trace, and `internal_`-prefixed context
  keys to the human-rendered error block on stderr.
- Does **not** alter the JSON error envelope on stdout (the envelope is
  complete by construction; see ADR-021).

### 4.5 Mutual exclusion

`--quiet` and `--verbose` are **mutually exclusive**. Passing both is a
**usage error** (exit 64) with the message:

```text
forge: --quiet and --verbose are mutually exclusive
```

The check runs during flag validation, before any command logic. This
is a deliberate choice: the flags express opposite intents, and
silently picking one would hide a user error. Scripts that build flag
lists dynamically should treat the conflict as a signal to fix the
script.

---

## 5. Interaction with other flags

| Flag | Interaction with log level |
|------|----------------------------|
| `--format json` | Does not change the log level or the logger's stream. Logger records continue on stderr in `--log-format`. |
| `--log-format json` | Does not change the log level. |
| `--verbose` + `--format json` | Logger records on stderr are DEBUG-and-above; stdout remains pure JSON. |
| `--quiet` + `--format json` | Logger records on stderr are ERROR only; the JSON envelope on stdout is printed regardless. |

---

## 6. Defaults and rationale

| Decision | Rationale |
|----------|-----------|
| Default is INFO | A first-time user sees meaningful progress without noise. DEBUG would be overwhelming; WARN would hide useful lifecycle events. |
| `--quiet` maps to ERROR | "Quiet" means "tell me only what's wrong". Progress is noise; errors are not. |
| `--verbose` maps to DEBUG | "Verbose" means "tell me everything". The diagnostic floor is DEBUG. |
| Explicit `--log-level` beats implicating flags | Users often set `--verbose` in an alias and need a one-shot override. |
| Env beats config | Twelve-Factor §III; lets a container set its own verbosity. |
| ERROR is unsuppressable | A logged error is a real problem; if it should be silent, don't log it. |
| Flags are mutually exclusive | The two intents contradict; a silent choice hides a bug. |

---

## 7. Testing strategy

The following are tested by the suite:

- **TDD:** `ParseLevel` accepts all four canonical values and the
  `warning` alias; rejects anything else.
- **TDD:** `Level.Enabled` implements the "that level and above" rule.
- **TDD:** `ResolveLevel` returns the highest-precedence source.
- **ATDD:** an invocation with `--verbose` and `--format json` produces
  DEBUG log records on stderr and a pure JSON envelope on stdout.
- **BDD:** given `FORGE_LOG_LEVEL=warn` and a config file with
  `log.level: debug`, when resolved, then the effective level is WARN.
- **Negative:** `--quiet --verbose` returns a usage error (exit 64)
  before any command logic runs.
- **Negative:** `--log-level=trace` returns a usage error.
- **Edge:** empty `FORGE_LOG_LEVEL=""` falls through to the config file.
- **Corner:** `FORGE_LOG_LEVEL=WARNING` (uppercase alias) resolves to
  WARN.
- **Boundary:** DEBUG records are invisible at INFO level; ERROR records
  are visible at every level.
- **Non-functional:** level resolution is O(1) and allocation-free on
  the hot path; concurrent `SetLevel` is race-free.

---

## 8. References

- [`output-streams.md`](output-streams.md) — the stdout/stderr contract.
- [`exit-codes.md`](exit-codes.md) — exit-code taxonomy.
- [`error-codes.md`](error-codes.md) — error code registry.
- [`../architecture/output-vs-logging.md`](../architecture/output-vs-logging.md)
  — the two-layer output/logging architecture.
- [ADR-007](../decisions/ADR-007-configuration-foundation-precedence-discovery-and-error-taxonomy.md)
  — configuration precedence.
- [ADR-021](../decisions/ADR-021-json-error-envelope-for-format-json.md)
  — the JSON error envelope.
- [ADR-022](../decisions/ADR-022-output-logging-separation.md)
  — output/logging separation.
- Twelve-Factor App §III — Config.
- `log/slog` — Go structured logging.
- WBS 12.1.2, FR-CLI-LOG-002, FR-CLI-LOG-003, Phase 6 WBS §1.3.
