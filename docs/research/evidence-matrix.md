# Evidence Matrix

**Document type:** Research
**Status:** Draft
**Version:** 0.1.0
**Author:** @thapelomagqazana
**Created:** 2026-10-09
**Last Updated:** 2026-10-09
**Supersedes:** —
**Superseded by:** —

---

## 1. Purpose

This document synthesises interview findings into classified evidence
that supports or refutes Forge's product hypotheses.

It exists to answer:

- Which findings were observed, and how strong is the evidence?
- Which hypotheses are supported, contradicted, or inconclusive?
- Which product requirements are validated?
- What remains unknown?

The output of this synthesis feeds:

- `docs/product-discovery.md` § 7 (updated hypothesis statuses)
- `docs/requirements-register.md` (new or revised requirements)
- `docs/phase-1-exit-report.md` (the Go/No-Go decision)

This document is not the interviews. It is the structured analysis of
what the interviews revealed.

---

## 2. Scope

**In scope:**

- Finding transcription and clustering
- Evidence strength classification
- Hypothesis classification
- Requirement derivation
- Contradiction analysis
- Uncertainty documentation

**Out of scope:**

- Interview mechanics (see
  [`docs/research/interview-protocol.md`](./interview-protocol.md))
- Interview operational guide (see
  [`docs/research/market-validation.md`](./market-validation.md))
- Raw findings (see [`docs/research/findings.md`](./findings.md))
- Final product decisions (see
  [`docs/phase-1-exit-report.md`](../phase-1-exit-report.md))

---

## 3. Synthesis Workflow

Evidence synthesis follows a seven-step workflow.

```text
┌─────────────────────────────────────┐
│  1. Transcribe findings             │
│     (findings.md populated)         │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│  2. Cluster by category             │
│     (11 categories)                 │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│  3. Score evidence strength         │
│     (Strong / Moderate / Weak /     │
│      Contradictory / Unknown)       │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│  4. Classify hypotheses             │
│     (Supported / Partial /          │
│      Unsupported / Inconclusive)    │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│  5. Derive requirements             │
│     (FR / NFR / UX / SEC)           │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│  6. Document contradictions         │
│     (explicit, not hidden)          │
└────────────────┬────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────┐
│  7. Record uncertainties            │
│     (what remains unknown)          │
└─────────────────────────────────────┘
```

Each step is documented below.

---

## 4. Step 1: Transcribe Findings

### 4.1 Purpose

Every interview produces raw notes. These are transcribed into
structured findings in [`docs/research/findings.md`](./findings.md).

### 4.2 Anonymity Rule

Participant identities are **never** recorded. Only participant IDs
are used.

**Correct:**

```text
FINDING-001
Participant: P-003
Observation: Participant described manually updating CI configuration
             across 12 repositories over 3 days.
```

**Incorrect:**

```text
FINDING-001
Participant: Jane Smith (jane@acme.com)
Observation: Jane described manually updating CI configuration
             across 12 repositories over 3 days.
```

### 4.3 What Is Recorded

Every finding records:

| Field | Description |
|-------|-------------|
| `FINDING-ID` | Unique identifier (`FINDING-001`, etc.) |
| `Participant` | Participant ID only (`P-001`) |
| `Persona` | `USER-001` through `USER-006` |
| `Category` | One of the 11 categories (§ 5) |
| `Observation` | What was observed |
| `Evidence` | Verbatim quotes and examples |
| `Frequency` | How often the pattern occurred |
| `Impact` | High / Medium / Low |
| `Current workaround` | What the participant does today |
| `Implication` | What this means for Forge |
| `Confidence` | Strong / Moderate / Weak / Contradictory / Unknown |
| `Related hypothesis` | `HYP-NNN` |

### 4.4 Source Material

Findings draw from:

- Interview recordings (if consented)
- Interview notes (taken live)
- Follow-up correspondence (if any)

Raw notes and recordings are **not** committed to the repository.
Only the structured findings are committed.

### 4.5 Verbatim Quotes

Quotes are used to illustrate findings, not to prove them. Every
quote is attributed to a participant ID.

**Correct:**

> "I had no idea the CI workflow had been removed until our deploy
> broke. That was three months after it happened." — P-003

**Incorrect:**

> "I had no idea the CI workflow had been removed..." — anonymous

### 4.6 Finding Status

Findings move through three states:

| State | Meaning |
|-------|---------|
| **Draft** | Recorded but not yet categorised |
| **Categorised** | Assigned to a category and hypothesis |
| **Classified** | Evidence strength assigned |

All findings must reach **Classified** before synthesis is complete.

---

## 5. Step 2: Cluster Findings

### 5.1 Purpose

Findings are grouped into categories. Clustering reveals patterns
that individual findings do not.

### 5.2 Categories

Eleven categories are used:

| # | Category | Definition |
|---|----------|------------|
| 1 | Project creation | Starting a new project or repository |
| 2 | Configuration | Configuring tools, dependencies, environments |
| 3 | Testing | Setting up and maintaining test infrastructure |
| 4 | CI/CD | Continuous integration and delivery |
| 5 | Security | Security posture, secrets, dependency scanning |
| 6 | Documentation | README, architecture docs, CONTRIBUTING, etc. |
| 7 | Standards | Team-wide engineering standards |
| 8 | Templates | Template creation, maintenance, distribution |
| 9 | Maintenance | Ongoing upkeep of repositories |
| 10 | Drift | Divergence from intended standards |
| 11 | Updates | Applying changes to existing repositories |

### 5.3 Category Assignment Rules

- Every finding belongs to exactly one category
- If a finding spans multiple categories, it is split into multiple
  findings
- Category assignment is based on the **primary concern** of the
  finding, not its context

### 5.4 Category Matrix

The following matrix is populated after all findings are
categorised.

| Category | Findings | Participants | Strong | Moderate | Weak |
|----------|----------|--------------|--------|----------|------|
| Project creation | — | — | — | — | — |
| Configuration | — | — | — | — | — |
| Testing | — | — | — | — | — |
| CI/CD | — | — | — | — | — |
| Security | — | — | — | — | — |
| Documentation | — | — | — | — | — |
| Standards | — | — | — | — | — |
| Templates | — | — | — | — | — |
| Maintenance | — | — | — | — | — |
| Drift | — | — | — | — | — |
| Updates | — | — | — | — | — |

This matrix is filled during synthesis.

### 5.5 Category Gap Analysis

After clustering:

- Categories with **zero findings** indicate either:
  - Genuine absence of pain in that area, OR
  - A gap in the interview questions
- Categories with **many findings** indicate concentrated pain

Both patterns are documented.

### 5.6 Cross-Category Patterns

Some findings span multiple categories. Cross-category patterns are
documented separately:

Example:

```text
Cross-category pattern: "Tooling fragmentation"

Observations across categories:
  - Configuration (FINDING-004, FINDING-009)
  - CI/CD (FINDING-012)
  - Templates (FINDING-018)

Pattern:
  Participants use different tools for different categories,
  leading to inconsistent configuration.

Implication:
  Forge could provide a single foundation model that unifies
  these categories.
```

Cross-category patterns are recorded in § 9 of this document.

---

## 6. Step 3: Score Evidence Strength

### 6.1 Purpose

Not all findings are equally strong. Scoring evidence strength
prevents over-interpretation.

### 6.2 The Five Levels

| Level | Meaning | Criteria |
|-------|---------|----------|
| **Strong** | Multiple independent confirmations | ≥ 3 participants, same finding, similar context |
| **Moderate** | Some corroboration | 2 participants, similar finding, OR 1 participant with strong concrete evidence |
| **Weak** | Single observation | 1 participant, finding without corroboration |
| **Contradictory** | Conflicting reports | Participants report opposite experiences |
| **Unknown** | Insufficient evidence | Cannot classify |

### 6.3 Scoring Rules

- **Do not upgrade** based on the interviewer's intuition
- **Do not downgrade** based on plausibility
- **Do not aggregate** across different questions
- **Do not treat quotes as evidence** — quotes illustrate, they do
  not prove
- **Do not treat willingness to try as evidence of demand**

### 6.4 Evidence Strength Formula

A rough guide:

```text
Strength = f(number of participants, concreteness, consistency)
```

Where:

- **Number of participants:** More is stronger
- **Concreteness:** Specific examples are stronger than general
  statements
- **Consistency:** Independent confirmation is stronger than
  repeated observations from one participant

### 6.5 Scoring Example

**FINDING-004:**

```text
Observation: Developers forget to add .gitignore when starting
             new projects.
Evidence:
  - P-001: "I always forget .gitignore"
  - P-003: "I have a template that includes it, otherwise I forget"
  - P-005: "It's the first thing I miss"
Frequency: 3 of 8 participants
Impact: Low
Confidence: Moderate
```

**Why Moderate, not Strong:**

- 3 participants mentioned forgetting `.gitignore`
- But each described a different context (personal project, team
  template, specific language)
- The finding is broad, not specific

### 6.6 Contradictory Evidence

Contradictory findings are explicitly marked:

**FINDING-012:**

```text
Observation: Participants disagree on whether template updates
             are a real problem.
Evidence:
  - P-002: "Copier handles updates fine for our Go projects"
  - P-004: "We gave up on updates entirely; it was too painful"
Contradictory:
  - P-002 reports Copier works
  - P-004 reports Copier's update path is broken for their use case
Confidence: Contradictory
```

Contradictory findings are **not** resolved by choosing a side.
Both sides are recorded.

### 6.7 Unknown Findings

Findings that cannot be classified are marked `Unknown`. Reasons
include:

- The participant's answer was vague
- The finding depends on context not captured
- The finding relies on hypotheticals

Unknown findings are retained but not used for hypothesis
classification.

### 6.8 No Statistical Claims

Five to ten interviews do **not** establish market-wide statistics.
The evidence matrix reports:

- **Patterns** observed in the sample
- **Strength** of individual findings
- **Uncertainty** about broader applicability

It does **not** report:

- Percentages of the market
- Statistical significance
- Confidence intervals

---

## 7. Step 4: Classify Hypotheses

### 7.1 Purpose

Each hypothesis from
[`docs/product-discovery.md`](../product-discovery.md) § 7 is
classified based on the evidence.

### 7.2 The Four Classifications

| Status | Meaning |
|--------|---------|
| **Supported** | Multiple independent confirmations across participants |
| **Partial** | Some evidence, some contradiction |
| **Unsupported** | Evidence contradicts the hypothesis |
| **Inconclusive** | Insufficient evidence to classify |

### 7.3 Classification Criteria

**Supported:**

- At least 3 participants independently confirm the hypothesis
- At least 2 findings are `Strong` or `Moderate`
- No findings directly contradict the hypothesis

**Partial:**

- 2+ participants confirm
- OR 1 participant confirms with strong evidence
- AND contradicting evidence exists

**Unsupported:**

- Multiple participants describe experiences contrary to the
  hypothesis
- OR the hypothesis is falsified by the evidence

**Inconclusive:**

- Fewer than 2 participants address the hypothesis
- OR the evidence is `Unknown` for all relevant findings

### 7.4 Hypothesis Classification Template

For each hypothesis:

```text
HYP-NNN: [Title]

Original statement:
  [From product-discovery.md § 7]

Evidence:
  Supporting:
    - FINDING-NNN (P-xxx): [brief]
    - FINDING-NNN (P-xxx): [brief]
  Contradicting:
    - FINDING-NNN (P-xxx): [brief]
  Neutral/Unclear:
    - FINDING-NNN (P-xxx): [brief]

Classification:
  Supported | Partial | Unsupported | Inconclusive

Reasoning:
  [Why this classification]

Remaining uncertainty:
  [What is still unknown]

Implications for Forge:
  [What this means for the product]
```

### 7.5 Hypotheses Under Test

The six hypotheses from `product-discovery.md` § 7 are classified:

| Hypothesis | Title | Status |
|------------|-------|--------|
| HYP-001 | Foundation drift is a real, recurring problem | — |
| HYP-002 | Project setup is repetitive | — |
| HYP-003 | Teams duplicate repository configuration | — |
| HYP-004 | Engineering standards drift after repository creation | — |
| HYP-005 | Existing template tooling handles updates poorly | — |
| HYP-006 | Teams lack visibility into repository foundation state | — |

Each is filled in during synthesis.

### 7.6 Worked Example (Illustrative)

The following illustrates the classification process. The
participant counts are **illustrative only**; actual counts depend on
the interviews conducted.

```text
HYP-004: Engineering standards drift after repository creation

Original statement:
  Once a repository has been created, its engineering standards
  drift away from what the team originally intended. This drift
  is not systematically detected.

Evidence:
  Supporting:
    - FINDING-004 (P-001): Described CI workflow removed without
      notice, discovered during incident
    - FINDING-007 (P-003): Team standards documented but not
      enforced; drift common
    - FINDING-011 (P-005): Discovered .gitignore missing after
      3 months
    - FINDING-014 (P-002): Reported "we don't really know" when
      asked how they detect drift
    - FINDING-019 (P-004): Platform engineer reported "no
      systematic way" to detect drift across 40 repositories
  Contradicting:
    - FINDING-008 (P-006): Reports strong CI enforcement prevents
      most drift
  Neutral:
    - FINDING-012 (P-007): Discusses drift but in the context of
      a different domain (infrastructure)

Classification:
  Supported

Reasoning:
  7 of 8 participants described some form of repository
  inconsistency. Multiple concrete examples of drift were recorded.
  One participant (P-006) reported that strong CI enforcement
  largely prevents drift, which is a legitimate counterexample but
  does not contradict the hypothesis — it suggests a mitigation.

Remaining uncertainty:
  - Scale and frequency across the broader market
  - Whether teams with strong CI enforcement still experience drift
  - Whether "drift" terminology resonates with all developers
  - How much drift is considered tolerable

Implications for Forge:
  - Drift detection is a credible product capability
  - Forge should provide a way to detect drift systematically
  - The CI enforcement counterexample suggests drift prevention
    via CI is valuable, but not sufficient
```

### 7.7 Classification Integrity Rules

- **Do not upgrade** classification based on hope or intuition
- **Do not ignore** contradictory evidence
- **Do not treat "supported" as "proven"** — supported means
  confirmed within the sample
- **Do not treat "unsupported" as "disproven"** — unsupported means
  the sample did not confirm

### 7.8 What "Supported" Does Not Mean

A supported hypothesis means:

- The evidence in the sample supports the hypothesis
- It remains a **hypothesis**, not a proven fact
- Broader market validation is still required

A supported hypothesis does **not** mean:

- The hypothesis is true for all developers
- The product will succeed
- The problem is worth paying to solve

These require additional validation (Phase 5 alpha testing, broader
market research).

---

## 8. Step 5: Derive Requirements

### 8.1 Purpose

Validated problems become product requirements. This section defines
how findings convert to requirements.

### 8.2 Requirement Types

| Type | Prefix | Derivation source |
|------|--------|-------------------|
| Functional Requirement | FR | Supported hypotheses about behaviour |
| Non-functional Requirement | NFR | Supported hypotheses about performance, compatibility |
| Security Requirement | SEC | Supported hypotheses about safety |
| UX Requirement | UX | Supported hypotheses about interaction |

### 8.3 Derivation Rules

- **A single finding does not produce a requirement.** Findings must
  be corroborated (§ 6) before conversion.
- **A supported hypothesis produces 1–3 requirements.** Over-
  derivation fragments the product.
- **Requirements must be testable.** A requirement that cannot be
  tested is not a requirement.
- **Requirements must be traceable.** Every requirement references
  its source finding(s).
- **Requirements are added to the register.** Derived requirements
  are added to
  [`docs/requirements-register.md`](../requirements-register.md).

### 8.4 Requirement Derivation Template

For each requirement:

```text
REQ-ID: FR-XXX-NNN
Type:    Functional | Non-functional | Security | UX
Source:  FINDING-NNN, FINDING-NNN
Hypothesis: HYP-NNN
Statement: [Verifiable requirement statement]
Rationale: [Why this requirement exists]
Acceptance: [How it will be tested]
```

### 8.5 Derived Requirements Table

The following table is populated during synthesis.

| Req ID | Type | Source findings | Hypothesis | Statement |
|--------|------|-----------------|------------|-----------|
| — | — | — | — | — |

### 8.6 Example (Illustrative)

The following illustrates the derivation process. The source
findings are illustrative only.

```text
REQ-ID: FR-CORE-007
Type:    Functional
Source:  FINDING-004, FINDING-007, FINDING-011, FINDING-019
Hypothesis: HYP-004 (supported)
Statement: Forge must detect drift between the declared foundation
           and the current repository state.
Rationale: Multiple participants described drift being detected
           only by chance (during incidents or audits). Systematic
           detection was requested.
Acceptance: Given a repository with a declared foundation and a
            missing required file, `forge check` reports the
            missing file as a drift finding.
```

### 8.7 Requirements Not Derived

Not every finding produces a requirement. Findings that are:

- Weak in evidence
- Contradicted by other findings
- Outside Forge's scope
- Better handled by existing tools

do not produce requirements. The reason is documented.

### 8.8 Requirements Deferred

Some findings suggest requirements that are valid but belong to later
phases. These are recorded as deferred requirements in the register.

Example:

```text
REQ-ID: FR-REGISTRY-005 (deferred)
Type:    Functional
Source:  FINDING-022
Hypothesis: HYP-005 (partial)
Statement: Forge must support publishing templates to a registry.
Deferred: Phase 16
Rationale: Requirement is valid but outside MVP scope.
```

---

## 9. Cross-Category Patterns

### 9.1 Purpose

Some findings span categories. Documenting cross-category patterns
reveals insights that per-category analysis misses.

### 9.2 Pattern Template

```text
Pattern: [Name]

Observations:
  - FINDING-NNN (P-xxx, category): [brief]
  - FINDING-NNN (P-xxx, category): [brief]
  - FINDING-NNN (P-xxx, category): [brief]

Pattern:
  [What the pattern is]

Evidence strength:
  Strong | Moderate | Weak

Implication for Forge:
  [What this means for the product]
```

### 9.3 Common Cross-Category Patterns

The following patterns are common in developer tooling research:

| Pattern | Description |
|---------|-------------|
| **Tooling fragmentation** | Different tools for different categories |
| **Standard-application gap** | Standards defined but not enforced |
| **Silent drift** | Changes happening without notification |
| **Knowledge concentration** | Standards living in one person's head |
| **Template rot** | Templates not updated after initial creation |
| **Documentation decay** | Docs going stale over time |
| **Manual toil** | Repeated manual work |
| **Context loss** | Original intent forgotten over time |

Patterns are recorded in this section during synthesis.

### 9.4 Pattern Validation

Patterns that appear across multiple participants strengthen:

- The overall product thesis
- Individual hypothesis classifications
- Requirement derivation

Patterns that appear in one participant only remain weak.

---

## 10. Contradiction Analysis

### 10.1 Purpose

Contradictions are the most valuable findings. They reveal where the
product thesis might be wrong.

### 10.2 Contradiction Template

```text
Contradiction: [Name]

Participant A (P-xxx):
  [What A reported]

Participant B (P-xxx):
  [What B reported]

Nature of contradiction:
  [Why these are in conflict]

Possible explanations:
  - Contextual difference: [explanation]
  - Definitional difference: [explanation]
  - Temporal difference: [explanation]
  - Unexplained

Implication for Forge:
  [What this means for the product]
```

### 10.3 Common Contradictions

| Contradiction | Possible explanation |
|---------------|---------------------|
| "Copier works" vs. "Copier doesn't work" | Different use cases |
| "Drift is a problem" vs. "Drift is tolerable" | Different team sizes |
| "Templates are useful" vs. "Templates are limiting" | Different project types |
| "We need more tooling" vs. "We have too much tooling" | Different contexts |

### 10.4 Handling Contradictions

Contradictions are:

- **Recorded** in full (not summarised away)
- **Analysed** for possible explanations
- **Not resolved** by choosing a side
- **Used to refine** the product thesis

A contradiction that cannot be explained is a signal that the
product thesis requires refinement or that the market is fragmented.

### 10.5 Contradiction Impact

| Contradiction impact | Response |
|---------------------|----------|
| Isolated (one participant vs. one) | Record; continue |
| Pattern (multiple participants each side) | Refine hypothesis |
| Fundamental (contradicts core thesis) | Revisit ADR-001 |

---

## 11. Uncertainty Register

### 11.1 Purpose

Everything the synthesis does not know is recorded. This prevents
false confidence.

### 11.2 Uncertainty Template

```text
Uncertainty: [Name]

Related to:
  [Hypothesis or requirement]

What is unknown:
  [Description]

Why it matters:
  [Impact if uncertainty is resolved negatively]

How to resolve:
  - [Action 1]
  - [Action 2]

Target resolution:
  [Phase or date]
```

### 11.3 Common Uncertainties

| Uncertainty | Common cause |
|-------------|--------------|
| Sample size | 5–10 interviews cannot represent a market |
| Persona coverage | Some personas not interviewed |
| Geographic bias | Participants from limited locations |
| Industry bias | Participants from limited industries |
| Terminology | "Drift" may not resonate |
| Willingness to pay | Not tested in Phase 1 |
| Long-term adoption | Not measured in Phase 1 |

### 11.4 Uncertainty Resolution

Uncertainties are resolved by:

- Phase 5 alpha testing (real users)
- Phase 6 public launch (broader adoption)
- Post-MVP research (paying customers)
- Post-Phase 17 team pilots (organisation validation)

Some uncertainties may remain unresolved indefinitely. These are
documented explicitly.

---

## 12. Synthesis Outputs

The synthesis produces five outputs.

### 12.1 Updated Hypothesis Statuses

Each hypothesis from `product-discovery.md` § 7 is updated with its
final status.

### 12.2 New Requirements

Validated problems become new requirements in
`docs/requirements-register.md`.

### 12.3 Contradictions and Uncertainties

Contradictions and uncertainties are documented in this document
(§ 10 and § 11).

### 12.4 Product Discovery Updates

`docs/product-discovery.md` § 3 (problem statement) and § 7
(hypotheses) are updated with the synthesis results.

### 12.5 Go/No-Go Recommendation

The synthesis feeds into the Go/No-Go decision in
`docs/phase-1-exit-report.md`.

---

## 13. Synthesis Integrity Rules

The following rules protect the integrity of the synthesis.

### 13.1 No Fabrication

Every finding is a real observation from a real interview. No
fabricated findings, quotes, or participants.

### 13.2 No Cherry-Picking

All findings are recorded, including those that contradict the
hypotheses. The matrix is not filtered.

### 13.3 No Over-Interpretation

Weak evidence is not presented as strong. One participant is not
presented as "developers."

### 13.4 No Statistical Claims

Interviews do not produce statistics. Percentages and confidence
intervals are not reported.

### 13.5 No Premature Closure

Contradictions and uncertainties are not resolved without evidence.
"Unresolved" is a valid state.

### 13.6 No Retroactive Rewriting

Hypotheses are not modified after the fact to match findings. If a
hypothesis is refuted, it is documented as refuted.

### 13.7 No Vague Findings

Findings are concrete. "Developers are frustrated" is not a finding.
"3 of 8 participants reported manually restoring missing CI
workflows" is a finding.

### 13.8 No Anonymous Findings

Every finding is attributed to a participant ID, even if that ID has
no other identifying information.

### 13.9 No Circular Reasoning

Findings do not reference requirements. Requirements derive from
findings, not the reverse.

### 13.10 No Confusing Feedback with Demand

Willingness to try is not demand. Positive feedback is not a
purchase. Both are signals, not proof.

---

## 14. Evidence Matrix (Live Document)

This section is populated during synthesis. Until then, it is a
template.

### 14.1 Category Matrix

| Category | Findings | Participants | Strong | Moderate | Weak | Unknown |
|----------|----------|--------------|--------|----------|------|---------|
| Project creation | — | — | — | — | — | — |
| Configuration | — | — | — | — | — | — |
| Testing | — | — | — | — | — | — |
| CI/CD | — | — | — | — | — | — |
| Security | — | — | — | — | — | — |
| Documentation | — | — | — | — | — | — |
| Standards | — | — | — | — | — | — |
| Templates | — | — | — | — | — | — |
| Maintenance | — | — | — | — | — | — |
| Drift | — | — | — | — | — | — |
| Updates | — | — | — | — | — | — |

### 14.2 Hypothesis Matrix

| Hypothesis | Status | Supporting findings | Contradicting findings |
|------------|--------|---------------------|------------------------|
| HYP-001 | — | — | — |
| HYP-002 | — | — | — |
| HYP-003 | — | — | — |
| HYP-004 | — | — | — |
| HYP-005 | — | — | — |
| HYP-006 | — | — | — |

### 14.3 Cross-Category Patterns

| Pattern | Findings | Strength | Implication |
|---------|----------|----------|-------------|
| — | — | — | — |

### 14.4 Contradiction Register

| Contradiction | Participants | Explanation | Impact |
|---------------|--------------|-------------|--------|
| — | — | — | — |

### 14.5 Uncertainty Register

| Uncertainty | Related to | Impact | Resolution path |
|-------------|-----------|--------|-----------------|
| — | — | — | — |

### 14.6 Derived Requirements

| Req ID | Type | Source findings | Hypothesis | Statement |
|--------|------|-----------------|------------|-----------|
| — | — | — | — | — |

---

## 15. Synthesis Completion Criteria

Synthesis is complete when:

- [ ] All findings are transcribed into `findings.md`
- [ ] All findings are categorised (11 categories)
- [ ] All findings have an evidence strength (Strong / Moderate /
      Weak / Contradictory / Unknown)
- [ ] All hypotheses are classified (Supported / Partial /
      Unsupported / Inconclusive)
- [ ] All classifications have documented reasoning
- [ ] All cross-category patterns are documented
- [ ] All contradictions are documented
- [ ] All uncertainties are documented
- [ ] All derived requirements are added to the register
- [ ] All source findings are linked to their requirements
- [ ] The live matrix in § 14 is fully populated
- [ ] The synthesis integrity rules (§ 13) have been respected
- [ ] The synthesis has been reviewed by a second person (if
      available)
- [ ] `product-discovery.md` § 3 and § 7 are updated
- [ ] `phase-1-exit-report.md` is ready to reference the synthesis

---

## 16. Relationship to Other Documents

| Document | Relationship |
|----------|--------------|
| [Interview Protocol](./interview-protocol.md) | Defines how interviews are conducted |
| [Market Validation](./market-validation.md) | Defines what to test during interviews |
| [Findings](./findings.md) | Where raw findings are recorded |
| [Participant Summary](./participant-summary.md) | Where participant profiles are recorded |
| [Product Discovery](../product-discovery.md) | Defines the hypotheses being classified |
| [Requirements Register](../requirements-register.md) | Where derived requirements are recorded |
| [Phase 1 Exit Report](../phase-1-exit-report.md) | Where the Go/No-Go decision is recorded |
| [ADR-001](../decisions/ADR-001-forge-as-foundation-manager.md) | The foundational decision this synthesis validates |

---

## 17. Open Questions

The following questions remain open and should be resolved during
synthesis:

- Should findings be re-classified if new interviews are added after
  the initial classification? (Current decision: yes, but only
  before the Go/No-Go decision is made.)
- How should a hypothesis with equal supporting and contradicting
  evidence be classified? (Current decision: `Partial`, with the
  contradiction documented.)
- Should cross-category patterns have their own evidence strength?
  (Current decision: yes, using the same classification levels.)
- Should uncertainties have target resolution dates, or only target
  phases? (Current decision: target phases, not dates.)
- How should requirements that depend on unresolved uncertainties be
  handled? (Current decision: recorded as `Proposed` until the
  uncertainty is resolved.)
- Should synthesis include a "confidence interval" for anything?
  (Current decision: no; qualitative research does not produce
  statistical intervals.)
- Should the synthesis be reviewed by an external party? (Current
  decision: an internal reviewer, if available; external review is
  a Phase 5+ consideration.)

---

## 18. Status

**Draft.**

This document is under active development during Phase 1. It
becomes **Approved** when:

- All findings are classified
- All hypotheses are classified
- All requirements are derived
- All contradictions and uncertainties are documented
- The synthesis integrity rules are respected
- The completion criteria (§ 15) are met
- The document is reviewed for consistency with
  [`docs/research/market-validation.md`](./market-validation.md) and
  [`docs/product-discovery.md`](../product-discovery.md)
