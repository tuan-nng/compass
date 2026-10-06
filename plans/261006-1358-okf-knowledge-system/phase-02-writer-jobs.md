---
phase: 2
title: "Stamper and sync job in Go"
status: completed
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
- the recorded fixtures moved from `okf-tools` (`test/stamp/fixtures/` and `test/sync/fixtures/`).

The writer workflow template moves with `templates/` in phase 01 and into `templates/hub/` in phase 03. Phase 04 rewrites `actions/writer` to build `compass`.

## Verification

- `go test ./...` exits 0. It covers every case above, including all 27 stamper tests (21 recorded-API, 6 front-matter) and 25 sync tests in the Python suites at `9dcdd60`.
- `compass stamp --help` and `compass sync --help` list `--hub`, `--dry-run`, `--token-env` and `--api-url`. `sync` also lists `--ignore-login` and `--now`.

## Result (2026-10-06)

Done; nothing is committed yet. All 27 stamper tests and 25 sync tests pass as Go tests on the moved fixtures. `compass stamp --help` and `compass sync --help` list the flags above.

An independent review of the port against the Python jobs found one blocker and three smaller differences. All are fixed, and each fix has a test that fails without it:

- **Blocker — the stamper accepted any actor.** The port dropped the check that a `checks.txt` actor is `process:<name>`. A row with `human:alice` would have made the writer app commit a `human:` stamp, which only review may grant, and a quoted actor could inject text into the `verified` entry. The check is back. `TestChecksRowActorMustBeProcess` shows both rows exit 2 with no API writes.
- **Sync read `#010` as PR 8.** Go's `fmt.Sscan` reads a leading zero as octal. It also missed non-ASCII digits that Python's `\d` matches. `links` now reads digits as Python's `int()` does; `TestLinksReadsReferencesAsPythonDoes` uses values taken from the Python function.
- **Sync treated `"head_sha": null` as a match.** Python only matches a missing key. Fixed; GitHub never sends null here, so no live decision changed.
- **Sync had its own, narrower time parser** for `--now` and commit dates. It now uses the stamper's `frontmatter.ParseTime`, which matches Python's `parse_time`.

Usage lines now start `compass stamp` and `compass sync`.
