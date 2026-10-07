#!/usr/bin/env bash
# `compass setup` checks the hub is readable and its folder is free or already
# a clone of it, installs okf into <prefix>/bin, clones the hub (or reuses the
# clone), records the hub and its folder in the user config, and puts the
# skill in each agent's user-level skill folder. It is idempotent and writes
# nothing when the hub is unreadable or an input is bad. HOME points at a
# scratch folder and the hub and its one repo are local bare repos behind
# github.com URLs, so nothing touches the real user setup or GitHub. Offline
# when okf is already cached (okf_bin).
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
okf=$(okf_bin) || { echo "FAIL: no okf binary"; exit 1; }

git_isolated "$work/gitconfig"
map_owner acme "$work/remotes"
make_folder_remote "$work/remotes/billing-api.git" "$TESTDATA/proto/billing-api/okf"
# The hub: the shipped template plus one repo, as an end user makes it.
cp -R "$TOOLS_ROOT/templates/hub" "$work/seed"
echo "billing-api https://github.com/acme/billing-api folder" >> "$work/seed/repos.txt"
git init -q "$work/seed" && git -C "$work/seed" add -A && git -C "$work/seed" commit -qm hub
git clone -q --bare "$work/seed" "$work/remotes/hub.git"
git clone -q --bare "$work/seed" "$work/remotes/other.git"

unset CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
export HOME=$work/home XDG_CONFIG_HOME=$work/home/.config OKF_HUB_CACHE=$work/home/.cache/okf-hub
mkdir -p "$HOME/.claude" "$HOME/.omp/agent"
cfg=$XDG_CONFIG_HOME/compass/config
bin=$HOME/.local/bin
clone=$HOME/src/hub

# snapshot <dir>: every file with its mode and checksum.
snapshot() {
  (cd "$1" && find . -type f -exec sh -c 'printf "%s %s " "$1" "$(stat -c %a "$1")"; cksum < "$1"' _ {} \; | sort)
}

compass setup --hub acme/hub --yes --okf-bin "$okf" </dev/null >"$work/out1" 2>&1
check "--yes with flags and no terminal exits 0" test $? -eq 0
for a in .claude/skills/okf .omp/agent/skills/okf; do
  check "skill installed at $a" cmp "$TOOLS_ROOT/skill/okf/SKILL.md" "$HOME/$a/SKILL.md"
done
check "no skill for an agent whose folder is missing" test ! -e "$HOME/.cursor"
check "okf installed in ~/.local/bin" "$bin/okf" version
check "hub cloned into ~/src/<repo> by default" test -f "$clone/repos.txt"
check "clone's origin is the hub" test "$(git -C "$clone" config --get remote.origin.url)" = https://github.com/acme/hub.git
check "config records the hub" grep -qx 'OKF_HUB=acme/hub' "$cfg"
check "config records the clone" grep -qx "OKF_HUB_DIR=$clone" "$cfg"

# The hub commands use the recorded clone when given no folder.
(cd "$work" && compass hub assemble --ci) >"$work/asm" 2>&1
check "hub assemble with no folder assembles the recorded clone" test -f "$clone/repos/billing-api/overview.md"
(cd "$work" && OKF=$bin/okf compass hub check) >"$work/hubcheck" 2>&1
check "hub check with no folder checks the recorded clone" test $? -eq 0
check "hub check names the clone" grep -q "strict validator: $clone" "$work/hubcheck"

before=$(snapshot "$HOME")
compass setup --hub acme/hub --yes --okf-bin "$okf" </dev/null >"$work/out2" 2>&1
check "second run exits 0" test $? -eq 0
check "second run leaves the same files" test "$before" = "$(snapshot "$HOME")"
check "second run reuses the clone" grep -q "reusing the clone of acme/hub at $clone" "$work/out2"

# Prompts: answers come from stdin; the configured hub and clone are the defaults.
printf '\n\n' | compass setup --okf-bin "$okf" --agents claude >"$work/prompt" 2>&1
check "prompts accept defaults" test $? -eq 0
check "prompt run keeps the configured hub" grep -qx 'OKF_HUB=acme/hub' "$cfg"
check "prompt asks for the hub with its default" grep -q 'Your hub repo (owner/name) \[acme/hub\]' "$work/prompt"
check "prompt asks for the folder with the recorded one" grep -qF "Folder to clone it into [$clone]" "$work/prompt"

# A clone with an ssh origin of the same hub is reused; a relative --hub-dir is
# recorded as an absolute path.
git clone -q "$work/remotes/hub.git" "$work/ssh-clone"
git -C "$work/ssh-clone" remote set-url origin git@github.com:Acme/hub.git
(cd "$work" && compass setup --hub acme/hub --hub-dir ssh-clone --yes --okf-bin "$okf" </dev/null) >"$work/ssh" 2>&1
check "an ssh clone of the same hub is reused" grep -q "reusing the clone of acme/hub at $work/ssh-clone" "$work/ssh"
check "a relative --hub-dir is recorded absolute" grep -qx "OKF_HUB_DIR=$work/ssh-clone" "$cfg"

CLAUDE_CONFIG_DIR=$work/cc PI_CODING_AGENT_DIR=$work/pi \
  compass setup --hub acme/hub --hub-dir "$work/c2" --yes --prefix "$work/p" --agents claude,omp --okf-bin "$okf" >/dev/null 2>&1
check "--prefix installs into DIR/bin" test -x "$work/p/bin/okf"
check "--hub-dir clones into DIR" test -f "$work/c2/repos.txt"
check "claude skill follows CLAUDE_CONFIG_DIR" test -f "$work/cc/skills/okf/SKILL.md"
check "omp skill follows PI_CODING_AGENT_DIR" test -f "$work/pi/skills/okf/SKILL.md"

# Refusals, each from a fresh HOME so "writes nothing" is checked on all of it.
refuse() { # refuse <name> <setup args...>: non-zero exit and HOME unchanged
  local name=$1; shift
  local before
  before=$(snapshot "$work/h2")
  HOME=$work/h2 XDG_CONFIG_HOME=$work/h2/.config compass setup "$@" --okf-bin "$okf" </dev/null >"$work/refused" 2>&1
  check "$name exits non-zero" test $? -ne 0
  check "$name writes nothing" test "$before" = "$(snapshot "$work/h2")"
  check "$name clones nothing" test ! -e "$work/h2/src/hub"
}
fresh_h2() { rm -rf "$work/h2" && mkdir -p "$work/h2/.claude"; }
fresh_h2
refuse "an unreadable hub" --hub acme/missing --yes
check "an unreadable hub is named" grep -q "cannot read hub acme/missing" "$work/refused"
refuse "an unknown agent" --hub acme/hub --yes --agents claude,vim
refuse "--yes without a hub" --yes
refuse "a hub that is not OWNER/REPO" --hub hub --yes
refuse "a hub named .." --hub acme/.. --yes
mkdir -p "$work/h2/busy" && echo x > "$work/h2/busy/notes.txt"
refuse "a hub folder that is not a clone" --hub acme/hub --hub-dir "$work/h2/busy" --yes
check "a busy hub folder is named" grep -q "hub folder $work/h2/busy is not empty and is not a git clone" "$work/refused"
fresh_h2
git clone -q "$work/remotes/other.git" "$work/h2/other"
git -C "$work/h2/other" remote set-url origin https://github.com/acme/other.git
refuse "a clone of another repo" --hub acme/hub --hub-dir "$work/h2/other" --yes
check "the other clone is named" grep -q "holds a clone of https://github.com/acme/other.git, not acme/hub" "$work/refused"
git -C "$work/h2/other" remote set-url origin https://x-access-token:s3cr3t-tok@github.com/acme/other.git
refuse "a token clone of another repo" --hub acme/hub --hub-dir "$work/h2/other" --yes
check "the refusal never prints the token" bash -c "! grep -q s3cr3t-tok '$work/refused'"

# A CI-style clone with a token in its URL is reused, and the token not printed.
git clone -q "$work/remotes/hub.git" "$work/token-clone"
git -C "$work/token-clone" remote set-url origin https://x-access-token:s3cr3t-tok@github.com/acme/hub
compass setup --hub acme/hub.git --hub-dir "$work/token-clone" --yes --okf-bin "$okf" </dev/null >"$work/token" 2>&1
check "a token clone of the hub is reused" grep -q "reusing the clone of acme/hub at $work/token-clone" "$work/token"
check "reuse never prints the token" bash -c "! grep -q s3cr3t-tok '$work/token' '$cfg'"
check "--hub OWNER/REPO.git is recorded as OWNER/REPO" grep -qx 'OKF_HUB=acme/hub' "$cfg"

finish
