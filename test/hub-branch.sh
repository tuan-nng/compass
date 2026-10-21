#!/usr/bin/env bash
# `compass hub branch` maps a code clone to its repos.txt row and prints the
# hub worktree for its branch <b> on hub branch <id>/<b> (plan decision 8 and
# Interfaces); --prune removes finished worktrees. Onboarding goes from one
# added repos.txt line to a pushed hub branch that passes `compass hub check`
# (plan Acceptance 1 and 2). The hub and the code repo are local bare repos
# behind github.com URLs (map_owner), so the suite is offline when okf is
# already cached (okf_bin).
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
okf=$(okf_bin) || { echo "FAIL: no okf binary"; exit 1; }
export OKF=$okf

git_isolated "$work/gitconfig"
R=$work/remotes
map_owner acme "$R"

# The hub: the shipped template plus one added repos.txt row. The code repo
# carries no compass file, branch or hook.
cp -R "$TOOLS_ROOT/templates/hub" "$work/seed"
echo "billing https://github.com/acme/billing-api" >> "$work/seed/repos.txt"
git init -q "$work/seed" && git -C "$work/seed" add -A && git -C "$work/seed" commit -qm hub
git clone -q --bare "$work/seed" "$R/hub.git"
git init -q "$work/code-seed" && echo code > "$work/code-seed/main.go"
git -C "$work/code-seed" add -A && git -C "$work/code-seed" commit -qm code
git clone -q --bare "$work/code-seed" "$R/billing-api.git"

H=$work/hub
git clone -q https://github.com/acme/hub.git "$H"
C=$work/code
git clone -q https://github.com/acme/billing-api.git "$C"
W=$H.worktrees

# hb <args...>: run `compass hub branch --hub $H` in the code clone; stdout
# to $work/out, stderr to $work/err, exit code in $rc.
hb() { (cd "$C" && compass hub branch --hub "$H" "$@") >"$work/out" 2>"$work/err"; rc=$?; }
# refused <name> <pattern>: the last hb exited 1 and its stderr has pattern.
refused() {
  check "$1 exits 1" test "$rc" -eq 1
  check "$1 is named" grep -qF -- "$2" "$work/err"
}
state() { git -C "$H" worktree list --porcelain; git -C "$H" for-each-ref; find "$W" | sort; }

# Onboarding.
git -C "$C" switch -q -c onboard
hb
P=$(cat "$work/out")
check "first run exits 0" test "$rc" -eq 0
check "it prints one absolute path" test "$(wc -l < "$work/out")" -eq 1 -a "${P:0:1}" = /
check "the worktree is $W/billing/onboard" test "$P" = "$W/billing/onboard"
check "the worktree is on billing/onboard" test "$(git -C "$P" branch --show-current)" = billing/onboard
check "it starts at the hub's default head" test "$(git -C "$P" rev-parse HEAD)" = "$(git -C "$H" rev-parse origin/main)"
mkdir -p "$P/repos/billing"
cat > "$P/repos/billing/overview.md" <<'EOF'
---
type: Overview
title: Billing API
description: What the billing API does.
tags: [overview]
status: stable
generated: { by: process:test, at: 2026-10-21T00:00:00Z }
---
# Billing API

Bills customers. See [the hub](/overview.md).
EOF
(cd "$P" && "$okf" index . >/dev/null)
git -C "$P" add -A
changed=$(git -C "$P" diff --cached --name-only | sort | tr '\n' ' ')
check "okf index changes exactly repos/billing/, repos/index.md and index.md" \
  test "$changed" = "index.md repos/billing/index.md repos/billing/overview.md repos/index.md "
check "hub check passes on the worktree" compass hub check "$P"
git -C "$P" commit -qm "billing: overview"
check "a plain git push publishes the branch" git -C "$P" push -q
check "the hub remote has billing/onboard" \
  test "$(git --git-dir "$R/hub.git" rev-parse refs/heads/billing/onboard)" = "$(git -C "$P" rev-parse HEAD)"
check "the hub clone's own checkout is untouched" test ! -e "$H/repos"

# Reuse: the same path, nothing created.
before=$(state)
hb
check "second run exits 0" test "$rc" -eq 0
check "second run prints the same path" test "$(cat "$work/out")" = "$P"
check "second run creates nothing" test "$before" = "$(state)"

# The recorded hub clone is used when --hub is not given.
mkdir -p "$work/cfg/compass" && echo "OKF_HUB_DIR=$H" > "$work/cfg/compass/config"
out=$(cd "$C" && XDG_CONFIG_HOME=$work/cfg compass hub branch 2>/dev/null)
check "with no --hub it uses the recorded hub clone" test "$out" = "$P"

# A remote billing/<b> is tracked, not recreated.
git clone -q https://github.com/acme/hub.git "$work/other"
git -C "$work/other" switch -q -c billing/remote-one
echo x > "$work/other/note.md" && git -C "$work/other" add -A && git -C "$work/other" commit -qm remote
git -C "$work/other" push -q origin billing/remote-one
git -C "$C" switch -q -c remote-one
hb
P2=$(cat "$work/out")
check "a remote hub branch: exit 0" test "$rc" -eq 0
check "a remote hub branch is checked out at its head" \
  test "$(git -C "$P2" rev-parse HEAD)" = "$(git -C "$work/other" rev-parse HEAD)"
check "a remote hub branch is tracked" \
  test "$(git -C "$P2" rev-parse --abbrev-ref '@{upstream}')" = origin/billing/remote-one

# A kept local billing/<b> with no worktree is reused.
git -C "$H" branch -q billing/kept origin/main
git -C "$H" worktree add -q "$work/tmp-wt" billing/kept
echo y > "$work/tmp-wt/kept.md" && git -C "$work/tmp-wt" add -A && git -C "$work/tmp-wt" commit -qm kept
git -C "$H" worktree remove "$work/tmp-wt"
git -C "$C" switch -q -c kept
hb
check "a kept local hub branch: exit 0" test "$rc" -eq 0
check "a kept local hub branch is reused with its commit" \
  test "$(git -C "$(cat "$work/out")" rev-parse HEAD)" = "$(git -C "$H" rev-parse billing/kept)"

# A kept billing/feat blocks billing/feat/x; deleting it unblocks feat/* below.
git -C "$H" branch -q billing/feat origin/main
git -C "$C" switch -q -c feat/x
hb
refused "a kept billing/feat blocking billing/feat/x" "hub ref billing/feat conflicts with hub branch billing/feat/x"
check "the conflict creates no worktree" test ! -e "$W/billing/feat"
git -C "$H" branch -q -D billing/feat

# Names with slashes.
git -C "$C" switch -q -c feat/retry
hb
check "feat/retry: exit 0" test "$rc" -eq 0
check "feat/retry opens $W/billing/feat/retry" test "$(cat "$work/out")" = "$W/billing/feat/retry"
check "feat/retry is on billing/feat/retry" \
  test "$(git -C "$W/billing/feat/retry" branch --show-current)" = billing/feat/retry
check "feat/retry pushes" git -C "$W/billing/feat/retry" push -q

# URL forms: each maps to the same row, so each run reuses the same worktree.
git -C "$C" switch -q onboard
for url in https://github.com/Acme/Billing-API.git ssh://git@github.com/acme/billing-api.git \
  git@github.com:acme/billing-api.git gh:acme/billing-api; do
  git -C "$C" remote set-url origin "$url"
  if [ "$url" = gh:acme/billing-api ]; then git -C "$C" config url.https://github.com/.insteadOf gh:; fi
  hb
  check "origin $url maps to billing" test "$rc" -eq 0 -a "$(cat "$work/out")" = "$P"
done
git -C "$C" config --unset url.https://github.com/.insteadOf
git -C "$C" remote set-url origin https://github.com/acme/billing-api.git

# Refusals.
git -C "$C" remote set-url origin https://x-access-token:s3cr3t-tok@github.com/acme/other.git
hb
refused "an unlisted origin" "origin https://github.com/acme/other.git is not listed in repos.txt"
check "the unlisted origin's token is not printed" bash -c "! grep -q s3cr3t-tok '$work/err'"
git -C "$C" remote set-url origin https://github.com/acme/billing-api.git

git -C "$C" checkout -q --detach
hb
refused "a detached HEAD" "has a detached HEAD"

git -C "$C" switch -q main
hb
refused "the default branch" "main is acme/billing-api's default branch: read its knowledge in the hub clone $H"
check "the default branch refusal says to write through a hub branch" grep -qF "write through an ordinary hub branch" "$work/err"

git -C "$work/other" push -q origin origin/main:refs/heads/billing/wip
git -C "$C" switch -q -c wip/x
hb
refused "a remote billing/wip blocking billing/wip/x" "hub ref origin/billing/wip conflicts with hub branch billing/wip/x"

(cd "$C" && compass hub branch) >"$work/out" 2>"$work/err"; rc=$?
refused "no recorded hub" "no hub folder given, and \`compass setup\` recorded none"

hb extra
check "a positional argument is a usage error" test "$rc" -eq 2
hb --help
check "--help exits 0 and names the worktree folder" test "$rc" -eq 0
check "--help names the worktree folder" grep -qF "<hub-clone>.worktrees/<id>/<b>" "$work/out"

# Default branch fallback: with origin/HEAD unset, ls-remote finds it.
git -C "$C" remote set-head origin -d
git -C "$C" switch -q main
hb
refused "the default branch with origin/HEAD unset" "main is acme/billing-api's default branch"
git -C "$C" switch -q onboard
hb
check "a work branch with origin/HEAD unset: exit 0" test "$rc" -eq 0 -a "$(cat "$work/out")" = "$P"

# Pruning. merged: pushed, merged into the hub's main, branch deleted;
# dirty: branch deleted, an uncommitted file; unpushed: branch deleted, a
# local commit; ignored: never pushed, only a file the hub's info/exclude
# ignores, which `git worktree remove` would delete; fresh: never pushed and
# no work, so nothing is lost; the onboarding and feat/retry worktrees stay
# on the remote.
# open_wt <b>: switch the code clone to new branch <b> and print its worktree.
open_wt() { git -C "$C" switch -q -c "$1" && hb && cat "$work/out"; }
P_merged=$(open_wt merged) P_dirty=$(open_wt dirty) P_unpushed=$(open_wt unpushed) P_fresh=$(open_wt fresh)
P_ignored=$(open_wt ignored)
echo '*.scratch' >> "$H/.git/info/exclude"
echo i > "$P_ignored/notes.scratch"
echo m > "$P_merged/merged.md" && git -C "$P_merged" add -A && git -C "$P_merged" commit -qm merged
git -C "$P_merged" push -q
git -C "$work/other" fetch -q origin
git -C "$work/other" switch -q main
git -C "$work/other" merge -q --no-ff -m merge origin/billing/merged
git -C "$work/other" push -q origin main
git -C "$P_dirty" push -q
git -C "$P_unpushed" push -q
for b in merged dirty unpushed; do git -C "$work/other" push -q origin --delete "billing/$b"; done
echo d > "$P_dirty/dirty.md"
echo u > "$P_unpushed/unpushed.md" && git -C "$P_unpushed" add -A && git -C "$P_unpushed" commit -qm unpushed
mkdir -p "$W/not-a-worktree" && echo z > "$W/not-a-worktree/file"

(cd "$work" && compass hub branch --hub "$H" --prune) >"$work/out" 2>"$work/err"; rc=$?
check "--prune exits 0" test "$rc" -eq 0
check "--prune removes the merged, clean worktree" test ! -e "$P_merged"
check "--prune prints the removed worktree" grep -qF "removed $P_merged:" "$work/out"
check "--prune deletes its local branch" bash -c "! git -C '$H' rev-parse -q --verify refs/heads/billing/merged"
check "--prune keeps the dirty worktree" test -f "$P_dirty/dirty.md"
check "--prune prints the dirty one with its reason" grep -qF "kept $P_dirty: it has uncommitted changes" "$work/out"
check "--prune keeps the unpushed worktree" test -d "$P_unpushed"
check "--prune prints the unpushed one with its reason" \
  grep -qF "kept $P_unpushed: billing/unpushed has 1 commit(s) not on origin/main" "$work/out"
check "--prune keeps the worktree with an ignored file" test -f "$P_ignored/notes.scratch"
check "--prune prints the ignored one with its reason" \
  grep -qF "kept $P_ignored: it has ignored files, which removing it would delete" "$work/out"
check "--prune removes a never-pushed worktree with no work" test ! -e "$P_fresh"
check "--prune keeps a worktree whose branch is on the remote" grep -qF "kept $P: billing/onboard is still on the hub remote" "$work/out"
check "--prune keeps the feat/retry worktree" test -d "$W/billing/feat/retry"
check "--prune leaves a folder that is not a worktree" test -f "$W/not-a-worktree/file"
check "--prune leaves the hub clone" test -f "$H/repos.txt"

finish
