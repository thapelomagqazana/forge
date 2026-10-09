# Forge Requirements Register

**Document type:** Register
**Status:** Approved
**Version:** 1.0.0
**Author:** @thapelomagqazana
**Created:** 2026-10-09
**Last Updated:** 2026-10-09
**Supersedes:** —
**Superseded by:** —

---

## 1. Purpose

This register is the canonical list of every requirement established
for the Forge project. Requirement identifiers are assigned here, once,
and referenced everywhere else.

This register:

- Defines the requirement identifier scheme
- Lists every requirement with status and traceability
- Provides the source of truth for implementation and test coverage
- Is referenced by specifications, ADRs, and code

---

## 2. Scope

**In scope:**

- Every requirement the project commits to
- Requirement identifiers, categories, and numbering rules
- Requirement status and lifecycle
- Traceability to sources and specifications

**Out of scope:**

- Requirement *definitions* (those live in the specifications)
- Test cases (those live with the code)
- ADRs (those live in [`docs/decisions/`](../decisions/))

This register indexes requirements. Specifications define them.

---

## 3. Identifier Scheme

Requirements use the format:

```text
[PREFIX]-[CATEGORY]-[NNN]
```

| Prefix | Meaning |
|--------|---------|
| `FR` | Functional Requirement |
| `NFR` | Non-functional Requirement |
| `SEC` | Security Requirement |
| `UX` | User Experience Requirement |
| `MKT` | Market Finding |

Categories: `CORE`, `CLI`, `BLUEPRINT`, `TEMPLATE`, `COMPONENT`,
`POLICY`, `CHECK`, `UPDATE`, `REGISTRY`, `PERF`, `COMPAT`, `RELI`.

`SEC-` is used without a category (format: `SEC-NNN`).

ADR identifiers use the format `ADR-NNN` and are owned by the
[decision log](../decisions/README.md).

Full rules are defined in
[`docs/documentation-guide.md`](../documentation-guide.md) § 8.

---

## 4. Status Lifecycle

```text
PROPOSED ──► ACCEPTED ──► IMPLEMENTED ──► VERIFIED
    │             │
    │             └──► DEFERRED
    │
    └──► REJECTED
```

| Status | Meaning |
|--------|---------|
| Proposed | Identified but not yet approved |
| Accepted | Approved; committed to implementation |
| Implemented | Code claims to satisfy the requirement |
| Verified | Tests demonstrate the requirement is satisfied |
| Deferred | Postponed to a future phase |
| Rejected | Determined not to be a requirement |
| Split | Replaced by two or more successors |
| Superseded | Replaced by another requirement |

---

## 5. Functional Requirements

### 5.1 Core Product Behaviour (`FR-CORE-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-CORE-001 | Forge must support a declarative Blueprint | Accepted | ADR-001 |
| FR-CORE-002 | Forge must validate a repository against its declared foundation | Accepted | ADR-001 |
| FR-CORE-003 | Forge must detect drift between expected and actual foundation state | Accepted | ADR-001 |
| FR-CORE-004 | Forge must evolve foundations safely without destroying developer changes | Accepted | ADR-001 |
| FR-CORE-005 | Forge must record the foundation a repository was created or adopted with | Accepted | ADR-001 |

### 5.2 CLI (`FR-CLI-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-CLI-001 | Forge must provide a `forge version` command | Accepted | Phase 2 |
| FR-CLI-002 | Forge must provide a `forge help` command that lists all commands | Accepted | Phase 2 |
| FR-CLI-003 | Forge must provide a `forge config` command to inspect configuration | Accepted | Phase 2 |
| FR-CLI-004 | Forge must support structured errors with stable exit codes | Accepted | Phase 2 |
| FR-CLI-005 | Forge must support both interactive and non-interactive execution | Accepted | Phase 2 |

### 5.3 Blueprint (`FR-BLUEPRINT-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-BLUEPRINT-001 | Blueprint must be represented in YAML | Accepted | blueprint-spec |
| FR-BLUEPRINT-002 | Blueprint must have a schema version | Accepted | blueprint-spec |
| FR-BLUEPRINT-003 | Blueprint must enforce required fields | Accepted | blueprint-spec |
| FR-BLUEPRINT-004 | Blueprint must support optional fields | Accepted | blueprint-spec |
| FR-BLUEPRINT-005 | Blueprint must validate types | Accepted | blueprint-spec |
| FR-BLUEPRINT-006 | Blueprint must apply defaults deterministically | Accepted | blueprint-spec |
| FR-BLUEPRINT-007 | Blueprint must produce actionable errors for invalid input | Accepted | blueprint-spec |
| FR-BLUEPRINT-008 | Blueprint must expose resolved configuration via inspection | Accepted | blueprint-spec |

### 5.4 Template (`FR-TEMPLATE-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-TEMPLATE-001 | Templates must have a manifest with identity and version | Accepted | template-spec |
| FR-TEMPLATE-002 | Templates must support variable substitution | Accepted | template-spec |
| FR-TEMPLATE-003 | Templates must support conditional file inclusion | Accepted | template-spec |
| FR-TEMPLATE-004 | Templates must be deterministic: same inputs produce same output | Accepted | template-spec |
| FR-TEMPLATE-005 | Templates must not execute arbitrary commands | Accepted | security-model |
| FR-TEMPLATE-006 | Templates must not escape the target directory | Accepted | security-model |

### 5.5 Component (`FR-COMPONENT-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-COMPONENT-001 | Components must be individually versioned | Accepted | component-spec |
| FR-COMPONENT-002 | Components must declare dependencies | Accepted | component-spec |
| FR-COMPONENT-003 | Components must declare conflicts | Accepted | component-spec |
| FR-COMPONENT-004 | Components must track ownership of generated files | Accepted | component-spec |
| FR-COMPONENT-005 | Components must support safe removal | Accepted | component-spec |

### 5.6 Policy (`FR-POLICY-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-POLICY-001 | Policies must be declaratively configurable | Accepted | validation-spec |
| FR-POLICY-002 | Policies must support severity levels | Accepted | validation-spec |
| FR-POLICY-003 | Policies must support exemptions with reasons | Accepted | validation-spec |
| FR-POLICY-004 | Policy evaluation must be deterministic | Accepted | validation-spec |

### 5.7 Validation and Drift (`FR-CHECK-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-CHECK-001 | Forge must validate repository structure against the foundation | Accepted | validation-spec |
| FR-CHECK-002 | Forge must report missing required files | Accepted | validation-spec |
| FR-CHECK-003 | Forge must report findings with a stable finding ID | Accepted | validation-spec |
| FR-CHECK-004 | Forge must produce machine-readable output in JSON | Accepted | validation-spec |
| FR-CHECK-005 | Forge must detect drift between expected and actual state | Accepted | ADR-001 |
| FR-CHECK-006 | Forge must report drift with category and severity | Accepted | ADR-001 |

### 5.8 Update (`FR-UPDATE-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-UPDATE-001 | Forge must detect developer modifications before updating | Accepted | update-model |
| FR-UPDATE-002 | Forge must refuse to silently overwrite developer changes | Accepted | update-model |
| FR-UPDATE-003 | Forge must support `--dry-run` for all update operations | Accepted | update-model |
| FR-UPDATE-004 | Forge must create a backup before any mutating update | Accepted | update-model |
| FR-UPDATE-005 | Forge must support rollback of failed updates | Accepted | update-model |

### 5.9 Registry (`FR-REGISTRY-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| FR-REGISTRY-001 | Forge must support publishing versioned templates | Deferred to Phase 16 | registry-spec |
| FR-REGISTRY-002 | Forge must verify checksums before installation | Deferred to Phase 16 | registry-spec |
| FR-REGISTRY-003 | Forge must verify signatures before installation | Deferred to Phase 16 | registry-spec |
| FR-REGISTRY-004 | Forge must support version pinning in generated projects | Deferred to Phase 16 | registry-spec |

---

## 6. Non-functional Requirements

### 6.1 Performance (`NFR-PERF-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| NFR-PERF-001 | CLI startup must be under 100ms for ordinary commands | Accepted | documentation-guide |
| NFR-PERF-002 | Forge check must complete in under 5 seconds on repositories with up to 10,000 files | Proposed | — |

### 6.2 Compatibility (`NFR-COMPAT-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| NFR-COMPAT-001 | Forge must run on Windows, Linux, and macOS | Accepted | documentation-guide |
| NFR-COMPAT-002 | Forge must produce identical output for identical inputs across platforms | Accepted | — |
| NFR-COMPAT-003 | Forge must support Go 1.22 and later | Accepted | Phase 2 |

### 6.3 Reliability (`NFR-RELI-`)

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| NFR-RELI-001 | Generation must be deterministic | Accepted | ADR-001 |
| NFR-RELI-002 | Generation must be atomic where practical | Accepted | security-model |
| NFR-RELI-003 | Generation must be idempotent | Accepted | security-model |
| NFR-RELI-004 | Failed updates must not leave the repository in an unknown state | Accepted | update-model |

---

## 7. Security Requirements

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| SEC-001 | No generated path may escape the target directory | Accepted | security-model |
| SEC-002 | No template may execute arbitrary commands without explicit opt-in | Accepted | security-model |
| SEC-003 | Forge must never silently overwrite protected developer files | Accepted | security-model |
| SEC-004 | Environment files must be excluded from version control by default | Accepted | security-model |
| SEC-005 | Forge must reject absolute paths in generated content | Accepted | security-model |
| SEC-006 | Forge must reject symlink escapes | Accepted | security-model |
| SEC-007 | Forge must not log secrets | Accepted | security-model |
| SEC-008 | Forge must verify package integrity before installation | Deferred to Phase 16 | registry-spec |
| SEC-009 | Forge must verify publisher signatures before installation | Deferred to Phase 16 | registry-spec |

---

## 8. User Experience Requirements

| ID | Requirement | Status | Source |
|----|-------------|--------|--------|
| UX-001 | Every error must include a suggested action | Accepted | CLI-ux-spec |
| UX-002 | Every mutating command must support `--dry-run` | Accepted | CLI-ux-spec |
| UX-003 | Every command must provide discoverable help | Accepted | CLI-ux-spec |
| UX-004 | Success output must state what happened, where, and what to do next | Accepted | CLI-ux-spec |
| UX-005 | Colour must never carry meaning alone | Accepted | CLI-ux-spec |
| UX-006 | Non-interactive execution must never prompt for input | Accepted | CLI-ux-spec |
| UX-007 | Machine-readable output must be free of terminal formatting codes | Accepted | CLI-ux-spec |

---

## 9. Market Findings

Market findings are observations from user research, not commitments.  
They inform decisions but do not themselves become requirements.

| ID | Finding | Status | Source |
|----|---------|--------|--------|
| MKT-001 | *(pending interviews)* | Unknown | — |
| MKT-002 | *(pending interviews)* | Unknown | — |
| MKT-003 | *(pending interviews)* | Unknown | — |

Statuses for market findings differ from requirements:

- **Supported** — multiple independent confirmations
- **Partial** — some evidence, some contradiction
- **Unsupported** — evidence contradicts
- **Unknown** — insufficient evidence

---

## 10. Traceability

Every requirement traces to a source. Valid sources are:

- An accepted ADR (`ADR-NNN`)
- A specification document (`docs/*-spec.md`)
- A research finding (`MKT-NNN`)
- An external constraint (regulation, platform, standard)

Every requirement will eventually trace forward to:

- An implementation (Phase 2+)
- A test (Phase 2+)
- A user-facing command or behaviour

The register does not track forward links during Phase 1. Those are
added when implementation begins.

---

## 11. Coverage Summary

| Prefix | Total | Accepted | Deferred | Proposed |
|--------|-------|----------|----------|----------|
| FR | 34 | 30 | 4 | 0 |
| NFR | 10 | 9 | 0 | 1 |
| SEC | 9 | 7 | 2 | 0 |
| UX | 7 | 7 | 0 | 0 |
| MKT | 3 | 0 | 0 | 3 (unknown) |
| **Total** | **63** | **53** | **6** | **4** |

This table is updated whenever a requirement is added or its status
changes.

---

## 12. Amendment Process

This register is amended by:

1. Adding new requirements to the appropriate section
2. Updating statuses as requirements progress
3. Recording supersessions when requirements are replaced
4. Updating the coverage summary in § 11
5. Updating the `Last Updated` field

Amendments do not require an ADR unless they change the identifier
scheme, add a new prefix, or add a new category.

---

## 13. Open Questions

- Should `MKT-` findings be promoted to formal requirements, or do
  they remain as evidence only?
- Should requirements have an explicit priority (must / should / may)
  in addition to status?
- Should the register be machine-readable (YAML/JSON) in addition to
  Markdown, to enable automated coverage checks in Phase 2?

---

## 14. Status

**Approved.**

This register governs the Forge requirement identifiers. Amendments
require a new version of this document.
