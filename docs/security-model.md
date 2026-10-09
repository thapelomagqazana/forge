# Safety & Security Model

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

This specification defines what Forge must **never** do accidentally,
and how it protects the developer's filesystem, secrets, and trust.

Forge writes to the developer's filesystem. This is a powerful and
dangerous capability. Unlike a code linter or a static analyzer,
Forge can:

- Create files
- Modify files
- Delete files
- Read configuration
- Read environment files

This specification exists to make every one of those operations safe
by default and to define the boundaries Forge will not cross.

It answers:

- What are the threats Forge must defend against?
- What is the filesystem boundary model?
- When may Forge overwrite files?
- May templates execute commands?
- How does Forge handle secrets?
- What supply-chain protections are required?
- What are the acceptance criteria for security?

---

## 2. Scope

**In scope:**

- Threat model
- Filesystem boundary model
- Overwrite policy
- Hook policy
- Secret-handling model
- Supply-chain model
- Security acceptance criteria
- Security test requirements

**Out of scope:**

- Validation rules (see
  [`docs/validation-spec.md`](./validation-spec.md))
- Update algorithm (see
  [`docs/update-model.md`](./update-model.md))
- Registry implementation (see
  [`docs/registry-spec.md`](./registry-spec.md), future)
- CLI command surface (see
  [`docs/cli-ux-spec.md`](./cli-ux-spec.md))

---

## 3. Security Philosophy

Forge's security model rests on five principles.

### 3.1 Principle of Least Surprise

Forge never does anything the developer did not ask for. Every
mutation is either:

- Explicitly requested by the developer
- Shown to the developer before it happens
- Reversible (backup and rollback)

### 3.2 Principle of Explicit Consent

Forge requires explicit consent for operations that:

- Overwrite existing files
- Delete files
- Execute external commands
- Access credentials

The default for all such operations is **no**.

### 3.3 Principle of Sandboxed Rendering

Templates render files. Templates do not execute code. This
eliminates the largest class of supply-chain attacks.

### 3.4 Principle of Evidence-Based Trust

Forge trusts artifacts (templates, components, foundations) only to
the extent that trust is verified:

- Checksums verify integrity
- Signatures verify origin (future)
- Provenance verifies build process (future)

Unsigned artifacts are still usable, but their trust level is
explicitly marked.

### 3.5 Principle of Fail-Safe Defaults

When Forge cannot determine the safe course of action, it fails
rather than guessing. Examples:

- Ambiguous foundation selection → error
- Malformed `forge.yaml` → refuse to modify
- Conflicting updates → require manual resolution

---

## 4. Threat Model

The following threats are considered in scope for Phase 1. Each is
described with its impact, likelihood, and mitigation.

### 4.1 Path Traversal

**Description:** A template or component attempts to write a file
outside the target directory using relative paths (`../`) or absolute
paths (`/etc/passwd`, `C:\Windows\System32\`).

**Impact:** High. An attacker could overwrite arbitrary files on the
developer's machine.

**Likelihood:** Medium. Requires a malicious or buggy template.

**Mitigation:**

- All generated paths are resolved against the target directory
- Any path that resolves outside the target directory is rejected
- Absolute paths are rejected
- Symlinks that would escape the target are rejected

**Acceptance criteria:** `SEC-001`.

### 4.2 Arbitrary Command Execution

**Description:** A template or component includes a hook that
executes a shell command, a script, or a package installation.

**Impact:** Critical. An attacker gains code execution on the
developer's machine.

**Likelihood:** High if hooks are supported; eliminated if they are
not.

**Mitigation:**

- Hooks are **not supported** in schema version 1
- Templates cannot execute commands, scripts, or package managers
- If hooks are introduced in the future, they require explicit
  opt-in and sandboxing

**Acceptance criteria:** `SEC-002`.

### 4.3 Malicious Templates

**Description:** A developer installs a template from an untrusted
source, and the template is designed to cause harm (delete files,
leak secrets, install malware).

**Impact:** High. Depends on the template's capabilities.

**Likelihood:** Medium. Increases if a public template registry
exists without moderation.

**Mitigation:**

- Sandboxed rendering (§ 3.3) limits what templates can do
- No command execution from templates
- No filesystem access outside the target directory
- Future: signature verification, trusted publishers
- Future: registry moderation

### 4.4 Unexpected Overwrites

**Description:** A template or component silently overwrites a file
the developer has modified.

**Impact:** Medium to High. Loses developer work.

**Likelihood:** High if not carefully controlled.

**Mitigation:**

- Overwrite policy (§ 6) classifies every write
- Developer-modified files require explicit confirmation
- `--dry-run` is mandatory for mutating commands
- Backups are created before any mutation

**Acceptance criteria:** `SEC-003`.

### 4.5 Symlink Attacks

**Description:** A template or component creates a symlink that
points outside the target directory, then writes through it.

**Impact:** High. Bypasses the path boundary.

**Likelihood:** Low. Requires a sophisticated attacker.

**Mitigation:**

- Symlinks are resolved to their canonical paths before validation
- Symlinks that would escape the target are rejected
- Symlink targets are never followed during writes

### 4.6 Secret Leakage

**Description:** Forge reads or logs a secret (API key, token,
password, private key) and either stores it in a file or exposes it
in output.

**Impact:** High. Compromises the developer's credentials.

**Likelihood:** Medium. Requires Forge to accidentally read a
sensitive file.

**Mitigation:**

- Forge does not read `.env` files
- Forge does not read credential stores
- Forge does not log secrets in error messages
- Forge's generated `.gitignore` excludes secret files
- Secret-scanning rules are part of the security baseline

**Acceptance criteria:** Part of § 8.

### 4.7 Untrusted Dependencies

**Description:** A template or component installs a package or
imports a dependency from an untrusted source.

**Impact:** High. Compromises the generated project.

**Likelihood:** Low. Templates do not install dependencies in
schema version 1.

**Mitigation:**

- Templates do not install dependencies
- Components do not install dependencies
- Generated projects use standard package managers (the developer's
  choice)
- Future: dependency integrity checks (checksums, signatures)

### 4.8 Registry Compromise

**Description:** A registry that hosts templates or components is
compromised, and malicious artifacts are distributed to developers.

**Impact:** High.

**Likelihood:** Low. Registry does not exist in Phase 1.

**Mitigation:**

- Registry is deferred to Phase 16
- Phase 16 will require signature verification
- Phase 16 will require provenance attestation
- Immutable artifact versions (no re-publishing)

### 4.9 Additional Threats Considered

| Threat | Impact | Mitigation |
|--------|--------|------------|
| Malformed YAML | Low | Schema validation before use |
| Malformed templates | Medium | Template validation |
| Denial of service (huge files) | Medium | Size limits on templates and packages |
| Zip-slip (archive extraction) | High | Path validation during extraction |
| Time-of-check/time-of-use (TOCTOU) | Medium | Atomic operations where practical |
| Race conditions | Low | Serialized writes |
| Environment variable injection | Medium | Environment variables not read by templates |
| Dependency confusion | High | Future: registry namespace verification |

---

## 5. Filesystem Boundary Model

This section defines exactly what paths Forge may read from and write
to.

### 5.1 The Target Directory

Every Forge operation has a **target directory**. This is the root of
the repository being modified.

- For `forge new`, the target directory is the newly-created project
  directory (`./payments-api/` by default).
- For `forge init`, the target directory is the current repository
  root.
- For `forge update`, the target directory is the current repository
  root.
- For `forge add`, the target directory is the current repository
  root.

### 5.2 Allowed Paths

Forge may read from and write to paths that resolve inside the target
directory.

```text
Target: ~/projects/payments-api

Allowed:
  ~/projects/payments-api/README.md
  ~/projects/payments-api/src/main.py
  ~/projects/payments-api/.github/workflows/ci.yml
  ~/projects/payments-api/tests/test_health.py
```

### 5.3 Forbidden Paths

Forge **never** reads from or writes to paths that resolve outside
the target directory.

```text
Target: ~/projects/payments-api

Forbidden:
  /etc/passwd
  ~/.ssh/id_rsa
  ~/projects/other-project/README.md
  ../other-project
  C:\Windows\System32\drivers\etc\hosts
```

Attempting to write to a forbidden path causes Forge to fail with a
security error (`SEC-001`).

### 5.4 Absolute Paths

Absolute paths in template targets or component manifests are
**rejected**.

```yaml
# Invalid
files:
  - path: Dockerfile.tmpl
    target: /etc/nginx/nginx.conf
```

Error:

```text
✗ Security violation

Absolute path not permitted in target:
  /etc/nginx/nginx.conf

Targets must be relative to the repository root.
```

### 5.5 Parent Traversal

Paths containing `..` are **resolved** and then checked against the
target directory.

If the resolved path is inside the target directory, the path is
allowed.

```yaml
# Valid (resolves inside target)
files:
  - path: shared/LICENSE
    target: ../payments-api/LICENSE   # Invalid: escapes target
```

In practice, `..` in target paths almost always escapes the target
and is rejected. The boundary check is the authority, not the
presence of `..`.

### 5.6 Symlinks

Symlinks are a critical security concern. Forge handles them as
follows.

**During reads:**

- Symlinks inside the target directory are followed for reads
- If the resolved path is outside the target directory, the read is
  refused

**During writes:**

- Symlink targets are resolved before any write
- If the resolved path is outside the target directory, the write is
  refused
- Forge does **not** write through symlinks

**During generation:**

- Templates may not create symlinks in schema version 1
- A future schema version may allow symlink creation with explicit
  opt-in

### 5.7 Existing Files

Forge never overwrites an existing file without following the
overwrite policy (§ 6).

Before any write, Forge checks:

1. Does the target file exist?
2. Is it tracked in `.forge/state.yaml`?
3. Has the developer modified it since Forge last wrote it?

The answers to these questions determine whether the write proceeds,
prompts, or fails.

### 5.8 Case Sensitivity

On case-insensitive filesystems (macOS, Windows), Forge must:

- Normalize paths to their canonical case
- Treat `README.md` and `readme.md` as the same file
- Refuse to write two files that differ only in case

On case-sensitive filesystems (Linux), Forge treats paths as
distinct.

### 5.9 Reserved Directories

The following directories are **never** written to by Forge except as
explicitly requested:

| Directory | Reason |
|-----------|--------|
| `.git/` | Managed by Git; Forge never modifies internals |
| `.forge/backups/` | Reserved for Forge backups |
| `node_modules/` | Managed by package managers |
| `vendor/` | Managed by package managers |
| `.venv/`, `venv/` | Managed by Python tooling |

### 5.10 Boundary Verification

Every path is verified by:

1. Converting the path to a canonical absolute path
2. Checking that the canonical path is inside the canonical target
   directory
3. Rejecting the path if the check fails

This is a mandatory step in every filesystem write.

---

## 6. Overwrite Policy

This section defines when Forge may write to a file.

### 6.1 Operation Classifications

Every filesystem operation is classified:

| Classification | Meaning |
|----------------|---------|
| **Create** | The file does not exist; Forge creates it |
| **Modify** | The file exists and Forge changes it |
| **Replace** | The file exists and Forge overwrites it entirely |
| **Delete** | The file exists and Forge removes it |

### 6.2 Create

**Behaviour:** Proceed without confirmation.

**Rationale:** Creating a new file cannot destroy anything.

**Exceptions:** A file that is about to be created is checked against
the reserved directory list (§ 5.9). Creating a file in a reserved
directory is refused.

### 6.3 Modify

**Behaviour:** Depends on file ownership and modification history.

**Ownership check:**

1. Is the file recorded in `.forge/state.yaml` as Forge-owned?
2. Has the file been modified since Forge last wrote it?
3. Is the modification compatible with the pending change?

**Cases:**

| Case | Behaviour |
|------|-----------|
| File is Forge-owned and unmodified | Proceed (structured merge) |
| File is Forge-owned but developer-modified | Prompt for confirmation |
| File is developer-owned | Refuse unless explicit opt-in |
| File is not tracked | Treat as developer-owned |

### 6.4 Replace

**Behaviour:** Requires explicit confirmation.

**Rationale:** Replace discards all content. If the file has been
modified by the developer, replace destroys their work.

**Confirmations:**

- Interactive: prompt
- Non-interactive: require `--yes` flag
- Dry-run: show what would be replaced

### 6.5 Delete

**Behaviour:** Requires explicit confirmation.

**Rationale:** Delete removes a file from the repository. Even if
backed up, the change affects the developer's workflow.

**Confirmations:**

- Interactive: prompt
- Non-interactive: require `--yes` flag
- Dry-run: show what would be deleted

### 6.6 Confirmation UX

Interactive confirmation follows a consistent pattern:

```text
⚠ Modify existing file

File: Dockerfile
Owner: component:docker@2.0.0
Modified by you: no

Changes:
  + RUN go build -o /app
  - RUN go build

[o] Overwrite
[d] Show diff
[s] Skip
[a] Abort

Choice [o/d/s/a]:
```

In non-interactive mode, Forge fails with a clear error unless `--yes`
or `--force` is provided.

### 6.7 Backups

Before any Modify, Replace, or Delete operation, Forge creates a
backup:

```text
.forge/
└── backups/
    └── 2026-10-09T12-00-00Z/
        ├── Dockerfile
        ├── pyproject.toml
        └── ...
```

Backups are:

- Timestamped
- Preserved until the next successful operation
- Restorable via `forge update --rollback` (Phase 14)

### 6.8 The Never-Overwrite List

The following files are **never** overwritten without explicit
developer confirmation, regardless of ownership:

| File | Reason |
|------|--------|
| `.git/config` | Repository configuration |
| `.gitignore` | Developer-maintained ignore rules |
| `LICENSE` | Legal file |
| `README.md` | Often heavily customized |
| `CONTRIBUTING.md` | Usually project-specific |
| `SECURITY.md` | Often includes contact info |
| `.env` | May contain secrets |

These files may be created if absent, but Forge never replaces their
content without explicit consent.

---

## 7. Hook Policy

This section defines whether templates and components may execute
code.

### 7.1 Default: No Execution

**Forge does not execute code from templates or components in schema
version 1.**

This is the single most important security decision in Forge.

### 7.2 What Is Prohibited

Templates and components may **not**:

- Execute shell commands
- Execute scripts (Bash, PowerShell, Python, etc.)
- Install packages (`npm install`, `pip install`, `go get`, etc.)
- Make network requests
- Read environment variables
- Read files outside the target directory
- Write files outside the target directory

### 7.3 Why This Policy

**Attack surface reduction.** Any hook mechanism is a code-execution
primitive. Even a well-designed hook system has:

- Sandbox escape risks
- Privilege escalation risks
- Supply-chain risks (a trusted template's hook is compromised)
- Audit complexity

By eliminating hooks entirely, Forge eliminates this class of
vulnerabilities.

### 7.4 What Replaces Hooks

Functionality that would otherwise require hooks is achieved via:

| Need | Solution |
|------|----------|
| Custom file content | Template rendering |
| Conditional files | Conditions (§ Template Spec) |
| Multi-file composition | Components (Phase 12) |
| Post-generation setup | Developer runs commands manually |
| Package installation | Developer runs their package manager |

Forge produces a repository. The developer runs the setup.

### 7.5 Future: Opt-In Hooks

A future schema version may introduce hooks with explicit opt-in. If
introduced, hooks must satisfy:

- The developer explicitly enables hooks per template
- Hooks run in a sandbox (container, restricted filesystem)
- Hooks are declared in the manifest with their full command
- Hooks are shown to the developer before execution
- Hooks are logged
- Hooks fail safely (rollback on error)

This is a future decision, not part of Phase 1.

### 7.6 Detecting Hook Attempts

If a template or component manifest contains a `hooks` block, Forge
fails template validation:

```text
✗ Template validation failed

Template: python-fastapi@1.0.0

Reason: 'hooks' is not supported in schema version 1.

Templates render files. They do not execute code.
See docs/security-model.md § 7 for the hook policy.
```

### 7.7 No Implicit Execution

Forge does **not** execute anything as a side effect of:

- Reading a template
- Rendering a template
- Validating a template
- Installing a component

The only commands Forge executes are those it owns (e.g., `git init`
if explicitly requested).

---

## 8. Secret-Handling Model

This section defines how Forge handles secrets.

### 8.1 What Counts as a Secret

Forge treats the following as secrets:

- Environment files (`.env`, `.env.local`, `.env.production`)
- Credentials files (`.aws/credentials`, `.docker/config.json`)
- API keys and tokens
- Private keys (`*.pem`, `*.key`, `id_rsa`, `id_ed25519`)
- Passwords
- Session tokens
- Database connection strings containing passwords

### 8.2 Forge Does Not Read Secrets

Forge **does not read** secret files, with one exception (§ 8.4).

Specifically, Forge does not:

- Read `.env` files
- Read `.aws/credentials` or similar
- Read private keys
- Read database connection strings
- Parse `forge.yaml` for secret values (secrets must not be in
  `forge.yaml`)

### 8.3 Forge Does Not Log Secrets

Forge's output never includes secret values. This includes:

- Error messages
- Debug output (`--verbose`)
- JSON output
- Log files
- Validation findings

If a secret value would appear in output, Forge replaces it with:

```text
<redacted>
```

Example:

```text
✗ Could not connect to database

Connection string: postgres://user:<redacted>@host/db
```

### 8.4 The `.gitignore` Exception

Forge reads `.gitignore` to verify that secret files are excluded.
This is the only file with secret-relevant content that Forge reads.

Forge checks `.gitignore` for the presence of patterns like:

- `.env`
- `*.pem`
- `*.key`
- `credentials`

Forge does **not** parse `.gitignore` for secret values (there are
none).

### 8.5 Forge Does Not Generate Secrets

Forge never generates API keys, tokens, passwords, or private keys.

Generated projects include `.env.example` (with placeholders), never
`.env`.

Example `.env.example`:

```text
DATABASE_URL=
API_KEY=
LOG_LEVEL=INFO
```

### 8.6 Forge Does Not Commit Secrets

Forge does not:

- Commit files to Git (except when explicitly asked via `forge new`)
- Stage files
- Modify Git configuration

When `forge new` initialises a Git repository, it does not commit
unless the developer explicitly runs `git commit`.

### 8.7 Secret-Scanning Rules

The security baseline (§ Validation Spec § 4.9) includes rules that
detect obvious secret patterns:

| Rule | Detects |
|------|---------|
| `VAL-SEC-001` | `.env` is listed in `.gitignore` |
| `VAL-SEC-002` | `.env.example` exists |
| `VAL-SEC-003` | No `.env` file is committed |
| `VAL-SEC-004` | CI workflow declares minimum permissions |

Additional rules may be added in future phases.

### 8.8 What Forge Never Claims

Forge does **not** claim:

- "This repository contains no secrets"
- "This repository is secure"
- "This repository is compliant with [standard]"

Forge reports the **presence or absence of controls**. It does not
certify outcomes.

---

## 9. Supply-Chain Model

This section defines the supply-chain protections required for
templates and components, both now and in future phases.

### 9.1 Phase 1 (Current)

In Phase 1, templates and components are:

- Bundled with Forge (built-in templates)
- Provided by the developer (local templates)
- Not distributed via a registry

There is no supply-chain risk in Phase 1 because there is no
distribution.

### 9.2 Phase 16 (Registry)

The registry (Phase 16) will introduce distribution. The following
protections will be required.

#### 9.2.1 Checksums

Every published artifact is accompanied by a cryptographic checksum:

```text
algorithm: sha256
digest: abc123def456...
```

Checksums verify integrity: the artifact received matches the
artifact published.

#### 9.2.2 Signatures

Every published artifact is signed by its publisher:

```text
signature: <signature bytes>
publisher: acme
key_id: <public key identifier>
```

Signatures verify origin: the artifact was published by the
claimed publisher.

#### 9.2.3 Trusted Publishers

The registry maintains a list of trusted publishers. Trust is:

- Explicit (publishers are verified)
- Revocable (compromised publishers are delisted)
- Auditable (every trust change is logged)

#### 9.2.4 Version Pinning

Generated projects pin the exact version of every template and
component:

```yaml
forge:
  blueprint: python-api
  blueprint_version: 2.4.0
  template: python-fastapi
  template_version: 1.0.0
```

Pinning prevents unintended upgrades.

#### 9.2.5 Provenance

Every published artifact includes provenance metadata:

```yaml
provenance:
  source:
    repository: github.com/acme/python-fastapi
    commit: abc123
  build:
    system: github-actions
    workflow: release.yml
    run_id: 12345
  published_at: "2026-10-09T12:00:00Z"
```

Provenance verifies that the artifact was built from the claimed
source.

### 9.3 Install-Time Verification

Before installing a template or component from the registry, Forge
verifies:

1. The checksum matches
2. The signature is valid
3. The publisher is trusted (or the developer accepts risk)
4. The version is available
5. The compatibility constraints are satisfied

If any check fails, installation is refused.

### 9.4 Offline Verification

Forge supports offline verification:

- Trusted publisher keys are cached
- Checksums are cached
- Artifacts are cached

Offline verification is required for air-gapped environments.

### 9.5 Deprecation and Revocation

If a publisher or artifact is revoked:

- New installations are refused
- Existing installations continue to work (but report a warning)
- The developer may choose to uninstall

### 9.6 Future Enhancements

The following are candidates for later phases:

- Reproducible builds for artifacts
- Transparency logs (à la Certificate Transparency)
- Attestation (SLSA-style provenance)
- Differential verification (peer comparison of artifacts)

These are not required for Phase 1 or Phase 16.

---

## 10. Security Acceptance Criteria

The following acceptance criteria define when Forge's security is
sufficient for release.

### 10.1 SEC-001: No Generated Path Escapes Target Directory

**Requirement:** Forge must never write to or read from a path that
resolves outside the target directory.

**Verification:**

- Unit tests for path resolution
- Tests for path traversal (`../`, `..%2F`, encoded variants)
- Tests for absolute paths (Unix and Windows)
- Tests for symlink escapes
- Integration tests with hostile templates

**Failure mode:** Forge refuses the operation with exit code 4.

### 10.2 SEC-002: No Template Executes Arbitrary Commands

**Requirement:** Templates and components may not execute shell
commands, scripts, or package installations without explicit
authorization.

**Verification:**

- Schema rejects `hooks` blocks
- Tests verify that no shell commands are executed during rendering
- Tests verify that no environment variables are read
- Tests verify that no network requests are made

**Failure mode:** Template validation fails.

### 10.3 SEC-003: Forge Never Silently Overwrites Developer Files

**Requirement:** Any write to a file that has been modified by the
developer requires explicit consent.

**Verification:**

- Tests for the overwrite policy (§ 6)
- Tests for each classification (Create, Modify, Replace, Delete)
- Tests for the never-overwrite list (§ 6.8)
- Tests verify that backups are created before mutation

**Failure mode:** Forge refuses the write and prompts for consent.

### 10.4 SEC-004: Secrets Are Not Read, Logged, or Generated

**Requirement:** Forge must not read secret files, log secret values,
or generate secrets.

**Verification:**

- Tests verify Forge does not read `.env` files
- Tests verify Forge does not log secret values
- Tests verify Forge does not generate `.env` files
- Tests verify generated `.gitignore` excludes secrets

**Failure mode:** N/A (this is a design invariant).

### 10.5 SEC-005: No Nondeterministic Behavior

**Requirement:** Validation and generation are deterministic: given
the same inputs, they produce the same outputs.

**Verification:**

- Tests run validation twice and compare findings
- Tests render templates twice and compare output byte-for-byte
- Tests run on multiple platforms and compare output

**Failure mode:** Tests fail; issue is a bug.

### 10.6 SEC-006: Fail-Safe on Corrupt State

**Requirement:** If Forge's state is corrupt (`.forge/state.yaml`
malformed, `forge.yaml` invalid), Forge fails safely and does not
modify the repository.

**Verification:**

- Tests for corrupt state files
- Tests verify no writes occur
- Tests verify error messages are actionable

**Failure mode:** Forge refuses the operation with exit code 1.

---

## 11. Security Testing Requirements

### 11.1 Test Categories

| Category | Coverage |
|----------|----------|
| **Path traversal** | Every path handling path |
| **Symlink escapes** | Symlink creation, resolution, writes |
| **Overwrite policy** | Every classification and case |
| **Hook rejection** | Every template validation |
| **Secret handling** | Reads, logs, generation |
| **Determinism** | Double-run equivalence |
| **Fail-safe** | Corrupt state, malformed inputs |
| **Cross-platform** | Linux, macOS, Windows |

### 11.2 Fuzz Testing

Forge uses Go's fuzz testing for:

- Path parsing
- YAML parsing
- Template rendering
- Blueprint validation

Fuzz tests run in CI as part of the standard test suite.

### 11.3 Security Regression Suite

Every discovered security issue becomes a permanent test. The suite
grows over time and cannot be disabled.

### 11.4 Third-Party Security Review

Before 1.0 release, Forge undergoes an external security review
covering:

- Filesystem boundary
- Template rendering
- Component composition
- Registry (when implemented)
- Secret handling

The review's findings are addressed before release.

---

## 12. Threat Summary

| Threat | Phase addressed | Mitigation | Acceptance criteria |
|--------|-----------------|------------|---------------------|
| Path traversal | Phase 1 | Boundary model | SEC-001 |
| Arbitrary execution | Phase 1 | No hooks | SEC-002 |
| Malicious templates | Phase 1 | Sandbox | SEC-001, SEC-002 |
| Unexpected overwrites | Phase 1 | Overwrite policy | SEC-003 |
| Symlink attacks | Phase 1 | Symlink resolution | SEC-001 |
| Secret leakage | Phase 1 | Never read/log/generate | SEC-004 |
| Untrusted dependencies | Phase 16 | No implicit install | — |
| Registry compromise | Phase 16 | Signatures, provenance | — |

---

## 13. Relationship to Other Specifications

| Specification | Relationship |
|---------------|--------------|
| [Blueprint](./blueprint-spec.md) | Blueprints declare intent; security governs execution |
| [forge.yaml](./forge-yaml-spec.md) | Corruption handling defined there |
| [Template](./template-spec.md) | Template sandbox defined here |
| [Component](./component-spec.md) | Component composition governed by ownership rules |
| [Validation](./validation-spec.md) | Security validation rules defined there |
| [Update Model](./update-model.md) | Update safety builds on overwrite policy |
| [CLI UX Spec](./cli-ux-spec.md) | Exit code 4 for security violations |
| [Architecture](./architecture.md) | Security module boundaries |

---

## 14. Open Questions

The following questions remain open and should be resolved before
implementation:

- Should Forge encrypt backups, or is filesystem-level protection
  sufficient?
- Should Forge verify that generated files do not contain obvious
  secrets before writing them?
- Should Forge detect when a template is being run for the first time
  from an untrusted source and prompt for consent?
- Should Forge support a "trust this template" mechanism to avoid
  re-prompting for the same template?
- Should Forge scan `.env.example` files for accidentally committed
  secrets?
- Should Forge validate that generated `.gitignore` files exclude
  secret patterns?
- Should Forge use a sandbox (e.g., `bubblewrap`, `firejail`) when
  running on Linux, even though no code is executed?
- What is the plan for TOCTOU attacks between path validation and
  file write?
- Should Forge refuse to operate on repositories with uncommitted Git
  changes, to prevent accidental loss of uncommitted work?
- Should Forge include a `.forge-ignore` file (like `.dockerignore`)
  to exclude paths from scanning?

These questions will be addressed in Phase 2 as implementation
begins.

---

## 15. Status

**Draft.**

This specification is under active development during Phase 1. It
becomes **Approved** when:

- The threat model is reviewed and complete
- The filesystem boundary model is tested
- The overwrite policy is tested
- The hook policy is confirmed
- The secret-handling model is reviewed
- The supply-chain model is confirmed for Phase 16
- All acceptance criteria are testable
- Open questions have been resolved or explicitly deferred
- The specification has been reviewed for consistency with
  [`docs/template-spec.md`](./template-spec.md) § 3.3 and
  [`docs/validation-spec.md`](./validation-spec.md) § 4.9
