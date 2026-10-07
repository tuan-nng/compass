---
phase: 4
title: "Local-first cut and the live bundle check"
status: completed
priority: P1
effort: 2.5d
dependencies: [1, 2]
---

# Phase 4: Local-first cut and the live bundle check

## Overview

Outcome: compass runs one thing on GitHub Actions, the bundle check on repo pull requests (plan decisions 14–16). Hub CI, the writer workflow and the GitHub Apps are gone from compass. The compass code through phase 03 is already on the private `tuan-nng/compass` (`master`, commit `4633439`), and this phase pushes its own changes there. Its Actions access changes from `none` to the account's private repos, so the scratch repos can call `actions/bundle-check`.

A scratch hub, `tuan-nng/okf-scratch-hub`, is made the way an end user would make one (plan decision 4):

1. copy `templates/hub/`;
2. list `okf-scratch-billing-api`, `okf-scratch-shared-auth` and `okf-scratch-web-app` in `repos.txt`;
3. copy in the prototype's cross-repo concepts from `testdata/proto`, so there are links to check.

It needs no app and no workflow. It is assembled and checked on this machine with the user's own git credentials.

Owns:

- **The local-first cut.**
  - `templates/hub/` loses its `.github/` workflows. Its overview concept, `CODEOWNERS` and `checks.txt` headers stop describing hub CI, the apps, the writer key and the `okf-write` environment. They describe local assembly and checks, and hub owners running `compass sync` and `compass stamp`.
  - `compass app create` and the `internal/app` package go. Their one caller is `cmd/compass/main.go`.
  - The skill re-checks a waiting hub pull request locally instead of re-running its CI (decision 15). It names hub owners running the stamper as the writer of `process:` stamps.
- **Setup points at repos, not an org (decisions 3 and 7).**
  - `compass setup` takes the hub repo and a clone folder, reuses or makes the clone, and records both. `hub assemble`, `hub check`, `stamp` and `sync` default to that clone when given no hub folder.
  - `OKF_ORG`, `config.env` and `--org` go. `repos.txt` accepts any GitHub owner, and `pins/okf.env` names the fork's release repo.
  - The changed contract has these callers: `config.Org` (5 call sites, one each in `internal/hub`, `internal/okfinstall`, `internal/setup`, `internal/stamp` and `internal/syncjob`) and `config.ParseRepos` (2, in `internal/stamp` and `internal/syncjob`). `OKF_ORG` or `--org` also appears in 16 files: 7 in `internal`, 5 in `templates` and 4 in `test`.
- **`actions/bundle-check`.** It sets up Go with a SHA-pinned `setup-go`, with its cache off (plan decision 16), then builds `compass` and runs `compass bundle check`.
- **Pushing this phase's changes to compass.** Before each push, `git ls-files` must list only what compass means to publish (plan Risks). On 2026-10-06 it listed no harness folder and no `CLAUDE.local.md`.
- **`tuan-nng/okf-scratch-hub`.**
- **The `okf.yml` workflows in the five scratch repos, now pinned to compass's action by SHA.** They live on `main` in `okf-scratch-bundle-check`, `okf-scratch-billing-api` and `okf-scratch-shared-auth`, and on `okf/main` in `okf-scratch-branch-mode` and `okf-scratch-web-app`.

The doc changes for decisions 13–16 belong to phase 05.

## Verification

- `go vet ./... && go test ./...` and `./test/run-all.sh` exit 0.
- `compass app create` exits non-zero with an unknown-command error.
- `test -e templates/hub/.github` exits 1.
- `grep -rniE 'app create|writer app|reader app|writer key|okf-write|hub CI|gh run rerun|check job|org that owns|<org> owns' --exclude-dir=testdata --exclude='*_test.go' templates skill internal cmd` exits 1. On 2026-10-06 it matched 29 lines outside `internal/app`, including `compass hub check --help` ("as hub CI does") and the skill's "the check job".
- `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 templates/*.yml` exits 0; the repo workflow templates replace phase 03's hub workflows as actionlint's input.
- `gh api repos/tuan-nng/compass/actions/permissions/access -q .access_level` prints `user`.
- On `okf-scratch-bundle-check`:
  - a new push run concludes `success`;
  - a PR that adds a concept without `okf index` concludes `failure` with "index files are out of date";
  - the "build compass" step takes at most 60 s.
- On `okf-scratch-branch-mode`, the `okf` workflow on a PR into `okf/main` concludes `success`.
- `gh api repos/tuan-nng/okf-scratch-hub/contents/.github` exits non-zero: the hub has no workflows.
- `compass setup --hub tuan-nng/okf-scratch-hub --hub-dir "$d" --yes` exits 0, with `d` a new temp folder, and leaves a clone of the hub there. A second run reuses it and reports each part current. `okf --version | jq -r .version` prints `0.5.0-tuan-nng.1`.
- In that clone, after setup, `compass hub assemble --ci && compass hub check` exits 0 with no folder argument, and `compass stamp --dry-run` and `compass sync --dry-run`, run without `--hub`, read that clone's `repos.txt`.
- On a branch of that clone, a cross-repo concept that links a concept missing from every repo makes `compass hub check` exit non-zero.
- `test -e config.env` exits 1, and `grep -rnE 'OKF_ORG|--org' internal cmd templates test skill pins assets.go` exits 1.

## Result (2026-10-07)

Every check above passes. Compass `be5b9cf` is on `tuan-nng/compass` `master`, and the five scratch repos pin its action to that SHA.

**Code (compass `be5b9cf`):**
- `compass app create`, `internal/app`, `templates/hub/.github/`, `config.env`, `OKF_ORG` and `--org` are gone. Caller counts matched the plan: 5 `config.Org` sites, 2 `config.ParseRepos` callers, 16 files, 1 `internal/app` caller.
- `config.ParseRepoURL` accepts any `https://github.com/<owner>/<repo>[.git]` and rejects `.` and `..` as repo names. `repos.txt`, `hub assemble` and the writers all use it. The stamper compares the run's repo name case-insensitively, since the owner now comes from `repos.txt` as written.
- `pins/okf.env` names `OKF_RELEASE_REPO=tuan-nng/okf`, and `compass okf install` requires it.
- `compass setup --hub OWNER/REPO [--hub-dir DIR]`:
  - It normalizes `owner/repo.git` to `owner/repo`.
  - The folder defaults to the recorded folder of the same hub, else `~/src/<repo>`.
  - It refuses, before writing anything, a non-empty folder that isn't a clone of the hub. It compares the raw `remote.origin.url`, then the `insteadOf`-rewritten one. It strips any user and token from the URL before comparing or printing.
  - It clones over https and records `OKF_HUB` and `OKF_HUB_DIR`. An unreadable user config stops it with nothing written.
- `hub assemble`, `hub check`, `stamp` and `sync` use `OKF_HUB_DIR` when given no folder.
- The skill and the hub template describe local assembly and checks, hub owners running `compass stamp` and `compass sync`, and running `okf index .` without a `repos/` folder. With `repos/` present, `okf index .` lists it in the root index; this was checked by hand.

**Local checks:**
- `go vet ./... && go test ./...` and `./test/run-all.sh` (7 suites) exit 0. The setup suite now covers:
  - a default `~/src/<repo>` clone;
  - the hub commands with no folder;
  - reuse of an ssh clone and of a token clone;
  - a busy folder, a clone of another repo, and `--hub acme/..`;
  - no token printed in any of these.
- `compass app create` exits 2 with "unknown command". `test -e templates/hub/.github` and `test -e config.env` exit 1.
- The hub-CI grep and the `OKF_ORG|--org` grep both exit 1. actionlint on `templates/*.yml` exits 0.
- The compass build compiles under Go 1.23, the version `go.mod` names and setup-go installs.
- `./test/skill-eval.sh --agent claude --scenario cross-repo --runs 1` passes with the changed skill.

**On GitHub:**
- The compass Actions access level is `user`.
- All five scratch push runs on compass's action conclude `success`: 37589450096, 37589453789, 37589458112, 37589461413 and 37589465416.
- `okf-scratch-bundle-check` PR #2 adds a concept without `okf index`. Run 37589606625 concludes `failure` with "index files are out of date".
- The "go build" step takes 19.3 s on the push run and 18.9 s on the PR run, uncached.
- `okf-scratch-branch-mode` PR #3 into `okf/main` concludes `success` (run 37589632769). Both PRs were closed and their branches deleted.
- `tuan-nng/okf-scratch-hub` (private) was made from `templates/hub/` plus three `repos.txt` rows and the prototype's `cross-repo/` and `decisions/`. It has no `.github`.
- Live setup ran with a scratch `HOME` and gh's git credential helper:
  - `compass setup --hub tuan-nng/okf-scratch-hub --hub-dir "$d" --yes` cloned the hub. The second run reported okf, the clone, the config and the skill as current. `okf --version | jq -r .version` printed `0.5.0-tuan-nng.1`.
  - In the clone, `compass hub assemble --ci && compass hub check` exits 0: 3 repos, 13 concepts.
  - From `/tmp`, without `--hub`, `compass sync --dry-run` logged `web-app: no okf/<b> branches`. `compass stamp --dry-run`, with one uncommitted `checks.txt` row, logged `would stamp contracts/invoice-api` for `billing-api`.
  - On a local branch of the clone, a cross-repo link to `/repos/billing-api/contracts/missing.md` makes `compass hub check` exit 1. The strict validator fails on that warning.

**Deviations and findings:**
- **The setup-go cache is off.** Decision 16 said it was on, but setup-go hashes only files under `GITHUB_WORKSPACE`, so it can't key on the action's own `go.sum`. Plan decision 16 and Risks are updated.
- **Container jobs:** the action reads its path from `$GITHUB_ACTION_PATH` in a run step, not from `github.action_path` in `with:`, which names the host path in container jobs (actions/runner#2185).
- **Found, not fixed (outside this phase):** `fetchBundle` (`internal/hub/assemble.go`) compares `git remote get-url origin`, which applies `insteadOf` rewrites, with the `repos.txt` URL. With any `insteadOf` rule, such as https-to-ssh or the test suites' file mappings, the cache never matches, and every run clones from scratch. This likely explains why `bench-assemble.sh` measured the same 5.0 s cold and warm. The fix, `git config --get remote.origin.url`, belongs in `/af:fix`.
