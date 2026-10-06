---
phase: 5
title: "Writer app and check job"
status: pending
priority: P1
effort: 2.5d
dependencies: [4]
---

# Phase 5: Writer app and check job

## Overview

Outcome: the hub writes `verified: process:<actor>` stamps, on a schedule, only on concepts whose named workflow job passed on the repo's default-branch head. It stamps the concepts as they were at that commit. The same logic serves both modes: it commits to the default branch in folder mode and to `okf/main` in branch mode. Plan decision 5 explains why this runs in the hub. The stamper's interface, failure behavior and evidence rules are in the plan's `## Design`.

This phase owns:

- **The move to the company org** (plan decision 12). `okf`, `okf-tools` and `knowledge-hub` move from `tuan-nng` to the company org. Every `uses:` reference, the assembly script's allowed URL prefix, and the `repos.txt` URLs are updated to the new owner. The reader app is re-created there.
- **The writer GitHub App and the `okf-write` environment** in the hub. Only the hub's default branch can use that environment (plan decision 6).
- **The stamper in `okf-tools`.** It edits only the `verified` field. `human:` entries keep their content. It supports the one-line form of `verified` that the test data uses (`verified: { by: …, at: … }`) and the list form. Adding a second entry to the one-line form turns it into a list holding the old entry unchanged, plus the new one. It refuses any other form.
- **The writer composite action in `okf-tools`,** and the hub workflow that runs it every 30 minutes and on manual dispatch. The workflow has one concurrency group that queues runs and never cancels one. Phase 08 adds the sync step, which runs before the stamper.
- **`checks.txt` in the hub,** already under CODEOWNERS since phase 04.
- **Updates to the design doc for plan decision 5.** The "Check job" row in section 2, its row in section 3, and section 5 change: the check job now runs from the hub in both modes. The folder-mode steps in section 4.8 gain the `checks.txt` row and the writer app's ruleset bypass.

## Verification

- `python3 -m unittest discover test/stamp` in `okf-tools` exits 0. It uses recorded GitHub API responses, with a test for each of these cases:
  - a covered concept gets stamped when its job succeeded on the head commit, with `at` set to the job's completion time;
  - nothing is stamped when the job failed, is missing, ran on another commit, or ran from a workflow file other than the named one;
  - a concept not listed in `checks.txt` is never stamped;
  - an entry from the actor older than `generated.at` is refreshed;
  - a concept whose `generated.at` is later than the job's completion time is skipped;
  - a concept without `generated` is stamped once and then left alone;
  - when every stamp is current, a second run makes no commit;
  - a one-line `human:` entry keeps its content when the `process:` entry is added beside it;
  - an unsupported `verified` form fails and leaves the concept untouched;
  - a missing concept id exits non-zero and names its `checks.txt` row;
  - when the branch moved after the stamper read it, it pushes nothing for that repo.
- On a scratch private folder-mode repo whose named job passed, after `gh workflow run writer -R <org>/knowledge-hub`:
  - `git log -1 --format=%an` on the default branch shows the writer app's bot account;
  - `git show --stat HEAD` lists only the covered concepts;
  - `git rev-parse HEAD~1` equals the commit the job ran on.
- A workflow on a non-default hub branch that asks for the `okf-write` environment is refused. `gh run view <id> --json conclusion -q .conclusion` prints `failure`, and the job never receives the key.
- After the move, `gh repo view <company-org>/okf-tools --json owner -q .owner.login` prints the company org. `gh workflow run hub -R <company-org>/knowledge-hub`, then `gh run watch`, ends with `success`.
