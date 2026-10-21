# Installing compass

This guide takes a team from nothing to a working setup:

1. Each developer installs `compass` and runs `compass setup` once per machine.
2. One person makes the team's hub repo from compass's template and protects
   it.
3. Each repo that keeps knowledge gets one row in the hub's `repos.txt`. The
   repo itself gets nothing.
4. Each developer adds one line to their user-level agent instructions, which
   sends agents to the hub.

CI and cloud agents need the same per-machine setup. The two commands that
write to GitHub have their own section near the end.

## Prerequisites

- **Linux or macOS, on amd64 or arm64.** These are the platforms the pinned
  `okf` release is built for; `compass okf install` refuses any other.
- **Go 1.23 or newer**, to build compass.
- **git 2.41 or newer.** `compass sync` builds each merge with
  `git --attr-source=... merge-tree --write-tree`, which older git lacks.
- **git access to GitHub over HTTPS without a prompt.** `compass setup` reads
  and clones the hub over `https://github.com/...` and never asks for a
  password. If you use the GitHub CLI, `gh auth setup-git` sets this up.
- **Push access to the hub,** for every developer whose agent writes
  knowledge (section 3).

## 1. Install compass

compass is built from a clone. Its Go module path is `compass`, not a GitHub
URL, so `go install github.com/...@latest` does not work.

```sh
git clone https://github.com/tuan-nng/compass
cd compass
make install
compass version
```

`make install` puts `compass` in `$(go env GOBIN)`, or `$(go env GOPATH)/bin`
if `GOBIN` is unset. That folder must be on your `PATH`, because agents call
`compass` too. `compass version` prints the commit you built.

Keep the clone. You copy the hub template and the agent pointer line from it
in the steps below.

## 2. Set up your machine

```sh
compass setup
```

It asks for two things:

- your hub repo, as `OWNER/REPO`;
- the folder to clone the hub into, by default `~/src/<repo>`.

Before writing anything, it checks that the hub is readable with your git
credentials, and that the folder is empty or already holds a clone of that
hub. If either check fails, it stops and names the problem. Otherwise it:

| What | Where |
|---|---|
| Installs the pinned `okf` CLI, after checking its sha256 | `~/.local/bin/okf` (`--prefix DIR` changes `~/.local`) |
| Clones the hub, or reuses the clone already there | the folder you chose |
| Records the hub and its folder | `$XDG_CONFIG_HOME/compass/config`, by default `~/.config/compass/config` |
| Installs the OKF skill for Claude Code | `${CLAUDE_CONFIG_DIR:-~/.claude}/skills/okf` |
| Installs the OKF skill for Cursor | `~/.cursor/skills/okf` |
| Installs the OKF skill for omp | `${PI_CODING_AGENT_DIR:-~/.omp/agent}/skills/okf` |

By default the skill goes to each agent whose folder already exists on the
machine; `--agents claude,cursor,omp` picks them yourself. `--hub` and
`--hub-dir` answer the two questions in advance. Setup prints a note if
`~/.local/bin` or `compass` is not on your `PATH`.

Running setup again is safe. It reports each part as current, or replaces it
when it changed.

Check the result:

```sh
okf --version      # JSON whose "version" is the pinned 0.5.0-tuan-nng.1
compass hub check  # checks the recorded hub clone; a new hub passes
```

`compass hub check`, `compass hub branch`, `compass sync` and
`compass stamp` use the recorded hub clone, so they work from any folder.

## 3. Make your team's hub

Do this once per team. Skip it if your team already has a hub.

The hub is an ordinary GitHub repo that holds all the knowledge: each listed
repo's concepts under `repos/<id>/`, and knowledge that spans repos beside
them. It also holds two control files: `repos.txt` lists the repos, and
`checks.txt` lists the CI jobs that earn stamps. It has no workflows and
stores no keys.

**One hub per access group.** Anyone who can read the hub reads every listed
repo's knowledge, and compass has no per-repo access control. If some repos'
knowledge must stay apart, make one hub for each group of people who may read
it.

```sh
gh repo create OWNER/knowledge-hub --private --clone
cp -R path/to/compass/templates/hub/. knowledge-hub/
cd knowledge-hub
```

Then:

1. In `CODEOWNERS`, replace `@OKF_HUB_OWNERS` with the people who approve
   changes to `CODEOWNERS`, `repos.txt` and `checks.txt`.
2. Commit and push.
3. Give push access to every developer whose agent writes knowledge. Their
   agents push hub branches named `<id>/<branch>`.
4. Protect the default branch, as below.

### Protect the hub

Every writing developer can push to the hub, so the review before each merge
holds only where GitHub enforces it. Add a ruleset on the default branch that:

- blocks direct pushes;
- requires a pull request, with a code-owner approval, so changes to the
  control files need the people named in `CODEOWNERS`.

Only the person who runs `compass stamp` needs a bypass of this ruleset,
because the stamper pushes stamps straight to the default branch.
`compass sync` merges approved pull requests, which the ruleset allows.

Rulesets need a public hub or a paid GitHub plan. On GitHub Free they work
only on public repos, so a private hub on Free relies on reviewers following
the rule by hand.

The hub's `overview.md` explains how to fill it in, write to it, check it,
and protect it. It travels with the hub, so the team reads it there.

## 4. Add a repo

Add one row to the hub's `repos.txt`, through a hub pull request:

```
billing-api https://github.com/acme/billing-api
```

The first column is the repo's id. It names the folder `repos/<id>/` and
prefixes the repo's hub branches, `<id>/<branch>`. An id may use letters,
digits, `.`, `_` and `-`. It may not start with `.` or `-`, contain `..`, or
end in `.lock`. The URL may name any owner.

Nothing changes in the repo itself: no folder, workflow, hook or instruction
file. Its `repos/<id>/` folder appears with the first hub branch that writes
to it.

To start the folder by hand, add `repos/billing-api/overview.md` in the same
hub pull request as the row:

```sh
mkdir -p repos/billing-api
cat > repos/billing-api/overview.md <<'EOF'
---
type: Overview
title: billing-api
description: What billing-api does and how its knowledge is organised.
tags: [overview]
status: draft
generated: { by: human:alice, at: 2026-10-07T12:00:00Z }
---
# billing-api

One paragraph on what this repo is for.
EOF
okf index .
compass hub check .
```

In `generated`, put your GitHub login after `human:` and the current UTC time
(`date -u +%Y-%m-%dT%H:%M:%SZ`). The strict validator fails a concept
without `generated`, or with a `by` that is not `human:<id>`,
`process:<id>` or `<producer>/<version>`.

`okf index .` at the hub root writes `repos/billing-api/index.md` and updates
`repos/index.md` and the root `index.md`. `compass hub check .` then checks
the hub with the new folder in place. Name the branch for this pull request so
it does not start with an id from `repos.txt`; `compass sync` treats those as
paired with a code branch.

## 5. Point agents at the hub

Agents load a skill only when its description matches the task, and the code
repos carry nothing that mentions the hub. So each developer adds the line in
compass's `templates/user-pointer.md` to their user-level agent instructions,
such as `~/.claude/CLAUDE.md` for Claude Code. It is the only thing that sends
an agent to the hub. It tells the agent to run `compass hub branch` in the
repo before starting work and, if the repo is listed, to run `compass sync`
and follow the OKF skill.

From then on, an agent working on branch `<b>` of a listed repo:

1. runs `compass hub branch` in the repo's clone, which prints the path of a
   hub worktree on `<id>/<b>`, at `<hub clone>.worktrees/<id>/<b>`;
2. reads and writes knowledge there;
3. runs `okf index .` and `compass hub check .` in the worktree, commits,
   pushes `<id>/<b>`, and opens a hub pull request that names the code pull
   request, as `owner/repo#N` or its URL.

A person with push access to the hub reviews and approves the hub pull
request. Once the code pull request merges, the next `compass sync` merges
it ("The two writers" below).

A hub pull request that a reviewer will merge by hand, such as one from
section 4, gets no check from `compass sync`. The reviewer runs
`compass hub check` on its branch before approving it.

## CI and cloud agents

A CI job or a cloud agent environment needs the same setup as a developer
machine, without questions:

```sh
make install   # in a compass clone, with Go 1.23 or newer
compass setup --hub OWNER/REPO --yes --agents claude
```

`--yes` takes the flags and defaults and asks nothing. The environment still
needs git credentials that can read the hub over HTTPS, and push to it if its
agents write knowledge.

Add the line in `templates/user-pointer.md` to the platform's
organisation-level agent instructions. No real CI or cloud agent has run it
yet.

## The two writers

Two commands write to GitHub. Both write only to the hub; they only read the
code repos.

- `compass sync` merges an approved hub pull request from `<id>/<b>` after
  the linked code pull request from `<b>` merges into the default branch of
  repo `<id>`. Before merging, it checks the merge result locally and refuses
  a branch that adds a finding. Otherwise it comments on the hub pull request
  and tries again next run. It also closes abandoned hub pull requests and
  deletes finished hub branches. Agents run it when a session starts; then
  `compass hub branch --prune` removes this machine's finished worktrees.
- `compass stamp` marks concepts `verified: process:<actor>` when the CI job
  that `checks.txt` names passed on the code repo's default branch head. It
  commits the stamps to the hub's default branch, with one fast-forward per
  run.

Both read a token from `GITHUB_TOKEN`, or from the variable `--token-env`
names. The token's account needs read access to every repo in `repos.txt`
and write access to the hub. When sync cannot read a repo, it reports that
repo's branches as failed, goes on with the others, and exits 1. The stamper
also needs the ruleset bypass from section 3. Without one, GitHub refuses
its push, and the stamper says so and exits non-zero.

Start with a dry run, which reads GitHub and makes no writes:

```sh
GITHUB_TOKEN=$(gh auth token) compass sync --dry-run
GITHUB_TOKEN=$(gh auth token) compass stamp --dry-run
```

## Updating

```sh
cd path/to/compass
git pull
make install
compass setup
```

Re-running setup installs a newer pinned `okf` or skill if the update changed
them. It leaves the hub clone alone. Nothing in the code repos needs to
change.

## Removing

compass keeps no state outside the places in the setup table, plus the hub
worktrees that `compass hub branch` creates. To remove it, delete:

- `compass` from `$(go env GOBIN)` or `$(go env GOPATH)/bin`;
- `okf` from `~/.local/bin`, or from the `--prefix` you chose;
- `~/.config/compass/`;
- the `skills/okf` folder in each agent's folder;
- the line from `templates/user-pointer.md` in your agent instructions;
- the hub worktrees, under `<hub clone>.worktrees/`;
- the hub clone, if you no longer need it.
