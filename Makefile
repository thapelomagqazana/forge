# =============================================================================
# Forge — Makefile
# =============================================================================
#
# This Makefile has two target families.
#
# Development targets are a compatibility shim over Taskfile.yml. They
# exist so downstream consumers who do not have Task installed —
# packagers, minimal CI images, contributors on systems where Task is
# hard to install — can still run `make build`. Every development
# target delegates to `task`. No build logic lives here.
#
# Release targets delegate to scripts/build.go, a Go program that runs
# on Linux, macOS, and Windows without a POSIX shell. The Go program
# is the only place cross-compilation logic lives. It reads
# docs/reference/platforms.yaml, injects the four version variables
# via -ldflags -X, packages each binary, writes SHA256SUMS, produces
# a CycloneDX SBOM, and signs with cosign.
#
# This Makefile MUST NOT contain:
#
#   * shell loops (`for ... do ... done`)
#   * process substitution (`<(cmd)`)
#   * any construct that does not run under `make` on Windows with
#     MSYS2, Git Bash, or `nmake`
#   * `-X` linker flags (those live in Taskfile.yml and
#     scripts/build.go)
#   * a `MODULE` variable (the module path is read from go.mod)
#   * `git ` commands (the driver reads git)
#
# The absence of those tokens in this file is deliberate and is
# enforced by verify:injection:single-authority in Taskfile.yml. The
# check permits a `task` delegation and forbids the tokens above. If
# you are tempted to add cross-compile logic to a target, put it in
# scripts/build.go instead and call that.
#
# See WBS 6.1.2, WBS 17.2.1, docs/guides/building.md.
# =============================================================================

TASK ?= task
GO   ?= go

# The default target is `help`. Typing `make` alone prints the target
# list rather than silently building or failing.
.DEFAULT_GOAL := help

# Every target is phony: none produces a file whose name matches the
# target. Declaring them .PHONY prevents `make` from skipping a target
# because a directory of the same name exists (e.g. a `build/` or
# `test/` directory would otherwise mask the target).
.PHONY: help \
        build build-debug run \
        test test-integration check fmt fmt-check vet lint clean \
        build-all build-linux build-darwin build-windows \
        checksums sign sbom release-dry-run release \
        toolchain verify-cgo verify-mod verify-tagged \
        verify:injection verify:injection-prefix

# =============================================================================
# Development targets — compatibility shim over Taskfile.yml
# =============================================================================
#
# These targets delegate to `task`. They do not contain build logic.
# They exist so `make build` works for someone who does not have Task
# installed (the target checks for `task` and prints a legible error if
# it is missing) and so external tooling that expects a Makefile finds
# one.
# =============================================================================

help: ## Print this help.
	@echo "Forge — development and release targets"
	@echo ""
	@echo "Development (delegates to Taskfile.yml):"
	@awk 'BEGIN { FS = ":.*?## " } /^(build|build-debug|run|test|test-integration|check|fmt|fmt-check|vet|lint|clean):.*?## / { printf "  make %-24s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Release (delegates to scripts/build.go):"
	@awk 'BEGIN { FS = ":.*?## " } /^(build-all|build-linux|build-darwin|build-windows|checksums|sign|sbom|release-dry-run|release):.*?## / { printf "  make %-24s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Diagnostics:"
	@awk 'BEGIN { FS = ":.*?## " } /^(toolchain|verify-cgo|verify-mod|verify-tagged|verify:injection|verify:injection-prefix):.*?## / { printf "  make %-24s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Run 'task --list' for the full Taskfile target list."

build: ## Build ./forge with injected metadata (delegates to Task).
	@command -v $(TASK) >/dev/null 2>&1 || { \
		echo "error: task is not installed. Install it from https://taskfile.dev" >&2; \
		echo "       or build directly: go build ./cmd/forge" >&2; \
		exit 1; \
	}
	@$(TASK) build

build-debug: ## Build ./forge without -trimpath for local debugging (delegates to Task).
	@$(TASK) build:debug

run: build ## Build then run the binary.
	@./forge --help

test: ## Run unit tests with the race detector (delegates to Task).
	@$(TASK) test

test-integration: ## Run process-boundary integration tests (delegates to Task).
	@$(TASK) test:integration

check: ## Run the full quality gate (delegates to Task).
	@$(TASK) check

fmt: ## Format Go source files (delegates to Task).
	@$(TASK) fmt

fmt-check: ## Verify Go source files are formatted (delegates to Task).
	@$(TASK) fmt:check

vet: ## Run go vet across all packages (delegates to Task).
	@$(TASK) vet

lint: ## Run Markdown and configuration linters (delegates to Task).
	@$(TASK) lint

clean: ## Remove the built binary and dist/ (delegates to Task for the binary).
	@$(TASK) clean
	@rm -rf dist
	@echo "OK: removed forge, forge.exe, dist/"

verify:injection: ## Build and print injected metadata (delegates to Task).
	@$(TASK) verify:injection

verify:injection-prefix: ## Fail if MODULE does not match go.mod (delegates to Task).
	@$(TASK) verify:injection-prefix

# =============================================================================
# Release targets — delegate to scripts/build.go
# =============================================================================
#
# Cross-compilation logic lives in scripts/build.go, not here. The
# driver reads docs/reference/platforms.yaml, injects the four -X
# variables, packages each binary, writes SHA256SUMS, produces a
# CycloneDX SBOM, and signs with cosign.
#
# Every release target depends on the four preconditions:
#
#   toolchain      prints the Go version
#   verify-mod     fails on go.mod/go.sum drift
#   verify-cgo     fails if CGO_ENABLED is not 0
#   verify-tagged  fails if HEAD is not an exact tag
#
# The preconditions run before the driver, so a release from a dirty
# tree fails before any binary is produced.
# =============================================================================

toolchain: ## Print the Go toolchain version.
	@$(GO) version

verify-mod: ## Fail if go.mod or go.sum has drifted.
	@$(TASK) module:verify:tidy

verify-cgo: ## Fail if CGO_ENABLED is not 0.
	@if [ "$$( $(GO) env CGO_ENABLED )" != "0" ]; then \
	  printf 'FAIL: CGO_ENABLED must be 0 for release builds\n' >&2; \
	  printf '      actual: %s\n' "$$( $(GO) env CGO_ENABLED )" >&2; \
	  printf '      run: export CGO_ENABLED=0\n' >&2; \
	  exit 1; \
	fi
	@printf 'OK: CGO_ENABLED=0\n'

verify-tagged: ## Fail if HEAD is not an exact tag (delegates to Task).
	@$(TASK) verify:tagged

build-all: toolchain verify-mod verify-cgo verify-tagged ## Cross-compile every Tier 1/2 platform.
	@$(GO) run ./scripts/build.go --all

build-linux: toolchain verify-mod verify-cgo verify-tagged ## Cross-compile linux/amd64 and linux/arm64.
	@$(GO) run ./scripts/build.go --os=linux

build-darwin: toolchain verify-mod verify-cgo verify-tagged ## Cross-compile darwin/amd64 and darwin/arm64.
	@$(GO) run ./scripts/build.go --os=darwin

build-windows: toolchain verify-mod verify-cgo verify-tagged ## Cross-compile windows/amd64 and windows/arm64.
	@$(GO) run ./scripts/build.go --os=windows

checksums: ## Produce dist/SHA256SUMS.
	@$(GO) run ./scripts/build.go --checksums-only

sign: ## Sign dist/forge-* with cosign keyless.
	@$(GO) run ./scripts/build.go --sign-only

sbom: ## Produce a CycloneDX SBOM at dist/forge.sbom.json.
	@$(GO) run ./scripts/build.go --sbom-only

# Run the full release pipeline without publishing.
#
# The order is deliberate:
#
#   1. build-all        cross-compile every Tier 1/2 platform
#   2. checksums        dist/SHA256SUMS
#   3. sbom             dist/forge.sbom.json
#   4. verify-binaries  run the ten-check smoke suite against the
#                       artifacts
#   5. sign             cosign keyless
#
# verify-binaries runs before sign. A signature over a broken
# binary is worse than no signature: it certifies the breakage.
# A failure in verify-binaries stops the pipeline before signing.
#
# The preconditions (toolchain, verify-mod, verify-cgo,
# verify-tagged) run before the first step. verify-tagged delegates
# to the Taskfile because the Makefile shim must not run git; see
# verify:injection:single-authority in Taskfile.yml.
release-dry-run: toolchain verify-mod verify-cgo verify-tagged ## Run the full release pipeline without publishing.
	@$(GO) run ./scripts/build.go --all --checksums-only
	@$(GO) run ./scripts/build.go --sbom-only
	@$(GO) run ./cmd/forge-internal-verify --dist=dist --verbose
	@$(GO) run ./scripts/build.go --sign-only
	@printf '\n'
	@printf 'OK: dry run complete\n'
	@printf '    artifacts in dist/\n'
	@printf '    nothing was published\n'

release: release-dry-run ## Run the full release pipeline, then publish.
	@$(GO) run ./scripts/build.go --publish