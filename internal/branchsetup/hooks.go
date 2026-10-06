package branchsetup

// The two hooks `compass branch setup` installs into .git/hooks and prints
// with --print-hook. Both are self-contained sh scripts; installed hooks never
// call compass, so a clone keeps pairing okf/ even without compass on PATH.
// The marker "okf-branch-hook" tells setup that a hook is its own.

const hookPostCheckout = `#!/bin/sh
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
`

const hookReferenceTransaction = `#!/bin/sh
# okf-branch-hook: when code branch <b> is deleted, delete knowledge branch okf/<b>
# unless that would lose work. Local only: the hub's sync job cleans up the remote.
# git runs this on every ref update, so it exits early for anything else.
[ "$1" = committed ] || exit 0
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE
while read -r _ new ref; do
  case "$new" in *[1-9a-f]*) continue;; esac          # deletions only (new id is all zeros)
  case "$ref" in refs/heads/okf/*) continue;; refs/heads/*) ;; *) continue;; esac
  # git gc and git pack-refs report every branch they pack as deleted: skip
  # branches that still exist.
  git show-ref -q --verify "$ref" && continue
  # Deleting a packed branch fires this hook twice, the first time while git
  # holds packed-refs.lock, so git branch -D would fail. Act on the second.
  [ -e "$(git rev-parse --git-common-dir)/packed-refs.lock" ] && continue
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
`
