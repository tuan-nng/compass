#!/usr/bin/env bash
# `compass hub assemble` and `compass hub check` against local bare remotes built
# from testdata/proto: billing-api (folder), web-app (branch), shared-auth
# (folder). https://github.com/testorg/<x> is rewritten to the local remotes.
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

OKF=$(okf_bin) || exit 1
export OKF
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
git_isolated "$work/gitconfig"
export OKF_HUB_CACHE="$work/cache"
R="$work/remotes"
mkdir -p "$R"
map_owner testorg "$R"
map_owner otherorg "$R"

P="$TESTDATA/proto"
make_folder_remote "$R/billing-api.git" "$P/billing-api/okf"
make_branch_remote "$R/web-app.git" "$P/web-app/okf"
make_folder_remote "$R/shared-auth.git" "$P/shared-auth/okf"

new_hub() {
  rm -rf "$1" && cp -R "$P/hub" "$1"
  cat > "$1/repos.txt" <<'EOF'
# name url mode
billing-api https://github.com/testorg/billing-api.git folder
web-app     https://github.com/testorg/web-app       branch
shared-auth https://github.com/testorg/shared-auth   folder
EOF
}
H="$work/hub"
new_hub "$H"

echo "== assemble from remotes"
check "assembles 3 repos in CI mode" "$COMPASS" hub assemble --ci "$H"
check "folder-mode repo copied from okf/ only" bash -c "test -f '$H/repos/billing-api/overview.md' && test ! -e '$H/repos/billing-api/src'"
check "branch-mode repo copied from okf/main without .github" bash -c "test -f '$H/repos/web-app/overview.md' && test ! -e '$H/repos/web-app/.github' && test ! -e '$H/repos/web-app/src'"
check "copied repo index.md has no frontmatter" bash -c "[ \"\$(head -1 '$H/repos/billing-api/index.md')\" != '---' ]"
check "hub root index.md keeps okf_version" grep -q '^okf_version:' "$H/index.md"
bl=$("$OKF" backlinks "$H" repos/billing-api/contracts/invoice-api | jq -r '.backlinks[]')
check "backlinks list cross-repo/invoice-dependency" grep -qx cross-repo/invoice-dependency <<<"$bl"
check "hub check passes (strict validator + counts)" "$COMPASS" hub check "$H"

echo "== updates and warm runs"
w="$work/push" && git clone -q "$R/billing-api.git" "$w"
cp "$P/billing-api/okf/gotchas/idempotency-key.md" "$w/okf/gotchas/new-fact.md"
sed -i 's/^title: .*/title: New fact/' "$w/okf/gotchas/new-fact.md"
git -C "$w" add -A && git -C "$w" commit -qm "okf: new fact" && git -C "$w" push -q origin HEAD
check "warm run picks up a new commit on the remote" "$COMPASS" hub assemble --ci "$H"
check "new concept present after warm run" test -f "$H/repos/billing-api/gotchas/new-fact.md"
git -C "$w" rm -q okf/gotchas/new-fact.md && git -C "$w" commit -qm "okf: drop" && git -C "$w" push -q origin HEAD
"$COMPASS" hub assemble --ci "$H" >/dev/null 2>&1
check "deleted concept gone after warm run" test ! -e "$H/repos/billing-api/gotchas/new-fact.md"

echo "== failures"
B="$work/hub-broken"
new_hub "$B"
cat > "$B/cross-repo/broken-link.md" <<'EOF'
---
type: Cross-Repo Dependency
title: Broken link
description: Links a concept that no repo has.
tags: [test]
---
Depends on [a missing contract](/repos/billing-api/contracts/missing.md).
EOF
"$COMPASS" hub assemble --ci "$B" >/dev/null 2>&1
check "hub concept with a broken repo link fails the hub check" bash -c "! '$COMPASS' hub check '$B'"

E="$work/hub-empty"
new_hub "$E"
mkdir -p "$work/empty-bundle/notes" && printf -- '---\ntype: Note\ntitle: x\n---\nx\n' > "$work/empty-bundle/notes/x.md"
make_folder_remote "$R/empty-repo.git" "$work/empty-bundle"
echo "empty-repo https://github.com/testorg/empty-repo folder" >> "$E/repos.txt"
out=$("$COMPASS" hub assemble --ci "$E" 2>&1); rc=$?
check "empty bundle exits non-zero" test "$rc" -ne 0
check "empty bundle names the repo" grep -q 'empty-repo: bundle has no index.md or overview.md' <<<"$out"

S="$work/hub-symlink"
new_hub "$S"
cp -R "$P/shared-auth/okf" "$work/sym-bundle"
ln -s ../concepts/session-token.md "$work/sym-bundle/concepts/alias.md"
make_folder_remote "$R/sym-repo.git" "$work/sym-bundle"
echo "sym-repo https://github.com/testorg/sym-repo folder" >> "$S/repos.txt"
out=$("$COMPASS" hub assemble --ci "$S" 2>&1); rc=$?
check "symlink in a bundle exits non-zero" test "$rc" -ne 0
check "symlink names the repo" grep -q 'sym-repo: bundle contains symlinks' <<<"$out"

U="$work/hub-unreachable"
new_hub "$U"
echo "gone-repo https://github.com/testorg/gone-repo folder" >> "$U/repos.txt"
out=$("$COMPASS" hub assemble --ci "$U" 2>&1); rc=$?
check "unreachable remote exits non-zero in CI" test "$rc" -ne 0
check "unreachable remote names the repo" grep -q 'gone-repo: cannot fetch' <<<"$out"
check "other repos still assembled when one fails" test -f "$U/repos/billing-api/overview.md"

for url in "http://github.com/testorg/billing-api" "file://$R/billing-api" "https://github.com/testorg/../x" "git@github.com:testorg/billing-api.git"; do
  X="$work/hub-url"
  new_hub "$X"
  echo "evil $url folder" >> "$X/repos.txt"
  rm -rf "$work/cache"
  out=$("$COMPASS" hub assemble --ci "$X" 2>&1); rc=$?
  check "rejects $url" bash -c "[ $rc -ne 0 ] && grep -q 'evil: URL' <<<'$out'"
  check "nothing fetched for $url" bash -c "[ ! -d '$X/repos' ] && [ ! -d '$work/cache' ]"
done

O="$work/hub-owner"
new_hub "$O"
echo "other-owner https://github.com/otherorg/billing-api folder" >> "$O/repos.txt"
check "assembles a repo from another GitHub owner" "$COMPASS" hub assemble --ci "$O"
check "another owner's repo is copied" test -f "$O/repos/other-owner/overview.md"

echo "== local mode"
L="$work/hub-local"
new_hub "$L"
"$COMPASS" hub assemble --local "$L" >/dev/null 2>&1
mv "$R/shared-auth.git" "$R/shared-auth.away"
rm -rf "$work/cache/shared-auth-folder"
out=$("$COMPASS" hub assemble --local "$L" 2>&1); rc=$?
check "local: unreachable remote exits 0" test "$rc" -eq 0
check "local: unreachable remote prints a warning" grep -q 'warning: shared-auth: cannot fetch' <<<"$out"
check "local: previous copy kept" test -f "$L/repos/shared-auth/concepts/session-token.md"
mv "$R/shared-auth.away" "$R/shared-auth.git"

echo "== repos.txt changes"
sed -i '/^shared-auth/d' "$L/repos.txt"
"$COMPASS" hub assemble --local "$L" >/dev/null 2>&1
check "a repo removed from repos.txt is removed from the view" test ! -e "$L/repos/shared-auth"

finish
