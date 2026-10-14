# ADR-035: Two-Layer Filesystem Architecture: Primitives and Policy Are Separate Packages

**Status:** Proposed
**Date:** 2026-10-14
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Forge reads and writes files across three operating systems on trees that come from untrusted sources: blueprints downloaded from a registry, templates authored by third parties, project directories the user did not author. Every such operation is a potential attack surface — `..` traversal, symlink escapes, Windows path-length limits, macOS Unicode normalization, case-insensitive collisions — and the failure modes are silent. A name that resolves differently than the caller intended does not crash; it reads or writes the wrong file.

Before this decision, the module had a single `internal/filesystem` package that combined two responsibilities: it wrapped `os` with a portable interface, and it applied the boundary model that kept operations inside the target directory. The two responsibilities had different audiences, different change cadences, and different failure modes. Merging them meant that a contributor who wanted to add a new portable primitive had to reason about the security policy, and a security reviewer who wanted to audit the boundary had to read the primitive implementations to find the checks. The result was that neither concern was well-served.

The situation was not sustainable. Phase 4 (render plan), Phase 7 (repository walk), Phase 14 (base-state hashing), and Phase 15 (scan and detection) all need filesystem access; each was on the cusp of inventing its own wrapper, and each wrapper would be a chance to miss a check. Without a structural answer, the boundary model would be enforced by convention rather than by construction, and convention has a known failure mode: the first contributor who has not read the convention writes code that violates it.

This ADR is the structural answer. It states how the two responsibilities are separated into two packages, how the import direction between them is enforced, and how the separation makes the security policy auditable.

Related prior work: WBS 13.1.1 (filesystem abstraction architecture), WBS 13.1.2 (API freeze), WBS 13.2.1 (portable primitives), WBS 13.2.2 (boundary wrapper), WBS 13.3.1 (path normalization), WBS 13.4.1 (boundary model), WBS 13.5.1 (error taxonomy), WBS 13.5.2 (symlink policy). This ADR formalises the layering that those WBS items assumed.

---

## 2. Options Considered

### Option A — Single package: `internal/filesystem` owns both primitives and policy

One package contains both the `os` wrapper and the boundary checks. Callers import it directly.

**Pros:**

- One import path, one package to learn.
- The boundary checks and the primitive they guard are next to each other in the source, so a reviewer sees both at once.
- No ambiguity about which package to reach for.

**Cons:**

- The package has two audiences with conflicting needs. A contributor adding a new primitive must read the policy to know what not to touch; a security reviewer must read the primitives to find the policy.
- The layering rule ("no unchecked filesystem access") is not expressible at the package level, because the only package that touches the disk is also the only package that is supposed to be imported by everyone. The rule becomes a convention.
- A domain package that imports the package for a primitive gets the policy for free, but also gets every other primitive — including ones it should not use. There is no way to give a caller a boundary-enforced subset without giving them the raw primitives too.
- Adding a new operation to the interface is a breaking change for every caller, because there is no way to distinguish "this caller wants primitives only" from "this caller wants the boundary too."

### Option B — Two packages: `internal/filesystem` (primitives) and `internal/filesystem/safefs` (policy)

The primitives live in `internal/filesystem` with no security policy. The boundary model lives in `internal/filesystem/safefs`, which wraps the primitives. Domain code imports only `safefs`. The import direction is one-way and enforced by CI.

**Pros:**

- Each package has one audience. `internal/filesystem` serves anyone who needs a portable primitive; `safefs` serves anyone who needs boundary enforcement. The two audiences do not overlap.
- The layering rule becomes a property of the package graph: domain packages cannot import `internal/filesystem`, because the CI lint rejects the import. The rule is enforced by the compiler and by CI, not by convention.
- The policy is auditable in one file. A security reviewer reads `internal/filesystem/safefs/` and sees every check, every error, and every operation, without reading the primitive implementations.
- The primitives are reusable. A future tool that wants a portable `os` wrapper without the boundary model imports `internal/filesystem` and gets exactly that. The boundary model does not impose itself.
- Adding a primitive to `internal/filesystem` is not a breaking change for `safefs` callers unless `safefs` chooses to expose it. The two interfaces evolve independently.
- The `os` import is confined to `internal/filesystem`. The CI lint can grep for `"os"` in every other package and fail the build on a violation.

**Cons:**

- Two packages instead of one; a contributor must know which one to reach for.
- A single operation passes through two layers: the `safefs.Root` method calls `internal/filesystem.FS`'s method, which calls `os`. The extra indirection is one function call per operation.
- The boundary between the two packages must be stable; a change to the `FS` interface ripples into `safefs`, `OSFS`, `MemFS`, and every test double.

### Option C — Three or more packages, splitting policy further

The policy itself is split: a boundary package, a symlink package, an error-taxonomy package, each with its own import rules.

**Pros:**

- Each policy concern has its own package with a narrow surface.
- A future addition to one policy does not require touching the others.

**Cons:**

- The policy concerns are interdependent. A boundary check that does not know about symlinks is incomplete; a symlink check that does not know about the boundary cannot refuse an escape. Splitting them means every operation passes through three or more packages, each calling the next, and the composition rules become a new source of complexity.
- The security reviewer's job gets harder, not easier: instead of reading one package, they read four and check the composition.
- The layering rule becomes a graph of allowed and disallowed edges, which is harder to state, harder to check, and easier to get wrong.

Option C was rejected because the complexity it adds is not repaid by the independence it claims. The policy concerns compose; keeping them in one package is the honest shape.

---

## 3. Decision

Forge adopts a two-package filesystem architecture. `internal/filesystem` provides portable, policy-free primitives that wrap `os` and `io/fs`. `internal/filesystem/safefs` provides the boundary model — root confinement, sub-boundaries, deny-list and allow-list checks, symlink policy, atomic writes — on top of those primitives. Domain packages import `safefs` and only `safefs`; the import of `internal/filesystem` from any package outside the two is a CI failure. The import direction is one-way: `safefs` imports `internal/filesystem`, never the reverse, and neither imports any domain package.

The decision rests on three assumptions:

1. **The policy and the primitives have different change cadences.** A new primitive is added when a consumer needs one; a policy changes when the security model changes. The two cadences are independent, and keeping the concerns in one package would couple them.
2. **The CI lint can enforce the import direction on every build.** The lint is a shell script that runs `go list` and greps the results; it is not a heuristic. The rule is a hard property of the build, not a guideline.
3. **The extra function call per operation is negligible.** A `Root.Open` calls `safefs`'s pipeline, which calls `OSFS.Open`, which calls `os.Open`. The extra frame is one function call in a path that already takes a syscall. The cost is invisible.

If assumption (2) fails — if the CI environment cannot run the lint on every build — the layering becomes a convention again, and the decision's value is reduced. The mitigation is to keep the lint small and fast (it is a single `go list -json` pass), so that it can run on every push without slowing the pipeline.

The decision is not provisional. The two-package split is intended to be stable; a future decision to merge them would need a new ADR superseding this one.

---

## 4. Consequences

### 4.1 Positive

- The boundary model is enforced by construction. A domain package that imports `internal/filesystem` fails the CI lint. The compiler and CI are the enforcement; reviewer diligence is not required.
- The policy is auditable in one place. A security reviewer reads `internal/filesystem/safefs/` and sees every check, every sentinel, and every operation. The primitives are below the abstraction and are not the reviewer's concern.
- The primitives are reusable. A future tool that needs a portable `os` wrapper without the boundary model imports `internal/filesystem`. The two consumers do not interfere with each other.
- The `os` import is confined to one package. Every filesystem call in the module goes through the same wrapper; the wrapper is small enough to read in one sitting; the confinement is checkable by grep.
- Adding a primitive does not break existing callers. `internal/filesystem`'s interface can grow without forcing `safefs` to grow. Adding an operation to `safefs` is a deliberate decision with a WBS item behind it; adding one to `internal/filesystem` is a smaller commitment.
- The layering rule is legible from a repository listing. A reader who sees `internal/filesystem` and `internal/filesystem/safefs` and knows nothing else can guess the relationship: the outer package wraps the inner one, and the outer one is what domain code uses.

### 4.2 Negative

- Two packages, two interfaces, two files to add when a new operation is needed. The contributor who adds a primitive writes it in `internal/filesystem`, adds it to the interface, implements it in `OSFS`, and adds a test double in `MemFS`; then, if the operation should be exposed to callers, adds the corresponding `Root` method in `safefs`. The cost is a few extra files per operation. The WBS 13.5.2 delivery, which added three primitives and four `Root` methods, is the concrete example.
- The boundary between the packages is a source of friction. A change to `internal/filesystem.FS` ripples into `safefs`, `OSFS`, `MemFS`, and every test double. The ripple is the price of the stability the interface provides.
- The `os`-confinement lint must be maintained. The lint is a shell script; a new allow-list entry requires a code review; a new package that legitimately needs `os` must be added to the allow-list with a comment naming the ADR. The friction is intentional — adding a legitimate `os` import should be a deliberate act — but it is friction.
- A caller that needs a primitive that `safefs` does not expose has to open a WBS item or an ADR to add the `Root` method. The interface is narrow by design; the narrowness is a cost the caller pays for the boundary.

### 4.3 Neutral

- The two packages share a name prefix (`internal/filesystem` and `internal/filesystem/safefs`). The nesting is deliberate — it makes the relationship visible in a repository listing — but it means that a reader who greps for `filesystem` finds both.
- The `safefs` name is a contraction. It is not obvious from the name what the package does; a reader has to look at the package doc. A future ADR could rename the package, but the cost of the rename (every import in the module, every test, every doc reference) outweighs the readability gain for a name that the doc comment explains.
- The CI lint is a shell script rather than a Go program. The shell script is smaller and faster to write; a Go program would be more portable to non-Unix CI environments. The trade was made in favour of the shell script because the CI runs on Unix today and the script is short enough to read in one sitting.

---

## 5. Related Requirements

- `FR-FS-001` — the filesystem abstraction requirement; this ADR establishes the two-package shape the requirement assumes.
- `FR-FS-002` through `FR-FS-020` — the operation surface; this ADR determines which layer each operation belongs to.
- `SEC-FS-001` — the filesystem security boundary requirement; this ADR establishes the boundary's physical location.
- `SEC-FS-002` through `SEC-FS-010` — the boundary checks; this ADR is the reason they can be audited in one place.
- `SEC-FS-017` through `SEC-FS-022` — the symlink and TOCTOU requirements; this ADR is the reason they are enforceable.
- `NFR-API-001` — the API stability requirement; this ADR defines the two interfaces the requirement governs.

> The following requirements must be added to the register when WBS 1.4 is complete: `NFR-LAYERING-001` (the `os`-import confinement is enforced on every build), `NFR-LAYERING-002` (the import direction between the two packages is one-way).

---

## 6. Related ADRs

- `ADR-001` — Forge as a foundation manager; the two-layer architecture is the substrate the foundation manager's filesystem work is built on.
- `ADR-011` — the closed error category taxonomy; the two packages' sentinels map to the categories this ADR defines.
- `ADR-012` — the error code registry; the codes the `safefs` sentinels use are in the registry this ADR governs.
- `ADR-013` — the error wrapping contract; the two packages' error helpers (`pathError`, `wrapFS`, `classify`) implement the contract.
- `ADR-016` — the error redaction policy; the two packages' context fields are chosen to comply with the policy.
- `ADR-018` — the exit-code taxonomy; the two packages' categories map to the exit codes this ADR governs.

---

## 7. Notes

### Open questions

- **Where does the `internal/filesystem/safefs` package's `Boundary` method live?** The `Root` has its own boundary (the `abs` field) and the `FS` has its own (`Boundary()`). In production they coincide; in tests they may not. The two-boundaries question is deferred to a follow-up ADR, tentatively titled "One Boundary or Two," and is tracked as a note on WBS 13.2.2.
- **Should `safefs` be renamed?** The name is a contraction of "safe filesystem." A more descriptive name (for example, `internal/filesystem/boundary`) would be clearer to a reader who has not seen the package doc. The rename would touch every import in the module; it is deferred until the package's interface stabilises. Tracked as a note on WBS 13.1.1.
- **Should the CI lint be a Go program?** The shell script is Unix-only. A Go program would be portable to Windows CI, if the project ever adds one. The change is deferred until a Windows CI is proposed. Tracked as a note on WBS 2.5.1.

### Follow-up decisions anticipated

- **ADR-NNN — Deny-List Semantics.** The default deny-list and its syntax are the subject of a separate ADR (currently delivered as WBS 13.4.1's reference document; the ADR is a follow-up).
- **ADR-NNN — Atomic Write Semantics.** The temp-file-rename-fsync sequence and its durability guarantees are the subject of a separate ADR (currently delivered as WBS 13.5.2's reference document; the ADR is a follow-up).
- **ADR-NNN — Symlink Policy Closure.** The closure of the symlink policy set (`NoFollow`, `FollowInRoot`) is a decision that warrants its own ADR. The current document references it as a policy but does not ratify the closure.

### External references

- Go standard library: `io/fs`, `os`, `path/filepath`, `crypto/rand`, `syscall`.
- WBS 13.1.1 through WBS 13.5.2 (the deliveries this ADR formalises).
- `docs/architecture/filesystem-layering.md` (the architecture document that elaborates this ADR).
- `scripts/deps/verify-fs-layering.sh` (the CI lint that enforces the layering).
- CWE-22, CWE-59, CWE-61, CWE-367 (the failure modes the two-layer split is designed to prevent).
