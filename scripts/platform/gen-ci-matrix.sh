#!/usr/bin/env bash
# gen-ci-matrix.sh — emit a GitHub Actions matrix from platforms.yaml.
#
# Usage:
#   ./scripts/platform/gen-ci-matrix.sh [--tier N]
#
# Output: a JSON object suitable for `strategy.matrix` in a GitHub
# Actions workflow:
#
#   {
#     "include": [
#       {"name": "linux-amd64", "goos": "linux", "goarch": "amd64",
#        "runner": "ubuntu-22.04", "tier": 1},
#       ...
#     ]
#   }
#
# Exit codes:
#   0    success
#   1    tool or validation failure
#   2    usage error
#   127  go is not on PATH
#
# The script intentionally avoids jq and yq so it runs on a bare
# ubuntu-latest runner. It delegates to cmd/forge-internal-platform,
# which is a thin wrapper over internal/platform. It MUST NOT invoke
# cmd/forge: that is the user-facing CLI and it has no --format=gha-matrix
# flag. The build-time matrix logic lives in the internal helper.
set -euo pipefail

# Resolve the repo root from the script's own location, not from cwd.
# `go test` runs each package's tests with cwd set to the package
# directory (e.g. test/ci/), not the repo root. Anchoring on
# BASH_SOURCE makes the script safe to invoke from any directory.
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

if ! command -v go >/dev/null 2>&1; then
  echo "gen-ci-matrix: go not found on PATH" >&2
  exit 127
fi

tier=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tier) tier="${2:-}"; shift 2 ;;
    -h|--help)
      sed -n '2,25p' "$0"
      exit 0
      ;;
    *) echo "gen-ci-matrix: unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [[ -n "$tier" ]]; then
  # `exec` replaces the shell so the script's exit code is the tool's
  # exit code, not bash's.
  exec go run ./cmd/forge-internal-platform --format=gha-matrix --tier "$tier"
fi
exec go run ./cmd/forge-internal-platform --format=gha-matrix