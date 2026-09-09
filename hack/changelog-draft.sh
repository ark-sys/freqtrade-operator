#!/usr/bin/env bash
# Drafts a starting point for CHANGELOG.md's [Unreleased] section from commits
# since the last tag (or, before any tag exists, full history), bucketed by a
# best-effort keyword match on each commit's leading verb. This repo has no
# conventional-commit prefixes to key off, so the categorization here is a
# rough first pass to edit by hand into CHANGELOG.md, not a final result -
# read what actually changed before trusting any of it.
set -euo pipefail

range=""
if last_tag=$(git describe --tags --abbrev=0 2>/dev/null); then
  range="${last_tag}..HEAD"
  echo "# Draft: commits since ${last_tag}" >&2
else
  echo "# Draft: no tags yet, using full history" >&2
fi

declare -A buckets=(
  [Added]="add|create|introduce|generate|extract|roll"
  [Security]="harden|lock"
  [Removed]="remove|delete|drop"
  [Fixed]="fix|resolve|correct|contain|guard|stop"
)

print_bucket() {
  local name="$1" pattern="$2" tmp
  tmp=$(git log --reverse --pretty=format:'%s' ${range} | grep -Ei "^(${pattern})\b" || true)
  if [ -n "$tmp" ]; then
    echo "### ${name}"
    echo "$tmp" | sed 's/^/- /'
    echo
  fi
}

matched_pattern=""
for name in Added Security Removed Fixed; do
  matched_pattern+="${matched_pattern:+|}${buckets[$name]}"
done

echo "## [Unreleased]"
echo
print_bucket "Added" "${buckets[Added]}"
print_bucket "Security" "${buckets[Security]}"
print_bucket "Removed" "${buckets[Removed]}"
print_bucket "Fixed" "${buckets[Fixed]}"

changed=$(git log --reverse --pretty=format:'%s' ${range} | grep -Eiv "^(${matched_pattern})\b" || true)
if [ -n "$changed" ]; then
  echo "### Changed (unmatched - recheck each of these)"
  echo "$changed" | sed 's/^/- /'
fi
