#!/usr/bin/env bash
set -euo pipefail

ADR_DIR="${1:?ADR directory required}"
INDEX="${ADR_DIR}/README.md"

if [[ ! -d "$ADR_DIR" ]]; then
  echo "Error: ADR directory does not exist: $ADR_DIR" >&2
  exit 1
fi

# Write the header + empty tables
{
  echo "# Forge Decision Log"
  echo
  echo "This index is auto-generated. Do not edit it manually."
  echo
  echo "## Accepted"
  echo
  echo "| ID | Title | Date |"
  echo "|----|-------|------|"
  echo
  echo "## Proposed"
  echo
  echo "| ID | Title | Date |"
  echo "|----|-------|------|"
  echo
  echo "## Superseded"
  echo
  echo "| ID | Title | Date |"
  echo "|----|-------|------|"
  echo
  echo "## Deprecated"
  echo
  echo "| ID | Title | Date |"
  echo "|----|-------|------|"
} > "$INDEX"

# Collect ADRs and append rows to the correct section
for f in "${ADR_DIR}"/ADR-*.md; do
  [[ -e "$f" ]] || continue
  [[ "$(basename "$f")" == *template* ]] && continue

  ID=$(basename "$f" .md | sed -E 's/^(ADR-[0-9]+)-.*/\1/')
  TITLE=$(grep -m1 '^# ' "$f" | sed 's/^# //')
  STATUS=$(grep -m1 '^\*\*Status:\*\*' "$f" | sed 's/\*\*Status:\*\* *//')
  DATE=$(grep -m1 '^\*\*Date:\*\*' "$f" | sed 's/\*\*Date:\*\* *//')

  case "$STATUS" in
    Accepted)   SECTION="Accepted" ;;
    Proposed)   SECTION="Proposed" ;;
    Superseded*) SECTION="Superseded" ;;
    Deprecated) SECTION="Deprecated" ;;
    *)          SECTION="Proposed" ;;
  esac

  ROW="| [${ID}](./$(basename "$f")) | ${TITLE} | ${DATE} |"

  awk -v section="## ${SECTION}" -v row="$ROW" '
    $0 == section { in_section = 1; print; next }
    /^## / { in_section = 0 }
    in_section && /^\|----\|/ { print; print row; in_section = 0; next }
    { print }
  ' "$INDEX" > "${INDEX}.tmp" && mv "${INDEX}.tmp" "$INDEX"
done

echo "Index updated: $INDEX"