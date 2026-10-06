---
phase: 7
title: "Sync job live"
status: pending
priority: P2
effort: 1.5d
dependencies: [6]
---

# Phase 7: Sync job live

## Overview

Outcome: the scratch hub's writer workflow runs `compass sync` every 30 minutes, before the stamper, over every branch-mode repo in `repos.txt`. Against a real GitHub repo, the sync job carries out each row of the branch-mode state table (research report section 5):

- it merges approved, green knowledge pull requests with a merge commit;
- it comments once when a knowledge pull request is missing, unapproved or failing;
- it deletes `okf/<b>` once its code branch is gone and its work has reached `okf/main`;
- it closes the knowledge pull request and deletes `okf/<b>` when the code pull request closed unmerged and `<b>` is gone;
- it closes abandoned knowledge pull requests after 7 days.

It merges only under the plan's trust-boundary conditions. It never:

- merges before the code pull request does;
- opens knowledge pull requests;
- touches code branches;
- deletes `okf/main`.

Owns:

- the `okf/main` ruleset on a scratch branch-mode repo in the company org;
- `test/sync-live.sh`, which sets up each row's situation, runs the job once, and checks the end state.

## Verification

- `./test/sync-live.sh <company-org>/<scratch-branch-repo>` exits 0. It checks that:
  - the merged knowledge pull request's merge commit has 2 parents;
  - each expected comment appears exactly once;
  - deleted branches are gone from `git ls-remote`;
  - `okf/main` still exists.
- With the schedule on, an approved, green knowledge pull request is merged by the first or second writer run that starts after its code pull request's `mergedAt` (plan decision 12). The check counts runs, not minutes.
