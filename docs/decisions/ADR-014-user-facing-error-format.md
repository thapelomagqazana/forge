# ADR-014: User-facing error format

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge prints errors to stderr. The output is consumed by two audiences: a human who reads it interactively and a script that captures it for logging, alerting, or CI gating. Both audiences require a stable shape. A human who has learned to read Forge errors should not have to relearn after an upgrade; a script that greps for a specific line, or a CI runner that asserts on a specific substring, should not break when a minor release changes a comma.

Before WBS 10.4.1, the CLI rendered errors through a local helper (`formatError`) that produced a two-block shape: an `Error:` line, an optional `Suggestion:` block. The shape had been established by WBS 7.3.2 as the "initial error format", and it was explicitly documented as provisional: the WBS 7.3.2 specification said "the format is the Phase 2 shape" and noted that "richer errors (with structured fields, error codes, file/line information) arrive with WBS 10.0". The local helper was not covered by golden files, and the format's rules were described in prose rather than enforced by tests. Different call sites could produce slightly different shapes because the helper accepted an `errorContext` struct whose fields the caller assembled.

The WBS 10.x series replaced the error type (`*forgeerr.Error` with a Code, a Category, a cause, a context map, and a remediation), the taxonomy (seven categories), the code registry (19 Codes), and the wrapping contract. The type carries more information than the WBS 7.3.2 format could render: a Code, a Category (implicitly, via the Code's prefix), a context map, and a remediation. The old `Error: <message>` shape discarded all but the message.

The WBS 10.4.1 task is the freeze. It supersedes the informal format from WBS 7.3.2 and defines the byte-exact string that `forgeerr.Format` produces. The format includes the message, the Code, the context lines (sorted alphabetically), and the optional `Suggestion:` block. The task's inputs are WBS 7.3.2 (the shape) and WBS 10.1.1 (the error's fields). The affected parties are every user who reads an error, every script that parses one, every log aggregator that indexes one, and every contributor who writes an error message.

The consequence of not deciding is that the format remains informal, different call sites produce slightly different shapes, and scripts that parse the output cannot rely on its structure. The freeze is the mechanism by which the format is a contract rather than a convention.

---

## 2. Options Considered

### Option A — Keep the pre-WBS-10.4.1 shape

Retain the `Error: <message>` line and the optional `Suggestion:` block. Do not render the Code, the Category, or the context.

**Pros:**

- No change to the shape; existing scripts continue to work.
- The format is the shortest possible; a reader sees only the message and the suggestion.
- The rendering is trivial.

**Cons:**

- The Code is not rendered. A user who wants to search the documentation or file a bug report has nothing to quote.
- The context is not rendered. A config error that knows the file and the line number cannot show them.
- The Category is not rendered (except implicitly through the Code, which is not rendered). A script cannot infer the category from the message.
- The format does not evolve with the type. As the type gains fields, the format falls further behind, and a future extension becomes a larger change.

### Option B — Single-line key-value format

Render the error as a single line of key-value pairs: `code=<code> category=<category> message="<message>" file=<file> line=<line>`. The shape is closer to a log line than to a human-readable block.

**Pros:**

- Compact; one line per error.
- The shape is easy to parse with a single regular expression.
- The keys are explicit; a script does not have to infer the structure from the indentation.

**Cons:**

- Multi-line values (a long message, a remediation) do not fit; they would have to be quoted and escaped.
- The format is not idiomatic for a user-facing CLI. A human reader expects a readable block, not a log line.
- The distinction between the message and the context is lost; the message is not visually separated from the fields.
- The format is essentially the log format (ADR-015), which the log channel already provides. Having the same shape for both the human-facing and the log-facing outputs duplicates the log format's purpose.

### Option C — Multi-line block with Code, context, and Suggestion

Render the error as a multi-line block: an `Error: <message>` line, a `Code:  <code>` line, one line per context entry (indented seven spaces), and an optional `Suggestion:` block.

**Pros:**

- The message is visually separated from the structured fields; a human reader sees the message first.
- The Code is rendered; a user can quote it, and a script can match on it.
- The context is rendered; a config error can show the file and the line number without forcing the user to parse the message.
- The `Suggestion:` block is preserved from the previous format; existing users' muscle memory is honored.
- The shape is extensible: a future field can be added as another indented line without breaking the existing structure.
- The alignment (two spaces after `Code:`, seven spaces of context indent) is easy to produce and easy to read.

**Cons:**

- The format is longer than the previous one; an error without context or remediation takes two lines instead of one.
- The alignment rules are byte-exact; a contributor who adds a field must know the column positions. The golden files catch the mistakes, but the rules are numerous (twelve in the current specification).
- The format is less compact than the single-line key-value format for a script that wants only the Code.
- The format is not a standard; a user who is familiar with a different CLI's error output has to learn Forge's.

### Option D — JSON envelope with a text fallback

Render the error as a JSON object when the output stream is not a terminal, and as a human-readable block when it is.

**Pros:**

- Machine consumers get structured output for free.
- Human readers get a readable block.

**Cons:**

- The dual behaviour is a maintenance burden: two renderers to keep in sync, two sets of golden files, two sets of tests.
- TTY detection introduces platform-specific behaviour; the output is no longer deterministic across environments, which breaks the golden-file test.
- The choice of shape depends on the consumer, not on the error; a script that pipes the output to a file and a user who reads it interactively see different shapes for the same error.
- The format is no longer one contract; it is two, and the two drift.

**This option was rejected before the ADR was written; it is listed here for completeness.**

---

## 3. Decision

The format is **Option C**: a multi-line block that renders the message, the Code, the context entries (sorted alphabetically), and the optional `Suggestion:` block. The format is byte-exact and enforced by golden files. Any change requires an ADR.

```text
Error: <message>
Code:  <code>
       <context-key-1>: <value-1>
       <context-key-2>: <value-2>

Suggestion:
  <remediation>
```

The balance tipped toward Option C for three reasons. First, the error type carries the Code and the context; the format must render them, or the type's information is wasted. Option A discards the information; Option B discards the human readability. Option C renders both without compromising either. Second, the format must be stable: the multi-line block is a shape that a human reader recognises and a script can parse with a simple line-oriented algorithm. The shape is idiomatic for a CLI. Third, the format must be extensible: a future field can be added as another indented line without breaking the existing structure. The single-line key-value format (Option B) and the JSON envelope (Option D) are less extensible in different ways; Option B cannot accommodate multi-line values, and Option D bifurcates the contract.

The twelve byte-level rules (F1 through F12) are part of the decision, not an implementation detail. They are documented in the CLI UX specification and in the `format.go` file's doc comment. The rules specify the indentation, the two-space and seven-space alignments, the blank line before the `Suggestion:` block, the trailing-newline rule, the no-colour rule, and the alphabetical sort of the context keys.

The trade-offs accepted are: the format is longer than the previous one (a two-line minimum rather than a one-line minimum); the alignment rules are byte-exact and numerous (twelve rules); the format is not a standard, so a user who is familiar with a different CLI's output has to learn Forge's. The last is the price of having any format at all.

The decision assumes that the format's two audiences — human readers and script consumers — are best served by the same shape. If a future phase finds that the two audiences require different shapes (for example, a script that wants only the Code), the ADR for that phase would introduce a machine-readable output mode (a `--format json` flag), not a bifurcation of the human-facing format.

---

## 4. Consequences

### 4.1 Positive

- The Code is rendered. A user can quote it in a bug report, and a script can match on it.
- The context is rendered. A config error that knows the file and the line number shows them; a security error that knows the offending path shows it.
- The format is byte-exact. Two runs of the same invocation produce the same bytes on stderr (given the same error).
- The format is enforced by golden files. A change to the format that is not accompanied by a regenerated golden file fails CI.
- The format is extensible. A future field can be added as another indented line, or as another block, without breaking the existing structure.
- The `Suggestion:` block is preserved from the previous format. Users' muscle memory is honored; the block's label and its two-space indent are unchanged.
- The format is a single contract. Both the human reader and the script consumer see the same shape; there is no TTY-dependent bifurcation.

### 4.2 Negative

- The format is longer than the previous one. A simple error without context or remediation takes two lines (`Error:` and `Code:`) instead of one.
- The twelve byte-level rules are numerous. A contributor who adds a field must know the column positions; the golden files catch the mistakes, but the rules are not trivially memorable.
- The format is not a standard. A user who is familiar with a different CLI's error output has to learn Forge's. The shape is idiomatic but is not universal.
- The format's byte-exactness means that a cosmetic change (an extra space, a different indent) is a breaking change. A reviewer must treat any change to `format.go` as a contract change, not a formatting tweak.
- The format's `Suggestion:` block is rendered only when the remediation is non-empty. An error without a remediation is one line shorter; a caller that wants to identify "errors without remediation" cannot do so by line count, because the block's absence and the presence of a context entry could coincide.

### 4.3 Neutral

- The format's `Code:` line is always rendered, even when the Code does not directly help the user. The Code is a stable identifier; it is what a user quotes and what a script matches on. Omitting it when it "does not help" would create two shapes, and the shape's predictability is worth the extra line.
- The format's context keys are sorted alphabetically. The sort is what makes the output deterministic; Go's map iteration order is randomized. The sort is O(N log N) with N typically 0–3; the cost is negligible.
- The format's `Options` struct is empty in Phase 2. The struct exists so that a future phase can add fields (colour policy, terminal width, redaction) without changing `Format`'s signature. The empty struct is a placeholder, not a feature.
- The format's fallback for a non-`*forgeerr.Error` input is a single line: `Error: <msg>\n`. The fallback is a safety net; every error path in Forge should produce a `*forgeerr.Error`. The fallback exists so that a future bug that leaks a plain error produces a readable diagnostic rather than an empty string.

---

## 5. Related Requirements

- `ERR-004` — The user-facing error format. This ADR is the format.
- `ERR-001` — The canonical error type. The format renders the type's fields; the type defines what can be rendered.
- `ERR-002` — The error code taxonomy and stability policy. The format's `Code:` line renders the Codes that the registry defines.
- `SEC-003` — The redaction policy. The format applies the redaction policy to context values; a secret-typed context value is rendered as `<redacted>`.
- `UX-CLI-005` *(pending)* — The format is documented in `docs/cli-ux-spec.md` and enforced by golden files. Must be added to the register when WBS 1.4 is complete.
- `NFR-TEST-005` *(pending)* — The format has a dedicated golden-file test (`format_test.go`). Must be added to the register when WBS 1.4 is complete.

---

## 6. Related ADRs

- `ADR-011` — Closed error category taxonomy. The format renders the Code, whose prefix is the Category's prefix. The two ADRs are independent but a reader of one benefits from reading the other.
- `ADR-012` — Error code registry and stability policy. The format renders the Code; the registry defines the Codes the format may render.
- `ADR-013` — Error wrapping contract. The format is applied to the `*Error` that `errors.As` extracts from a wrapped chain; the wrapping contract is the extraction mechanism.
- `ADR-015` — Programmer-facing error format. The two formats are separate; the user-facing format is for humans, the log-facing format is for machine consumers. A reader of one benefits from reading the other.
- `ADR-016` — Error redaction policy. The format applies the redaction to context values; the policy defines what is redacted and how.
- `ADR-017` — Error verbose modes. The format's verbose mode (enabled by `--verbose`) renders additional blocks and relaxes one rule; the mode matrix is defined in that ADR.

---

## 7. Notes

### Open questions deferred

- **Should the format render the Category as a separate line?** The Category is derivable from the Code's prefix; a separate `Category:` line would duplicate the information. A future ADR could add the line if a user-facing use case emerges.

- **Should the format render the remediation with a different label?** The label is `Suggestion:`. A future ADR could rename it, but the label is part of the frozen contract and a change would require an ADR.

- **Should the format support a `--format json` mode for the error output?** The user-facing format is currently the only shape for stderr. A machine-readable mode would be a separate contract, likely introduced when a use case requires it (for example, a CI integration that wants the Code as a field).

- **Should the format's `Error:` prefix be configurable?** The prefix is frozen. A configurable prefix would bifurcate the contract; a caller that wants a different prefix uses `--format json` or wraps the output itself.

### Follow-up decisions anticipated

- **WBS 10.4.2** (programmer-facing format) renders the same error as a structured log entry. The two formats are complementary; the log entry renders the fields as data, the user-facing format renders them as prose.
- **WBS 10.4.3** (redaction) applies to the format's context values. The format's `redact` and `redactPath` calls are the application points.
- **WBS 10.4.4** (verbose modes) extends the format with additional blocks. The format's `Options.Verbose` field selects the mode; the additional blocks are inserted between the context and the suggestion.
- **WBS 10.5.1** (boundary integration) routes every error through the format. The boundary's rendering is the format's only caller in Phase 2.

### External references

- The `grep`-ability of the `Code:` line was a consideration; a script that wants to find every `FORGE_CONFIG_*` error can grep for the prefix. The format's `Code:` line is at column 1, which makes the grep trivial.
- The two-space alignment after `Code:` and the seven-space context indent were chosen so that the values align under the message's first character. The alignment was validated against several error shapes before being frozen.
- The `Suggestion:` block's shape (label on its own line, two-space indent for the body) is consistent with the WBS 7.3.2 shape; the block was preserved to honor existing users' expectations.
