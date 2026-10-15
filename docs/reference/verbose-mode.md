# Verbose Mode

<!--
  Document:     verbose-mode.md
  Purpose:      Normative reference for the --verbose flag, DEBUG
                output, and the -v reservation. What verbose mode
                does, which subsystems emit DEBUG records, and why
                -v means --version, not --verbose.
  Audience:     CLI users, operators, script authors, contributors.
  Stability:    Public contract for the flag and the stream
                behaviour. The set of DEBUG records is
                semi-stable (may change per release).
  Traceability: FR-CLI-LOG-009, Phase 6 WBS §20.2, WBS 12.5.1
  See also:     log-levels.md, output-streams.md,
                ../architecture/output-vs-logging.md
  Last Review:  2026-10-12
-->

## 1. What verbose mode does

`forge --verbose` sets the log level to DEBUG. In debug mode, the
CLI emits diagnostic records about how it resolved its inputs:
config discovery, config loading, version resolution, format
selection, and other decision points.

Verbose mode affects **only** stderr. It never changes stdout. See
§4 for the stream contract.

Verbose mode is one of three ways to select DEBUG:

| Input | Effect |
|-------|--------|
| `--verbose` | DEBUG (unless `--log-level` overrides) |
| `--log-level=debug` | DEBUG (explicit override; wins over `--verbose`) |
| `FORGE_LOG_LEVEL=debug` | DEBUG (used only if no CLI flag is set) |

The three are equivalent in outcome; they differ only in
precedence. The full chain is documented in
[`log-levels.md`](log-levels.md) §2.

### 1.1 The log format

Verbose mode's records are rendered in the format selected by
`FORGE_LOG_FORMAT`:

| `FORGE_LOG_FORMAT` | Rendering |
|--------------------|-----------|
| unset, empty, or unrecognized | `human` (default) |
| `json` | one JSON object per line |

The human rendering is:

```text
2026-10-12T10:00:00Z DEBUG version service: resolved build info version=1.2.3 commit=abc1234 dirty=false source=ldflags
```

The JSON rendering is:

```json
{"ts":"2026-10-12T10:00:00Z","level":"debug","msg":"version service: resolved build info","attrs":{"version":"1.2.3","commit":"abc1234","dirty":false,"source":"ldflags"}}
```

The human format is not stable; consumers must not parse it. The
JSON format is documented by `schemas/log-record.schema.json` and
is stable within a major version. See
[`log-record-format.md`](log-record-format.md).

There is no `--log-format` flag in the current release. The env
var is the only selector. A future WBS adds the flag; the env var
remains the fallback.

## 2. The `-v` reservation

`--verbose` has **no short form**. The single letter `-v` is
registered by Cobra as the short form of `--version` when the root
command's `Version` field is non-empty.

### 2.1 Why `-v` means `--version`

The decision follows precedent:

| Tool | `-v` means |
|------|-----------|
| `git` | version |
| `docker` | version |
| `kubectl` | version |
| `go` | version (as `go version -v`, but the flag is on the subcommand) |
| `forge` | version |

In each of these tools, the user types `-v` in a terminal to see
"what version am I running?". The habit is nearly universal. A tool
that gives `-v` to `--verbose` (e.g., `curl -v`, `ssh -v`) inverts
the expectation, and `curl -v` is a well-known exception that
reminds us the choice is contested.

Forge chooses the majority convention: `-v` is `--version`.

### 2.2 Why not `-V` for `--version` and `-v` for `--verbose`

`-V` is occasionally used for version (e.g., `go env -w`), but it
is rare and requires a capital letter, which is slower to type and
harder to remember. The `-v`/`-V` pair is a known source of
confusion. Forge avoids it by using only the long form for
`--verbose`.

### 2.3 How the collision is tested

Three tests pin the decision:

- `TestVersionBehaviour_VerboseHasNoShortFlag` (in
  `internal/cli/root_test.go`) asserts that `forge -v` prints the
  version block, not a verbose banner.
- `TestContract_VerboseHasNoShortForm` (in
  `internal/cli/flags_test.go`) asserts that the `--verbose`
  flag's `Shorthand` field is empty.
- `TestContract_VersionHasShortFormV` (in
  `internal/cli/flags_test.go`) asserts that the `--version` flag's
  `Shorthand` field is `"v"`. The three tests together make the
  reservation a closed system: `-v` is version, so `--verbose`
  cannot take it; `--verbose` has no short form, so `-v` is free
  for `--version`.

## 3. Which subsystems emit DEBUG

The set below is the **current** set. It is semi-stable: a future
release may add records, remove records, or change the fields on an
existing record. Scripts that parse DEBUG output should treat it as
diagnostic, not as product output.

The records are grouped by subsystem. Each subsystem's records are
emitted by that subsystem's code; the CLI's process boundary does
not synthesize records on a subsystem's behalf.

### 3.1 Process boundary (`internal/cli/execute.go`)

The process boundary configures the logger but does not emit
records itself. It calls `logger.SetDefault` with the resolved
decision before the command tree runs; the records below are
emitted by the subsystems the tree invokes.

### 3.2 Version CLI handler (`internal/cli/version.go`)

The `forge version` command emits two records before writing its
output:

| Message | Fields | When |
|---------|--------|------|
| `version: resolved build info` | `version`, `commit`, `build_date`, `dirty` | After `appversion.Get()` returns, before the format dispatch. |
| `version: selected format` | `format`, `accepted` | After the format switch resolves; `accepted` is `false` when the format is unknown. |

The `accepted=false` case is followed by the unknown-format error;
the record is emitted before the error so a reader with
`--verbose` sees both.

### 3.3 Version application service (`internal/app/version/service.go`)

The version service emits two records per resolution:

| Message | Fields | When |
|---------|--------|------|
| `version service: reading build info` | (none) | Before `buildInfo()` runs. |
| `version service: resolved` | `version`, `commit`, `build_date`, `dirty`, `duration_ms` | After `buildInfo()` returns. |

The `duration_ms` field is the wall time between the two records.
It is reported because `debug.ReadBuildInfo` can be slow in
environments with a constrained `/proc`.

### 3.4 Config CLI handler (`internal/cli/config.go`)

The `forge config` command emits three records:

| Message | Fields | When |
|---------|--------|------|
| `config: resolving invocation` | `path_mode`, `format` | After flag parsing, before any resolution. |
| `config: resolved path` | `path`, `exists`, `mode` | After the resolved Config is obtained; `path` is `(none)` when no file was resolved. |
| `config: selected format` | `format`, `mode`, `accepted` | After the format dispatch; `mode` is `path` or `full`; `accepted` is absent for `--path` (the path-mode renderer accepts both formats). |

### 3.5 Config loader (`internal/config/loader.go`)

The loader emits four records tracing its pipeline:

| Message | Fields | When |
|---------|--------|------|
| `config loader: entered` | `path` | Before any work; `path` is `(none)` when the empty-path shortcut applies. |
| `config loader: read file` | `path`, `bytes`, `bom` | After the file is read; `bom` reports the BOM check's result (always `false` when the record is emitted). |
| `config loader: parsed YAML` | `path`, `bytes`, `log_level_set`, `output_format_set` | After the YAML document parses; the `*_set` fields report which fields the file actually specified. |
| `config loader: validated` | `path`, `log_level`, `output_format` | After the enum validation; the values are normalized. |

The loader's error branches do **not** emit records. An error is
not a decision; the boundary renders it with `forgeerr.Format` on
stderr, and that rendering is the diagnostic.

### 3.6 Config discovery (`internal/config/discover.go`)

The discovery algorithm emits one record per candidate it
evaluates:

| Message | Fields | When |
|---------|--------|------|
| `config discovery: explicit candidate` | `source`, `path`, `exists`, `readable` | For each explicit source (`--config` or `FORGE_CONFIG`); `source` is `flag` or `env`. |
| `config discovery: candidate` | `source`, `path`, `exists`, `readable` | For each optional source (project-local, project-local-legacy, xdg, home, appdata). |

The algorithm short-circuits at the first existing candidate, so
the number of records equals the number of candidates the
algorithm evaluated, not the number of candidates it defined. A
run that resolves to the canonical project-local file at source 3
emits one record (for source 3); a run that finds nothing emits
one record per optional source evaluated. The `readable` field
duplicates `exists` in the current implementation because
discovery checks existence, not readability; a future change that
adds a readability check will populate it meaningfully.

### 3.7 Subsystems that do not emit DEBUG

The **non-emitting** subsystems are as important to name as the
emitting ones:

| Subsystem | Why no DEBUG |
|-----------|--------------|
| `internal/output` | Product output is not diagnostic. Emitting DEBUG here would duplicate the logger's work. |
| `internal/forgeerr` | Error formatting is a classification concern, not a diagnostic one. Errors flow through their own stream. |
| `internal/streams` | The streams package is a plumbing layer; it has no decisions to report. |
| Cobra's own code | Cobra's internals are stable; wrapping them with DEBUG produces noise. |

A future subsystem that adds DEBUG records must update this
document's §3 in the same change. The document is the register of
which subsystems emit; a subsystem that emits without being
listed is a documentation defect.

## 4. The stream contract

Verbose mode obeys the two-layer output architecture:

| Stream | Content under `--verbose` |
|--------|---------------------------|
| stdout | Product output, unchanged from the default run. |
| stderr | DEBUG records, INFO records, WARN records, ERROR records. |

### 4.1 What "unchanged" means

`forge version` and `forge --verbose version` produce
**byte-identical stdout**. The `--verbose` flag does not add a
banner, a timing line, or a header to stdout. This is asserted by
`TestVerbose_StdoutUnchanged_BDD` in `cmd/forge/binary_integration_test.go`.

### 4.2 What changes on stderr

`forge version` (default) emits no log records at all on stderr.
`forge --verbose version` emits the four records from §3.2 and
§3.3, in this order:

```text
version: resolved build info     (handler)
version: selected format         (handler)
```

Wait — the handler's records are emitted *after* the service's,
because the handler calls the service before emitting its own.
The actual order is:

```text
version service: reading build info     (service)
version service: resolved               (service)
version: resolved build info            (handler)
version: selected format                (handler)
```

A reader of the DEBUG log sees the service's records first (the
resolution), then the handler's records (the dispatch on the
resolved values). This ordering is the point: the log is a
top-down trace of the invocation, with the innermost work
appearing before the outermost report.

### 4.3 Under `--format json`

Under `--format json`, stdout remains exactly one JSON document,
and the DEBUG records continue on stderr unchanged. This is the
`--format json` interaction rule from
[`log-levels.md`](log-levels.md) §5.

## 5. Idempotence

Setting `--verbose` more than once is the same as setting it once:

| Invocation | Effective level |
|-----------|-----------------|
| `forge --verbose version` | DEBUG |
| `forge --verbose --verbose version` | DEBUG |
| `forge --verbose version --verbose` | DEBUG |

The flag is a boolean; Cobra's parser accepts repeats and stores
`true` regardless of how many times it appeared. The verbose test
suite includes a case for the repeated form
(`TestVerbose_RepeatedFlagIsIdempotent`).

## 6. Secrets and redaction

DEBUG records must not contain secrets. The redaction policy for
errors ([ADR-016](../decisions/ADR-016-error-redaction-policy.md))
applies to DEBUG records by extension: a value that would be
redacted in an error must be redacted in a DEBUG record.

The rule is enforced by WBS 12.7.1, which audits the emitted
records. This WBS does not duplicate the audit; it commits to the
policy.

In practice, the records listed in §3 do not include credentials,
tokens, or full environment dumps. The config discovery records
name candidate *paths*, which can be sensitive (a home directory
reveals a username), but they are the same paths the config
command already reports on stdout. A future record that would
include a credential or a token must be added in the same change
as a redaction test.

## 7. The set of DEBUG records is semi-stable

A consumer that parses DEBUG output treats it as diagnostic, not
as product output. The stability policy is:

| Aspect | Stability |
|--------|-----------|
| The `--verbose` flag's existence and stream behaviour | **Stable.** Public contract. |
| The `--verbose` flag's effect on the log level | **Stable.** Public contract. |
| The reserved meaning of `-v` | **Stable.** Public contract. |
| The set of subsystem records | **Semi-stable.** May change per release. |
| The fields on a given record | **Semi-stable.** May change per release. |
| The human rendering's format | **Not stable.** Do not parse. |
| The JSON rendering's schema | **Stable within a major version.** |

A script that needs to assert "the invocation was verbose" should
check the flag, not parse the DEBUG output. A script that needs to
assert "the version was resolved from a specific source" should
run `forge version --format json` and parse stdout, not parse
stderr.

## 8. Frequently asked questions

**Q: Why does `--verbose` not have a short form?**
A: Because `-v` is taken by `--version`. See §2.

**Q: Does `--verbose` make the CLI slower?**
A: Slightly. DEBUG records are formatted and written to stderr on
every emission. The overhead is small for the current record count
(< 20 per command) but is not zero. Production scripts that do not
need diagnostics should not pass `--verbose`.

**Q: Can I get DEBUG output without `--verbose`?**
A: Yes: `FORGE_LOG_LEVEL=debug forge version` is equivalent. See
§1.

**Q: Does `--verbose` affect `--format json`?**
A: No. Under `--format json`, stdout remains exactly one JSON
document. DEBUG records continue on stderr, unchanged. This is
the `--format json` interaction rule from
[`log-levels.md`](log-levels.md) §5.

**Q: What if I pass `--verbose --quiet`?**
A: It is a usage error (exit 64). The flags are mutually
exclusive; see `log-levels.md` §4.5.

**Q: Where does DEBUG output go when stderr is a TTY?**
A: The same place as when stderr is a pipe: to stderr. The CLI
does not detect TTYs and does not change its behaviour based on
where stderr points. A user who wants coloured output on a TTY is
asking for a terminal feature, not a CLI feature.

**Q: Why do some records have the same message but different
fields?**
A: `config discovery: candidate` appears once per optional
candidate; the `source` field distinguishes the candidate.
`config: selected format` appears twice in the `--path` mode
(once for the invocation, once for the format dispatch) but the
`mode` field distinguishes the two invocations. A consumer that
filters by message should also inspect the fields.

**Q: Why is the human format "not stable"?**
A: The human format's exact rendering (the separator between the
level, the message, and the fields; the field ordering; the
timestamp format) is determined by `internal/logger`'s formatter
and may change per release to improve readability. A script that
parses it will break. The JSON format is the stable alternative.

**Q: Can I disable DEBUG records without disabling verbose mode?**
A: No. `--verbose` and `--log-level=debug` both enable DEBUG
records. To suppress them, do not pass those flags and do not set
`FORGE_LOG_LEVEL=debug`. If a specific record is too noisy, the
correct response is to report it as a defect and have the emitting
subsystem remove or refine it; the CLI does not provide per-record
filtering.

## 9. References

- [`log-levels.md`](log-levels.md) — the full level semantics and
  precedence chain.
- [`output-streams.md`](output-streams.md) — the stdout/stderr
  contract per exit code.
- [`log-record-format.md`](log-record-format.md) — the JSON
  record schema and the human rendering.
- [`../architecture/output-vs-logging.md`](../architecture/output-vs-logging.md)
  — the two-layer output architecture.
- [ADR-016](../decisions/ADR-016-error-redaction-policy.md) —
  error redaction policy.
- FR-CLI-LOG-009, Phase 6 WBS §20.2, WBS 12.5.1.
