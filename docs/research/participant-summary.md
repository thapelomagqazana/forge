# Participant Summary

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

This document records anonymised summaries of every participant
interviewed during Phase 1. It exists to answer:

- Who was interviewed, in aggregate?
- What contexts did they represent?
- Was the recruitment target met?
- What gaps remain?

This document does **not** record:

- Findings (see [`findings.md`](./findings.md))
- Hypothesis classifications (see
  [`evidence-matrix.md`](./evidence-matrix.md))
- Verbatim quotes (see [`findings.md`](./findings.md))
- Personal identifying information of any kind

The participant summary is a **profile index**, not a findings
document.

---

## 2. Scope

**In scope:**

- Participant IDs and anonymised profiles
- Persona assignment
- Contextual metadata (role type, team size range, languages,
  tooling)
- Interview status (completed, planned, declined)
- Recruitment coverage against targets
- Interviewer notes about coverage gaps

**Out of scope:**

- Findings (see [`findings.md`](./findings.md))
- Hypothesis classifications (see
  [`evidence-matrix.md`](./evidence-matrix.md))
- Personal identifying information
- Consent records (stored separately, outside the repository)
- Recording files or transcripts (stored outside the repository)

---

## 3. Anonymity Rules

Participant identity is **never** recorded. The following rules apply
without exception.

### 3.1 What Is Recorded

- Participant ID (`P-001`, `P-002`, …)
- Persona assignment (`USER-001` through `USER-006`)
- Role category (e.g., "backend developer", "platform engineer")
- Team size range (e.g., "1–5", "6–20", "21–100", "100+")
- Repository count range (e.g., "1–5", "6–20", "21–100", "100+")
- Languages used (list, not by proficiency)
- Tooling categories (not specific vendors unless volunteered
  publicly)
- Interview date (month and year only)

### 3.2 What Is Never Recorded

- Full names
- Partial names
- Email addresses
- Employer names
- Specific repository names
- Specific repository URLs
- Product names that could identify the employer
- Geographic location beyond country
- Team names
- Manager names
- Any other identifying information

### 3.3 Participant ID Assignment

Participant IDs are assigned sequentially as interviews are
completed:

- First completed interview: `P-001`
- Second completed interview: `P-002`
- And so on.

Participant IDs are **not** reassigned if an interview is cancelled.
Cancelled interviews retain their IDs with the status `Cancelled` so
that the numbering is not misleading.

---

## 4. Participant Profile Template

Each participant entry follows this template.

```text
Participant ID:       P-NNN
Persona:              USER-NNN
Role category:        [e.g., backend developer]
Team size:            [1–5 | 6–20 | 21–100 | 100+]
Repository count:     [1–5 | 6–20 | 21–100 | 100+]
Languages:            [list]
Tooling categories:   [list — categories only, not vendors unless public]
Interview date:       YYYY-MM
Interview duration:   NN minutes
Recording consent:    Yes | No
Interview status:     Completed | Cancelled | Declined | Planned
Notes:                [optional anonymised note]
```

The **Notes** field is used only for context that does not identify
the participant. Examples:

- "Team primarily uses Go for internal services."
- "Described an internal platform engineering function."
- "Reported experience with both small and large teams."

The Notes field never contains:

- Quotes (those belong in `findings.md`)
- Findings (those belong in `findings.md`)
- Personal details

---

## 5. Participant List

This section is populated during Phase 1. Until then, it is a
template.

### 5.1 Recruitment Target

From [`docs/research/interview-protocol.md`](./interview-protocol.md)
§ 3.3:

| Persona | Target count |
|---------|--------------|
| USER-001 Individual Developer | 1–2 |
| USER-002 Software Engineer | 2–3 |
| USER-003 Tech Lead | 1–2 |
| USER-004 Platform Engineer | 1–2 |
| USER-005 Engineering Manager | 1 |
| USER-006 Student or Learner | 0–1 |
| **Total** | **5–10** |

### 5.2 Participants Interviewed

| ID | Persona | Role category | Team size | Repo count | Status |
|----|---------|---------------|-----------|------------|--------|
| — | — | — | — | — | — |

*(Populated as interviews are completed)*

### 5.3 Participants Declined or Cancelled

| ID | Persona | Status | Reason (if provided) |
|----|---------|--------|----------------------|
| — | — | — | — |

### 5.4 Planned but Not Yet Conducted

| ID | Persona | Status |
|----|---------|--------|
| — | — | — |

---

## 6. Participant Details

Each participant receives a detailed entry. Entries are ordered by
participant ID.

*(Populated as interviews are completed)*

### P-001 — Placeholder

```text
Participant ID:       P-001
Persona:              USER-NNN
Role category:        [role category]
Team size:            [range]
Repository count:     [range]
Languages:            [list]
Tooling categories:   [list]
Interview date:       YYYY-MM
Interview duration:   NN minutes
Recording consent:    Yes | No
Interview status:     Completed
Notes:                [optional anonymised note]
```

### P-002 — Placeholder

```text
Participant ID:       P-002
Persona:              USER-NNN
Role category:        [role category]
Team size:            [range]
Repository count:     [range]
Languages:            [list]
Tooling categories:   [list]
Interview date:       YYYY-MM
Interview duration:   NN minutes
Recording consent:    Yes | No
Interview status:     Completed
Notes:                [optional anonymised note]
```

---

## 7. Coverage Analysis

After interviews are complete, coverage is analysed against the
recruitment target.

### 7.1 Persona Coverage

| Persona | Target | Actual | Status |
|---------|--------|--------|--------|
| USER-001 Individual Developer | 1–2 | — | — |
| USER-002 Software Engineer | 2–3 | — | — |
| USER-003 Tech Lead | 1–2 | — | — |
| USER-004 Platform Engineer | 1–2 | — | — |
| USER-005 Engineering Manager | 1 | — | — |
| USER-006 Student or Learner | 0–1 | — | — |
| **Total** | **5–10** | **—** | **—** |

Status values:

- **Met:** Actual is within target range
- **Under:** Actual is below target range
- **Over:** Actual is above target range (acceptable)
- **Missing:** Actual is 0

### 7.2 Diversity Coverage

Beyond persona, the following dimensions are tracked to detect
sampling bias.

| Dimension | Categories observed | Notes |
|-----------|---------------------|-------|
| Team size | — | — |
| Repository count | — | — |
| Languages | — | — |
| Tooling | — | — |
| Industry (if volunteered) | — | — |
| Geography (country only) | — | — |

### 7.3 Coverage Gaps

Coverage gaps are documented explicitly. Examples:

- "No participant from USER-004 Platform Engineer; findings reflect
  only individual and team-level perspectives."
- "All participants from the same geography; findings may not
  generalise across regions."
- "Limited exposure to certain languages (Rust); findings may not
  apply to Rust ecosystems."

Gaps are recorded even when they are unavoidable. Unrecorded gaps
create false confidence.

### 7.4 Saturation Assessment

Saturation is reached when new interviews produce no new categories
of findings. This is assessed in
[`docs/research/evidence-matrix.md`](./evidence-matrix.md) § 5, but
the raw signal (consecutive interviews with no new categories) is
recorded here.

| Interview | New categories introduced | Cumulative categories |
|-----------|---------------------------|----------------------|
| P-001 | — | — |
| P-002 | — | — |
| … | … | … |

Saturation is reached when the last 3 interviews introduce no new
categories.

---

## 8. Recruitment Funnel

The recruitment funnel is tracked to understand conversion.

| Stage | Count | Notes |
|-------|-------|-------|
| Contacted | — | — |
| Responded | — | — |
| Scheduled | — | — |
| Completed | — | — |
| Declined | — | — |
| Cancelled | — | — |

The funnel is a diagnostic tool, not a metric to optimise. Low
response rates suggest the recruitment channel is not appropriate for
the target personas.

---

## 9. Interviewer Notes

Interviewer notes record observations about the interview process
itself, not the participants' findings.

Examples:

- "The interview protocol worked well for USER-001 but needed
  significant follow-up probing for USER-004."
- "Question A.4 was consistently misunderstood; recommend
  rewording."
- "The 30-minute time limit was too short for platform engineers."

These notes inform future interview rounds and refinement of the
protocol.

### 9.1 Protocol Observations

| Observation | Affected persona | Recommendation |
|-------------|------------------|----------------|
| — | — | — |

### 9.2 Recruitment Observations

| Observation | Recommendation |
|-------------|----------------|
| — | — |

---

## 10. Ethical Compliance

This section records ethical compliance for the interview round.

### 10.1 Consent

Every participant is informed of:

- The purpose of the interview (discovery, not sales)
- The anonymity policy
- Their right to stop at any time
- Their right to skip any question
- The recording policy (if applicable)

Consent is recorded **outside** this repository (e.g., in a signed
form or email trail).

### 10.2 Data Handling

- Recordings are deleted after findings are recorded (unless the
  participant agreed to longer retention)
- Raw notes are deleted after Phase 1 exit
- Only anonymised summaries are committed to the repository
- No personal information is committed

### 10.3 Participant Rights

Every participant has the right to:

- Withdraw from the study at any time
- Request deletion of their findings
- Request a copy of the anonymised findings related to their
  interview

These rights are honoured without question.

### 10.4 Compliance Confirmation

- [ ] All participants consented to the interview
- [ ] All participants were informed of the anonymity policy
- [ ] All participants were informed of the recording policy
- [ ] No personal information has been committed to the repository
- [ ] Recordings will be deleted at Phase 1 exit
- [ ] Raw notes will be deleted at Phase 1 exit

---

## 11. Summary Statistics

*(Populated after all interviews are complete)*

### 11.1 Aggregate

| Metric | Value |
|--------|-------|
| Total interviews completed | — |
| Total interviews declined | — |
| Total interviews cancelled | — |
| Personas covered | — |
| Personas missing | — |
| Average duration | — |
| Recordings obtained | — |
| Consent obtained | 100% |

### 11.2 Coverage by Persona

| Persona | Count | Percentage |
|---------|-------|------------|
| USER-001 | — | — |
| USER-002 | — | — |
| USER-003 | — | — |
| USER-004 | — | — |
| USER-005 | — | — |
| USER-006 | — | — |

### 11.3 Coverage by Team Size

| Team size | Count |
|-----------|-------|
| 1–5 | — |
| 6–20 | — |
| 21–100 | — |
| 100+ | — |

### 11.4 Coverage by Repository Count

| Repository count | Count |
|------------------|-------|
| 1–5 | — |
| 6–20 | — |
| 21–100 | — |
| 100+ | — |

---

## 12. Completion Criteria

The participant summary is complete when:

- [ ] All completed interviews have a detailed participant entry
- [ ] All declined or cancelled interviews are recorded
- [ ] Coverage against persona targets is analysed
- [ ] Diversity coverage is analysed
- [ ] Coverage gaps are documented explicitly
- [ ] Saturation assessment is recorded
- [ ] Recruitment funnel is populated
- [ ] Interviewer notes about the process are recorded
- [ ] Ethical compliance is confirmed
- [ ] Summary statistics are populated
- [ ] No personal information appears anywhere in the document

---

## 13. Relationship to Other Documents

| Document | Relationship |
|----------|--------------|
| [Interview Protocol](./interview-protocol.md) | Defines how interviews are conducted |
| [Market Validation](./market-validation.md) | Defines what is tested during interviews |
| [Findings](./findings.md) | Where raw findings are recorded |
| [Evidence Matrix](./evidence-matrix.md) | Where findings are synthesised |
| [Product Discovery](../product-discovery.md) | Defines the hypotheses being tested |
| [Phase 1 Exit Report](../phase-1-exit-report.md) | Uses the summary as evidence of coverage |

---

## 14. Open Questions

The following questions remain open and should be resolved during
Phase 1:

- Should participant entries be kept permanently or deleted at
  Phase 1 exit? Current decision: retained as anonymised summaries,
  since they document the research effort.
- Should participant IDs be reused if a participant is re-interviewed
  in a later phase? Current decision: no; new IDs are assigned.
- Should the participant summary record tooling preferences? Current
  decision: yes, but only at the category level (e.g., "uses a
  templating tool", not the vendor name).
- Should recruitment sources be documented? Current decision: no;
  sources could identify participants indirectly.
- How should a participant who requests deletion be handled?
  Current decision: their findings are removed from `findings.md`
  and their entry is removed from this document; the total
  participant count is adjusted and the removal is noted.

---

## 15. Status

**Draft.**

This document is under active development during Phase 1. It becomes
**Approved** when:

- All completed interviews are recorded
- Coverage analysis is complete
- Saturation is assessed
- Ethical compliance is confirmed
- Open questions have been resolved or explicitly deferred
- No personal information appears in the document
- The document is reviewed for consistency with
  [`docs/research/interview-protocol.md`](./interview-protocol.md) § 9
  (Data Handling)
