#!/usr/bin/env bash
# Throwaway prototype: assemble a hub view from sibling repo checkouts.
# usage: assemble.sh <workspace-dir-with-repos> <hub-dir> <repo>...
set -euo pipefail
ws=$1 hub=$2; shift 2
mkdir -p "$hub/repos"
for r in "$@"; do
  dst="$hub/repos/$r"
  rsync -a --delete "$ws/$r/okf/" "$dst/"
  # Spec §8/§12: only the bundle-root index.md may carry frontmatter.
  if [ -f "$dst/index.md" ] && [ "$(head -1 "$dst/index.md")" = "---" ]; then
    awk 'NR==1 && /^---$/ {f=1; next} f && /^---$/ {f=0; next} !f' "$dst/index.md" > "$dst/index.md.tmp"
    mv "$dst/index.md.tmp" "$dst/index.md"
  fi
done
