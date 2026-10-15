#!/usr/bin/env bash
# =============================================================================
# measure-tier3.sh — measure Tier 3's wall-clock time over the last N CI
# runs and report the median against the tier's budget.
# =============================================================================
#
# Purpose
# -------
#
# Tier 3's budget in internal/ci/tiers.go is 60 minutes. The budget is
# enforced by the quarterly review of measured timings, not by a CI
# check: a blocking timing check would fail on a noisy runner and
# block main pushes for a reason unrelated to the contributor's
# change. This script is the measurement the review reads.
#
# The script fetches the last N completed runs of ci.yml from the
# GitHub API, extracts each run's wall-clock duration, sorts the
# durations, and reports the median against the 60-minute budget. A
# median that exceeds the budget is reported as a finding, not as a
# script failure: the owner decides whether to optimise the tier or
# to amend the budget.
#
# # How the duration is derived
#
# GitHub Actions does not report a run's wall-clock duration
# directly. The script derives it from the run's `createdAt` and
# `updatedAt` timestamps: the difference is the run's elapsed time
# from creation to completion, which is an upper bound on the
# job-level wall-clock (it includes queueing time). A more precise
# measurement would use the run's jobs' `startedAt` and
# `completedAt` via `gh api /repos/{owner}/{repo}/actions/runs/{id}/jobs`,
# but that is one API call per run. The two-timestamp form is one API
# call per run list and is precise enough for a quarterly review.
#
# # Why the script is not in CI
#
# The script depends on `gh` (the GitHub CLI) and on network access
# to the GitHub API. A CI job that ran it would need `gh` installed
# and a token with `actions: read` on the repository. The script's
# consumer is a human; the human's environment has both. The script
# is documented in the Release Engineering calendar, not in
# ci.yml.
#
# # Usage
#
#   scripts/ci/measure-tier3.sh [--runs N] [--workflow NAME]
#
#   --runs N        Number of runs to sample. Default: 20. The
#                   default matches the "last 20 PRs" cadence
#                   documented for Tier 1 and Tier 2.
#   --workflow NAME Workflow file to sample. Default: ci.yml. The
#                   value is the workflow's file name; the script
#                   passes it to `gh run list --workflow`.
#
# # Exit codes
#
#   0 — the script ran and produced a report. The report may contain
#       a finding; the exit code does not distinguish "no finding"
#       from "finding". The finding is in the output.
#   1 — the script could not run: `gh` is not installed, no runs
#       were found, or the API call failed. The error is printed to
#       stderr.
#
# # Related
#
#   - scripts/ci/measure-tier1.sh — the Tier 1 counterpart.
#   - scripts/ci/measure-tier2.sh — the Tier 2 counterpart.
#   - docs/reference/ci-tiers.md — the tier table.
#   - docs/decisions/ADR-047-tier3-depth-workflow.md — the ADR.
# =============================================================================
set -euo pipefail

# --- Defaults --------------------------------------------------------------

# The number of runs to sample. Twenty is the same default the Tier 1
# and Tier 2 scripts use; the number matches the "last 20 PRs"
# cadence the tier model documents.
runs=20

# The workflow file to sample. The default is ci.yml, which invokes
# the tier3 reusable workflow. The caller can pass a different value
# to sample a different workflow (for example, a future workflow that
# runs Tier 3 on its own).
workflow="ci.yml"

# The budget in seconds. Sixty minutes, matching
# internal/ci/tiers.go's Tier3.Budget.
budget=3600

# --- Argument parsing ------------------------------------------------------

# Parse the script's flags. The loop handles --runs, --workflow, and
# -h/--help. An unknown flag is an error; the script exits 1 with a
# message naming the flag.
while [ $# -gt 0 ]; do
  case "$1" in
    --runs)
      runs="$2"
      shift 2
      ;;
    --workflow)
      workflow="$2"
      shift 2
      ;;
    -h|--help)
      cat <<'USAGE'
Usage: measure-tier3.sh [--runs N] [--workflow NAME]

  --runs N        Number of runs to sample. Default: 20.
  --workflow NAME Workflow file to sample. Default: ci.yml.

The script reports the median duration of the last N completed runs
of the workflow against the Tier 3 budget (60 minutes). A finding is
reported; the script's exit code is 0 whether or not a finding is
present.
USAGE
      exit 0
      ;;
    *)
      echo "measure-tier3: unknown flag: $1" >&2
      exit 1
      ;;
  esac
done

# Validate the run count. A non-numeric value or a value less than 1
# is an error. The check uses a POSIX-compatible expression so that
# the script runs under bash without relying on bash's [[ ]].
case "$runs" in
  ''|*[!0-9]*)
    echo "measure-tier3: --runs must be a positive integer (got: $runs)" >&2
    exit 1
    ;;
esac
if [ "$runs" -lt 1 ]; then
  echo "measure-tier3: --runs must be at least 1 (got: $runs)" >&2
  exit 1
fi

# --- Prerequisites ---------------------------------------------------------

# The script requires the GitHub CLI. `gh` is the standard way to
# read the repository's run list from a shell; the alternative (the
# REST API via curl) would require the caller to supply a token and
# construct the URL. `gh` handles authentication from the caller's
# keyring or GH_TOKEN environment variable.
if ! command -v gh >/dev/null 2>&1; then
  echo "measure-tier3: gh is not installed; install GitHub CLI to use this script" >&2
  exit 1
fi

# --- Fetch the run durations -----------------------------------------------

# Fetch the last N completed runs of the workflow. The `gh run list`
# output is JSON; the `--jq` expression extracts each run's duration
# in seconds by subtracting the creation timestamp from the update
# timestamp. The `sort -n` sorts the durations numerically; the
# result is the input to the median calculation.
#
# The `--status completed` filter excludes in-flight and queued runs.
# A queued run has a `createdAt` timestamp and no `updatedAt`
# timestamp; including it would produce a nonsense duration.
#
# The 2>/dev/null redirect suppresses gh's progress messages. An
# error (a missing repository, an authentication failure) is written
# to stderr and the script exits because `set -o pipefail` is on.
if ! durations="$(
  gh run list \
    --workflow "$workflow" \
    --status completed \
    --limit "$runs" \
    --json createdAt,updatedAt \
    --jq '.[] | ((.updatedAt | fromdateiso8601) - (.createdAt | fromdateiso8601))' \
    2>/dev/null \
  | sort -n
)"; then
  echo "measure-tier3: failed to list runs of $workflow; check gh authentication and network access" >&2
  exit 1
fi

# If the durations are empty, the workflow has no completed runs, or
# the repository has no runs of that workflow. The script cannot
# produce a report; it exits with an error naming the workflow.
if [ -z "$durations" ]; then
  echo "measure-tier3: no completed runs of $workflow found" >&2
  exit 1
fi

# --- Compute the median ----------------------------------------------------

# Count the durations. The count is the number of lines in the
# output; `wc -l` counts lines, and the `tr -d ' '` strips the
# leading spaces `wc` produces on some systems.
count=$(echo "$durations" | wc -l | tr -d ' ')

# The median's index. For an even count, the script takes the lower
# of the two middle values; the true median would be the average of
# the two, but the difference is not material for a quarterly review.
# The index is 1-based because `sed -n "${median_idx}p"` is 1-based.
median_idx=$(( (count + 1) / 2 ))
median=$(echo "$durations" | sed -n "${median_idx}p")

# Convert the median to minutes and seconds for the report. Bash's
# integer arithmetic is sufficient: the durations are whole seconds.
median_min=$(( median / 60 ))
median_sec=$(( median % 60 ))
budget_min=$(( budget / 60 ))

# --- Report ----------------------------------------------------------------

echo "Workflow: $workflow"
echo "Runs:     $count"
echo "Median:   ${median}s (${median_min}m ${median_sec}s)"
echo "Budget:   ${budget}s (${budget_min}m)"
echo

# The comparison. The script reports one of three outcomes:
#
#   OK       — the median is at or below the budget.
#   FINDING  — the median exceeds the budget; the owner should
#              review the tier.
#
# The script does not report a third "critical" outcome: the
# threshold between "needs review" and "needs immediate action" is a
# judgement call, and the owner makes it after reading the report.
if [ "$median" -le "$budget" ]; then
  echo "OK: median ${median}s is within the Tier 3 budget of ${budget}s"
  exit 0
fi

echo "FINDING: median ${median}s exceeds the Tier 3 budget of ${budget}s"
echo
echo "Action: the Release Engineering owner should review the tier and"
echo "either optimise it or amend the budget in internal/ci/tiers.go"
echo "with an update to ADR-047."
echo
echo "Sample: the last ${count} completed runs' durations (sorted):"
echo "$durations" | while read -r d; do
  printf '  %ss\n' "$d"
done
exit 0