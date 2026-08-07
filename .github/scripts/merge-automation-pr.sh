#!/usr/bin/env bash

set -euo pipefail

pr_number="${1:?pull request number is required}"
check_name="${2:-linting}"
repository="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
max_update_attempts=4
check_timeout_seconds=3600
poll_interval_seconds=10

wait_for_check() {
  local head_sha="$1"
  local deadline=$((SECONDS + check_timeout_seconds))

  while ((SECONDS < deadline)); do
    local current_head_sha payload status conclusion details_url
    current_head_sha="$(
      gh pr view "$pr_number" --repo "$repository" --json headRefOid --jq '.headRefOid'
    )"
    if [[ "$current_head_sha" != "$head_sha" ]]; then
      echo "Pull request head changed from ${head_sha} to ${current_head_sha} while checks were pending"
      return 2
    fi

    payload="$(
      gh api \
        -H "Accept: application/vnd.github+json" \
        "repos/${repository}/commits/${head_sha}/check-runs?check_name=${check_name}&filter=latest&per_page=100"
    )"
    status="$(jq -r '.check_runs | max_by(.id) | .status // "missing"' <<<"$payload")"
    conclusion="$(jq -r '.check_runs | max_by(.id) | .conclusion // ""' <<<"$payload")"
    details_url="$(jq -r '.check_runs | max_by(.id) | .html_url // ""' <<<"$payload")"

    case "${status}:${conclusion}" in
      completed:success)
        echo "Required check '${check_name}' passed for ${head_sha}"
        return 0
        ;;
      completed:*)
        echo "::error::Required check '${check_name}' completed with '${conclusion}' (${details_url})"
        return 1
        ;;
      *)
        echo "Waiting for '${check_name}' on ${head_sha}: ${status}${conclusion:+/${conclusion}}"
        sleep "$poll_interval_seconds"
        ;;
    esac
  done

  echo "::error::Timed out waiting for '${check_name}' on ${head_sha}"
  return 1
}

wait_for_updated_head() {
  local previous_head_sha="$1"
  local deadline=$((SECONDS + check_timeout_seconds))

  while ((SECONDS < deadline)); do
    local current_head_sha
    current_head_sha="$(
      gh pr view "$pr_number" --repo "$repository" --json headRefOid --jq '.headRefOid'
    )"

    if [[ "$current_head_sha" != "$previous_head_sha" ]]; then
      echo "GitHub updated PR branch from ${previous_head_sha} to ${current_head_sha}"
      return 0
    fi

    echo "Waiting for GitHub to finish updating PR branch from ${previous_head_sha}"
    sleep "$poll_interval_seconds"
  done

  echo "::error::Timed out waiting for GitHub to update PR branch from ${previous_head_sha}"
  return 1
}

for ((attempt = 1; attempt <= max_update_attempts; attempt++)); do
  pr_data="$(gh pr view "$pr_number" --repo "$repository" --json baseRefName,headRefOid,isDraft,state)"
  state="$(jq -r '.state' <<<"$pr_data")"
  is_draft="$(jq -r '.isDraft' <<<"$pr_data")"
  base_ref="$(jq -r '.baseRefName' <<<"$pr_data")"
  head_sha="$(jq -r '.headRefOid' <<<"$pr_data")"

  if [[ "$state" != "OPEN" ]]; then
    echo "::error::Pull request #${pr_number} is not open"
    exit 1
  fi
  if [[ "$is_draft" == "true" ]]; then
    echo "::error::Pull request #${pr_number} is a draft"
    exit 1
  fi

  if wait_for_check "$head_sha"; then
    :
  else
    wait_status=$?
    if ((wait_status == 2)); then
      continue
    fi
    exit "$wait_status"
  fi

  latest_head_sha="$(
    gh pr view "$pr_number" --repo "$repository" --json headRefOid --jq '.headRefOid'
  )"
  if [[ "$latest_head_sha" != "$head_sha" ]]; then
    echo "Pull request head changed while checks ran; validating the new head"
    continue
  fi

  base_sha="$(
    gh api "repos/${repository}/git/ref/heads/${base_ref}" --jq '.object.sha'
  )"
  comparison="$(
    gh api "repos/${repository}/compare/${base_sha}...${head_sha}" --jq '.status'
  )"

  case "$comparison" in
    ahead | identical)
      gh pr merge "$pr_number" \
        --repo "$repository" \
        --admin \
        --squash \
        --delete-branch \
        --match-head-commit "$head_sha"
      exit 0
      ;;
    behind | diverged)
      echo "Base branch advanced; updating PR before merge (attempt ${attempt}/${max_update_attempts})"
      if ! update_output="$(
        gh api \
          --method PUT \
          "repos/${repository}/pulls/${pr_number}/update-branch" \
          -f "expected_head_sha=${head_sha}" 2>&1
      )"; then
        if [[ "$update_output" != *"expected head sha didn't match current head ref"* ]]; then
          echo "$update_output" >&2
          exit 1
        fi
        echo "PR head changed while update-branch was being scheduled"
      fi
      wait_for_updated_head "$head_sha"
      ;;
    *)
      echo "::error::Unexpected comparison state '${comparison}' for ${base_sha}...${head_sha}"
      exit 1
      ;;
  esac
done

echo "::error::Base branch kept changing; refusing to merge pull request #${pr_number}"
exit 1
