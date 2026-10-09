# Market Validation Interviews

- **Document type:** Research
- **Status:** Draft
- **Version:** 0.1.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-09
- **Supersedes:** —
- **Superseded by:** —

---

## 1. Purpose

This document operationalises the interview protocol defined in
[`docs/research/interview-protocol.md`](./interview-protocol.md). It
exists to answer a specific question:

> Does the problem Forge addresses exist in the form Forge intends to
> solve it?

The interview protocol defines **how** to conduct interviews. This
document defines **what to test** during those interviews.

The output of the interviews is:

- Raw evidence in
  [`docs/research/findings.md`](./findings.md)
- Hypothesis classification in
  [`docs/research/evidence-matrix.md`](./evidence-matrix.md)
- A Go/No-Go decision on the product thesis in
  [`docs/phase-1-exit-report.md`](../phase-1-exit-report.md)

This document is not the interviews. It is the guide that makes the
interviews comparable, honest, and useful.

---

## 2. Scope

**In scope:**

- Interview count and recruitment
- Non-pitching rules
- Test plans for existing alternatives
- Test plans for foundation drift
- Test plans for template updates
- Willingness-to-try protocol
- Evidence classification
- Post-interview synthesis

**Out of scope:**

- Interview mechanics (see
  [`docs/research/interview-protocol.md`](./interview-protocol.md))
- Recruitment channels (handled outside the repository)
- Consent forms (handled separately)
- Statistical analysis (this is qualitative research)

---

## 3. Interview Count and Recruitment

### 3.1 Target

**Minimum:** 5 interviews.
**Maximum:** 10 interviews.
**Target:** 7–8 interviews.

Stop early if:

- New interviews produce no new findings (saturation)
- At least one participant from each priority persona has been
  interviewed

Continue if:

- Fewer than 5 interviews completed
- A priority persona has not been interviewed
- Key hypotheses remain `Unknown`

### 3.2 Priority Personas

At minimum, one participant from each of:

- `USER-001` Individual Developer
- `USER-003` Tech Lead
- `USER-004` Platform Engineer

These three are most directly connected to the hypotheses in
[`docs/product-discovery.md`](../product-discovery.md) § 7.

### 3.3 Diversity Target

| Persona | Target count |
|---------|--------------|
| USER-001 Individual Developer | 1–2 |
| USER-002 Software Engineer | 2–3 |
| USER-003 Tech Lead | 1–2 |
| USER-004 Platform Engineer | 1–2 |
| USER-005 Engineering Manager | 1 |
| USER-006 Student or Learner | 0–1 |
| **Total** | **5–10** |

Variation matters more than statistical significance.

---

## 4. Non-Pitching Rules

The single most important rule of market validation:

> **Do not pitch Forge.**

### 4.1 Why

If the interviewer describes Forge, the participant's answers will be
biased toward agreeing with the described product. The result is
worthless evidence.

### 4.2 What Pitching Looks Like

**Bad questions:**

- "Would you use Forge?"
- "Would you use a tool that detects drift?"
- "If I built a CLI that solved this, would you try it?"
- "Do you think foundation drift is a problem?"

These are hypothetical and leading. They produce polite agreement,
not evidence.

**Good questions:**

- "Walk me through the last repository you created."
- "How do you know whether a repository has drifted?"
- "Can you show me an example of drift you've seen?"
- "What did you do about it?"

These are behavioural and past-tense. They produce real evidence.

### 4.3 What to Say When Asked

If the participant asks what you are building:

> "I'm researching how developers set up and maintain engineering
> standards across repositories. I'd rather not describe my ideas
> yet, because it might bias your answers. After we finish, I'm
> happy to share."

If they insist:

> "I'm exploring tooling that helps teams keep repositories aligned
> with their engineering standards."

One sentence. Then move on.

### 4.4 The Vending Machine Test

Before every interview, ask: *"Am I listening, or am I pitching?"*

If the participant spoke less than 70% of the time, you were
pitching.

---

## 5. Test Plans

Each section below defines a specific hypothesis to test and the
questions that test it.

### 5.1 Test Current Alternatives

**Hypotheses tested:** HYP-002 (repetition), HYP-003 (duplication),
HYP-005 (update tooling).

**Goal:** Understand what the participant uses today, why, and what
fails.

#### 5.1.1 Questions

1. What do you use today to start a new project?
2. Why that tool rather than something else?
3. What does it do well?
4. What does it do poorly?
5. What is expensive about it (time, learning, money, maintenance)?
6. What is annoying about it?

#### 5.1.2 What to Listen For

- Specific tool names (GitHub templates, Cookiecutter, Copier,
  Backstage, internal tooling)
- Concrete pain points, not general complaints
- Workarounds the participant has built
- The emotional weight of the pain (frustration, resignation,
  indifference)

#### 5.1.3 What Would Falsify

The hypothesis is falsified if:

- Participants report their current tool works well
- The pain is trivial (under 5 minutes per project)
- Participants cannot articulate a specific failure

#### 5.1.4 Recording

For each participant, record:

| Field | Example |
|-------|---------|
| Current tool | GitHub templates |
| Reason for choice | "It's built in" |
| What works | "Simple, no setup" |
| What fails | "No variable substitution" |
| Cost | "Setup takes 20+ minutes" |
| Annoyance | "I forget files every time" |

### 5.2 Test Foundation Drift

**Hypotheses tested:** HYP-001 (umbrella), HYP-004 (drift), HYP-006
(visibility).

**Goal:** Determine whether foundation drift is a real, recurring
problem.

#### 5.2.1 Questions

1. Has a repository you worked on ever stopped following your team's
   standards?
2. How did you discover it?
3. What happened?
4. How long had it been drifting?
5. Who noticed first?
6. What did you do about it?
7. Has this happened more than once?

#### 5.2.2 What to Listen For

- Concrete examples, not hypotheticals
- Discovery method (chance vs. deliberate detection)
- Frequency (one-off vs. recurring)
- Cost (time, risk, rework)
- Emotional response (surprise, frustration, resignation)

#### 5.2.3 What Would Falsify

The hypothesis is falsified if:

- Participants report they never experience drift
- Drift is always discovered immediately
- Drift is trivial to correct
- Participants consider drift acceptable

#### 5.2.4 Recording

For each drift example:

| Field | Example |
|-------|---------|
| Repository | `payments-api` |
| Standard that drifted | CI workflow |
| Discovery method | "Noticed in code review" |
| Time to discover | "3 months" |
| Impact | "Deploy broke" |
| Response | "Fixed manually" |
| Recurrence | "Happens often" |

#### 5.2.5 The Critical Question

The most important question in the interview:

> "If you had known about this drift the day it appeared, what would
> have changed?"

If the answer is "nothing" or "not much," drift is not a real problem
for this participant. Record this honestly.

### 5.3 Test Template Updates

**Hypotheses tested:** HYP-005 (update tooling).

**Goal:** Determine whether updating existing repositories is a real
problem.

#### 5.3.1 Questions

1. If your project template changed today, how would you update
   existing repositories?
2. Have you ever needed to update an existing repository to match a
   new template version?
3. What happened?
4. What made it difficult?
5. What did you do instead?

#### 5.3.2 What to Listen For

- Whether the participant has updated templates
- Whether the update preserved developer modifications
- Whether the update was manual or automated
- Whether the participant avoided updates (and why)

#### 5.3.3 What Would Falsify

The hypothesis is falsified if:

- Participants report updates are easy
- Participants never need to update
- The existing tool (Copier) handles updates well

#### 5.3.4 Recording

For each participant:

| Field | Example |
|-------|---------|
| Update tool | "Manual" |
| Frequency | "Rarely" |
| Method | "Copy-paste" |
| Preserved dev changes | "No, we lose them" |
| Alternative | "Accept drift" |
| Pain level | "Medium" |

### 5.4 Test Willingness to Try

**Goal:** Measure stated willingness to try an early implementation.

#### 5.4.1 The Question

At the end of every interview:

> "Would you be willing to try an early implementation on a
> non-critical project?"

#### 5.4.2 Recording

| Response | Meaning |
|----------|---------|
| Yes | The participant is willing to try |
| Maybe | The participant is hesitant; probe for why |
| No | The participant declines; probe for why |

#### 5.4.3 What This Is Not

Willingness to try is **not** product validation. It is a signal, not
proof. A "Yes" without follow-through is worth less than a "Maybe"
followed by actual usage.

#### 5.4.4 What to Record Alongside the Answer

For every response, record:

- The reason (if the participant volunteers one)
- Any conditions ("only if it's open source," "only if it doesn't
  touch my code")
- Any constraints ("only on a side project")

### 5.5 Additional Test Areas

While the four areas above are the primary focus, the following may
also be probed.

#### 5.5.1 Team Standards Propagation

**Hypothesis tested:** HYP-003 (duplication).

**Question:** "When your team introduces a new standard, how does it
reach existing repositories?"

#### 5.5.2 Repository Creation Volume

**Hypothesis tested:** HYP-002 (repetition).

**Question:** "How many new repositories have you created in the last
year?"

#### 5.5.3 Visibility Across Repositories

**Hypothesis tested:** HYP-006 (visibility).

**Question:** "How do you currently answer 'which of our
repositories are aligned with our standards?'"

#### 5.5.4 Existing Tools' Failure Modes

**Hypothesis tested:** HYP-005 (update tooling).

**Question:** "Tell me about a time a tool didn't work the way you
expected."

---

## 6. Evidence Classification

Every finding is classified for strength.

### 6.1 Classification Levels

| Level | Meaning |
|-------|---------|
| **Strong** | Multiple participants independently report the same pain with concrete examples |
| **Moderate** | Multiple participants report similar pain with some detail |
| **Weak** | One participant reports pain, or pain is described without detail |
| **Contradictory** | Participants report conflicting experiences |
| **Unknown** | Insufficient evidence to classify |

### 6.2 Classification Rules

- **Do not upgrade evidence based on the interviewer's intuition.**
  If only one participant reports a pain, it is Weak.
- **Do not downgrade evidence based on plausibility.** If a
  participant reports a pain that seems unlikely, record it
  honestly.
- **Do not aggregate across different questions.** Each finding is
  classified independently.
- **Do not treat quotes as evidence.** Quotes illustrate; they do
  not prove.

### 6.3 Evidence Record

Every finding is recorded with:

| Field | Description |
|-------|-------------|
| `FINDING-ID` | Unique identifier (`FINDING-001`, etc.) |
| `Participant` | Participant ID (`P-001`, etc.) |
| `Persona` | `USER-001` through `USER-006` |
| `Category` | One of the 11 categories from the protocol |
| `Observation` | What was observed |
| `Evidence` | Verbatim quotes and examples |
| `Frequency` | How often the pattern occurred |
| `Impact` | High / Medium / Low |
| `Current workaround` | What the participant does today |
| `Implication` | What this means for Forge |
| `Confidence` | Strong / Moderate / Weak / Contradictory / Unknown |
| `Related hypothesis` | `HYP-NNN` |

Findings are stored in
[`docs/research/findings.md`](./findings.md).

### 6.4 Hypothesis Updates

After all interviews, each hypothesis is classified:

| Status | Meaning |
|--------|---------|
| Supported | Multiple independent confirmations |
| Partial | Some evidence, some contradiction |
| Unsupported | Evidence contradicts |
| Inconclusive | Insufficient evidence |

Statuses are recorded in
[`docs/research/evidence-matrix.md`](./evidence-matrix.md).

---

## 7. Post-Interview Synthesis

After all interviews, the following synthesis steps are performed.

### 7.1 Classify Findings

Every finding is classified by category:

1. Project creation
2. Configuration
3. Testing
4. CI/CD
5. Security
6. Documentation
7. Standards
8. Templates
9. Maintenance
10. Drift
11. Updates

### 7.2 Classify Hypotheses

Each hypothesis is classified based on the evidence.

### 7.3 Update the Product Discovery

Findings inform updates to
[`docs/product-discovery.md`](../product-discovery.md) § 7
(hypotheses) and § 3 (problem statement).

### 7.4 Prepare the Go/No-Go Decision

The synthesis feeds into
[`docs/phase-1-exit-report.md`](../phase-1-exit-report.md).

The Go/No-Go decision is based on:

- Whether the core hypotheses (HYP-001, HYP-004) are supported
- Whether the differentiation (HYP-005) is credible
- Whether developers report willingness to try
- Whether the problem is painful enough to justify a product

### 7.5 Contradictions

Contradictions are valuable. Record them explicitly. Do not paper
over them.

Examples:

- Participant A reports drift is a critical problem; Participant B
  reports it is trivial.
- Participant C reports Copier solves updates; Participant D reports
  Copier does not.

Contradictions are recorded in
[`docs/research/evidence-matrix.md`](./evidence-matrix.md).

---

## 8. Anti-Patterns

The following are explicitly avoided during market validation.

### 8.1 Pitching

Describing Forge, its features, or its design before the interview is
complete.

### 8.2 Leading Questions

Questions that suggest the desired answer ("Would you find X useful?").

### 8.3 Hypotheticals

Questions about what the participant *would* do rather than what they
*have* done.

### 8.4 Cherry-Picking

Reporting only findings that support the hypotheses.

### 8.5 Over-Interpretation

Treating one participant's opinion as a market-wide truth.

### 8.6 Vanity Metrics

Treating willingness-to-try as proof of demand. It is not.

### 8.7 Confusing Politeness with Demand

Participants often say "yes" to be polite. Look for behaviour, not
agreement.

### 8.8 Skipping Saturation

Stopping interviews before either saturation is reached or 5
interviews are completed.

### 8.9 Ignoring Contradictions

Failing to record or analyse conflicting evidence.

### 8.10 Rationalising Weak Evidence

Claiming a hypothesis is supported when evidence is weak or
contradictory.

---

## 9. Sampling and Saturation

### 9.1 Sample Size Justification

Qualitative research does not require statistical significance. The
goal is to reach **saturation**, the point at which new interviews
produce no new findings.

For a focused problem domain like foundation drift:

- 5 interviews typically surface the main patterns
- 7–8 interviews typically reach saturation
- 10 interviews are the upper bound to prevent diminishing returns

### 9.2 Saturation Criteria

Saturation is reached when:

- At least 3 consecutive interviews produce no new categories of
  findings
- Priority personas are covered
- The core hypotheses have clear evidence (supported or
  unsupported)

### 9.3 When to Stop

Stop interviews when:

- Saturation is reached
- Minimum 5 interviews completed
- Priority personas covered

Do not stop when:

- Only 5 interviews completed but saturation not reached
- A priority persona is missing
- A hypothesis is still `Unknown` due to insufficient evidence

### 9.4 When to Continue

Continue beyond 10 interviews only if:

- A critical hypothesis remains `Unknown`
- A specific finding requires validation with a different persona

Document the reason for extending beyond 10.

---

## 10. Interview Log Template

For each interview, the following is recorded (anonymised).

```text
Interview Log
─────────────

Participant ID:       P-NNN
Persona:              USER-NNN
Date:                 YYYY-MM-DD
Duration:             NN minutes
Recording consent:    Yes / No

Interviewer notes:
  (Brief summary of the interview, key observations)

Findings produced:
  - FINDING-NNN
  - FINDING-NNN
  - FINDING-NNN

Willingness to try:
  Yes / Maybe / No
  Reason: (if volunteered)

Follow-up actions:
  (If any)
```

Interview logs are stored outside the repository (only anonymised
findings are committed).

---

## 11. Analysis Checklist

After all interviews, the following analysis is performed.

### 11.1 Data Preparation

- [ ] All interviews recorded or transcribed
- [ ] All findings written in the standard format
- [ ] All findings assigned to categories
- [ ] All findings linked to participants and hypotheses

### 11.2 Classification

- [ ] Each hypothesis classified (Supported / Partial / Unsupported /
      Inconclusive)
- [ ] Each finding classified (Strong / Moderate / Weak /
      Contradictory / Unknown)
- [ ] Contradictions recorded
- [ ] Unknowns identified

### 11.3 Synthesis

- [ ] Findings grouped by category
- [ ] Patterns identified
- [ ] Outliers identified
- [ ] Recommendations derived

### 11.4 Documentation

- [ ] Findings recorded in `findings.md`
- [ ] Hypothesis statuses recorded in `evidence-matrix.md`
- [ ] Participant summary updated
- [ ] Product discovery updated (§ 3 and § 7)
- [ ] Interview log summarised

### 11.5 Decision

- [ ] Go/No-Go criteria evaluated
- [ ] Exit report prepared
- [ ] Decision recorded

---

## 12. Integrity Rules

The following rules protect the integrity of the research.

### 12.1 Honest Reporting

Every finding is reported honestly, including findings that
contradict the hypotheses.

### 12.2 No Rationalisation

Weak evidence is not presented as strong. Contradictions are not
dismissed.

### 12.3 No Pitching

The interviewer does not describe Forge until the interview is
complete.

### 12.4 No Leading

Questions are behavioural and neutral. No question presumes an
answer.

### 12.5 Anonymity

Participant identities are not recorded. Only IDs are used.

### 12.6 Consent

Recording and note-taking are done only with consent.

### 12.7 Retention

Raw notes are deleted after the phase exit report is written.
Anonymised findings are retained.

### 12.8 No Cherry-Picking

Findings are not filtered to support a narrative. The full set of
findings is committed.

---

## 13. Relationship to Other Documents

| Document | Relationship |
|----------|--------------|
| [Interview Protocol](./interview-protocol.md) | Defines how to conduct interviews |
| [Product Discovery](../product-discovery.md) | Defines the hypotheses being tested |
| [Findings](./findings.md) | Where findings are recorded |
| [Participant Summary](./participant-summary.md) | Where participant profiles are recorded |
| [Evidence Matrix](./evidence-matrix.md) | Where hypothesis statuses are recorded |
| [Phase 1 Exit Report](../phase-1-exit-report.md) | Where the Go/No-Go decision is recorded |

---

## 14. Open Questions

The following questions remain open and should be resolved before
interviews begin:

- Should interviews be recorded (audio/video) or only transcribed?
  Current decision: with consent, audio is recorded; otherwise,
  notes are taken live.
- Should participants be shown a prototype during the interview?
  Current decision: no; the interview is discovery only.
- Should follow-up interviews be conducted with the same
  participants after Phase 5? Possibly, but not in Phase 1.
- How is willingness to try converted into actual alpha testing?
  See `docs/phase-5-exit-report.md` for the alpha plan (future).
- Should participants be compensated? Current decision: no cash
  compensation; goodwill and reciprocal time are expected.
- How is participant diversity ensured beyond persona coverage?
  Ongoing consideration; recruit through diverse channels.

---

## 15. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The interview protocol is finalised
- The test plans are validated with a pilot interview
- The evidence classification scheme is applied successfully
- The anti-patterns are documented and internalised
- Open questions have been resolved or explicitly deferred
- The document is reviewed for consistency with
  [`docs/research/interview-protocol.md`](./interview-protocol.md)
