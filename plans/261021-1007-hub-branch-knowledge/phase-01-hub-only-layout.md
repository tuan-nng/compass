---
phase: 1
title: "Hub-only layout and removals"
status: completed
priority: P1
effort: 2d
dependencies: []
---

# Phase 1: Hub-only layout and removals

## Overview

Outcome: a hub with knowledge committed under `repos/<id>/` checks clean with `compass hub check`. `repos.txt` has two columns. Every piece of folder mode, branch mode, assembly and the per-repo bundle check is gone.

Sync and stamp change only enough to compile without `Mode`, and their mode-only tests go:
- sync handles every listed repo the way it handled a branch-mode repo;
- stamp handles every listed repo the way it handled a folder-mode repo.

Phase 03 replaces both behaviors; this phase adds none.

Owns:

- **`internal/config`.**
  - **The parser.** `repos.txt` takes two columns, and the id and duplicate rules are those in plan Design, Components. The `Repo.Mode` field is removed.
    - `Mode` readers: 1 in `internal/stamp`, 1 in `internal/syncjob`, 1 test in `internal/config`.
    - `ParseRepos` callers: 1 in `internal/stamp`, 1 in `internal/syncjob`.
  - **New helpers.** The hub-identity helper and the reader for control files on the fetched default branch (plan decision 7). Phases 02 and 03 call them.
  - **Moved helpers.** `githubRepo`, `withoutUserinfo` and `existingClone`'s raw-then-rewritten origin read move here from `internal/setup` (plan decision 12). Their callers: 1 file in `internal/setup`.
- **`internal/hub`.** `compass hub check` meets plan decision 9. Assembly, with its parser and cache, is deleted. `repoDirs`, defined in `check.go`, stays. The check is also callable in process and returns its findings keyed by hub-relative path, so two trees' findings compare; phase 03's sync diffs them (plan decision 11).
- **Removals,** as listed in plan Design: the three commands and their registry entries in `cmd/compass`, the two packages, the action, the four templates and the four suites. `run-all.sh` runs only the remaining suites. The `cmd/compass` summaries of `hub check` and `sync` drop "assembled" and "branch-mode", so phase 03 leaves `cmd/compass` alone.
- **Test wiring for `okf`.** `make test` installs the pinned `okf` into `.cache/pinned/<tag>/`, the folder `test/lib.sh` `okf_bin` uses, before `go test`. Go tests that need `okf` take `$OKF`, else that folder, and fail naming `make test` when neither exists; they never skip. A bare `go test` therefore passes once `make test` has run.
- **Remaining suites and test data.**
  - `test/setup.sh` checks a template hub clone with `compass hub check`, with no assembly.
  - `test/lib.sh` drops the mode helpers.
  - `test/gen-scale.sh` and `test/bench-scale.sh` build the scale hub in the committed `repos/rNN/` layout, with indexes from `okf index` at the hub root. The bench times `compass hub check` as well. `test/scale-counts.sh`, which asserts the generator's `repos/rNN/okf/` paths and its 10,050-file count, follows the new layout.
  - `test/check-fixture.sh` keeps building its hub view with `copy_hub`. Its wording drops "assembled".
  - `testdata/` stays as raw source material for test hubs.
- **`templates/hub/`.** The files change only as far as the format needs:
  - `.gitignore`, whose only line ignores `repos/`, is deleted;
  - the `repos.txt` header shows two columns;
  - `CODEOWNERS` and `index.md` drop the assembly wording.

  Phase 04 rewrites `overview.md`.
- **`internal/setup`.** Help text and messages name no removed command.

Contract changes:

- **`repos.txt` format.** Files that name the three-column form:
  - 2 in `internal/config`, the parser and its test;
  - 2 in `internal/hub`, the assembly parser and its test, both deleted;
  - 2 test files, in `internal/stamp` and `internal/syncjob`;
  - 2 in `templates/hub`;
  - 3 in `test`;
  - 2 in `docs`, which phase 04 owns.
- **`compass hub check`** gains two failures, a stale index and a symlink, and drops one, a missing `repos/`. Its callers:
  - 1 in `cmd/compass`;
  - 1 suite, `test/setup.sh`;
  - the skill and the docs, in phase 04.

## Verification

- `make test` exits 0.
- `make build && bin/compass --help | grep -cE 'bundle check|hub assemble|branch setup'` prints `0`.
- `LC_ALL=C ls actions/bundle-check internal/bundlecheck internal/branchsetup templates/folder-workflow.yml templates/okf-main-workflow.yml templates/hooks templates/AGENTS-pointer.md 2>&1 | grep -c 'No such file'` prints `7`.
- `git grep -lIiE 'okf/main|(folder|branch)[ -]mode|assembl|bundle[ -]check|branch setup|folder\|branch' -- internal cmd templates/hub test testdata ':!internal/syncjob' ':!internal/stamp' ':!templates/hub/overview.md' ':!test/skill-eval.sh' ':!test/skill-eval'` exits 1. Phase 03 clears the two excluded packages, and phase 04 clears the excluded files.
- `go test ./internal/hub/` exits 0 and covers:
  - a committed template hub with no `repos/` passes;
  - a hub whose `repos/<id>/` or `repos/index.md` index is stale fails and names the file, and the folder is unchanged afterwards (`git status --porcelain` is empty);
  - an empty `repos/<id>/` fails;
  - a symlink anywhere in the hub, inside or outside `repos/`, fails.
- `go test ./internal/config/` exits 0 and refuses, naming the line:
  - a three-column line;
  - an id with a leading `.` or a `.lock` suffix;
  - two ids that differ only in case;
  - two rows with the same owner/repo.
- `go test ./internal/setup/` exits 0 with the moved URL helpers.
- `./test/bench-scale.sh` exits 0, with the `hub check` median under 10 s.

## Result (2026-10-21)

Everything above is built, and every check passes. Not committed.

- **Removals.** The 17 files are deleted. `bin/compass --help` lists setup, okf install, validate, hub check, hub branch, stamp, sync and version.
- **Config.** `ParseRepos` reads `<id> <URL>`. Beyond the plan, an id may not start with `-` or contain `..`, so `<id>/<b>` is always a valid branch name. `internal/config/git.go` holds the URL and remote helpers (`WithoutUserinfo`, `GitHubRepo`, `OriginRepos`, `HubRepo`, `RemoteDefault`, `ControlFile`, `HubRepos`, `Fetch`). After review, `WithoutUserinfo` also strips userinfo from a URL that does not parse, so a token in it is never printed.
- **`hub check`.** `hub.Check(dir)` returns findings in this order: symlinks (if any exist, only those, since the other checks would follow them), the validator, per-repo concept counts and empty `repos/<id>/`, then stale or missing indexes from `okf index` on a temp copy. A check that cannot run exits 2, not 1.
- **Tests and suites.** `internal/okftest.Binary` gives Go tests `$OKF` or the pinned `okf`, and `make test` installs it first. `test/gen-scale.sh` writes the hub layout; `test/bench-scale.sh` times `hub check` at a 3.0 s median.
- **Checks.** `make test` passes all Go packages and 5 suites; the removal `ls` prints `7`; the `git grep --untracked` form of the plan's grep exits 1.
