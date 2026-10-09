# Findings

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

This document is the **raw findings log** for Phase 1. It records
every observation produced by developer interviews, in a consistent
structure that supports later synthesis.

It exists to answer:

- What did participants actually say?
- What evidence supports each finding?
- Which hypothesis does each finding relate to?
- What did we observe, in a form that can be analysed?

It does **not** classify or synthesise findings. Classification and
synthesis happen in
[`docs/research/evidence-matrix.md`](./evidence-matrix.md).

This document is the **evidence base**. Every conclusion in Phase 1
must trace back to a finding recorded here.

---

## 2. Scope

**In scope:**

- Raw findings from interviews
- Verbatim quotes (anonymised by participant ID)
- Concrete examples provided by participants
- Observation of current workarounds
- Frequency observations
- Impact observations
- Links to related hypotheses

**Out of scope:**

- Findings that are not from interviews (e.g., from existing
  documentation, competitive analysis — these belong in their own
  documents)
- Classifications and strength scoring (see
  [`docs/research/evidence-matrix.md`](./evidence-matrix.md))
- Hypothesis updates (see
  [`docs/research/evidence-matrix.md`](./evidence-matrix.md))
- Requirements derivation (see
  [`docs/requirements-register.md`](../requirements-register.md))
- Personal identifying information

---

## 3. Finding Structure

Every finding is recorded with the following fields.

| Field | Description |
|-------|-------------|
| `FINDING-ID` | Unique identifier (`FINDING-001`, `FINDING-002`, …) |
| `Participant` | Participant ID (`P-001`), or `MULTIPLE` if a finding spans participants |
| `Persona` | `USER-001` through `USER-006` |
| `Category` | One of the 11 categories (§ 4) |
| `Observation` | What was observed, in one or two sentences |
| `Evidence` | Verbatim quotes and concrete examples |
| `Frequency` | How often the pattern occurred |
| `Impact` | `High` / `Medium` / `Low` with brief justification |
| `Current workaround` | What the participant does today |
| `Implication for Forge` | What this means for the product, if anything |
| `Confidence` | `Strong` / `Moderate` / `Weak` / `Contradictory` / `Unknown` |
| `Related hypothesis` | `HYP-NNN` |

The **Confidence** field is set at record time and revised during
synthesis. Initial confidence is based on the strength of the
evidence within the interview (specific quote + concrete example =
higher confidence than a general statement).

---

## 4. Categories

Findings are assigned to exactly one category:

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

If a finding spans multiple categories, it is split into multiple
findings.

---

## 5. Finding Index

The finding index is a live table. It is updated as findings are
recorded.

| ID | Participant | Persona | Category | Hypothesis | Confidence |
|----|-------------|---------|----------|------------|------------|
| — | — | — | — | — | — |

*(Populated as findings are recorded)*

### 5.1 Finding Counts

| Category | Count |
|----------|-------|
| Project creation | — |
| Configuration | — |
| Testing | — |
| CI/CD | — |
| Security | — |
| Documentation | — |
| Standards | — |
| Templates | — |
| Maintenance | — |
| Drift | — |
| Updates | — |
| **Total** | **—** |

### 5.2 Finding Counts by Hypothesis

| Hypothesis | Count |
|------------|-------|
| HYP-001 | — |
| HYP-002 | — |
| HYP-003 | — |
| HYP-004 | — |
| HYP-005 | — |
| HYP-006 | — |
| Untagged | — |
| **Total** | **—** |

---

## 6. Findings

Findings are recorded in order of `FINDING-ID`. Each finding follows
the template in § 7.

*(Populated as findings are recorded)*

---

## 7. Finding Template

Every finding uses this template.

```text
FINDING-ID: FINDING-NNN

Participant:  P-NNN
Persona:      USER-NNN
Category:     [category]

Observation:
  [What was observed. One or two sentences.]

Evidence:
  [Verbatim quotes and concrete examples. Anonymised to participant
  ID.]

Frequency:
  [How often the pattern occurred across participants.]

Impact:
  [High | Medium | Low with brief justification.]

Current workaround:
  [What the participant does today to cope.]

Implication for Forge:
  [What this means for the product, if anything.]

Confidence:
  [Strong | Moderate | Weak | Contradictory | Unknown]

Related hypothesis:
  [HYP-NNN]
```

---

## 8. Worked Example (Illustrative)

The following illustrates how a finding is recorded. The participant,
the observation, and the evidence are **illustrative only**.

```text
FINDING-ID: FINDING-001

Participant:  P-003
Persona:      USER-003
Category:     Drift

Observation:
  Participant described an incident where the CI workflow for a
  service was removed and went undetected for approximately three
  months.

Evidence:
  "I had no idea the CI workflow had been removed until our deploy
  broke. That was three months after it happened." — P-003

  The participant showed the timeline: the workflow was removed in
  a refactor PR, and no one noticed because there was no
  enforcement of the CI configuration.

Frequency:
  First observation of this pattern in the sample.

Impact:
  High. Deployment failed in production. The failure was
  discovered by chance, not by any systematic check.

Current workaround:
  Manual code review. The participant reports this is inconsistent
  and misses changes.

Implication for Forge:
  Supports the need for continuous foundation verification.
  Suggests that drift detection (forge diff, Phase 9) would have
  surfaced this.

Confidence:
  Moderate. Single participant, but concrete example with
  timeline.

Related hypothesis:
  HYP-004
```

This example is included to show the format. It is not a real
finding and is not counted in the index.

---

## 9. Recording Rules

The following rules ensure findings are consistent and usable.

### 9.1 One Finding Per Observation

Each finding records **one** observation. If a participant makes
three distinct observations in one interview, three findings are
recorded.

### 9.2 Anonymity

Participants are referred to only by ID. Names, employers,
repository names, and other identifying information are never
recorded.

### 9.3 Verbatim Quotes Are Preserved

Quotes are transcribed verbatim. Do not paraphrase, clean up
grammar, or merge quotes.

If a participant requested a quote not be recorded, that request is
honoured. The finding is recorded without the quote.

### 9.4 Concrete Examples

Where a participant provided a concrete example (e.g., "this
happened in March", "we have 12 repositories"), the example is
recorded alongside the finding.

Abstract observations without examples are marked with lower
confidence.

### 9.5 Frequency Is Recorded

Frequency is recorded as observed across the sample, not within the
participant. For example:

- "First observation of this pattern" (a new pattern)
- "Second participant to report this pattern"
- "Consistent pattern across all participants"

Frequency is updated during synthesis.

### 9.6 Impact Is Justified

Impact is recorded with a one-sentence justification. "High" without
context is not acceptable.

### 9.7 Implications Are Tentative

The `Implication for Forge` field is a **tentative** observation made
during recording. It may be revised during synthesis. It is not a
requirement.

### 9.8 Confidence Is Honest

Confidence reflects the evidence, not the recorder's belief. Weak
evidence is recorded as weak, even if the recorder believes the
finding is important.

### 9.9 Hypothesis Linking

Every finding links to the hypothesis it tests. If a finding does
not relate to any hypothesis, it is recorded as `Untagged` and
flagged for the synthesis phase (it may indicate a gap in the
hypotheses).

### 9.10 No Interpretation

The finding is the observation, not the interpretation. If the
recorder draws a conclusion, that conclusion is recorded separately
in the `Implication for Forge` field.

### 9.11 No Cross-Referencing

Findings do not reference other findings. Cross-referencing happens
in synthesis.

### 9.12 No Category Duplication

A finding belongs to exactly one category. If it seems to belong to
two, it is split.

---

## 10. Contradictions

Contradictions are recorded as findings like any other observation.
They are flagged as `Contradictory` in the Confidence field and are
linked to the same hypothesis as the finding they contradict.

Example:

```text
FINDING-ID: FINDING-014

Participant:  P-005
Persona:      USER-002
Category:     Updates

Observation:
  Participant reports that Copier handles template updates
  adequately for their team's Python projects.

Evidence:
  "We use Copier and updates are basically fine. The three-way
  merge works." — P-005

Frequency:
  Contradicts FINDING-012 (P-002), which reported Copier's update
  path is broken.

Impact:
  Medium. Requires investigation during synthesis.

Current workaround:
  Uses Copier updates.

Implication for Forge:
  Contradicts the assumption that existing tools handle updates
  poorly. Requires synthesis.

Confidence:
  Contradictory

Related hypothesis:
  HYP-005
```

Contradictions are not resolved in `findings.md`. They are recorded
and left for synthesis.

---

## 11. Finding Revision

Findings are revised only in limited cases.

### 11.1 What Can Be Revised

- Typos in quotes (with a note)
- Frequency updates (as more participants are interviewed)
- Confidence revisions (during synthesis)
- Category reassignment (if a finding was miscategorised)

### 11.2 What Cannot Be Revised

- The observation itself
- The evidence (quotes, examples)
- The participant ID or persona
- The related hypothesis (unless the hypothesis itself changes)

### 11.3 Revision Log

Every revision is recorded in a revision log at the bottom of the
finding:

```text
Revision history:
  2026-10-09: Recorded (P-003).
  2026-10-15: Frequency updated to "3 of 8 participants" after
              additional interviews.
  2026-10-20: Confidence revised from Moderate to Strong during
              synthesis.
```

This preserves the history of how findings evolved.

---

## 12. Data Handling

### 12.1 What Is Committed

- Anonymised findings
- Verbatim quotes with participant ID only
- Concrete examples with identifiers removed

### 12.2 What Is Not Committed

- Raw interview notes
- Recordings
- Transcripts
- Personal information

Raw materials are stored outside the repository and deleted at
Phase 1 exit.

### 12.3 Deletion on Request

If a participant requests their findings be deleted:

1. All findings from that participant are removed from this document
2. The participant summary entry is removed
3. The change is recorded in the revision history of this document
   (without identifying the participant)
4. Total counts are adjusted

### 12.4 Retention

Findings are retained indefinitely in the repository as anonymised
records. They are the primary evidence for the Phase 1 hypotheses.

---

## 13. Quality Rules

The following quality rules apply.

### 13.1 No Fabrication

Every finding is a real observation from a real interview. No
fabricated findings, quotes, or participants.

### 13.2 No Cherry-Picking

All findings are recorded, including those that contradict the
hypotheses.

### 13.3 No Aggregation

Findings are recorded individually. Aggregation happens in synthesis.

### 13.4 No Interpretation

Findings record observations, not interpretations. Interpretation
happens in the `Implication for Forge` field, and is tentative.

### 13.5 No Missing Fields

Every field in the template is populated. If a field does not apply,
it is marked with a clear placeholder (`—` or `Not applicable`).

### 13.6 No Orphan Findings

Every finding is linked to a hypothesis. If a finding does not
relate to any hypothesis, it is recorded as `Untagged` and flagged
for review during synthesis.

### 13.7 No Duplicate Findings

Before recording a new finding, verify it is not a duplicate of an
existing finding. If it is a duplicate, update the existing finding
instead.

---

## 14. Completion Criteria

This document is complete when:

- [ ] All interviews have been conducted
- [ ] All findings from all interviews are recorded
- [ ] Every finding has a unique `FINDING-ID`
- [ ] Every finding has all required fields populated
- [ ] Every finding is categorised
- [ ] Every finding is linked to a hypothesis (or marked `Untagged`)
- [ ] Finding counts match the index in § 5
- [ ] Contradictions are recorded and flagged
- [ ] No personal information appears anywhere in the document
- [ ] Raw notes and recordings have been deleted (Phase 1 exit)
- [ ] The document has been reviewed for consistency with
      [`docs/research/interview-protocol.md`](./interview-protocol.md)
      § 6

---

## 15. Relationship to Other Documents

| Document | Relationship |
|----------|--------------|
| [Interview Protocol](./interview-protocol.md) | Defines the finding template and category taxonomy |
| [Market Validation](./market-validation.md) | Defines what is tested during interviews |
| [Participant Summary](./participant-summary.md) | Records who was interviewed |
| [Evidence Matrix](./evidence-matrix.md) | Synthesises these findings into classifications |
| [Product Discovery](../product-discovery.md) | Defines the hypotheses these findings test |
| [Requirements Register](../requirements-register.md) | Requirements derived from synthesised findings |
| [Phase 1 Exit Report](../phase-1-exit-report.md) | Uses findings as evidence |

---

## 16. Open Questions

The following questions remain open and should be resolved during
Phase 1:

- Should findings from informal conversations (not full interviews)
  be recorded? Current decision: no; only formal interviews produce
  findings.
- Should findings be recorded for persona `USER-006` (Student)? If
  interviews are conducted, yes; otherwise, no.
- How should a finding that spans multiple hypotheses be handled?
  Current decision: split into multiple findings, one per
  hypothesis.
- Should the finding index in § 5 include a column for synthesis
  status? Current decision: no; synthesis tracking belongs in
  `evidence-matrix.md`.
- Should unflattering findings be recorded (e.g., "participant
  reported they don't care about the problem")? Current decision:
  yes, always. These are the most valuable findings.
- Should follow-up clarifications from participants be added to
  findings? Current decision: yes, with a revision log entry.

---

## 17. Status

**Draft.**

This document is under active development during Phase 1. It becomes
**Approved** when:

- All interviews are complete
- All findings are recorded
- All findings are categorised and linked
- Completion criteria (§ 14) are met
- Open questions have been resolved or explicitly deferred
- The document is reviewed for consistency with
  [`docs/research/interview-protocol.md`](./interview-protocol.md)
