# Forge

**Keep your engineering foundation intact.**

Forge is a cross-platform CLI for defining, generating, validating,
and evolving software project foundations as code.

Your repository changes every day. Your engineering standards
shouldn't disappear with it.

---

## Table of Contents

- [What is an Engineering Foundation?](#what-is-an-engineering-foundation)
- [The Forge Loop](#the-forge-loop)
- [Why Forge?](#why-forge)
- [Status](#status)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Example](#example)
- [Commands](#commands)
- [Project Structure](#project-structure)
- [Development](#development)
- [Contributing](#contributing)
- [Documentation](#documentation)
- [License](#license)

---

## What is an Engineering Foundation?

Every software project has an implicit foundation:

- the language and framework,
- the project structure,
- the testing setup,
- the CI configuration,
- the security controls,
- the documentation,
- the development commands.

These decisions are made once, at the start of a project, and then
slowly forgotten. Six months later, the repository still works, but
the foundation is no longer what the team intended.

**Forge makes the foundation explicit.** It defines the foundation as
a versioned, declarative artifact, and provides the tools to create,
verify, understand, and evolve it over the life of the project.

---

## The Forge Loop

```text
CREATE  →  VERIFY  →  EXPLAIN  →  EVOLVE
```

Four verbs. One loop.

| Verb        | Command                | Purpose                                        |
|-------------|------------------------|------------------------------------------------|
| **CREATE**  | `forge new`            | Establish the foundation.                      |
| **VERIFY**  | `forge check`          | Confirm the repository still satisfies it.     |
| **EXPLAIN** | `forge explain`        | Understand what the foundation is and why.     |
| **EVOLVE**  | `forge update`         | Apply foundation updates safely.               |

The loop is designed to run throughout the life of the repository,
not just once at the start.

---

## Why Forge?

Project scaffolding tools are common: Cookiecutter, `create-react-app`,
`cargo new`, `dotnet new`, and many internal templates. They all solve
the same problem: how to start a new project quickly.

Forge addresses a different problem: **how to keep the project's
engineering foundation intact as the project evolves**.

The distinction matters:

| Traditional scaffolding | Forge                                       |
|-------------------------|---------------------------------------------|
| One-time generation     | Continuous lifecycle                        |
| A collection of files   | A versioned, declarative foundation         |
| Template-based          | Blueprint + Components + Policies           |
| Forgotten after day one | Verified, explained, and evolved            |
| Individual developer    | Developer → Team → Organisation             |

Forge begins as a scaffolding tool and matures into a project-foundation
manager. See [Status](#status) for what is currently implemented.

---

## Status

Forge is in **early development**.

| Phase | Description                            | Status         |
|-------|----------------------------------------|----------------|
| 1     | Problem & Design Validation            | Complete       |
| 2     | Forge Core Foundation                  | In progress    |
| 3     | Blueprint Engine                       | Planned        |
| 4     | Template Engine                        | Planned        |
| 5     | Presentable MVP                        | Planned        |
| 6–21  | Developer experience, validation, drift detection, components, updates, adoption, registry, teams, organisations, enterprise governance, foundation platform | Planned |

The current milestone is Phase 2: a reliable, cross-platform,
testable technical skeleton that later phases can build on. No
user-facing commands exist yet.

See [`docs/phase-2-requirements.md`](docs/phase-2-requirements.md)
for the authoritative requirement register.

---

## Installation

Installation instructions will be published when Forge reaches
Phase 5 (Presentable MVP).

For contributors who want to build Forge from source, see
[Development](#development).

---

## Quick Start

The following workflow is the target for Phase 5. It is not yet
available.

```bash
# Create a project from a foundation.
forge new payments-api --template python-fastapi

# Move into the new project.
cd payments-api

# Verify the foundation.
forge check

# Understand what the foundation contains.
forge explain

# See what has drifted.
forge diff
```

Each command is designed to be understandable on its own, and to
compose into a coherent workflow.

---

## Example

A Forge project is described by a **Blueprint** — a declarative,
versioned specification of the engineering foundation. A typical
Blueprint looks like:

```yaml
forge:
  blueprint: python-api
  version: 2

project:
  name: payments-api
  type: service

language:
  name: python
  version: "3.13"

framework:
  name: fastapi

testing:
  required: true

container:
  required: true

ci:
  required: true

security:
  baseline: standard
```

From this Blueprint, Forge composes the appropriate templates,
components, and policies into a coherent repository foundation.

The repository records what it was built from:

```yaml
# forge.yaml (generated in the project root)
forge:
  blueprint: python-api
  version: 2

components:
  python: "3.13"
  fastapi: "0.115"
  postgres: "16"
  pytest: "8"
  docker: "27"
  github-actions: "4"
```

This metadata is what allows Forge to verify, explain, and evolve
the foundation over time.

---

## Commands

The target command surface for the Presentable MVP (Phase 5) is:

| Command                | Purpose                                              |
|------------------------|------------------------------------------------------|
| `forge new <name>`     | Create a new project from a foundation.              |
| `forge init`           | Adopt an existing repository into the Forge model.   |
| `forge validate`       | Validate configuration and repository structure.     |
| `forge explain`        | Explain the current project's foundation.            |
| `forge template list`  | List available templates.                            |
| `forge version`        | Display version information.                         |

Commands planned for later phases include `forge check`,
`forge diff`, `forge update`, `forge add`, `forge remove`,
`forge doctor`, and the registry commands
(`forge template inspect|search|install|publish`).

See [`docs/cli-ux-spec.md`](docs/cli-ux-spec.md) for the full CLI
specification, including syntax, flags, exit codes, and output
formats.

---

## Project Structure

```text
forge/
├── cmd/
│   └── forge/              # Process entry point
├── internal/
│   ├── cli/                # CLI command tree and handlers
│   ├── config/             # CLI configuration loading
│   ├── version/            # Build metadata
│   ├── blueprint/          # Project specification (Phase 3)
│   ├── template/           # Template engine (Phase 4)
│   ├── renderer/           # File renderer (Phase 4)
│   ├── filesystem/         # Safe filesystem abstraction (Phase 2)
│   └── forgeerr/           # Structured error model (Phase 10)
├── templates/              # Bundled templates (Phase 4+)
├── examples/               # Example blueprints and repositories
├── docs/                   # Documentation and specifications
├── scripts/                # Build, verification, and tooling scripts
├── .githooks/              # Versioned Git hooks
├── .github/                # GitHub configuration (CI, templates)
├── Taskfile.yml            # Task runner definitions
├── go.mod                  # Go module definition
├── LICENSE                 # Apache-2.0
└── README.md               # This file
```

The architecture follows Clean Architecture principles: dependencies
point inward, the domain layer is pure, and side effects are
centralised in the infrastructure layer. See
[`docs/architecture.md`](docs/architecture.md) for the full model.

---

## Development

### Prerequisites

- **Git** 2.30 or later.
- **Go** 1.23.0 or later. Go's automatic toolchain selection will
  install the pinned version (1.23.4) if you are on a newer release.
- **[Task](https://taskfile.dev/)** 3.x.
- **Node.js** 18 or later (only for Markdown linting).
- **`markdownlint-cli2`** (install with
  `npm install -g markdownlint-cli2`).

### Getting started

```bash
# Clone the repository.
git clone git@github.com:thapelomagqazana/forge.git
cd forge

# Install the Git hooks (run once per clone).
task hooks:install

# Confirm the module and version matrix are correctly configured.
task verify

# Run the full quality gate.
task check

# Build the binary.
task build
```

The binary is produced at `./forge`. To run it:

```bash
./forge version
./forge --help
```

### Task runner

Every routine operation is exposed as a `task`. Run `task --list`
to see all available tasks. The most important ones:

| Task                     | Purpose                                                    |
|--------------------------|------------------------------------------------------------|
| `task check`             | Run the full quality gate.                                 |
| `task verify`            | Run every verification check.                              |
| `task test`              | Run unit tests with the race detector.                     |
| `task test:integration`  | Run process-boundary integration tests.                    |
| `task build`             | Build the binary with injected metadata.                   |
| `task fmt`               | Format all Go source files.                                |
| `task lint`              | Run all linters (currently Markdown).                      |
| `task tidy`              | Run `go mod tidy` (maintainers only; clean branch).        |
| `task hooks:install`     | Install the local Git hooks.                               |

### Environment

Forge requires two environment variables for reproducible builds:

```bash
export GOFLAGS=-mod=readonly
export GOTOOLCHAIN=auto
```

Add these to your shell profile. See
[`docs/development.md`](docs/development.md) for the full setup
guide, including Windows instructions and troubleshooting.

---

## Contributing

Forge is in early development. Contribution guidelines are being
finalised and will be published before the first public release.

In the meantime:

- **Bug reports and feature requests** are welcome via
  [GitHub Issues](https://github.com/thapelomagqazana/forge/issues).
  Use the structured templates.
- **Questions** belong in
  [GitHub Discussions](https://github.com/thapelomagqazana/forge/discussions).
- **Security vulnerabilities** must not be reported publicly. See
  `SECURITY.md` for the private reporting process.
- **Pull requests** should follow the template in
  [`.github/pull_request_template.md`](.github/pull_request_template.md).

Before opening a pull request, read
[`docs/development.md`](docs/development.md) and run
`task check`. The CI will run the same checks.

### Commit conventions

Forge follows the [Conventional Commits](https://www.conventionalcommits.org/)
specification. Commit messages are validated by the `commit-msg` Git
hook. The format is:

```text
<type>(<scope>): <subject>
```

Where `<type>` is one of `feat`, `fix`, `docs`, `style`, `refactor`,
`perf`, `test`, `build`, `ci`, `chore`, or `revert`.

### Dependency policy

Every third-party dependency is governed by
[`docs/dependency-policy.md`](docs/dependency-policy.md). Adding a
new dependency requires an ADR and a registry entry. The policy is
enforced by the `verify:deps` task in CI.

---

## Documentation

Forge's documentation lives in [`docs/`](docs/). The documents are
organised as follows.

### Specifications

- [`docs/product-discovery.md`](docs/product-discovery.md) — the
  product problem and user research.
- [`docs/cli-ux-spec.md`](docs/cli-ux-spec.md) — the CLI's command
  surface, flags, output, and exit codes.
- [`docs/blueprint-spec.md`](docs/blueprint-spec.md) — the Blueprint
  schema.
- [`docs/forge-yaml-spec.md`](docs/forge-yaml-spec.md) — the
  `forge.yaml` format.
- [`docs/template-spec.md`](docs/template-spec.md) — the template
  format.
- [`docs/component-spec.md`](docs/component-spec.md) — the component
  format.
- [`docs/validation-spec.md`](docs/validation-spec.md) — validation
  rules and results.
- [`docs/security-model.md`](docs/security-model.md) — the security
  model.
- [`docs/update-model.md`](docs/update-model.md) — the safe-update
  model.

### Engineering

- [`docs/architecture.md`](docs/architecture.md) — the technical
  architecture, module structure, and execution model.
- [`docs/dependency-policy.md`](docs/dependency-policy.md) — the
  dependency addition, versioning, and upgrade policy.
- [`docs/development.md`](docs/development.md) — the development
  guide.
- [`docs/phase-2-requirements.md`](docs/phase-2-requirements.md) —
  the Phase 2 requirement register.

### Decisions

- [`docs/decisions/`](docs/decisions/) — Architecture Decision
  Records (ADRs). Each ADR records a decision, the alternatives
  considered, and the consequences.

### Research

- [`docs/research/`](docs/research/) — the market validation
  research, interview protocols, findings, and evidence.

---

## License

Forge is licensed under the [Apache License 2.0](LICENSE).

By contributing to Forge, you agree that your contributions will be
licensed under the same license. See the
[`LICENSE`](LICENSE) file for details.

---

## Acknowledgements

Forge's design has been informed by the work of many projects and
individuals in the developer-tooling community. The project's
positioning, architecture, and roadmap draw on the lessons learned
from Cookiecutter, Copier, Backstage, and the many language-native
project generators that came before it.

The specific decisions Forge makes are documented in the ADRs under
[`docs/decisions/`](docs/decisions/).
