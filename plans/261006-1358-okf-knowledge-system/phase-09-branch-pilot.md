---
phase: 9
title: "Branch-mode pilot"
status: pending
priority: P2
effort: 1.5d + pilot window
dependencies: [7, 8]
---

# Phase 9: Branch-mode pilot

## Overview

Outcome: one real branch-mode repo, named by its maintainers when phase 08 picks the pilot repos, follows the full loop in design section 4.9. The agents that CI and cloud platforms run on it also find its knowledge (plan decision 9). Its pilot window starts.

Owns:

- **Onboarding the repo,** following design section 4.8's branch-mode steps:
  - `okf/main`, created with `compass branch setup --init`;
  - an `okf/main` ruleset that allows changes only through pull requests and only merge commits. It dismisses stale approvals and lets the writer app bypass;
  - the `okf/main` workflow from `templates/`;
  - a `repos.txt` row with mode `branch`;
  - a `checks.txt` row where a check exists;
  - each developer running `compass branch setup` and adding the user-level pointer line.
- **CI and cloud agent environments.** `compass setup --yes` runs in each environment the pilot repo uses, and the pointer line goes into each platform's organisation-level instructions.
- **The pilot guide,** extended for branch mode.

## Verification

- `git ls-remote origin refs/heads/okf/main` in the pilot repo prints one line.
- `gh api repos/<org>/<repo>/rules/branches/okf/main` lists a `pull_request` rule that allows only `merge` and dismisses stale reviews on push.
- On the first knowledge pull request after its code merged, `gh pr view <n> --json mergedBy -q .mergedBy.login` prints the writer app's bot login.
- Where a `checks.txt` row exists, the next writer run adds a `process:` entry on `okf/main`, committed by the writer bot.
- Three CI or cloud agent runs, on three different tasks, each show branch setup's `okf/ -> okf/main` line in their log, and each cite a concept from `okf/`.
- `compass pilot report --repos pilot.txt --since <start>` prints a number, not `n/a`, for this repo's branch-mode metric.
