#!/usr/bin/env sh
# =============================================================================
# scripts/deps/verify-no-unused.sh — unused dependency check (WBS 2.5.1)
# =============================================================================
#
# Verify that no direct dependency is declared without being imported.
#
# An unused direct dependency is a red flag: it usually means a
# dependency was added speculatively and never used, or was removed
# from the code without being removed from go.mod.
#
# `go mod why -m <path>` outputs the import chain that requires the
# module. If the chain ends with "does not need module X", the module
# is not imported by any file.
#
# Exit codes:
#
#   0  Every direct dependency is imported.
#   1  One or more direct dependencies are unused.
# =============================================================================

set -eu

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

mainmodule=$(go list -m)

direct=$(
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
    why=$(go mod why -m "$path" 2>/dev/null || echo "")
    if echo "$why" | grep -q "does not need module"; then
        echo "FAIL: $path is declared as a direct dependency but is" >&2
        echo "      not imported by any file. Remove it from go.mod," >&2
        echo "      or import it." >&2
        fail=1
    else
        echo "OK: $path is imported"
    fi
done <<EOF
$direct
EOF

exit $fail