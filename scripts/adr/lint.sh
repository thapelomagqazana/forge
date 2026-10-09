#!/usr/bin/env bash
set -euo pipefail

ADR_DIR="${1:?ADR directory required}"
REQUIRED=("Status:" "Date:" "Deciders:" "## 1. Context" "## 2. Options Considered" "## 3. Decision" "## 4. Consequences")
FAILED=0

for f in "${ADR_DIR}"/ADR-*.md; do
  [[ -e "$f" ]] || continue
  for field in "${REQUIRED[@]}"; do
    if ! grep -q "$field" "$f"; then
      echo "FAIL: $f missing '$field'"
      FAILED=1
    fi
  done
done

if [[ $FAILED -eq 0 ]]; then
  echo "All ADRs pass lint."
else
  exit 1
fi