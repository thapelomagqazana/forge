# ADR-001: Forge is a Foundation Manager

**Status:** Proposed
**Date:** 2026-10-09
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Software teams repeatedly create repositories from templates, then
allow those templates to drift. Existing tools address project
*creation* — GitHub template repositories, Cookiecutter, Copier,
Yeoman, language-native generators, and platform scaffolders such as
Backstage — but few address the ongoing *maintenance* of the
engineering foundation after creation. Cookiecutter does not support
updates at all. Copier supports updates via a three-way merge, but
its update path requires Copier-compatible templates and does not
provide validation, drift detection, or a declarative foundation
model. Backstage provides templates within a hosted developer portal,
but it is not a developer-machine tool and does not verify that
scaffolded components remain aligned with their templates after
creation.

The result is a persistent gap. A repository typically starts with
strong engineering standards — testing, CI, security configuration,
documentation, structure — but those standards are not enforced
continuously. Over time, files are deleted, configuration changes,
people leave, projects are copied, and standards erode. Teams notice
drift by chance (during code review, during an incident, during a
security audit) rather than through systematic detection. The longer
the drift window, the more expensive the correction.

Forge's Phase 1 discovery work has identified six hypotheses about
this problem. If those hypotheses hold, Forge must decide what it
fundamentally is. Two candidate definitions are on the table. The
first treats Forge as a best-in-class scaffolding CLI. The second
treats Forge as a system for defining, generating, validating, and
evolving the engineering foundation of a repository throughout its
lifecycle. The choice determines the product architecture, the
roadmap, the MVP scope, and how Forge is positioned relative to the
existing tools described above. Without an explicit decision here,
Phase 1 specifications risk fragmenting — some assuming a scaffolding
tool, others assuming a lifecycle manager — and Phase 2
implementation would have no coherent target. The decision affects
every downstream specification, every requirement, and every command
in the CLI surface.

---

## 2. Options Considered

### Option A — Scaffolding CLI

Forge is positioned as a best-in-class project generator. The product
centres on `forge new` and template discovery. The value proposition
is: *"get a project started faster and more consistently."*

**Pros:**

- Smaller scope; faster to ship
- Familiar category with proven demand
- Immediate, concrete utility
- Lower architectural complexity
- Competitive benchmarks exist and are well understood
- Easier to explain in a single sentence

**Cons:**

- Highly commoditized. GitHub templates, Cookiecutter, Copier,
  Yeoman, and language-native generators already occupy this space.
- One-time value. Developers use the tool when starting a project and
  then abandon it. This produces weak recurring engagement.
- No clear differentiation. "Another project generator" is a
  positioning that requires competing on template quality — a
  maintenance-heavy axis where incumbents already have a head start.
- Limited expansion path. There is no natural way to grow from a
  scaffolder to a team-level or organisation-level product without
  redefining the product.
- Weak commercial potential. The scaffolding problem is largely
  solved; the market will compare Forge to existing tools and find it
  redundant.

### Option B — Foundation Manager

Forge is positioned as a system for defining, generating, validating,
and evolving the engineering foundation of a repository throughout
its lifecycle. Scaffolding (`forge new`) is one capability among
several. The core lifecycle is:

```text
CREATE → VERIFY → EXPLAIN → EVOLVE
```

The value proposition is: *"keep the engineering foundation of your
repositories intact as they evolve."*

**Pros:**

- Addresses a persistent, recurring problem (drift) that existing
  tools do not solve.
- Recurring value through `forge check`, `forge diff`, and
  `forge update`.
- Clear differentiation. "Foundation Manager" is a category no
  existing tool claims.
- Natural expansion path: individual developer → team → organisation.
- Aligns with observed reality: standards erode over time, and no
  widely adopted tool offers continuous foundation verification.
- Enables a coherent roadmap through Phase 21 without redefining the
  product.
- Enables a credible commercial layer (private foundations,
  centralised standards, drift reporting, governance) that grows out
  of actual usage rather than being imposed from day one.

**Cons:**

- Larger scope. Requires disciplined MVP boundaries to avoid
  over-engineering.
- Requires market validation of "foundation drift" as a real,
  actionable problem. If the problem is not painful, the product has
  no reason to exist.
- More architectural complexity: state tracking, base-state
  comparison, three-way merge, drift detection, safe updates.
- Longer path to first external release.
- Requires that the product be trusted not to destroy developer work
  during updates — a property that is expensive to build and to
  prove.
- Harder to explain in a single sentence than "a better project
  generator."

### Option C — Both (Scaffolding CLI first, Foundation Manager later)

Forge launches as a scaffolding CLI, then evolves into a Foundation
Manager once adoption is established.

**Pros:**

- Smaller initial scope
- Faster path to first release
- Adoption can be validated before investing in the larger product

**Cons:**

- This is not really a distinct product decision; it is a *sequencing*
  decision. The product identity must be chosen first, or the roadmap
  has no target.
- Risks shipping a scaffolding CLI that has no reason to exist
  relative to Cookiecutter or Copier.
- If the product identity is not chosen now, the specifications
  produced in Phase 1 will drift, and Phase 2 will inherit the
  ambiguity.
- "Evolve later" is a strategy that frequently fails because the
  initial product attracts the wrong users and cannot transition to
  the larger market.

---

## 3. Decision

**Adopt Option B. Forge is a Foundation Manager.**

Scaffolding (`forge new`) is the entry point, not the product. The
central abstraction is the **Engineering Foundation** — a
declarative, versioned, verifiable description of what a repository
should be. Templates remain necessary as an implementation mechanism,
but they are subordinate to the foundation. A template materialises a
blueprint; it does not define the product.

The reasoning for choosing Option B over Option A:

1. **Option A is already solved.** The scaffolding category is
   crowded and mature. A new entrant has no defensible wedge.
2. **The problem persists after creation.** Scaffolding solves a
   one-time problem; foundation drift is continuous. Recurring
   problems produce recurring value, and recurring value is the basis
   of a durable product.
3. **The abstraction is stronger.** "Engineering Foundation" is a
   category Forge can own. "Template" is a category that already has
   incumbents.
4. **The expansion path is credible.** Developer → team →
   organisation is a natural progression for a foundation manager,
   but not for a scaffolder.
5. **The market hypothesis is testable.** "Do teams experience
   foundation drift, and would they act on it?" is a question that
   interviews and alpha testing can answer. "Would developers use
   another scaffolding tool?" is a question that is already answered
   negatively by the current market.

Option C was rejected because it defers the identity decision rather
than making it. Sequencing is a valid strategy once the identity is
chosen, but it is not a substitute for the decision. If the product
identity is not settled now, the Phase 1 specifications will
diverge, and Phase 2 will inherit an ambiguous target.

**Assumptions on which this decision depends:**

- Foundation drift is a real, recurring problem for developers and
  teams. This is the subject of Phase 1 interviews (HYP-001, HYP-004).
- Existing tools do not adequately address drift. This is the subject
  of the competitive analysis and is partially confirmed by the
  absence of a widely adopted foundation-validation tool.
- Developers will adopt a new CLI tool to address drift. This is
  tested during interviews (HYP-006) and during the Phase 5 alpha.

If any of these assumptions is falsified by Phase 1 evidence, this
ADR is superseded before Phase 2 begins.

---

## 4. Consequences

### 4.1 Positive

- Clear differentiation from existing scaffolding tools.
- Recurring value through `forge check`, `forge diff`, and
  `forge update`.
- A natural expansion path: individual developer → team →
  organisation, without redefining the product.
- The "Engineering Foundation" concept becomes Forge's intellectual
  centre and the anchor for all subsequent specifications.
- Enables a coherent roadmap through Phase 21.
- Positions Forge to solve a problem that is not currently solved by
  any widely adopted tool.
- Enables a commercial layer (private foundations, centralised
  standards, drift reporting, governance) that emerges from actual
  usage rather than being imposed early.
- Aligns the product with Forge's own documentation and validation
  philosophy: standards made explicit and verifiable.

### 4.2 Negative

- Larger scope than a scaffolding CLI. Requires stricter
  architectural discipline to avoid over-engineering.
- The MVP must actively resist feature creep; every capability must
  justify its place against the CREATE → VERIFY → EXPLAIN → EVOLVE
  lifecycle.
- The market must validate "foundation drift" as a real, actionable
  problem. If interviews show drift is not painful enough, the
  product has no reason to exist.
- Some developers will expect a scaffolding tool and be surprised by
  the Foundation Manager framing. Onboarding must be designed to
  surface the lifecycle early.
- Requires solving hard technical problems (base-state tracking,
  three-way merge, drift detection, safe updates) that a scaffolding
  CLI would not require.
- Requires earning developer trust not to destroy their work — a
  property that must be demonstrated, not merely claimed.
- Harder to explain in a single sentence than a scaffolding tool.
  Positioning must be sharpened through Phase 1 and Phase 5.

### 4.3 Neutral

- Templates remain necessary as an implementation mechanism, but
  become subordinate to blueprints. Template authors must adopt the
  new framing.
- Components and policies become first-class concepts. This is a
  scope increase over Option A, but consistent with the Foundation
  Manager identity.
- The product name "Forge" continues to fit both interpretations, so
  no renaming is required.
- The roadmap is longer, but every phase has a clear target and a
  clear exit condition.

---

## 5. Related Requirements

This decision establishes the following requirements. Requirement IDs
referenced below must exist in
[`docs/requirements-register.md`](../requirements-register.md) before
this ADR is marked **Accepted**.

- `FR-CORE-001` — Forge must support a declarative Blueprint.
- `FR-CORE-002` — Forge must validate a repository against its
  declared foundation.
- `FR-CORE-003` — Forge must detect drift between the expected
  foundation and the actual repository state.
- `FR-CORE-004` — Forge must evolve foundations safely without
  destroying developer changes.
- `FR-CORE-005` — Forge must record the foundation that a repository
  was created or adopted with.

> The following requirements must be added to the register when
> WBS 1.4 is completed: `FR-CORE-001` through `FR-CORE-005`.

---

## 6. Related ADRs

- —

This is the foundational ADR for Forge. All subsequent ADRs reference
it as the origin of Forge's product identity.

---

## 7. Notes

This ADR does not prescribe the MVP command surface. That is recorded
in [`docs/mvp-scope.md`](../mvp-scope.md) and refined in
[`docs/cli-ux-spec.md`](../cli-ux-spec.md).

This ADR does not decide the update model (three-way merge vs. patch
vs. regenerate). That decision belongs in a dedicated ADR during
Phase 1.

This ADR does not decide the component composition model. That
decision belongs in a dedicated ADR during Phase 1 or Phase 2.

The strategic framing of this decision is documented in
[`docs/product-discovery.md`](../product-discovery.md) and depends on
validation from the developer interviews described in
[`docs/research/interview-protocol.md`](../research/interview-protocol.md).

**Open questions deferred by this ADR:**

- Should the product identity be revisited if HYP-004 (drift) is
  unsupported by interviews?
- Should the roadmap prioritise `forge check` over `forge update` in
  the phases following MVP? This is a sequencing decision, not an
  identity decision, and belongs in a separate ADR.
- Should "Foundation Manager" be Forge's external positioning, or
  should the external framing be more concrete ("keep your
  repository's engineering foundation intact")? This belongs in
  Phase 5 positioning work.

**Follow-up decisions anticipated:**

- An ADR on the update model (Phase 1).
- An ADR on the blueprint schema version 1 freeze (Phase 1).
- An ADR on the component composition model (Phase 12 or earlier, as
  needed).
- An ADR on the registry trust model (Phase 16).

---

## Appendix A: How to Use This Template

This appendix is a guide for the author. It is **not** included when
creating an actual ADR — delete this entire appendix after copying
the template.

### Step 1 — Copy the template

```bash
cp docs/decisions/template.md \
   docs/decisions/ADR-NNN-short-kebab-title.md
```

Replace `NNN` with the next available number. See
[`README.md`](./README.md) § 6 for numbering rules.

### Step 2 — Fill in the metadata block

Set:

- `Status` — leave as `Proposed`
- `Date` — the date you begin drafting
- `Deciders` — the person or role making the decision
- `Supersedes` — the previous ADR, if any
- `Superseded by` — leave as `—` until superseded

### Step 3 — Write each section

Follow the guidance in each section above. Do not skip sections.
Use `—` for empty ones, except in Options Considered where at least
two options are required.

### Step 4 — Reference requirements

Every requirement ID you mention must exist in
[`docs/requirements-register.md`](../requirements-register.md), or
be listed as pending in the register.

### Step 5 — Review

Open a pull request. Request review from at least one person (yourself,
if working solo). Confirm the review checklist from
[`README.md`](./README.md) § 7.

### Step 6 — Accept

After review, change `Status` to `Accepted`. Update the index in
[`README.md`](./README.md) § 10.

### Step 7 — Commit

Commit the ADR and the README index update together as a single
change. This ensures the index never lags behind the directory.

---

## Appendix B: Worked Example

This document is the worked example. It demonstrates the completed
form of an ADR produced from `docs/decisions/template.md`.

---

*End of ADR-001.*
