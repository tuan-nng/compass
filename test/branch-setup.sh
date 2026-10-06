#!/usr/bin/env bash
# Rebuilds every case of the branch-mode evidence (compass:
# docs/research/okf-knowledge-system/evidence/branch-mode.md) against
# compass branch setup, plus the command's refusals and the husky snippets.
# Uses local bare repos as origin in a mktemp -d folder; no network.
# Prints "pass: <case>" or "FAIL: <case>" per case; exits 1 if any case fails.
# Reads bundles with the pinned okf (okf_bin in lib.sh) and skips those reads
# when it cannot be installed. OKF_TEST_KEEP=1 keeps the scratch folder.
set -uo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=lib.sh
. "$root/test/lib.sh"   # builds or reuses $COMPASS
okf_bin=$(okf_bin) || okf_bin=
scratch=$(mktemp -d)
if [ -n "${OKF_TEST_KEEP-}" ]; then
  echo "scratch: $scratch"
else
  trap 'rm -rf "$scratch"' EXIT
fi
[ -n "$okf_bin" ] || echo "note: no pinned okf, so the okf reads are skipped"

# Keep the user's git configuration and hooks out of the test.
export HOME=$scratch/home GIT_CONFIG_GLOBAL=$scratch/home/.gitconfig GIT_CONFIG_NOSYSTEM=1
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR OKF_GIT_VERSION
mkdir -p "$HOME" "$scratch/bin"
ln -s "$COMPASS" "$scratch/bin/compass"
export PATH=$scratch/bin:$PATH
git config --global user.name okf-test
git config --global user.email okf-test@example.invalid
git config --global init.defaultBranch main
git config --global pull.rebase false
cd "$scratch" || exit 1

# ---- assertions (each case runs in a subshell with set -e) ----
fail() { echo "  $*" >&2; exit 1; }
has() { case $2 in *"$1"*) ;; *) fail "expected \"$1\" in output:"$'\n'"$2" ;; esac; }
lacks() { case $2 in *"$1"*) fail "did not expect \"$1\" in output:"$'\n'"$2" ;; esac; }
same() { [ "$1" = "$2" ] || fail "$3: expected \"$2\", got \"$1\""; }
# ok CMD...: run it, fail the case if it fails, print its stdout and stderr.
ok() { local o; o=$("$@" 2>&1) || fail "\"$*\" failed: $o"; printf '%s\n' "$o"; }
# refuses TEXT CMD...: the command must exit non-zero and print TEXT.
refuses() {
  local want=$1 o rc=0; shift
  o=$("$@" 2>&1) || rc=$?
  [ "$rc" -ne 0 ] || fail "\"$*\" should have failed; it printed: $o"
  has "$want" "$o"
}
branches() { git branch --format='%(refname:short)' | paste -sd' ' -; }
okf_branch() { git -C okf branch --show-current; }
# names DIR: the names in DIR, sorted, on one line (like ls).
names() { local f out=; for f in "$1"/*; do out="$out${out:+ }${f##*/}"; done; echo "$out"; }
# nothing_set_up: a refused run left no okf/, no hooks and no config behind.
nothing_set_up() {
  [ ! -e okf ] || fail "okf/ was created"
  local hooks; hooks=$(git rev-parse --git-common-dir)/hooks
  ! grep -qs okf-branch-hook "$hooks/post-checkout" "$hooks/reference-transaction" ||
    fail "hooks were installed"
  [ -z "$(git config okf.defaultBranch || true)" ] || fail "okf.defaultBranch was set"
  ! git show-ref -q --verify refs/heads/okf/main || fail "okf/main was created"
}
# seed_origin NAME BRANCH: a bare repo whose BRANCH holds main.go and cmd/okf/okf.go.
seed_origin() {
  git init -q --bare -b "$2" "$1.git"
  git init -q -b "$2" "$1.seed"
  mkdir -p "$1.seed/cmd/okf"
  echo 'package main' > "$1.seed/main.go"
  echo 'package okf' > "$1.seed/cmd/okf/okf.go"
  git -C "$1.seed" add -A
  git -C "$1.seed" commit -qm seed
  git -C "$1.seed" push -q "$scratch/$1.git" "$2"
}
clone() { git clone -q "$scratch/origin.git" "$scratch/$1"; }

# run_case NAME FUNCTION: set -e is ignored in a tested subshell, so run it
# first and read $? afterwards.
failed=0
run_case() {
  local name=$1 log=$scratch/case.log; shift
  ( set -e; "$@" ) >"$log" 2>&1
  # shellcheck disable=SC2181
  if [ $? -eq 0 ]; then
    echo "pass: $name"
  else
    echo "FAIL: $name"
    sed 's/^/    /' "$log"
    failed=$((failed + 1))
  fi
}

# The world: origin with main; carol clones before okf/main exists.
( set -e; seed_origin origin main; clone carol ) >"$scratch/case.log" 2>&1
# shellcheck disable=SC2181
if [ $? -ne 0 ]; then echo "FAIL: scratch setup"; cat "$scratch/case.log"; exit 1; fi

# ---- evidence cases ----
case_setup_init() {   # 1 (alice)
  clone alice; cd alice
  refuses "This repo has no okf/main branch yet. Re-run with --init to create it." compass branch setup
  [ ! -e okf ] || fail "the refused run created okf/"
  local o; o=$(ok compass branch setup --init)
  has "Created okf/main. Publish it with: git -C okf push -u origin okf/main" "$o"
  has "okf/ -> okf/main" "$o"
  ok git -C okf push -q -u origin okf/main >/dev/null
  same "$(grep okf "$(git rev-parse --git-path info/exclude)")" /okf/ "info/exclude"
  same "$(git status --porcelain)" "" "git status"
  same "$(git ls-tree -r --name-only main | paste -sd' ' -)" "cmd/okf/okf.go main.go" "main tree"
  same "$(git -C okf ls-tree -r --name-only HEAD)" index.md "okf/main tree"
  if [ -n "$okf_bin" ]; then
    o=$("$okf_bin" validate okf 2>&1) || true
    has '"valid": true' "$o"; has '"errors": 0' "$o"
  fi
}

case_code_folder() {   # 2
  cd alice
  echo 'package okf // new' > cmd/okf/new.go
  local s; s=$(git status --porcelain); rm cmd/okf/new.go
  same "$s" "?? cmd/okf/new.go" "git status"
}

case_follow() {   # 3
  cd alice
  has "okf/ -> okf/feat/retry" "$(ok git switch -q -c feat/retry)"
  printf -- '---\ntype: Gotcha\ntitle: Retry budget\n---\nThree retries.\n' > okf/retry-budget.md
  git -C okf add -A; git -C okf commit -qm 'okf: retry budget'
  ok git push -q -u origin feat/retry >/dev/null
  ok git -C okf push -q -u origin okf/feat/retry >/dev/null
  if [ -n "$okf_bin" ]; then
    same "$("$okf_bin" list okf | grep -c '"id"')" 1 "okf list concepts"
  fi
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  same "$(names okf)" index.md "ls okf"
  same "$(git status --porcelain)" "" "git status"
  # A switch inside okf/ runs the shared hooks too; they must not pair okf/ itself.
  git -C okf switch -q okf/feat/retry; git -C okf switch -q okf/main
  same "$(git branch --list 'okf/okf/*')" "" "okf/okf/* branches"
}

case_teammate() {   # 4 (bob)
  clone bob; cd bob
  has "okf/ -> okf/main" "$(ok compass branch setup)"
  has "okf/ -> okf/feat/retry" "$(ok git switch -q feat/retry)"
  same "$(names okf)" "index.md retry-budget.md" "ls okf"
  same "$(git -C okf rev-parse --abbrev-ref '@{upstream}')" origin/okf/feat/retry "upstream"
}

case_delete_pushed() {   # 5
  cd bob
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  local o; o=$(ok git branch -D feat/retry)
  has "Deleted branch okf/feat/retry" "$o"; has "Deleted branch feat/retry" "$o"
  same "$(branches)" "main okf/main" "local branches"
  same "$(git ls-remote --heads origin | sed 's/.*refs.heads.//' | paste -sd' ' -)" \
       "feat/retry main okf/feat/retry okf/main" "remote branches"
}

case_keep_unpushed() {   # 6
  cd bob
  has "okf/ -> okf/feat/x" "$(ok git switch -q -c feat/x)"
  echo one > okf/one.md; git -C okf add -A; git -C okf commit -qm one
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  local o; o=$(ok git branch -D feat/x)
  has "Kept okf/feat/x: it has commits that are not on the remote." "$o"
  has "Deleted branch feat/x" "$o"
  same "$(branches)" "main okf/feat/x okf/main" "local branches"
  ok git branch -D okf/feat/x >/dev/null
}

case_keep_uncommitted() {   # 7
  cd bob
  git switch -q -c feat/y; git switch -q main
  git -C okf switch -q okf/feat/y; echo draft > okf/draft.md
  local o; o=$(ok git branch -D feat/y)
  has "Kept okf/feat/y: okf/ has uncommitted changes on it." "$o"
  has "Deleted branch feat/y" "$o"
  same "$(okf_branch)" okf/feat/y "okf/ branch"
  same "$(git branch --list okf/feat/y | tr -d ' *+')" okf/feat/y "kept branch"
  same "$(cat okf/draft.md)" draft "draft"
  rm okf/draft.md; git -C okf switch -q okf/main; ok git branch -D okf/feat/y >/dev/null
}

case_carry_edits() {   # 8
  cd bob
  has "okf/ -> okf/feat/z" "$(ok git switch -q -c feat/z)"
  echo draft > okf/draft.md
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  local s; s=$(git -C okf status --short --branch)
  has "## okf/main...origin/okf/main" "$s"; has "?? draft.md" "$s"
  rm okf/draft.md
  local o; o=$(ok git branch -D feat/z)
  has "Deleted branch okf/feat/z" "$o"; has "Deleted branch feat/z" "$o"
}

case_rename() {   # 9 and 9b
  cd bob
  has "okf/ -> okf/feat/old" "$(ok git switch -q -c feat/old)"
  ok git push -q -u origin feat/old >/dev/null
  ok git -C okf push -q -u origin okf/feat/old >/dev/null
  has "Deleted branch okf/feat/old" "$(ok git branch -m feat/old feat/new)"
  same "$(git branch --show-current)" feat/new "code branch"
  same "$(okf_branch)" okf/main "okf/ branch after rename"
  git switch -q main
  has "okf/ -> okf/feat/new" "$(ok git switch -q feat/new)"
  # 9b: rename the knowledge branch first and the pairing survives.
  git branch -m okf/feat/new okf/feat/newer; git branch -m feat/new feat/newer
  same "$(git branch --show-current)" feat/newer "code branch"
  same "$(okf_branch)" okf/feat/newer "okf/ branch"
}

case_detached() {   # 10
  cd bob
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  git switch -q --detach HEAD
  same "$(okf_branch)" okf/main "okf/ branch on detached HEAD"
  git switch -q feat/newer; git switch -q --detach HEAD
  same "$(okf_branch)" okf/feat/newer "okf/ branch on detached HEAD"
  has "okf/ -> okf/main" "$(ok git switch -q main)"
}

case_second_worktree() {   # 11
  cd bob
  git worktree add -q ../bob-wt -b feat/wt
  same "$(names ../bob-wt)" "cmd main.go" "ls ../bob-wt"
  git -C ../bob-wt switch -q -c feat/wt2
  same "$(git branch --list 'okf/feat/wt*')" "" "knowledge branches for the worktree"
  same "$(okf_branch)" okf/main "okf/ branch"
}

case_init_older_clone() {   # 12 (carol)
  cd carol
  local o; o=$(ok compass branch setup --init)
  has "okf/ -> okf/main" "$o"; lacks "Created okf/main" "$o"
  same "$(git -C okf log --oneline | wc -l | tr -d ' ')" 1 "okf/main commits"
  same "$(git -C okf rev-parse --abbrev-ref '@{upstream}')" origin/okf/main "upstream"
}

case_master() {   # 13 (m)
  seed_origin origin2 master
  git clone -q "$scratch/origin2.git" m; cd m
  local o; o=$(ok compass branch setup --init)
  has "okf/ -> okf/main" "$o"
  same "$(git config okf.defaultBranch)" master "okf.defaultBranch"
  ok git -C okf push -q -u origin okf/main >/dev/null
  has "okf/ -> okf/feat/m" "$(ok git switch -q -c feat/m)"
  has "okf/ -> okf/main" "$(ok git switch -q master)"
  same "$(branches)" "feat/m master okf/feat/m okf/main" "local branches"
  o=$(ok git branch -D feat/m)
  has "Deleted branch okf/feat/m" "$o"; has "Deleted branch feat/m" "$o"
  same "$(branches)" "master okf/main" "local branches"
}

case_hooks_path() {   # 14 (erin)
  clone erin; cd erin
  git config core.hooksPath .husky
  refuses "core.hooksPath is set, so git ignores .git/hooks. Add the two hooks" compass branch setup
  nothing_set_up
}

case_gc() {   # 15
  cd alice
  has "okf/ -> okf/feat/keep" "$(ok git switch -q -c feat/keep)"
  echo k > okf/k.md; git -C okf add -A; git -C okf commit -qm k
  ok git push -q -u origin feat/keep >/dev/null; ok git -C okf push -q -u origin okf/feat/keep >/dev/null
  has "okf/ -> okf/feat/g" "$(ok git switch -q -c feat/g)"
  echo g > okf/g.md; git -C okf add -A; git -C okf commit -qm g
  ok git push -q -u origin feat/g >/dev/null; ok git -C okf push -q -u origin okf/feat/g >/dev/null
  ok git gc -q >/dev/null; ok git pack-refs --all >/dev/null
  same "$(okf_branch)" okf/feat/g "okf/ branch after gc"
  same "$(git branch --list '*feat/g' '*feat/keep' | tr -d ' *+' | paste -sd' ' -)" \
       "feat/g feat/keep okf/feat/g okf/feat/keep" "branches after gc"
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  local o; o=$(ok git branch -D feat/g)
  has "Deleted branch okf/feat/g" "$o"; has "Deleted branch feat/g" "$o"
  same "$(git branch --list '*feat/g' | wc -l | tr -d ' ')" 0 "feat/g branches left"
  same "$(git branch --list okf/feat/keep | tr -d ' *+')" okf/feat/keep "okf/feat/keep"
}

case_pull() {   # 16
  cd alice
  has "okf/ -> okf/feat/auth" "$(ok git switch -q -c feat/auth)"
  printf -- '---\ntype: Gotcha\ntitle: Session auth\n---\nx\n' > okf/auth.md
  git -C okf add -A; git -C okf commit -qm 'okf: session auth'
  ok git push -q -u origin feat/auth >/dev/null; ok git -C okf push -q -u origin okf/feat/auth >/dev/null
  # dave stands in for the sync job: merge the knowledge into okf/main on origin.
  clone dave
  git -C ../dave switch -q okf/main
  git -C ../dave merge -q --no-ff -m 'Merge okf/feat/auth' origin/okf/feat/auth
  ok git -C ../dave push -q origin okf/main >/dev/null
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  ok git pull -q >/dev/null
  same "$(names okf)" index.md "ls okf after git pull"
  has "## okf/main...origin/okf/main [behind 2]" "$(git -C okf status --short --branch)"
  ok git -C okf pull -q --ff-only >/dev/null
  same "$(names okf)" "auth.md index.md" "ls okf after git -C okf pull"
}

# ---- the script's other promises ----
case_rerun() {
  cd alice
  local hooks before o
  hooks=$(git rev-parse --git-common-dir)/hooks
  before=$(cat "$hooks/post-checkout" "$hooks/reference-transaction" | cksum)
  for _ in 1 2; do
    o=$(ok compass branch setup); has "okf/ -> okf/main" "$o"
  done
  o=$(ok compass branch setup --init); lacks "Created" "$o"
  same "$(grep -c '^/okf/$' "$(git rev-parse --git-path info/exclude)")" 1 "/okf/ lines in exclude"
  same "$(cat "$hooks/post-checkout" "$hooks/reference-transaction" | cksum)" "$before" "hooks"
  same "$(git worktree list | wc -l | tr -d ' ')" 2 "worktrees"
  same "$(git status --porcelain)" "" "git status"
  # Re-run on a feature branch with a knowledge draft: both stay.
  git switch -q -c feat/i; echo d > okf/d.md
  has "okf/ -> okf/feat/i" "$(ok compass branch setup)"
  same "$(cat okf/d.md)" d "draft"
  rm okf/d.md; git switch -q main; ok git branch -D feat/i >/dev/null
  # Re-run after okf/ was deleted by hand.
  rm -rf okf
  has "okf/ -> okf/main" "$(ok compass branch setup)"
  same "$(names okf)" "auth.md index.md" "ls okf"
}

case_print_hook() {
  cd alice
  local hooks; hooks=$(git rev-parse --git-common-dir)/hooks
  same "$(compass branch setup --print-hook post-checkout)" "$(cat "$hooks/post-checkout")" "post-checkout"
  same "$(compass branch setup --print-hook reference-transaction)" \
       "$(cat "$hooks/reference-transaction")" "reference-transaction"
}

case_husky() {
  clone fred; cd fred
  # husky v9 layout: core.hooksPath=.husky/_, whose wrappers run .husky/<hook>
  # with sh -e. This wrapper is husky's runner trimmed to what the hooks need.
  mkdir -p .husky/_
  cat > .husky/_/h <<'EOF'
#!/usr/bin/env sh
n=$(basename "$0")
s=$(dirname "$(dirname "$0")")/$n
[ ! -f "$s" ] && exit 0
sh -e "$s" "$@"
EOF
  # shellcheck disable=SC2016 # the wrapper expands $0 when git runs it
  printf '#!/usr/bin/env sh\n. "$(dirname "$0")/h"\n' > .husky/_/post-checkout
  chmod +x .husky/_/h .husky/_/post-checkout
  git config core.hooksPath .husky/_
  cp "$root/templates/hooks/husky/post-checkout" .husky/post-checkout
  cp "$root/templates/hooks/husky/reference-transaction" .husky/_/reference-transaction
  chmod +x .husky/_/reference-transaction
  has "okf/ -> okf/main" "$(ok compass branch setup --no-hooks)"
  [ ! -e .git/hooks/post-checkout ] || fail "--no-hooks installed .git/hooks/post-checkout"
  has "okf/ -> okf/feat/h" "$(ok git switch -q -c feat/h)"
  echo h > okf/h.md; git -C okf add -A; git -C okf commit -qm h
  ok git push -q -u origin feat/h >/dev/null; ok git -C okf push -q -u origin okf/feat/h >/dev/null
  has "okf/ -> okf/main" "$(ok git switch -q main)"
  local o; o=$(ok git branch -D feat/h)
  has "Deleted branch okf/feat/h" "$o"; has "Deleted branch feat/h" "$o"
}

# ---- refusals ----
case_unknown_head() {
  clone olga; cd olga
  git remote set-head origin -d
  refuses "origin/HEAD is unknown; run: git remote set-head origin --auto" compass branch setup
  nothing_set_up
}

case_foreign_hook() {
  clone paul; cd paul
  printf '#!/bin/sh\necho other tool\n' > .git/hooks/post-checkout
  refuses ".git/hooks/post-checkout already exists. Merge the okf hook into it by hand" compass branch setup
  same "$(cat .git/hooks/post-checkout)" "$(printf '#!/bin/sh\necho other tool')" "foreign hook"
  nothing_set_up
}

case_old_git_init() {
  seed_origin origin3 main
  git clone -q "$scratch/origin3.git" gina; cd gina
  refuses "--init needs git 2.42 or newer" \
    env OKF_GIT_VERSION="git version 2.39.5 (Apple Git-154)" compass branch setup --init
  [ ! -e okf ] || fail "okf/ was created"
  ! git show-ref -q --verify refs/heads/okf/main || fail "okf/main was created"
  has "Created okf/main" "$(ok env OKF_GIT_VERSION="git version 2.42.0" compass branch setup --init)"
  # An old git can still join a repo whose okf/main exists, even with --init.
  clone hank; cd ../hank
  has "okf/ -> okf/main" "$(ok env OKF_GIT_VERSION="git version 2.41.0" compass branch setup --init)"
}

case_old_git_hooks() {
  clone ivan; cd ivan
  refuses "needs git 2.28 or newer" env OKF_GIT_VERSION="git version 2.27.0" compass branch setup
  refuses "needs git 2.28 or newer" env OKF_GIT_VERSION="git version 2.27.0" compass branch setup --no-hooks
  nothing_set_up
  has "okf/ -> okf/main" "$(ok env OKF_GIT_VERSION="git version 2.28.0" compass branch setup)"
}

case_no_origin() {
  git clone -q -o upstream "$scratch/origin.git" judy; cd judy
  refuses "This clone has no remote named origin." compass branch setup
  nothing_set_up
}

case_foreign_okf() {
  clone kate; cd kate
  local want="okf already exists and is not this clone's okf worktree."
  mkdir okf; echo x > okf/notes.md
  refuses "$want" compass branch setup; same "$(cat okf/notes.md)" x "okf/notes.md"; rm -r okf
  echo x > okf
  refuses "$want" compass branch setup; same "$(cat okf)" x "okf file"; rm okf
  ln -s ../alice/okf okf
  refuses "$want" compass branch setup; [ -L okf ] || fail "symlink removed"; rm okf
  git clone -q "$scratch/origin.git" okf
  refuses "$want" compass branch setup; rm -rf okf
  git worktree add -q okf -b feat/code-at-okf
  refuses "$want" compass branch setup
  same "$(git -C okf branch --show-current)" feat/code-at-okf "code worktree branch"
  git worktree remove okf
  nothing_set_up
  # A folder-mode repo: okf/ is committed to the code branch.
  mkdir okf; echo '# bundle' > okf/index.md; git add okf; git commit -qm 'okf: folder mode'
  refuses "$want" compass branch setup
  rm -r okf
  refuses "okf is tracked on this code branch" compass branch setup
  nothing_set_up
}

run_case "setup and --init" case_setup_init
run_case "a code folder named okf stays visible" case_code_folder
run_case "okf/ follows a code branch switch" case_follow
run_case "a teammate's pushed branch" case_teammate
run_case "deleting a branch deletes its pushed knowledge branch" case_delete_pushed
run_case "deleting a branch keeps unpushed knowledge commits" case_keep_unpushed
run_case "deleting a branch keeps uncommitted changes in okf/" case_keep_uncommitted
run_case "uncommitted knowledge edits carried across a switch" case_carry_edits
run_case "renaming a branch" case_rename
run_case "detached HEAD leaves okf/ where it was" case_detached
run_case "a second worktree" case_second_worktree
run_case "--init in a clone made before okf/main was pushed" case_init_older_clone
run_case "default branch named master" case_master
run_case "hook manager: core.hooksPath refused" case_hooks_path
run_case "git gc leaves pairings alone" case_gc
run_case "git pull does not update okf/" case_pull
run_case "idempotent re-run" case_rerun
run_case "--print-hook matches the installed hooks" case_print_hook
run_case "hook manager: husky snippets with --no-hooks" case_husky
run_case "refused: unknown origin/HEAD" case_unknown_head
run_case "refused: a hook from another tool" case_foreign_hook
run_case "refused: git older than 2.42 for --init" case_old_git_init
run_case "refused: git older than 2.28 for the hooks" case_old_git_hooks
run_case "refused: no remote named origin" case_no_origin
run_case "refused: a top-level okf path that is not the worktree" case_foreign_okf

[ "$failed" -eq 0 ] || { echo "$failed case(s) failed"; exit 1; }
