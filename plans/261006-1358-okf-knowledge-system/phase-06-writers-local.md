---
phase: 6
title: "Stamper refusal and writer checks on folders"
status: pending
priority: P1
effort: 0.5d
dependencies: [4]
---

# Phase 6: Stamper refusal and writer checks on folders

## Overview

Outcome: `compass stamp` tells a refused push apart from a branch that moved (plan decision 13), and both writers are proven against a hub folder copied from `templates/hub/`. Every check runs on folders and recorded GitHub responses, with no GitHub repo (plan decision 22).

The writers read only `repos.txt` and `checks.txt` from the hub folder (`internal/stamp/stamp.go` `Run`). Every other input comes from the GitHub API, which the Go tests already serve from recorded responses through `ghfake` (22 stamper tests and 25 sync tests from phase 02, covering all seven state-table rows). So this phase adds no live run. The first live stamp and sync runs happen on the pilot repos: phase 08 checks the stamp commit, and phase 09 the sync merge.

Owns:

- **The stamper's refusal message (plan decision 13).** Today a 409 or 422 on the ref update is always reported as "moved" (`internal/stamp/stamp.go`, the `/git/refs/heads/` PATCH). After a refused update, the stamper reads the branch head again:
  - if the head is no longer the commit it read, it reports "moved", as now;
  - if the head is unchanged, nothing raced it. It reports that GitHub refused the push, quotes GitHub's message, names the likely cause (branch protection or a ruleset without a bypass for this account), and exits non-zero.

  Rejected alternative: matching GitHub's message text. The text for a ruleset refusal is unknown without a live protected repo, and GitHub can change it. The head re-read works whatever the text says.
  Rejected alternative: matching GitHub's message text. The text for a ruleset refusal is unknown without a live protected repo, and GitHub can change it. The head re-read works whatever the text says. GitHub documents only 200, 409 and 422 for this endpoint ("Update a reference", REST API docs for Git references, API version 2022-11-28), so the re-read covers every documented refusal. Any other status still exits non-zero and names the repo, as now. If the re-read itself fails, the stamper exits non-zero and names the repo, like any other API error (plan Design, Failure modes).

## Verification

- `go vet ./... && go test ./...` and `./test/run-all.sh` exit 0.
- A stamper test with a refused ref update and an unchanged head exits non-zero and logs GitHub's message and the protection cause, not "moved". With the head changed, the existing "moved" test still passes (`TestBranchMovedAfterReadPushesNothing`).
- Neither test retries or forces the push: `ghfake`'s write log shows one tree, one commit and one refused ref update.
- The template-hub test passes for both writers, and `git status --short templates/hub` prints nothing after it runs.
- `grep -rn 'okf-scratch' internal test cmd templates skill` exits 1.
