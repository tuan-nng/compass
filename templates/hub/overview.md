---
type: Overview
title: Knowledge hub
description: How to fill in this hub, connect it to its repos, and protect the files that decide what the writer key touches.
tags: [overview, hub]
status: stable
generated: { by: compass/hub-template, at: 2026-10-06T12:00:00Z }
---
# Knowledge hub

This repo is an OKF knowledge hub made from the compass hub template. It holds
knowledge that spans repos, in [OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md),
and assembles each listed repo's own bundle into `repos/<name>/` (git
ignores that folder).

## Fill it in

- `repos.txt`: one row per repo, `<name> <URL> <folder|branch>`. Only
  `https://github.com/<org>/` URLs of the org that owns this hub are accepted.
- `checks.txt`: one row per machine check whose passing run on a repo's
  default branch stamps concepts `verified: process:<actor>`.
- `cross-repo/`: concepts of type `Cross-Repo Dependency`, the only place
  links between repos live. Link a repo's concept from the hub root, as
  `/repos/<name>/<id>.md`.
- `decisions/` and `glossary/`: decisions and vocabulary that span repos.

After adding or changing concepts, run `okf index .` and commit the
regenerated `index.md` files.

## Connect it

1. In `.github/workflows/hub.yml` and `.github/workflows/writer.yml`, replace
   `OKF_ORG` with the account that owns compass and `COMPASS_SHA` with the
   full commit SHA of the compass version to use.
2. In `CODEOWNERS`, replace `@OKF_HUB_OWNERS` with the people who approve
   changes to the workflows, `CODEOWNERS`, `repos.txt` and `checks.txt`.
3. Create the reader app, which hub CI uses to read the listed repos:
   `compass app create --role reader --hub <org>/<this-repo>`. Install it on
   the repos in `repos.txt` only.
4. Create the writer app, which the writer workflow uses to merge knowledge
   pull requests and push `process:` stamps:
   `compass app create --role writer --hub <org>/<this-repo>`. It creates the
   `okf-write` environment, limited to this repo's default branch, and keeps
   the app's key there.
5. Each developer runs `compass setup` and gives this repo as the hub.

## Protect it

The writer key can push to every listed repo, so the files that decide what
it runs and touches need review:

- a ruleset on the default branch that blocks direct pushes and requires a
  pull request with a code-owner approval;
- `CODEOWNERS` on `.github/`, `CODEOWNERS`, `repos.txt` and `checks.txt`;
- the `okf-write` environment, usable only from the default branch.

Rulesets and environments on a private repo need a paid GitHub plan (Team or
Enterprise). On GitHub Free they work only on public repos, so a private hub
on Free has neither protection; don't create the writer app there.

## Use it

Assemble the view locally with `compass hub assemble .`, then query it with
`okf search . --text <terms>` or `okf backlinks . repos/<name>/<id>`. Hub CI
runs the same assembly nightly, on manual dispatch and on pull requests. No
repo pipeline triggers it: when a hub pull request links knowledge that hasn't
reached a repo's default branch (or `okf/main`) yet, re-run its check after
that change merges.
