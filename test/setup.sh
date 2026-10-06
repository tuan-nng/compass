#!/usr/bin/env bash
# `compass setup` checks the hub is readable, installs okf into <prefix>/bin,
# records the org and hub in the user config, and puts the skill in each
# agent's user-level skill folder. It is idempotent and writes nothing when
# the hub is unreadable or an input is bad. HOME points at a scratch folder and
# the hub is a local bare repo behind a github.com URL, so nothing touches the
# real user setup or GitHub. Offline when okf is already cached (okf_bin).
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
okf=$(okf_bin) || { echo "FAIL: no okf binary"; exit 1; }

git_isolated "$work/gitconfig"
map_org acme "$work/remotes"
git init -q "$work/seed" && git -C "$work/seed" commit -q --allow-empty -m hub
git clone -q --bare "$work/seed" "$work/remotes/hub.git"

unset CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR OKF_ORG
export HOME=$work/home XDG_CONFIG_HOME=$work/home/.config
mkdir -p "$HOME/.claude" "$HOME/.omp/agent"
cfg=$XDG_CONFIG_HOME/compass/config
bin=$HOME/.local/bin

# snapshot <dir>: every file with its mode and checksum.
snapshot() {
  (cd "$1" && find . -type f -exec sh -c 'printf "%s %s " "$1" "$(stat -c %a "$1")"; cksum < "$1"' _ {} \; | sort)
}

compass setup --org acme --hub acme/hub --yes --okf-bin "$okf" </dev/null >"$work/out1" 2>&1
check "--yes with flags and no terminal exits 0" test $? -eq 0
for a in .claude/skills/okf .omp/agent/skills/okf; do
  check "skill installed at $a" cmp "$TOOLS_ROOT/skill/okf/SKILL.md" "$HOME/$a/SKILL.md"
done
check "no skill for an agent whose folder is missing" test ! -e "$HOME/.cursor"
check "okf installed in ~/.local/bin" "$bin/okf" version
check "config records the org" grep -qx 'OKF_ORG=acme' "$cfg"
check "config records the hub" grep -qx 'OKF_HUB=acme/hub' "$cfg"

# The org in the user config reaches the other commands: hub assembly refuses
# a URL outside it, naming the configured org, before any fetch.
mkdir -p "$work/hub"
echo "x https://github.com/not-acme/x folder" > "$work/hub/repos.txt"
(cd "$work" && compass hub assemble "$work/hub") >"$work/asm" 2>&1
check "hub assemble refuses a URL outside the configured org" test $? -ne 0
check "hub assemble reads OKF_ORG=acme from the user config" grep -q "https://github.com/acme/" "$work/asm"

before=$(snapshot "$HOME")
compass setup --org acme --hub acme/hub --yes --okf-bin "$okf" </dev/null >/dev/null 2>&1
check "second run exits 0" test $? -eq 0
check "second run leaves the same files" test "$before" = "$(snapshot "$HOME")"

# Prompts: answers come from stdin; the configured hub is the default.
printf 'acme\n\n' | compass setup --okf-bin "$okf" --agents claude >"$work/prompt" 2>&1
check "prompts accept answers and defaults" test $? -eq 0
check "prompt run keeps the configured hub" grep -qx 'OKF_HUB=acme/hub' "$cfg"
check "prompt asks for the hub with its default" grep -q 'Your hub repo (owner/name) \[acme/hub\]' "$work/prompt"

CLAUDE_CONFIG_DIR=$work/cc PI_CODING_AGENT_DIR=$work/pi \
  compass setup --org acme --hub acme/hub --yes --prefix "$work/p" --agents claude,omp --okf-bin "$okf" >/dev/null 2>&1
check "--prefix installs into DIR/bin" test -x "$work/p/bin/okf"
check "claude skill follows CLAUDE_CONFIG_DIR" test -f "$work/cc/skills/okf/SKILL.md"
check "omp skill follows PI_CODING_AGENT_DIR" test -f "$work/pi/skills/okf/SKILL.md"

# Refusals, each from a fresh HOME so "writes nothing" is checked on all of it.
refuse() { # refuse <name> <setup args...>: non-zero exit and an empty HOME
  local name=$1; shift
  rm -rf "$work/h2" && mkdir -p "$work/h2/.claude"
  HOME=$work/h2 XDG_CONFIG_HOME=$work/h2/.config compass setup "$@" --okf-bin "$okf" </dev/null >"$work/refused" 2>&1
  check "$name exits non-zero" test $? -ne 0
  check "$name writes nothing" test -z "$(cd "$work/h2" && find . -mindepth 1 ! -path ./.claude)"
}
refuse "an unreadable hub" --org acme --hub acme/missing --yes
check "an unreadable hub is named" grep -q "cannot read hub acme/missing" "$work/refused"
refuse "an unknown agent" --org acme --hub acme/hub --yes --agents claude,vim
refuse "--yes without a hub" --org acme --yes
refuse "a hub that is not OWNER/REPO" --org acme --hub hub --yes

finish
