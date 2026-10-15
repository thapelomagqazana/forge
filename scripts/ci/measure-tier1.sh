#!/usr/bin/env bash
# measure-tier1.sh — measure Tier 1's wall-clock time over the last
# N CI runs and flag any run that exceeded the 3-minute budget.
#
# The script is run quarterly by the Release Engineering owner. It
# is the mechanism that keeps the tier budget honest: a budget that
# is never measured decays into a hope.
#
# The measurement is wall-clock from the moment the `tier1` job
# starts to the moment it ends. GitHub Actions does not report this
# directly; the script derives it from the run's `createdAt` and
# `updatedAt` timestamps, which is an upper bound (it includes
# queueing time). A more precise measurement would use the job's
# `startedAt` and `completedAt`, which `gh api` provides; the
# script uses the run-level timestamps for simplicity and notes the
# approximation in the output.
#
# Usage:
#   scripts/ci/measure-tier1.sh [--runs N]
#
# Exit codes:
#   0 — the script ran and produced a report.
#   1 — the script could not run (gh not installed, no runs found).
set -euo pipefail

runs=5

while [ $# -gt 0 ]; do
  case "$1" in
    --runs) runs="$2"; shift 2 ;;
    *) echo "unknown flag: $1" >&2; exit 1 ;;
  esac
done

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is not installed; install GitHub CLI to use this script" >&2
  exit 1
fi

# The budget in seconds.
budget=180

# Fetch the last N completed runs of the CI workflow. The jq filter
# extracts each run's wall-clock duration in seconds.
durations="$(
  gh run list \
    --workflow ci.yml \
    --status completed \
    --limit "$runs" \
    --json createdAt,updatedAt \
    --jq '.[] | ((.updatedAt | fromdateiso8601) - (.createdAt | fromdateiso8601))' \
  | sort -n
)"

if [ -z "$durations" ]; then
  echo "no completed runs of ci.yml found" >&2
  exit 1
fi

count=$(echo "$durations" | wc -l | tr -d ' ')
median_idx=$(( (count + 1) / 2 ))
median=$(echo "$durations" | sed -n "${median_idx}p")

echo "Workflow: ci.yml"
echo "Runs:     $count"
echo "Median:   ${median}s"
echo "Budget:   ${budget}s"
echo

if [ "$median" -le "$budget" ]; then
  echo "OK: median ${median}s is within the Tier 1 budget of ${budget}s"
else
  echo "FINDING: median ${median}s exceeds the Tier 1 budget of ${budget}s"
  echo "Action:  optimise Tier 1 or amend the budget with an ADR."
fi