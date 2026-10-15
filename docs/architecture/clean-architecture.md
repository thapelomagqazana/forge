# Clean Architecture Layer Model

> **Status:** Adopted
> **Enforced by:** [`internal/arch`](../../internal/arch/layers.go) (Go test) + [`scripts/arch/verify-layers.sh`](../../scripts/arch/verify-layers.sh) (pre-commit / CI)
> **Traceability:** FR-ARCH-001, NFR-ARCHITECTURE-001
> **WBS:** 14.1.1

---

## 1. Purpose

Forge is a foundation manager. Its value is not in any single command but in the
**stability of the seams** between commands, business rules, and the outside
world. This document defines the seam model that every phase from 3 onward must
honor.

The model is a pragmatic application of Robert C. Martin's *Clean Architecture*
and Alistair Cockburn's *Hexagonal Architecture*, adapted to Go's package
conventions and to Forge's existing layering decisions:

- [ADR-022 — Output and logging are separate architectural layers](../decisions/ADR-022-output-and-logging-are-separate-architectural-layers.md)
- [ADR-028 — The stream contract is enforced at the boundary](../decisions/ADR-028-the-stream-contract-is-enforced-at-the-boundary-output-stdout-logger-stderr-errors-stderr.md)
- [ADR-019 — Single exit boundary in main](../decisions/ADR-019-single-exit-boundary-in-main.md)
- [ADR-035 — Two-layer filesystem architecture](../decisions/ADR-035-two-layer-filesystem-architecture-primitives-and-policy-are-separate-packages.md)

This document is the **canonical** layer model. Where it and any other document
disagree, this document wins; where it and `internal/arch/layers.go` disagree,
the golden test in `internal/arch/layers_test.go` fails.

---

## 2. The five layers

```text
        ┌───────────────────────────────────────────────────────────┐
        │                    PRESENTATION                           │
        │  internal/cli                                             │
        │  Cobra commands · flags · help · human rendering          │
        │  single exit boundary (main returns code)                 │
        └────────────────────────────┬──────────────────────────────┘
                                     │ (calls)
                                     ▼
        ┌───────────────────────────────────────────────────────────┐
        │                    APPLICATION                            │
        │  internal/application                                     │
        │  use-case orchestration · one service per command         │
        │  e.g. NewProjectService, CheckService, DiffService        │
        └────────────────────────────┬──────────────────────────────┘
                                     │ (calls)
                                     ▼
        ┌───────────────────────────────────────────────────────────┐
        │                    DOMAIN                                 │
        │  internal/domain/...                                      │
        │  pure business logic · bounded contexts:                  │
        │  blueprint · template · component · policy · foundation   │
        │  defines interfaces it needs (consumer-owned)             │
        └────────────────────────────┬──────────────────────────────┘
                                     │ (interfaces only)
                                     ▼
        ┌───────────────────────────────────────────────────────────┐
        │                    INFRASTRUCTURE                         │
        │  internal/infrastructure/...                              │
        │  adapters: filesystem · config · renderer · registry ·    │
        │  git · logger impl · error formatting                     │
        │  depends on domain (implements its interfaces)            │
        └───────────────────────────────────────────────────────────┘

        ┌───────────────────────────────────────────────────────────┐
        │                    SHARED                                 │
        │  internal/shared/...                                      │
        │  cross-cutting primitives, no upward dependencies:        │
        │  errors · logging interfaces · version                    │
        └───────────────────────────────────────────────────────────┘
```

The arrows above show **calls at runtime** (Presentation calls Application calls
Domain). The **compile-time** dependency arrows point *inward*: inner layers
never import outer layers. See §3.

---

### 2.1 Presentation — `internal/cli`

**Responsibility.** The user-facing surface: Cobra command definitions, flag
parsing, help text, human and JSON rendering decisions, and the **single exit
boundary** (per ADR-019).

**What it may contain.**

- Cobra commands and their `RunE` wiring.
- Flag parsing and validation that is *purely syntactic* (types, ranges).
- Human/JSON selection and final rendering to stdout/stderr.
- The composition root: constructing concrete adapters and injecting them into
  Application services.
- The mapping from `forgeerr` categories/codes to process exit codes.

**What it must not contain.**

- Business rules. If a rule can be stated without mentioning Cobra, flags,
  terminals, or exit codes, it does not belong here.
- Direct calls into `os`, `os/exec`, or network packages. All I/O goes through
  Application services and their ports.
- Duplicated orchestration: if two commands share a use case, that use case
  lives in Application.

**Imports allowed.** Everything. Presentation is the composition root.

---

### 2.2 Application — `internal/application`

**Responsibility.** Use-case orchestration. **One service per command.** A
service turns a user-facing operation into a sequence of domain calls.

**Shape of a service.**

- A plain struct with injected dependencies — *all interfaces*.
- A `Run(ctx, Input) (Output, error)` method (or a small set of methods, one
  per variant of the same use case).
- No direct I/O. If it needs to read a file, it calls a port; the port is
  implemented in Infrastructure (or by a domain type).
- Errors are returned unchanged; wrapping is allowed per
  [ADR-013 — Error wrapping contract](../decisions/ADR-013-error-wrapping-contract.md).

**Construction.** Constructors validate dependencies and panic on `nil`.
A nil dependency is a *wiring bug*, not a runtime condition; surfacing it at
construction gives a stack trace pointing at the composition root.

**What it must not contain.**

- Persistence, network, or filesystem calls.
- Formatting for humans or for JSON — that is Presentation, via the renderer.
- Domain rules. If it looks like a rule, it belongs in Domain.

**Imports allowed.** Domain, Shared.
**Imports forbidden.** Presentation, Infrastructure.

---

### 2.3 Domain — `internal/domain/...`

**Responsibility.** Pure business logic, organized into **bounded contexts**.
One sub-package per context:

| Bounded context | Package                        | Owns                                              |
|-----------------|--------------------------------|---------------------------------------------------|
| blueprint       | `internal/domain/blueprint`    | Blueprint parsing, validation, semantic rules     |
| template        | `internal/domain/template`     | Template resolution, inheritance, variables       |
| component       | `internal/domain/component`    | Component identity, composition, dependency rules |
| policy          | `internal/domain/policy`       | Overwrite/symlink/permission policies             |
| foundation      | `internal/domain/foundation`   | Project layout rules, file placement              |

**Interface ownership.** Domain declares **every interface it needs** from
infrastructure. This is the *consumer-owns-the-interface* rule (see §4). It is
what makes the dependency arrow point inward and what allows Domain to be tested
with fakes.

**What it must not contain.**

- Imports of `os`, `os/exec`, `net/http`, `time.Now`, or any other
  environment-coupled package.
- Knowledge of Cobra, flags, exit codes, ANSI, JSON, or HTTP.
- Knowledge of the module layout (i.e. it must not import `internal/cli`,
  `internal/application`, or `internal/infrastructure`).

**Imports allowed.** Shared, the standard library (with the caveats above).
**Imports forbidden.** Presentation, Application, Infrastructure.

---

### 2.4 Infrastructure — `internal/infrastructure/...`

**Responsibility.** Adapters that implement interfaces declared by Domain (and,
where the consumer is Application, by Application). Infrastructure is *the only
place that talks to the outside world*.

**Examples.**

- `filesystem` — `osfs`, `memfs`, `safefs` policy (per ADR-035).
- `config` — loader, discovery, precedence (per ADR-007).
- `renderer` — human and JSON output.
- `registry` — component and template registries.
- `git` — repository operations.
- `logger` — the slog-backed implementation of the logger interface declared in
  Shared (per ADR-026).

**What it must not contain.**

- Business rules. If it can be stated without mentioning a syscall, a file, a
  socket, or a process, it belongs in Domain.
- Orchestration. If it sequences multiple domain calls, that sequence belongs
  in Application.
- Interfaces *for consumers*. Infrastructure defines interfaces only for its
  own internal seams; consumer-facing interfaces live with the consumer.

**Imports allowed.** Domain, Shared.
**Imports forbidden.** Presentation, Application.

---

### 2.5 Shared — `internal/shared/...`

**Responsibility.** Cross-cutting primitives with **no upward dependencies**.
Today this means:

| Primitive | Current home | Target home |
|-----------|--------------|-------------|
| Error taxonomy (`forgeerr`) | `internal/forgeerr` | `internal/shared/errors` |
| Logger interface | interface half of `internal/logger` | `internal/shared/logging` |
| Version metadata | `internal/version` | `internal/shared/version` |

**What it must not contain.**

- Anything that imports Domain, Application, Infrastructure, or Presentation.
  If a primitive needs upward knowledge, it belongs in that outer layer, not in
  Shared.
- Convenience helpers that could live in the standard library. A "utils"
  package at the root of `internal/` is an anti-pattern (see §7).

**Imports allowed.** Shared, the standard library.
**Imports forbidden.** Everything else.

---

## 3. Allowed dependency matrix

Row = importer, Column = imported. **✓ = allowed, ✗ = forbidden.**

| importer ↓ \ imported → | Presentation | Application | Domain | Infrastructure | Shared |
|-------------------------|:------------:|:-----------:|:------:|:--------------:|:------:|
| **Presentation**        | ✓            | ✓           | ✓      | ✓              | ✓      |
| **Application**         | ✗            | ✓           | ✓      | ✗              | ✓      |
| **Domain**              | ✗            | ✗           | ✓      | ✗              | ✓      |
| **Infrastructure**      | ✗            | ✗           | ✓      | ✓              | ✓      |
| **Shared**              | ✗            | ✗           | ✗      | ✗              | ✓      |

The machine-checkable form of this table lives in
[`internal/arch/layers.go`](../../internal/arch/layers.go) as the `allowedImport`
function, and the golden assertion lives in
[`internal/arch/testdata/layer_diagram.golden.txt`](../../internal/arch/testdata/layer_diagram.golden.txt).

**Two rules to remember.**

1. **The arrow points inward.** Inner layers never import outer layers.
2. **Shared imports nothing above it.** If a primitive needs upward knowledge,
   it belongs in that outer layer, not in Shared.

The matrix is deliberately *not* a "layers may call the layer below" ladder.
Application may not import Infrastructure even though Infrastructure is "below"
it in the diagram: the arrows in the diagram are runtime calls, and the
compile-time dependency arrow points the other way. Application names the
interface; Infrastructure implements it.

---

## 4. Interface ownership rule

> **Consumer defines, implementer implements.**

If Application needs to read a project's manifest, **Application declares the
interface** (or, if the concept is domain, Domain declares it), and
Infrastructure provides the concrete implementation. This is what makes the
dependency arrow point inward and what allows the consumer to be tested with a
hand-written fake.

**Forbidden:**

- Infrastructure packages declaring interfaces "for" consumers. That inverts the
  dependency and breaks substitutability: the consumer would have to import the
  implementer to name the interface.
- Interfaces declared in a neutral "contracts" package that both sides import.
  This hides the direction of the dependency and creates a coupling hub.

**Permitted and encouraged:**

- A consumer declaring a *narrow* interface (`CheckPort` with one method) rather
  than reusing a broad one.
- Domain declaring an interface that an Infrastructure adapter implements
  *and* that Application consumes — the interface still lives with its
  consumer.
- Small interfaces declared in Infrastructure for Infrastructure's *own*
  internal seams (e.g. between two adapters), provided no outer layer imports
  them.

---

## 5. Enforcement

The model is **not advisory**. It is enforced three ways:

### 5.1 Compiler

Go's import graph already forbids cycles. The layer rules are a stricter
ordering on top: they forbid certain *acyclic* imports too. A clean compile is
necessary but not sufficient.

### 5.2 `internal/arch` package tests

[`internal/arch`](../../internal/arch/layers.go) parses every package under
`internal/...` with [`golang.org/x/tools/go/packages`](https://pkg.go.dev/golang.org/x/tools/go/packages),
classifies each by import path, and asserts the matrix in §3. The tests run as
part of `go test ./...`:

- `TestEveryInternalPackageIsClassified` — **closed by default**. A new package
  that is neither under a layer prefix nor in the grandfather allowlist fails
  here. This is the single most important guardrail: it forces an explicit
  architectural decision for every new package.
- `TestNoUpwardImports` — asserts the *real* import graph matches the matrix,
  including transitive imports via `packages.NeedImports`.
- `TestLayerMatrixGolden` — asserts the in-code matrix matches the published
  golden file, so the docs and the code cannot drift.
- `TestGrandfathersHaveRemovalPhase` — no permanent exceptions.

### 5.3 `scripts/arch/verify-layers.sh`

A fast wrapper around `go test ./internal/arch/...`, intended for the pre-commit
hook and CI. It loads only the arch package, so the hot path stays under a
second on a warm cache. On failure it prints a human-readable hint pointing at
this document and at `internal/arch/layers.go`.

### 5.4 Classification rules

A package is classified by the **longest matching prefix** in
`internal/arch/layers.go`'s `DefaultRules()`:

| Prefix                        | Layer          |
|-------------------------------|----------------|
| `internal/cli`                | Presentation   |
| `internal/application`        | Application    |
| `internal/domain`             | Domain         |
| `internal/infrastructure`     | Infrastructure |
| `internal/shared`             | Shared         |

Prefix matching respects a `/` boundary: `internal/applicationfoo` does **not**
match the `internal/application` rule, and is therefore unclassified. This is
deliberate: it prevents accidental classification of packages that merely share
a name prefix.

Packages that predate the model are listed in `DefaultGrandfathers()` with a
`Reason` and a `RemovalPhase`. **The allowlist must shrink, never grow, without
a superseding ADR.** Every entry is a promise to move.

---

## 6. What this buys us

- **Testability.** Domain and Application are tested with hand-written fakes.
  No temp directories, no real clock, no network, no flakiness.
- **Substitutability.** Swap `osfs` for `memfs` in a test without touching
  Domain. Swap the slog-backed logger for a recording logger. Swap the renderer
  for a golden-file writer.
- **Reversibility.** Changing the CLI framework — an option ADR-002 explicitly
  reserved — does not touch Domain or Application. Changing the config format
  does not touch Domain. Changing the filesystem implementation does not touch
  Application.
- **Auditability.** The dependency matrix is a single table a security reviewer
  can verify in under a minute, and the `internal/arch` test verifies it
  automatically on every commit.
- **Onboarding.** "Where does this package go?" has a default answer. When the
  answer is genuinely unclear, the closed-by-default test forces a written
  decision.

---

## 7. Anti-patterns this model rejects

| Anti-pattern                                             | Why it is rejected                                                                 |
|----------------------------------------------------------|------------------------------------------------------------------------------------|
| Domain importing `os`                                    | Couples business rules to the host; makes them untestable without a tempdir.       |
| Application constructing `osfs.New()`                    | Wires infrastructure inside a use case; breaks substitutability.                   |
| CLI reaching into `safefs` internals                     | Skips the use-case layer; duplicates orchestration; scatters policy.               |
| Infrastructure defining interfaces "for" consumers       | Inverts the dependency arrow; forces consumers to import implementers.             |
| A `utils` package at the root of `internal/`             | Becomes a second Shared with unclear ownership; a cycle magnet.                    |
| Shared importing Domain                                  | Turns Shared into a hub every package must import; makes cycles likely.            |
| A "contracts" package both sides import                  | Hides the direction of the dependency; creates a coupling hub.                     |
| Application returning `[]byte` already JSON-encoded      | Bakes presentation into a use case; makes the use case untestable without a parser.|
| Domain returning `*forgeerr.Error` with a CLI exit code  | Exit codes are a presentation concern; Domain must not know about the process.     |
| A service with multiple unrelated `Run` variants         | Hides two use cases behind one type; violates one-service-per-command.             |

---

## 8. Migration

The model was adopted in **WBS 14.1.1**. This task ships:

- this specification,
- the enforcement package [`internal/arch`](../../internal/arch/layers.go),
- the pre-commit / CI wrapper [`scripts/arch/verify-layers.sh`](../../scripts/arch/verify-layers.sh),
- the `internal/application` package skeleton with one canonical example
  service (`CheckService`) and its tests,
- placeholder `doc.go` files for `internal/domain`,
  `internal/infrastructure`, and `internal/shared`, so that all five layer
  roots exist and are classified from day one.

Physical relocation of existing packages is sequenced under later phases. The
mapping is recorded in `DefaultGrandfathers()`:

| Existing package                 | Target layer    | Target package                          | Removal phase |
|----------------------------------|-----------------|-----------------------------------------|---------------|
| `internal/app/version`           | Application     | `internal/application/version`          | 14.2.x        |
| `internal/config`                | Infrastructure  | `internal/infrastructure/config`        | 14.2.x        |
| `internal/filesystem`            | Infrastructure  | `internal/infrastructure/filesystem`    | 14.2.x        |
| `internal/filesystem/safefs`     | Infrastructure  | `internal/infrastructure/filesystem/safefs` | 14.2.x    |
| `internal/output`                | Infrastructure  | `internal/infrastructure/renderer`      | 14.2.x        |
| `internal/streams`               | Infrastructure  | `internal/infrastructure/streams`       | 14.2.x        |
| `internal/logger` (interface)    | Shared          | `internal/shared/logging`               | 14.3.x        |
| `internal/logger` (slog adapter) | Infrastructure  | `internal/infrastructure/logging`       | 14.3.x        |
| `internal/forgeerr`              | Shared          | `internal/shared/errors`                | 14.3.x        |
| `internal/version`               | Shared          | `internal/shared/version`               | 14.3.x        |

Until relocation, the grandfather allowlist classifies these packages. The
allowlist is a **debt register**, not a permanent home: the arch test fails if
any entry lacks a `RemovalPhase`, and a future test will assert the list
shrinks over time.

---

## 9. How to add a new package

1. **Decide the layer.** Ask: does it orchestrate (Application), encode rules
   (Domain), talk to the outside world (Infrastructure), define cross-cutting
   primitives (Shared), or present to the user (Presentation)?
2. **Place it under the layer prefix.** If your package is a sub-package of an
   existing layer, you are done — it inherits classification by prefix.
3. **If it does not fit, do not guess.** A package that does not fit an existing
   layer is a signal: either the layer model is incomplete (rare, and requires
   a new ADR), or the package belongs in an existing layer but you have not yet
   named which responsibility it carries.
4. **If it predates the model,** add a `Grandfather` entry with a `Reason` and
   a `RemovalPhase`. A grandfather entry without a removal phase fails the test.
5. **Run `scripts/arch/verify-layers.sh`.** It must pass before you commit.

---

## 10. How to change the model

Changing the model is an ADR-level decision, not a PR-level one. The steps:

1. Open an ADR proposing the change. State the requirement driving it.
2. Update `internal/arch/layers.go` (`allowedImport`, `DefaultRules`,
   `DefaultGrandfathers`) **and** this document in the **same PR**.
3. Regenerate the golden file (`internal/arch/testdata/layer_diagram.golden.txt`)
   from `arch.MatrixGolden()`.
4. The `TestLayerMatrixGolden` test will fail until the docs, the code, and the
   golden file agree. That failure is the point.

---

## 11. Cross-references

- [ADR-019 — Single exit boundary in main](../decisions/ADR-019-single-exit-boundary-in-main.md) — why the exit boundary is in Presentation.
- [ADR-022 — Output and logging are separate architectural layers](../decisions/ADR-022-output-and-logging-are-separate-architectural-layers.md) — why renderer and logger are distinct adapters.
- [ADR-026 — The logger interface has four level methods and a `With` combinator](../decisions/ADR-026-the-logger-interface-has-four-level-methods-and-a-with-combinator-the-implementation-is-slog-backed-and-hidden.md) — why the logger interface lives in Shared and its adapter in Infrastructure.
- [ADR-028 — The stream contract is enforced at the boundary](../decisions/ADR-028-the-stream-contract-is-enforced-at-the-boundary-output-stdout-logger-stderr-errors-stderr.md) — how Presentation enforces the stream contract.
- [ADR-035 — Two-layer filesystem architecture](../decisions/ADR-035-two-layer-filesystem-architecture-primitives-and-policy-are-separate-packages.md) — why filesystem primitives and policy are separate packages.
- [Layer dependency matrix](./layer-dependency-matrix.md) — the machine-checkable form of §3.
- [`internal/arch`](../../internal/arch/layers.go) — the enforcement package.
- [`internal/application`](../../internal/application/doc.go) — the use-case layer and its canonical example service.
- [Component spec](./component-spec.md), [Blueprint spec](./blueprint-spec.md), [Template spec](./template-spec.md) — domain bounded contexts that live under `internal/domain/...`.
