#!/usr/bin/env bash
# Sketch: set up branch-mode OKF knowledge in one clone. Run inside the clone.
# usage: okf-branch-setup.sh [--init]
#   --init  create the okf/main branch; only the first person to set up a repo needs it
#
# Result: okf/ is a git worktree holding the knowledge branch that pairs with
# the current code branch (okf/main for the default branch, okf/<b> for <b>).
# Two local hooks keep it paired. Nothing is committed to the code branches.
# Assumes the remote is called origin.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

if [ -n "$(git config core.hooksPath || true)" ]; then
  echo "core.hooksPath is set, so git ignores .git/hooks. Add the two hooks" \
       "from this script to that hook manager by hand." >&2
  exit 1
fi
def=$(git symbolic-ref -q --short refs/remotes/origin/HEAD) ||
  { echo "origin/HEAD is unknown; run: git remote set-head origin --auto" >&2; exit 1; }
git config okf.defaultBranch "${def#origin/}"

# 1. Check out the knowledge trunk at okf/ and hide it from the code branches.
git fetch -q origin 2>/dev/null || echo "Could not fetch origin; using local refs." >&2
git worktree prune                        # forget an okf/ folder deleted by hand
if [ ! -e okf/.git ]; then
  if git show-ref -q --verify refs/heads/okf/main; then
    git worktree add -q okf okf/main
  elif git show-ref -q --verify refs/remotes/origin/okf/main; then
    git worktree add -q --track -b okf/main okf origin/okf/main
  elif [ "${1-}" = --init ]; then
    git worktree add -q --orphan -b okf/main okf
    printf -- '---\nokf_version: "0.2"\n---\n# %s\n' "$(basename "$PWD")" > okf/index.md
    git -C okf add index.md
    git -C okf commit -q -m "okf: create knowledge branch"
    echo "Created okf/main. Publish it with: git -C okf push -u origin okf/main"
  else
    echo "This repo has no okf/main branch yet. Re-run with --init to create it." >&2
    exit 1
  fi
fi
exclude=$(git rev-parse --git-path info/exclude)
mkdir -p "$(dirname "$exclude")"
grep -qx /okf/ "$exclude" 2>/dev/null || echo /okf/ >> "$exclude"   # top level only

# 2. Install the hooks. Refuse to overwrite hooks that are not ours.
hooks=$(git rev-parse --git-common-dir)/hooks
mkdir -p "$hooks"
for h in post-checkout reference-transaction; do
  if [ -e "$hooks/$h" ] && ! grep -q okf-branch-hook "$hooks/$h"; then
    echo "$hooks/$h already exists. Merge the okf hook into it by hand." >&2
    exit 1
  fi
done

cat > "$hooks/post-checkout" <<'EOF'
#!/bin/sh
# okf-branch-hook: switch okf/ to the knowledge branch paired with this code branch.
[ "$3" = 1 ] || exit 0                    # branch checkouts only
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE  # let git -C okf find the okf/ worktree
[ -e okf/.git ] || exit 0                 # no okf/ here (another worktree, or okf/ itself)
b=$(git branch --show-current)
case "$b" in ''|okf/*) exit 0;; esac      # detached HEAD: okf/ stays where it is
if [ "$b" = "$(git config okf.defaultBranch)" ]; then k=okf/main; else k="okf/$b"; fi
if ! git show-ref -q --verify "refs/heads/$k"; then
  if git show-ref -q --verify "refs/remotes/origin/$k"; then
    git branch -q --track "$k" "origin/$k"          # a teammate pushed it
  elif git show-ref -q --verify refs/remotes/origin/okf/main; then
    git branch -q --no-track "$k" origin/okf/main
  else
    git branch -q --no-track "$k" okf/main
  fi
fi
if git -C okf switch -q "$k"; then
  echo "okf/ -> $k"
else
  echo "okf/ stayed on $(git -C okf branch --show-current). Commit or stash its" \
       "changes, then run: git -C okf switch $k" >&2
fi
EOF

cat > "$hooks/reference-transaction" <<'EOF'
#!/bin/sh
# okf-branch-hook: when code branch <b> is deleted, delete knowledge branch okf/<b>
# unless that would lose work. Local only: the hub's sync job cleans up the remote.
# git runs this on every ref update, so it exits early for anything else.
[ "$1" = committed ] || exit 0
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE
while read -r _ new ref; do
  case "$new" in *[1-9a-f]*) continue;; esac          # deletions only (new id is all zeros)
  case "$ref" in refs/heads/okf/*) continue;; refs/heads/*) ;; *) continue;; esac
  b=${ref#refs/heads/}
  [ "$b" = "$(git config okf.defaultBranch)" ] && continue
  k="okf/$b"
  [ "$k" = okf/main ] && continue
  git show-ref -q --verify "refs/heads/$k" || continue
  if [ -n "$(git rev-list -n 1 "refs/heads/$k" --not --remotes)" ]; then
    echo "Kept $k: it has commits that are not on the remote." >&2
    continue
  fi
  top=$(git rev-parse --show-toplevel 2>/dev/null) || top=.
  if [ "$(git -C "$top/okf" branch --show-current 2>/dev/null)" = "$k" ]; then
    if [ -n "$(git -C "$top/okf" status --porcelain)" ]; then
      echo "Kept $k: okf/ has uncommitted changes on it." >&2
      continue
    fi
    git -C "$top/okf" switch -q okf/main ||
      { echo "Kept $k: could not switch okf/ to okf/main." >&2; continue; }
  fi
  git branch -D "$k"
done
EOF
chmod +x "$hooks/post-checkout" "$hooks/reference-transaction"

# 3. Pair okf/ with the branch that is checked out now.
"$hooks/post-checkout" x x 1
