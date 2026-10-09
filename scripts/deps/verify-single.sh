#!/usr/bin/env sh
# =============================================================================
# scripts/deps/verify-single.sh — allowlist verification (WBS 2.5.1)
# =============================================================================
#
# Verify that the set of direct dependencies in go.mod matches the
# allowlist declared in Taskfile.yml (`DEP_ALLOWED_DIRECT`).
#
# The allowlist is passed as the first argument: a space-separated
# list of module paths. For example:
#
#   sh scripts/deps/verify-single.sh "github.com/spf13/cobra"
#
# Adding a dependency requires updating DEP_ALLOWED_DIRECT in
# Taskfile.yml AND adding a registry entry to
# docs/dependency-policy.md in the same commit.
#
# Exit codes:
#
#   0  The actual direct dependencies match the allowlist.
#   1  The sets differ.
# =============================================================================

set -eu

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

# Parse the allowlist. It arrives as a single space-separated string.
allowlist="${1:-}"
if [ -z "$allowlist" ]; then
    echo "FAIL: no allowlist provided as first argument" >&2
    echo "      Usage: sh scripts/deps/verify-single.sh '<module> <module> ...'" >&2
    exit 1
fi

expected=$(printf '%s\n' "$allowlist" | tr ' ' '\n' | grep -v '^$' | sort)

# Extract actual direct dependencies.
mainmodule=$(go list -m)

actual=$(
    go list -m -json all 2>/dev/null \
    | awk '
        /^\{/          { path=""; indirect=0 }
        /"Path":/      {
            gsub(/.*"Path": *"/, ""); gsub(/".*/, "");
            path=$0
        }
        /"Indirect": true/ { indirect=1 }
        /^\}/          {
            if (path != "" && !indirect) print path
        }
    ' \
    | grep -v "^$mainmodule$" \
    | sort
)

if [ "$actual" != "$expected" ]; then
    echo "FAIL: direct dependencies do not match DEP_ALLOWED_DIRECT" >&2
    echo "  expected:" >&2
    printf '%s\n' "$expected" | sed 's/^/    /' >&2
    echo "  actual:" >&2
    printf '%s\n' "$actual" | sed 's/^/    /' >&2
    echo "" >&2
    echo "If a new dependency was intentionally added, update" >&2
    echo "DEP_ALLOWED_DIRECT in Taskfile.yml and add a registry" >&2
    echo "entry to docs/dependency-policy.md in the same commit." >&2
    exit 1
fi

echo "OK: direct dependencies match DEP_ALLOWED_DIRECT"