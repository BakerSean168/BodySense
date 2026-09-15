#!/usr/bin/env bash
set -Eeuo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: lint-commit-range.sh <base-sha> <head-sha>" >&2
  exit 2
fi

base_sha="$1"
head_sha="$2"

git cat-file -e "${base_sha}^{commit}"
git cat-file -e "${head_sha}^{commit}"

merge_typed=0
while IFS= read -r commit_sha; do
  [ -n "$commit_sha" ] || continue

  subject="$(git log -1 --format=%s "$commit_sha")"
  case "$subject" in
    merge:*)
      read -r -a commit_line <<<"$(git rev-list --parents -n 1 "$commit_sha")"
      parent_count=$(( ${#commit_line[@]} - 1 ))
      if [ "$parent_count" -le 1 ]; then
        echo "commit $commit_sha uses merge: type but is not a Git merge commit" >&2
        exit 1
      fi
      merge_typed=$((merge_typed + 1))
      ;;
  esac
done < <(git rev-list --reverse "${base_sha}..${head_sha}")

pnpm exec commitlint --from "$base_sha" --to "$head_sha"
echo "COMMITLINT_RANGE=PASS merge_typed=$merge_typed"
