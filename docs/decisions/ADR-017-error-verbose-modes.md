# ADR-017: Error verbose modes

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's user-facing error format (ADR-014) renders the message, the Code, the context entries, and the optional `Suggestion:` block. The format is frozen and is the same for every invocation. For a user who runs Forge interactively, the format is sufficient: it names the failure, quotes the Code, and suggests a remediation. For a developer who is debugging a failure — particularly a failure that reached the user through a chain of wrapped errors, a failure whose cause is in a different module, a failure whose context includes an `internal_`-prefixed diagnostic — the format is silent. The cause is not rendered; the stack is not available; the internal context is filtered out; the home directory's path is redacted to `~`, which hides the location of the file that triggered the failure.

The gap between the two audiences is the gap between a user and a developer. A user wants the shortest path to a fix; a developer wants the deepest insight into the failure. A single format cannot serve both. The format that serves the user hides the cause (it is a programmer's concern), hides the stack (it is noise), filters the internal context (it is not actionable), and redacts the home directory (it is private). The format that serves the developer shows all four. The two formats are the same for the first three lines — the message, the Code, and the context — and diverge after that.

The WBS 10.4.4 task is the freeze. It defines the two modes, the elements that change between them, and the rule that secret redaction is unchanged. The task's inputs are WBS 10.4.1 (the user-facing format, which the verbose mode extends), WBS 10.4.2 (the log format, which is separate from the verbose mode), WBS 10.4.3 (the redaction policy, which the verbose mode relaxes for paths but not for secrets), and WBS 5.3.1 (the `--verbose` global flag). The affected parties are every user who passes `--verbose`, every developer who debugs a failure, and the security review that confirms the mode does not leak secrets.

The consequence of not deciding is that `--verbose` has no defined effect. The flag is registered, the global flag interaction matrix (WBS 7.4.3) mentions it, and the log level is set to debug — but the *error* format does not change. A developer who passes `--verbose` to see the cause chain sees the same error block as a user who did not. The flag's value for debugging is limited to the log output, which is not where the developer looks when an error is on the screen. The freeze is the mechanism by which the flag affects the error's rendering.

---

## 2. Options Considered

### Option A — No verbose mode; the format is the same for every invocation

Do not extend the error format for `--verbose`. The flag affects only the log level; the error's rendering is the same in both modes.

**Pros:**

- The format is a single contract; there is no mode matrix to document or test.
- The rendering is simpler; a contributor has one shape to maintain.
- The verbose flag's effect is confined to the logging subsystem, which is a single place.

**Cons:**

- The flag is less useful. A developer who passes `--verbose` for a diagnostic sees no additional information in the error block.
- The cause chain is invisible to the developer who is looking at the error. The developer must dig into the log output to find the cause, which is a second step.
- The `internal_`-prefixed context entries are filtered in both modes. The developer cannot see the diagnostics that were deliberately added to the error.
- The home-directory redaction is applied in both modes. The developer sees `~/config.yaml`, which hides the file's location; a developer debugging a path-resolution failure cannot see where the path resolved.
- The stack trace is not available. A developer who wants to know the call chain that produced the failure has no way to see it in the error block.

### Option B — Verbose mode as a separate error type

Introduce a separate error type (`*VerboseError`) that carries the additional fields, and render it with a different format.

**Pros:**

- The additional fields are typed; the compiler enforces their presence.
- The rendering is separate; the verbose format can diverge from the non-verbose one without a mode matrix.

**Cons:**

- The type proliferation is real. A `*Error` that wants to be rendered in verbose mode must be converted to a `*VerboseError`, and the conversion has to be done by every caller that wants the verbose rendering.
- The two types would have to be kept in sync. A new field on `*Error` would need a corresponding field on `*VerboseError`, and the two would eventually drift.
- The `errors.As` extraction would have to handle both types. A caller that extracts `*Error` from a chain would not find a `*VerboseError`, and vice versa.
- The API surface is larger. Every construction site would have to choose between the two types, which is a decision the type system does not help with.

### Option C — Verbose mode as a flag on `Options`

Extend the `Options` struct with a `Verbose` field. The `Format` function dispatches on the field; when it is true, the function renders three additional blocks (the cause chain, the stack, and the `internal_`-prefixed context entries) and relaxes one rule (the home-directory redaction). The non-verbose rendering is unchanged; a caller that passes the zero-value `Options` sees the same string as before.

**Pros:**

- The rendering is a single function; the dispatch is inside it. A contributor has one place to maintain.
- The additional blocks are rendered conditionally; a non-verbose invocation's output is byte-identical to the pre-WBS-10.4.4 output.
- The `Options` struct is the same boundary that already carries the colour policy and the redaction flag; adding a field does not change the function's signature.
- The `Verbose` flag is read from the command tree after Cobra parses the arguments; the flag's value is the user's, not a default.
- The `internal_`-prefixed context entries are filtered only in the non-verbose mode; the verbose mode renders them. The convention (prefix `internal_` for diagnostics) is what makes the filter possible without a separate field.
- The home-directory redaction is relaxed in verbose mode; the secret redaction is unchanged. The relaxation is scoped to paths; the secret rule is absolute.

**Cons:**

- The `Format` function is longer; it has a branch for each mode. The branch is small (a conditional call to three helper functions) but is a branch.
- The mode matrix is a contract; a future change to either mode requires an ADR. The friction is the point but is a cost.
- The `internal_` prefix is a convention, not a type. A caller who wants a context entry visible in both modes must not prefix it with `internal_`; the convention is documented but is not enforced by the type system.
- The `Stack` field is caller-provided; the package does not capture a stack. A caller that wants a stack has to capture one and pass it; in Phase 2, no caller does, so the `Stack:` block is never rendered. The plumbing is in place but is not exercised.
- The mode matrix includes a rule ("home-directory redaction is relaxed in verbose mode") that is a relaxation of a security-adjacent policy. The relaxation is deliberate (a developer needs the full path), but the relaxation is a change to what the user sees, and the change is silent.

### Option D — Verbose mode as a separate format function

Introduce a separate `FormatVerbose` function alongside `Format`. Callers choose between the two.

**Pros:**

- The two functions are separate; each can be maintained independently.
- The verbose format can diverge from the non-verbose one without a mode matrix.

**Cons:**

- The API surface is larger. Every caller chooses between the two functions; the choice is a decision the type system does not help with.
- The two functions would have to be kept in sync. A change to the format's shared portion (the message, the Code, the context) would have to be applied to both; the two would eventually drift.
- The verbose function's signature is likely the same as the non-verbose one's; the two functions differ only in behaviour, which is the classic case for a flag rather than a second function.
- The `Options` struct already exists as the boundary for future extension; adding a second function duplicates the boundary.

**This option was rejected before the ADR was written; it is listed here for completeness.**

---

## 3. Decision

The mode distinction is **Option C**: `Options.Verbose` is a field on the existing `Options` struct; `Format` dispatches on the field. In verbose mode, the function renders three additional blocks and relaxes one rule. In non-verbose mode, the function is unchanged.

The verbose mode's additions are:

- A `Cause:` block, containing the cause chain reachable via `errors.Unwrap`. Each level is rendered on its own line, indented two spaces.
- A `Stack:` block, containing the caller-supplied stack frames. The block is omitted when no stack is provided.
- The `internal_`-prefixed context entries, which the non-verbose mode filters out.

The verbose mode's one relaxation is:

- Home-directory redaction is relaxed: a context value that is a path under the user's home directory is rendered with the full path. The secret redaction is unchanged; the rule "Secrets are never shown" is absolute in both modes.

The verbose mode's blocks are inserted between the context lines and the `Suggestion:` block, so the output reads:

```text
Error: <message>
Code:  <code>
       <context-key-1>: <value-1>
       <context-key-2>: <value-2>

Cause:
  <cause-1>
  <cause-2>

Stack:
  <frame-1>
  <frame-2>

Suggestion:
  <remediation>
```

The balance tipped toward Option C for three reasons. First, the non-verbose output is unchanged. A caller that passes the zero-value `Options` sees byte-identical output to the pre-WBS-10.4.4 version. This is the property that makes the change safe: every existing caller works without modification, and every existing golden file continues to match. Second, the `Options` struct is the right boundary. The struct already carries the colour policy and the redaction flag; adding the verbose flag is consistent with the struct's purpose. Third, the mode matrix is small. Three additions and one relaxation; the matrix fits in a table, and each row is testable. A larger mode matrix would have argued for a separate function; this one does not.

The trade-offs accepted are: the `Format` function is longer; the mode matrix is a contract; the `internal_` prefix is a convention, not a type; the `Stack` field is caller-provided and is not exercised in Phase 2; and the home-directory relaxation is a silent change to what the user sees. The last is the most significant; it is documented in the ADR and in the CLI UX specification, and the verbose flag's name is the signal that the user has opted into the additional information.

The decision assumes that the two modes are the right decomposition. If a future phase finds a need for a third mode (for example, a "trace" mode with even more information), the ADR for that phase extends the `Options` struct with a new field or replaces the `Verbose` bool with an enumerated `Mode` type. The extension would be an additive change, not a breaking one, because the current `Verbose` field's semantics are a subset of the future mode's.

---

## 4. Consequences

### 4.1 Positive

- The non-verbose output is byte-identical to the pre-WBS-10.4.4 output. Existing callers and golden files are unaffected.
- A developer who passes `--verbose` sees the cause chain, the stack (when available), the `internal_`-prefixed context entries, and the full paths. The additional information is what the developer needs to diagnose the failure.
- The `Options` struct is the same boundary as before; adding the verbose field does not change the function's signature.
- The `Verbose` flag is read from the command tree after Cobra parses the arguments; the flag's value is the user's, not a default.
- The secret redaction is unchanged in both modes. The rule "Secrets are never shown" is absolute; the verbose mode does not weaken it.
- The mode matrix is small and testable. Three additions and one relaxation; each row is covered by a test.
- The verbose blocks are inserted in a fixed order (Cause, Stack, Suggestion). The order is part of the contract; a future change to the order requires an ADR.

### 4.2 Negative

- The `Format` function is longer. The branch on `Options.Verbose` is small but is present; a contributor reading the function sees the two modes interleaved.
- The mode matrix is a contract. A change to either mode requires an ADR; the friction is deliberate but is real.
- The `internal_` prefix is a convention, not a type. A caller who wants a context entry visible in both modes must not prefix it with `internal_`; the convention is documented but is not enforced by the type system. A caller who prefixes a value that should be visible in both modes has made a naming error that the compiler does not catch.
- The `Stack` field is caller-provided. In Phase 2, no caller provides a stack; the `Stack:` block is never rendered. The plumbing is in place but is not exercised by any production path.
- The home-directory relaxation is a silent change to what the user sees. A developer who passes `--verbose` and pipes the output to a log leaks the full path; the leak is not a secret leak (the secret rule is unchanged) but is a privacy leak. The verbose flag's name is the signal that the user has opted into the additional information; a user who did not read the flag's documentation may not realize the path is now visible.
- The cause chain is rendered in full; a chain with many levels produces a long output. The `maxCauseDepth` constant caps the chain at thirty-two levels; a chain longer than the cap is truncated with a diagnostic line. The cap is generous but is a limit; a chain that exceeds it is truncated.
- The `Stack:` block's frames are caller-formatted. The format does not parse, reorder, or truncate the frames. A caller who provides a malformed stack produces malformed output; the format's contract is the frame strings, not their content.

### 4.3 Neutral

- The `Options.Verbose` field is a `bool`. A future phase that wants more than two modes would replace the bool with an enumerated `Mode` type; the replacement is a breaking change to the struct's shape but is additive in the sense that the two-mode behaviour is a subset of the multi-mode behaviour.
- The `Options.Stack` field is a `[]string`. The field is ignored in non-verbose mode; a caller that provides a stack but does not set `Verbose` sees no `Stack:` block. The field's presence is independent of the flag's value; the two are consulted independently.
- The `Cause:` block's rendering uses `errors.Unwrap` to traverse the chain. The traversal is depth-first; a chain that branches (for example, a cause whose `Unwrap` returns multiple errors via a custom implementation) is followed along the first branch. The standard library's `errors.Unwrap` returns a single error; a branching chain is not supported by the current traversal.
- The `internal_` filter is applied before the sort. The filtered keys are removed from the map, and the remaining keys are sorted. The filter's cost is O(N) with N typically small; the sort's cost is O(N log N) on the filtered set.
- The verbose mode's blocks are inserted between the context and the suggestion, not after the suggestion. The placement is deliberate: the `Suggestion:` block is the last block, so a reader who only wants the remediation can read the last block without scanning the diagnostic blocks. A future change to the placement requires an ADR.

---

## 5. Related Requirements

- `LOG-004` — The verbose mode. This ADR is the mode matrix.
- `ERR-005` — The verbose mode's additions to the user-facing format. This ADR is the format.
- `ERR-004` — The user-facing error format. The verbose mode extends the format; the non-verbose format is unchanged.
- `SEC-003` — The redaction policy. The verbose mode relaxes the home-directory rule but not the secret rule; the relaxation is documented in ADR-016's notes and in this ADR.
- `UX-CLI-006` *(pending)* — The verbose mode is documented in `docs/cli-ux-spec.md` and is controlled by the `--verbose` global flag. Must be added to the register when WBS 1.4 is complete.
- `NFR-TEST-008` *(pending)* — The verbose mode has dedicated tests covering each addition and the relaxation. Must be added to the register when WBS 1.4 is complete.

---

## 6. Related ADRs

- `ADR-014` — User-facing error format. The verbose mode extends the format; the non-verbose format is the baseline.
- `ADR-015` — Programmer-facing error format. The log format is separate from the verbose mode; the two are complementary.
- `ADR-016` — Error redaction policy. The verbose mode relaxes the home-directory rule but not the secret rule; the relaxation is a rendering concern, not a policy change.
- `ADR-011` — Closed error category taxonomy. The verbose mode renders the same categories as the non-verbose mode; the taxonomy is unaffected.
- `ADR-012` — Error code registry and stability policy. The verbose mode renders the same Codes as the non-verbose mode; the registry is unaffected.
- `ADR-013` — Error wrapping contract. The verbose mode's `Cause:` block uses `errors.Unwrap`; the wrapping contract defines the chain the block traverses.

---

## 7. Notes

### Open questions deferred

- **Should the stack be captured by the package?** In Phase 2, the stack is caller-provided. A future phase could capture a stack at the error's construction site, but the capture has a cost on every error construction, whether or not the error is rendered in verbose mode. The caller-provided design defers the cost to the caller who wants the stack.

- **Should the `internal_` prefix be a type?** In Phase 2, the prefix is a convention. A future phase could introduce a `Visibility` enum on context entries, but the enum would be a larger change than the convention and is not required by any Phase 2 use case.

- **Should the home-directory relaxation be opt-in separately?** In Phase 2, the relaxation is tied to `--verbose`. A future phase could introduce a separate flag (for example, `--verbose-paths`) to decouple the path visibility from the diagnostic blocks. The decoupling would be additive; the current behaviour is a subset.

- **Should the verbose mode's blocks be configurable?** In Phase 2, the blocks are fixed. A future phase could introduce per-block flags (for example, `--verbose-cause`, `--verbose-stack`), but the flags would proliferate and would complicate the mode matrix. The current design's single flag is the pragmatic choice.

- **Should the cause chain's depth be configurable?** The `maxCauseDepth` constant caps the chain at thirty-two levels. A future phase could raise the cap or make it configurable; the change would be a local edit to the constant, not a contract change.

### Follow-up decisions anticipated

- **WBS 10.5.1** (boundary integration) reads the verbose flag from the command tree and passes it to the format. The mode's dispatch is in the format; the boundary is the caller.
- **WBS 12.0** (logging) consumes the log format. The log format is separate from the verbose mode; the logging subsystem's configuration is independent of the `--verbose` flag's effect on the user-facing format.
- **WBS 10.5.x** (future) may capture a stack at the error's construction site and pass it through `Options.Stack`. The plumbing is in place; the capture is the future work.

### External references

- The `slog` package's treatment of structured logging influenced the mode distinction: the log format (ADR-015) is the data-oriented rendering, the verbose mode is the human-oriented rendering. The two are complementary, not competing.
- The GNU `grep --debug` and `curl --verbose` flags are precedents for the "additional diagnostic output on stderr" pattern. The pattern's convention is that the flag enables additional output; the additional output is scoped to the diagnostic channel (stderr), not the result channel (stdout).
- The Unix convention that "diagnostics go to stderr" is the reason the verbose mode's blocks are written to stderr, not stdout. The convention is documented in the CLI UX specification's "Output Stream Boundary" section.
