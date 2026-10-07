---
phase: 6
title: "Stamper refusal and writer checks on folders"
status: completed
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

  Rejected alternative: matching GitHub's message text. The text for a ruleset refusal is unknown without a live protected repo, and GitHub can change it. The head re-read works whatever the text says. GitHub documents only 200, 409 and 422 for this endpoint ("Update a reference", REST API docs for Git references, API version 2022-11-28), so the re-read covers every documented refusal. Any other status still exits non-zero and names the repo, as now. If the re-read itself fails, the stamper exits non-zero and names the repo, like any other API error (plan Design, Failure modes).
- **Writers on a template hub.** A Go test copies `templates/hub/` into a temporary folder, adds one `branch`-mode `repos.txt` row and one `checks.txt` row under the shipped headers, and runs `compass stamp` and `compass sync` against it with recorded responses. The row is `branch` mode because `compass sync` skips folder-mode repos; the stamper then stamps `okf/main`. This proves the shipped template's control files parse as the writers expect.

## Verification

- `go vet ./... && go test ./...` and `./test/run-all.sh` exit 0.
- A stamper test with a refused ref update and an unchanged head exits non-zero and logs GitHub's message and the protection cause, not "moved". `TestBranchMovedAfterReadPushesNothing` serves a different head on the re-read, and still exits 0 and logs "not pushed: main moved". Today that test registers one ref route that always returns the same head, so without that change it would take the refusal path.
- Neither test retries or forces the push: `ghfake`'s write log shows one tree, one commit and one refused ref update.
- The template-hub test passes for both writers, and `git status --short templates/hub` prints nothing after it runs.
- `grep -rn 'okf-scratch' internal test cmd templates skill` exits 1.

## Result (2026-10-07)

Everything above is built, and every check passes.

- **Refusal.** After a 409 or 422 on the ref update, `stampRepo` (`internal/stamp/stamp.go`) reads the branch head again. If it moved, the log says "not pushed: <branch> moved", as before, and the run exits 0. If it didn't, the row line ends "(not pushed: refused)". A `stamp: error: <repo>: GitHub refused the update to <branch> at <sha> with HTTP <status> "<message>"` line follows. It says that branch protection or a ruleset likely blocks this account, which needs a bypass, and the run exits 1. If the re-read fails, the repo fails with `stamp: error: <repo>: …`, like any API error.
- **Tests.**
  - `TestBranchMovedAfterReadPushesNothing` now serves the read head once and a new head on the re-read.
  - `TestRefusedPushWithUnchangedHeadExitsNonzero` covers 409 and 422 with a synthetic message, so it doesn't depend on GitHub's wording.
  - `TestRefusedPushThenFailedReReadExitsNonzeroNamingRepo` covers the failed re-read.
  - These three tests assert one tree, one commit and one ref update.
  - `TestBranchModeRefusedPushOnOkfMainExitsNonzero` refuses `okf/main`. If the re-read compares against the default-branch head instead of the `okf/main` head it read, this test fails; a temporary edit confirmed that.
- **Template hub.**
  - `ghfake.TemplateHub` finds the module root through `go.mod`, so it also works under `-trimpath`. It copies `templates/hub/` into a temp folder and appends one row to each control file.
  - `TestBranchModeChecksDefaultHeadAndStampsOkfMain` now runs on a bare hub and on the template hub.
  - `TestRow2OnTemplateHubMergesKnowledgePr` runs sync on the template hub.
  - Each package uses its own fixture repo (`billing-api` for stamp, `web-app` for sync), so the two rows name different repos.
- `templates/hub/overview.md` now says the stamper reports a refused push and exits non-zero. `internal/github/github.go`'s package comment no longer mentions a scheduled rerun.
- **Checks.**
  - `go vet ./... && go test ./...` passes, and so does `go test -trimpath` on both writer packages.
  - `./test/run-all.sh` passes all 7 suites.
  - The `okf-scratch` grep exits 1.
  - The checksums of `templates/hub` are the same before and after the tests. `git status` shows it changed only because of the `overview.md` edit above.
- An independent review found no blockers. Its minor code findings are fixed: the separate error line, the branch-mode test, the `stampRepo` comment, the `-trimpath` lookup and the stale comment.
