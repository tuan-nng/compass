---
type: Overview
title: Knowledge hub
description: How to fill in this hub, write knowledge through hub branches, check it, run the stamper and sync job, and protect its control files.
tags: [overview, hub]
status: stable
generated: { by: compass/hub-template, at: 2026-10-07T12:00:00Z }
---
# Knowledge hub

This repo is an OKF knowledge hub made from the compass hub template. It holds
all the knowledge for the repos it lists, in [OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md):
each repo's concepts under `repos/<id>/`, and knowledge that spans repos
everywhere else. The code repos carry no knowledge files, workflows or hooks.
Everything runs on people's machines, with their own GitHub credentials: the
hub has no CI, no workflows and no stored keys.

Anyone who can read this hub reads the knowledge of every repo it lists. A
team that needs to keep some repos' knowledge apart runs one hub per access
group.

## Fill it in

- `repos.txt`: one row per repo, `<id> <URL>`. Adding a repo takes only this
  row. Any `https://github.com/<owner>/<repo>` URL is accepted, so a hub can
  list repos from several accounts. The id names the folder `repos/<id>/` and
  prefixes the repo's hub branches, `<id>/<branch>`.
- `checks.txt`: one row per machine check whose passing run on a repo's
  default branch earns concepts a `verified: process:<actor>` stamp.
- `repos/<id>/`: the repo's own concepts. A repo's folder appears with the
  first hub branch that writes to it.
- `cross-repo/`: concepts of type `Cross-Repo Dependency`, the only place
  links between repos live. Link a repo's concept from the hub root, as
  `/repos/<id>/<concept>.md`.
- `decisions/` and `glossary/`: decisions and vocabulary that span repos.

After adding or changing concepts, run `okf index .` at the hub root and
commit the regenerated `index.md` files.

## Connect it

1. In `CODEOWNERS`, replace `@OKF_HUB_OWNERS` with the people who approve
   changes to `CODEOWNERS`, `repos.txt` and `checks.txt`.
2. Give every developer whose agent writes knowledge push access to this hub.
   Their agents push hub branches named `<id>/<branch>`.
3. Each developer runs `compass setup` and gives this repo as the hub. Setup
   clones it (by default into `~/src/<repo>`) and records the clone, so the
   commands below need no folder argument when run for that clone.
4. Each developer adds the line in compass's `templates/user-pointer.md` to
   their user-level agent instructions, and each CI or cloud agent platform
   adds it to its organisation-level instructions. The code repos carry
   nothing, so that line is the only thing that sends an agent here.

## Write to it

When an agent works on branch `<b>` of a listed repo, it runs
`compass hub branch` in that repo's clone. The command prints the path of a
worktree of this hub on branch `<id>/<b>`, in a folder next to the hub clone,
and creates it on the first run. The agent reads and writes there:

1. It changes concepts under `repos/<id>/` and, when needed, hub-level
   concepts such as `cross-repo/`.
2. It runs `okf index .` and `compass hub check .` in the worktree, commits,
   and pushes `<id>/<b>`.
3. It opens a hub pull request into the default branch that names the code
   pull request, as `owner/repo#N` or its URL.

A person with write access to this hub reviews and approves the hub pull
request, before or after the code pull request merges. Once the code merges,
`compass sync` merges the approved hub pull request (below).

A change that belongs to no code branch, such as a fix to a hub-level
concept, goes through an ordinary hub pull request that a reviewer merges by
hand.

## Check it

`compass hub check` runs the strict validator, so it fails on a broken link
between repos. It also fails when a repo folder holds no concepts, when an
`index.md` differs from what `okf index .` would write, and when any symlink
exists. It never changes the folder it checks.

`compass sync` runs the check itself before it merges a hub pull request.
A hub pull request merged by hand gets no such check, so the reviewer runs
`compass hub check` on its branch before approving it.

Query the hub with `okf search . --text <terms>` or
`okf backlinks . repos/<id>/<concept>`.

## Run the writers

Both writers run from a hub clone, on demand, with a GitHub token in
`GITHUB_TOKEN` (or the variable named by `--token-env`). They only read the
code repos; every write goes to this hub. Each run is safe to repeat;
`--dry-run` reports what it would do without writing.

- `compass sync` merges an approved hub pull request from `<id>/<b>` once the
  code pull request from `<b>` in repo `<id>` has merged into its default
  branch, the two pull requests are linked, and its own check of the merge
  result finds nothing new. Otherwise it comments on the hub pull request
  and tries again on the next run. It also closes abandoned hub pull requests
  and deletes finished hub branches. Agents run it at the start of each
  session, then `compass hub branch --prune` to remove finished worktrees.
- `compass stamp` commits `process:` stamps for the concepts `checks.txt`
  covers, once their check passed on the code repo's default branch head. It
  pushes straight to this hub's default branch, with one fast-forward per
  run, so the person who runs it needs a ruleset bypass here. Without one,
  GitHub refuses the push, and the stamper says so and exits non-zero.

## Protect it

Every writing developer can push to this hub, so push access no longer marks
a hub owner. The approval before each merge and the review of the control
files hold only with a ruleset on the default branch that:

- blocks direct pushes;
- requires a pull request, with a code-owner approval for the files that
  `CODEOWNERS` covers: `CODEOWNERS`, `repos.txt` and `checks.txt`.

`repos.txt` decides which code repos `compass sync` reads and which hub
branches it may merge, and `checks.txt` decides what gets stamped, so changes
to them need that review. The stamper's runner is the only one that needs a
bypass. `compass sync` merges approved pull requests, which the ruleset
allows.

Rulesets on a private repo need a paid GitHub plan. On GitHub Free they work
only on public repos, so a private hub on Free relies on reviewers following
the rule by hand.
