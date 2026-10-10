# =============================================================================
# Forge — Makefile (compatibility shim)
# =============================================================================
#
# This Makefile is NOT the authority for build metadata injection.
# The authority is Taskfile.yml. This file exists only so that
# downstream consumers who do not have Task installed — packagers,
# minimal CI images, contributors on systems where Task is hard to
# install — can still run `make build`.
#
# Every target delegates to `task`. Do not add injection logic here:
# it would duplicate Taskfile.yml and drift. The absence of -X flags,
# git commands, and a MODULE variable in this file is deliberate and
# enforced by TestAC1_TaskfileInjectsAllFourVariables, which reads
# Taskfile.yml, not this file.
#
# See WBS 6.1.2 and docs/development.md "Build Metadata Injection".
# =============================================================================

TASK ?= task

.PHONY: build build:release verify:injection verify:injection-prefix clean help

build: ## Build ./forge with injected metadata (delegates to Task).
	@command -v $(TASK) >/dev/null 2>&1 || { \
		echo "error: task is not installed. Install it from https://taskfile.dev" >&2; \
		echo "       or build directly: go build ./cmd/forge" >&2; \
		exit 1; \
	}
	$(TASK) build

build:release: ## Build a release binary (delegates to Task).
	$(TASK) build:release

verify:injection: ## Build and print injected metadata (delegates to Task).
	$(TASK) verify:injection

verify:injection-prefix: ## Fail if MODULE does not match go.mod (delegates to Task).
	$(TASK) verify:injection-prefix

clean: ## Remove the built binary (delegates to Task).
	$(TASK) clean

help: ## Print this help.
	@echo "This Makefile is a compatibility shim over Taskfile.yml."
	@echo "Run 'task --list' for the full target list."
	@echo ""
	@echo "Available shim targets:"
	@grep -E '^[a-zA-Z_:%-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  %-28s %s\n", $$1, $$2}'