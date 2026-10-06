#!/usr/bin/env bash
# The scale generator matches the counts the research protocol states.
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

d=$(mktemp -d)
trap 'rm -rf "$d"' EXIT
"$TOOLS_ROOT/test/gen-scale.sh" "$d" >/dev/null

check "10050 .md files under repos/" test "$(find "$d/repos" -name '*.md' | wc -l)" -eq 10050
hits=$(grep -rl zebracorn-needle "$d/repos")
check "zebracorn-needle only in r37/okf/d3/c123.md" test "$hits" = "$d/repos/r37/okf/d3/c123.md"
total=$(find "$d/repos" -name 'c*.md' | wc -l)
cross=$(grep -rlE '\]\(\.\./\.\./r[0-9]{2}/' "$d/repos" | wc -l)
pct=$((cross * 100 / total))
check "cross-repo link share $pct% is within 8-12%" test "$pct" -ge 8 -a "$pct" -le 12

finish
