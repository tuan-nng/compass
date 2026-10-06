---
phase: 7
title: "Branch-mode setup script and okf/main CI"
status: completed
priority: P2
effort: 2d
dependencies: [1]
---

# Phase 7: Branch-mode setup script and okf/main CI

## Overview

Outcome: the branch-mode setup script is ready for real repos, and every case recorded in `docs/research/okf-knowledge-system/evidence/branch-mode.md` runs as an automated test. A workflow stored only on `okf/main` is shown to run on GitHub for pull requests into that branch. That removes the untested assumption in research report section 5, "Review and merge".

This phase owns:

- **The setup script in `okf-tools`,** `bin/okf-branch-setup.sh`, hardened in place from the copy of the sketch (`docs/research/okf-knowledge-system/evidence/okf-branch-setup.sh`) that phase 03 imports (plan decision 17). It keeps the sketch's behavior, including its three refusals: an unknown `origin/HEAD`, a missing `okf/main` without `--init`, and a hook that belongs to another tool. It adds clear failures when:
  - git is older than the minimums in the branch-mode evidence: 2.42 for `--init`, 2.28 for the hooks;
  - the clone has no remote named `origin`;
  - a top-level `okf` path already exists and is not the worktree.

  It must also stay idempotent when run again.
- **A test script** that rebuilds the evidence cases on scratch bare repos:
  - setup and `--init`;
  - a code folder named `okf`;
  - switching branches, a teammate's pushed branch, and uncommitted knowledge edits carried across a switch;
  - deleting a branch, including the two "kept" cases;
  - renaming a branch, a detached HEAD, a second worktree;
  - a default branch named `master`;
  - a hook manager;
  - `git gc`;
  - `git pull` not updating `okf/`.
- **The `okf/main` workflow file template.** It calls the bundle-check action on `.`. A scratch private GitHub repo proves it runs.
- **Updates to the report and the design doc** that turn this assumption into a tested fact: research report section 5, "Review and merge"; design section 2, the "Repo CI" row; and design section 9, item 5.

An optional extra, beyond design section 8's "add the hooks by hand": ready-made hook snippets for husky and lefthook, the two hook managers the report names.

## Verification

- `./test/branch-setup.sh` in `okf-tools` exits 0, and prints a pass line for each evidence case and each new failure case.
- `shellcheck bin/okf-branch-setup.sh` exits 0.
- On a scratch private GitHub repo whose `okf/main` holds the workflow file:
  - a pull request from `okf/test` into `okf/main` makes `gh pr checks <n>` list the bundle check as passed;
  - a second pull request with an out-of-date index makes `gh pr checks <n>` exit non-zero.
