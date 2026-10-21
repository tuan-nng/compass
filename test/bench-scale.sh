#!/usr/bin/env bash
# Benchmark the pinned okf and `compass hub check` on the 10,000-concept hub
# (research protocol, T10; plan decision 12): median of 5 runs after 1 warm-up
# for validate, a search for zebracorn-needle (exactly one hit), backlinks for
# repos/r37/d3/c123, and hub check, which also runs `okf index` on a copy.
# Then the scale hub becomes a clone with a local bare origin listing one code
# repo, and `compass hub branch` is timed in a clone of that repo: the median
# first run over 5 new branches, and the median reuse (plan Quality targets).
# Targets: validate < 10 s, search < 2 s, backlinks < 2 s, hub check < 10 s,
# hub branch first run < 5 s, hub branch reuse < 1 s.
#
# Usage: bench-scale.sh
set -uo pipefail
export LC_ALL=C
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

O=$(okf_bin) || exit 1
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
H="$work/hub"
"$TOOLS_ROOT/test/gen-scale.sh" "$H" >/dev/null
"$O" index "$H" >/dev/null || { echo "FAIL: okf index on the scale hub"; exit 1; }
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
check "hub check passes on the scale hub" env OKF="$O" "$COMPASS" hub check "$H"

ok=1
report() {
  local name=$1 limit=$2 med=$3
  printf '%-10s median %.3f s (target < %s s)\n' "$name" "$med" "$limit"
  [ "$(echo "$med < $limit" | bc)" -eq 1 ] || ok=0
}
report validate 10 "$(median_of "$O" validate "$H")"
report search 2 "$(median_of "$O" search "$H" --text zebracorn-needle)"
report backlinks 2 "$(median_of "$O" backlinks "$H" repos/r37/d3/c123)"
report 'hub check' 10 "$(median_of env OKF="$O" "$COMPASS" hub check "$H")"

git_isolated "$work/gitconfig"
map_owner scale "$work/remotes"
echo "r00 https://github.com/scale/code" > "$H/repos.txt"
git -C "$H" init -q && git -C "$H" add -A && git -C "$H" commit -qm scale
git clone -q --bare "$H" "$work/remotes/hub.git"
git -C "$H" remote add origin https://github.com/scale/hub.git
git -C "$H" fetch -q origin && git -C "$H" remote set-head origin main
git init -q "$work/code-seed" && echo code > "$work/code-seed/main.go"
git -C "$work/code-seed" add -A && git -C "$work/code-seed" commit -qm code
git clone -q --bare "$work/code-seed" "$work/remotes/code.git"
code=$work/code
git clone -q https://github.com/scale/code.git "$code"
hub_branch() { (cd "$code" && "$COMPASS" hub branch --hub "$H"); }

# first_runs: hub branch on 5 new code branches, each creating its worktree;
# prints the median seconds.
first_runs() {
  local i t0 t1 times=()
  for i in 1 2 3 4 5; do
    git -C "$code" switch -q -c "bench-$i"
    t0=$(date +%s.%N); hub_branch >/dev/null 2>&1; t1=$(date +%s.%N)
    times+=("$(echo "$t1 - $t0" | bc)")
  done
  printf '%s\n' "${times[@]}" | sort -g | sed -n 3p
}
report 'hub branch first run' 5 "$(first_runs)"
check "hub branch opened the worktree of r00/bench-5" test "$(hub_branch)" = "$H.worktrees/r00/bench-5"
report 'hub branch reuse' 1 "$(median_of hub_branch)"
check "all medians within their targets" test "$ok" -eq 1
finish
