---
phase: 7
title: "Publish compass"
status: in-progress
priority: P1
effort: 0.5d
dependencies: [5]
---

# Phase 7: Publish compass

## Overview

Outcome: `tuan-nng/compass` is public (plan decision 21), so repos in any account can run `actions/bundle-check` pinned by SHA, and no org name or move is needed.

Publishing exposes the whole git history, so it goes ahead only after two checks pass:
- a secret scan of every commit. No scanner is installed here, so this phase picks one and records it;
- a read of the docs and plans for private repos or people.

Any finding stops the phase (plan Risks: publishing compass).

Owns:

- **The pre-publish checks and their recorded results** in this phase file.
- **The visibility change.**

## Verification

- The chosen secret scanner, run over every commit, reports no findings, and the command and tool version are recorded here.
- `gh repo view tuan-nng/compass --json visibility -q .visibility` prints `PUBLIC`.
- With no credentials, the action's code downloads: `curl -sfL -o /dev/null https://codeload.github.com/tuan-nng/compass/tar.gz/$(git rev-parse origin/master)` exits 0. That is the fetch a runner in another account makes. On 2026-10-07, while compass was private, it exited 22 (HTTP 404), and the same command against the public `tuan-nng/okf` exited 0. The first workflow run on the action happens on a pilot repo (phase 08's first verification check), not on a scratch repo (plan decision 22).

## Pre-publish checks (2026-10-07)

The secret scan is clean. The history read found one problem: the AgentFlow harness was in history. Following the user's decision, history was rewritten to drop it (below). The GitHub side of the rewrite is still open.

- **Secret scan: no findings.** Scanner: gitleaks v8.30.1, run as `go run github.com/zricethezav/gitleaks/v8@v8.30.1 git --log-opts="--all" --redact .`, exit 0, "no leaks found". It scanned 27 of the 28 commits on all refs. The 28th, `59060d6`, only deletes files, so it adds nothing to scan. The `s3cr3t-tok` hits in history are test strings in `test/` that check a token is never printed.
- **Finding: the AgentFlow harness was in history.** The root commit (old `037d770`, 2026-09-29) added AgentFlow 0.14.0's `.agentflow/`, `.claude/`, `.cursor/` and `.omp/` files, and old `59060d6` (2026-10-06) untracked them. Neither commit touched anything else. AgentFlow comes from a private repo, and the installed `af` binary is built from a local, unpublished module, so this content is not public anywhere else. The user chose to rewrite history rather than accept it or start from a single commit.
- **Named but not exposed: the user's former private repos.** History names `tuan-nng/okf-tools`, `tuan-nng/knowledge-hub` and the six `okf-scratch-*` repos, and local paths under `/mnt/data/works/`. All eight repos are deleted (`gh repo view` fails to resolve each), and history holds their names, not their content. `tuan-nng/okf` is already public.
- **People: none.** Every commit's author is the placeholder `Global User <global@test.com>`. Names and emails in test fixtures are made up (`Alice Nguyen <alice@acme.test>`, `bob`, `carol`, `dave` on `acme`). Every other GitHub repo named in the docs is a public project cited in the research.

## History rewrite (2026-10-07)

Local `master` no longer contains the harness. `origin/master` still does, so publishing waits on how the rewritten history reaches GitHub.

- **Backup:** before the rewrite, all refs were saved to `/tmp/compass-pre-rewrite.bundle` (`git bundle verify` passed; old tip `00dcdad`).
- **Rewrite:** done in a separate clone with `uvx git-filter-repo --invert-paths --path .agentflow/ --path .claude/ --path .cursor/ --path .omp/ --path CLAUDE.local.md` (git-filter-repo `a40bce548d2c`). Both harness commits became empty and were pruned, leaving 26 of 28 commits. The new tip `7e4e6a8` has the same tree as the old tip `00dcdad` (`1ed406b`). No object path in the rewritten history is under the four folders, and gitleaks v8.30.1 over all 26 commits reports no leaks. Local `master` was moved to `7e4e6a8` with `git reset --soft`, which keeps the working tree, and the build and `go test ./...` pass.
- **Earlier records keep their old SHAs**, because they record what was true when they were written. For example, the deleted scratch repos pinned `be5b9cf`. The cited ones map as follows:

  | Old | New | Cited in |
  |---|---|---|
  | `c57b3e4` | `b8ead66` | phase 01 |
  | `be5b9cf` | `75e094e` | phases 03 and 04, journal `2026-10-07-setup-go-cache-outside-workspace.md` |
  | `4633439` | `a017c46` | phase 04, plan.md |
  | `9e74c4f` | `fe6965f` | phase 05, plan.md |

- **Decided: force-push only (user, 2026-10-07).** GitHub's docs on removing sensitive data say that after a force-push, the old commits can still be reached through their SHAs in cached views. The user accepted that the harness may stay reachable that way for a while after the repo goes public, and chose not to use a new repo or ask GitHub Support to purge. The repo has no pull requests, forks or Actions runs to hold old refs. The user runs the force-push and the visibility change. The old remote tip is `322786a`.
