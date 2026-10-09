#!/usr/bin/env bash
set -euo pipefail

ADR_DIR="${1:?ADR directory required}"
shift || true
STATUS_FILTER=""

for arg in "$@"; do
  case "$arg" in
    --status=*) STATUS_FILTER="${arg#--status=}" ;;
  esac
done

printf '%-10s %-12s %-60s\n' "ID" "STATUS" "TITLE"
printf '%-10s %-12s %-60s\n' "----" "-------" "-----"

for f in "${ADR_DIR}"/ADR-*.md; do
  [[ -e "$f" ]] || continue
  ID=$(basename "$f" .md | sed -E 's/^(ADR-[0-9]+)-.*/\1/')
  TITLE=$(grep -m1 '^# ' "$f" | sed 's/^# //')
  STATUS=$(grep -m1 '^\*\*Status:\*\*' "$f" | sed 's/\*\*Status:\*\* *//')

  if [[ -n "$STATUS_FILTER" && "$STATUS" != *"$STATUS_FILTER"* ]]; then
    continue
  fi

  printf '%-10s %-12s %-60s\n' "$ID" "$STATUS" "$TITLE"
done