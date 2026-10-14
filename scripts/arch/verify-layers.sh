#!/usr/bin/env bash
#
# verify-layers.sh — fast architectural guardrail for Forge.
#
# Purpose
#   Fail fast when a change introduces an import that violates the clean
#   architecture layer matrix (ADR-040) or leaves a new internal package
#   unclassified. Designed to run as a pre-commit hook and in CI.
#
# Usage
#   scripts/arch/verify-layers.sh          # run the arch tests
#   scripts/arch/verify-layers.sh -v       # verbose
#
# Exit codes
#   0  architecture is clean
#   1  violation or unclassified package
#   2  environment error (go missing, wrong directory)
#
# Why a script and not just `go test`
#   The script pins the *specific* packages and flags so that the hook is
#   stable even if the repo's default test invocation changes. It also prints
#   a human-readable hint on failure, which the raw `go test` output does not.

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v go >/dev/null 2>&1; then
  echo "verify-layers: go not found in PATH" >&2
  exit 2
fi

if [[ ! -f go.mod ]]; then
  echo "verify-layers: go.mod not found in $ROOT" >&2
  exit 2
fi

verbose=""
if [[ "${1:-}" == "-v" ]]; then
  verbose="-v"
fi

# The arch package is the only thing this script needs to run. It loads the
# full internal/... graph itself, so we do not need ./... here — which keeps
# the hook fast (no compilation of unrelated packages).
if ! go test ${verbose} ./internal/arch/...; then
  cat >&2 <<'EOF'

──────────────────────────────────────────────────────────────
Architecture violation detected (ADR-040).

Fix one of:
  • Add your package under an existing layer prefix:
      internal/cli, internal/application, internal/domain,
      internal/infrastructure, internal/shared
  • If your package predates the model, add a Grandfather entry in
      internal/arch/layers.go  — with a Reason and RemovalPhase.
  • If your import is genuinely allowed by the model, update
      docs/architecture/clean-architecture.md AND
      internal/arch/layers.go in the same PR (the golden test will
      force the docs and code to agree).

See docs/architecture/clean-architecture.md §3 and §5.
──────────────────────────────────────────────────────────────
EOF
  exit 1
fi

echo "verify-layers: OK"