---
phase: 2
title: "Stamper and sync job in Go"
status: pending
priority: P1
effort: 2d
dependencies: [1]
---

# Phase 2: Stamper and sync job in Go

## Overview

Outcome: `compass stamp` and `compass sync` make the same decisions as the Python jobs in `okf-tools` commits `783c201` and `b10aaf1`. They are tested against the same recorded GitHub API responses. The fake transport fails any test that makes a request with no recording. The plan's Design states the stamper and sync contracts and the sync job's merge conditions.

The stamper's tested cases:

- it stamps a covered concept when its job succeeded on the head commit, with `at` set to the job's completion time;
- it stamps nothing when the job failed, is missing, ran on another commit, ran from another workflow file, or wasn't a push;
- it never stamps a concept that isn't in `checks.txt`;
- it refreshes an actor entry older than `generated.at`, and skips a concept generated after the job completed;
- it stamps a concept without `generated` once;
- a second run makes no commit when stamps are current;
- a one-line `human:` entry keeps its content beside the new `process:` entry;
- an unsupported `verified` form fails and leaves the concept untouched;
- a missing concept id exits non-zero and names its row;
- it pushes nothing when the branch moved after it was read.

The sync job's tested cases, beyond one test per state-table row:

- a pull request from a fork is ignored;
- an approval from either app, or on an older commit, doesn't count;
- a head with no bundle-check result isn't merged;
- a code pull request merged into a non-default branch counts as not merged;
- an old merged code pull request that doesn't link the knowledge pull request never triggers a merge;
- the merge call passes the checked head SHA;
- a merged pull request is never merged again;
- a second run posts no duplicate comment;
- dry-run makes no API writes.

Owns:

- the stamper, sync and front-matter packages;
- `actions/writer` and the writer workflow template, which move to `templates/hub/` in phase 03;
- the recorded fixtures moved from `okf-tools`.

## Verification

- `go test ./...` exits 0. It covers every case above, including all 27 stamper tests and 25 sync tests in the Python suites at `9dcdd60`.
- `compass stamp --help` and `compass sync --help` list `--hub`, `--dry-run`, `--token-env` and `--api-url`. `sync` also lists `--ignore-login` and `--now`.
- `actionlint` on the writer workflow template exits 0.
