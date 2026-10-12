# ADR-013: Error wrapping contract

**Status:** Accepted
**Date:** 2026-10-12
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's errors flow through the standard library's error-chain protocols. Callers match on them with `errors.Is` and extract them with `errors.As`. Both functions rely on the error's `Unwrap`, `Is`, and `As` methods to traverse the chain. The `*forgeerr.Error` type has implemented those three methods since WBS 10.1.1, but the semantics were described informally: the doc comments said the methods matched "by code" and that `As` "assigned the error", and the tests verified a handful of cases without pinning the exact contract.

The informal description allowed an implementation that was correct for the common cases and wrong for the edges. The pre-WBS-10.3.2 `Is` method, for example, used `errors.As` internally to look for a `*Error` inside the target's chain. That worked when the target was a bare `*Error` or a plain error, but it made `Is` traverse the *target's* chain — which is not what `Is` should do. The standard library's `errors.Is` already handles traversal of the receiver's chain; a per-link `Is` that also traverses the target's chain does redundant work and produces surprising results when the target is a wrapper around a `*Error`. The discrepancy was not caught by the tests because no test constructed a target that was a wrapper.

The WBS 10.3.2 task is the freeze. It supersedes the informal description from the original WBS 10.3 and specifies the exact semantics of `Unwrap`, `Is`, and `As` on `*Error`, including the interaction with `errors.Is`, `errors.As`, `fmt.Errorf("%w", ...)`, and `errors.Join`. The task's inputs are WBS 10.1.1 (the type) and WBS 10.1.2 (the constructors). The affected parties are every error-handling call site: the CLI's error path, every command's error inspection, every test that matches on an error, and the standard library's traversal engine, whose behaviour the contract shapes.

The consequence of not deciding is that the contract remains a comment. A future change to `Is` — a reasonable-looking optimisation, a well-intentioned extension — would not fail any test, and the standard library's traversal would produce different results. The freeze is the mechanism by which the per-link semantics are pinned and the traversal is delegated to the standard library.

---

## 2. Options Considered

### Option A — No contract; rely on the standard library's defaults

Remove the `Unwrap`, `Is`, and `As` methods from `*Error`. Let the standard library's `errors.Is` and `errors.As` treat the error as a plain wrapper: `Unwrap` is not implemented, so the chain does not traverse; `Is` and `As` are not implemented, so the standard library's default comparison is used.

**Pros:**

- The error type is simpler; fewer methods to maintain and document.
- No risk of the methods behaving inconsistently with the standard library's expectations.
- The type is a "plain" error, which is a well-understood shape.

**Cons:**

- `errors.Is` on two independently-constructed `*Error` values with the same Code would return `false` (the default comparison uses pointer identity). Sentinel-style matching would require exporting global sentinel values, which the project has decided against.
- `errors.As` on a wrapped `*Error` would extract the outer wrapper, not the inner `*Error`. A caller that wants the `*Error` would have to unwrap manually.
- The chain would not traverse: a `*Error` with a cause would not match the cause through `errors.Is`. The cause chain would be opaque to the standard library.
- The type would lose the ability to participate in the chain protocols, which is the primary motivation for the methods' existence.

### Option B — Methods as currently implemented, with a written specification

Keep the methods' current semantics (`Unwrap` returns the immediate cause; `Is` matches by Code; `As` assigns the receiver) and document them in the ADR. Leave the implementations as they are.

**Pros:**

- Minimal change to the code.
- The contract is recorded for future readers.
- The tests can be extended to cover the documented cases without changing the implementations.

**Cons:**

- The current `Is` implementation uses `errors.As` internally, which traverses the target's chain. The written specification would say "matches by Code, not by pointer identity", but the implementation would also do something the specification does not mention (traverse the target). A reader comparing the spec to the code would find a discrepancy.
- The behaviour for a target that is a wrapper around a `*Error` is not what the specification's plain reading suggests; the discrepancy is exactly the kind of subtle behaviour that a written contract is meant to pin.
- The freeze is weaker than it could be: a future "optimisation" that removed the `errors.As` call would change the behaviour for wrapped targets, and the tests would not catch it.

### Option C — Methods with the frozen semantics, implemented to match

Keep the three methods, freeze their semantics as per-link operations that do not traverse, and rewrite the `Is` implementation to use a direct type assertion rather than `errors.As`. Add tests for every rule, including the cases the current tests miss (wrapped target, typed-nil target, deeply-nested chain).

**Pros:**

- The specification and the implementation agree exactly. A reader of the spec can predict the behaviour from the code, and a reader of the code can predict the behaviour from the spec.
- The behaviour is deterministic and independent of the target's chain shape. `Is` on a `*Error` and a wrapped `*Error` returns `false`, which is what the specification says: the comparison is between the receiver and the target, not between the receiver and any error reachable from the target.
- The standard library's `errors.Is` traverses the receiver's chain and invokes `Is` at each level. The per-link semantics compose correctly: a `*Error` whose cause is another `*Error` is matched at the appropriate level.
- The typed-nil cases are handled: a typed-nil `*Error` target returns `false` from `Is` (rather than panicking), and a nil `**Error` target returns `false` from `As` (rather than panicking).
- The tests cover every rule, so a regression in any of them is caught.

**Cons:**

- The `Is` rewrite is a behaviour change for the wrapped-target case. A caller that relied on `Is` traversing the target's chain would be affected. The pre-WBS-10.3.2 code was never released, so no production caller exists; the change is safe.
- The specification is longer than the current doc comments; the ADR records more detail than the code's documentation did.
- A future extension (for example, an `Is` that matches by message in addition to Code) is now a contract change, not a local edit. The friction is the point.

### Option D — Sentinels instead of methods

Remove the `Is` method and export a global sentinel value for each Code. Callers match on the sentinel with `errors.Is`.

**Pros:**

- The pattern is familiar from the standard library (`os.ErrNotExist`, `io.EOF`).
- `errors.Is` on the sentinel traverses the chain using the standard library's `Unwrap`, so no custom `Is` is needed.

**Cons:**

- Each Code requires a global sentinel. The registry has 19 Codes in Phase 2, and the number grows. Nineteen globals is a lot of surface area.
- The sentinel's identity is a global; a caller that constructs an error with the same Code but does not use the sentinel does not match. The duality (Code vs sentinel) is a source of confusion.
- The registry already defines the Codes; adding a parallel set of sentinels duplicates the identification mechanism.
- The pattern does not scale to a registry that grows to dozens or hundreds of codes.

**This option was rejected before the ADR was written; it is listed here for completeness.**

---

## 3. Decision

The contract is **Option C**: `Unwrap`, `Is`, and `As` are frozen as per-link operations with the following semantics.

- **`Unwrap` returns the immediate cause only.** It does not traverse the chain. The standard library's `errors.Is` and `errors.As` walk the chain by calling `Unwrap` repeatedly.
- **`Is` matches by Code.** Two `*Error` values are the same logical error if and only if their Codes are equal. The message, category, cause, context, and remediation are ignored. The comparison is a direct type assertion and a string comparison; it does not use `errors.As` and does not traverse either chain. A target that is not a `*Error` returns `false`. A typed-nil `*Error` target returns `false`.
- **`As` assigns the receiver to a `**Error` target.** A target that is not a `**Error` returns `false`. A nil `**Error` target returns `false`. A `**Error` target whose inner pointer is nil is valid: the assignment writes the receiver's pointer into the caller's variable.
- **No sentinels.** The Code is the sentinel. Callers that want to match on a specific Code use `errors.As` to extract the `*Error` and compare its `Code`.
- **`errors.Join` is supported.** A `*Error` participates in a joined error as an ordinary operand.
- **`fmt.Errorf("%w", ...)` preserves the Code.** The wrapper is traversed by `errors.Is` and `errors.As` via the wrapper's own `Unwrap` method.
- **Double-wrapping is impossible.** `forgeerr.Wrap` returns the existing `*Error` unchanged if the cause is already a `*Error`.

The balance tipped toward Option C for three reasons. First, the specification and the implementation must agree. The current `Is` implementation uses `errors.As` internally, which contradicts the specification's plain reading. The rewrite removes the discrepancy, and a reader can predict the behaviour from the spec or the code without checking the other. Second, the per-link semantics are the correct shape. The standard library's `errors.Is` is the traversal engine; a per-link `Is` that also traverses the target does redundant work and produces surprising results. Third, the typed-nil cases are real: a caller that passes a typed-nil `*Error` as a target should receive `false`, not a panic. The current implementation panics; the rewrite handles the case.

The trade-offs accepted are: the `Is` rewrite is a behaviour change (safe, because no production caller exists); the specification is longer than the current documentation (the point of the ADR); and a future extension of `Is` (for example, matching by message) is now a contract change rather than a local edit (the friction is the point). The decision assumes that the Code is the sole identifying property of a `*Error`; if a future phase introduces a second identity (for example, a stable "kind" distinct from the Code), an ADR would extend the `Is` rule.

---

## 4. Consequences

### 4.1 Positive

- The specification and the implementation agree. A reader of either can predict the behaviour of the other without checking.
- The behaviour is deterministic and independent of the target's chain shape. `Is` on a `*Error` and a wrapped `*Error` returns `false`, which is what the specification says: the comparison is between the receiver and the target, not between the receiver and any error reachable from the target.
- The standard library's `errors.Is` traverses the receiver's chain and invokes `Is` at each level. A `*Error` whose cause is another `*Error` is matched at the appropriate level. The per-link semantics compose correctly.
- The typed-nil cases are handled. A typed-nil `*Error` target returns `false` from `Is` rather than panicking. A nil `**Error` target returns `false` from `As` rather than panicking. The two guards are one-line checks; the cost is negligible and the safety is real.
- The tests cover every rule, including the cases the previous tests missed. A regression in any rule fails a specific test.

### 4.2 Negative

- The `Is` rewrite is a behaviour change for the wrapped-target case. A caller that relied on `Is` traversing the target's chain would be affected. No production caller exists (the pre-WBS-10.3.2 code was never released), so the change is safe, but the change is real and is recorded here.
- The specification is longer than the previous documentation. The ADR records more detail than the code's doc comments did, which is the point of the ADR but is a cost for a reader who only wants the summary.
- A future extension of `Is` (for example, matching by message in addition to Code) is now a contract change. The friction is deliberate: the contract is frozen, and a change to it is a change to the ADR.
- The `As` implementation does not recurse into the receiver's chain. A caller that calls `outer.As(&target)` on a `*Error` whose cause is another `*Error` receives `outer`, not `inner`. The standard library's `errors.As` recurses; the per-link `As` does not. The distinction is documented but is a subtlety a caller must remember.

### 4.3 Neutral

- The `Is` implementation uses a direct type assertion (`target.(*Error)`) rather than `errors.As`. The direct assertion is faster and more predictable; the previous implementation's use of `errors.As` was the source of the discrepancy the ADR resolves.
- The `As` implementation is unchanged from its previous form (it already used a direct type assertion). The ADR's contribution to `As` is the documented semantics, not a code change.
- The `Unwrap` implementation is unchanged. The ADR's contribution to `Unwrap` is the documented semantics: it returns the immediate cause, not the transitive chain.
- The double-wrap rule (`Wrap` returns the existing `*Error` unchanged) is established by WBS 10.1.2 (the constructor API) and reaffirmed here because it interacts with the wrapping contract: a double-wrapped `*Error` would put the same Code in the chain twice, which would confuse `errors.Is` matching and formatter output.

---

## 5. Related Requirements

- `ERR-003` — The error wrapping contract. This ADR is the contract.
- `ERR-001` — The canonical error type. This ADR depends on ERR-001's type definition.
- `ERR-002` — The error code taxonomy and stability policy. The `Is` method matches by Code; the Codes' stability (ADR-012) is what makes Code-based matching safe across releases.
- `FR-CORE-009` *(pending)* — The `*forgeerr.Error` type implements `Unwrap`, `Is`, and `As` with the semantics frozen in this ADR. Must be added to the register when WBS 1.4 is complete.
- `NFR-TEST-004` *(pending)* — The wrapping contract has a dedicated test file (`wrap_test.go`) covering every rule and the typed-nil cases. Must be added to the register when WBS 1.4 is complete.

---

## 6. Related ADRs

- `ADR-011` — Closed error category taxonomy. The `Is` method matches by Code, and the Code's prefix determines the category. The two ADRs are independent but a reader of one benefits from reading the other.
- `ADR-012` — Error code registry and stability policy. The `Is` method's Code-based matching relies on the Codes being stable; ADR-012 establishes the stability.
- `ADR-016` — User-facing error format. The format renders the Code; the wrapping contract determines how the Code is reached from a chain of wrapped errors. The two ADRs interact at the boundary: `errors.As` extracts the `*Error`, the formatter renders it.
- `ADR-017` — Programmer-facing error format. The log format renders the Code as a structured field. The relationship with the wrapping contract is the same as ADR-016's.

---

## 7. Notes

### Open questions deferred

- **Should `Is` match by Category as well as by Code?** A caller that wants to match "any config error" would have to enumerate the Codes or use `errors.As` and inspect the category. A future ADR could add a category-based match, but the addition would extend the contract and is not required by any Phase 2 use case.

- **Should `As` recurse into the receiver's chain?** The standard library's `errors.As` recurses; the per-link `As` does not. A caller that wants the inner `*Error` uses `errors.As`, which handles the recursion. The per-link `As` is the building block; the recursion is the standard library's job.

- **Should the contract cover `errors.Join`'s treatment of multiple `*Error` operands?** The current contract says `errors.Join` is supported and a `*Error` participates as an ordinary operand. The standard library's `Is` on a joined error checks each operand; a `*Error` operand matches by Code, and a non-`*Error` operand matches by the standard library's default comparison. The behaviour is correct but is not pinned by a dedicated test; a future phase may add one.

### Follow-up decisions anticipated

- **WBS 10.4.1** (user-facing format) will render the Code that the wrapping contract exposes. The format's `Code:` line is the rendering of the same Code that `Is` matches on.
- **WBS 10.4.2** (programmer-facing format) will render the Code as a structured log field. The relationship is the same.
- **WBS 10.5.1** (boundary integration) will read the Category from the extracted `*Error` and derive the exit code. The wrapping contract's `As` is the extraction mechanism the boundary uses.

### External references

- The Go blog post "Working with Errors in Go 1.13" by Damien Neil and Jonathan Amsterdam describes the design of `errors.Is`, `errors.As`, and the `Unwrap` method. The post's treatment of the per-link semantics is the primary influence on this ADR.
- The standard library's `errors` package source is the reference for the traversal algorithm. The per-link methods compose with the traversal; the traversal is not the methods' responsibility.
- The Go 1.20 release notes describe `errors.Join`; the contract's treatment of joined errors is consistent with the standard library's behaviour.
