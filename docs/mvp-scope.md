# MVP Scope

- **Document type:** Specification
- **Status:** Draft
- **Version:** 0.1.0
- **Author:** @thapelomagqazana
- **Created:** 2026-10-09
- **Last Updated:** 2026-10-09
- **Supersedes:** —
- **Superseded by:** —

---

## 1. Purpose

This document defines the boundary of Forge's Minimum Viable Product
(MVP). It exists to prevent Phase 1 scope from leaking into Phases
2–10, and to give Phase 2–5 engineers an unambiguous target.

The MVP is deliberately small. Every capability in it must justify
its place. Every capability outside it must justify its exclusion.

This document answers:

- What commands does the MVP include?
- What commands does the MVP exclude?
- What is the end-to-end user journey?
- What are the acceptance criteria?
- What does the MVP explicitly not attempt to do?

This document is binding. If a feature is not listed in § 4 (MVP
Capabilities), it is not in the MVP.

---

## 2. Scope

**In scope:**

- MVP command surface
- MVP capabilities per command
- Explicit exclusions
- MVP user journey
- MVP acceptance criteria
- MVP quality targets
- Definition of done

**Out of scope:**

- Post-MVP roadmap (see [`docs/product-discovery.md`](./product-discovery.md))
- Implementation details (see [`docs/architecture.md`](./architecture.md))
- Full command specification (see [`docs/cli-ux-spec.md`](./cli-ux-spec.md))
- Blueprint schema (see [`docs/blueprint-spec.md`](./blueprint-spec.md))
- Template format (see [`docs/template-spec.md`](./template-spec.md))

---

## 3. MVP Philosophy

The MVP is designed around three principles.

### 3.1 Small Enough to Finish

The MVP includes five commands. It does not include every command
Forge will eventually have. The goal is a working, useful product
that a developer can install and use today.

### 3.2 Complete Enough to Judge

The MVP demonstrates the entire CREATE → VERIFY → EXPLAIN loop. A
developer can:

- Create a project from a foundation
- Adopt an existing repository
- Validate the foundation
- Understand the foundation
- List available templates

This is enough for a developer to judge whether Forge is worth
continuing to use.

### 3.3 Honest About Limitations

The MVP does not:

- Detect drift (`forge diff` is Phase 9)
- Continuously verify foundations (`forge check` is Phase 7)
- Update foundations safely (`forge update` is Phase 14)
- Support teams or organisations (Phase 17+)
- Distribute templates (registry is Phase 16)

These limitations are documented and communicated. The MVP does not
pretend to do what it does not do.

---

## 4. MVP Capabilities

### 4.1 MVP Commands

The MVP includes exactly these commands:

| Command | Purpose | Verb |
|---------|---------|------|
| `forge new` | Create a new project from a foundation | CREATE |
| `forge init` | Adopt an existing repository into Forge | CREATE (ADOPT) |
| `forge validate` | Validate a repository against its foundation | VERIFY |
| `forge explain` | Explain the current foundation | EXPLAIN |
| `forge template list` | List available templates | DISTRIBUTE |
| `forge version` | Show version information | SUPPORT |

Any command not in this table is **not** in the MVP.

### 4.2 `forge new`

**Purpose:** Create a new repository from a foundation.

**Capabilities:**

- Interactive wizard for project setup
- Non-interactive mode with flags
- Template selection
- Blueprint-based generation
- Deterministic output
- `--dry-run` support
- `--output <path>` for custom target directory
- `--format json` for machine-readable output
- `--quiet` and `--verbose`
- Safe filesystem writes (respects the security model)
- Automatic `forge.yaml` generation
- Git initialization (optional via flag)
- Next steps guidance in output

**Limitations:**

- No custom template installation (templates are bundled)
- No remote template fetching
- No component addition at creation time (components arrive in
  Phase 12)
- No policy configuration at creation time (policies arrive in
  Phase 8)

**Not in scope:**

- Post-generation command execution
- Dependency installation
- Git commit
- Remote repository creation

### 4.3 `forge init`

**Purpose:** Adopt an existing repository into Forge.

**Capabilities:**

- Repository technology detection
- Foundation recommendation based on detection
- Interactive confirmation
- Non-interactive mode with `--foundation <id>`
- Generates `forge.yaml` at repository root
- Does not modify source files
- Detects existing `forge.yaml` and refuses to overwrite
- `--dry-run` support
- `--format json`

**Limitations:**

- Detection supports only technologies listed in § 4.9
- Ambiguous detection requires explicit `--foundation`
- Does not modify source files (only writes `forge.yaml`)
- Does not install dependencies

**Not in scope:**

- Automatic repair of missing foundation elements
- Automatic creation of missing files
- Migration from other tools

### 4.4 `forge validate`

**Purpose:** Validate a repository against its declared foundation.

**Capabilities:**

- Reads `forge.yaml`
- Validates schema conformance
- Validates structure (required files and directories)
- Validates configuration (required tooling)
- Validates security baseline (declared controls)
- Reports findings with rule IDs
- Severity levels: `INFO`, `WARNING`, `ERROR`
- Remediation guidance for each finding
- `--format json` for machine-readable output
- Exit code 0 on success, 1 on validation failure
- `--file <path>` for custom config location

**Limitations:**

- Does not detect drift from a baseline (that is `forge diff`,
  Phase 9)
- Does not check components (that is Phase 12)
- Does not evaluate policies (that is Phase 8)
- Does not run repository code
- Does not modify the repository

**Not in scope:**

- Continuous validation in CI (that is `forge check`, Phase 7)
- Historical comparison
- Cross-repository validation

### 4.5 `forge explain`

**Purpose:** Explain the current foundation of the repository.

**Capabilities:**

- Reads `forge.yaml`
- Shows the Blueprint and its version
- Shows the Template and its version
- Shows declared components (if any)
- Shows active policies (if any)
- Shows provenance (source of each element)
- Human-readable output with clear sections
- `--format json` for machine-readable output

**Limitations:**

- Does not include policy evaluation results (that is `forge check`,
  Phase 7)
- Does not include drift information (that is `forge diff`, Phase 9)
- Does not query remote registries

**Not in scope:**

- Interactive exploration
- Sub-command depth (`forge explain policy testing` is future)
- Historical explanation

### 4.6 `forge template list`

**Purpose:** List available templates.

**Capabilities:**

- Lists all bundled templates
- Shows name, version, and description
- Deterministic ordering
- `--format json`

**Limitations:**

- Only bundled templates (no registry)
- No filtering or search
- No template details (`forge template inspect` is Phase 16)

**Not in scope:**

- Template installation
- Template publishing
- Template updates

### 4.7 `forge version`

**Purpose:** Show version information.

**Capabilities:**

- Shows Forge version, commit, build date, and Go version
- `--format json`

**Limitations:**

- None. This is a simple command.

### 4.8 Bundled Templates

The MVP ships with five templates:

| Template | Language | Purpose |
|----------|----------|---------|
| `python-fastapi` | Python | FastAPI API foundation |
| `go-api` | Go | HTTP service foundation |
| `go-cli` | Go | Command-line tool foundation |
| `typescript-node` | TypeScript | Node.js API foundation |
| `react-app` | TypeScript | React application foundation |

These templates are:

- Bundled with the Forge binary
- Versioned with Forge
- Tested against the MVP command surface
- Documented in `docs/template-spec.md` § 11

Additional templates are added in later phases.

### 4.9 Supported Languages

The MVP supports four languages:

| Language | Minimum version |
|----------|-----------------|
| Python | 3.12 |
| Go | 1.22 |
| TypeScript | 5.0 |
| Rust | 1.75 (planned, may not ship in MVP) |

Additional languages are added in later phases.

### 4.10 Bundled Blueprints

The MVP ships with five Blueprints matching the templates:

- `python-api`
- `go-api`
- `go-cli`
- `typescript-node`
- `react-app`

Blueprints are documented in `docs/blueprint-spec.md` § 12.

### 4.11 Configuration

The MVP supports:

- Reading `forge.yaml` from the repository root
- Writing `forge.yaml` on `forge new` and `forge init`
- Validating `forge.yaml` schema

The MVP does not support:

- Global configuration files
- Environment variable configuration
- Configuration inheritance

---

## 5. MVP Exclusions

The following are explicitly excluded from the MVP. Each exclusion has
a target phase or is deferred indefinitely.

### 5.1 Registry

**What it is:** A remote service for distributing templates and
components.

**Why excluded:** The registry is a large infrastructure project that
requires signatures, provenance, moderation, and versioning. It is
not needed to validate the MVP hypothesis (that developers want a
foundation manager).

**Target phase:** 16.

**Consequence:** The MVP ships with five bundled templates. Custom
templates require local installation (not supported in MVP).

### 5.2 Enterprise Features

**What it is:** SSO, RBAC, audit logs, compliance reporting, enterprise
integrations.

**Why excluded:** Enterprise features are premature. The MVP targets
individual developers and small teams.

**Target phase:** 20.

**Consequence:** The MVP has no authentication, no user accounts, no
permissions.

### 5.3 Dashboard

**What it is:** A web-based UI for viewing foundation state.

**Why excluded:** The MVP is CLI-first. A dashboard is only useful
once there is data from many repositories, which requires team and
organisation features.

**Target phase:** 19.

**Consequence:** All MVP output is terminal-based.

### 5.4 SSO and Identity

**What it is:** Single sign-on, user accounts, authentication.

**Why excluded:** The MVP is a local tool. There is no hosted service
to authenticate against.

**Target phase:** 20.

**Consequence:** The MVP does not track users.

### 5.5 Organisation Management

**What it is:** Centralised management of standards across an
organisation.

**Why excluded:** The MVP targets individual developers. Organisation
features require team features first.

**Target phase:** 18.

**Consequence:** The MVP has no concept of teams or organisations.

### 5.6 Advanced Update Engine

**What it is:** `forge update` with three-way merge and rollback.

**Why excluded:** The update engine is the most technically demanding
part of Forge. It requires base state tracking, merge logic, and
conflict resolution. It is not needed to validate the MVP hypothesis.

**Target phase:** 14.

**Consequence:** The MVP does not update foundations. Developers
regenerate or edit `forge.yaml` manually.

### 5.7 Drift Detection

**What it is:** `forge diff` that compares expected vs. actual state.

**Why excluded:** Drift detection requires a baseline (from
`forge new` or `forge init`) and a comparison algorithm. The MVP
focuses on validation, not drift.

**Target phase:** 9.

**Consequence:** The MVP does not detect drift.

### 5.8 Continuous Verification

**What it is:** `forge check` for CI usage.

**Why excluded:** `forge check` requires drift detection and CI
integration. It is not needed for the MVP.

**Target phase:** 7.

**Consequence:** The MVP does not run in CI. Developers run
`forge validate` locally.

### 5.9 Component System

**What it is:** Composable components (`forge add`, `forge remove`).

**Why excluded:** Components are an architectural enhancement. They
require the component model (Phase 10), composition rules, and
ownership tracking. Not needed for MVP.

**Target phase:** 12.

**Consequence:** The MVP does not compose components. Foundations
are monolithic.

### 5.10 Policy Engine

**What it is:** Configurable policies (`policies:` in `forge.yaml`).

**Why excluded:** Policies require the policy engine, severity
handling, and exemptions. Not needed for MVP.

**Target phase:** 8.

**Consequence:** The MVP uses hard-coded validation rules.

### 5.11 AI-Generated Foundations

**What it is:** Using AI to suggest or generate foundations.

**Why excluded:** AI introduces unpredictability, security concerns,
and reproducibility problems. It is not compatible with the MVP's
commitment to determinism.

**Target phase:** Not planned.

**Consequence:** The MVP has no AI features.

### 5.12 Large Template Ecosystem

**What it is:** Hundreds of templates covering every language and
framework.

**Why excluded:** A large template ecosystem requires maintenance,
quality control, and a registry. The MVP ships with five excellent
templates rather than 100 mediocre ones.

**Target phase:** Post-Phase 16.

**Consequence:** The MVP supports five templates and four languages.

### 5.13 IDE Integration

**What it is:** VS Code, JetBrains, or Neovim plugins.

**Why excluded:** IDE integration requires a stable CLI, a plugin
system, and maintenance for multiple IDEs. Not needed for MVP.

**Target phase:** Deferred indefinitely.

**Consequence:** The MVP is a terminal tool only.

### 5.14 Web Portal

**What it is:** A web interface for interacting with Forge.

**Why excluded:** The MVP is CLI-first. A web portal requires a
hosted service, which requires identity and organisation features.

**Target phase:** 19 (dashboard), or later.

**Consequence:** All MVP interaction is via the CLI.

### 5.15 Cloud Sync

**What it is:** Syncing foundations across machines via a cloud
service.

**Why excluded:** Cloud sync requires a hosted service and identity
management. Not needed for the MVP.

**Target phase:** Not planned.

**Consequence:** The MVP is fully local.

### 5.16 Multi-Repository Management

**What it is:** Managing foundations across many repositories at once.

**Why excluded:** Multi-repository management requires team or
organisation features. Not needed for the MVP.

**Target phase:** 17.

**Consequence:** The MVP operates on one repository at a time.

### 5.17 Custom Rule Plugins

**What it is:** User-defined validation rules via plugins.

**Why excluded:** Plugins require a plugin system, sandboxing, and
security review. Not needed for the MVP.

**Target phase:** Future phase (TBD).

**Consequence:** The MVP has a fixed rule set.

### 5.18 Post-Generation Hooks

**What it is:** Commands that run after generation (e.g., install
dependencies).

**Why excluded:** Hooks introduce arbitrary code execution. This
violates the security model.

**Target phase:** Possibly never, or with strict sandboxing.

**Consequence:** The MVP does not execute code from templates.

### 5.19 Remote Repository Creation

**What it is:** Creating a repository on GitHub/GitLab via API.

**Why excluded:** Remote repository creation requires authentication,
API integration, and error handling for external services. Not
needed for the MVP.

**Target phase:** Future phase (TBD).

**Consequence:** The MVP creates local repositories only. Developers
push to remotes manually.

### 5.20 Automatic Dependency Installation

**What it is:** Running `npm install`, `pip install`, `go mod
download`, etc. after generation.

**Why excluded:** Dependency installation requires network access,
package manager availability, and error handling. It also executes
external code, which the security model prohibits.

**Target phase:** Possibly never, or with explicit consent.

**Consequence:** The MVP generates a repository; the developer
installs dependencies manually.

### 5.21 Exclusion Summary

| Feature | Target phase | Reason for exclusion |
|---------|--------------|----------------------|
| Registry | 16 | Large infrastructure project |
| Enterprise features | 20 | Premature |
| Dashboard | 19 | Requires team data |
| SSO | 20 | Local tool |
| Organisation management | 18 | Requires team features |
| Advanced update engine | 14 | Technically demanding |
| Drift detection | 9 | Requires baseline |
| Continuous verification | 7 | Requires drift detection |
| Component system | 12 | Architectural enhancement |
| Policy engine | 8 | Configurable rules |
| AI features | Not planned | Non-deterministic |
| Large template ecosystem | Post-16 | Requires registry |
| IDE integration | Deferred | Requires plugin system |
| Web portal | 19+ | Requires hosted service |
| Cloud sync | Not planned | Requires hosted service |
| Multi-repo management | 17 | Requires team features |
| Custom rule plugins | Future | Requires plugin system |
| Post-generation hooks | Possibly never | Security violation |
| Remote repo creation | Future | External API integration |
| Dependency installation | Possibly never | Executes external code |

---

## 6. MVP User Journey

The MVP is designed to support one primary user journey.

### 6.1 The Journey

```text
┌───────────────────────────────────┐
│  1. Install Forge                 │
│     Download binary for OS        │
└───────────────┬───────────────────┘
                │
                ▼
┌───────────────────────────────────┐
│  2. forge new payments-api        │
│     Interactive wizard            │
│     Selects: Python, FastAPI,     │
│              PostgreSQL, pytest,  │
│              Docker, GitHub       │
│              Actions              │
└───────────────┬───────────────────┘
                │
                ▼
┌───────────────────────────────────┐
│  3. Select foundation             │
│     Forge recommends python-api   │
│     Developer confirms            │
└───────────────┬───────────────────┘
                │
                ▼
┌───────────────────────────────────┐
│  4. Generate repository           │
│     Forge creates:                │
│       payments-api/               │
│         README.md                 │
│         pyproject.toml            │
│         src/                      │
│         tests/                    │
│         Dockerfile                │
│         .github/workflows/ci.yml  │
│         forge.yaml                │
└───────────────┬───────────────────┘
                │
                ▼
┌───────────────────────────────────┐
│  5. Inspect forge.yaml            │
│     Developer reviews the         │
│     foundation declaration        │
└───────────────┬───────────────────┘
                │
                ▼
┌───────────────────────────────────┐
│  6. forge validate                │
│     Validates the generated       │
│     repository against its        │
│     foundation                    │
│     Result: PASS                  │
└───────────────┬───────────────────┘
                │
                ▼
┌───────────────────────────────────┐
│  7. Start development             │
│     cd payments-api               │
│     Install dependencies          │
│     Write code                    │
└───────────────────────────────────┘
```

### 6.2 The Journey in Commands

```bash
# 1. Install Forge
# (download binary, add to PATH)

# 2. Verify installation
forge version

# 3. Create a project
forge new payments-api

# (Forge runs an interactive wizard)

# 4. Enter the project
cd payments-api

# 5. Inspect the foundation
forge explain

# 6. Validate
forge validate

# 7. Start development
pip install -e .
pytest
```

### 6.3 The Adoption Journey (for existing repositories)

A developer with an existing repository can also adopt Forge:

```bash
# 1. Enter the repository
cd existing-project

# 2. Adopt into Forge
forge init

# (Forge detects technology, recommends a foundation)

# 3. Validate
forge validate

# 4. Continue development
```

### 6.4 The Non-Interactive Journey (for scripts and CI)

```bash
forge new payments-api \
  --template python-fastapi \
  --output ./projects \
  --non-interactive \
  --format json
```

### 6.5 Journey Completion Criteria

The journey is complete when:

- A repository is generated
- The repository's `forge.yaml` is valid
- `forge validate` returns exit code 0
- The developer can start writing code

The journey does not require:

- A running Forge service
- Network access
- A user account
- Registration
- Purchase

---

## 7. MVP Acceptance Criteria

The MVP is complete when the following criteria are met. Each
criterion is objectively testable.

### 7.1 Command Acceptance Criteria

#### 7.1.1 MVP-001: Create a Project from a Blueprint

**Criterion:** A developer can create a project from a Blueprint using
`forge new`.

**Test:**

```bash
forge new test-project --template python-fastapi --non-interactive
```

**Expected:**

- Exit code 0
- `test-project/` directory exists
- `test-project/forge.yaml` exists and is valid
- Expected files exist (`README.md`, `pyproject.toml`, `src/`,
  `tests/`, etc.)
- No files outside `test-project/` were created

#### 7.1.2 MVP-002: Deterministic Generation

**Criterion:** The same inputs produce the same output on every run.

**Test:**

```bash
forge new test-a --template python-fastapi --non-interactive
forge new test-b --template python-fastapi --non-interactive
diff -r test-a test-b
```

**Expected:**

- `diff -r` reports no differences
- (Except for `forge.yaml` which contains timestamps; these are
  normalised for the test)

#### 7.1.3 MVP-003: Generation Cannot Escape Target Directory

**Criterion:** Generation never writes outside the target directory.

**Test:**

```bash
# Run generation in a temporary directory
mkdir /tmp/test-target
cd /tmp/test-target
forge new test-project --template python-fastapi --non-interactive
# Verify no writes to /tmp/test-target/../ (parent)
```

**Expected:**

- Only `test-project/` is created
- No files in `/tmp/`

Also test with a hostile template that attempts path traversal:

```bash
# Use a template with ../ in a target path
forge new test --template hostile-traversal --non-interactive
```

**Expected:**

- Exit code 4 (security violation)
- No files created outside the target directory

#### 7.1.4 MVP-004: Generated Project Contains Valid Forge Metadata

**Criterion:** Every generated project contains a valid `forge.yaml`.

**Test:**

```bash
forge new test-project --template python-fastapi --non-interactive
forge validate test-project/forge.yaml
```

**Expected:**

- Exit code 0
- `forge.yaml` conforms to the schema
- All required fields are present
- Referenced template and blueprint exist

#### 7.1.5 MVP-005: Validation Detects Foundation Violations

**Criterion:** `forge validate` detects and reports when the repository
no longer satisfies its foundation.

**Test:**

```bash
forge new test-project --template python-fastapi --non-interactive
cd test-project
rm README.md
forge validate
```

**Expected:**

- Exit code 1
- Output contains a finding for missing `README.md`
- Finding includes rule ID, severity, location, and remediation

#### 7.1.6 MVP-006: `forge init` Adopts an Existing Repository

**Criterion:** A developer can adopt an existing repository.

**Test:**

```bash
mkdir existing-project
cd existing-project
echo "def hello(): pass" > main.py
git init
forge init --foundation python-api --non-interactive
```

**Expected:**

- Exit code 0
- `forge.yaml` exists at repository root
- `main.py` was not modified
- `forge.yaml` correctly references `python-api`

#### 7.1.7 MVP-007: `forge explain` Shows Foundation Details

**Criterion:** A developer can understand the foundation.

**Test:**

```bash
forge new test-project --template python-fastapi --non-interactive
cd test-project
forge explain
```

**Expected:**

- Exit code 0
- Output includes Blueprint name and version
- Output includes Template name and version
- Output includes components (if any)
- Output includes policies (if any)

#### 7.1.8 MVP-008: `forge template list` Shows Available Templates

**Criterion:** A developer can discover available templates.

**Test:**

```bash
forge template list
```

**Expected:**

- Exit code 0
- Output lists all five bundled templates
- Each entry shows name, version, and description
- Ordering is deterministic

#### 7.1.9 MVP-009: Dry-Run Does Not Write Files

**Criterion:** `--dry-run` previews changes without writing.

**Test:**

```bash
mkdir /tmp/test-dry-run
cd /tmp/test-dry-run
forge new test-project --template python-fastapi --dry-run
ls -la
```

**Expected:**

- Exit code 0
- Output shows what would be created
- `/tmp/test-dry-run/` is empty (no `test-project/`)
- No files were created

#### 7.1.10 MVP-010: JSON Output is Valid and Versioned

**Criterion:** `--format json` produces valid, versioned JSON.

**Test:**

```bash
forge validate --format json | jq .
```

**Expected:**

- Exit code matches validation result
- Output is valid JSON
- Output includes `schemaVersion` and `forgeVersion`
- Findings array is populated

#### 7.1.11 MVP-011: Exit Codes are Stable

**Criterion:** Exit codes follow the documented contract.

**Test:**

```bash
forge validate --file nonexistent.yaml; echo $?        # Should be 1
forge unknown-command; echo $?                         # Should be 2
```

**Expected:**

- Exit codes match the contract in
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md) § 7

#### 7.1.12 MVP-012: Errors are Actionable

**Criterion:** Every error includes a suggested action.

**Test:**

```bash
forge new test --template nonexistent --non-interactive
```

**Expected:**

- Exit code 2
- Error message includes the missing template name
- Error message includes a suggestion (e.g., `forge template list`)

### 7.2 Quality Criteria

#### 7.2.1 MVP-013: Cross-Platform Support

**Criterion:** The MVP runs on Linux, macOS, and Windows.

**Test:** Run the full test suite on all three platforms.

**Expected:** All tests pass.

#### 7.2.2 MVP-014: CLI Startup Under 100ms

**Criterion:** `forge version` and `forge --help` return in under
100ms.

**Test:**

```bash
time forge version
time forge --help
```

**Expected:** Both commands return in under 100ms on a modern
machine.

#### 7.2.3 MVP-015: Test Coverage

**Criterion:** The MVP has comprehensive test coverage.

**Test:** `go test -cover ./...`

**Expected:**

- Core packages: ≥ 80% coverage
- CLI commands: integration tests for each command

#### 7.2.4 MVP-016: Documentation

**Criterion:** The MVP is documented.

**Required documentation:**

- `README.md` with quickstart
- Installation guide
- Command reference
- Template authoring guide
- Security model documentation

---

## 8. MVP Quality Targets

| Target | Metric |
|--------|--------|
| CLI startup | < 100ms |
| Generation time | < 5s for typical projects |
| Cross-platform | Linux, macOS, Windows |
| Deterministic output | Byte-identical across runs |
| Security | No path traversal, no arbitrary execution |
| Coverage | ≥ 80% on core packages |
| Documentation | Every command documented |

---

## 9. MVP Non-Goals

Explicitly, the MVP does **not** attempt to:

### 9.1 Replace Existing Tools

The MVP does not replace Git, CI systems, package managers, IDEs, or
deployment platforms. It complements them.

### 9.2 Generate Application Code

The MVP generates foundation files (structure, configuration, tests,
documentation, CI). It does not generate business logic, domain
models, or API implementations.

### 9.3 Guarantee Security

The MVP implements security **controls** (path boundary, no code
execution, secret exclusion). It does not guarantee that generated
projects are secure.

### 9.4 Support Every Language

The MVP supports four languages (Python, Go, TypeScript, Rust). More
languages are added in later phases.

### 9.5 Scale to Large Organisations

The MVP targets individual developers and small teams. Organisation
features are later phases.

### 9.6 Provide a Hosted Service

The MVP is a local CLI tool. There is no hosted service, no user
accounts, no cloud sync.

### 9.7 Be Free of Bugs

The MVP is early software. It is expected to have bugs. The
development process is designed to find and fix them quickly.

### 9.8 Be Complete

The MVP is a starting point, not a finished product. It is
intentionally incomplete in ways documented in § 5.

---

## 10. Definition of Done

The MVP is considered done when:

### 10.1 Command Completeness

- [ ] All MVP commands are implemented
- [ ] All commands pass their acceptance criteria (§ 7.1)
- [ ] All commands have working `--help`
- [ ] All commands have consistent exit codes
- [ ] All commands support `--format json` where applicable

### 10.2 Security

- [ ] Filesystem boundary is enforced
- [ ] Path traversal is blocked
- [ ] No arbitrary code execution from templates
- [ ] Secrets are not read, logged, or generated
- [ ] Security tests pass

### 10.3 Quality

- [ ] Determinism tests pass
- [ ] Cross-platform tests pass
- [ ] Startup time < 100ms
- [ ] Test coverage ≥ 80%

### 10.4 Documentation

- [ ] README is complete
- [ ] Installation guide is complete
- [ ] Command reference is complete
- [ ] Template authoring guide is complete
- [ ] Security model is documented

### 10.5 Distribution

- [ ] Binary is available for Linux, macOS, Windows
- [ ] Installation instructions are tested
- [ ] Version information is correct

### 10.6 User Validation

- [ ] At least 10 developers have installed Forge
- [ ] At least 5 developers have successfully generated a project
- [ ] Feedback is collected and analysed
- [ ] Major usability issues are addressed

### 10.7 Sign-Off

- [ ] Phase 5 exit report is written
- [ ] Go/No-Go decision is recorded
- [ ] Phase 6 planning begins

---

## 11. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Templates are too generic | Medium | High | Ship 5 excellent templates, not 100 mediocre |
| Developers don't understand the foundation concept | Medium | High | Clear documentation; `forge explain` |
| MVP lacks the capabilities developers want | Medium | Medium | Validate with real users; iterate |
| Security flaw in MVP | Low | Critical | Comprehensive security testing; external review |
| Slow adoption | Medium | Medium | Focus on developer experience; build in public |
| Scope creep into post-MVP features | High | High | This document is binding |
| Template quality is inconsistent | Medium | Medium | Ship fewer, better templates |

---

## 12. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Product Discovery](./product-discovery.md) | Defines the problem the MVP addresses |
| [CLI UX Spec](./cli-ux-spec.md) | Full command specification for MVP commands |
| [Blueprint Spec](./blueprint-spec.md) | Blueprint schema used by MVP |
| [forge.yaml Spec](./forge-yaml-spec.md) | Configuration format used by MVP |
| [Template Spec](./template-spec.md) | Templates bundled with MVP |
| [Security Model](./security-model.md) | Security controls enforced by MVP |
| [Validation Spec](./validation-spec.md) | Validation rules used by MVP |
| [Architecture](./architecture.md) | Module structure for MVP implementation |

---

## 13. Open Questions

The following questions remain open and should be resolved before
Phase 5:

- Should `forge version` be included in the MVP or added later?
  (Current decision: included.)
- Should `forge template list` show templates from local directories
  as well as bundled templates?
- Should the MVP support `.forgeignore` for excluding files from
  generation?
- Should the MVP support generating a `.gitignore` that excludes
  Forge's own state (`.forge/`)?
- Should the MVP include a `forge doctor` command to diagnose the
  environment?
- Should the MVP support generating projects for Windows-specific
  paths, or is the current path handling sufficient?
- Should the MVP include a `forge completion` command for shell
  completion?
- Should the MVP support coloured output, or should it be
  monochrome-only for maximum compatibility?
- Should the MVP bundle a `forge.yaml.example` file with each
  template?
- Should the MVP generate a `.editorconfig` file by default?
- What is the minimum acceptance rate for the alpha test (5 of 10
  developers keep the project)?

These questions will be addressed during Phase 2–5 as implementation
progresses.

---

## 14. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The command set is confirmed
- The exclusions are confirmed
- The user journey is validated
- The acceptance criteria are tested
- The definition of done is agreed
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md) and
  [`docs/product-discovery.md`](./product-discovery.md) § 5.4
