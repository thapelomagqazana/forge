#!/usr/bin/env bash
set -euo pipefail

ADR_DIR="${1:?ADR directory required}"
TITLE="${2:?Title required}"
TEMPLATE="${ADR_DIR}/template.md"

if [[ ! -d "$ADR_DIR" ]]; then
  echo "Error: ADR directory does not exist: $ADR_DIR" >&2
  exit 1
fi

if [[ ! -f "$TEMPLATE" ]]; then
  echo "Error: ADR template does not exist: $TEMPLATE" >&2
  exit 1
fi

# Find the next available number
LAST=$(ls "${ADR_DIR}"/ADR-*.md 2>/dev/null \
  | sed -E 's/.*ADR-([0-9]+).*/\1/' \
  | sort -n \
  | tail -1 || true)
NEXT=$(printf "%03d" $((10#${LAST:-0} + 1)))

# Generate kebab-case slug
SLUG=$(printf '%s' "$TITLE" \
  | tr '[:upper:]' '[:lower:]' \
  | sed 's/[^a-z0-9]/-/g' \
  | sed 's/--*/-/g' \
  | sed 's/^-//;s/-$//')

if [[ -z "$SLUG" ]]; then
  echo "Error: Title produced an empty slug: '$TITLE'" >&2
  exit 1
fi

FILENAME="${ADR_DIR}/ADR-${NEXT}-${SLUG}.md"
DATE=$(date +%Y-%m-%d)

if [[ -e "$FILENAME" ]]; then
  echo "Error: ADR already exists: $FILENAME" >&2
  exit 1
fi

# Populate template
sed \
  -e "s/ADR-NNN/ADR-${NEXT}/" \
  -e "s/\[Short Decision Title\]/${TITLE}/" \
  -e "s/YYYY-MM-DD/${DATE}/" \
  "$TEMPLATE" > "$FILENAME"

echo "Created: $FILENAME"