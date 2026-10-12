# ADR-012: Error code registry and stability policy

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge emits errors that carry a machine-readable Code: a stable string of the form `FORGE_<CATEGORY>_<SPECIFIC>`. The Code is what a script matches on to branch on a specific failure, what a bug report quotes to identify the exact path that produced the error, and what a log consumer indexes to count errors by kind. Code identity is therefore part of Forge's public surface in a way that error messages are not: a message is prose that may be reworded between releases, but a Code is an identifier that must remain stable for the lifetime of the version series.

Before the registry, the set of Codes was implicit. Each error path constructed its Code from a string literal at the call site, the shape was checked by the Code regex in the constructor, and the only guarantee of stability was that nobody had renamed an existing Code yet. The set was not enumerable: a tool that wanted to list every Code Forge could emit had to grep the source tree, and a consumer that wanted to know whether a Code was valid had to guess from the shape. The invariant "codes are stable" was a promise without an enforcement mechanism.

The WBS 10.3.1 task is the freeze. It supersedes the informal Code set from WBS 10.1.1 and defines a closed registry in `internal/forgeerr/codes.go`. The registry is the single source of truth for the set of Codes Forge emits; the constructor rejects any Code that is not a member. The task's inputs are WBS 10.2.1 (the taxonomy, which determines each Code's category and exit code by prefix) and WBS 8.5.1 (the config codes, which are the largest family in Phase 2). The affected parties are Forge Core (who maintain the registry), every command author (who picks a Code when constructing an error, or adds one), and every downstream consumer (scripts, CI, log aggregators, bug-report templates).

The consequence of not deciding is that the "codes are stable" promise remains unenforceable. A typo in a Code's tail would produce a new Code that no consumer recognises; a reworded message with the same Code would silently change the semantics a consumer relies on; a Code that a script matched on could be removed in a refactor and the script would fail at runtime, not at compile time. The registry turns the promise into a check.

---

## 2. Options Considered

### Option A — No registry; rely on the shape regex

Keep the current approach: validate each Code against `^FORGE_[A-Z][A-Z0-9_]*$` in the constructor, and trust the source tree to use sensible Codes. Stability is a convention documented in a specification.

**Pros:**

- No new artifact to maintain.
- Adding a Code is a single-line change at the call site.
- The regex catches the most common mistakes (lowercase, missing prefix, malformed shape).

**Cons:**

- The regex characterises shape, not membership. A typo such as `FORGE_CONFIG_MALFORM` (missing `ED`) is a perfectly well-formed string and a completely wrong Code, and the regex accepts it.
- The set is not enumerable; tooling cannot list the Codes or check whether one is valid.
- The stability promise is unenforceable. A Code can be renamed in a refactor without any signal, and no test catches the change.
- Two contributors can invent two Codes for the same logical failure, and nothing detects the duplication.
- Documentation drifts. A Code mentioned in `docs/` may not exist; a Code that exists may not be documented.

### Option B — Registry as a list of Patterns

Define the registry as a list of regex patterns rather than a list of specific Codes. A Code is valid if it matches any pattern in the list.

**Pros:**

- Adding a new Code is often possible without a registry edit (a pattern may cover it).
- The list of patterns is shorter than the list of Codes.

**Cons:**

- Patterns are weaker than specific Codes. A pattern that accepts `FORGE_CONFIG_*` accepts `FORGE_CONFIG_TYPO` and `FORGE_CONFIG_FOO`; the registry no longer guarantees that only specific Codes are valid.
- Stability becomes a pattern-stability promise, which is weaker: adding a new Code under an existing pattern is not visible in the diff.
- Documentation cannot be generated from patterns without listing the Codes they cover, which reintroduces the enumeration problem.
- The pattern-based approach is what most CLI tools use implicitly; it is the approach the current state already approximates, and it is the approach the WBS is asking to replace.

### Option C — Closed registry of specific Codes, with a stability policy

Enumerate every Code Forge emits in `internal/forgeerr/codes.go` as a member of a map. The constructor rejects any Code that is not in the map. The registry entries carry metadata (introduced-phase, deprecation flag). The stability policy is documented in an ADR and enforced by the registry's closed-set property.

**Pros:**

- The set is enumerable. Documentation, tooling, and tests all read the same registry.
- The regex and the registry check are complementary: the regex catches shape errors, the registry catches membership errors. Neither subsumes the other.
- The stability promise is enforced. Renaming a Code requires removing one entry and adding another, which is visible in the diff; the golden-file test fails if the registry and the documentation drift.
- Deprecation is representable. A Code that is superseded remains in the registry with a `deprecated: true` flag, so historical logs and scripts continue to resolve.
- The registry's closure is what makes "adding a Code requires an ADR" enforceable: a PR that adds a Code without an ADR is visible in review because the ADR is a required artifact.

**Cons:**

- Adding a Code requires editing the registry, regenerating the golden file, and updating the documentation. The coordination cost is real.
- The registry can grow large; Phase 2 has 19 Codes, and later phases will add more. The maintenance burden is proportional to the number of Codes.
- The registry duplicates information that is already implicit in the Code's prefix (the category is derivable from the prefix, so the registry does not need to record it). The duplication is minimal but exists.

### Option D — Registry as a list of specific Codes, without stability policy

Enumerate the Codes but do not record a stability policy. The registry is a validation aid, not a contract.

**Pros:**

- Same enumeration benefits as Option C.
- Fewer constraints on future changes; Codes can be renamed freely because no policy forbids it.

**Cons:**

- Loses the primary benefit: consumers cannot rely on Codes being stable, so scripts and log aggregators cannot match on them.
- The registry becomes a list of currently-valid Codes rather than a contract, which is a weaker artifact than the WBS requires.
- The "codes are stable" requirement in the WBS is not satisfied.

**This option was rejected before the ADR was written; it is listed here for completeness.**

---

## 3. Decision

The registry is **Option C**: a closed, enumerable set of specific Codes in `internal/forgeerr/codes.go`, with a stability policy enforced by the registry's closed-set property and by the golden-file test that keeps the registry and the documentation in sync.

The balance tipped toward Option C for three reasons. First, the regex alone cannot catch the class of error the registry exists to prevent: a well-formed but wrong Code. The regex and the registry check are complementary, and only the registry check catches a typo in the Code's tail. Second, the stability promise is the point of the ADR; a registry without a stability policy is a list, not a contract, and the WBS's KPI ("zero codes renamed after shipping") is unmeasurable without the policy. Third, the enumeration property is what makes the documentation, the tooling, and the tests consistent: all three read the same source, and the golden-file test catches any drift.

The trade-offs accepted are: the coordination cost of adding a Code (registry edit, golden-file regeneration, documentation update); the maintenance burden proportional to the number of Codes; and the partial duplication of the category information that the prefix already encodes (the registry does not duplicate the category itself, but the registry's keys are Codes whose prefixes carry the category, and a reader who wants to know a Code's category consults `Category.Prefix` rather than the registry). The coordination cost is the mechanism by which the registry stays stable: a change that adds or renames a Code is expensive enough that it is reviewed.

The decision assumes that the set of Codes is finite and known at each phase. If a future phase introduces a Code that is not enumerable at design time (for example, a Code derived from a user-supplied string), the registry would not be the right home for it, and the ADR for that phase would introduce a separate mechanism. No such requirement exists in the current plan.

---

## 4. Consequences

### 4.1 Positive

- Every Code Forge can emit is a member of the registry. The constructor rejects any Code that is not a member, so a typo in a Code's tail is caught at construction time, in the developer's test, rather than at runtime in a user's script.
- The set is enumerable. `AllCodes()` returns the Codes in sorted order; the golden file `internal/forgeerr/testdata/codes/codes.golden.txt` records the current set; the reference documentation is generated from the same source. A Code that is declared but not registered, or registered but not documented, fails a test.
- The stability policy is enforceable. Renaming a Code is a diff that removes one registry entry and adds another; the diff is visible in review, and the ADR that authorises the change is a required artifact.
- Deprecation is representable. A Code that is superseded remains in the registry with a `deprecated: true` flag. Historical logs and scripts continue to resolve; the formatter can surface a deprecation warning if a future phase chooses to.
- The category and exit code are derivable from the Code's prefix, so the registry does not duplicate them. The registry's only metadata is the introduced-phase and the deprecation flag, both of which are facts that cannot be derived.

### 4.2 Negative

- Adding a Code requires an ADR, an edit to `codes.go`, a regenerated golden file, and an updated reference document. The four artifacts must be in the same commit; the sync test fails otherwise. The coordination cost is the mechanism by which the registry stays stable, but it is a cost.
- The registry's size grows with each phase. Phase 2 has 19 Codes; later phases will add more. The maintenance burden is proportional to the number of Codes, though the golden-file test automates the mechanical parts.
- The registry is a second place where the Code's shape is constrained. The regex in `forgeerr.go` constrains the shape; the registry constrains the membership. A reader who wants to know "is this Code valid" must consult both. The two are complementary, but the two-place constraint is a small cognitive cost.
- Deprecation is advisory in Phase 2. A deprecated Code is accepted by the constructor; no warning is emitted. A future phase may add a warning, but until then, a caller that uses a deprecated Code receives no signal that the Code is deprecated.

### 4.3 Neutral

- The registry's keys are Codes, and the value is a `codeSpec` struct carrying the metadata. The struct is unexported; the registry is a package-level map. The shape is stable but is not part of the public API; a future refactor that changes the internal representation is not a breaking change to consumers.
- The registry is sorted before being returned by `AllCodes()`, so the iteration order is deterministic. The sort is O(N log N) with N = 19 in Phase 2; the cost is negligible.
- The registry's entries carry an `introduced` string ("Phase 2", "Phase 4", ...). The string is documentation; no behaviour depends on it. A future phase that wants to gate a Code by phase could use the field, but the current code does not.

---

## 5. Related Requirements

- `ERR-002` — The error code taxonomy and stability policy. This ADR establishes the registry that ERR-002 references.
- `EXIT-002` — The exit-code mapping. This ADR's registry entries do not carry exit codes; the codes are derived from the category, which is derived from the prefix. EXIT-002 is satisfied by the taxonomy ADR (ADR-011), not by this one.
- `SEC-003` — The redaction policy. The redaction policy applies to context values, not to Codes; the two requirements interact only in that a Code's prefix may appear in a redacted context value, which is not redacted (the prefix is not user data).
- `FR-CORE-008` *(pending)* — The error code registry is the single source of truth for the set of Codes. Must be added to the register when WBS 1.4 is complete.
- `NFR-MAINT-003` *(pending)* — Adding a Code requires a coordinated edit to four artifacts. The requirement is a maintenance constraint; the ADR is where the constraint is established.

---

## 6. Related ADRs

- `ADR-011` — Closed error category taxonomy. This ADR depends on ADR-011: the registry's Codes have prefixes that are the categories defined there. The two ADRs are frozen together; a change to one usually implies a change to the other.
- `ADR-016` — User-facing error format. The format renders the Code in the `Code:` line. This ADR establishes the Codes the format may render.
- `ADR-017` — Programmer-facing error format. The log format renders the Code as a structured field (`error.code`). The relationship is the same as with ADR-016: this ADR establishes the Codes; that ADR establishes how they are rendered.

---

## 7. Notes

### Open questions deferred

- **Should the registry be exposed as a public API?** The registry is unexported; `AllCodes()`, `Lookup()`, and `IsRegistered()` are the public surface. A future phase that wants to expose the registry as a machine-readable catalog (for example, as a JSON file) can generate it from `AllCodes()` without changing the package's API.

- **Should deprecated Codes emit a warning?** The registry records deprecation, but the constructor does not act on it. A future phase could emit a warning when a deprecated Code is used, but the warning would be a behavioural change and requires its own ADR.

- **Should Codes be organized into families?** The registry's Codes are not grouped by family (config, filesystem, security, ...). The grouping is implicit in the prefix; a future phase that wants to expose the families could add a `Family` field to `codeSpec` without changing the Code's string value.

- **Should the registry be generated from a specification file?** The registry is currently hand-written in Go. A future phase could move it to a YAML file and generate the Go code, but the current size (19 Codes) does not justify the tooling.

### Follow-up decisions anticipated

- **WBS 10.4.1** (user-facing format) will render the Code in the `Code:` line. This ADR establishes the Codes; the format is a separate freeze.
- **WBS 10.4.2** (programmer-facing format) will render the Code as a structured log field. The relationship is the same.
- **WBS 10.5.1** (boundary integration) will read the Code's category to derive the exit code. This ADR establishes the Codes; the boundary reads the category, not the Code, for the exit code.

### External references

- The GNU Coding Standards' rule that "the exit status should be zero on success and non-zero on failure" is the floor; Forge's exit-code contract is a refinement that distinguishes failure categories.
- `sysexits.h`'s `EX_SOFTWARE = 70` was considered for the `INTERNAL` category's exit code and rejected in favour of `ExitFailure = 1`, which is the more portable convention.
- The "Semantic Versioning" specification's rules for breaking changes informed the stability policy: a Code that is renamed is a breaking change to consumers, and the version series' contract is that Codes are stable within a major version.
