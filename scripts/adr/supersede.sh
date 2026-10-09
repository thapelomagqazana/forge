#!/usr/bin/env bash
set -euo pipefail

ADR_DIR="${1:?ADR directory required}"
OLD_NUM="${2:?Old ADR number required}"
NEW_TITLE="${3:?New title required}"

OLD_FILE=$(ls "${ADR_DIR}"/ADR-$(printf "%03d" "$OLD_NUM")-*.md 2>/dev/null | head -1 || true)

if [[ -z "$OLD_FILE" ]]; then
  echo "Error: ADR-$(printf "%03d" "$OLD_NUM") not found in $ADR_DIR" >&2
  exit 1
fi

# 1. Create the new ADR
NEW_FILE=$(bash "$(dirname "$0")/new.sh" "$ADR_DIR" "$NEW_TITLE" | grep -oP 'Created: \K.*')

# 2. Update the old ADR's status
sed -i.bak "s/^\*\*Status:\*\*.*/\*\*Status:\*\* Superseded by $(basename "$NEW_FILE" .md)/" "$OLD_FILE"
rm -f "${OLD_FILE}.bak"

# 3. Record the supersession in the new ADR
OLD_ID=$(basename "$OLD_FILE" .md)
sed -i.bak "s/^\*\*Supersedes:\*\* —/\*\*Supersedes:\*\* ${OLD_ID}/" "$NEW_FILE"
rm -f "${NEW_FILE}.bak"

echo "Superseded: ${OLD_ID} → $(basename "$NEW_FILE" .md)"