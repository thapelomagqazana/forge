# ADR-039: The Filesystem API Is Frozen by a Decision Table

**Status:** Proposed
**Date:** 2026-10-14
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge's filesystem abstraction is the substrate that every domain package builds on: blueprint loaders, template renderers, project scanners, drift detectors, and the update mechanism all reach the disk through the same set of operations. The set has grown organically across several WBS deliveries. Phase 2 introduced the initial eight operations (`Read`, `Write`, `Exists`, `MkdirAll`, `Remove`, `List`, `Stat`, `Boundary`). WBS 13.1.1 added three to support the symlink policy (`Open`, `Lstat`, `Readlink`). WBS 13.5.2 added three to support the atomic-write primitive (`WriteFileExclusive`, `Rename`, `FsyncDir`). Each addition was justified by a real consumer at the time, but the set as a whole was never declared closed. A reader could not tell from the code whether the set was meant to grow further, whether any operation was provisional, or what criteria would justify a new one.

The absence of a stated freeze had two costs. First, a contributor who wanted a new operation had no way to know whether the operation belonged in the abstraction or in the caller. A `Copy` operation, for example, is a plausible addition; without a policy, a contributor could add it, and the abstraction would grow by one operation per contributor's preference. Second, a reviewer auditing the abstraction had no way to reject an operation whose only justification was "it might be useful." The set of operations was what it was; the set had no shape beyond the sum of past decisions, and the sum was invisible.

The situation required a decision because three things depend on the set being stable. The `api.go` files index the surface for reviewers. The golden files under `testdata/` record the set for CI. The reference documentation at `docs/reference/filesystem-api.md` describes the set for callers. All three need a canonical source to be compared against, and the canonical source is the set of methods on the `FS` interface and on `*safefs.Root`. If the set changes without those three being updated, the reviewer's index drifts, the CI golden fails, and the documentation describes a surface that no longer exists. The freeze must be explicit so the four artifacts — the interface, the index, the golden, and the doc — are pinned to the same answer.

Related prior work: WBS 13.1.1 (the layering architecture), WBS 13.1.2 (the API freeze document and its tests), WBS 13.5.2 (the most recent addition to the surface).

---

## 2. Options Considered

### Option A — No freeze: the surface grows by precedent

The abstraction's surface is whatever the accumulated decisions have produced. A new operation is added when a consumer needs it; no index, no golden file, no reference document is maintained. The interface is the surface, and the source code is the reference.

**Pros:**

- The surface is always correct by construction: whatever is in the code is the surface.
- No index to maintain, no golden file to regenerate, no documentation to keep in sync.
- No enforcement mechanism is needed because there is nothing to enforce against.
- A contributor adding an operation has no barrier beyond the review of the operation itself.

**Cons:**

- A reviewer cannot tell whether an operation was deliberate or was added because the contributor found it convenient. Every operation looks the same in the code.
- The set of operations is invisible until a reader enumerates the interface by hand. There is no single document that names the surface.
- The "no operation exists just in case" rule cannot be stated, let alone enforced. An operation that has no consumer is indistinguishable from one that does.
- A change to the surface is not flagged in CI. A future contributor could add a method and every existing test could still pass, because nothing was asserting the surface's exact contents.
- The reference documentation drifts. Every operations doc describes whatever the doc's author remembered, not what the code actually exposes.

### Option B — A written policy: "the surface is closed"

The abstraction declares in its package doc that the surface is closed: no new operation is added without an ADR, and the ADR must name a consumer. The policy is stated in prose; enforcement is by code review.

**Pros:**

- The rule is stated. A contributor who reads the package doc knows the surface is meant to be stable.
- A reviewer can cite the policy when rejecting an operation whose only justification is "it might be useful."
- No additional files, no additional tests, no additional CI steps.

**Cons:**

- The policy is a convention. It is enforced by the reviewer's memory, and a reviewer who does not know the policy cannot enforce it.
- A change to the surface is not caught mechanically. A contributor who adds an operation without an ADR passes every existing test; the drift is only visible if a reviewer notices.
- The prose can drift from the code. The package doc says "the surface is closed" and lists the operations; if the list is out of date, the doc and the code disagree, and there is no mechanism to detect the disagreement.
- The reference documentation is not pinned to the surface. The doc is hand-maintained; every future addition requires a manual doc edit that can be forgotten.

### Option C — A decision table: the surface is enumerated, pinned, and tested

The abstraction's surface is enumerated in three artifacts that must agree: the `api.go` index files (one for `internal/filesystem`, one for `internal/filesystem/safefs`), the golden files under `testdata/`, and the reference document at `docs/reference/filesystem-api.md`. A test (`TestAPI_FSInterfaceIsFrozen` and its sibling) asserts that the set of methods on the interface is exactly the set of operations named in the index. A second test asserts that every operation in the index has a `Consumer:` annotation. A third test (the golden comparison) asserts that the rendered surface matches the committed golden file. The CI runs all three. A change to the surface requires editing all four artifacts in the same commit.

**Pros:**

- The surface is machine-readable. A reader can `grep Consumer api.go` and see every operation's justification.
- The surface is pinned. A change that updates the interface without updating the index fails the freeze test; a change that updates the index without updating the interface fails the same test.
- Every operation is justified. The `Consumer:` annotation names the WBS item that motivated the operation; an operation without one fails the tests.
- The reference documentation is machine-verified. The doc contains a rendering of the surface; the golden comparison detects drift.
- A reviewer auditing the surface can read one file (`api.go`) and see every operation, its purpose, its boundary behavior, and its consumer.
- A contributor adding an operation has a clear checklist: edit the interface, edit the implementation, edit the index, regenerate the golden, update the doc. The four edits are visible in the diff, and a reviewer sees all four together.

**Cons:**

- Four artifacts to maintain per change. The index, the golden, the doc, and the interface must all be updated; a contributor who updates three of the four fails CI, which is the desired signal but is also friction.
- The index is a hand-written list. A contributor who forgets to add a row for a new operation fails the freeze test, which reports the missing operation by name; the fix is to add the row. The mechanical work is small but non-zero.
- The freeze test parses Go source (for the interface) and comment text (for the index). Both parsers are heuristic; a future change to the index's formatting could break the parser. The parsers are small and tested, but they are a maintenance burden.
- The freeze does not prevent every change. A contributor can still add an operation to the interface; the freeze test just requires that the index be updated in the same commit. The freeze is a "must update the index" rule, not a "must not add" rule.

### Option D — A generated interface: the index is derived from the code

The index and the doc are generated from the interface definition. A code-generation step runs in CI; the interface's method set is the source of truth, and the artifacts are derived. A contributor who edits the interface must run the generator; a contributor who edits the generated artifacts must run the generator in reverse (which is not supported).

**Pros:**

- The surface is always in sync. The generated artifacts cannot drift from the interface.
- No hand-written index. The generator produces it from the interface's methods.

**Cons:**

- The generator must parse Go source or use reflection. Parsing is fragile; reflection runs at runtime, which means the generator's output depends on the compiled binary, which couples the generator to the build.
- The `Consumer:` annotation cannot be generated from the code. It is metadata that the contributor supplies; the generator has no way to know it.
- The generator is more code than the hand-written index it replaces. The index is small (a few dozen rows); the generator is a program with its own tests, its own bugs, and its own maintenance.
- A contributor editing the interface must run the generator, which means the interface's edit is not complete until the generator runs. The step is easy to forget; CI would catch it, but the failure mode is a "you forgot to run the generator" message rather than a "you forgot to update the index" message.
- The generator is not necessary for the surface's size. The `FS` interface has fourteen methods and `*Root` has a comparable number. The generator's value would be significant if the surface were hundreds of operations; it is not.

---

## 3. Decision

Forge adopts Option C. The filesystem abstraction's surface is enumerated in the two `api.go` index files, pinned by the golden files under `testdata/`, and described in `docs/reference/filesystem-api.md`. Three tests enforce the agreement: `TestAPI_FSInterfaceIsFrozen` (the interface's method set equals the index's operation set), `TestAPI_EveryOperationHasConsumer` (every operation has a `Consumer:` annotation), and the golden comparison (the rendered surface matches the committed golden file). The CI runs all three. A change to the surface requires editing the four artifacts in the same commit.

The decision rests on four assumptions:

1. **The surface is small enough that a hand-written index is maintainable.** Fourteen methods on `FS` plus a comparable number on `Root` is not a surface that needs code generation. If the surface grows past a few dozen operations, this assumption fails and Option D becomes attractive.
2. **The three tests are sufficient to pin the surface.** A change to the interface is caught by the first test; a change to the index is caught by the first test; a missing consumer is caught by the second; a rendering drift is caught by the third. The three cover the classes of divergence that a hand-edited surface can produce.
3. **The `Consumer:` annotation is worth the maintenance.** Every operation in the index names a WBS item. The annotation is the index's justification; without it, the index is a list, and a list is not a decision. The maintenance cost is one line per operation, and the value is a reviewer's ability to reject an unjustified addition.
4. **The parsers are stable.** The freeze test parses the interface's method set with `go/parser` and the index's operation set with a line-oriented extractor. Both are small, tested, and unlikely to change. If either parser breaks, the failure is a test failure with a clear message, not a silent divergence.

If assumption (1) fails — if the surface grows past a few dozen operations — the mitigation is Option D's code generation. The generator would replace the hand-written index with a derived one, and the `Consumer:` annotation would move from the index to a side file that the generator reads. The change is contained.

The decision is not provisional. The freeze is the policy; a change to the policy would be a new ADR superseding this one.

---

## 4. Consequences

### 4.1 Positive

- A reviewer auditing the surface reads one file and sees every operation, its purpose, its boundary behavior, and its consumer. The index is the reviewer's shortcut to the surface.
- A contributor adding an operation has a checklist: edit the interface, edit the implementation, edit the index, regenerate the golden, update the doc. The four edits appear in the same commit, and a reviewer sees them together.
- CI catches drift. A contributor who updates the interface without the index fails the freeze test; a contributor who updates the index without the interface fails the same test. The failure names the missing operation.
- Every operation is justified. An operation without a `Consumer:` annotation fails the consumer test, and the failure names the operation. The "no operation exists just in case" rule is enforced by a test, not by a reviewer's memory.
- The reference documentation is machine-verified. The doc's operations table is a rendering of the surface; the golden comparison detects drift, and the regeneration command is one line.
- The golden file is diffable. A change to the surface produces a diff that names the added, removed, or modified operations. The reviewer sees the change in one place.
- The pattern scales to future additions. A new operation is added by following the checklist; the checklist is the same regardless of the operation. The pattern is uniform.

### 4.2 Negative

- Four artifacts to maintain per change. The interface, the index, the golden, and the doc. A contributor who updates three of the four fails CI. The friction is intentional — it forces the four to agree — but it is friction.
- The freeze test's parsers are heuristics. The interface parser uses `go/parser` and is precise; the index parser is line-oriented and depends on the index's formatting. A change to the index's formatting could break the parser, and the fix is to update the parser. The parser is small, but it is code that exists to test other code.
- The `Consumer:` annotation is a manual step. A contributor who adds an operation must remember to name a consumer. If they name one that does not exist, the test does not catch it (the test checks for the annotation, not for the consumer's existence); the annotation's validity is a reviewer's concern.
- The doc's operations table is hand-edited. The regeneration command produces the golden file, and the contributor copies the table into the doc; the copy is a manual step. A future change could make the doc's table a generated file that the doc includes, but the current structure is a copy.
- The freeze does not prevent a determined contributor from adding an operation. The freeze test requires the index to be updated; a contributor who updates the index and the interface in the same commit passes the test. The freeze's value is that the change is visible in the diff, not that the change is impossible.

### 4.3 Neutral

- The `api.go` files are named for the artifact they represent (an index of the API surface). The name is short and the file's purpose is stated in its doc comment.
- The golden file is under `testdata/codes/codes.golden.txt`. It is not under `testdata/api/`; the choice of directory reflects the file's purpose (it is the codes registry's golden, rendered for the reference doc) rather than the surface's golden. A future contributor could argue for a different location; the current location is stable.
- The `Consumer:` annotation is a convention. The test checks that the annotation is present; it does not check the annotation's content. The content is the reviewer's concern. The convention is documented in the index's doc comment.

---

## 5. Related Requirements

- `NFR-API-001` — the API stability requirement; this ADR is the mechanism that enforces it.
- `FR-FS-002` through `FR-FS-020` — the operation surface requirements; this ADR is the freeze that governs them.
- `NFR-LAYERING-001` — the `os`-import confinement requirement; this ADR's freeze is separate from the layering lint but complements it.
- `NFR-LAYERING-002` — the one-way import direction requirement; this ADR's freeze covers the two packages' interfaces; the layering lint covers the import graph.
- `FR-FS-001` — the filesystem abstraction requirement; the frozen surface is the requirement's concrete form.

> The following requirements must be added to the register when WBS 1.4 is complete: `NFR-API-002` (the surface is enumerated in an index), `NFR-API-003` (every operation has a `Consumer:` annotation), `NFR-API-004` (the reference documentation is machine-verified against the surface).

---

## 6. Related ADRs

- `ADR-035` — Two-Layer Filesystem Architecture; this ADR is the freeze that governs the two layers' interfaces.
- `ADR-011` — Closed Error Category Taxonomy; the closure argument for the error categories is the same argument this ADR makes for the surface.
- `ADR-012` — Error Code Registry and Stability Policy; the registry's freeze is the pattern this ADR follows for the surface.
- `ADR-034` — The Redaction Policy Is Closed; the closure argument for the redaction policy is the precedent for closing the surface.

---

## 7. Notes

### Open questions

- **Should the freeze extend to `*Root`'s method set as well as `FS`?** The current freeze covers both: `api.go` under `internal/filesystem` enumerates `FS`'s methods, and `api.go` under `internal/filesystem/safefs` enumerates `*Root`'s methods. The two files are tested separately. The pattern is uniform.
- **Should the golden file be regenerated automatically on every commit?** A pre-commit hook could run the regeneration and stage the result. The hook would eliminate the "forgot to regenerate" failure mode, at the cost of a hook that every contributor must install. The current practice is to run the regeneration manually and commit the result; the freeze test catches the omission. Deferred; tracked as a note on WBS 2.5.1.
- **Should the index include the parameter types and return types?** The current index names the operation and its parameters in prose (for example, `Read(path) ([]byte, error)`). A stricter index would parse the signatures from the interface and store them; the freeze test would then compare signatures, not just names. The stricter version would catch a signature change that keeps the name; the current version does not. The stricter version is more code and is deferred. Tracked as a note on WBS 13.1.2.

### Follow-up decisions anticipated

- **ADR-NNN — Error Code Registry Closure.** The error code registry's freeze is documented in ADR-012; the surface's freeze (this ADR) is the analogue for the operation surface. A future ADR could unify the two freeze policies under a single "closure" ADR.
- **ADR-NNN — Golden File Discipline.** The package has three golden families (`codes.golden.txt`, the format goldens, the log goldens); a future ADR could state the golden file policy as a single decision.
- **ADR-NNN — Regeneration Commands.** The regeneration commands are documented in the test files; a future ADR could state them as a single list.

### External references

- `internal/filesystem/api.go` — the index of `FS`'s surface
- `internal/filesystem/safefs/api.go` — the index of `*Root`'s surface
- `internal/filesystem/api_test.go` — the freeze test
- `internal/filesystem/safefs/api_test.go` — the freeze test for `Root`
- `testdata/codes/codes.golden.txt` — the golden file for the codes registry (the pattern this ADR follows)
- `docs/reference/filesystem-api.md` — the reference document that is machine-verified against the surface
- WBS 13.1.2 — the delivery that introduced the freeze document and its tests
