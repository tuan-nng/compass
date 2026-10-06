# Shared helpers for the compass test suites. Source it; do not run it.
# shellcheck shell=bash

TOOLS_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck disable=SC2034  # used by the scripts that source this file
TESTDATA="$TOOLS_ROOT/testdata"

# COMPASS: the binary under test. $COMPASS overrides; otherwise the clone is
# built into .cache/bin/compass (run-all.sh builds once and exports it).
if [ -z "${COMPASS:-}" ]; then
  COMPASS="$TOOLS_ROOT/.cache/bin/compass"
  (cd "$TOOLS_ROOT" && go build -o "$COMPASS" ./cmd/compass) || { echo "lib.sh: go build failed" >&2; exit 1; }
fi
export COMPASS
compass() { "$COMPASS" "$@"; }

# Keep the developer's own `compass setup` config (its org and hub) out of the
# suites: they read a config folder that holds none (test/setup.sh sets its own).
export XDG_CONFIG_HOME="$TOOLS_ROOT/.cache/no-user-config"

fails=0

pass() { printf 'pass: %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*"; fails=$((fails + 1)); }

# check <name> <command...>: run the command quietly and record pass/fail.
check() {
  local name=$1; shift
  if "$@" >/dev/null 2>&1; then pass "$name"; else fail "$name"; fi
}

# finish: print a summary and exit non-zero when any check failed.
finish() {
  if [ "$fails" -eq 0 ]; then echo "ok: all checks passed"; exit 0; fi
  echo "failed: $fails check(s)"; exit 1
}

# okf_bin: path to the pinned okf. $OKF overrides; otherwise the pinned
# release is installed once into .cache/pinned/<tag>/.
okf_bin() {
  if [ -n "${OKF:-}" ]; then echo "$OKF"; return; fi
  local tag dest
  tag=$(sed -n 's/^OKF_TAG=//p' "$TOOLS_ROOT/pins/okf.env")
  dest="$TOOLS_ROOT/.cache/pinned/$tag"
  if [ ! -x "$dest/okf" ]; then
    "$COMPASS" okf install --dest "$dest" >&2 || return 1
  fi
  echo "$dest/okf"
}

strict_validate() { "$COMPASS" validate "$@"; }

# strip_root_frontmatter <index.md>: drop the frontmatter block, as assembly does.
strip_root_frontmatter() {
  local f=$1
  [ -f "$f" ] && [ "$(head -1 "$f")" = "---" ] || return 0
  awk 'NR==1 && /^---$/ {f=1; next} f && /^---$/ {f=0; next} !f' "$f" > "$f.tmp"
  mv "$f.tmp" "$f"
}

# copy_hub <set-dir> <dest> [strip]: build a hub view by copying the hub and
# each repo bundle into <dest>/repos/<name>/. With "strip", remove each copied
# root index.md frontmatter (the conventions); without it, keep a raw copy
# (the protocol's hub-copy).
copy_hub() {
  local set=$1 dest=$2 mode=${3:-}
  rm -rf "$dest"
  cp -R "$set/hub" "$dest"
  mkdir -p "$dest/repos"
  local r
  for r in billing-api web-app shared-auth; do
    cp -R "$set/$r/okf" "$dest/repos/$r"
    if [ "$mode" = strip ]; then strip_root_frontmatter "$dest/repos/$r/index.md"; fi
  done
}

# git_isolated: point git at a throwaway global config (and no system config),
# so tests can add url.insteadOf rewrites without touching the user's setup.
git_isolated() {
  local cfg=$1
  export GIT_CONFIG_GLOBAL=$cfg GIT_CONFIG_NOSYSTEM=1
  git config --global user.email test@example.com
  git config --global user.name test
  git config --global init.defaultBranch main
  git config --global protocol.file.allow always
}

# map_org <org> <remotes-dir>: make https://github.com/<org>/<x> fetch from
# <remotes-dir>/<x>, so scripts see real-looking URLs while tests stay offline.
map_org() {
  git config --global "url.file://$2/.insteadOf" "https://github.com/$1/"
}

# make_folder_remote <bare-path> <bundle-dir>: a code repo whose default
# branch holds some code plus the bundle at okf/.
make_folder_remote() {
  local bare=$1 bundle=$2 w
  w=$(mktemp -d)
  git init -q "$w"
  mkdir -p "$w/src" && echo "package main" > "$w/src/main.go"
  cp -R "$bundle" "$w/okf"
  git -C "$w" add -A && git -C "$w" commit -qm base
  git clone -q --bare "$w" "$bare"
  rm -rf "$w"
}

# make_branch_remote <bare-path> <bundle-dir>: a code repo in branch mode:
# main holds only code, the orphan okf/main holds the bundle at its root.
make_branch_remote() {
  local bare=$1 bundle=$2 w
  w=$(mktemp -d)
  git init -q "$w"
  mkdir -p "$w/src" && echo "package main" > "$w/src/main.go"
  git -C "$w" add -A && git -C "$w" commit -qm base
  git -C "$w" switch -q --orphan okf/main
  git -C "$w" rm -rqf --cached . >/dev/null 2>&1 || true
  rm -rf "${w:?}/src"
  cp -R "$bundle/." "$w/"
  mkdir -p "$w/.github/workflows" && echo "name: okf" > "$w/.github/workflows/okf.yml"
  git -C "$w" add -A && git -C "$w" commit -qm "okf: bundle"
  git -C "$w" switch -q main
  git clone -q --bare "$w" "$bare"
  rm -rf "$w"
}
