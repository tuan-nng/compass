---
phase: 9
title: "Branch-mode pilot"
status: pending
priority: P2
effort: 1.5d + pilot window
dependencies: [6, 8]
---

# Phase 9: Branch-mode pilot

## Overview

Outcome: one real branch-mode repo, named by its maintainers when phase 06 picks the pilot repos (plan decision 13), follows the full loop in design section 4.9. The agents that CI and cloud platforms run on that repo also find its knowledge. Its pilot window starts.

This phase owns:

- **Onboarding the repo,** following the branch-mode steps in design section 4.8:
  - `okf/main`, created with the setup script's `--init`;
  - a ruleset on `okf/main` that:
    - allows changes only through pull requests;
    - allows merge commits only;
    - dismisses approvals when new commits are pushed;
    - lets the writer app bypass it;
  - the `okf/main` workflow file from phase 07;
  - its `repos.txt` row with mode `branch`;
  - a `checks.txt` row where a check exists;
  - each developer running the setup script and adding the user-level pointer line.
- **How CI and cloud agents learn that a repo uses branch mode** (design section 9, item 5). Following plan decisions 14 and 17, the installer runs in every CI and cloud agent environment the pilot repo uses, so the skill, the pinned `okf` and the setup script are there. The pointer line is added to each platform's organisation-level instructions.
- **The pilot guide,** extended for branch mode.

## Verification

- `git ls-remote origin refs/heads/okf/main` in the pilot repo prints one line.
- `gh api repos/<org>/<repo>/rules/branches/okf/main` lists a `pull_request` rule that allows only the `merge` method and dismisses stale reviews on push.
- On the pilot repo's first knowledge pull request after its code merged, `gh pr view <n> --json mergedBy -q .mergedBy.login` prints the writer app's bot login.
- Where the repo has a `checks.txt` row, the writer's next run adds a `process:` entry to a covered concept on `okf/main`. It is committed by the writer app's bot account.
- Three CI or cloud agent runs on the pilot repo, on three different tasks, each show the setup script's `okf/ -> okf/main` line in their log, and each cite a concept from `okf/`.
- `python3 bin/okf-pilot-report --repos pilot.txt --since <start>` prints a number, not `n/a`, for the branch-mode metric of this repo.
