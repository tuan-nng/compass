---
phase: 6
title: "Company org, writer app and check job"
status: pending
priority: P1
effort: 2d
dependencies: [5]
---

# Phase 6: Company org, writer app and check job

## Overview

Outcome: compass and the fork live in the company org, and the hub writes `verified: process:<actor>` stamps on a schedule (plan decisions 13 and 14). It stamps only concepts whose named workflow job passed on the repo's default-branch head.

This phase waits for the company org (plan decision 7, and the GitHub Free risk). The org's plan supports rulesets and environments on private repos, so the two checks deferred under GitHub Free run here.

Owns:

- **The org move.**
  - `compass` and `okf` move to the company org, and `config.env` takes the new name.
  - The pin's release repo follows from `OKF_ORG`.
  - The scratch hub and scratch repos are re-created in the org, or moved there, and their `uses:` references and `repos.txt` URLs are updated.
  - The reader app is re-created there.
- **Hub protections on the scratch hub:**
  - the default-branch ruleset that requires a code-owner approval;
  - the `okf-write` environment, limited to the default branch;
  - the writer app, created with `compass app create --role writer`.
- **A `checks.txt` row** on the scratch hub, for a scratch folder-mode repo whose named job runs on pushes to the default branch.
- **Doc changes for plan decision 13.** The check job now runs from the hub in both modes:
  - the "Check job" rows in design sections 2 and 3;
  - research report section 5, where `okf/main` protection excepts "the check job's `verified: process:` commits, as in folder mode";
  - the folder-mode steps in design section 4.8 gain the `checks.txt` row and the writer app's ruleset bypass.

## Verification

- `gh repo view <company-org>/compass --json owner -q .owner.login` prints the company org.
- `gh workflow run hub -R <company-org>/<scratch-hub>`, then `gh run watch <id> --exit-status -R <company-org>/<scratch-hub>`, exits 0.
- A hub PR that changes `.github/workflows/` shows `reviewDecision` = `REVIEW_REQUIRED` until a code owner approves (`gh pr view <n> --json reviewDecision`).
- A workflow on a non-default hub branch that asks for `okf-write` is refused: `gh run view <id> --json conclusion -q .conclusion` prints `failure`, and the job never receives the key.
- After `gh workflow run writer -R <company-org>/<scratch-hub>`, on the scratch folder-mode repo:
  - `git log -1 --format=%an` shows the writer app's bot;
  - `git show --stat HEAD` lists only the covered concepts;
  - `git rev-parse HEAD~1` equals the commit the job ran on.
- A second writer run makes no commit.
