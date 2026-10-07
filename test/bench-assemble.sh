#!/usr/bin/env bash
# Time `compass hub assemble` over N local bare remotes built from the scale set
# (plan decision 12: 50 repos under 300 s from an empty cache, under 60 s when
# nothing changed). Every fifth repo is in branch mode.
#
# Usage: bench-assemble.sh [N]    (default 50)
set -uo pipefail
export LC_ALL=C
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

n=${1:-50}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
git_isolated "$work/gitconfig"
export OKF_HUB_CACHE="$work/cache"
R="$work/remotes"
mkdir -p "$R"
map_owner benchorg "$R"

"$TOOLS_ROOT/test/gen-scale.sh" "$work/scale" "$n" >/dev/null
H="$work/hub"
mkdir -p "$H"
printf -- '---\nokf_version: "0.2"\n---\n# Hub\n' > "$H/index.md"
: > "$H/repos.txt"
for d in "$work/scale/repos"/r*; do
  r=$(basename "$d")
  printf -- '---\ntype: Overview\ntitle: %s\n---\n%s\n' "$r" "$r" > "$d/okf/overview.md"
  if [ $((10#${r#r} % 5)) -eq 4 ]; then
    make_branch_remote "$R/$r.git" "$d/okf"; kind=branch
  else
    make_folder_remote "$R/$r.git" "$d/okf"; kind=folder
  fi
  echo "$r https://github.com/benchorg/$r $kind" >> "$H/repos.txt"
done

t0=$(date +%s.%N)
"$COMPASS" hub assemble --ci "$H" >/dev/null || { echo "bench-assemble: cold run failed" >&2; exit 1; }
t1=$(date +%s.%N)
"$COMPASS" hub assemble --ci "$H" >/dev/null || { echo "bench-assemble: warm run failed" >&2; exit 1; }
t2=$(date +%s.%N)

files=$(find "$H/repos" -name '*.md' | wc -l | tr -d ' ')
cold=$(echo "$t1 - $t0" | bc)
warm=$(echo "$t2 - $t1" | bc)
printf 'repos: %s  files: %s\ncold: %.1f s (target < 300 s)\nwarm: %.1f s (target < 60 s)\n' "$n" "$files" "$cold" "$warm"
ok=1
[ "$(echo "$cold < 300" | bc)" -eq 1 ] || ok=0
[ "$(echo "$warm < 60" | bc)" -eq 1 ] || ok=0
if [ "$ok" -ne 1 ]; then echo "bench-assemble: over target"; exit 1; fi
echo "bench-assemble: within targets"
