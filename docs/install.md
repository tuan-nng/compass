# Installing compass

This guide takes a team from nothing to a working setup:

1. Each developer installs `compass` and runs `compass setup` once per machine.
2. One person makes the team's hub repo from compass's template.
3. Each repo that keeps knowledge gets a bundle, a CI check, and a row in the hub.

CI and cloud agents need the same per-machine setup. Hub owners also run the
two commands that write to GitHub. Both have their own sections near the end.

## Prerequisites

- **Linux or macOS, on amd64 or arm64.** These are the platforms the pinned
  `okf` release is built for; `compass okf install` refuses any other.
- **Go 1.23 or newer**, to build compass.
- **git.** Branch mode needs git 2.28 or newer, and 2.42 or newer to create a
  repo's `okf/main` branch.
- **git access to GitHub over HTTPS without a prompt.** `compass setup` reads
  and clones the hub over `https://github.com/...` and never asks for a
  password. If you use the GitHub CLI, `gh auth setup-git` sets this up.

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

Keep the clone. You copy the hub template and the repo workflows from it in
the steps below.

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
okf --version   # JSON whose "version" is the pinned 0.5.0-tuan-nng.1
```

`compass hub assemble` and `compass hub check` use the recorded hub clone,
so they work from any folder. On a new hub they exit 1 with
`repos.txt lists no repos`; they are useful once section 4 adds a repo.

## 3. Make your team's hub

Do this once per team. Skip it if your team already has a hub.

The hub is an ordinary GitHub repo holding knowledge that spans repos, plus
two control files: `repos.txt` lists the repos to assemble, and `checks.txt`
lists the CI jobs that earn stamps. It has no workflows and stores no keys.

```sh
gh repo create OWNER/knowledge-hub --private --clone
cp -R path/to/compass/templates/hub/. knowledge-hub/
cd knowledge-hub
```

Then:

1. In `CODEOWNERS`, replace `@OKF_HUB_OWNERS` with the people who approve
   changes to `CODEOWNERS`, `repos.txt` and `checks.txt`.
2. Commit and push.
3. Add a ruleset on the default branch that blocks direct pushes and requires
   a pull request with a code-owner approval. On GitHub Free, rulesets work
   only on public repos. A private hub on Free relies on reviewers following
   the rule by hand.

The hub's `overview.md` explains how to fill it in, check it, and protect it.
It travels with the hub, so the team reads it there.

## 4. Add a repo

The repo's maintainers first choose a mode:

- **Folder mode:** knowledge lives in `okf/` on the default branch and is
  reviewed in the code pull request.
- **Branch mode:** knowledge lives on an `okf/main` branch, for maintainers
  who want no knowledge files on their code branches.

Both modes run compass's bundle check in CI. The workflow templates pin the
action to a compass commit, so the check always builds the code you chose.
Before committing a workflow, replace two placeholders in it:

- `COMPASS_REPO` with `tuan-nng/compass`;
- `COMPASS_SHA` with a full 40-character compass commit SHA, such as the one
  `git ls-remote https://github.com/tuan-nng/compass master` prints.

### Folder mode

In a clone of the repo, on a new branch:

```sh
mkdir okf
cat > okf/index.md <<'EOF'
---
okf_version: "0.2"
---
# Index
EOF
cat > okf/overview.md <<'EOF'
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
okf index okf
git add okf
git commit -m "okf: start the bundle"
compass bundle check okf
```

In `generated`, put your GitHub login after `human:` and the current UTC time
(`date -u +%Y-%m-%dT%H:%M:%SZ`). The strict validator fails a concept
without `generated`, or with a `by` that is not `human:<id>`,
`process:<id>` or `<producer>/<version>`.

`okf index okf` rewrites `okf/index.md` and adds an `index.md` to each folder.
Committing that now keeps the one-time change out of the first agent pull
request. `compass bundle check okf` runs the same check as CI. It refuses
uncommitted changes in the bundle, so commit first.

Then, in the same pull request:

1. Copy `templates/folder-workflow.yml` from compass to
   `.github/workflows/okf.yml`, and fill in the two placeholders.
2. Add the line in `templates/AGENTS-pointer.md` to the repo's `AGENTS.md`
   or `CLAUDE.md`. It tells agents to read `okf/index.md` and follow the skill.

Last, add a row to the hub's `repos.txt` through a hub pull request:

```
billing-api https://github.com/acme/billing-api folder
```

Once the repo's change is on its default branch, check the hub with the new
row in place:

```sh
compass hub assemble
compass hub check
```

### Branch mode

1. Agree with the maintainers on the `okf/...` branches. Hub owners need
   write access, because `compass sync` merges into `okf/main` and
   `compass stamp` commits to it.
2. In a clone, create the branch. Only the first person does this.

   ```sh
   compass branch setup --init
   ```

   It creates the empty branch `okf/main` with a root `index.md` and checks
   it out at `okf/`. Add `okf/overview.md` as in folder mode, then:

   ```sh
   okf index okf
   git -C okf add -A
   git -C okf commit -m "okf: start the bundle"
   git -C okf push -u origin okf/main
   ```

3. On `okf/main` only, add `templates/okf-main-workflow.yml` as
   `.github/workflows/okf.yml` and fill in the two placeholders. The code
   branches never carry it.
4. Protect `okf/main` so changes arrive only through pull requests, with a
   bypass for the hub owners.
5. Add a row to the hub's `repos.txt`:

   ```
   web-app https://github.com/acme/web-app branch
   ```

The repo's `AGENTS.md` lives on the code branches, so it cannot point agents
at `okf/main`. Instead, each developer adds the line in
`templates/user-pointer.md` to their user-level agent instructions, then runs
this once in each clone:

```sh
compass branch setup
```

It checks out `okf/` and installs two git hooks. From then on, switching
branches switches `okf/` to the matching knowledge branch.

If the repo uses a hook manager, setup refuses before changing anything. It
checks whether git's `core.hooksPath` is set, or a hook that isn't compass's
is already in `.git/hooks`.
Run `compass branch setup --no-hooks` and add the hooks through the manager
instead. `templates/hooks/` has snippets for husky and lefthook, and
`compass branch setup --print-hook post-checkout` prints a hook script.

## CI and cloud agents

A CI job or a cloud agent environment needs the same setup as a developer
machine, without questions:

```sh
make install   # in a compass clone, with Go 1.23 or newer
compass setup --hub OWNER/REPO --yes --agents claude
```

`--yes` takes the flags and defaults and asks nothing. The environment still
needs git credentials that can read the hub over HTTPS.

For branch-mode repos, add the line in `templates/user-pointer.md` to the
platform's organisation-level agent instructions. CI and cloud agents start
from fresh clones without the hooks, and the line tells them to run
`compass branch setup` first. This has only been tested in a headless
scenario so far.

## Hub owners: the two writers

Two commands write to GitHub. Hub owners run them from their hub clone, on
demand, during the pilot daily:

- `compass stamp` marks concepts `verified: process:<actor>` when the CI job
  that `checks.txt` names passed on the repo's default branch.
- `compass sync` merges approved, green branch-mode knowledge pull requests
  after their code pull request merges, and deletes finished `okf/<b>`
  branches.

Both read a token from `GITHUB_TOKEN`, or from the variable `--token-env`
names. The token's account needs write access to every repo in `repos.txt`:
the stamper commits through the GitHub API, and sync merges, closes and
comments on pull requests and deletes branches. A protected default branch
or `okf/main` also needs a bypass for that account. Without one, GitHub
refuses the stamper's push, and the stamper says so and exits non-zero.

Start with a dry run, which reads GitHub and makes no writes:

```sh
GITHUB_TOKEN=$(gh auth token) compass stamp --dry-run
GITHUB_TOKEN=$(gh auth token) compass sync --dry-run
```

## Updating

```sh
cd path/to/compass
git pull
make install
compass setup
```

Re-running setup installs a newer pinned `okf` or skill if the update changed
them. It leaves the hub clone alone. Repos pick up a new bundle check only
when you change `COMPASS_SHA` in their workflow.

## Removing

compass keeps no state outside the places in the setup table, plus a clone
cache for assembly. To remove it, delete:

- `compass` from `$(go env GOBIN)` or `$(go env GOPATH)/bin`;
- `okf` from `~/.local/bin`, or from the `--prefix` you chose;
- `~/.config/compass/`;
- the `skills/okf` folder in each agent's folder;
- the assembly cache, `${XDG_CACHE_HOME:-~/.cache}/okf-hub`, or
  `$OKF_HUB_CACHE` if you set it;
- the hub clone, if you no longer need it.

In a branch-mode clone, also remove the two hooks from `.git/hooks/`
(`post-checkout` and `reference-transaction`, marked `okf-branch-hook`) and
the `okf/` worktree, with `git worktree remove okf`.
