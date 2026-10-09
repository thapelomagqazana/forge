# Competitive and Alternative Workflow Analysis

**Document type:** Report
**Status:** Approved
**Version:** 1.0.0
**Author:** @thapelomagqazana
**Created:** 2026-10-09
**Last Updated:** 2026-10-09
**Supersedes:** —
**Superseded by:** —

---

## 1. Purpose

This document catalogues the alternative workflows that developers use
today to create and maintain software repositories. It exists to answer
one question:

> What must Forge outperform, and what must it complement?

The critical insight from the brief: **do not define competitors merely
as companies.** The real competitors are alternative *workflows* — the
things developers actually do, not the products they nominally use.

This analysis is a hypothesis. It must be tested against interview
evidence before being treated as fact.

---

## 2. Scope

**In scope:**

- Project generation workflows
- Template maintenance workflows
- Repository configuration management workflows
- Developer portal and platform workflows
- Manual and copy-paste workflows
- Capability comparison matrix
- Switching cost analysis

**Out of scope:**

- Commercial vendor comparisons (pricing, licensing, sales motion)
- Competitive positioning for marketing purposes
- Alternative workflows in adjacent domains (deployment, monitoring,
  observability)

---

## 3. Catalogue of Existing Approaches

Existing approaches fall into five categories. Each is described with
its workflow, strengths, and limitations.

### 3.1 Repository Templates (GitHub/GitLab)

**Workflow:**
Mark a repository as a template. Other developers click "Use this
template" to create a new repository from it. GitHub creates a clean
copy with no commit history [https://raw.githubusercontent.com/github/docs/refs/heads/main/content/repositories/creating-and-managing-repositories/creating-a-template-repository.md#1].

**Strengths:**

- Zero setup — built into the platform
- Clean history — the new repository starts fresh
- Simple mental model — "this is a template"

**Limitations:**

- **No variable substitution.** File contents cannot be templated
  with project-specific values.
- **No update mechanism.** Once created, the repository has no
  relationship to the template.
- **Single-repository scope only.** No cross-repository composition.
- **No validation.** Nothing checks whether the generated repository
  matches the template.

### 3.2 Cookiecutter

**Workflow:**
Define a template directory with a `cookiecutter.json` file containing
prompts and default values. Run `cookiecutter <template>` to generate
a project directory with templated file names and contents [https://cookiecutter.readthedocs.io/_/downloads/en/latest/pdf/#12#1].

**Strengths:**

- Mature, widely used, Python ecosystem
- Variable substitution in file names and contents [https://onlinelibrary.wiley.com/doi/full/10.1002/spe.70024#2]
- Pre- and post-generation hooks [https://cookiecutter.readthedocs.io/_/downloads/en/latest/pdf/#12#1]
- Large library of community templates

**Limitations:**

- **No template update mechanism.** Once generated, a project cannot
  be updated from the template.
- **Python-specific.** Although the generated projects can be any
  language, the tool itself requires a Python interpreter [https://cookiecutter.readthedocs.io/_/downloads/en/latest/pdf/#12#1].
- **Hooks are arbitrary code execution.** Pre-gen and post-gen hooks
  run shell scripts or Python code without sandboxing [https://onlinelibrary.wiley.com/doi/full/10.1002/spe.70024#2].
- **No validation or drift detection.**

### 3.3 Copier

**Workflow:**
Define a template with a `copier.yml` file and Jinja-templated files.
Run `copier copy <template> <destination>` to generate a project. Run
`copier update` to apply template changes to an existing project
[https://copier.readthedocs.io/en/v6.0.0/updating/].

**Strengths:**

- **Template updates.** Copier is the closest competitor to Forge's
  update capability. `copier update` compares the template version
  used at creation with the current version and applies differences
  without overwriting local changes [https://copier.readthedocs.io/en/v6.0.0/updating/].
- Variable substitution in file names and contents
- Works with local paths and Git URLs [https://copier.readthedocs.io/en/v6.0.0/updating/]
- Jinja templating engine — powerful and well-known
- Non-overwriting by default [https://copier.readthedocs.io/en/v6.0.0/updating/]

**Limitations:**

- **Requires Copier adoption.** Templates must be Copier-compatible.
- **Python-specific tooling.** Requires Python 3.10+ and Git 2.27+
  [https://copier.readthedocs.io/en/v6.0.0/updating/].
- **Update works only for Git-based templates.** Uses Git tags for
  versions and diffs [https://copier.readthedocs.io/en/v6.0.0/updating/].
- **No concept of components or composition.** Templates are
  monolithic.
- **No drift detection.** `copier update` applies changes, but does
  not tell you whether the repository has drifted.
- **No validation.** Nothing checks whether the repository still
  satisfies the template's intent.

### 3.4 Yeoman

**Workflow:**
Install `yo` and a generator (e.g., `generator-webapp`). Run
`yo <generator>` and answer interactive prompts. Yeoman generates the
project using the generator's logic [https://yeoman.io/learning/index.html].

**Strengths:**

- Large ecosystem of generators
- Interactive prompting model
- Node.js ecosystem integration [https://yeoman.io/learning/index.html]

**Limitations:**

- **Generators are programs, not templates.** Writing a generator
  requires JavaScript programming [https://yeoman.io/learning/index.html].
- **No update mechanism.** Once generated, no relationship to the
  generator remains.
- **No template composition.** Generators are monolithic.
- **Requires global installation** of `yo` and generators
  [https://yeoman.io/learning/index.html].

### 3.5 Framework-Specific Generators

**Workflow:**
Framework-specific CLIs (`create-next-app`, `cargo new`,
`dotnet new`, `spring init`, etc.) generate framework-appropriate
project structures.

**Strengths:**

- Excellent UX for their specific framework
- Deep framework integration
- Frequently updated by framework maintainers

**Limitations:**

- **Single-framework.** A Next.js generator cannot generate a Go API.
- **No cross-framework foundation.** Cannot express shared engineering
  standards across languages.
- **No update mechanism.**
- **No validation or drift detection.**

### 3.6 Backstage Scaffolder

**Workflow:**
Define a `template.yaml` with parameters and a sequence of actions
(fetch skeleton, publish to Git, register in catalog). Developers fill
a form in the Backstage UI, and the scaffolder executes the actions
[https://notes.kodekloud.com/docs/Prep-Course-Certified-Backstage-Associate-CBA-Certification/Templates/Demo-Template-Basics/page#scaffolder-actions-at-a-glance#1].

**Strengths:**

- **Multi-step automation.** Can create repositories, configure
  cloud resources, register in catalogs, all in one workflow
  [https://notes.kodekloud.com/docs/Prep-Course-Certified-Backstage-Associate-CBA-Certification/Templates/Demo-Template-Basics/page#scaffolder-actions-at-a-glance#1].
- **Powerful variable substitution**.
- **Enterprise-grade** — RBAC, audit, catalog integration.
- **Opinionated golden paths** [https://notes.kodekloud.com/docs/Prep-Course-Certified-Backstage-Associate-CBA-Certification/Templates/Demo-Template-Basics/page#scaffolder-actions-at-a-glance#1].

**Limitations:**

- **Requires Backstage platform.** Not a standalone CLI.
- **Significant operational cost.** Running Backstage requires
  Node.js infrastructure, plugin management, RBAC configuration.
- **Template authoring complexity.** `template.yaml` is verbose and
  requires significant effort.
- **Not developer-machine-first.** Backstage is a hosted platform,
  not a CLI tool.
- **No drift detection.** Backstage creates components but does not
  continuously verify that they match the template's intent.
- **No update mechanism.** Once created, the scaffolded repository
  has no ongoing relationship with the template.

### 3.7 Internal Company Templates

**Workflow:**
A company maintains a reference repository internally. Developers copy
it, rename files, and adjust values manually. Or the company maintains
a custom generator script.

**Strengths:**

- Tailored to the organisation
- Can encode organisation-specific standards

**Limitations:**

- **Duplication and drift.** Copies diverge from the reference.
- **No update mechanism.** Changes to the reference don't propagate.
- **Maintenance burden.** Internal tooling is often under-resourced.
- **Knowledge concentration.** Usually maintained by one person.

### 3.8 Copy/Paste from Existing Repositories

**Workflow:**
Find a similar repository, copy its files, rename and adjust.

**Strengths:**

- Zero tooling required
- Works for any language or framework

**Limitations:**

- **Propagates drift.** Copies inherit the source repository's
  deviations from standards.
- **Inconsistent results.** Different developers copy from different
  sources.
- **No reproducibility.** The result depends on which repository was
  copied.

### 3.9 Manual Setup

**Workflow:**
Create each file manually. `mkdir`, `touch`, `git init`, write
configuration files from memory or documentation.

**Strengths:**

- Complete control
- No dependency on tooling

**Limitations:**

- **Error-prone.** Items are forgotten.
- **Slow.** Setup can take hours.
- **Inconsistent.** Different developers produce different setups.

### 3.10 Custom Scripts

**Workflow:**
Write a shell script, Makefile, or Python script that automates
project setup.

**Strengths:**

- Tailored to specific needs
- Can encode organisation-specific standards

**Limitations:**

- **Maintenance burden.** Scripts break when dependencies change.
- **No standardisation.** Every team writes its own.
- **No update mechanism.**
- **Often unmaintained.** Original author leaves, script rots.

---

## 4. Capability Matrix

The following matrix compares capabilities across alternative
workflows. Forge's capabilities are **hypotheses** — they represent
what Forge intends to provide, not what has been built.

**Legend:**

- ✓ = Capability present
- ✗ = Capability absent
- ~ = Capability partial or context-dependent
- F = Future (Forge intends to provide, but not in MVP)
- — = Not applicable

| Capability | GitHub Templates | Cookiecutter | Copier | Yeoman | Framework Gen | Backstage | Internal | Copy/Paste | Manual | Forge |
|------------|-----------------|--------------|--------|--------|---------------|-----------|----------|------------|--------|-------|
| **Project generation** | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ |
| **Blueprint (declarative model)** | ✗ | ~ | ~ | ✗ | ✗ | ~ | ✗ | ✗ | ✗ | ✓ |
| **Components (composable)** | ✗ | ✗ | ✗ | ✗ | ✗ | ~ | ✗ | ✗ | ✗ | ✓ |
| **Validation** | ✗ | ✗ | ✗ | ✗ | ✗ | ~ | ✗ | ✗ | ✗ | ✓ |
| **Drift detection** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |
| **Safe updates** | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |
| **Existing repo adoption** | ✗ | ✗ | ~ | ✗ | ✗ | ~ | ✗ | ✗ | ✗ | ✓ |
| **Organisation foundation** | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ~ | ✗ | ✗ | F |
| **Explainability** | ✗ | ✗ | ✗ | ✗ | ✗ | ~ | ✗ | ✗ | ✗ | ✓ |
| **CLI-first** | ✗ | ✓ | ✓ | ✓ | ✓ | ✗ | ~ | ✓ | ✓ | ✓ |
| **Cross-language** | ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | ~ | ✓ | ✓ | ✓ |
| **Non-executing templates** | ✓ | ✗ | ✗ | ✗ | ✓ | ~ | ~ | ✓ | ✓ | ✓ |
| **Version pinning** | ✗ | ✗ | ✓ | ✗ | ✗ | ~ | ✗ | ✗ | ✗ | ✓ |
| **Provenance** | ✗ | ✗ | ~ | ✗ | ✗ | ~ | ✗ | ✗ | ✗ | ✓ |
| **Deterministic output** | ✓ | ✓ | ✓ | ~ | ✓ | ✓ | ~ | ✗ | ✗ | ✓ |
| **Template registry** | ✗ | ✓ | ✓ | ✓ | ✗ | ✓ | ✗ | ✗ | ✗ | F |

### Notes on the Matrix

**Copier is the closest competitor.** It is the only tool that
provides safe updates for generated projects [https://copier.readthedocs.io/en/v6.0.0/updating/].
However, Copier's update mechanism applies only to Git-based templates
and requires Copier adoption. It does not provide validation, drift
detection, or a declarative blueprint model.

**Backstage provides the most complete platform** but is not a
developer-machine tool.  
It requires significant organisational
investment to operate. Backstage scaffolds components but
does not continuously verify that they remain aligned with their
templates.

**Forge's differentiated hypothesis** is the combination of:

- Declarative blueprint (not just a template)
- Composable components (not monolithic templates)
- Continuous validation (not just generation)
- Drift detection (not just creation)
- Safe updates (three-way merge, not overwrite)
- Explainability (provenance for every decision)
- CLI-first (not a hosted platform)

This combination does not exist in any single alternative workflow.

---

## 5. Switching Cost Analysis

### 5.1 Why Developers Would Switch

**From GitHub Templates:**

- Need for variable substitution and conditional files
- Need for update capabilities
- Need for validation across repositories
- Need for drift detection

**From Cookiecutter:**

- No update mechanism in Cookiecutter is a known gap
- Cookiecutter's hooks are unsafe (arbitrary code execution) [https://onlinelibrary.wiley.com/doi/full/10.1002/spe.70024#2]
- Need for a non-Python tool [https://cookiecutter.readthedocs.io/_/downloads/en/latest/pdf/#12#1]
- Need for validation and drift detection

**From Copier:**

- Copier is the closest alternative, so switching cost is highest
- Copier's update mechanism requires Git-based templates [https://copier.readthedocs.io/en/v6.0.0/updating/]
- Copier has no drift detection or validation
- Copier has no component composition
- Copier has no explainability
- Copier has no registry

**From Yeoman:**

- Generators require JavaScript programming [https://yeoman.io/learning/index.html]
- No update mechanism
- No composition

**From Framework Generators:**

- Need for cross-language engineering foundations
- Need for consistency beyond a single framework

**From Backstage:**

- Backstage is a platform investment; Forge is a CLI tool
- Backstage is not developer-machine-first
- Backstage does not detect drift or validate
- Forge can be adopted by individual developers before
  organisational investment

**From Manual/Copy-Paste:**

- Speed and consistency
- Avoid forgetting items
- Reproducibility

### 5.2 Why Developers Would Not Switch

**Habit and familiarity:**
Developers already know their current workflow. Switching requires
learning a new tool.

**Sufficient current tools:**
For single-repository, single-project workflows, GitHub templates are
often sufficient. The problem Forge addresses (drift) only
manifests at scale or over time.

**Trust and security:**
Forge executes in the developer's filesystem. Developers must trust it
not to destroy work. Alternatives like GitHub templates are
non-destructive.

**Adoption friction:**
Installing a new CLI tool requires: downloading a binary, ensuring it
runs on the developer's OS, learning its commands. Framework
generators are often already installed.

**Copier already exists:**

Copier provides updates [https://copier.readthedocs.io/en/v6.0.0/updating/].  
A developer who has adopted
Copier has little reason to switch unless Forge offers significantly
more.

**No perceived problem:**
If a developer does not experience drift as a problem, they will not
seek a solution.

### 5.3 What Forge Must Integrate With

**Must integrate with:**

- Git (repository detection, state tracking)
- GitHub/GitLab/Bitbucket (repository discovery, CI integration)
- CI systems (GitHub Actions, GitLab CI)
- Existing repository structures (adopt existing repos)
- Existing package managers (npm, pip, cargo, Go mod)

**Must not replace:**

- Git (Forge complements Git, not replaces it)
- CI systems (Forge runs within CI, not instead of it)
- Package managers (Forge respects existing dependency management)
- IDEs (Forge is a CLI, not an IDE plugin)

### 5.4 What Forge Must Avoid

**Must not require:**

- Python (unlike Cookiecutter/Copier)
- Node.js (unlike Yeoman) [https://yeoman.io/learning/index.html]
- A hosted platform (unlike Backstage)
- A framework-specific ecosystem (unlike create-next-app)

**Must not do:**

- Execute arbitrary code from templates (unlike Cookiecutter hooks) [https://onlinelibrary.wiley.com/doi/full/10.1002/spe.70024#2]
- Destroy developer modifications (unlike naive update)
- Require organisational investment to be useful (unlike Backstage)

---

## 6. Forge's Wedge

### 6.1 The Differentiated Hypothesis

Forge should **not** compete primarily as another boilerplate generator.

Forge's differentiated hypothesis is:

> **Managing the engineering foundation throughout the repository
> lifecycle.**

This is a hypothesis, not a fact. It must be validated against market
evidence before being treated as a competitive position.

### 6.2 The Wedge Statement

Forge's wedge is the combination of four capabilities that no single
alternative provides:

1. **Declarative foundation.** A versioned, verifiable Blueprint that
   describes what the repository should be — not just files to copy.
2. **Continuous validation.** `forge check` verifies that the
   repository still satisfies its foundation.
3. **Drift detection.** `forge diff` identifies where the repository
   has diverged from its expected state.
4. **Safe evolution.** `forge update` applies foundation changes
   without destroying developer modifications.

No existing alternative provides all four.

### 6.3 What This Wedge Is Not

- It is not "a better template engine." Copier and Cookiecutter are
  mature; Forge will not out-template them.
- It is not "a lighter Backstage." Backstage serves a different
  audience (platform teams) with a different delivery model (hosted
  platform).
- It is not "a framework generator." Framework generators serve
  single-framework workflows; Forge targets cross-framework
  foundations.

### 6.4 Competitive Positioning Summary

| Against | Forge's distinction |
|---------|---------------------|
| GitHub Templates | Foundation lifecycle, not one-time copy |
| Cookiecutter | Validation, drift detection, safe updates |
| Copier | Blueprint model, components, drift detection, explainability |
| Yeoman | Declarative templates (not programs), safe updates |
| Framework Generators | Cross-framework foundation, not single-framework |
| Backstage | CLI-first, developer-machine, not hosted platform |
| Internal Templates | Portable, versioned, updatable |
| Copy/Paste | Reproducible, validated, consistent |
| Manual | Automated, deterministic |

### 6.5 The Critical Validation Question

The wedge hypothesis must be tested against interview evidence. The
critical questions are:

1. Do developers experience foundation drift as a real, recurring
   problem? (Tests HYP-001, HYP-004)
2. Do they currently use a workflow that fails to address it?
   (Tests HYP-005)
3. Would they adopt a new tool to address it? (Tests HYP-006)

If these questions are answered negatively, the wedge must be
revisited before Phase 2.

---

## 7. Assumptions vs Facts

| Claim | Type |
|-------|------|
| Copier is the closest competitor | Fact (Copier supports updates) [https://copier.readthedocs.io/en/v6.0.0/updating/] |
| Cookiecutter has no update mechanism | Fact (verifiable) |
| GitHub templates have no variable substitution | Fact (verifiable) |
| Backstage requires platform investment | Fact (verifiable) [citation:12] |
| Developers experience foundation drift | Assumption (HYP-001) |
| Existing tools fail to address drift | Assumption (HYP-005) |
| Teams lack visibility into drift | Assumption (HYP-006) |
| Developers would adopt a new tool | Assumption |
| The "engineering foundation" concept resonates | Assumption |

Assumptions are targets for interview validation.

---

## 8. Open Questions

- Is Copier's update mechanism sufficient for most developers, making
  Forge's update capability unnecessary?
- Do developers care about drift detection, or is generation
  sufficient?
- Would developers adopt a new tool for drift detection if the
  problem is invisible?
- Is the "engineering foundation" vocabulary compelling, or is it
  abstract?
- Does the CLI-first approach serve platform engineers, or do they
  require a dashboard?

These questions are addressed by interviews in Phase 1 and by
prototype validation in later phases.

---

## 9. Next Steps

1. Test the wedge hypothesis via developer interviews (WBS 3.x).
2. Record findings in
   [`docs/research/findings.md`](./research/findings.md).
3. Update hypothesis statuses in
   [`docs/research/evidence-matrix.md`](./research/evidence-matrix.md).
4. If the wedge is refuted, update this document and
   [ADR-001](./decisions/ADR-001-forge-as-foundation-manager.md).
5. Feed validated findings into
   [`docs/product-discovery.md`](./product-discovery.md) § 6.

---

## 10. Status

**Approved.**

This document is the competitive analysis for Phase 1. It is a
hypothesis until market evidence supports it. Amendments require a
new version of this document.
