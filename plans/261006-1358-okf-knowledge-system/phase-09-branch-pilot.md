---
phase: 9
title: "Branch-mode pilot"
status: pending
priority: P2
effort: 1.5d + pilot window
dependencies: [8]
---

# Phase 9: Branch-mode pilot

## Overview

Outcome: one real branch-mode repo, named by its maintainers when phase 08 picks the pilot repos, follows the full loop in design section 4.9. The agents that CI and cloud platforms run on it also find its knowledge (plan decision 9). Its first knowledge pull request is the sync job's first live run and proves the merge row live. The other state-table rows are proven only on recorded GitHub responses (phases 02 and 06, plan decision 22), and the pilot meets them as they occur. Its pilot window starts.

Owns:

- **Onboarding the repo,** following design section 4.8's branch-mode steps:
  - `okf/main`, created with `compass branch setup --init`;
  - an `okf/main` ruleset that allows changes only through pull requests and only merge commits. It dismisses stale approvals and lets the hub owner who runs `compass stamp` bypass it (plan decision 13);
  - the `okf/main` workflow from `templates/`;
  - a `repos.txt` row with mode `branch`;
  - a `checks.txt` row where a check exists;
  - each developer running `compass branch setup` and adding the user-level pointer line.
- **CI and cloud agent environments.** `compass setup --yes` runs in each environment the pilot repo uses, and the pointer line goes into each platform's organisation-level instructions.
- **The pilot guide,** extended for branch mode.

## Verification

- `git ls-remote origin refs/heads/okf/main` in the pilot repo prints one line.
- `gh api repos/<owner>/<repo>/rules/branches/okf/main` lists a `pull_request` rule that allows only `merge` and dismisses stale reviews on push.
- On the first approved, green knowledge pull request after its code merged:
  - the writer owner's first `compass sync` run after the code pull request's `mergedAt` merges it (plan decision 12);
  - `gh pr view <n> --json mergedBy -q .mergedBy.login` prints the writer owner's login;
  - its merge commit has 2 parents.
- Where a `checks.txt` row exists, the writer owner's next `compass stamp` run adds a `process:` entry on `okf/main`.
- Three CI or cloud agent runs, on three different tasks, each show branch setup's `okf/ -> okf/main` line in their log, and each cite a concept from `okf/`.
- `compass pilot report --repos pilot.txt --since <start>` prints a number, not `n/a`, for this repo's branch-mode metric.
