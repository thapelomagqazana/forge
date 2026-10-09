# Product Discovery

- **Document type:** Report
- **Status:** Draft
- **Version:** 0.1.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-09
- **Supersedes:** —
- **Superseded by:** —

---

## 1. Purpose

This document records what we believe about the problem Forge solves,
**before** any interviews are conducted. It is the hypothesis that
Phase 1 will test.

This document is deliberately incomplete. Evidence is collected during
Phase 1 interviews and recorded in
[`docs/research/findings.md`](./research/findings.md). Hypothesis
statuses are tracked in
[`docs/research/evidence-matrix.md`](./research/evidence-matrix.md).

The output of this document is a validated (or refuted) product
thesis that Phase 2 can build against.

---

## 2. Scope

**In scope:**

- The problem Forge intends to solve
- The users who experience that problem
- The lifecycle within which the problem occurs
- The current alternatives users rely on
- Explicit problem hypotheses to be tested
- Explicit separation of assumptions from facts

**Out of scope:**

- Solution design (see [`docs/mvp-scope.md`](./mvp-scope.md))
- Architecture (see [`docs/architecture.md`](./architecture.md))
- Specification of Blueprint, Template, Component, Policy (see their
  respective `*-spec.md` documents)
- Recorded findings (see [`docs/research/findings.md`](./research/findings.md))

---

## 3. Problem Statement

Software teams repeatedly establish engineering standards for their
repositories — testing, CI, security configuration, documentation,
structure — but those standards are not maintained consistently as
repositories evolve.

Six months after creation, a repository may still work correctly while
no longer matching the engineering foundation the team intended.
Standards erode through:

- Legitimate developer modifications that become permanent
- Dependency and tooling changes not propagated everywhere
- Priorities shifting and older standards receiving less attention
- New standards introduced without retrofitting old repositories

This erosion is not systematically detected. Teams notice drift by
chance — during code review, during an incident, during a security
audit — rather than through continuous verification.

The result is:

- **Inconsistent repositories**: Two repositories in the same team can
  have different testing, CI, and security postures despite sharing
  the same nominal standards.
- **Lost intent**: The original reason for a configuration choice is
  often unknown six months later.
- **Expensive correction**: Fixing drift across many repositories is
  manual, error-prone, and often deferred indefinitely.

The engineering foundation of a repository — what makes it a "proper
engineering project" and not merely a folder of code — is treated as a
one-time setup task rather than a maintained artifact.

**The specific problem Forge targets:**

> Engineering foundations are established once but not maintained as
> repositories evolve. There is no lightweight way to define,
> verify, explain, and safely evolve them.

---

## 4. Target Users

Users are defined with distinct problems, contexts, and motivations.
No two personas share the same problem.

Each persona is recorded with:

- **Role** — what the person does day-to-day
- **Context** — team size, repository count, environment
- **Problem** — the specific problem this persona experiences
- **Current workaround** — what they do today to cope
- **Motivation** — what they want to achieve
- **Frequency of pain** — how often the problem occurs
- **Adoption power** — whether they adopt, influence, or pay
- **Persona role** — Experiencer | Champion | Buyer | Learner

### Persona Roles

| Role | Meaning |
|------|---------|
| **Experiencer** | Directly feels the problem and uses the tool day-to-day |
| **Champion** | Adopts the tool for a team and drives wider adoption |
| **Buyer** | Controls budget and makes purchasing decisions |
| **Learner** | Uses the tool to build skills; not a commercial buyer |

A single tool cannot serve all personas equally. The MVP targets one
primary persona and one secondary persona. Others are served in later
phases.

### MVP Persona Priority

| Persona | Priority | Rationale |
|---------|----------|-----------|
| USER-001 Individual Developer | **Primary** | Fastest validation; open-source adoption wedge |
| USER-003 Tech Lead | **Secondary** | Early champion for team adoption |
| USER-002 Software Engineer | Phase 5+ | Reaches Forge through team adoption |
| USER-004 Platform Engineer | Phase 17+ | Commercial buyer; requires organisational maturity |
| USER-005 Engineering Manager | Phase 18+ | Influences budget; not a direct user |
| USER-006 Student or Learner | Phase 5+ | Community and feedback value; not a buyer |

---

### USER-001 — Individual Developer

**Persona role:** Experiencer
**MVP priority:** Primary

**Role**
Developer working alone or on small projects, responsible for the
entire engineering foundation of their repositories.

**Context**

- Creates 1–5 repositories per year
- Works across 1–3 languages
- Uses GitHub, GitLab, or similar
- Comfortable with CLI tools
- Not embedded in a team process

**Problem**
Starting a new project involves repeating the same engineering setup
work that was done for previous projects. The repetition is tedious
and error-prone, and setup items are frequently forgotten.

**Current workaround**
Copy a previous repository, use a personal template, or use a
framework-specific generator (`create-react-app`, `cargo new`, etc.).
Frequently forgets items that are added later, leading to
inconsistency across personal projects.

**Motivation**
Save time; start projects consistently; avoid forgetting important
setup; establish good habits early.

**Frequency of pain**
Episodic — a few times per year.

**Adoption power**
Adopts independently. No purchase authority, but drives adoption of
free tools.

**Success criteria**

- Project setup takes under 2 minutes
- Project starts with working tests, CI, and linting
- Consistent foundations across personal projects

**Why this persona is first**
Individual developers are the fastest path to validation. They adopt
tools without needing team approval, provide direct feedback, and are
the natural seed for later team and organisational adoption. Their
adoption is also what makes Forge credible as an open-source project.

---

### USER-002 — Software Engineer (Team Member)

**Persona role:** Experiencer
**MVP priority:** Phase 5+

**Role**
Engineer in a team of 5–20 developers, contributing to one or more
repositories owned by the team.

**Context**

- Contributes to shared repositories
- Follows team standards but does not define them
- Uses the same tools as the team
- Not involved in tooling decisions

**Problem**
Team standards exist but are not visible or continuously enforced.
Drift is noticed only when it causes a problem. Following standards
sometimes requires guessing, especially when the standards are
documented inconsistently.

**Current workaround**
Reads team documentation (when available), copies from other
repositories, asks colleagues. Notices drift only by chance during
code review or during incidents.

**Motivation**
Stay aligned with team expectations; avoid rework; understand what
"correct" looks like without needing to ask.

**Frequency of pain**
Continuous but low-grade. Sharp spikes during incidents or audits,
when the cost of drift becomes visible.

**Adoption power**
No adoption authority. Uses whatever the team has standardised.
Cannot bring Forge into the team on their own.

**Success criteria**

- Team standards are visible without needing to ask
- Failures are explained rather than merely reported
- Fewer surprises during code review

**Why this persona is not MVP**
This persona cannot adopt Forge independently. They reach Forge
*through* their team. Designing for them is valuable long-term, but
they cannot be the first adopters.

---

### USER-003 — Tech Lead

**Persona role:** Champion
**MVP priority:** Secondary

**Role**
Technical lead for a team of 5–15 developers, working across 5–20
repositories. Balances team autonomy with consistency of engineering
standards.

**Context**

- Responsible for defining and maintaining technical standards
- Reviews and approves architectural changes
- Coordinates across multiple repositories
- Frequently the person who notices drift first
- Has technical authority but not budget authority

**Problem**
Standards are defined once but degrade across repositories over time.
Keeping 10–20 repositories aligned is manual. There is no lightweight
way to check whether each repository still satisfies the team's
standards. Code review catches some drift, but not systematically.

**Current workaround**
Code reviews, spot checks, manually maintained scripts, occasional
cleanup efforts. Prioritises by visibility rather than by risk. Accepts
that some drift will remain undetected.

**Motivation**
Consistency without becoming a bottleneck; visibility into standards
compliance; reduced maintenance burden; fewer unpleasant surprises.

**Frequency of pain**
Weekly to monthly. Sharp spikes when new standards are introduced or
when a team member leaves.

**Adoption power**
Champion. Can introduce tools to the team if value is obvious. Cannot
make purchasing decisions, but shapes team tooling.

**Success criteria**

- Can answer "which of our repositories are aligned?" in under a
  minute
- Can introduce a standard and see it propagated
- Receives fewer "why is this repository set up differently?"
  questions

**Why this persona is secondary**
Tech leads amplify adoption. Once an individual developer has used
Forge, tech leads are the natural next step — they bring Forge from
one project to many. Designing for them is essential by Phase 7, when
`forge check` and `forge diff` become valuable.

---

### USER-004 — Platform Engineer

**Persona role:** Buyer
**MVP priority:** Phase 17+

**Role**
Engineer responsible for internal developer platforms, tooling, and
standards across an organisation. Manages tooling for 20–500+
repositories.

**Context**

- Owns golden paths, CI templates, internal developer portals
- Balances standardisation with team autonomy
- Frequently maintains a Backstage-style developer portal or custom
  internal tooling
- Reports to engineering leadership
- Has budget authority or strong influence over it

**Problem**
There is no lightweight way to know the engineering foundation state
of the organisation's repositories. Existing tooling either focuses on
dependencies (Dependabot, Renovate, Snyk), on runtime (observability
platforms), or on developer portal catalogues (Backstage). None
directly answers: "Which repositories still satisfy the current
engineering foundation?"

**Current workaround**
Custom scripts, manual surveys, developer portal cataloguing, spot
checks. Rarely comprehensive. Frequently answers questions only
*after* an incident or audit.

**Motivation**
Organisational visibility; compliance evidence; reduced platform team
maintenance burden; standards enforcement without becoming a
gatekeeper.

**Frequency of pain**
Continuous. Measured in organisational risk, not developer
convenience.

**Adoption power**
Buyer. Controls budget for tooling. Can mandate adoption across teams
but prefers bottom-up adoption to avoid political friction.

**Success criteria**

- Can answer organisation-wide questions about foundation state
- Can produce evidence for compliance or audit
- Reduces platform-team maintenance effort
- Teams adopt standards without being forced

**Why this persona is deferred**
Platform engineers require organisational maturity that does not exist
in Phase 1–5. Their problems are real but cannot be addressed until
Forge has proven value for individuals and teams. Designing for this
persona before validating the individual case would be premature.

---

### USER-005 — Engineering Manager

**Persona role:** Buyer
**MVP priority:** Phase 18+

**Role**
Manager of one or more engineering teams, owning delivery and quality
outcomes.

**Context**

- Reports to engineering leadership
- Does not usually write code
- Concerned with risk, compliance, and team efficiency
- Influences budget allocation for tooling

**Problem**
Cannot easily answer questions like:

- "Are our repositories compliant with our engineering standards?"
- "Which teams are following standards and which are not?"
- "Are we exposed to risks from outdated tooling?"

**Current workaround**
Relies on tech leads and platform teams for status. Rarely has
quantified answers. Relies on audits and incident reports for
visibility.

**Motivation**
Visibility for decision-making; defensible compliance evidence;
reduced organisational risk; alignment with engineering leadership's
priorities.

**Frequency of pain**
Monthly to quarterly. Primarily around audits, planning cycles, or
incidents.

**Adoption power**
Indirect. Does not use Forge directly. Influences budget through
platform engineering or through top-down mandates.

**Success criteria**

- Regular reports on foundation compliance
- Reduced audit preparation effort
- Clear evidence of risk reduction

**Why this persona is deferred**
Engineering managers are not users. They are stakeholders who
eventually benefit from the organisational layer. Designing for them
in Phase 1 would distort Forge toward reporting and dashboards, at
the expense of the CLI-first foundation that makes the product
credible.

---

### USER-006 — Student or Learner

**Persona role:** Learner
**MVP priority:** Phase 5+

**Role**
Developer learning software engineering practices. Creates 1–10
repositories per year, mostly for learning or portfolio purposes.

**Context**

- Not yet embedded in a professional team
- Uses modern tools and frameworks
- Wants to build good habits
- Follows tutorials and GitHub examples

**Problem**
Does not know what a "good" project foundation looks like. Existing
tutorials are inconsistent and often teach bad practices. Wants a
starting point that reflects real-world engineering standards.

**Current workaround**
Follows tutorials, copies from GitHub, uses framework-specific
generators (`create-react-app`, etc.). Does not know what is missing
until told.

**Motivation**
Learn good practices; build a portfolio that reflects professional
standards; avoid learning bad habits.

**Frequency of pain**
Episodic. High during learning phases.

**Adoption power**
Very high — actively looking for guidance. Provides strong feedback
and community growth.

**Success criteria**

- Project starts with recognised good practices
- Understands *why* each setup item matters
- Can explain their project's structure in interviews

**Why this persona is not primary**
Students provide adoption, feedback, and community value, but they do
not experience the *recurring* foundation-drift problem that Forge
targets. They benefit most from `forge new` and `forge explain`, and
less from `forge diff` and `forge update`. Their inclusion is
strategically important but not central to the MVP.

---

### User Relationships

```text
                        ┌────────────────────┐
                        │  USER-005          │
                        │  Engineering Mgr   │
                        │  (Buyer)           │
                        └────────┬───────────┘
                                 │ influences budget
                                 ▼
                        ┌────────────────────┐
                        │  USER-004          │
                        │  Platform Engineer │
                        │  (Buyer)           │
                        └────────┬───────────┘
                                 │ defines standards
                                 ▼
                        ┌────────────────────┐
                        │  USER-003          │
                        │  Tech Lead         │
                        │  (Champion)        │
                        └────────┬───────────┘
                                 │ adopts for team
                                 ▼
                        ┌────────────────────┐
                        │  USER-002          │
                        │  Engineer          │
                        │  (Experiencer)     │
                        └────────────────────┘

                        ┌────────────────────┐
                        │  USER-001          │
                        │  Individual Dev    │
                        │  (Experiencer)     │
                        └────────────────────┘
                        (adopts independently)

                        ┌────────────────────┐
                        │  USER-006          │
                        │  Student           │
                        │  (Learner)         │
                        └────────────────────┘
                        (adopts for learning)
```

The arrows show influence, adoption, and payment flows — not
hierarchy. USER-001 and USER-006 are outside the team structure and
adopt independently.

---

### Cross-Persona Comparison

| Persona | Problem | Frequency | Role | Adopts at |
|---------|---------|-----------|------|-----------|
| USER-001 Individual Developer | Repetitive setup; forgetting items | Episodic | Experiencer | Phase 1 |
| USER-002 Software Engineer | Standards invisible; guessing | Continuous, low-grade | Experiencer | Phase 5+ |
| USER-003 Tech Lead | Drift across repositories | Weekly–monthly | Champion | Phase 7+ |
| USER-004 Platform Engineer | No organisation-wide visibility | Continuous | Buyer | Phase 17+ |
| USER-005 Engineering Manager | No compliance evidence | Monthly–quarterly | Buyer | Phase 18+ |
| USER-006 Student | Doesn't know good practices | Episodic | Learner | Phase 5+ |

The problems are **distinct**. No two personas share the same stated
problem. This confirms the design principle that Forge cannot serve
all personas with the same feature set.

---

### Personas and Hypotheses

Each persona is linked to specific hypotheses from § 7:

| Persona | Relevant hypotheses |
|---------|---------------------|
| USER-001 | HYP-002 (repetition) |
| USER-002 | HYP-004 (drift), HYP-006 (visibility) |
| USER-003 | HYP-004 (drift), HYP-005 (update tooling) |
| USER-004 | HYP-003 (duplication), HYP-006 (visibility) |
| USER-005 | HYP-006 (visibility) |
| USER-006 | HYP-002 (repetition) |

Interview recruitment must cover personas in proportion to their
contribution to hypothesis validation. See
[`docs/research/interview-protocol.md`](./research/interview-protocol.md)
§ 3.3 for the recruitment plan.

---

### What This Persona Set Is Not

This persona set is deliberately narrow. It excludes:

- **Security engineers** — they care about security tooling, not
  engineering foundations broadly. Forge's security features are
  supporting capabilities, not the product.
- **DevOps engineers** — their focus is on deployment and
  infrastructure, not repository structure.
- **Open-source maintainers** — they share some characteristics with
  USER-003, but their problems are more about contribution workflows
  than engineering foundations.
- **Non-technical stakeholders** — Forge is a developer tool. Non-
  technical personas reach it only through aggregate reports, which
  are a Phase 18+ concern.

Adding personas requires justification in an ADR. The narrowness is
intentional: it keeps Phase 1 focused on validating the core thesis
before expanding scope.

---

## 5. Repository Lifecycle

The problem Forge targets does not occur at a single moment. It
occurs across the lifecycle of a repository.

```text
Idea
 │
 ▼
┌──────────────────────┐
│ Repository creation  │  ← engineering foundation established
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Initial configuration│  ← tools wired up, CI configured
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Development          │  ← foundation begins to erode
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ CI / automation      │  ← foundation partially enforced
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Dependency changes   │  ← drift accumulates
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Team / people change │  ← original intent lost
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Architecture change  │  ← foundation no longer fits
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ Maintenance          │  ← drift is only occasionally visible
└──────────────────────┘
```

### Where the problem occurs

| Lifecycle stage | Foundation state | Problem visibility |
|-----------------|------------------|-------------------|
| Repository creation | Strong — foundation established | Problem invisible |
| Initial configuration | Strong — tooling wired up | Problem invisible |
| Development | Erosion begins | Occasionally visible |
| CI / automation | Partial enforcement | Partially visible |
| Dependency changes | Drift accumulates | Mostly invisible |
| Team / people change | Original intent lost | Invisible without documentation |
| Architecture change | Foundation no longer fits | Visible but deferred |
| Maintenance | Drift has accumulated | Visible when it causes problems |

### The drift window

The critical observation is the **drift window** between foundation
establishment (T = 0) and foundation failure (T = N):

- Foundation is established at T = 0
- Drift accumulates silently from T = 1 onward
- Drift is typically detected only when it causes a visible problem
- The longer the drift window, the more expensive the correction

Forge's value proposition is to **close the drift window** by making
foundation state continuously verifiable.

---

## 6. Current Alternatives

Users rely on a mix of existing tools. Each addresses part of the
problem but none addresses the full lifecycle.

### 6.1 Project Scaffolding Tools

| Tool | Strength | Limitation |
|------|----------|------------|
| GitHub template repositories | Simple, integrated | One-time; no updates; no verification |
| Cookiecutter | Mature templating | No update support; Python-only ecosystem |
| Copier | Supports template updates | Narrow scope; requires Copier adoption |
| Yeoman | Large ecosystem | Generation-focused; aging |
| `create-react-app` and framework generators | Excellent UX for specific frameworks | Single-framework; no cross-language foundation |
| `cargo new`, `dotnet new`, `go mod init` | Native; integrated | Language-specific |
| Spring Initializr | Excellent Java experience | Java-specific |
| Backstage scaffolder | Enterprise-grade; integrates with portals | Requires Backstage platform |

**Gap:** All are oriented toward *creation*. None addresses
*maintenance* of the foundation after creation.

### 6.2 Verification and Compliance Tools

| Tool | Strength | Limitation |
|------|----------|------------|
| Dependabot, Renovate | Dependency updates | Dependencies only |
| Snyk, Trivy, Grype | Vulnerability scanning | Security posture only |
| CodeQL, Semgrep | Static analysis | Code quality only |
| Pre-commit | Local hooks | Developer-machine-only; no repository state |
| Custom CI checks | Flexible | Repository-specific; not reusable |

**Gap:** None evaluates the *engineering foundation* — the combined
state of testing, CI, structure, security configuration,
documentation, and commands.

### 6.3 Developer Portals and Platforms

| Tool | Strength | Limitation |
|------|----------|------------|
| Backstage | Catalogue, scaffolder, plugins | Heavy platform investment |
| Internal developer portals | Custom to organisation | Expensive to build and maintain |
| Custom internal tooling | Tailored | Duplicated effort across organisations |

**Gap:** Portals are *hosted platforms*, not developer-machine tools.
They require organisational investment to build and adopt. They do not
provide a lightweight, CLI-first alternative.

### 6.4 Manual Approaches

| Approach | Strength | Limitation |
|----------|----------|------------|
| Copy-paste from existing repository | Zero setup | Propagates drift |
| Team documentation / wiki | Human-readable | Not enforced; goes stale |
| Code review | Human judgment | Inconsistent; does not scale |
| Manual scripts | Tailored | Unmaintainable; repository-specific |

**Gap:** All manual approaches require ongoing effort that is rarely
sustained.

### 6.5 The Gap

Existing alternatives cluster around two poles:

- **Creation tools** (templates, scaffolders, generators) — strong at
  T = 0, absent at T = N
- **Verification tools** (Dependabot, Snyk, CodeQL) — narrow scope,
  never foundation-level

No widely adopted tool:

- Treats the engineering foundation as a versioned, declarative
  artifact
- Continuously verifies that a repository still satisfies its declared
  foundation
- Detects drift between expected and actual state
- Safely evolves foundations without destroying developer changes
- Does all of this via a lightweight, cross-platform CLI

This gap is Forge's opportunity. Whether it is a *real* opportunity
is the central question of Phase 1.

---

## 7. Problem Hypotheses

This section records what we believe about the problem **before**
conducting interviews. Every hypothesis is an assumption, not a fact.

Hypotheses are recorded with the format:

- **Statement** — the claim
- **Rationale** — why we currently believe it
- **Assumption** — what we are assuming, made explicit
- **Falsification** — what would disprove the claim
- **Test Plan** — how the claim will be tested
- **Status** — Unknown until Phase 1 findings are synthesised
- **Confidence** — initial confidence, before evidence

Status values:

| Status | Meaning |
|--------|---------|
| Unknown | No evidence collected yet |
| Supported | Multiple independent confirmations |
| Partial | Some evidence, some contradiction |
| Unsupported | Evidence contradicts |
| Inconclusive | Insufficient evidence |

All hypotheses begin as `Unknown`.

---

### HYP-001 — Foundation drift is a real, recurring problem

**Statement**
Developers and teams struggle not only to create software
repositories consistently, but to maintain consistent engineering
foundations as repositories evolve.

**Rationale**
Repositories are created with strong initial standards (tests, CI,
security, documentation), but those standards are not usually enforced
continuously. Over time, files are deleted, configuration changes,
people leave, projects are copied, and standards erode. This is
observed anecdotally but not measured.

**Assumption**
That the *initial* state of a repository is typically strong, and that
erosion happens *after* creation. If repositories are typically created
without standards in the first place, the problem is different and
Forge's framing is wrong.

**Falsification**
This hypothesis is disproved if interviews show that:

- Teams rarely or never experience foundation drift
- Drift, when it occurs, is trivially correctable and not worth tooling
- Developers consider drift a normal, acceptable state
- Existing tools already solve drift adequately

**Test Plan**

- Interview questions in
  [`docs/research/interview-protocol.md`](./research/interview-protocol.md)
  § A.4 and § B.4
- Prompt participants for real examples of drift
- Record frequency, impact, and existing workarounds

**Status**
Unknown

**Confidence**
Initial: Medium. Based on observation and personal experience, not
on measured evidence.

---

### HYP-002 — Project setup is repetitive

**Statement**
Starting a new software project requires repeating the same
engineering setup work that was already done for previous projects.

**Rationale**
Every new repository typically begins with a similar set of files:
`.gitignore`, `README.md`, testing setup, CI configuration, linting,
formatting, and (where applicable) Docker and environment files.
Developers repeat this work many times over a career.

**Assumption**
That the repeated work is *meaningfully* similar across projects. If
every project is genuinely unique in its setup, no tool can reduce the
repetition.

**Falsification**
This hypothesis is disproved if:

- Developers report that setup rarely repeats because each project is
  distinctive
- Existing templates already cover the repetition adequately
- The repetition is small enough not to matter (e.g. under 5 minutes)

**Test Plan**

- Interview questions in
  [`docs/research/interview-protocol.md`](./research/interview-protocol.md)
  § A.1–§ A.4
- Ask participants to walk through the last repository they created
- Measure the reported setup time

**Status**
Unknown

**Confidence**
Initial: High. This is the most obviously true hypothesis, but also
the least differentiated from existing tools.

---

### HYP-003 — Teams duplicate repository configuration

**Statement**
Engineering teams duplicate repository configuration across multiple
repositories rather than centralising and sharing it.

**Rationale**
Most teams have more than one repository and want them to share
standards (CI, testing, security, linting). In practice, this is
achieved by copy-paste, by manually maintained internal templates, or
by repeated recreation. Centralisation is rare because the tooling for
it is either heavy (Backstage, internal platforms) or nonexistent.

**Assumption**
That teams *want* centralisation but lack a lightweight mechanism for
it. If teams are content with duplication, this hypothesis is less
relevant.

**Falsification**
This hypothesis is disproved if:

- Teams report they do not want centralised repository standards
- Teams already have a working lightweight centralisation mechanism
- Duplication is deliberate and unproblematic

**Test Plan**

- Interview questions in
  [`docs/research/interview-protocol.md`](./research/interview-protocol.md)
  § B.1–§ B.3
- Ask participants to describe how a change to team standards
  propagates across repositories

**Status**
Unknown

**Confidence**
Initial: Medium. Strong intuition, but requires confirmation.

---

### HYP-004 — Engineering standards drift after repository creation

**Statement**
Once a repository has been created, its engineering standards drift
away from what the team originally intended. This drift is not
systematically detected.

**Rationale**
Standards erode for four common reasons:

- Developers make legitimate exceptions that become permanent
- Dependencies and tooling evolve and are not updated everywhere
- Priorities change and old standards receive less attention
- New standards are introduced but not retrofitted to old repositories

Teams notice drift by chance (during code review, during an incident),
not by systematic detection.

**Assumption**
That drift is *systematically invisible* today. If teams already
detect drift effectively, Forge's value proposition is weaker.

**Falsification**
This hypothesis is disproved if:

- Teams report they already detect drift continuously
- Drift detection is built into existing workflows
- Drift is detected but considered unimportant

**Test Plan**

- Interview questions in
  [`docs/research/interview-protocol.md`](./research/interview-protocol.md)
  § B.4
- Ask for real examples of drift discovery
- Ask *how* drift was discovered (chance vs. deliberate)

**Status**
Unknown

**Confidence**
Initial: High. This is the central hypothesis for Forge's product
identity per [ADR-001](./decisions/ADR-001-forge-as-foundation-manager.md).

---

### HYP-005 — Existing template tooling handles updates poorly

**Statement**
Existing project template and scaffolding tools (Cookiecutter,
Copier, GitHub templates, language-native generators) do not
adequately support updating already-generated repositories without
losing developer modifications.

**Rationale**
Most template tools are oriented toward *generation* (T = 0), not
*evolution* (T = N). Cookiecutter does not support updates at all.
Copier supports updates via three-way merge, but is limited to
template-driven projects and is not widely adopted for cross-language
foundations. Copier is the closest competitor; its existence is a
signal that update is a real need.

**Assumption**
That update is a distinct problem from generation, and that existing
tools solve it poorly or not at all.

**Falsification**
This hypothesis is disproved if:

- Teams report satisfactory experiences with existing update tools
- Update is not a real concern
- Teams regenerate repositories rather than updating them

**Test Plan**

- Interview questions in
  [`docs/research/interview-protocol.md`](./research/interview-protocol.md)
  § C.2–§ C.4
- Ask participants to describe what happens when their templates
  change
- Compare against existing tools' documented capabilities

**Status**
Unknown

**Confidence**
Initial: Medium. The Copier comparison weakens this hypothesis; more
evidence is needed.

---

### HYP-006 — Teams lack visibility into repository foundation state

**Statement**
Engineering leaders and platform teams lack a lightweight way to see
the engineering foundation state of their repositories — which
repositories are aligned with current standards, which have drifted,
and which are outdated.

**Rationale**
Where visibility exists today, it is either:

- Manual (spreadsheets, spot checks)
- Expensive (Backstage, internal portals, custom tooling)
- Not focused on engineering foundations (Dependabot, Snyk, and
  similar tools focus on dependencies, not structure)

There is no widely adopted tool that answers: "Which of our
repositories still have the foundation they were supposed to have?"

**Assumption**
That this visibility is *valuable* to platform and engineering teams,
and that no lightweight solution currently exists.

**Falsification**
This hypothesis is disproved if:

- Teams report existing visibility is sufficient
- Teams report they do not need this visibility
- Existing tools already provide it at reasonable cost

**Test Plan**

- Interview questions in
  [`docs/research/interview-protocol.md`](./research/interview-protocol.md)
  § B.1–§ B.4
- Target platform engineers specifically
- Ask how they currently answer "which repositories have drifted?"

**Status**
Unknown

**Confidence**
Initial: Medium. Requires validation with platform engineering
interviews specifically.

---

### Hypothesis Relationships

The hypotheses are not independent. They form a chain:

```text
HYP-002 (repetition)
      │
      ▼
HYP-003 (duplication)          HYP-004 (drift)
      │                              │
      └──────────┬───────────────────┘
                 ▼
             HYP-001 (umbrella: foundation drift)
                 │
                 ├── HYP-005 (update tooling gap)
                 │
                 └── HYP-006 (visibility gap)
```

If HYP-001 is unsupported, the entire product thesis
([ADR-001](./decisions/ADR-001-forge-as-foundation-manager.md)) must be
revisited. If HYP-001 is supported but HYP-004 is unsupported, Forge's
product identity needs adjustment — the problem is real but the drift
framing is wrong.

---

## 8. Assumptions vs Facts

The following table makes explicit what is an assumption versus what
is an observed fact at the time of writing:

| Claim | Type |
|-------|------|
| Repositories are created with strong initial standards | Assumption |
| Standards erode after creation | Assumption |
| Existing tools handle updates poorly | Assumption |
| Copier exists and supports three-way merge | Fact (verifiable) |
| Cookiecutter does not support updates | Fact (verifiable) |
| Teams want centralised standards | Assumption |
| Platform teams lack visibility today | Assumption |
| Developers repeat setup work | Assumption |
| Existing scaffolding tools are widely used | Fact (verifiable) |
| The scaffolding category is mature | Fact (verifiable) |
| Backstage provides software templates and cataloguing | Fact (verifiable) |

Assumptions are the targets of Phase 1 interviews. Facts are anchors
that constrain the design space.

---

## 9. Product Thesis

If HYP-001 through HYP-006 are supported, the product thesis is:

> **Forge keeps the engineering foundation of repositories intact as
> they evolve.**
>
> Developers define a foundation declaratively. Forge creates a
> repository from it, verifies it continuously, detects drift,
> explains the reason for a finding, and safely evolves the
> foundation when it changes.
>
> The core lifecycle is:
>
> ```text
> CREATE → VERIFY → EXPLAIN → EVOLVE
> ```

This thesis is recorded in full in
[ADR-001](./decisions/ADR-001-forge-as-foundation-manager.md). It
depends on all six hypotheses. If any is refuted, the thesis must be
adjusted before Phase 2 begins.

---

## 10. Non-Goals

Explicitly, Forge is **not**:

- A generic code generator
- A CI replacement
- A deployment platform
- A project management system
- An architecture authority
- A generic linter
- A dependency scanner
- A developer portal
- A hosted platform (in Phase 1–5)

This boundary prevents scope creep and keeps the product coherent.

---

## 11. Open Questions

The following questions remain open and must be resolved during
Phase 1:

- Is foundation drift painful enough that developers will adopt new
  tooling rather than copy an existing repository?
- Does the update problem (HYP-005) materially differentiate Forge
  from Copier?
- Will teams centralise their foundation definitions via a
  lightweight CLI, or is Backstage-style investment required?
- Is the CLI-first approach viable for platform engineers, or do they
  require a hosted dashboard from day one?
- Does the "Engineering Foundation" vocabulary resonate with
  developers, or does it feel abstract?

These questions are addressed by interviews in WBS 3.x and by
prototype validation in later phases.

---

## 12. Next Steps

1. Test each hypothesis via
   [`docs/research/interview-protocol.md`](./research/interview-protocol.md).
2. Record findings in
   [`docs/research/findings.md`](./research/findings.md).
3. Update hypothesis statuses in
   [`docs/research/evidence-matrix.md`](./research/evidence-matrix.md).
4. If any hypothesis is refuted, update this document and
   [ADR-001](./decisions/ADR-001-forge-as-foundation-manager.md).
5. Feed validated findings into
   [`docs/cli-ux-spec.md`](./cli-ux-spec.md),
   [`docs/blueprint-spec.md`](./blueprint-spec.md), and
   [`docs/mvp-scope.md`](./mvp-scope.md).

---

## 13. Status

**Draft.**

This document is under active development during Phase 1. It becomes
**Approved** when:

- All hypotheses have been classified in
  [`docs/research/evidence-matrix.md`](./research/evidence-matrix.md)
- The product thesis has been validated or adjusted
- Open questions have been resolved or deferred with rationale
