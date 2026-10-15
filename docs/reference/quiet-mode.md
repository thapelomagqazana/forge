# Quiet Mode

<!--
  Document:     quiet-mode.md
  Purpose:      Normative reference for --quiet / -q and quiet mode.
                What each command emits under --quiet, how quiet
                mode interacts with --format json, and why exit
                codes are preserved.
  Audience:     CLI users, operators, script authors, CI authors.
  Stability:    Public contract for the flag and the stream
                behavior. The per-command behavior table is
                semi-stable (may change per release).
  Traceability: FR-CLI-LOG-010, WBS 12.6.1, Phase 6 WBS §19
  See also:     log-levels.md, output-streams.md, verbose-mode.md
  Last Review:  2026-10-12
-->

## 1. What quiet mode does

`forge --quiet` (shorthand: `forge -q`) sets the invocation's
log level to ERROR. In quiet mode:

- Log records at INFO, WARN, and DEBUG are filtered out.
- ERROR records remain.
- Product output on stdout is unchanged.
- The human error block on failure is emitted on stderr.
- The JSON error envelope on failure is emitted on stdout.

Quiet mode is a *diagnostic verbosity* setting. It does not change
what a command produces; it changes how much diagnostic noise the
command emits around the result.

## 2. What quiet mode does NOT do

Quiet mode is often confused with "silence the CLI entirely". It is
not that. Quiet mode **does not**:

- suppress the command's product output on stdout;
- suppress the JSON envelope on stdout under `--format json`;
- suppress the error block on stderr when a command fails;
- change the process's exit code;
- affect the `forge version` or `forge --version` output (both are
  product output and are always emitted).

If a caller wants "no stdout, only the exit code", the caller
should redirect stdout to `/dev/null` (or the platform equivalent).
`--quiet` is a diagnostic choice, not a data-suppression choice.

## 3. The per-command behavior table

The following table is the **normative** per-command contract for
quiet mode. Each row states what the command emits on stdout and
stderr under `--quiet`, relative to the default (no `--quiet`)
behavior.

| Command | stdout under --quiet | stderr under --quiet | Exit code |
|---------|----------------------|----------------------|-----------|
| `forge version` | The version block, unchanged. | Empty. | Preserved. |
| `forge --version` | Same as `forge version`. | Empty. | Preserved. |
| `forge config` | The full config output, unchanged. | Empty (DEBUG/INFO records are filtered). | Preserved. |
| `forge config --path` | The resolved path, unchanged. | Empty. | Preserved. |
| `forge config --format json` | The JSON envelope, unchanged. | Empty. | Preserved. |
| `forge check` | The final one-line status. Per-check lines are suppressed. | Empty on success; error block on failure. | Preserved. |
| `forge new` | The created path (one line). Progress lines are suppressed. | Empty on success; error block on failure. | Preserved. |
| `forge explain` | The full explanation, unchanged. | Empty. | Preserved. |
| `forge update` | The final one-line summary. Per-change lines are suppressed. | Empty on success; error block on failure. | Preserved. |

### 3.1 What "suppressed per-check lines" means

`forge check` without `--quiet` emits one line per check performed,
followed by a summary line. With `--quiet`, the per-check lines are
suppressed; only the summary line is emitted. The exit code is
unchanged: `forge check` exits 1 (findings) or 0 (clean) regardless
of `--quiet`.

The rationale: a CI script wants the exit code and a one-line
summary in its logs. The per-check lines are useful when a human
is reading the output; under `--quiet`, the human has asked for
less.

### 3.2 What "suppressed progress lines" means

`forge new` without `--quiet` emits progress lines ("resolving
template", "writing files", "done") followed by the created path.
With `--quiet`, the progress lines are suppressed; the created
path is emitted as a single line.

The rationale: a script that runs `forge new` wants to capture
the created path. The progress lines are noise; the path is the
result.

## 4. Interaction with `--format json`

Under `--format json`, the command's result is a JSON envelope on
stdout. `--quiet` does not suppress the envelope; it only filters
log records on stderr.

| Flag combination | stdout | stderr |
|------------------|--------|--------|
| `--format json` | One JSON document. | Log records at the effective level. |
| `--quiet --format json` | One JSON document, unchanged. | Log records at ERROR only. |
| `--quiet --format json` (failure) | One JSON error envelope. | The human error block and ERROR records. |

The envelope is the result; `--quiet` does not filter results.

### 4.1 JSON validity is invariant

Under `--format json`, stdout contains exactly one JSON document,
whether `--quiet` is set or not. This is asserted by the
integration test `TestQuiet_FormatJSON_ProducesValidJSON` in
`cmd/forge/binary_integration_test.go`.

## 5. Exit codes are preserved

Quiet mode does not change the process's exit code. Every command
exits with the same code under `--quiet` as it would without it.

The exit code is the primary classification a script uses. A
script that branches on `$?` must not need to know whether the
invocation was quiet. The integration test
`TestQuiet_PreservesExitCode` in
`cmd/forge/binary_integration_test.go` exercises every documented
exit code and asserts the code is identical with and without
`--quiet`.

## 6. Errors are never suppressed

Quiet mode sets the log level to ERROR. The error block on stderr
is not a log record; it is the user-facing explanation of a
failure. It is always emitted on failure, regardless of `--quiet`.

Under `--format json`, the JSON error envelope on stdout is also
emitted, because it is the command's result in JSON mode.

| Failure case | stdout | stderr |
|--------------|--------|--------|
| `--quiet` (text) | Empty. | The human error block. |
| `--quiet --format json` | The JSON error envelope. | The human error block. |
| `--quiet --log-format json` | Empty. | The human error block and JSON error records at ERROR level. |

## 7. Interaction with `--verbose`

`--quiet` and `--verbose` are mutually exclusive. Passing both is
a usage error (exit 64). The conflict is detected before any
command runs.

```text
$ forge --quiet --verbose version
Error: --quiet and --verbose are mutually exclusive
Code:  FORGE_CLI_UNKNOWN_FLAG
       flags: --quiet, --verbose

Suggestion:
  Use one of --quiet or --verbose, not both.
```

The rationale: the flags express opposite intents. Silently
choosing one would hide a user error. The exit code is 64 (usage
error), consistent with the CLI's taxonomy.

## 8. Interaction with `--log-level`

`--log-level` is the explicit level selector. When it is set, it
wins over `--quiet`:

| Args | Effective level |
|------|-----------------|
| `--quiet` | ERROR |
| `--quiet --log-level=debug` | DEBUG |
| `--quiet --log-level=info` | INFO |
| `--quiet --log-level=warn` | WARN |
| `--quiet --log-level=error` | ERROR |

The precedence chain is documented in
[`log-levels.md`](log-levels.md) §2. `--quiet` is an implicating
flag; `--log-level` is explicit and wins.

## 9. Interaction with `FORGE_LOG_LEVEL`

`FORGE_LOG_LEVEL` is consulted only if no CLI flag sets the level.
`--quiet` is a CLI flag, so it wins over `FORGE_LOG_LEVEL`:

| Args | Env | Effective level |
|------|-----|-----------------|
| `--quiet` | (unset) | ERROR |
| `--quiet` | `debug` | ERROR |
| (no flag) | `debug` | DEBUG |

The precedence chain is documented in
[`log-levels.md`](log-levels.md) §2.

## 10. Stability

| Aspect | Stability |
|--------|-----------|
| The `--quiet` flag's existence, shorthand, and stream contract | **Stable.** Public contract. |
| The exit-code preservation guarantee | **Stable.** Public contract. |
| The JSON envelope's presence under `--format json` | **Stable.** Public contract. |
| The per-command behavior table | **Semi-stable.** May change per release. |
| The interaction rule with `--verbose` | **Stable.** Public contract. |

A future change to the per-command behavior table is recorded in
the release notes and in the diff to this document.

## 11. Frequently asked questions

**Q: Why doesn't `--quiet` silence stdout?**
A: Because stdout carries the result. `--quiet` is a diagnostic
verbosity setting. If you want no stdout, redirect it to
`/dev/null`.

**Q: Does `--quiet` suppress the JSON envelope on stdout?**
A: No. The envelope is the result. See §4.

**Q: Does `--quiet` make the CLI silent?**
A: No. Errors are always emitted on stderr. See §6.

**Q: Can I use `-q` and `--verbose` together?**
A: No. The flags are mutually exclusive. See §7.

**Q: Can I override `--quiet` with `--log-level`?**
A: Yes. `--log-level` is the explicit selector and wins. See §8.

**Q: Does `--quiet` change the exit code?**
A: No. See §5.

## 12. References

- [`log-levels.md`](log-levels.md) — the full level semantics and
  precedence chain.
- [`output-streams.md`](output-streams.md) — the stdout/stderr
  contract per exit code.
- [`verbose-mode.md`](verbose-mode.md) — the symmetric reference
  for `--verbose`.
- [`../architecture/output-vs-logging.md`](../architecture/output-vs-logging.md)
  — the two-layer output architecture.
- FR-CLI-LOG-010, WBS 12.6.1, Phase 6 WBS §19.
