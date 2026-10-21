---
phase: 2
title: "compass hub branch and the end-to-end suite"
status: completed
priority: P1
effort: 1.5d
dependencies: [1]
---

# Phase 2: `compass hub branch` and the end-to-end suite

## Overview

Outcome: an agent in a clone of a listed repo runs `compass hub branch` and gets the path of a hub worktree on `<id>/<b>`, and `compass hub branch --prune` removes finished worktrees. Both behave as plan Interfaces and decision 8 define. The command uses git only: no GitHub API and no push. An end-to-end suite proves the local half of plan Acceptance 1.

Owns:

- **The command,** in `internal/hub`, with its registry entry in `cmd/compass`. It matches the origin with phase 01's moved URL helpers and reads `repos.txt` with phase 01's control-file reader (plan decisions 7 and 12).
- **The worktree layout and `--prune`.** A worktree is removed only when all three hold:
  - its branch is gone from the hub remote;
  - its tree is clean;
  - it has no commits that the remote's default branch lacks.

  Otherwise it stays, and its path and reason are printed.
- **`test/hub-branch.sh`,** added to `run-all.sh`. It runs against local bare remotes with `map_owner`'s `insteadOf` rewrite. It covers the command and the onboarding flow, from one `repos.txt` line to a pushed hub branch.
- **The `hub branch` timings** in `test/bench-scale.sh`.

No existing contract changes.

## Verification

`./test/hub-branch.sh` exits 0 and covers:

- **Onboarding:** a hub with one added `repos.txt` row, and a code repo with no compass file, branch or hook. Then:
  - `compass hub branch` prints a path, and `git -C <path> branch --show-current` prints `<id>/<b>`;
  - writing an `overview.md` under `repos/<id>/`, then `okf index .`, changes exactly `repos/<id>/`, `repos/index.md` and the root `index.md`;
  - `compass hub check <path>` exits 0;
  - the branch pushes to the hub remote.
- **Reuse:** a second run prints the same path and creates nothing. A remote `<id>/<b>` that already exists is tracked, not recreated. A kept local `<id>/<b>` is reused.
- **Names:** a branch name with slashes, such as `feat/retry`, works.
- **URL forms:** origin given as `https://github.com/o/r.git`, as `ssh://git@github.com/o/r.git`, as `git@github.com:o/r.git`, and through an `insteadOf` rewrite all map to the same row.
- **Refusals,** each exiting 1 with its cause named:
  - an unlisted origin;
  - a detached HEAD;
  - the repo's default branch, with the hub clone's path in the message;
  - a kept `<id>/feat` blocking `<id>/feat/x`;
  - no recorded hub.
- **Default branch fallback:** with `origin/HEAD` unset, the default branch is still found.
- **Pruning:** after `git push origin --delete <id>/<b>` on the hub remote, `--prune` removes the clean worktree. It keeps one with an uncommitted file and one with an unpushed commit, and prints both paths.

Then:

- `./test/bench-scale.sh` exits 0, with the `hub branch` first run under 5 s and reuse under 1 s.
- `make test` exits 0.

## Result (2026-10-21)

Everything above is built, and every check passes. Not committed.

- `internal/hub/branch.go` holds `compass hub branch`; worktrees live at `<hub-clone>.worktrees/<id>/<b>`. `test/hub-branch.sh` (63 checks) covers onboarding, reuse, the refusals and pruning.
- **Prune choices beyond the plan.**
  - `--prune` also deletes the local branch of each worktree it removes.
  - A worktree whose branch was never pushed and has no commits counts as finished, so it is removed. Nothing is lost, but a parallel session's worktree can go before its first write; that session's next `compass hub branch` opens it again.
  - After review, a worktree holding any ignored file is kept ("it has ignored files, which removing it would delete"), since `git worktree remove` deletes them.
- **Checks.** `./test/hub-branch.sh` and `go test ./internal/hub/` exit 0. `./test/bench-scale.sh`: first run median 0.31 s, reuse 0.028 s. `make test` exits 0.
