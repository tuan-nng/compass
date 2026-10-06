#!/usr/bin/env bash
# Benchmark the pinned okf on the 10,000-concept hub (research protocol, T10;
# plan decision 12): median of 5 runs after 1 warm-up for validate, a search for
# zebracorn-needle (exactly one hit), and backlinks for repos/r37/d3/c123.
# Targets: validate < 10 s, search < 2 s, backlinks < 2 s.
#
# Usage: bench-scale.sh
set -uo pipefail
export LC_ALL=C
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

O=$(okf_bin) || exit 1
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
"$TOOLS_ROOT/test/gen-scale.sh" "$work/scale" >/dev/null
H="$work/hub"
mkdir -p "$H/repos"
printf -- '---\nokf_version: "0.2"\n---\n# Scale hub\n' > "$H/index.md"
for d in "$work/scale/repos"/r*; do
  cp -R "$d/okf" "$H/repos/$(basename "$d")"
  strip_root_frontmatter "$H/repos/$(basename "$d")/index.md"
done
echo "okf: $("$O" --version | jq -r .version)  hub: $(find "$H" -name '*.md' | wc -l | tr -d ' ') .md files"

# median_of <cmd...>: 1 warm-up, then 5 timed runs; prints the median seconds.
median_of() {
  local _ t0 t1 times=()
  "$@" >/dev/null 2>&1
  for _ in 1 2 3 4 5; do
    t0=$(date +%s.%N); "$@" >/dev/null 2>&1; t1=$(date +%s.%N)
    times+=("$(echo "$t1 - $t0" | bc)")
  done
  printf '%s\n' "${times[@]}" | sort -g | sed -n 3p
}

hits=$("$O" search "$H" --text zebracorn-needle | jq -c '[.results[].id]')
check "search finds exactly one hit: repos/r37/d3/c123" test "$hits" = '["repos/r37/d3/c123"]'

ok=1
report() {
  local name=$1 limit=$2 med=$3
  printf '%-10s median %.3f s (target < %s s)\n' "$name" "$med" "$limit"
  [ "$(echo "$med < $limit" | bc)" -eq 1 ] || ok=0
}
report validate 10 "$(median_of "$O" validate "$H")"
report search 2 "$(median_of "$O" search "$H" --text zebracorn-needle)"
report backlinks 2 "$(median_of "$O" backlinks "$H" repos/r37/d3/c123)"
check "all medians within the decision 12 targets" test "$ok" -eq 1
finish
