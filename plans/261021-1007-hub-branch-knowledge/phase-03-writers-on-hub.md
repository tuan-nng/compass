---
phase: 3
title: "Writers on the hub"
status: completed
priority: P1
effort: 2d
dependencies: [1]
---

# Phase 3: Writers on the hub

## Overview

Outcome: `compass sync` runs the plan's hub state table, with the merge conditions in plan Trust boundaries and plan decision 11. `compass stamp` commits to the hub (plan decision 10). Neither writes to a code repo, and both read their control files per plan decision 7.

Owns:

- **`internal/syncjob`.**
  - **Two repos per pair.** The syncer talks to the code repo, read-only, and to the hub, found through phase 01's hub-identity helper.
  - **What carries over:** the state logic, comment markers, approval rules, link parsing and merge handling. That covers `links`, `approver`, `hasWrite`, the merge call in `tryMerge`, and the code-pull-request checks in `branch`. Each link direction resolves against the other pull request's repo (plan decision 5).
  - **What changes:**
    - Lookups move from `okf/<b>` branches in the code repo to hub branches `<id>/*`, plus the latest closed hub pull request for the new state-table rows.
    - The check-run gate is replaced by the local consistency check: the path and file-mode rules first, then the merge result and the default head checked in process with `compass hub check`'s logic, comparing their findings.
    - Building the merge result must not run the user's git hooks or filters.
    - Short `#N` links stop counting (plan decision 5).
    - Comments that went to the code pull request go to sync's output instead (plan decision 6).
- **`internal/stamp`.** It reads concepts from the hub and commits them there, at `repos/<id>/<concept>.md`. It makes one fast-forward per run, and its refusal message names the hub branch.
- **Tests.** Sync's tests pair recorded GitHub responses through `ghfake` with a real local hub repo, in a temporary folder, that holds the branches the responses name. That is how the local check runs on real files. `ghfake`'s `WriteHub` and `TemplateHub`, which both packages' tests use and which today write plain files, build that hub: a git clone whose `origin` names a GitHub hub repo, fetched from a local bare remote through an `insteadOf` rewrite (plan decisions 7 and 12). Fixtures in both packages' `testdata/fixtures/` are re-recorded for hub paths. New fixtures copy the shape of the nearest existing one, because no live GitHub run comes before the pilot (old plan decision 22).

Contract changes:

- `syncjob.Run` keeps its single caller, the unexported `run` in `internal/syncjob/cli.go` behind `syncjob.Main`; its arguments may change. `stamp.Run` likewise keeps its caller in `internal/stamp`.
- The sync and stamp command lines are unchanged.
- Their behavior is the contract that phase 04 documents.

## Verification

`go test ./internal/syncjob/ ./internal/stamp/` exits 0 and covers:

- each state-table row in plan Interfaces, with the expected writes. That includes a hub pull request merged by squash and one closed unmerged, each followed by the branch's deletion once `<b>` is gone.
- one refusal for each failed merge condition in plan Trust boundaries:
  - **who and what:** a forked head; an unlisted id; a base other than the default branch;
  - **links:** an unlinked pair; a short `#N` as the only link;
  - **approval:** a stale approval; an approval from a reader;
  - **conflicts:** GitHub reporting a conflict; a local merge conflict;
  - **path rule:** a changed path in `repos/<other>/`; a rename out of `repos/<other>/`; a deletion in `repos/<other>/`; a path under sibling id `a-b` on a branch for id `a`; a change to `checks.txt`; a `.gitattributes` anywhere;
  - **file-mode rule:** a symlink pointing outside the hub, where the comment quotes no file content; a submodule link;
  - **the check:** a failure on the merge result while the branch alone passes, such as the default branch having deleted a concept the branch links to;
  - **timing:** the default head moving between the check and the merge;
- a branch for a new id that changes `repos/<id>/`, `repos/index.md` and the root `index.md` merges;
- a clean branch merges while the default head already carries a finding (plan decision 11);
- `repos.txt` read from the fetched default branch wins over a different working-tree copy;
- the merge passes the checked head SHA, and dry-run makes no write;
- in every test, no recorded write under `/repos/<owner>/<code-repo>/`: a shared assertion fails any test that records one;
- a branch whose code pull request is open costs at most 5 recorded requests;
- stamp writes one `PATCH` of the hub's default ref per run, with tree entries under `repos/<id>/`, makes no commit when stamps are current, and names a refused push.

Then:

- `make test` exits 0.
- `git grep -nIiE 'okf/main|okf/<b>|(folder|branch)[ -]mode|bundle[ -]check' -- internal/syncjob internal/stamp cmd` exits 1.

## Result (2026-10-21)

Everything above is built, and every check passes. Not committed.

- **Sync.** `internal/syncjob` runs the state table against a real local hub. The merge is built with `git merge-tree --write-tree`, with hooks off, in-tree and user attribute files off, and files written raw through `cat-file --batch`. `hub.Check` findings are diffed against the default head (decision 11).
- **Path rule reading.** `repos.txt`, `checks.txt`, `.gitignore` and `.github/` are refused only at the hub root. `CODEOWNERS`, `.gitattributes` and `.gitmodules` are refused at any depth. A bare `repos` path is refused.
- **Review fixes.** An independent review found no blocker; these elevated and minor findings are fixed, each with a test that fails without the fix:
  - the path and file-mode rules also run on the diff from the default head to the merge result, which catches several merge bases and edits that rename detection carries into another repo's folder (`TestRenameOnMainCarryingBranchEditIntoAnotherFolderIsRefused`); plan Trust boundaries now say so;
  - a top-level folder that case-folds to `repos` but is not `repos`, and root control files in any case, are refused (`TestPathRule`, `TestPathRuleRefusals`);
  - after a merge, sync fetches again and resets the default head, so several ready branches merge in one run (`TestTwoReadyBranchesMergeInOneRun`);
  - an unsafe tree path blocks only its branch, with a comment (`TestUnsafePathBlocksOnlyItsBranch`).
- **Stamp.** `internal/stamp` reads code repos and writes one fast-forward of the hub per run, with entries under `repos/<id>/`.
- **Fakes.** `ghfake.WriteHub`/`TemplateHub` build git hubs with an origin; `ghfake.ReadOnly` fails a test on any write under a code repo's path.
- **Checks.** `go test -race ./internal/syncjob/ ./internal/hub/` and `make test` exit 0; the phase grep exits 1.
- **Minimum git.** `merge-tree --write-tree` with `--attr-source` needs git 2.41; `docs/install.md` now says so.
