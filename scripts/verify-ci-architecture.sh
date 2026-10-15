#!/usr/bin/env bash
# verify-ci-architecture.sh — verify the CI architecture contract.
#
# This script is the shell entry point used by Taskfile.yml and by local
# developers. It runs the Go tests that enforce the architecture. It is
# deliberately thin: the tests are the source of truth.
#
# Exit codes:
#   0 — architecture is consistent.
#   1 — architecture is violated.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

go test ./internal/ci/... -count=1 -v