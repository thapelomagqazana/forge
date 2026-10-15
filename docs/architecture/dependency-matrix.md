# Allowed Dependency Matrix

> **Status:** Adopted
> **Extends:** [clean-architecture.md](./clean-architecture.md) §3 (layer matrix)
> **Machine-readable:** [dependency-matrix.yaml](./dependency-matrix.yaml)
> **Schema:** [dependency-matrix.schema.json](../../schemas/dependency-matrix.schema.json)
> **Enforced by:** [`internal/arch/matrix.go`](../../internal/arch/matrix.go) · [`scripts/arch/verify-matrix.sh`](../../scripts/arch/verify-matrix.sh)
> **Traceability:** FR-ARCH-002, NFR-ARCHITECTURE-002
> **WBS:** 14.1.2
> **Consumed by:** WBS 14.6.1 (CI lint rule)

---

## 1. Purpose

[clean-architecture.md](./clean-architecture.md) defines the five-layer model
and a 5×5 matrix. That matrix is the right altitude for a diagram, but it is
not detailed enough to answer the questions that come up in code review:

- May `internal/cli/config.go` import `internal/config` directly?
- May `internal/application/check` import `internal/filesystem`?
- May `internal/domain/blueprint` import `internal/logger`?
- Is `internal/forgeerr` importable from every layer?
- May `internal/cli` import `internal/domain` to reference a type returned by
  an application service?

This document answers those questions. It refines the 5×5 layer matrix into a
**package-group matrix**, adding:

- **Named groups** below the layer level (e.g. `infrastructure.filesystem`).
- **Conditional cells** (`⚠`) for imports that are allowed only under a stated
  condition.
- **An exception registry** for cross-cutting imports that every group may use.

The matrix is the direct input to the CI lint rule in WBS 14.6.1. Any change
to this document that is not reflected in `dependency-matrix.yaml` fails the
arch guardrail.

---

## 2. Package groups

Groups are the unit of the matrix. Each group belongs to exactly one layer.

| importer \ imported | presentation.cli | application | domain | domain.blueprint | domain.component | domain.foundation | domain.policy | domain.template | infrastructure | infrastructure.config | infrastructure.filesystem | infrastructure.git | infrastructure.logging | infrastructure.registry | infrastructure.renderer | infrastructure.streams | architecture.distribution | architecture.platform | architecture.release | architecture.tooling | shared | shared.errors | shared.logging | shared.version |
|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **presentation.cli** | ✓ | ✓ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **application** | ✗ | ✓ | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **domain** | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **domain.blueprint** | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **domain.component** | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **domain.foundation** | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **domain.policy** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **domain.template** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **infrastructure** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **infrastructure.config** | ✗ | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ✓ | ⚠ | ✗ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infrastructure.filesystem** | ✗ | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ✗ | ✓ | ✗ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infrastructure.git** | ✗ | ✗ | ✗ | ✗ | ✗ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infrastructure.logging** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infrastructure.registry** | ✗ | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ⚠ | ⚠ | ✗ | ⚠ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infrastructure.renderer** | ✗ | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ✗ | ✗ | ✗ | ⚠ | ✗ | ✓ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infrastructure.streams** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ⚠ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **architecture.distribution** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **architecture.platform** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **architecture.release** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **architecture.tooling** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | ✓ | ✗ |
| **shared** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ | ✗ |
| **shared.errors** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **shared.logging** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ |
| **shared.version** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |

**Why groups and not just layers.** Three reasons:

1. **`infrastructure` is not monolithic.** A rule that says "Application may
   not import Infrastructure" is easier to enforce when the lint tool can name
   `infrastructure.filesystem` as the specific forbidden target. An error
   message that says *"internal/application/check imports
   internal/filesystem/safefs (group infrastructure.filesystem); see
   dependency-matrix.md row application, column infrastructure.filesystem"*
   is actionable. A message that says *"layer violation"* is not.

2. **Shared has internal structure.** `shared.errors` and `shared.logging` are
   cross-cutting exceptions that every group may import. `shared.version` is
   not. Treating Shared as a single node would either forbid `shared.errors`
   everywhere (unusable) or allow `shared.version` everywhere (too broad).

3. **The grandfather allowlist maps to groups.** A grandfathered package like
   `internal/filesystem` is not a separate architectural unit; it is the
   current physical location of the `infrastructure.filesystem` group. The
   group abstraction lets the matrix refer to the group, and the group lets a
   future relocation happen without changing the matrix.

---

## 3. The matrix

Row = **importer**, Column = **imported**.
`✓` = allowed. `✗` = forbidden. `⚠` = allowed under the condition in §4.

| importer ↓ \ imported → | pres.cli | app | dom.blueprint | dom.template | dom.component | dom.policy | dom.foundation | infra.fs | infra.config | infra.renderer | infra.streams | infra.logging | infra.registry | infra.git | shared.errors | shared.logging | shared.version |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **presentation.cli**        | ✓ | ✓ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **application**             | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **domain.blueprint**        | ✗ | ✗ | ✓ | ✗ | ⚠ | ⚠ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ⚠ | ✗ |
| **domain.template**         | ✗ | ✗ | ⚠ | ✓ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ⚠ | ✗ |
| **domain.component**        | ✗ | ✗ | ⚠ | ⚠ | ✓ | ✗ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ⚠ | ✗ |
| **domain.policy**           | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ⚠ | ✗ |
| **domain.foundation**       | ✗ | ✗ | ⚠ | ✗ | ⚠ | ⚠ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ⚠ | ✗ |
| **infra.filesystem**        | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✓ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infra.config**            | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✓ | ✗ | ✗ | ✓ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infra.renderer**          | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ✗ | ✓ | ⚠ | ✓ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infra.streams**           | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infra.logging**           | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✓ | ✓ | ✓ |
| **infra.registry**          | ✗ | ✗ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ⚠ | ✗ | ✗ | ✓ | ✓ | ✗ | ✓ | ✓ | ✓ |
| **infra.git**               | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ⚠ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✓ | ✓ | ✓ | ✓ |
| **shared.errors**           | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ |
| **shared.logging**          | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ |
| **shared.version**          | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |

Seventeen groups, seventeen columns, seventeen rows. Every cell is `✓`, `✗`,
or `⚠`. There are no blank cells.

---

## 4. Conditions on ⚠ cells

The following conditions apply wherever the corresponding ⚠ appears.

### C1 — Types-only import from `presentation.cli` to `domain.*`

**Applies to:** row `presentation.cli`, columns `domain.blueprint`,
`domain.template`, `domain.component`, `domain.policy`, `domain.foundation`.

**Condition.** `presentation.cli` may import a domain package only to reference
a *type* that appears in an `application` service's `Input` or `Output`, or in
an error value returned by an application service. It must not call domain
functions, construct domain values, or reach into domain internals.

**Why.** Application services return domain types (e.g. `CheckOutput.Findings`
is a slice of `application.Finding`, which the application may later re-export
from a domain type). For the CLI to render those types, it must name them. The
alternative — having every service project every domain type into a
presentation-specific DTO — is a large amount of boilerplate that would need to
be maintained in lockstep with the domain.

**Enforcement.** The lint rule (14.6.1) cannot easily distinguish "type
reference" from "function call" without type-checking. For 14.1.2, this
condition is **documented but not mechanically enforced** beyond the layer
check. 14.6.1 may add a type-checker-based rule. Until then, reviewers enforce
it, and the rationale is recorded here so the rule is not "reinterpreted"
later.

### C2 — Conditional import from `presentation.cli` to `infrastructure.*`

**Applies to:** row `presentation.cli`, columns `infra.fs`, `infra.registry`,
`infra.git`.

**Condition.** `presentation.cli` is the composition root. It may import any
`infrastructure` group to construct an adapter and inject it into an
`application` service. It must not call adapter methods directly; the only
permitted use is construction and injection.

**Why.** The composition root is where concrete dependencies are chosen. If
`presentation.cli` could not import `infrastructure`, nothing would ever
construct the adapters, and the whole application would be non-functional. This
is the same exception the layer matrix already permits implicitly; the group
matrix makes it explicit and names which groups are actually constructed today.

### C3 — Conditional import from `presentation.cli` to `infra.config` and `infra.renderer`

**Applies to:** row `presentation.cli`, columns `infra.config`,
`infra.renderer`.

**Condition.** Same as C2. Listed separately because these are the two adapter
groups the CLI is expected to construct directly (config loading for global
flags, renderer for human/JSON output selection). The other infrastructure
groups are constructed by other groups or by future composition layers.

### C4 — Conditional import from domain to `shared.logging`

**Applies to:** rows `domain.*`, column `shared.logging`.

**Condition.** A domain package may import `shared.logging` only to name the
`Logger` *interface* it receives as a constructor parameter. It must not
construct a logger, must not call `slog.*`, and must not reference the
`infrastructure.logging` implementation.

**Why.** Domain code legitimately emits diagnostic events during use-case
execution. Per ADR-027, domain packages emit diagnostics only through the
logger interface. The interface lives in Shared; the implementation lives in
Infrastructure. Naming the interface in a constructor signature is the
mechanism by which the dependency arrow points inward.

**Enforcement.** The group matrix makes the import of `shared.logging`
*allowed* from domain. Whether the import is used only to name an interface is
a code-review matter (and a future type-checker rule in 14.6.1).

### C5 — Conditional import between domain bounded contexts

**Applies to:** rows `domain.blueprint`, `domain.template`, `domain.component`,
`domain.foundation`; columns for the other domain contexts.

**Condition.** A domain bounded context may import another domain bounded
context only for *types*, never for behavior. Behavior shared between contexts
belongs in a shared domain primitive or in the consumer context.

**Why.** The bounded contexts are being introduced in Phase 3. Allowing them to
call each other's functions freely would collapse them into a single context
with extra directories. Restricting cross-context imports to types preserves
the boundaries without forcing every shared type into a "domain.shared"
package.

**Enforcement.** Documented; not mechanically enforced in 14.1.2. The `✗`
between `domain.template` and `domain.foundation` (and vice versa) reflects an
intentional decision that those two contexts do not currently share types.

### C6 — Conditional import from `infrastructure.*` to `domain.*`

**Applies to:** rows `infra.*`, columns `domain.*` that are ⚠.

**Condition.** An infrastructure adapter may import the domain types it
implements interfaces for, and the domain types it operates on. It must not
call domain behavior that is not part of an interface it implements.

**Why.** The adapter implements an interface declared by a consumer (per
ADR-040's interface ownership rule). To implement it, the adapter names the
interface and its parameter types. Those types come from the domain.

### C7 — Conditional import from `infrastructure.*` to `infra.filesystem`

**Applies to:** rows `infra.config`, `infra.registry`, columns `infra.fs`.

**Condition.** These adapters may import `infra.filesystem` to read and write
files through the safe filesystem API. They must not reach into the
implementation of `infra.filesystem` (e.g. call unexported helpers) and must
not assume the concrete filesystem is `osfs`.

**Why.** `infra.filesystem` is the safe wrapper around `os` (per ADR-035).
Adapters that read files legitimately depend on it. The condition exists to
prevent the adapter from coupling to `osfs` specifically when `memfs` is a
legitimate substitution.

### C8 — Conditional import from `infra.renderer` to `infra.streams`

**Applies to:** row `infra.renderer`, column `infra.streams`.

**Condition.** The renderer may import `infra.streams` to detect and handle
broken pipes and to write to the configured output streams.

**Why.** The renderer writes to `stdout`; the stream wrapper is what makes
broken-pipe handling correct (ADR-029). Coupling the renderer to the stream
wrapper is the point.

### C9 — Conditional import from `presentation.cli` to `infra.git`

**Applies to:** row `presentation.cli`, column `infra.git`.

**Condition.** Same as C2. Listed separately because `infra.git` is a future
group and its construction site is not yet fixed.

---

## 5. Exceptions

Cross-cutting exceptions apply to *every* row, regardless of the matrix.

| Exception                | Applies to                        | Rationale                                                                                                     |
|--------------------------|-----------------------------------|---------------------------------------------------------------------------------------------------------------|
| `shared.errors`          | every group except itself         | Every group must be able to construct and classify errors. Per ADR-011/012/013/014, the error taxonomy is universal. |
| `shared.logging`         | every group except itself         | Every group must be able to emit diagnostics. Per ADR-022/026/027, diagnostics flow through the logger interface. |
| Standard library         | every group                       | Not a Forge package; not classified by this matrix.                                                            |
| External dependencies    | every group, subject to dependency-policy.md | Not Forge packages. Governed by the dependency policy, not this matrix.                              |

`shared.version` is **not** a cross-cutting exception. It is imported by
`presentation.cli` and by `infrastructure.*` (which needs to stamp artifacts
with the build version), but not by `application` or `domain`. If a future use
case needs the version, it is passed in as a value by the composition root, not
imported.

The exception list is closed. Adding a new cross-cutting exception requires a
superseding ADR.

---

## 6. Cycle-freedom

The matrix is acyclic. A directed graph whose nodes are groups and whose edges
are `✓` or `⚠` cells contains no cycle. This is asserted by
`TestMatrixIsAcyclic` in `internal/arch/matrix_test.go`.

Cycles would mean two groups could transitively depend on each other. Go's
import graph forbids direct cycles but permits indirect cycles through
interface indirection that only the loader can see; the arch test is stricter
because it forbids even the *appearance* of a mutual allowance.

---

## 7. Machine-readable form

The canonical YAML is at
[`docs/architecture/dependency-matrix.yaml`](./dependency-matrix.yaml). It is
parsed by `internal/arch/matrix.go` and validated against
[`schemas/dependency-matrix.schema.json`](../../schemas/dependency-matrix.schema.json).

The YAML form is authoritative for the lint rule. The Markdown form is
authoritative for humans. `TestMatrixYAMLMatchesMarkdown` asserts that the two
encode the same matrix; a PR that changes one without the other fails CI.

---

## 8. Change procedure

Changing the matrix is an **ADR-level** decision, not a PR-level one. The
procedure is:

1. Open an ADR proposing the change. State the requirement driving it and the
   alternative(s) considered.
2. Edit `docs/architecture/dependency-matrix.yaml`.
3. Regenerate the Markdown form: `go test ./internal/arch -run TestMatrixYAMLMatchesMarkdown -update`.
4. Add or update the corresponding condition or exception in §4 or §5.
5. Run `scripts/arch/verify-matrix.sh`.

The `TestMatrixYAMLMatchesMarkdown` test fails until YAML, Markdown, and the
in-code matrix agree.

---

## 9. Cross-references

- [clean-architecture.md](./clean-architecture.md) — the five-layer model this matrix refines.
- [layer-dependency-matrix.md](./layer-dependency-matrix.md) — the 5×5 layer matrix.
- [dependency-matrix.yaml](./dependency-matrix.yaml) — the machine-readable form.
- [`internal/arch/matrix.go`](../../internal/arch/matrix.go) — the parser, validator, and rule engine.
- [WBS 14.6.1](../phase-2-requirements.md) — the CI lint rule that consumes this matrix.
