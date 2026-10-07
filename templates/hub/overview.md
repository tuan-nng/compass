---
type: Overview
title: Knowledge hub
description: How to fill in this hub, assemble and check it on your machine, run the stamper and sync job, and protect its control files.
tags: [overview, hub]
status: stable
generated: { by: compass/hub-template, at: 2026-10-07T12:00:00Z }
---
# Knowledge hub

This repo is an OKF knowledge hub made from the compass hub template. It holds
knowledge that spans repos, in [OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md),
and assembles each listed repo's own bundle into `repos/<name>/` (git
ignores that folder). Everything runs on people's machines, with their own
GitHub credentials: the hub has no CI, no workflows and no stored keys.

## Fill it in

- `repos.txt`: one row per repo, `<name> <URL> <folder|branch>`. Any
  `https://github.com/<owner>/<repo>` URL is accepted, so a hub can list repos
  from several accounts.
- `checks.txt`: one row per machine check whose passing run on a repo's
  default branch earns concepts a `verified: process:<actor>` stamp.
- `cross-repo/`: concepts of type `Cross-Repo Dependency`, the only place
  links between repos live. Link a repo's concept from the hub root, as
  `/repos/<name>/<id>.md`.
- `decisions/` and `glossary/`: decisions and vocabulary that span repos.

After adding or changing concepts, delete any assembled `repos/` folder, run
`okf index .` and commit the regenerated `index.md` files. With `repos/`
present, the root index would list it.

## Connect it

1. In `CODEOWNERS`, replace `@OKF_HUB_OWNERS` with the people who approve
   changes to `CODEOWNERS`, `repos.txt` and `checks.txt`.
2. Each developer runs `compass setup` and gives this repo as the hub. Setup
   clones it (by default into `~/src/<repo>`) and records the clone, so the
   commands below need no folder argument when run for that clone.
3. Each listed repo runs the compass bundle check on its pull requests: copy
   compass's `templates/folder-workflow.yml` (folder mode) or
   `templates/okf-main-workflow.yml` (branch mode, on `okf/main` only).

## Check it

Before opening a hub pull request, and before approving one, assemble the
view and check it:

```
compass hub assemble
compass hub check
```

The check fails on a broken cross-repo link. No repo pipeline re-checks the
hub: when a hub pull request links knowledge that hasn't reached a repo's
default branch (or `okf/main`) yet, run both commands again after that change
merges.

Query the assembled view with `okf search . --text <terms>` or
`okf backlinks . repos/<name>/<id>`.

## Run the writers

Hub owners run the writers from their clone, on demand, with a GitHub token in
`GITHUB_TOKEN` (or the variable named by `--token-env`). Each run is safe to
repeat; `--dry-run` reports what it would do without writing.

- `compass stamp` commits `process:` stamps for the concepts `checks.txt`
  covers, once their check passed on the default branch head. It pushes
  straight to each repo's default branch (or `okf/main`), so a protected
  branch needs a bypass for the person who runs it.
- `compass sync` merges approved, green branch-mode knowledge pull requests
  after their code pull request merges, and cleans up `okf/<b>` branches.

## Protect it

`repos.txt` decides what every developer's assembly fetches and `checks.txt`
decides what gets stamped, so changes to them need review:

- a ruleset on the default branch that blocks direct pushes and requires a
  pull request with a code-owner approval;
- `CODEOWNERS` on `CODEOWNERS`, `repos.txt` and `checks.txt`.

Rulesets on a private repo need a paid GitHub plan. On GitHub Free they work
only on public repos, so a private hub on Free relies on reviewers following
the rule by hand.
