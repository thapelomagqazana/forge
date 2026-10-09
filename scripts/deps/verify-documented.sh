#!/usr/bin/env sh
# =============================================================================
# scripts/deps/verify-documented.sh — registry verification (WBS 2.5.1)
# =============================================================================
#
# Verify that every direct dependency has an entry in the registry
# section of the dependency policy document.
#
# The policy path is passed as the first argument:
#
#   sh scripts/deps/verify-documented.sh docs/dependency-policy.md
#
# Exit codes:
#
#   0  Every direct dependency is documented.
#   1  One or more dependencies are missing from the policy.
# =============================================================================

set -eu

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

policy="${1:-docs/dependency-policy.md}"
if [ ! -f "$policy" ]; then
    echo "FAIL: $policy is missing" >&2
    exit 1
fi

# Extract direct dependencies.
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
    | grep -v "^$mainmodule$"
)

fail=0

while read -r path; do
    [ -z "$path" ] && continue
    if grep -q "$path" "$policy"; then
        echo "OK: $path is documented in $policy"
    else
        echo "FAIL: $path is not documented in $policy" >&2
        echo "      Add a registry entry to Section 7 of the policy." >&2
        fail=1
    fi
done <<EOF
$actual
EOF

exit $fail