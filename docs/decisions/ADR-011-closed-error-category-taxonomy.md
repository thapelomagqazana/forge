# ADR-011: Closed Error Category Taxonomy

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge emits errors from many sources: CLI parsing, configuration loading, filesystem operations, schema and blueprint validation, template rendering, and internal invariants. Each source has its own vocabulary for what went wrong, and each vocabulary suggests a different exit code for a script to branch on. Without a shared classification, the CLI would have to decide exit codes case by case, every command would be tempted to invent its own codes, and a script author would have no way to know which exit codes to expect from a given invocation.

The WBS 10.1.1 task introduced a `Category` type on `*forgeerr.Error` to give every error a single classification. The initial type was `uint8`-backed with five members: `CategoryUsage`, `CategoryConfig`, `CategoryValidation`, `CategoryIO`, and `CategoryInternal`. The type was sufficient for the first set of errors but did not cover three scopes the later WBS items required: security (path traversal, symlink escape, unsafe templates), templates (a Phase 4 concern that needs its own scope so that errors are classified before the subsystem ships), and the CLI's own usage errors as a distinct category from the catch-all "usage". Additionally, the `uint8` representation leaked numeric values into serialized forms (log output, JSON error envelopes), and inserting a new member between existing ones required renumbering, which was a footgun that would eventually cause a code to be misclassified.

The WBS 10.2.1 task is the freeze. It supersedes the placeholder from 10.1.1 and defines the taxonomy that every subsequent phase consumes. The task's inputs are WBS 4.1.2 (exit codes) and WBS 8.5.1 (config errors); its outputs are `internal/forgeerr/category.go`, `internal/forgeerr/category_test.go`, and `docs/reference/error-categories.md`. The affected parties are the Forge Core Team (who maintain the taxonomy), every command author in Phases 2–21 (who choose a category when constructing an error), and every script author or CI system that branches on the exit code.

The consequence of not deciding is that the taxonomy would grow by accretion: each new error would pick whatever category looked reasonable locally, the aggregate would drift, and the exit-code mapping would become impossible to document. The freeze is the mechanism by which the aggregate is reviewable.

---

## 2. Options Considered

### Option A — Open taxonomy, string-backed

Define `Category` as a `string` type and accept any non-empty string as a category. Each category's prefix and exit code are derived by convention from the string's first segment.

**Pros:**

- No central list to maintain; adding a category is a local change.
- String categories serialize naturally.
- No renumbering when a new category is inserted.

**Cons:**

- No way to guarantee that every error has a well-defined exit code: an unknown category would have to fall back to a default, which hides misclassifications.
- No way to enumerate the set for documentation, validation, or tooling.
- Two contributors could invent two spellings of the same category, and the codebase would carry both.
- The exit-code mapping could not be a `switch` over a closed set; it would have to be a lookup with a fallback.

### Option B — Closed taxonomy, `uint8`-backed

Keep the `uint8` representation from WBS 10.1.1 and extend the set to the seven members the WBS requires. Bound-check `valid()` numerically; use a sentinel `categoryMax` for the upper bound.

**Pros:**

- Compact representation.
- Fast membership check.
- Familiar from the standard library's `syscall.Errno` pattern.

**Cons:**

- Numeric values leak into serialized forms; a log or JSON consumer sees `2` rather than `CLI`.
- Inserting a member between existing ones requires renumbering every member after it, which is a footgun that a future contributor will eventually get wrong.
- The sentinel `categoryMax` has to be maintained alongside the set; forgetting to update it when a member is added produces silent misclassification.
- The `String()` method is a `switch` over numeric values, which reads as a lookup table rather than a description.

### Option C — Closed taxonomy, string-backed

Define `Category` as a `string` type, enumerate the seven members as constants, expose the closed set via `AllCategories()`, and validate membership by scanning the set. Each category carries its Code prefix and exit code as method behaviour on the type.

**Pros:**

- String values serialize readably into logs and JSON (`"CONFIG"` rather than `2`).
- No renumbering when the set changes; the string is the identity.
- The membership check is a scan over a canonical slice, which cannot drift from the constants because the constants are the slice.
- The prefix and exit code are methods on the category, which means there is exactly one place to change each.
- The set is enumerable for documentation, tests, and tooling.

**Cons:**

- Slightly larger in-memory representation than `uint8` (a string header versus a byte).
- The membership check is a linear scan rather than a comparison; for seven members this is faster than a map lookup in practice.
- The `String()` method has to handle the invalid case explicitly (return `"unknown"`) rather than relying on a `default:` case in a numeric `switch`.
- Adding a member is an edit to `category.go`, `category_test.go`, and `docs/reference/error-categories.md`, plus an ADR; the coordination cost is real.

### Option D — No `Category` type; encode the category in the Code prefix

Remove the `Category` type entirely. Derive the category from the Code's prefix when a caller needs it, and derive the exit code from the prefix by a lookup table.

**Pros:**

- One fewer concept in the error type.
- The Code is already required to carry the category as its prefix, so the derivation is mechanical.

**Cons:**

- Every consumer that needs the exit code would have to parse the prefix and consult a table, which duplicates the derivation at every call site.
- The table would live somewhere, and the "somewhere" would be one of the consumers, which couples the consumer to the taxonomy.
- The prefix-to-category derivation is exactly what the `Category` type was introduced to encapsulate; removing the type moves the logic without removing it.

**This option was rejected before the ADR was written; it is listed here for completeness.**

---

## 3. Decision

The taxonomy is **Option C**: a closed, string-backed `Category` type with seven members (`CLI`, `CONFIG`, `FILESYSTEM`, `VALIDATION`, `TEMPLATE`, `SECURITY`, `INTERNAL`). Each member carries its Code prefix and process exit code as method behaviour. Membership is enforced by a scan over a canonical slice; a value that is not in the slice is not a category. Adding a member requires an ADR.

The balance tipped toward Option C for three reasons. First, the exit-code mapping must be a `switch` over a closed set: a lookup with a fallback would allow a misclassification to silently produce a default code, and the default would hide the bug. Second, the string representation is what every downstream consumer sees; a log entry that says `"category":"CONFIG"` is legible, and a JSON envelope that says `"category":2` is not. Third, the "no renumbering" property is what makes future additions safe: a contributor who adds a member does not have to audit every existing value for off-by-one errors.

The trade-offs accepted are: a slightly larger in-memory representation than `uint8` (irrelevant at the CLI boundary, where errors are constructed once per process); a linear membership check (faster than a map at this size); and the coordination cost of adding a member (three files plus an ADR). The last is the point: the coordination cost is the mechanism by which the set stays closed.

The decision assumes that the seven members are the right set for Phases 2 through 21. If a future phase introduces a scope that does not fit an existing member, the ADR for that phase will amend this one. The decision is not provisional; it is a freeze, and the amendment procedure is the safety valve.

---

## 4. Consequences

### 4.1 Positive

- Every error has exactly one category, and every category has exactly one Code prefix and one exit code. The mapping is total and unambiguous.
- The set is enumerable. `docs/reference/error-categories.md` is generated from `AllCategories()`, so a category that is declared in code but missing from the documentation is impossible.
- The category's prefix is the single source of truth for the Code's shape. The constructor's prefix check (WBS 10.1.2, AC7) consults `Category.Prefix()`, and no parallel table exists.
- A script author can enumerate the exit codes by reading one file (`docs/reference/error-categories.md`) and can rely on the values being stable across releases.
- The `String()` method is total: an invalid category returns `"unknown"` rather than panicking, so a diagnostic that logs a zero-value category does not crash.

### 4.2 Negative

- Adding a category requires an ADR, an edit to `category.go`, a test update in `category_test.go`, an edit to the exit-code constants in `internal/cli/exitcodes.go`, and an update to `docs/reference/error-categories.md`. The coordination cost is real and is borne by whoever adds the category.
- The membership check is a linear scan. For the current seven members, the scan is faster than a map lookup and does not allocate. If the set grows beyond roughly twenty members, the scan becomes slower than a map, and the implementation should switch. The switch is a local change to `valid()`; the call sites do not change.
- The category is a separate concept from the Code, even though the Code's prefix encodes the category. A reader who sees `FORGE_CONFIG_MALFORMED` must know that `FORGE_CONFIG_` maps to `CategoryConfig`; the mapping is documented but is not visible at the call site. The redundancy is deliberate (the prefix is machine-readable, the category is human-readable), but it is redundancy.
- A category whose scope overlaps another's requires a boundary rule in the ADR. `FILESYSTEM` and `SECURITY`, for example, both concern paths, and the boundary rule (deliberate boundary violation versus ordinary I/O failure) is a policy decision that the ADR records.

### 4.3 Neutral

- The `TEMPLATE` category is declared but has no registered codes in Phase 2. The category exists so that a Phase 4 error can be classified without a taxonomy change. The category is not "unused"; it is "declared but not yet exercised", and the distinction is documented in the CLI UX specification's "Pending categories" section.
- The `VALIDATION` category has the same status: declared, pending its first code.
- The taxonomy's members are ordered by scope, not by severity or frequency. The ordering is stable (the tests pin it) but is not semantic; a reader should not infer that `CLI` is more severe than `CONFIG` because it appears first.

---

## 5. Related Requirements

- `ERR-002` — The error code taxonomy and its stability rules. This ADR establishes the taxonomy that ERR-002 references.
- `EXIT-002` — The exit-code mapping. This ADR defines the categories whose exit codes EXIT-002 freezes.
- `FR-CORE-007` *(pending)* — The `*forgeerr.Error` type's category field is validated against the closed set. Must be added to the register when WBS 1.4 is complete.
- `UX-CLI-004` *(pending)* — The exit codes are documented in `docs/cli-ux-spec.md` and referenced by scripts. Must be added to the register when WBS 1.4 is complete.

---

## 6. Related ADRs

- `ADR-001` — Forge as a foundation manager. The taxonomy's `FILESYSTEM` and `SECURITY` categories exist because a foundation manager manipulates the filesystem and enforces boundaries; the decision that Forge is a foundation manager is prior to the decision about how its errors are classified.
- `ADR-008` — YAML library choice. The `CONFIG` category's `malformed YAML` code is produced by the YAML library; the choice of library constrains the diagnostics that category's codes must cover.
- `ADR-016` — User-facing error format. The format renders the category indirectly through the Code's prefix; the two ADRs are independent but a reader of one benefits from reading the other.

---

## 7. Notes

### Open questions deferred

- **Should `FILESYSTEM` and `SECURITY` be merged?** The two categories overlap: both concern paths. The current boundary is deliberate-versus-ordinary: a path that escapes the project root because a user supplied it is `SECURITY`; a path that escapes because the program computed it from a symlink is `FILESYSTEM`. If a future phase finds the boundary ambiguous, an ADR can merge them; the merge would be a breaking change to the exit-code contract (the merged category would have to choose one of the two exit codes).

- **Should `VALIDATION` and `TEMPLATE` be merged?** Both are Phase 4 concerns. The current split is schema-versus-rendering: a blueprint that fails its schema check is `VALIDATION`; a template that fails to resolve is `TEMPLATE`. If Phase 4 finds the boundary blurry, an ADR can merge them.

- **Should the taxonomy grow a `NETWORK` category?** Phase 2 through Phase 21 do not fetch anything over the network; the `Registry` module (Phase 16) will. The category should be added by the ADR that introduces the registry, not preemptively.

### External references

- The standard library's `syscall.Errno` type is the closest precedent in the Go ecosystem. It is `uint8`-backed and uses a numeric `switch` for `Error()`; this ADR departs from the precedent because the string representation is more important for a CLI's user-facing output than for a syscall's return value.
- The `no-color.org` convention's influence on error categorization is nil; the reference is included only because the "no colour" rule (WBS 7.4.2) and the "closed taxonomy" rule (this ADR) both concern the discipline of freezing a user-facing contract.

### Follow-up decisions anticipated

- **WBS 10.3.1** (error code registry) will define the codes that belong to each category. This ADR establishes the categories; the codes are a separate freeze.
- **WBS 10.4.1** (user-facing format) will render the code and the context. The category is rendered indirectly through the code's prefix; this ADR does not prescribe the format.
- **WBS 10.5.1** (boundary integration) will map errors to exit codes by reading `Category.ExitCode()`. This ADR establishes the method; the boundary is a separate freeze.
