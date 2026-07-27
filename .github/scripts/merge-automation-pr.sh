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
    local payload status conclusion details_url
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

  wait_for_check "$head_sha"

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
      gh api \
        --method PUT \
        "repos/${repository}/pulls/${pr_number}/update-branch" \
        -f "expected_head_sha=${head_sha}" >/dev/null
      ;;
    *)
      echo "::error::Unexpected comparison state '${comparison}' for ${base_sha}...${head_sha}"
      exit 1
      ;;
  esac
done

echo "::error::Base branch kept changing; refusing to merge pull request #${pr_number}"
exit 1
