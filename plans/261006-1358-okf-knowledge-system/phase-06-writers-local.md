---
phase: 6
title: "Stamper and sync job run from a developer machine"
status: pending
priority: P1
effort: 2d
dependencies: [4]
---

# Phase 6: Stamper and sync job run from a developer machine

## Overview

Outcome: a person with their own GitHub token runs `compass stamp` and `compass sync` against a temporary local hub folder made from `templates/hub/`, under the personal account, and both do on GitHub what their recorded-API tests showed (plan decision 13). The writers read only `repos.txt` and `checks.txt` from the hub folder (`internal/stamp/stamp.go` `Run`), so no hub repo on GitHub is needed. Phase 04's `tuan-nng/okf-scratch-hub` is to be deleted (plan decision 4).

**The stamper** writes `verified: process:<actor>` stamps only for concepts whose named workflow job passed on the repo's default-branch head. The evidence job is the `okf` workflow's `bundle-check` job on `okf-scratch-bundle-check`, which runs on pushes to `main`.

**The sync job** carries out the rows of the branch-mode state table (research report section 5) against `okf-scratch-branch-mode`:

1. it does nothing while the code pull request is open, or closed or missing with `<b>` still on the remote;
2. after the code pull request merged, it merges an approved, green knowledge pull request with a merge commit;
3. after the code pull request merged, it comments on both pull requests when the knowledge pull request is unapproved, failing or conflicting, and tries again next run;
4. after the code pull request merged, it comments on the code pull request when `okf/<b>` has commits not on `okf/main` and no knowledge pull request is open;
5. when the code pull request merged, `<b>` is gone and all of `okf/<b>` is on `okf/main`, it deletes `okf/<b>`;
6. when the code pull request closed unmerged and `<b>` is gone, it closes the knowledge pull request and deletes `okf/<b>`;
7. with no code pull request, `<b>` gone and the last commit on `okf/<b>` older than 7 days, it closes the knowledge pull request and deletes `okf/<b>`.

Each comment is posted once; a hidden marker stops repeats (plan Design, Data). It merges only under the plan's trust-boundary conditions. It never:

- merges before the code pull request does;
- opens knowledge pull requests;
- touches code branches;
- deletes `okf/main`.

Row 2 needs an approval from a human other than the pull request's author, and the personal account has only one. So row 2 is proven live at the branch pilot (phase 09), and this phase covers rows 1 and 3–7. Its recorded-API tests from phase 02 still cover row 2.

Owns:

- **A temporary stamp hub.** A folder outside the compass repo, copied from `templates/hub/`, never pushed. Its `repos.txt` has one row for `okf-scratch-bundle-check` (folder), and its `checks.txt` has one row for that repo's `bundle-check` job and two of its concepts. The stamper reads only repos named in `repos.txt` (`templates/hub/checks.txt` header). The folder is deleted afterwards.
- **`test/sync-live.sh`.** It builds a temporary hub whose `repos.txt` lists only the repo it is given, in branch mode, because `compass sync` has no repo filter and would otherwise act on every branch-mode repo a hub lists. For each covered row it sets up the situation, runs `compass sync --hub <temp hub>` once with the caller's token, and checks the end state.
- **The stamper's refusal message (plan decision 13).** When branch protection refuses its push, it says so instead of reporting that the branch moved. Today both answers can arrive as HTTP 422 from the ref update, which the stamper reports as "moved" (`internal/stamp/stamp.go`, the `/git/refs/heads/` PATCH). [INFERENCE] GitHub tells them apart only by the message. First, a throwaway public scratch repo with a ruleset that blocks direct pushes shows the status and message GitHub returns. The repo is deleted afterwards.

## Verification

- In the temporary stamp hub, `GITHUB_TOKEN=$(gh auth token) compass stamp --hub .` exits 0. Then, on `okf-scratch-bundle-check`:
  - `git log -1 --format=%ae` shows the token account's commit email;
  - `git show --stat HEAD` lists only the covered concepts;
  - `git rev-parse HEAD~1` equals the commit the job ran on.
- A second `compass stamp` run makes no commit.
- `GITHUB_TOKEN=$(gh auth token) ./test/sync-live.sh tuan-nng/okf-scratch-branch-mode` exits 0. For rows 1 and 3–7 it checks that:
  - each expected comment appears exactly once;
  - deleted branches are gone from `git ls-remote`;
  - `okf/main` still exists;
  - in the row 1 situation, the run makes no write: no merge, comment, close or branch deletion;
- `compass stamp` on the throwaway protected repo exits non-zero, and its log names the protection instead of "moved". `gh repo view` on that repo then exits non-zero.
