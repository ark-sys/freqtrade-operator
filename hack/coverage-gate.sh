#!/usr/bin/env bash
# Coverage gate (P5-4): "start where Phase 5 lands, ratchet up, never let it drop" - not "hit
# some aspirational number today". Run after `make test` (needs cover-unit.out and
# cover-integration.out, which together cover every package test-unit/test-integration split
# across). Excludes zz_generated.*.go from every total: deepcopy methods are mechanical,
# unedited-by-hand output, and including them just dilutes the signal this gate exists to give -
# a repo can be thoroughly tested and still show a mediocre blended percentage purely from
# generated-file bulk.
set -euo pipefail

cd "$(dirname "$0")/.."

# Raise these as coverage actually improves - this is a floor, not a target. Currently (D3,
# re-baselined after A2's dead-code deletion, A4's builder refactor, and D1's new pairlist option
# tests): ~73.5-73.8% overall (a few tenths of a point of natural run-to-run variance observed,
# not a regression), all four resources packages + configbuilder in the 89.8-100% range - see the
# commit that introduced this script for the original (68%/80%) numbers.
OVERALL_MIN=70
PACKAGE_MIN=85
# Every package the plan's own Definition of done names an explicit ≥80% target for.
declare -a GATED_PACKAGES=(
  "controllers/tradebot/resources"
  "controllers/backtest/resources"
  "controllers/frequi/resources"
  "controllers/tradebotconfig/configbuilder"
)

if [[ ! -f cover-unit.out || ! -f cover-integration.out ]]; then
  echo "cover-unit.out / cover-integration.out not found - run 'make test' first" >&2
  exit 1
fi

merged="$(mktemp)"
trap 'rm -f "$merged"' EXIT
{
  head -1 cover-unit.out
  tail -n +2 cover-unit.out
  tail -n +2 cover-integration.out
} | grep -v "zz_generated" >"$merged"
# grep above also ate the header line if it happened to match (it never does, but be exact
# about the file's shape rather than relying on that): re-stitch it back on if missing.
if ! head -1 "$merged" | grep -q "^mode:"; then
  { head -1 cover-unit.out; cat "$merged"; } >"${merged}.fixed"
  mv "${merged}.fixed" "$merged"
fi

total_pct() {
  go tool cover -func="$1" | tail -1 | grep -oE '[0-9]+\.[0-9]+%$' | tr -d '%'
}

failed=0

overall=$(total_pct "$merged")
echo "Overall (excl. generated): ${overall}% (floor: ${OVERALL_MIN}%)"
if (($(echo "$overall < $OVERALL_MIN" | bc -l))); then
  echo "FAIL: overall coverage ${overall}% is below the ${OVERALL_MIN}% floor" >&2
  failed=1
fi

for pkg in "${GATED_PACKAGES[@]}"; do
  pkg_profile="$(mktemp)"
  { head -1 "$merged"; grep "github.com/ark-sys/freqtrade-operator/${pkg}/" "$merged"; } >"$pkg_profile"
  pkg_pct=$(total_pct "$pkg_profile")
  rm -f "$pkg_profile"
  echo "${pkg}: ${pkg_pct}% (floor: ${PACKAGE_MIN}%)"
  if (($(echo "$pkg_pct < $PACKAGE_MIN" | bc -l))); then
    echo "FAIL: ${pkg} coverage ${pkg_pct}% is below the ${PACKAGE_MIN}% floor" >&2
    failed=1
  fi
done

exit "$failed"
