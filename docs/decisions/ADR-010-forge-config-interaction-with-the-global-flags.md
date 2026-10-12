# ADR-010: `forge config` Interaction with the Global Flags

**Status:** Proposed
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** The clause in ADR-006 that describes `--format` as a per-command flag.
**Superseded by:** —

---

## 1. Context

ADR-008 fixed `forge config`'s scope and ADR-009 fixed its output formats. Neither addressed how the command composes with the CLI's global flags: `--verbose`, `--quiet`, `--config`, and `--format`. The gap matters because the flags are visible to every subcommand, and a user who types `forge --quiet config` or `forge config --path --format json` reasonably expects a defined behaviour. Without one, the composition is an implementation detail that can drift between releases.

Two sub-problems sit inside the gap. First, `forge config` needs a way to print only the resolved config file path, without the full-configuration output. This is not a rendering variant of the two formats ADR-009 froze; it is a distinct mode with a distinct output (a single line, in text or JSON). The mode exists because scripts commonly want the path alone — to validate it, to pass it to another tool, to compare it across invocations — and parsing the full output for the path is brittle. A dedicated flag is the natural mechanism.

Second, the flag-interaction matrix that § 4.10i of the CLI UX specification froze does not include `forge config`'s rows. The matrix covers `--help`, `--version`, and unknown-command invocations. Adding `forge config` to the matrix means stating, per flag, whether the flag affects the command's output, its diagnostics, both, or neither. The statement is small but is a contract: a script that relies on `--quiet` not suppressing the config output would break if a future change made it suppress.

A third force is the `--format` flag's scope. ADR-006 described `--format` as per-command. The shipped implementation registers it as a global flag on the root command, inherited by every subcommand that renders output. The implementation is correct — the semantics are uniform across commands, and a per-command flag would proliferate declarations — but the specification's description lags. This ADR records the correction so that future readers, who consult the specification before the code, see the correct model.

The problem this ADR addresses: *what does `forge config` do when combined with each global flag, and what does `--path` mean?* Without a frozen answer, the command's behaviour with the flags is undefined, and a change to any of them can alter the command's output without a reviewable signal. The freeze bounds the cost of a future edit.

This ADR relates to ADR-006 (help and CLI UX contracts), ADR-007 (configuration foundation), ADR-008 (`forge config` scope), ADR-009 (`forge config` output format), WBS 9.3.1 (`--path` semantics), WBS 9.4.2 (flag interaction), and CLI UX specification § 4.6.3, § 4.6.4, § 4.10i, § 4.12, and § 4.14. The audience is the Forge Core Team, script authors who compose flags with the command, and any future contributor who proposes a new global flag or a change to `--path`.

---

## 2. Options Considered

### Option A — No `--path` flag; the path is one field of the full output

`forge config` produces only the full output (text or JSON). The resolved config file path appears as the `Source:` line in text and as `config_path` in JSON. Scripts that want only the path parse the full output. No `--path` flag is added. The command's interaction with `--quiet`, `--verbose`, and `--config` is left undefined; each behaves as the flag's general semantics imply.

**Pros:**

- One output to design; no second mode to freeze.
- The path is already present in both formats, so a script can extract it with a small parse (in JSON, `jq .config_path`; in text, `grep '^  Source:'`).
- The command's flag surface stays minimal: no `--path`.
- No ambiguity about how `--path` composes with `--format` (there is no `--path`).

**Cons:**

- Extracting the path from the text format is fragile: it depends on the label's spelling, its column padding, and the line's position. A change to any of them breaks the extraction.
- The command does not offer a mode whose output is *just* the path, which is what a shell script needs. `forge config | grep '^  Source:'` returns the label and padding; the script must strip them. `forge config --format json | jq -r .config_path` returns `null` for the no-file case, which the script must handle.
- The flag-interaction rows are left implicit. A reader of the specification cannot point at a frozen statement; they must infer the behaviour from the flags' general semantics.
- The command's behaviour with `--quiet` is not pinned. A future change that made `--quiet` suppress the config output would break no test and no frozen statement; it would surface only when a user reported it.

### Option B — A dedicated `--path` flag, with explicit flag-interaction rules

`forge config` gains a command-local `--path` flag. When set, the command prints only the resolved config file path: a single line in text mode, a single-key JSON object in JSON mode. The flag composes with `--format` and, where applicable, with `--config`. The command's interaction with each global flag is stated in a frozen matrix.

**Pros:**

- Scripts that want the path get it directly, with no parsing. `forge config --path` returns one line: the path, or `<none>`. The output is designed for shell consumption (a single line, a trailing newline, no decoration).
- The `--path` output is unambiguous. There is no label to strip, no column to align, no parse to write. The line is the value.
- The `--path` mode composes with `--format` in a defined way: the JSON mode returns `{"config_path": "<path>"}` or `{"config_path": null}`, a one-key object whose key is the same `config_path` used in the full JSON output.
- The command's interaction with the global flags is frozen. A reader can look up the row for any combination and see the defined behaviour.
- The `--path` output is deterministic and cheap to produce. No labels, no alignment, no source attribution; just the path.

**Cons:**

- Two output modes now exist (full and `--path`), each with two formats (text and JSON), giving four combinations. Each combination needs golden files, tests, and a specification entry.
- The `--path` flag's name is short and could be confused with a subcommand; the specification and the help text must disambiguate.
- The flag-interaction rules are stated in the specification and must be mirrored in tests. A future change to a flag's semantics (for example, if `--quiet`'s contract ever included "suppress all output") must be reflected in the config command's rows, or the two diverge.
- The `--path` mode duplicates information available in the full output. A user who wants both the path and the full configuration runs the command twice or parses the full output for the path.

### Option C — A `--path` subcommand: `forge config path`

Instead of a flag, `forge config` gains a subcommand named `path`. The subcommand prints the resolved config file path, and its siblings (`forge config show`, `forge config get`, etc.) are reserved for future phases. The full command remains the parent's default output.

**Pros:**

- Subcommands are a natural extension of the command. `forge config path` reads as a noun phrase: "the config's path". The grammar is clear.
- Future subcommands (`get`, `set`, `unset`) slot in without changing the parent's flag surface.
- The parent command's `Args` changes from `cobra.NoArgs` to accepting a subcommand; the change is bounded.

**Cons:**

- Subcommands conflict with the parent's default behaviour. In Cobra, a parent with subcommands and a `RunE` handler that also produces output is possible but requires care: `forge config` (no arguments) must still print the full configuration, and `forge config path` must dispatch to the subcommand. The two coexist, but the shape is less common than a flag.
- The subcommand reserves the name `path` and the names of any siblings. A future decision to add a flag named `--path` would then collide; the two mechanisms are mutually exclusive for the same behaviour.
- `forge config path` and `forge config --path` read similarly in a shell, and a user who has learned one form must remember not to type the other. The confusion is small but real.
- The subcommand is a larger surface than the flag for the same behaviour. A subcommand needs its own `Short`, `Long`, `Example`, and `Args`, and it appears in `forge config --help`'s "Available Commands:" section. The flag needs only a usage string.

### Option D — A `--path` flag, but no explicit interaction rules

`forge config` gains a `--path` flag with the same output as Option B, but the command's interaction with the global flags is left to the flags' general semantics: `--quiet` suppresses diagnostics, `--verbose` adds them, `--config` determines the loaded file, and the command's output is unaffected by any of them. No frozen matrix is stated; the behaviour is inferred from the flags' individual contracts.

**Pros:**

- One less table to freeze and one less test to write.
- The behaviour is what a reader would infer from the flags' contracts; no additional statement is needed.
- A change to a flag's general semantics automatically propagates to the config command without a separate edit.

**Cons:**

- The inference is correct today but is not a contract. A future change to a flag that happens to affect the config command's output — for example, if `--quiet`'s contract were ever widened — would alter the command's behaviour without an ADR and without a test failure.
- A reader cannot point at a frozen statement; the specification's row for the config command is absent, which suggests (wrongly) that the interaction is undefined.
- The command's behaviour with `--config` is not pinned. The flag determines the loaded file, which appears in the output's `Source:` line; a change to how the loader treats the flag (for example, its error on a missing file) would alter the command's output without a frozen row.

---

## 3. Decision

The decision is **Option B**: `forge config` gains a command-local `--path` flag, and the command's interaction with the global flags is stated in a frozen matrix.

`--path` is a command-local boolean flag (not persistent). When set, it suppresses the full-configuration output and prints only the resolved config file path. The output is a single line: the path, or `<none>` if no file was resolved. In JSON mode (`--path --format json`), the output is a one-key object: `{"config_path": "<path>"}` or `{"config_path": null}`. The key `config_path` is the same key used in the full JSON output; the two modes share it by design.

The command's interaction with the global flags is:

- **`--config <path>`** determines which file the loader reads. The resolved path appears in the `Source:` line (text) and in `config_path` (JSON). A missing explicit path is a loader error (exit 3), produced before the command runs.
- **`--quiet`** sets the log level to ERROR. It does not suppress the command's stdout. The command's output is a result, not a diagnostic; `--quiet`'s contract is to suppress diagnostics, not results.
- **`--verbose`** sets the log level to DEBUG. In Phase 2, no command emits debug output; the config command is no exception. A future phase (WBS 12.0) will add discovery debug output on stderr; when it does, the config command will emit it, and the flag's row will be updated.
- **`--format`** selects the rendering, in both the full and `--path` modes. The flag is global, not per-command; the earlier description of it as per-command is superseded by this ADR.

Option B was chosen over Option A because the `--path` mode's output is designed for the script that wants only the path; parsing the full output for the path is brittle, and the flag exists precisely to avoid that brittleness. Option B was chosen over Option C because the flag is a smaller surface than a subcommand for the same behaviour, and the parent command's default output (the full configuration) is what `forge config` with no arguments should do; a subcommand would complicate that default. Option B was chosen over Option D because the frozen matrix is the contract that a future change would need to update; leaving the interaction implicit means a future change can alter the behaviour without a reviewable signal.

The chosen interaction assumes:

1. **`--quiet`'s contract remains "suppress diagnostics, not results".** If the contract were ever widened, the config command's row would need to change and the change would be an ADR-worthy event.
2. **`--verbose`'s Phase 2 behaviour is "set the log level; emit nothing".** A future phase that adds debug output to the config command updates the flag's row in the same ADR or WBS item that adds the output.
3. **`--format`'s value set is `{text, json}`.** A third format requires an ADR (see ADR-009's notes); the config command's rows would need to reflect the third value in both the full and `--path` modes.

---

## 4. Consequences

### 4.1 Positive

- Scripts that want only the resolved config file path get it directly, in a shape designed for shell consumption. The output has no label to strip and no format to parse.
- The `--path` output's shape is predictable: one line in text, one key in JSON. A script can capture it and use it without further processing.
- The `--path` mode composes with `--format` in a defined way. A JSON-mode script can read `config_path` from the `--path` output with the same key it would use in the full output.
- The command's interaction with each global flag is frozen. A reader can point at the row and cite it; a future change to a row is a change to the ADR.
- The `--path` output's JSON key is the same `config_path` used in the full output. The two modes are consistent by construction; a script that handles one can handle the other.
- The `--config` interaction is now explicit. The flag determines the loaded file; the file's path appears in the `--path` output and in the full output's `Source:` line. A user who wants to verify that a specific file was loaded has a direct way to do so.
- The `--quiet` interaction is pinned. The command's output is a result, not a diagnostic; `--quiet` does not suppress it. A script that pipes the output is unaffected by the flag.

### 4.2 Negative

- Two output modes (full and `--path`) exist, each in two formats (text and JSON). The testing surface is four combinations, and each combination has its own golden file. The duplication is the accepted cost of serving both audiences for both modes.
- The `--path` flag's name is short and could be confused with a subcommand by a user who has seen `forge config path` in another tool. The specification and the command's help text must disambiguate; the confusion is small but is real.
- The frozen matrix is a maintenance burden. A future change to a global flag's contract (for example, if `--quiet`'s scope widened) requires updating the config command's rows in the same change. The two are coupled by the matrix; the coupling is the point, but it is also a cost.
- The `--path` mode duplicates information available in the full output. A user who wants both the path and the full configuration must either run the command twice or parse the full output for the path. The flag serves the user who wants the path *alone*; it does not eliminate the need for the full output.
- The `--path --format json` mode's output is a one-key object that shares its key with the full output's `config_path` field. A script that reads both modes must handle the same key in two shapes: as a scalar within a five-key object in the full output, and as the sole key in the `--path` output. The two are compatible — the key's value is the same type (`string` or `null`) in both — but the shapes differ and the script must account for the difference.
- The `--path` flag is command-local, not persistent. A user who types `forge --path config` (putting the flag before the command name) receives a usage error. The flag's scope is correct for its semantics (only `forge config` has a concept of "path only"), but the placement requirement is a small usability constraint that the help text must document.

### 4.3 Neutral

- The `--path` flag has no short form. The single-letter aliases are reserved for other purposes in the CLI (`-v` for `--version`, `-h` for `--help`), and `-p` is not conventional for "path" in the CLI's flag set. A short form would collide with future flags; none is added.
- The `--path` mode's text output has no title, no separator, and no label. The output is *just* the path. This is a departure from the full output's structure, but it is the correct shape for the mode: a script that reads the output should not have to parse decoration.
- The `--path` mode's JSON output is a one-key object, not a bare string. The alternative (a bare string, `"/home/user/.config/forge/config.yaml"`) would be shorter but would not compose with the JSON convention: a consumer expecting a JSON object would receive a string. The one-key object is the correct shape for a JSON-emitting command.
- The `--path` mode's JSON output uses `null` for the no-file case, matching the full output's `config_path`. The alternative (`""`) would be ambiguous; `null` is the correct representation of "no file resolved".
- The `--format` flag's description in ADR-006 is superseded by this ADR's description. The flag is global, not per-command; the earlier description was written before the flag was registered. The supersession is a correction, not a change to the flag's behaviour; the flag has always been global in the shipped implementation.
- The `--quiet` interaction's row in the matrix is a special case of the CLI's general rule ("`--quiet` suppresses diagnostics, not results"). The row's presence in the matrix is a reminder, not a new rule; the rule's general statement is in § 4.10i.

---

## 5. Related Requirements

The requirements below are drawn from
[`docs/requirements-register.md`](../requirements-register.md).
Requirement IDs that do not yet exist are marked as pending.

- `FR-CLI-007` — `forge config --path` prints only the resolved config file path. This ADR establishes the requirement and its output shape.
- `FR-CLI-006` — `forge config` displays the resolved CLI configuration. This ADR refines the requirement with the command's flag-interaction rules.
- `FR-CFG-002` — the source of each configuration value is observable. The `--config` flag determines which file supplies the values; the command's output reflects the resolved file's path.
- `UX-008` — the command's output is unaffected by `--quiet`. This ADR is the source of the requirement.
- `UX-009` — the command's `--path` mode composes with `--format`. This ADR is the source of the requirement.
- `LOG-004` — `--verbose` sets the log level to DEBUG and does not affect stdout. The config command honours this rule.
- `LOG-005` — `--quiet` sets the log level to ERROR and does not suppress results. The config command honours this rule.

The following requirement is proposed but not yet registered:

> The following requirement must be added to the register when
> WBS 1.4 is complete: `UX-0NN` — the `--path` mode's JSON output
> shares the `config_path` key with the full JSON output, so that a
> consumer can read the path from either mode with the same key.

---

## 6. Related ADRs

- `ADR-006` — help and CLI UX contracts. This ADR supersedes the clause in ADR-006 that describes `--format` as a per-command flag. ADR-006's other clauses (help behaviour, help invocation matrix, output stream boundary, colour policy) remain in force and are honoured by this ADR's decision. The supersession is a correction of one clause, not a replacement of the ADR.
- `ADR-007` — configuration foundation, precedence, discovery, and error taxonomy. This ADR's `--config` interaction relies on ADR-007's discovery algorithm and on its error taxonomy for the missing-file case. The two are complementary.
- `ADR-008` — `forge config` command scope. This ADR is the flag-interaction counterpart to ADR-008's scope statement. ADR-008 established the command's subject; this ADR establishes the flags that select or restrict its output. The two are complementary.
- `ADR-009` — `forge config` output format. This ADR's `--path` mode reuses ADR-009's formats in a restricted shape. The full output's formats are ADR-009's subject; the `--path` mode's shape is this ADR's subject. The two ADRs share the JSON key `config_path` by design.

---

## 7. Notes

**Deferred decisions.** The following are anticipated but not decided here.

- **A `--path` subcommand.** If a future phase decides that `forge config path` (a subcommand) is preferable to `forge config --path` (a flag), the change requires a new ADR. The subcommand and the flag would be mutually exclusive for the same behaviour; adding the subcommand would require removing the flag or providing both (which is rejected as a mechanism duplication).
- **Additional global flags.** If a future phase adds a fifth global flag (for example, `--no-color`), the config command's row in the interaction matrix must be added in the same change. The matrix's format accommodates new rows without an ADR; the addition is a row, not a rewrite.
- **A `--path` short form.** If a future phase reserves a short letter for `--path` (for example, `-p`), the flag's declaration changes and the help output updates. The decision is deferred; the current flag has no short form.
- **Write operations.** A future `forge config set` subcommand would interact with the global flags in ways this ADR does not cover. The write surface requires its own ADR (see ADR-008's notes), and the write command's flag-interaction rules would be stated there.

**Anticipated debug output.** WBS 12.0 will add discovery debug output to the config command. When it lands, the command will emit diagnostic lines on stderr when `--verbose` is set, and the flag's row in this ADR's matrix will be updated to say so. The `--quiet` row is unaffected: the precedence rule (`--quiet` wins) means no debug output appears when both flags are set.

**On the `--format` global scope.** The CLI's `--format` flag is registered once on the root command and inherited by every subcommand that renders output. The flag's value set is per-command (the config command accepts `text` and `json`; the version command accepts `text` and `json`; future commands may accept additional values). The `--format` flag's *scope* (global) and its *value set* (per-command) are separate; a future reader should not confuse the two. The scope is stated in ADR-009 and § 4.12 of the spec; the value sets are stated in § 4.14.

**On the `--quiet` interaction.** The rule that `--quiet` does not suppress results is a special case of the CLI's general stream-boundary contract (§ 4.10h): results go to stdout; diagnostics go to stderr; `--quiet` affects only the latter. The config command's row is a reminder of the general rule, not an exception to it. If a future flag or command ever blurred the boundary, the config command's row would need to be re-examined.

**Follow-up.** No follow-up ADRs are anticipated from this decision. The three ADRs (ADR-008, ADR-009, ADR-010) together freeze the command's scope, its output format, and its flag interaction. A future change to any of the three is a change to a specific ADR.

---

*End of ADR-010.*
