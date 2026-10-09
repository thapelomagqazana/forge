# ADR-NNN: [Short Decision Title]

**Status:** Proposed
**Date:** YYYY-MM-DD
**Deciders:** [@thapelomagqazana]
**Supersedes:** —
**Superseded by:** —

---

## 1. Context

Describe the situation that requires a decision.

This section must answer:

- What problem or force is at play?
- What constraints exist (technical, product, market)?
- What happens if no decision is made?
- What prior decisions, requirements, or specifications does this
  relate to?
- Who is affected by the outcome?

Do **not** state the decision in this section. Context is for framing
only.

**Guidance:**

- If the context cannot be explained in three to five paragraphs, the
  decision is probably too large and should be split.
- Avoid mixing motivation with justification. Motivation belongs here;
  justification belongs in the Decision section.

---

## 2. Options Considered

Every ADR must consider at least two genuine options. A single-option
ADR is not a decision — it is a declaration.

### Option A — [Name]

Short description of the option.

**Pros:**

- ...
- ...
- ...

**Cons:**

- ...
- ...
- ...

### Option B — [Name]

Short description of the option.

**Pros:**

- ...
- ...
- ...

**Cons:**

- ...
- ...
- ...

### Option C — [Name] *(optional)*

Add additional options only when they are genuinely distinct. Do not
pad the list with strawmen.

**Pros:**

- ...

**Cons:**

- ...

---

## 3. Decision

State the chosen option plainly. One paragraph.

Then explain the reasoning: what tipped the balance toward this option
and against the alternatives. The reasoning must be honest — including
tradeoffs that were accepted rather than resolved.

**Guidance:**

- The decision must be unambiguous. A reader should be able to say
  "the decision was X" after one reading.
- If the decision depends on assumptions that could later be falsified,
  name those assumptions explicitly.
- If the decision is provisional or time-bounded, say so.

---

## 4. Consequences

### 4.1 Positive

What becomes easier or possible because of this decision?

- ...
- ...

### 4.2 Negative

What becomes harder, what is now constrained, what tradeoffs were
accepted?

- ...
- ...

Negative consequences must be honest. If an option was chosen despite
a real downside, record it here. Future readers need to know what was
accepted knowingly.

### 4.3 Neutral

What changed but is neither clearly positive nor negative?

- ...

---

## 5. Related Requirements

List the requirement IDs from
[`docs/requirements-register.md`](../requirements-register.md) that
this decision establishes, affects, or constrains.

- `FR-xxx` — brief note
- `SEC-xxx` — brief note
- `NFR-xxx` — brief note
- `UX-xxx` — brief note

If a requirement referenced here does not yet exist, note it explicitly:

> The following requirements must be added to the register when
> WBS 1.4 is complete: `FR-CORE-00N`.

Use `—` if there are no related requirements.

---

## 6. Related ADRs

List ADRs that this decision depends on, supersedes, or interacts with.

- `ADR-xxx` — brief note on the relationship
- `ADR-yyy` — brief note on the relationship

Use `—` if there are none.

---

## 7. Notes

Optional section. Use for:

- Open questions that this ADR defers
- Follow-up decisions that are anticipated
- Links to external discussion (issues, discussions, meeting notes)
- Any caveat that does not fit cleanly above

If nothing needs to be noted, write:

> None.

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

See [`ADR-001-forge-as-foundation-manager.md`](./ADR-001-forge-as-foundation-manager.md)
for a complete, accepted ADR produced from this template.

---

*End of template.*
