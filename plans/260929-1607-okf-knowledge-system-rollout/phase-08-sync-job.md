---
phase: 8
title: "Sync job"
status: pending
priority: P2
effort: 3d
dependencies: [5, 7]
---

# Phase 8: Sync job

## Overview

Outcome: the hub's writer workflow runs the sync job every 30 minutes, before the stamper, over every branch-mode repo in `repos.txt`. The job carries out each row of the state table in research report section 5, "Branch mode":

- it merges approved, green knowledge pull requests with a merge commit;
- it comments when a knowledge pull request is missing, unapproved or failing;
- it deletes `okf/<b>` once its code branch is gone and its work has reached `okf/main`;
- it closes the knowledge pull request and deletes `okf/<b>` when the code pull request closed without merging and `<b>` is gone;
- it closes abandoned knowledge pull requests after 7 days.

It merges only when every condition under "Merges by the sync job" in the plan's trust boundaries holds. Its interface and failure behavior are in the plan's `## Design`.

This phase owns:

- the sync job in `okf-tools`;
- its step in the writer composite action;
- a live test script that runs against a scratch private GitHub repo in branch mode.

The job must never do what design section 3 forbids:

- merge a knowledge pull request before its code pull request merges, or without a human approval;
- open knowledge pull requests itself;
- touch code branches;
- delete `okf/main`.

## Verification

- `python3 -m unittest discover test/sync` in `okf-tools` exits 0. It runs against recorded GitHub API responses, with one test for each state-table row, plus these:
  - a pull request from a fork is ignored;
  - an approval from either app, or on a commit older than the head, is not counted;
  - a knowledge pull request with no bundle-check result on its head is not merged;
  - a code pull request merged into a branch other than the default is treated as not merged;
  - with a reused branch name, an old merged code pull request that does not link the knowledge pull request never triggers a merge;
  - the merge call passes the checked head commit, so a push after the check makes the merge fail;
  - a merged pull request is never merged again;
  - a second run posts no duplicate comment;
  - dry-run makes no API writes.
- `./test/sync-live.sh <org>/<scratch-repo>` exits 0. It sets up each row's situation, runs the job once, and checks the end state:
  - the merged knowledge pull request's merge commit has 2 parents;
  - each expected comment appears exactly once;
  - deleted branches are gone from `git ls-remote`;
  - `okf/main` still exists.
- In the live test, with the schedule on, an approved, green knowledge pull request is merged by the first or second writer run that starts after its code pull request's `mergedAt` (plan decision 9). GitHub can delay scheduled runs, so the check counts runs, not minutes.
