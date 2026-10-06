---
title: "OKF knowledge system: compass CLI, hub template and pilot"
description: "People clone compass, build one compass binary, connect it to their own knowledge hub, and pilot the OKF knowledge system on real repos, folder mode first, then branch mode."
status: in-progress
priority: P1
effort: 17d build + 4-week pilot per mode
branch: master
tags: [okf, knowledge, agents, tooling, go, cli]
created: 2026-10-06
---

# OKF knowledge system: compass CLI, hub template and pilot

## Overview

**Problem:** The design in `docs/design/okf-knowledge-system-ux.md` is complete on paper, but most of it doesn't run yet. What runs today is a set of bash and Python scripts spread over a separate tooling repo, `tuan-nng/okf-tools`, plus a hub repo the project owns, `tuan-nng/knowledge-hub`. There is no single entry point.
**Change:** Put all tooling into compass as one Go binary, `compass`. People clone compass, build it, and run `compass setup`, which asks for the knowledge hub they provide. Compass ships a hub template instead of owning a hub. Then finish the remaining pieces (the skill, the check job, the sync job) and pilot the system, folder mode first and branch mode later.
**Why this way:** The user chose compass as the only code home, a Go binary, and an end-user hub (decisions 1–4). The strongest rejected alternative kept the tested scripts behind a thin `compass` wrapper. It needs no rewrite, but users would need bash, Python and uv, and there would be no build step. The main risk is behavior lost in the rewrite, so the existing test suites carry over as end-to-end checks against the binary.

The work is grouped into five milestones. Each one leaves something usable behind:

| Milestone | What works when it closes |
|---|---|
| M1 One tool | `compass` builds from a clone, replaces every script, and passes every test the scripts passed. |
| M2 Live on GitHub | `compass setup` connects a machine to a hub. A hub made from the template, and the scratch repos, run green on compass actions. `okf-tools` and `knowledge-hub` are gone. |
| M3 Writers live | In the company org, the check job writes `process:` stamps and the sync job merges knowledge pull requests. |
| M4 Folder-mode pilot | 1–2 real folder-mode repos are live, and pilot metrics are collected. |
| M5 Branch mode and read-out | A branch-mode repo is live, and the read-out decides whether to roll out further. |

This plan replaces two earlier plans, merged on 2026-10-06: `260929-1607-okf-knowledge-system-rollout` and `261006-1351-compass-go-cli`. Git history keeps both.

## Landed before this plan

All of this was verified on 2026-10-06. Code paths refer to `okf-tools`, which phase 01 moves into compass.

- **Tooling foundation (`okf-tools`).**
  - Rebuilt test data: the planted-facts fixture, the prototype hub, and a scale generator with 10,050 files and the needle only in `r37/d3/c123`.
  - A pinned `okf` install with sha256 checks.
  - The bundle-check composite action.
  - `test/run-all.sh`.
  - Live check on `tuan-nng/okf-scratch-bundle-check`: the push run succeeded, and a PR with a stale index failed with "index files are out of date". The design doc's command output was re-run from the rebuilt data.
- **The okfcli fork.**
  - `tuan-nng/okf` release `v0.5.0-tuan-nng.1` reads `stale_after` as an RFC 3339 datetime or a date, in validation and in `okf show`'s `stale` field. The upstream pull request okfcli/okf#39 is open.
  - The pin moved to the release (`okf-tools` `fa42d76`, tag `v0.2.0`).
  - Scale medians: validate 0.340 s, search 0.311 s, backlinks 0.307 s.
- **Branch-mode setup and `okf/main` CI.**
  - `okf-branch-setup.sh` is hardened and passes 25 automated cases, with hook snippets for husky and lefthook.
  - The `okf/main` workflow template is proven on `tuan-nng/okf-scratch-branch-mode`: a clean PR passed, and a PR with a stale index failed.
  - The design doc and research report section 5 record this as a tested fact. Research report section 8, item 2 still lists it as a to-do; phase 05 removes it.
- **Hub assembly, not yet live.**
  - Built: the assembly script, the hub check, the hub action and workflow template, and the GitHub App helper.
  - `test/assemble.sh` passes. `bench-assemble.sh 50` runs in 5.0 s cold and 5.0 s warm.
  - Scratch repos `okf-scratch-billing-api`, `okf-scratch-shared-auth` and `okf-scratch-web-app` exist. No hub CI run has happened.
- **Code with no live run yet.**
  - The stamper and sync job in Python: 27 stamper tests (21 on recorded API responses, 6 front-matter unit tests) and 25 sync tests on recorded API responses, `okf-tools` `783c201`..`9dcdd60`. These commits are local only: GitHub's `okf-tools` stops at `fa42d76`.
  - The skill, the installer, the pointer lines and the skill-eval harness: uncommitted in `okf-tools`.

## Decisions

Confirmed with the user:

1. **Code home: compass.** All tooling code, actions, templates, test data and the skill live in compass. Compass is already the private `tuan-nng/compass` on GitHub (default branch `master`), holding only docs and plans. The code is pushed there, and its Actions access opens to the account's private repos. Rejected alternatives:
   - a separate tooling repo, which means two places to clone and keep in step;
   - keeping compass local and publishing actions from a second repo.
2. **One Go binary.** `compass` replaces every script with a subcommand. It uses only the Go standard library and `gopkg.in/yaml.v3`, the YAML library okfcli already uses. The rejected alternative was a thin wrapper around the existing bash and Python.
3. **The end user provides the hub; setup connects to it.** Compass owns no hub repo. It owns the hub format: the layout, the `repos.txt` and `checks.txt` formats, and a hub template folder with:
   - the workflows, `CODEOWNERS` and `.gitignore`;
   - an overview concept explaining how to fill the hub in.

   An end user copies the template into their own repo and creates the reader and writer GitHub Apps with `compass app create`. `compass setup` asks for the org and that hub repo, and checks the hub is readable. It records them in the user's config, installs the pinned `okf`, and installs the skill into each agent's user-level skill folder. The same command, with flags instead of prompts, sets up CI and cloud agents. The rejected alternative created the hub during setup, which puts org-admin work in every developer's setup.
4. **Delete `okf-tools` and `tuan-nng/knowledge-hub`** once the scratch repos run green on compass. `knowledge-hub` holds only seed copies of test data. For live checks, a scratch hub, `tuan-nng/okf-scratch-hub`, plays the end user's hub. It is created from the template alone, which proves the template works. The rejected alternative archived both repos.
5. **okfcli#34: a pinned, patched fork** (`<org>/okf`), with the same patch sent upstream. The fork stays separate from compass. okfcli keeps its code under `internal/`, so Go doesn't let compass import it, and `compass` installs and calls the pinned `okf` binary instead. We drop the fork once upstream releases a fix. The rejected alternative was date-only `stale_after`, which departs from the spec (design section 4.6).
6. **Mode order.** Folder mode pilots first (M4), and branch mode joins in M5. The rejected alternative piloted both at once, which would delay the first pilot until the sync job is live.
7. **The org name is configuration.**
   - `config.env` in compass holds `OKF_ORG`, and an `OKF_ORG` environment variable overrides it.
   - Hub actions take the repository owner.
   - The okf release repo is derived as `$OKF_ORG/okf`.
   - The phases run under the personal account `tuan-nng` until the company org exists. Phase 06 moves compass and the fork there.
8. **Pilot repos are chosen when phase 08 starts,** after M1–M2 give maintainers a working demo. They should be repos whose contract or schema test runs as a GitHub Actions job on pushes to the default branch (decision 13).
9. **CI and cloud agents find branch-mode knowledge through the skill plus a pointer line.** `compass setup` runs in every CI and cloud agent environment. Each platform's organisation-level instructions get one line: check `git ls-remote origin okf/main`, and follow the OKF skill if that branch exists. The rejected alternatives:
   - the skill alone, which has no trigger in a fresh clone without `okf/`;
   - a marker file on code branches, which branch-mode maintainers don't want.
10. **OpenKB is not adopted.** It is an alpha CLI that compiles documents into a wiki with an LLM. It has no trust fields, no hub, no git support and no failing lint. Two things are kept from it:
    - the skill adapts its rule that wiki text is data, never instructions (Apache-2.0, credited);
    - the read-out decides whether to try it as a one-off drafting tool (decision 11).
11. **Seeding from docs waits for the pilot.** Most "why" knowledge lives in code, READMEs and pull requests. The read-out decides whether to run a one-off trial that drafts concepts from a repo's prose docs. Any drafts go through normal review, as `status: draft` with no `verified` entry.
12. **Quality targets.** Each is checked in the phase that owns it:
    - pinned `okf` on the 10,000-concept hub: median validate under 10 s, search under 2 s, backlinks under 2 s;
    - assembling 50 repos from local bare remotes: under 5 min cold, under 60 s warm;
    - an approved, green knowledge pull request merges within 2 sync runs after its code pull request merges;
    - building `compass` in a CI job takes at most 60 s.

Decided in planning:

13. **The check job runs from the hub, for both modes.** It uses the GitHub API to find a named workflow job on the head commit of the repo's default branch. If that job passed, it stamps the concepts the check covers. The hub keeps the list of covered concepts under CODEOWNERS review, so a repo pull request cannot stamp itself. The rejected alternative ran the job in each repo's CI after merge. That needs a write credential and a workflow file in every repo, which branch-mode code branches can't hold.
14. **Two GitHub Apps.**
    - The reader app can only read repo contents; hub pull request CI uses it.
    - The writer app can push contents, manage pull requests and read Actions results. Only the sync job and the check job use it, and its key lives in a GitHub environment that only the hub's default branch can use.

    The rejected alternative, one app, lets any hub branch mint a write token for every pilot repo.
15. **No repo pipeline triggers the hub.** Hub CI runs nightly, on manual dispatch, and on hub pull requests. A hub pull request that waits on a repo change goes green when its author re-runs its check after that change merges, and the skill says so. The rejected alternative needs a hub write credential in every pilot repo.
16. **The CI entry points are composite actions that build `compass` from source.** Callers pin each action by commit SHA. The action sets up Go with a SHA-pinned `actions/setup-go`, with its build cache on, and builds `compass` from the action's own checkout. So the binary always matches the pinned code, with no release pipeline. Rejected alternatives:
    - release binaries: faster, but a second pin and a release pipeline;
    - reusable workflows: a called workflow runs with its caller's token, which can't check out a private compass to build it.
17. **Neutral module path.** The Go module is named `compass`, so no account name appears in the code (decision 7). People build from a clone with `make install`. The rejected alternative, `github.com/<org>/compass`, hard-codes today's account into every import.
18. **Old test suites become end-to-end checks.** The bash suites move to compass and drive the built binary, so the cases stay and only the commands change. The recorded-API tests become Go tests on the same JSON fixtures. The rejected alternative rewrote everything as Go unit tests and loses the black-box evidence.
19. **Strict validator: port it to Go** (confirmed by the user on 2026-10-06). The vendored okf-skills validator is 573 lines of Python, MIT-licensed, parsing YAML with pyyaml. It becomes a Go package behind `compass validate`. A differential test runs both versions on the fixture, the prototype hub and a set of broken bundles, and requires the same findings. The Python copy stays as a test-only reference. Rejected alternatives: running Python under uv at runtime, which needs Python and uv everywhere; and dropping it for `okf validate`, which loses findings only the strict validator reports, such as a missing frontmatter block.

## Design

**Components:**

- **`compass` binary** (`cmd/compass` and internal packages). Subcommands:
  - `setup`;
  - `okf install`;
  - `validate`, the strict validator (decision 19);
  - `bundle check`;
  - `hub assemble` and `hub check`;
  - `branch setup`;
  - `app create`;
  - `stamp` and `sync`;
  - `pilot report`;
  - `version`.
- **Composite actions:** `actions/bundle-check`, `actions/hub` and `actions/writer`, which build `compass` and run one subcommand each (decision 16).
- **Templates:**
  - repo workflows for folder mode and `okf/main`;
  - hook snippets and agent pointer lines;
  - `templates/hub/`, the hub template (decision 3).
- **The skill,** `skill/okf/SKILL.md`, embedded in the binary.
- **Pins and config:** `pins/okf.env` and `config.env`, embedded at build time.
- **Test data and suites:** `testdata/` and `test/`, moved from `okf-tools`.
- **`<org>/okf` fork:** release binaries with checksums. It adds only datetime `stale_after`.
- **The end user's hub,** made from the template. It holds `repos.txt`, `checks.txt`, cross-repo concepts and the workflows that call compass actions. It holds the reader and writer app credentials, and git ignores its assembled `repos/` folder.
- **Pilot repos.** Each owns its bundle: `okf/` on its code branches, or its `okf/…` branches.

**Interfaces:**

- **Pinned `okf`.** The same commands and JSON as okfcli v0.5.0. `stale_after` takes an RFC 3339 datetime or a date, and `okf show` reports `.concept.stale: true` once it has passed.
- **`compass bundle check <dir>`.** Fails on a strict-validator finding or an out-of-date index.
- **`compass hub assemble [--ci|--local] <hub>`.** Reads `repos.txt` lines of the form `<name> <URL> <folder|branch>`, and accepts only `https://github.com/$OKF_ORG/` URLs. It keeps a clone cache, writes `repos/<name>/`, and removes repos no longer listed. It exits non-zero and names the repo when:
  - fetching fails;
  - the bundle is empty;
  - the bundle holds a symlink.

  In local mode, a repo that can't be fetched keeps its previous copy and gets a warning.
- **`compass hub check <hub>`.** Runs the strict validator, and checks that each repo's concept count is above zero and matches the files copied.
- **`compass stamp`.** Reads `checks.txt` rows of the form `<repo> <workflow-file> <job> <process-actor> <concept-id>...`, split shell-style, so a job name with spaces is quoted. For each row it stamps the concepts as they are at the commit where the named job last succeeded on a push to the default branch's head. It sets `verified.at` to the job's completion time. It skips:
  - concepts generated after that time;
  - concepts already holding a current entry from that actor.

  It pushes only as a fast-forward. In branch mode it stamps `okf/main`.
- **`compass sync`.** Carries out the branch-mode state table (research report section 5) over every branch-mode repo, with dry-run. It merges only under the conditions in Trust boundaries.
- **`compass setup`.** Prompts for the org and hub, or takes `--org`, `--hub` and `--yes`. If the hub isn't readable it writes nothing.
- **`compass pilot report`.** Reports the five metrics from design section 9, item 5, per repo, for a date window.

**Data:**

- Concepts belong to each repo.
- `repos.txt`, `checks.txt`, `CODEOWNERS` and `.github/` belong to the end user's hub, and every change needs code-owner review.
- The user config (`$XDG_CONFIG_HOME/compass/config`) holds the org and hub for one machine. CI never reads it.
- The clone cache is per machine.
- Sync state is not persisted. Each comment carries a hidden marker, so the next run doesn't repeat it.
- Pilot metrics are committed to compass at the read-out.

**Flow, folder mode:**

1. The agent edits `okf/` in the code pull request.
2. The bundle check runs.
3. A reviewer approves and may add a `human:` stamp.
4. The pull request merges.
5. The nightly hub run assembles the change.
6. The writer workflow, every 30 minutes, commits `process:` stamps through the writer app's ruleset bypass.

**Flow, branch mode:**

1. The code and knowledge pull requests are opened together, and one links the other.
2. The `okf/main` workflow checks the knowledge pull request.
3. A reviewer approves both.
4. The code pull request merges.
5. The next writer run's sync step merges the knowledge pull request with a merge commit.
6. The stamper step stamps `okf/main`.
7. The hub assembles `okf/main`.

**Failure modes:**

- **The GitHub API is down or rate-limited.** The writer jobs exit non-zero and the next run retries. Every action is safe to repeat.
- **Writer runs overlap.** One concurrency group queues runs and never cancels one.
- **A sync action meets a conflict or failing checks.** The job comments once and retries on the next run.
- **The stamper's push races a merge.** The fast-forward fails and the repo is skipped. The next run stamps only if the check passed on the new head.
- **The check never runs on the default-branch head.** Nothing is stamped, and each run logs `no check run on head`.
- **A `checks.txt` concept id is missing, or a `verified` form is unsupported.** The stamper fails, names the row or concept, and rewrites nothing.
- **A repo can't be fetched, or its bundle is empty.** The hub run fails and names the repo.
- **The Go build fails inside an action.** The job fails before any check runs.
- **The hub is unreadable during setup.** Setup exits non-zero, names the repo, and writes nothing.
- **Upstream okfcli ships a different fix.** We write datetimes only, so our data survives either behavior.

**Trust boundaries:**

- **The writer key** can push to every pilot repo. It is usable only from the hub's default branch, through the `okf-write` environment. The hub template documents what that requires:
  - a ruleset that blocks direct pushes and requires a code-owner approval;
  - CODEOWNERS on `.github/`, `CODEOWNERS`, `repos.txt` and `checks.txt`.
- **The reader key** can be read by anyone with write access to the hub, because pull request CI runs before review. To limit that:
  - the app is installed only on listed repos;
  - it can only read contents;
  - its tokens are short-lived;
  - assembly accepts only `$OKF_ORG` URLs.
- **Pinned tools.**
  - Compass actions are pinned by commit SHA, and third-party actions by SHA.
  - The `okf` binary is pinned by sha256.
  - Tokens reach `compass` only as action inputs or through the variable named by `--token-env`, and are never written to disk.
- **Merges by the sync job.** The sync job merges only when every condition holds:
  - the head is `okf/<b>` in the same repo, never a fork;
  - a human with write access approved the current head;
  - the bundle check passed on that head, from the `github-actions` app;
  - the code pull request from `<b>` merged into the default branch;
  - the two pull requests link each other in at least one direction.

  It merges with a merge commit, passing the head SHA it checked.
- **The check job's evidence** is only a job from the named workflow file, triggered by a push to the default branch.
- **Repo bundles are untrusted text.** The skill tells agents to treat concepts as claims, never instructions.
- **`human:` stamps** stay checked by review alone (design section 8).

## Phases

| # | Milestone | Phase | Outcome | Status |
|---|---|---|---|---|
| 01 | M1 | [phase-01-cli-core.md](phase-01-cli-core.md) | `compass` builds from a clone and replaces every script except the writer jobs; the moved suites pass against it | completed |
| 02 | M1 | [phase-02-writer-jobs.md](phase-02-writer-jobs.md) | `compass stamp` and `compass sync` pass the recorded-API tests the Python jobs pass | completed |
| 03 | M2 | [phase-03-setup-skill-template.md](phase-03-setup-skill-template.md) | `compass setup` installs `okf` and the skill against the user's hub; the skill passes its scenarios; the hub template ships | in-progress |
| 04 | M2 | [phase-04-live-on-github.md](phase-04-live-on-github.md) | Compass code is pushed; a scratch hub made from the template and the scratch repos run green on compass actions | pending |
| 05 | M2 | [phase-05-cutover.md](phase-05-cutover.md) | `okf-tools` and `tuan-nng/knowledge-hub` are deleted, and the docs describe compass and the end-user hub | pending |
| 06 | M3 | [phase-06-org-and-check-job.md](phase-06-org-and-check-job.md) | Compass and the fork move to the company org; the writer app stamps live | pending |
| 07 | M3 | [phase-07-sync-live.md](phase-07-sync-live.md) | The sync job carries out every state-table row against a scratch branch-mode repo | pending |
| 08 | M4 | [phase-08-folder-pilot.md](phase-08-folder-pilot.md) | 1–2 folder-mode repos are live, and `compass pilot report` measures them | pending |
| 09 | M5 | [phase-09-branch-pilot.md](phase-09-branch-pilot.md) | One branch-mode repo is live, and CI and cloud agents find its knowledge | pending |
| 10 | M5 | [phase-10-pilot-readout.md](phase-10-pilot-readout.md) | A read-out with the five metrics and a roll-out decision is committed to compass | pending |

Dependencies:

- 02 needs the module and shared packages from 01, so 01 and 02 overlap once those land.
- 03 needs 01.
- 04 needs 01–03.
- 05 needs 04.
- 06 needs 05 and the company org.
- 07 needs 06.
- 08 needs 06 and the pilot repos.
- 09 needs 07 and 08.
- 10 needs 09 and the pilot windows.

Scope: ten phases. Each closes one milestone step with its own exit check. M1 rewrites about 2,600 lines (824 of shell and Python entry points, 1,113 of Python library, 573 of validator) in one module, and splitting it further would hide which half works.

## Non-Goals

- Changing what any check, the stamper or the sync job decides while porting to Go. Phases 01–02 move behavior; they don't change it.
- Creating a hub in `setup`, or hosting a hub for end users (decision 3).
- Release binaries for `compass` (decision 16).
- Enforcing who writes a `human:` stamp.
- Removing a `process:` stamp when its check later fails; `stale_after` still ages the concept.
- okfcli#35: the skill reads no custom frontmatter keys.
- Tooling that moves concepts (design section 4.7).
- Git for Windows support for branch setup and hooks.
- Any dependency on OpenKB or an LLM-calling tool in the skill's commands, the actions or the hub jobs (decision 10).
- An MCP server or any agent interface other than the CLI and the skill.
- Rolling out beyond the pilot repos.

## Acceptance Criteria

1. A fresh compass clone builds `compass` with `make install`, with Go, make and git as the only prerequisites. `compass setup` against a readable hub leaves a working `okf`, the skill in each detected agent folder, and a config. Against an unreadable hub it writes nothing.
2. Every case in the moved suites passes against `compass`, and both recorded-API test sets pass in Go.
3. A hub made only by copying `templates/hub/` and running `compass app create` assembles on GitHub. It fails on a broken cross-repo link, an empty bundle and a symlink.
4. `tuan-nng/okf-tools` and `tuan-nng/knowledge-hub` no longer exist, and nothing live refers to them.
5. In a folder-mode pilot repo, an agent with the skill reads with trust rules and records 1–3 knowledge files in a code pull request. The bundle check passes, and a reviewer's `verified` line makes `okf show` report `human-reviewed`.
6. The stamper stamps only concepts in `checks.txt`, only after the named check passed, and makes no commit when stamps are current.
7. In a branch-mode repo, the `okf/main` workflow checks knowledge pull requests, and the sync job carries out each state-table row.
8. The read-out reports all five metrics for at least one repo per mode, over at least four weeks, and states a roll-out decision.

## Verification

Run from the compass root:

- `go vet ./... && go test ./...` exits 0.
- `./test/run-all.sh` exits 0.
- `gh run list -R <hub> -w hub -L 1 --json conclusion -q '.[0].conclusion'` prints `success`, for the scratch hub in M2 and the pilot hub after.
- `gh repo view tuan-nng/okf-tools` and `gh repo view tuan-nng/knowledge-hub` both exit non-zero.
- `compass pilot report --repos pilot.txt --since <start>` prints a value for each of the five metrics, for at least one repo in each mode.
- `af plans` reports zero findings.

## Risks

- **The rewrite drops a behavior.** Signal: a moved end-to-end case fails, or a live run differs from its recorded result. Response: fix in place. The old scripts stay readable in the local `okf-tools` clone until phase 05.
- **The validator port disagrees with okf-skills.** Signal: the differential test reports a difference. Response: fix the port. If the YAML parsing differences can't be closed, return to planning on decision 19.
- **Building Go in every CI job is slow.** Signal: the build step goes over 60 s. Response: rely on `setup-go`'s cache. If that isn't enough, return to planning on decision 16.
- **Pushing compass exposes local-only files.** The harness folders and `CLAUDE.local.md` are ignored only through this clone's `.git/info/exclude`. Signal: before a push, `git ls-files` lists harness folders or `CLAUDE.local.md`. Response: stop and ask the user.
- **The account can't delete repos.** The `gh` token lacks `delete_repo`. Signal: HTTP 403. Response: the user runs `gh auth refresh -s delete_repo`, or deletes the repos in the GitHub UI.
- **GitHub Free blocks hub protections on private repos.** Rulesets return HTTP 403, and private-repo environments need a paid plan. So the CODEOWNERS-review check and the `okf-write` environment refusal can't be shown until the company org exists. Signal: phase 06 starts before the org exists. Response: phase 06 waits for the org, as the user chose.
- **The company org grant arrives late.** Signal: phase 06 can't move a repo or install an app. Response: M1–M2 stay usable under the personal account, and M3 waits.
- **Upstream disagrees with the fork.** Signal: okfcli/okf#39 is rejected, or a release parses datetimes differently. Response: keep the fork pinned. Return to planning only if upstream rejects datetimes outright.
- **Skills don't auto-load.** Signal: an agent in phase 03 skips the skill despite the pointer line. Response: make the pointer line the main trigger. Return to planning if Cursor can't load user-level skills.
- **The ruleset bypass is refused.** Signal: a pilot maintainer declines it. Response: that repo gets no `process:` stamps during the pilot, and the read-out says so.
- **Assembly is slower through GitHub than locally.** Signal: the hub workflow takes over 10 minutes. Response: use shallow partial clones or parallel fetches.
- **The check never runs where the stamper looks.** Signal: a `checks.txt` row logs `no check run on head` for a week. Response: change that repo's triggers with its maintainers, or remove the row.
- **CI and cloud agents miss branch-mode knowledge.** Signal: phase 03's fresh-clone scenario, or a phase 09 agent run, never runs branch setup. Response: return to planning for how branch-mode repos announce themselves.

## Unresolved Questions

Inputs with fixed deadlines:

1. **The company org name, and who approves app installs.** Needed before phase 06.
2. **The pilot repos, with their maintainers' agreement.** Needed before phase 08.

## Validation Log

### Session 1 — 2026-10-06
**Trigger:** `/af:plan validate`, plan written in an earlier session. **Claims checked:** every cited path, commit, count, flag, doc section and live repo, by three read-only scouts plus direct `gh`, `git` and `okf` runs
**Verified:** all claims not listed below | **Failed:** 22 | **Unverified:** 0 (the account plan name is hidden; the ruleset API's "Upgrade to GitHub Pro" 403 confirms the Free-plan risk)
- Failed: frontmatter `branch: main` — compass uses `master`; fixed in frontmatter
- Failed: compass "pushed" in phase 04 — `tuan-nng/compass` exists, private, `master`, created 2026-09-29; fixed in decision 1, frontmatter, Risks, phase 04
- Failed: "27 recorded-API tests" — `okf-tools/test/stamp/test_frontmatter.py` holds 6 unit tests; fixed in Landed, phase 02
- Failed: move "at `9dcdd60`" loses uncommitted skill, skill-eval, pointers, `THIRD_PARTY.md`, `bin/okf-tools-install`; `run-all.sh` runs 9 suites, 2 of them Python unit tests; fixed in phases 01, 03
- Failed: hook snippets and `test/skill-eval/analyze.py:356` call `okf-branch-setup.sh`; fixed in phases 01, 03
- Failed: `actions/writer` owned by phases 02 and 04; `actionlint` not installed; fixed in phases 02, 03
- Failed: `okf --version` prints JSON (`okf/cmd/okf/main.go:47-48`); fixed in phase 04
- Failed: okfcli#37 reason, decision 16 reason, "2,500 lines", plan.md "tested fact" (research:503-505); fixed in place
- Failed: phase 03 doc greps miss 11 of 17 workaround lines; phase 05 grep subjective, misses `okf_validate.py`; phase 06 cites design section 5 instead of research:327-328; research:471 cites the replaced plan; fixed in phases 03, 05, 06
- Failed: phase 07 omits state-table row 1, "conflicting", and the "Merged" condition (research:347-353); fixed in phase 07
- Failed: folder workflow pushes only `main` (`okf-tools/templates/folder-workflow.yml:10-11`); weak ready-for-review check; `gh run watch` without `--exit-status`; fixed in phases 04, 06, 08
- Decided: strict validator (decision 19) — port to Go with a differential test
