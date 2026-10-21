#!/usr/bin/env bash
# Run a coding agent headless with the okf skill on the test data and check
# what it did. Each run gets fresh local bare remotes: three code repos that
# hold no knowledge (billing-api from test/skill-eval/code), and a hub that
# lists them in repos.txt and holds testdata/proto's knowledge under
# repos/<id>/. The agent works in a billing-api clone on a feature branch, with
# a knowledge-hub clone next to it recorded as the hub. The user-level pointer
# line (templates/user-pointer.md) is the only trigger; code repos carry
# nothing. No run touches GitHub: https://github.com/$org/ URLs are rewritten
# to the local remotes.
#
# Usage: skill-eval.sh --agent claude|omp [--runs N] [--scenario NAME]
#                      [--timeout SECS] [--model MODEL]
#   --scenario   read, write, cross-repo, injected, autoload, or all
#                (default: the first four)
#   --runs       runs per scenario (default 1)
#   --timeout    seconds per agent run (default 900)
#   --model      model flag passed to the agent (default: the agent's default)
# Env: OKF                 okf binary to install (default: the pinned release)
#      OKF_EVAL_OUT        where runs and transcripts go
#                          (default .cache/skill-eval/<time>-<agent>)
#      ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN
#                          when set, claude runs with its own CLAUDE_CONFIG_DIR
#                          holding only the installed skill. Without either, an
#                          empty config dir has no login, so claude keeps the
#                          real login but drops user settings, skills, hooks and
#                          plugins (--setting-sources project,local) and loads
#                          the installed user-level skill folder via --add-dir.
# Prints one line per check, a result per run, and "<scenario>: k/N passed".
# Exits 1 if any run fails; prints "skipped: <reason>" and exits 0 when the
# agent cannot run headless here.
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

agent="" runs=1 scenario="" timeout_s=900 model=""
die() { echo "skill-eval: $*" >&2; exit 2; }
while [ $# -gt 0 ]; do
  case $1 in
    --agent) agent=${2:-}; shift 2 ;;
    --runs) runs=${2:-}; shift 2 ;;
    --scenario) scenario=${2:-}; shift 2 ;;
    --timeout) timeout_s=${2:-}; shift 2 ;;
    --model) model=${2:-}; shift 2 ;;
    -h|--help) sed -n '2,31p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done
case $agent in claude|omp) ;; *) die "--agent must be claude or omp" ;; esac
[[ $runs =~ ^[1-9][0-9]*$ ]] || die "--runs must be a positive number"
[[ $timeout_s =~ ^[1-9][0-9]*$ ]] || die "--timeout must be a positive number"
case ${scenario:-core} in
  core) scenarios=(read write cross-repo injected) ;;
  all) scenarios=(read write cross-repo injected autoload) ;;
  read|write|cross-repo|injected|autoload) scenarios=("$scenario") ;;
  *) die "unknown scenario: $scenario" ;;
esac

if ! command -v "$agent" >/dev/null 2>&1; then
  echo "skipped: $agent CLI not found on PATH"; exit 0
fi
if [ "$agent" = omp ] && ! omp --help 2>/dev/null | grep -q -- '--mode='; then
  echo "skipped: this omp has no --mode json for headless runs"; exit 0
fi

here=$TOOLS_ROOT/test/skill-eval
okf=$(okf_bin) || die "no okf binary"
org="okf-eval"   # the GitHub owner the local remotes stand in for
out=${OKF_EVAL_OUT:-$TOOLS_ROOT/.cache/skill-eval/$(date -u +%Y%m%dT%H%M%SZ)-$agent}
mkdir -p "$out"
out=$(cd "$out" && pwd)
home=$out/home
echo "skill-eval: agent $agent, okf $("$okf" version 2>/dev/null | tr -d '\n' | head -c 80), output in $out"

# Install exactly what a user gets, into a scratch HOME: compass on PATH (as
# `make install` puts it), then `compass setup` against a local hub remote
# behind its github.com URL.
mkdir -p "$home/.local/bin" "$out/setup"
cp "$COMPASS" "$home/.local/bin/compass"
(
  export GIT_CONFIG_GLOBAL=$out/setup/gitconfig GIT_CONFIG_NOSYSTEM=1
  : > "$GIT_CONFIG_GLOBAL"
  git config --global user.name okf-eval
  git config --global user.email okf-eval@example.invalid
  git config --global "url.file://$out/setup/remotes/.insteadOf" "https://github.com/$org/"
  rm -rf "$out/setup/seed" "$out/setup/remotes"
  git init -q "$out/setup/seed" && git -C "$out/setup/seed" commit -q --allow-empty -m hub
  git clone -q --bare "$out/setup/seed" "$out/setup/remotes/knowledge-hub.git"
  HOME=$home XDG_CONFIG_HOME=$home/.config CLAUDE_CONFIG_DIR=$home/.claude PI_CODING_AGENT_DIR=$home/.omp/agent \
    "$home/.local/bin/compass" setup --hub "$org/knowledge-hub" --hub-dir "$home/src/knowledge-hub" --yes \
    --prefix "$home/.local" --agents "$agent" --okf-bin "$okf" </dev/null
) > "$out/install.log" 2>&1 || { cat "$out/install.log"; die "compass setup failed"; }

claude_isolated=""
if [ "$agent" = claude ]; then
  if [ -n "${ANTHROPIC_API_KEY:-}${CLAUDE_CODE_OAUTH_TOKEN:-}" ]; then
    claude_isolated=1
    echo "skill-eval: claude config dir $home/.claude (token from the environment)"
  else
    echo "skill-eval: claude uses the real login with --setting-sources project,local;" \
         "skill from $home/.claude/skills via --add-dir"
  fi
else
  printf 'skills:\n  customDirectories:\n    - %s\n' "$home/.omp/agent/skills" > "$out/omp-overlay.yml"
  echo "skill-eval: omp uses the real login, no extensions or rules, only the okf skill from $home/.omp/agent/skills"
fi

pointer_user=$(cat "$TOOLS_ROOT/templates/user-pointer.md")
branch=feat/invoice-retries   # the code branch every run works on

# setup_git <run>: per-run git config; github.com/<org>/x -> <run>/remotes/x.git
setup_git() {
  export GIT_CONFIG_GLOBAL=$1/gitconfig GIT_CONFIG_NOSYSTEM=1
  : > "$GIT_CONFIG_GLOBAL"
  git config --global user.name okf-eval
  git config --global user.email okf-eval@example.invalid
  git config --global init.defaultBranch main
  git config --global protocol.file.allow always
  git config --global "url.file://$1/remotes/.insteadOf" "https://github.com/$org/"
}

# code_remote <bare> [code-dir]: a code repo with no knowledge file.
code_remote() {
  local bare=$1 code=${2:-} w
  w=$(mktemp -d)
  git init -q "$w"
  if [ -n "$code" ]; then cp -R "$code/." "$w/"; else echo "# $(basename "$bare" .git)" > "$w/README.md"; fi
  git -C "$w" add -A && git -C "$w" commit -qm "$(basename "$bare" .git)"
  git clone -q --bare "$w" "$bare"
  rm -rf "$w"
}

refs() { for b in "$1"/remotes/*.git; do echo "== $b"; git -C "$b" for-each-ref; done; }

# prepare <scenario> <run>: build the remotes, the clone and the hub clone.
prepare() {
  local sc=$1 run=$2 set=$2/set seed=$2/hub-seed r
  rm -rf "$run"; mkdir -p "$run/remotes"
  setup_git "$run"
  cp -R "$TESTDATA/proto" "$set"
  if [ "$sc" = injected ]; then
    cat "$here/inject-repo.md" >> "$set/billing-api/okf/contracts/invoice-api.md"
    cat "$here/inject-hub.md" >> "$set/web-app/okf/services/web-app.md"
  fi
  code_remote "$run/remotes/billing-api.git" "$here/code"
  code_remote "$run/remotes/web-app.git"
  code_remote "$run/remotes/shared-auth.git"
  # The hub: proto's hub concepts plus each repo's knowledge under repos/<id>/.
  copy_hub "$set" "$seed" strip
  : > "$seed/repos.txt"
  for r in billing-api web-app shared-auth; do
    echo "$r https://github.com/$org/$r" >> "$seed/repos.txt"
  done
  (cd "$seed" && "$okf" index . >/dev/null) || return 1
  git -C "$seed" init -q && git -C "$seed" add -A && git -C "$seed" commit -qm hub
  git clone -q --bare "$seed" "$run/remotes/knowledge-hub.git"
  rm -rf "$seed"
  git clone -q "https://github.com/$org/knowledge-hub" "$run/knowledge-hub"
  git clone -q "https://github.com/$org/billing-api" "$run/billing-api"
  git -C "$run/billing-api" switch -q -c "$branch"
  { echo "REPO_BASE=$(git -C "$run/billing-api" rev-parse HEAD)"
    echo "HUB_BASE=$(git -C "$run/knowledge-hub" rev-parse HEAD)"
    echo "BRANCH=$branch"; } > "$run/base.env"
  # The setup config, with the hub clone pointed at this run's hub.
  mkdir -p "$run/config/compass"
  sed "s|^OKF_HUB_DIR=.*|OKF_HUB_DIR=$run/knowledge-hub|" "$home/.config/compass/config" > "$run/config/compass/config"
  refs "$run" > "$run/refs-before.txt"
  rm -f /tmp/okf-eval-pwned
}

# run_agent <run> <prompt-file> [system-prompt-line]: transcript in <run>/transcript.jsonl
run_agent() {
  local run=$1 prompt append=${3:-} cmd
  prompt=$(cat "$2")
  if [ "$agent" = claude ]; then
    cmd=(claude -p "$prompt" --output-format stream-json --verbose
         --permission-mode bypassPermissions --strict-mcp-config --no-session-persistence)
    if [ -z "$claude_isolated" ]; then cmd+=(--setting-sources "project,local" --add-dir "$home"); fi
  else
    cmd=(omp -p "$prompt" --mode json --no-session --no-title --no-extensions --no-rules
         --skills okf --config "$out/omp-overlay.yml" --auto-approve --max-time "$timeout_s")
  fi
  [ -z "$model" ] || cmd+=(--model "$model")
  [ -z "$append" ] || cmd+=(--append-system-prompt "$append")
  (
    cd "$run/billing-api" || exit 1
    export PATH=$home/.local/bin:$PATH GH_TOKEN=okf-eval-no-github
    export XDG_CONFIG_HOME=$run/config   # the config `compass setup` wrote, per run
    unset OKF
    if [ -n "$claude_isolated" ]; then export CLAUDE_CONFIG_DIR=$home/.claude; fi
    timeout -k 30 "$timeout_s" "${cmd[@]}" < /dev/null > "$run/transcript.jsonl" 2> "$run/agent.err"
  )
  local rc=$?
  [ "$rc" -ne 124 ] || echo "  agent timed out after ${timeout_s}s"
  refs "$run" > "$run/refs-after.txt"
}

analyze() {
  python3 "$here/analyze.py" "$1" --agent "$agent" --transcript "$2/transcript.jsonl" --run "$2" \
    --okf "$okf" --compass "$home/.local/bin/compass" --start "$3" | sed 's/^/  /'
  return "${PIPESTATUS[0]}"
}

status=0
summary=()
for sc in "${scenarios[@]}"; do
  passed=0
  for i in $(seq 1 "$runs"); do
    run=$out/$sc-$i
    echo "== $sc run $i/$runs ($run)"
    start=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    case $sc in
      autoload)
        prepare "$sc" "$run" || { echo "  setup failed"; continue; }
        run_agent "$run" "$here/prompts/read.txt"
        if analyze autoload "$run" "$start" >/dev/null; then
          echo "  loaded: yes (pointer line: no)"; ok=0
        else
          echo "  loaded: no (pointer line: no); retrying with the user-level pointer line"
          run=$run-pointer
          prepare "$sc" "$run" || { echo "  setup failed"; continue; }
          run_agent "$run" "$here/prompts/read.txt" "$pointer_user"
          if analyze autoload "$run" "$start" >/dev/null; then
            echo "  loaded: yes (pointer line: yes)"; ok=0
          else
            echo "  loaded: no (pointer line: yes)"; ok=1
          fi
        fi
        ;;
      *)
        prepare "$sc" "$run" || { echo "  setup failed"; continue; }
        run_agent "$run" "$here/prompts/$sc.txt" "$pointer_user"
        analyze "$sc" "$run" "$start"; ok=$?
        ;;
    esac
    if [ "$ok" -eq 0 ]; then passed=$((passed + 1)); echo "  result: pass"; else echo "  result: FAIL"; fi
  done
  summary+=("$sc: $passed/$runs passed")
  [ "$passed" -eq "$runs" ] || status=1
done

echo "== summary ($agent)"
printf '%s\n' "${summary[@]}"
exit $status
