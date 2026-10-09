# Forge Decision Log

- **Document type:** Register
- **Status:** Approved
- **Version:** 1.0.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-09
- **Supersedes:** —
- **Superseded by:** —

---

## 1. Purpose

This directory contains Forge's Architecture Decision Records (ADRs).
Every major architectural or product decision is recorded here.

An ADR captures **what** was decided, **why**, **what alternatives
were considered**, and **what the consequences are**. ADRs are the
traceable rationale behind Forge's specifications and code.

This README is the governing document for the decision log. It defines
how ADRs are written, reviewed, superseded, and indexed.

---

## 2. Scope

**In scope:**

- Decisions that affect Forge's architecture
- Decisions that affect Forge's product identity or positioning
- Decisions that close a non-trivial tradeoff
- Decisions that constrain future phases
- Decisions that would be expensive to reverse

**Out of scope:**

- Routine implementation choices (naming, formatting, file layout)
- Style and documentation conventions (governed by
  [`docs/documentation-guide.md`](../documentation-guide.md))
- Decisions already fully captured in a specification, unless they are
  architectural in nature

When in doubt, record the decision. Over-documenting decisions is
cheaper than reconstructing them later.

---

## 3. When to Record an ADR

Record an ADR when a decision:

- Chooses between competing technical approaches
- Establishes or changes a Forge concept (Foundation, Blueprint,
  Component, Policy, Drift, etc.)
- Affects the product's positioning or scope
- Constrains future phases
- Would be expensive to reverse
- Would reasonably be questioned by a future contributor

Do **not** record an ADR for:

- Typos, formatting, or style
- Reversible implementation details
- Choices made within a single file or function

---

## 4. ADR Lifecycle

An ADR moves through these statuses:

```text
PROPOSED ──► ACCEPTED ──► SUPERSEDED
                 │
                 └──► DEPRECATED
```

| Status | Meaning |
|--------|---------|
| **Proposed** | Drafted; under discussion; not yet binding |
| **Accepted** | Approved; binding on implementation |
| **Superseded** | Replaced by a newer ADR; retained for history |
| **Deprecated** | No longer relevant; retained for history |

Rules:

- Only **Accepted** ADRs constrain implementation
- ADRs are **never edited** after acceptance, except to update their
  `Status` field to `Superseded by ADR-NNN`
- A superseding ADR must reference the superseded ADR
- A superseded ADR must reference its successor
- Status transitions are recorded, not silently changed

---

## 5. ADR Format

Every ADR follows the structure defined in
[`docs/documentation-guide.md`](../documentation-guide.md) § 10 and
scaffolded in [`template.md`](./template.md).

Required sections:

1. **Status** — Proposed / Accepted / Superseded / Deprecated
2. **Date** — when the decision was made
3. **Deciders** — who decided
4. **Supersedes** — previous ADR, if any
5. **Superseded by** — successor ADR, if any
6. **Context** — what situation requires a decision
7. **Options Considered** — at least two, each with pros and cons
8. **Decision** — the chosen option, stated plainly
9. **Consequences** — positive, negative, neutral
10. **Related Requirements** — IDs from
    [`docs/requirements-register.md`](../requirements-register.md)
11. **Related ADRs** — superseded or related ADRs
12. **Notes** — optional additional context

---

## 6. File Naming

ADR filenames use the pattern:

```text
ADR-NNN-short-kebab-case-title.md
```

Examples:

- `ADR-001-forge-as-foundation-manager.md`
- `ADR-002-blueprint-as-source-of-truth.md`
- `ADR-003-three-way-merge-for-updates.md`

Rules:

- `NNN` is zero-padded to three digits
- Numbers are assigned sequentially and never reused
- Titles are lowercase, hyphenated, and descriptive
- Titles are stable once the ADR is created
- Filenames are stable once committed; renames are avoided

---

## 7. Review and Acceptance

An ADR moves from Proposed to Accepted when:

1. The author drafts the ADR with all required sections
2. The author opens a pull request (or equivalent review)
3. At least one reviewer confirms:
   - The context accurately states the problem
   - The options are genuinely considered
   - The consequences are honest (including negative ones)
   - The decision is consistent with existing ADRs
   - The referenced requirement IDs exist in the register
4. The ADR is merged and its status set to `Accepted`

For solo development, the author may act as their own reviewer, but
the review must still be recorded (for example, in the PR description
or commit message).

---

## 8. Superseding an ADR

When a decision changes:

1. Draft a new ADR that references the old one
2. In the new ADR's `Context`, explain why the old decision requires
   replacement
3. In the new ADR's `Decision`, state the new choice
4. In the new ADR's `Supersedes` field, name the old ADR
5. In the old ADR, change `Status` to `Superseded by ADR-NNN`
6. In the old ADR, set `Superseded by` to the new ADR's filename
7. Do not delete or rewrite the old ADR

The old ADR remains in the log as historical evidence.

---

## 9. Relationship to Other Documents

| Document | Relationship |
|----------|--------------|
| [`docs/documentation-guide.md`](../documentation-guide.md) | Defines ADR format, metadata block, and naming conventions |
| [`docs/requirements-register.md`](../requirements-register.md) | ADRs reference requirement IDs |
| `docs/*-spec.md` | Specifications trace back to ADRs |
| [`docs/architecture.md`](../architecture.md) | Architecture reflects accepted ADRs |
| [`docs/phase-1-exit-report.md`](../phase-1-exit-report.md) | Summarises ADRs from Phase 1 |

Every specification claim that is not obvious should trace to an ADR.

---

## 10. ADR Index

The following table is the canonical index of Forge's decision log.
It must be kept in sync with the directory contents.

| ID | Title | Status | Date |
|----|-------|--------|------|
| [ADR-001](./ADR-001-forge-as-foundation-manager.md) | Forge is a Foundation Manager, not a Template Generator | Accepted | 2026-10-09 |

*New ADRs are appended in numeric order.*

---

## 11. Index Maintenance

When a new ADR is added:

1. Assign the next available number
2. Create the file from [`template.md`](./template.md)
3. Add a row to the index table in § 10
4. Update the `Last Updated` field of this README
5. Commit the ADR and the index update together as a single change

When an ADR is superseded:

1. Change the old ADR's status field
2. Update the old ADR's `Superseded by` field
3. Update the index row for the old ADR to reflect the new status
4. Add the superseding ADR to the index
5. Update the `Last Updated` field of this README

When an ADR is deprecated:

1. Change its status to `Deprecated`
2. Update the index row
3. Update the `Last Updated` field of this README

The index is the authoritative discovery surface. If it and the
directory disagree, the directory is correct — fix the index.

---

## 12. Directory Contents

```text
docs/decisions/
├── README.md                                ← this document
├── template.md                              ← reusable ADR scaffold
└── ADR-NNN-short-title.md                   ← individual decisions
```

No subdirectories. If the log grows large enough to require grouping
(for example, by phase), a new ADR will propose the change.

---

## 13. Open Questions

- Should ADRs be required for **every** new top-level concept, or only
  when there is a genuine tradeoff?
- Should Proposed ADRs block the start of the next phase, or may they
  be deferred with a recorded rationale?
- Should there be a meta-ADR recording the decision to use ADRs, and
  if so, would that be circular or useful?
- Should superseded ADRs be moved to a `superseded/` subdirectory, or
  remain in place with updated status? Current decision: remain in
  place.

---

## 14. Status

**Approved.**

This README governs the Forge decision log. Amendments require a new
version of this document and, where they affect architecture, an ADR.
