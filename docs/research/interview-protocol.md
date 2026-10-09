# Developer Interview Protocol

- **Document type:** Research
- **Status:** Approved
- **Version:** 1.0.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-09
- **Supersedes:** —
- **Superseded by:** —

---

## 1. Purpose

This protocol defines how Forge conducts developer interviews during
Phase 1. It exists to ensure that:

- Interviews test the hypotheses in
  [`docs/product-discovery.md`](../product-discovery.md) § 7
- Questions are neutral and do not pitch Forge
- Findings are captured consistently and can be compared
- Participants are treated consistently and respectfully
- Evidence is separated from interpretation

The output of the interviews is raw evidence recorded in
[`docs/research/findings.md`](./findings.md) and classified in
[`docs/research/evidence-matrix.md`](./evidence-matrix.md).

---

## 2. Scope

**In scope:**

- Interview question sets
- Participant recording structure
- Recruitment targets
- Interview conduct rules
- Evidence recording format
- Finding category taxonomy

**Out of scope:**

- Recruitment sourcing (see § 3.3 for targets, not channels)
- Consent forms (handled separately, outside this repository)
- Competitive analysis (see
  [`docs/product-discovery.md`](../product-discovery.md) § 6)
- Hypothesis classification (see `evidence-matrix.md`)

---

## 3. Interview Preparation

### 3.1 Prepare Interview Protocol

This document is the protocol. It is followed verbatim for every
interview to ensure comparability.

### 3.2 Recording Structure

Every participant is recorded with the following fields.

| Field | Description |
|-------|-------------|
| **Participant ID** | `P-001`, `P-002`, etc. Anonymous |
| **Persona** | USER-001 through USER-006 |
| **Role** | Job title or functional role |
| **Team size** | Number of engineers in the team |
| **Repository count** | Approximate count they interact with |
| **Languages** | Languages they work with |
| **Current tooling** | Tools they currently rely on |
| **Repository creation process** | How they start a new project |
| **Pain points** | Problem statements in their own words |
| **Current workaround** | What they do today to cope |
| **Frequency** | How often the pain occurs |
| **Impact** | Severity of the pain (high / medium / low) |
| **Desired outcome** | What they would want instead |
| **Direct evidence** | Verbatim quotes and concrete examples |

**Do not collect:**

- Full names
- Email addresses
- Employer names
- Specific repository URLs
- Any information not required to understand the problem

Participants are referred to only by ID in all recorded materials.

### 3.3 Recruit 5–10 Developers

The objective is **variation**, not statistical significance. A
recruitment target of 5 minimum, 10 maximum.

**Target diversity:**

| Persona | Target count | Reason |
|---------|--------------|--------|
| USER-001 Individual Developer | 1–2 | Primary MVP persona |
| USER-002 Software Engineer | 2–3 | Team-member perspective |
| USER-003 Tech Lead | 1–2 | Champion persona; drift perspective |
| USER-004 Platform / DevEx Engineer | 1–2 | Buyer perspective; visibility hypothesis |
| USER-005 Engineering Manager | 1 | Influence perspective |
| USER-006 Student or Learner | 0–1 | Optional; distinct learning perspective |

**Total:** 5 minimum, 10 maximum.

**Rule:** At least one participant must be from each of USER-001,
USER-003, and USER-004. These three personas are most directly
connected to the hypotheses in `product-discovery.md` § 7.

### 3.4 Conduct Interviews

Every interview follows the same rules:

- **Duration:** 30–45 minutes.
- **Format:** One-on-one. Video or voice. Prefer live conversation.
- **Recording:** With consent. Otherwise take notes live.
- **Opening:** State that this is discovery, not a product pitch.
- **Closing:** Thank the participant. Do not reveal the hypotheses.
- **Behaviour:** Listen more than talk. Target: participant speaks
  70% of the time.

**Do not pitch Forge.**

If the participant asks what you are building, respond:

> "I am researching how developers set up and maintain engineering
> standards across repositories. I would rather not describe my ideas
> yet, because it might bias your answers. After we finish, I am
> happy to share."

If the participant insists, keep the description to one sentence and
move on:

> "I am exploring tooling that helps teams keep repositories aligned
> with their engineering standards."

Do not describe specific features, commands, or design choices.

### 3.5 Record Evidence

Every finding is recorded with the following fields.

| Field | Description |
|-------|-------------|
| **FINDING-ID** | `FINDING-001`, `FINDING-002`, etc. |
| **Observation** | What was observed |
| **Evidence** | Verbatim quotes, concrete examples |
| **Frequency** | How often the pattern occurred |
| **Impact** | High / Medium / Low |
| **Current workaround** | What the participant does today |
| **Implication for Forge** | What this means, if anything |
| **Confidence** | High / Medium / Low |
| **Participant ID** | Which interview produced this |
| **Related hypothesis** | Which HYP-xxx this tests |

Findings are stored in
[`docs/research/findings.md`](./findings.md).

### 3.6 Identify Recurring Problems

Findings are grouped into categories after all interviews are
complete. The categories are:

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

Every finding belongs to exactly one category. Categories are used to
cluster patterns and to detect gaps (a category with no findings may
indicate a question gap, not a real absence of pain).

---

## 4. Interview Structure

The interview has four sections. Each section is bounded and follows
the same order for every participant.

### Opening (2 minutes)

> "Thank you for making time. I am researching how developers and
> teams set up software projects and keep them aligned with their
> engineering standards. There are no right or wrong answers — I am
> interested in what actually happens, not what should happen.
>
> Do you mind if I take notes? [or: Do you mind if I record this?]
>
> Before we start — is it okay if I ask about your recent experience
> with a project you have worked on?"

### Section A — Repository Creation (8–12 minutes)

Goal: understand the participant's current process for starting a new
repository. Test HYP-002 (repetition) and HYP-003 (duplication).

**Questions:**

1. Walk me through the last repository you created, from the moment
   you decided to start it.
2. Did you use a template? If yes, what kind, and who maintains it?
3. What gets generated automatically for you today?
4. What do you have to add manually afterwards?
5. How long does it typically take from "I need a new project" to
   "I am writing code"?

Follow-ups to use when relevant:

- "You mentioned [X]. Can you tell me more about that?"
- "How often does that happen?"
- "What would happen if you did not do that?"

### Section B — Engineering Standards (8–12 minutes)

Goal: understand what standards exist, how they are communicated, and
how drift is detected. Test HYP-004 (drift) and HYP-006 (visibility).

**Questions:**

1. What standards must every repository your team owns follow?
2. How are those standards communicated to developers?
3. How are they enforced?
4. How do you know whether a repository has drifted from those
   standards?
5. Can you give me an example of a repository that drifted? What
   happened?

Follow-ups to use when relevant:

- "How did you discover that drift?"
- "What did you do about it?"
- "How long had it been drifting?"
- "Who noticed first?"

### Section C — Template Maintenance (5–8 minutes)

Goal: understand how templates are updated and what happens to
customised files. Test HYP-005 (update tooling).

**Questions:**

1. If you use templates, how are they updated?
2. What happens when developers customise the generated files?
3. How do you distribute template changes to existing repositories?
4. What makes updating templates difficult?

**If the participant does not use templates**, skip to Section D. Do
not force the topic.

Follow-ups to use when relevant:

- "Has a template update ever caused problems?"
- "How do you decide whether to update an existing repository?"

### Section D — Existing Repositories (5–8 minutes)

Goal: understand how standards are retrofitted onto existing
repositories. Test HYP-003 (duplication) and HYP-004 (drift).

**Questions:**

1. What happens to repositories created before your current standards
   existed?
2. How do you bring them up to standard?
3. Is that automated, or is it manual?

Follow-ups to use when relevant:

- "How often do you do this?"
- "What is the hardest part?"

### Closing (2 minutes)

> "That is all my questions. Is there anything I should have asked
> but did not?"

Then thank the participant. If they asked earlier what you are
building, you may now briefly describe it in one or two sentences.
Do not turn this into a sales pitch.

---

## 5. Interview Conduct Rules

### 5.1 Neutrality

- Do not lead participants toward a specific answer.
- Do not use words from Forge's product vocabulary (foundation,
  blueprint, drift, etc.).
- Do not react with enthusiasm or disappointment to specific answers.

### 5.2 Behavioural Focus

- Ask about what participants **did**, not what they **would** do.
- "What happened the last time?" is better than "What would you do
  if...?"
- "Can you show me?" is better than "Can you describe?"

### 5.3 Follow-up Discipline

- When a participant mentions something interesting, follow up.
- When they mention something irrelevant, do not chase it.
- Silence is allowed. Let participants think.

### 5.4 Non-Pitching

- Do not describe Forge's features, commands, or design.
- Do not ask "would you use X?" — that is not discovery.
- Do not defend Forge if the participant criticises existing tools.
- If the participant asks about Forge, defer to the closing.

### 5.5 Consent

- Ask before recording.
- Ask before taking notes if the participant might be uncomfortable.
- Offer to share the findings (anonymised) after Phase 1.

---

## 6. Evidence Recording Template

For each finding, use this template:

```text
FINDING-ID: FINDING-NNN

Participant:  P-NNN
Persona:      USER-NNN
Category:     [one of the 11 categories]

Observation:
  What was observed. One or two sentences.

Evidence:
  Verbatim quotes and concrete examples.

Frequency:
  How often this pattern occurred across participants.

Impact:
  High / Medium / Low, with brief justification.

Current workaround:
  What the participant does today.

Implication for Forge:
  What this means for the product, if anything.

Confidence:
  High / Medium / Low.

Related hypothesis:
  HYP-NNN.
```

Every finding must be recorded. Findings that contradict the
hypotheses must be recorded with equal care.

---

## 7. Recurring Problem Categories

After all interviews are complete, findings are grouped into these
categories:

| Category | Definition |
|----------|------------|
| Project creation | Starting a new project |
| Configuration | Configuring tools, dependencies, environments |
| Testing | Setting up and maintaining test infrastructure |
| CI/CD | Continuous integration and delivery |
| Security | Security posture, secrets, dependency scanning |
| Documentation | README, architecture, CONTRIBUTING, etc. |
| Standards | Team-wide engineering standards |
| Templates | Template creation, maintenance, distribution |
| Maintenance | Ongoing upkeep of repositories |
| Drift | Divergence from intended standards |
| Updates | Applying changes to existing repositories |

Categories are used only for clustering. They are not a taxonomy of
solutions.

If a category has zero findings across all interviews, that is itself
a finding worth noting — it may indicate either a genuine absence of
pain or a gap in the interview questions.

---

## 8. Sample Size and Saturation

- **Minimum:** 5 interviews
- **Maximum:** 10 interviews
- **Target:** 7–8 interviews

Stop early if:

- New interviews produce no new findings (saturation reached)
- At least one participant from each priority persona has been
  interviewed (see § 3.3)

Continue if:

- Fewer than 5 interviews have been completed
- A priority persona has not been interviewed
- Key hypotheses remain `Unknown` due to insufficient evidence

Do not aim for statistical significance. Aim for honest exploration.

---

## 9. Data Handling

### 9.1 Storage

- Raw notes and recordings are stored outside the repository.
- Only anonymised findings are committed to
  [`docs/research/findings.md`](./findings.md).
- Participant IDs (`P-001`, etc.) are the only identifiers retained.

### 9.2 Retention

- Recordings: deleted after findings are recorded, unless the
  participant explicitly agrees to longer retention.
- Raw notes: retained until Phase 1 exit report is written, then
  deleted.
- Anonymised findings: retained indefinitely in the repository.

### 9.3 Access

- Only the interviewer has access to raw notes.
- Anonymised findings are accessible to all Forge contributors.

---

## 10. What to Do When Things Go Wrong

### 10.1 Participant wants to talk about Forge

Defer to the closing. Politely redirect:

> "I would rather not bias your answers. Can I ask about that after
> we finish?"

### 10.2 Participant is not engaged

Try to make the conversation more concrete:

> "Can you walk me through a specific repository you worked on
> recently?"

### 10.3 Participant disagrees with the premise

That is valuable evidence. Record it honestly. Do not argue.

> "That is a useful perspective. Can you tell me more about why?"

### 10.4 Participant is an expert in an adjacent area

Stay focused on the protocol. Do not chase their expertise into
unrelated topics.

---

## 11. Post-Interview Actions

Within 24 hours of each interview:

1. Transcribe key quotes and observations.
2. Assign a Participant ID if not already assigned.
3. Write up findings using the template in § 6.
4. Add each finding to
   [`docs/research/findings.md`](./findings.md).
5. Update
   [`docs/research/participant-summary.md`](./participant-summary.md)
   with the participant's profile (anonymised).
6. If a finding clearly contradicts or supports a hypothesis, note
   it tentatively (do not finalise classification until all
   interviews are complete).

After all interviews:

1. Classify every finding by category.
2. Update
   [`docs/research/evidence-matrix.md`](./evidence-matrix.md) with
   hypothesis statuses.
3. Update
   [`docs/product-discovery.md`](../product-discovery.md) § 7 with
   refined hypothesis statuses.
4. If any hypothesis is refuted, note it explicitly and adjust
   [`ADR-001`](../decisions/ADR-001-forge-as-foundation-manager.md)
   accordingly.

---

## 12. Open Questions

- Should interviews be recorded by default, or only with explicit
  consent?
- Should the interviewer be one person consistently, or may multiple
  people conduct interviews using this protocol?
- Should interviews be conducted before or after the specifications
  are drafted? Current plan: in parallel, since each informs the
  other.
- What is the exact recruitment channel? (Not covered here; handled
  outside the repository.)

---

## 13. Status

**Approved.**

This protocol governs all Phase 1 interviews. Amendments require a new
version of this document.
