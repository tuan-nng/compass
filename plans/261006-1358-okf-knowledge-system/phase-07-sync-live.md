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

Outcome: the scratch hub's writer workflow runs `compass sync` every 30 minutes, before the stamper, over every branch-mode repo in `repos.txt`. Against a real GitHub repo, the sync job carries out each of the seven rows of the branch-mode state table (research report section 5):

1. it does nothing while the code pull request is open, or closed or missing with `<b>` still on the remote;
2. after the code pull request merged, it merges an approved, green knowledge pull request with a merge commit;
3. after the code pull request merged, it comments on both pull requests when the knowledge pull request is unapproved, failing or conflicting, and tries again next run;
4. after the code pull request merged, it comments on the code pull request when `okf/<b>` has commits not on `okf/main` and no knowledge pull request is open;
5. when the code pull request merged, `<b>` is gone and all of `okf/<b>` is on `okf/main`, it deletes `okf/<b>`;
6. when the code pull request closed unmerged and `<b>` is gone, it closes the knowledge pull request and deletes `okf/<b>`;
7. with no code pull request, `<b>` gone and the last commit on `okf/<b>` older than 7 days, it closes the knowledge pull request and deletes `okf/<b>`.

Each comment is posted once; a hidden marker stops repeats (plan Design, Data).

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
  - `okf/main` still exists;
  - in the row 1 situation, the run makes no write: no merge, comment, close or branch deletion.
- With the schedule on, an approved, green knowledge pull request is merged by the first or second writer run that starts after its code pull request's `mergedAt` (plan decision 12). The check counts runs, not minutes.
