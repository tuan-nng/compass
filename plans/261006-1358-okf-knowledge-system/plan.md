---
title: "OKF knowledge system: compass CLI, hub template and pilot"
description: "People clone compass, build one compass binary, connect it to their own knowledge hub, and pilot the OKF knowledge system on real repos, folder mode first, then branch mode."
status: in-progress
priority: P1
effort: 17.5d build + 4-week pilot per mode
branch: master
tags: [okf, knowledge, agents, tooling, go, cli]
created: 2026-10-06
---

# OKF knowledge system: compass CLI, hub template and pilot

## Overview

**Problem:** The design in `docs/design/okf-knowledge-system-ux.md` is complete on paper, but most of it doesn't run yet. What runs today is a set of bash and Python scripts spread over a separate tooling repo, `tuan-nng/okf-tools`, plus a hub repo the project owns, `tuan-nng/knowledge-hub`. There is no single entry point.
**Change:** Put all tooling into compass as one Go binary, `compass`. People clone compass, build it, and run `compass setup`, which asks for the knowledge hub they provide. Compass ships a hub template instead of owning a hub. Everything except one merge gate runs on people's machines: the only GitHub Actions job is the bundle check on each repo's pull requests. Then finish the remaining pieces (the skill, the stamper, the sync job) and pilot the system, folder mode first and branch mode later.
**Why this way:** The user chose compass as the only code home, a Go binary, an end-user hub, and local-first operation (decisions 1–4 and 13–16). The strongest rejected alternative kept the tested scripts behind a thin `compass` wrapper. It needs no rewrite, but users would need bash, Python and uv, and there would be no build step. There are two main risks. Behavior could be lost in the rewrite, so the existing test suites carry over as end-to-end checks against the binary. Stamps and merges wait until someone runs them, and the pilot measures that delay.

The work is grouped into five milestones. Each one leaves something usable behind:

| Milestone | What works when it closes |
|---|---|
| M1 One tool | `compass` builds from a clone, replaces every script, and passes every test the scripts passed. |
| M2 Live on GitHub | `compass setup` connects a machine to a hub. Compass's bundle-check action gates the scratch repos' pull requests, and a hub made from the template assembles and checks on a developer machine. `okf-tools`, `knowledge-hub` and the scratch repos are gone. |
| M3 Writers ready | `compass stamp` names a refused push, and both writers pass their checks on a hub folder copied from the template, with recorded GitHub responses (decision 22). Compass is public, so any account's repos can use its action. |
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
  - Scratch repos `okf-scratch-billing-api`, `okf-scratch-shared-auth` and `okf-scratch-web-app` exist. No assembly from GitHub has run.
- **Code with no live run yet.**
  - The stamper and sync job in Python: 27 stamper tests (21 on recorded API responses, 6 front-matter unit tests) and 25 sync tests on recorded API responses, `okf-tools` `783c201`..`9dcdd60`. These commits are local only: GitHub's `okf-tools` stops at `fa42d76`.
  - The skill, the installer, the pointer lines and the skill-eval harness: uncommitted in `okf-tools`.

## Decisions

Confirmed with the user:

1. **Code home: compass.** All tooling code, the action, templates, test data and the skill live in compass. Compass is already the private `tuan-nng/compass` on GitHub (default branch `master`), holding only docs and plans. The code is pushed there, and its Actions access opens to the account's private repos until compass goes public (decision 21). Rejected alternatives:
   - a separate tooling repo, which means two places to clone and keep in step;
   - keeping compass local and publishing actions from a second repo.
2. **One Go binary.** `compass` replaces every script with a subcommand. It uses only the Go standard library and `gopkg.in/yaml.v3`, the YAML library okfcli already uses. The rejected alternative was a thin wrapper around the existing bash and Python.
3. **The end user provides the hub; setup connects to it.** Compass owns no hub repo. It owns the hub format: the layout, the `repos.txt` and `checks.txt` formats, and a hub template folder with:
   - `CODEOWNERS` and `.gitignore`;
   - an overview concept explaining how to fill the hub in and run it.

   An end user copies the template into their own repo. `compass setup` asks for that hub repo and for a folder to clone it into, defaulting to `~/src/<repo>`; `--hub` and `--hub-dir` skip the prompts. It reuses an existing clone of the same repo, clones it otherwise, and stops before writing anything if the hub isn't readable. It records the hub and its folder in the user's config, installs the pinned `okf`, and installs the skill into each agent's user-level skill folder. Hub work happens in that clone, and the hub commands use it unless given another folder. How compass itself reaches a developer machine is out of scope; `make install` is one way. The same command, with flags instead of prompts, sets up CI and cloud agents. The rejected alternative created the hub during setup, which puts org-admin work in every developer's setup.
4. **Delete `okf-tools`, `tuan-nng/knowledge-hub` and the scratch repos** once the scratch repos run green on compass. `knowledge-hub` holds only seed copies of test data. For phase 04's live checks, five scratch repos and a scratch hub, `tuan-nng/okf-scratch-hub`, played the end user's repos and hub. The hub was created from the template alone, which proved the template works. After phase 04 nothing uses them (decision 22), and compass owns no hub (decision 3), so phase 05 deletes all six. The rejected alternative archived the repos.
5. **okfcli#34: a pinned, patched fork** (`tuan-nng/okf`, public), with the same patch sent upstream. `pins/okf.env` names the release repo with the tag and checksums. The fork stays separate from compass. okfcli keeps its code under `internal/`, so Go doesn't let compass import it, and `compass` installs and calls the pinned `okf` binary instead. We drop the fork once upstream releases a fix. The rejected alternative was date-only `stale_after`, which departs from the spec (design section 4.6).
6. **Mode order.** Folder mode pilots first (M4), and branch mode joins in M5. The rejected alternative piloted both at once, which would delay the first pilot until the sync job is live.
7. **No org setting (revised 2026-10-06).** The hub repo is the only thing a user names. `repos.txt` may list any `https://github.com/<owner>/<repo>` URL, so a hub can span accounts. The workflow templates name the compass repo by placeholder. Accepted cost: a reviewed `repos.txt` line can make every developer's assembly fetch any GitHub repo; its bundle is untrusted text either way (Design, Trust boundaries). The same line also makes `compass stamp` and `compass sync` write to that repo, with only the rights of the person who runs them. The rejected alternative, the decision before revision, set `OKF_ORG` in `config.env` or the user config and limited `repos.txt` to that account. It made every user know an org name, and it blocked hubs spanning accounts.
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
    - an approved, green knowledge pull request merges on the first `compass sync` run after its code pull request merges;
    - building `compass` in a CI job takes at most 60 s.

Decided in planning. Decisions 13–16 were revised on 2026-10-06, when the user chose local-first operation with one CI gate. Each revised decision keeps the version it replaced as a rejected alternative.

13. **The stamper and the sync job run on a person's machine, for both modes.** A hub owner runs `compass sync` and `compass stamp` with their own GitHub token, on demand; during the pilot, each working day.
    - The sync job keeps its merge conditions (Design, Trust boundaries), and that person makes the merge.
    - The stamper uses the GitHub API to find a named workflow job on the head commit of the repo's default branch. If that job passed, it stamps the concepts the check covers, in a commit made by that person.
    - The hub keeps the list of covered concepts under CODEOWNERS review, so a repo pull request cannot stamp itself.
    - The stamper pushes straight to the default branch (or `okf/main`), so the person who runs it needs a ruleset bypass on each protected repo. When protection refuses the push, the stamper says so, instead of reporting that the branch moved. The rejected alternatives were a pull request per stamp, which adds review load and changes the stamper's output, and no `process:` stamps in the pilot.

    Accepted cost: a `process:` stamp commit made by a person looks like any other commit, so there's no bot identity to tell it apart from a hand-written stamp. `compass stamp --dry-run` re-derives each stamp from the API evidence. Rejected alternatives:
    - a scheduled hub workflow with a writer app (the decision before revision). Stamps carry a bot identity, but it needs a stored write key, plus rulesets and environments that GitHub Free refuses on private repos;
    - running the job in each repo's CI after merge. That needs a write credential and a workflow file in every repo, which branch-mode code branches can't hold.
14. **No GitHub Apps.** Every compass command uses its caller's own GitHub credentials, so it can do only what that person can. The rejected alternative, a reader app for hub CI and a writer app for the writer workflow (the decision before revision), served only the hub-side Actions that decision 15 removes.
15. **The hub has no CI, and no repo pipeline triggers it.** Cross-repo checks run where people read: `compass hub assemble`, then `compass hub check`. The agent runs them before it opens a hub pull request, and the reviewer runs them before approving. A hub pull request that waits on a repo change is checked again locally after that change merges, and the skill says so. Accepted cost: nothing reports a broken cross-repo link until someone assembles. Rejected alternatives:
    - hub CI nightly, on manual dispatch and on hub pull requests (the decision before revision). It needs the reader app and Actions in the hub;
    - no Actions anywhere, with the bundle check as a git hook. Hooks don't run for cloud agents, CI agents, web edits or `--no-verify`, so stale or invalid bundles would reach the default branches every reader assembles.
16. **The only CI entry point is `actions/bundle-check`,** a composite action that builds `compass` from source. Callers pin it by commit SHA. It sets up Go with a SHA-pinned `actions/setup-go` and builds `compass` from the action's own checkout. So the binary always matches the pinned code, with no release pipeline. The setup-go cache is off (changed 2026-10-07, phase 04): setup-go keys its cache on a `go.sum` under `GITHUB_WORKSPACE` and ignores files outside it, and compass's `go.sum` sits in the action's checkout, outside the caller's workspace. The uncached build takes about 19 s, under the 60 s target. Rejected alternatives:
    - release binaries: faster, but a second pin and a release pipeline;
    - reusable workflows: a called workflow runs with its caller's token, which can't check out a private compass to build it.
17. **Neutral module path.** The Go module is named `compass`, so no account name appears in the code (decision 7). People build from a clone with `make install`. The rejected alternative, `github.com/<org>/compass`, hard-codes today's account into every import.
18. **Old test suites become end-to-end checks.** The bash suites move to compass and drive the built binary, so the cases stay and only the commands change. The recorded-API tests become Go tests on the same JSON fixtures. The rejected alternative rewrote everything as Go unit tests and loses the black-box evidence.
19. **Strict validator: port it to Go** (confirmed by the user on 2026-10-06). The vendored okf-skills validator is 573 lines of Python, MIT-licensed, parsing YAML with pyyaml. It becomes a Go package behind `compass validate`. A differential test runs both versions on the fixture, the prototype hub and a set of broken bundles, and requires the same findings. The Python copy stays as a test-only reference. Rejected alternatives: running Python under uv at runtime, which needs Python and uv everywhere; and dropping it for `okf validate`, which loses findings only the strict validator reports, such as a missing frontmatter block.
20. **A stats repo holds pilot results (2026-10-06).** `compass setup` optionally takes a stats repo, cloned the same way as the hub. `compass pilot report` computes the metrics from GitHub pull requests and git history and writes its report into that clone, where it can be shared. The read-out lives there too, because compass goes public and pilot data names private repos. Nothing is recorded on developer machines. Rejected alternatives:
    - usage events recorded by each compass install: telemetry, a new data format, and a push from every machine;
    - committing metrics to compass, which becomes public.
21. **Compass goes public before the pilot (2026-10-06).** Then pilot repos in any account can run `actions/bundle-check`, pinned by SHA, with no org move and no org name. Rejected alternatives:
    - moving compass to the pilot repos' account, which ties compass to one org;
    - building compass in each workflow with a token secret in every pilot repo;
    - piloting only on repos under `tuan-nng`.
22. **Tests need no extra GitHub repo (2026-10-07).** Every check after phase 04 runs on folders: local bare remotes, hub folders copied from `templates/hub/`, and recorded GitHub API responses served through `ghfake` (`--api-url` for the writers). GitHub-side behavior is next exercised on the pilot repos, which exist anyway: the first stamp in phase 08, the first sync merge in phase 09, and the first public run of the action in phase 08. Accepted cost: a GitHub behavior the recorded responses miss first shows up during the pilot, on a real repo (Risks). Rejected alternative: scratch repos under the personal account, as in phases 02–04 and the plan before this revision. They prove GitHub behavior before the pilot, but they are repos to create, keep in step and delete, and a throwaway protected repo was needed only to learn one message text.

## Design

**Components:**

- **`compass` binary** (`cmd/compass` and internal packages). Subcommands:
  - `setup`;
  - `okf install`;
  - `validate`, the strict validator (decision 19);
  - `bundle check`;
  - `hub assemble` and `hub check`;
  - `branch setup`;
  - `stamp` and `sync`;
  - `pilot report`;
  - `version`.
- **Composite action:** `actions/bundle-check`, which builds `compass` and runs `compass bundle check` (decision 16).
- **Templates:**
  - repo workflows for folder mode and `okf/main`;
  - hook snippets and agent pointer lines;
  - `templates/hub/`, the hub template (decision 3).
- **The skill,** `skill/okf/SKILL.md`, embedded in the binary.
- **Pins:** `pins/okf.env`, embedded at build time. It names the fork's release repo, tag and checksums.
- **Test data and suites:** `testdata/` and `test/`, moved from `okf-tools`.
- **`tuan-nng/okf` fork:** release binaries with checksums. It adds only datetime `stale_after`.
- **The end user's hub,** made from the template. It holds `repos.txt`, `checks.txt` and cross-repo concepts. It holds no workflows and no credentials, and git ignores its assembled `repos/` folder.
- **Pilot repos.** Each owns its bundle: `okf/` on its code branches, or its `okf/…` branches.
- **The stats repo,** any repo the pilot team names. It holds `compass pilot report` output and the read-out (decision 20).

**Interfaces:**

- **Pinned `okf`.** The same commands and JSON as okfcli v0.5.0. `stale_after` takes an RFC 3339 datetime or a date, and `okf show` reports `.concept.stale: true` once it has passed.
- **`compass bundle check <dir>`.** Fails on a strict-validator finding or an out-of-date index.
- **`compass hub assemble [--ci|--local] [<hub>]`.** The hub folder defaults to the clone recorded by `compass setup`; the same default applies to `hub check`, `stamp --hub` and `sync --hub`. Reads `repos.txt` lines of the form `<name> <URL> <folder|branch>`, and accepts any `https://github.com/<owner>/<repo>` URL. It keeps a clone cache, writes `repos/<name>/`, and removes repos no longer listed. It exits non-zero and names the repo when:
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
- **`compass setup`.** Prompts for the hub repo, its clone folder and an optional stats repo, or takes `--hub`, `--hub-dir`, `--stats`, `--stats-dir` and `--yes`. If the hub isn't readable it writes nothing. `--org` is gone (decision 7).
- **`compass pilot report`.** Reports the five metrics from design section 9, item 5, per repo, for a date window, and writes the report into the stats clone.

**Data:**

- Concepts belong to each repo.
- `repos.txt`, `checks.txt` and `CODEOWNERS` belong to the end user's hub, and every change needs code-owner review.
- The user config (`$XDG_CONFIG_HOME/compass/config`) holds the hub, the stats repo and their clone folders for one machine. CI never reads it.
- The clone cache is per machine.
- Sync state is not persisted. Each comment carries a hidden marker, so the next run doesn't repeat it.
- Pilot metrics and the read-out live in the stats repo (decision 20).

**Flow, folder mode:**

1. The agent edits `okf/` in the code pull request.
2. The bundle check runs.
3. A reviewer approves and may add a `human:` stamp.
4. The pull request merges.
5. The next local assembly, by any developer or agent, picks the change up.
6. A hub owner's next `compass stamp` run commits `process:` stamps for the covered concepts.

**Flow, branch mode:**

1. The code and knowledge pull requests are opened together, and one links the other.
2. The `okf/main` workflow checks the knowledge pull request.
3. A reviewer approves both.
4. The code pull request merges.
5. A hub owner's next `compass sync` run merges the knowledge pull request with a merge commit.
6. Their next `compass stamp` run stamps `okf/main`.
7. The next local assembly picks up `okf/main`.

**Failure modes:**

- **The GitHub API is down or rate-limited.** `compass stamp` or `compass sync` exits non-zero. Every action is safe to repeat, so the person runs it again.
- **Two people run the writers at once.** Merges pass the head SHA they checked, and stamps push only as fast-forwards, so the later write fails and the next run sees the result. [INFERENCE] A comment can be posted twice if both runs read before either writes.
- **Nobody runs the writers.** Knowledge pull requests stay open after their code merges, and covered concepts go without `process:` stamps. Pilot metric 5 measures the first.
- **A sync action meets a conflict or failing checks.** The job comments once and retries on the next run.
- **The stamper's push races a merge.** The fast-forward fails and the repo is skipped. The next run stamps only if the check passed on the new head.
- **The check never runs on the default-branch head.** Nothing is stamped, and each run logs `no check run on head`.
- **A `checks.txt` concept id is missing, or a `verified` form is unsupported.** The stamper fails, names the row or concept, and rewrites nothing.
- **A repo can't be fetched, or its bundle is empty.** Assembly names the repo. In local mode an unfetchable repo keeps its previous copy (Interfaces).
- **The Go build fails inside the bundle-check action.** The job fails before any check runs.
- **The hub is unreadable during setup.** Setup exits non-zero, names the repo, and writes nothing.
- **Upstream okfcli ships a different fix.** We write datetimes only, so our data survives either behavior.

**Trust boundaries:**

- **No stored credentials.** No app keys exist. `compass` takes a token only through the variable named by `--token-env`, or uses the caller's git credentials. It never writes a token to disk, and it can do only what its caller can.
- **The hub's control files.** `repos.txt` decides what gets assembled and `checks.txt` decides what gets stamped. So CODEOWNERS covers them and `CODEOWNERS` itself, and the hub template documents a ruleset that requires a code-owner approval. That review is enforced only where rulesets are available (Risks: GitHub Free).
- **Assembly accepts only `https://github.com/<owner>/<repo>` URLs.** Any owner is allowed (decision 7). A `repos.txt` change passes code-owner review before developers' machines fetch it, and what it fetches is copied as untrusted text, never run.
- **Pinned tools.** The compass action is pinned by commit SHA, third-party actions by SHA, and the `okf` binary by sha256.
- **Merges by the sync job.** The sync job merges only when every condition holds:
  - the head is `okf/<b>` in the same repo, never a fork;
  - a human with write access approved the current head;
  - the bundle check passed on that head, from the `github-actions` app;
  - the code pull request from `<b>` merged into the default branch;
  - the two pull requests link each other in at least one direction.

  It merges with a merge commit, as the person who runs it, passing the head SHA it checked.
- **The stamper's evidence** is only a job from the named workflow file, triggered by a push to the default branch.
- **Repo bundles are untrusted text.** The skill tells agents to treat concepts as claims, never instructions.
- **`human:` stamps** stay checked by review alone (design section 8). `process:` stamps carry decision 13's accepted cost.

## Phases

| # | Milestone | Phase | Outcome | Status |
|---|---|---|---|---|
| 01 | M1 | [phase-01-cli-core.md](phase-01-cli-core.md) | `compass` builds from a clone and replaces every script except the writer jobs; the moved suites pass against it | completed |
| 02 | M1 | [phase-02-writer-jobs.md](phase-02-writer-jobs.md) | `compass stamp` and `compass sync` pass the recorded-API tests the Python jobs pass | completed |
| 03 | M2 | [phase-03-setup-skill-template.md](phase-03-setup-skill-template.md) | `compass setup` installs `okf` and the skill against the user's hub; the skill passes its scenarios; the hub template ships | completed |
| 04 | M2 | [phase-04-live-on-github.md](phase-04-live-on-github.md) | Hub CI, the apps and the writer workflow are gone; compass is pushed; the scratch repos run the bundle check from compass's action; a hub made from the template assembles and checks locally | completed |
| 05 | M2 | [phase-05-cutover.md](phase-05-cutover.md) | `okf-tools`, `tuan-nng/knowledge-hub` and the six scratch repos are deleted, and the docs describe compass, the end-user hub and local-first operation | completed |
| 06 | M3 | [phase-06-writers-local.md](phase-06-writers-local.md) | `compass stamp` names a refused push instead of "moved"; both writers pass on a hub folder copied from the template, with recorded GitHub responses | completed |
| 07 | M3 | [phase-07-publish-compass.md](phase-07-publish-compass.md) | Compass is public after a history check, and its action's code downloads without credentials | pending |
| 08 | M4 | [phase-08-folder-pilot.md](phase-08-folder-pilot.md) | 1–2 folder-mode repos are live, and `compass pilot report` measures them | pending |
| 09 | M5 | [phase-09-branch-pilot.md](phase-09-branch-pilot.md) | One branch-mode repo is live, and CI and cloud agents find its knowledge | pending |
| 10 | M5 | [phase-10-pilot-readout.md](phase-10-pilot-readout.md) | A read-out with the five metrics and a roll-out decision is committed to the stats repo | pending |

Dependencies:

- 02 needs the module and shared packages from 01, so 01 and 02 overlap once those land.
- 03 needs 01.
- 04 needs 01, 02 and phase 03's code. Phase 03's last item, first planned as a manual Cursor run and done on 2026-10-07 as a Claude run, didn't block it; that run used the skill as phase 04 left it. Phase 04's frontmatter therefore lists only 01 and 02.
- 05 needs 04.
- 06 needs 04.
- 07 needs 05.
- 08 needs 06, 07, the pilot repos, the stats repo, and the `fetchBundle` cache fix that phase 04's Result names, done through `/af:fix`. `internal/hub/assemble.go` compares `git remote get-url origin`, which applies `insteadOf` rewrites, with the `repos.txt` URL. So under any `insteadOf` rule, including the suites' `map_owner` (`test/lib.sh`), every run clones from scratch, and the warm assembly times in Landed and phase 01 measured cold clones. The fix's test shows a warm run reusing the cache under an `insteadOf` rule.
- 09 needs 08.
- 10 needs 09 and the pilot windows.

Scope: ten phases. Each closes one milestone step with its own exit check. M1 rewrites about 2,600 lines (824 of shell and Python entry points, 1,113 of Python library, 573 of validator) in one module, and splitting it further would hide which half works.

## Non-Goals

- Changing what any check, the stamper or the sync job decides while porting to Go. Phases 01–02 move behavior; they don't change it.
- Creating a hub in `setup`, or hosting a hub for end users (decision 3).
- How compass reaches a developer machine (decision 3).
- Release binaries for `compass` (decision 16).
- Enforcing who writes a `human:` stamp.
- Removing a `process:` stamp when its check later fails; `stale_after` still ages the concept.
- okfcli#35: the skill reads no custom frontmatter keys.
- Tooling that moves concepts (design section 4.7).
- Git for Windows support for branch setup and hooks.
- Any dependency on OpenKB or an LLM-calling tool in the skill's commands, the action or the writer commands (decision 10).
- An MCP server or any agent interface other than the CLI and the skill.
- Rolling out beyond the pilot repos.

## Acceptance Criteria

1. A fresh compass clone builds `compass` with `make install`, with Go, make and git as the only prerequisites. `compass setup` against a readable hub leaves a working `okf`, the skill in each detected agent folder, a clone of the hub (an existing clone is reused), and a config. Against an unreadable hub it writes nothing.
2. Every case in the moved suites passes against `compass`, and both recorded-API test sets pass in Go.
3. A hub made only by copying `templates/hub/` assembles on a developer machine with `compass hub assemble`. Assembly fails on an empty bundle and a symlink, and `compass hub check` fails on a broken cross-repo link.
4. `tuan-nng/okf-tools`, `tuan-nng/knowledge-hub` and the `okf-scratch-*` repos no longer exist, and nothing live refers to them.
5. In a folder-mode pilot repo, an agent with the skill reads with trust rules and records 1–3 knowledge files in a code pull request. The bundle check passes, and a reviewer's `verified` line makes `okf show` report `human-reviewed`.
6. The stamper stamps only concepts in `checks.txt`, only after the named check passed, and makes no commit when stamps are current.
7. In a branch-mode repo, the `okf/main` workflow checks knowledge pull requests. `compass sync`, run by a person, carries out each state-table row on recorded GitHub responses, and merges the pilot's first approved, green knowledge pull request live.
8. The read-out reports all five metrics for at least one repo per mode, over at least four weeks, and states a roll-out decision.

## Verification

Run from the compass root:

- `go vet ./... && go test ./...` exits 0.
- `./test/run-all.sh` exits 0.
- In a clone of the hub, `compass hub assemble --ci . && compass hub check .` exits 0: for the scratch hub in M2 (done in phase 04) and the pilot hub after.
- `gh repo view tuan-nng/okf-tools` and `gh repo view tuan-nng/knowledge-hub` both exit non-zero.
- `compass pilot report --repos pilot.txt --since <start>` prints a value for each of the five metrics, for at least one repo in each mode, and writes the same report into the stats clone.
- `af plans` reports zero findings.

## Risks

- **The rewrite drops a behavior.** Signal: a moved end-to-end case fails, or a live run differs from its recorded result. Response: fix in place. The old scripts stay readable in the local `okf-tools` clone until phase 05.
- **The validator port disagrees with okf-skills.** Signal: the differential test reports a difference. Response: fix the port. If the YAML parsing differences can't be closed, return to planning on decision 19.
- **Building Go in the bundle-check job is slow.** Signal: the build step goes over 60 s (19 s uncached on 2026-10-07). Response: cache `GOCACHE` with `actions/cache`, keyed on a hash of the action's own Go sources computed in a run step. If that isn't enough, return to planning on decision 16.
- **Pushing compass exposes local-only files.** The harness folders and `CLAUDE.local.md` are ignored only through this clone's `.git/info/exclude`. Signal: before a push, `git ls-files` lists harness folders or `CLAUDE.local.md`. Response: stop and ask the user.
- **Publishing compass exposes something private.** Phase 07 makes the whole history public. Signal: a secret-scan finding, or a private repo or person named in history. Response: stop and ask the user. Publishing then needs a history rewrite or a fresh repo.
- **The account can't delete repos.** The `gh` token lacks `delete_repo`. Signal: HTTP 403. Response: the user runs `gh auth refresh -s delete_repo`, or deletes the repos in the GitHub UI.
- **The pilot account refuses rulesets on private repos.** GitHub Free returns HTTP 403 "Upgrade to GitHub Pro" (checked on `tuan-nng`). Without rulesets, neither the hub's code-owner review nor the `okf/main` ruleset is enforced. Signal: the pilot repos' account is on Free and the repos are private. Response: the folder pilot goes ahead and the read-out names the gap. The branch pilot waits for an account that allows rulesets.
- **Upstream disagrees with the fork.** Signal: okfcli/okf#39 is rejected, or a release parses datetimes differently. Response: keep the fork pinned. Return to planning only if upstream rejects datetimes outright.
- **Skills don't auto-load.** Signal: an agent in phase 03 skips the skill despite the pointer line. Response: make the pointer line the main trigger. Return to planning if Cursor can't load user-level skills. Cursor is untested, because the user has no Cursor access; its docs say it loads `~/.cursor/skills/` and `~/.claude/skills/` (phase 03 Result). The first pilot developer on Cursor is the first real check.
- **The stamp runner has no bypass on a protected default branch.** Signal: `compass stamp` reports a protection refusal on a pilot repo (decision 13). Response: that repo gets no `process:` stamps until its maintainers grant the bypass, and the read-out says so.
- **Nobody runs the writers.** Signal: pilot metric 5 shows knowledge pull requests still open a day after their code merged, or covered concepts lack `process:` stamps a week after their check passed. Response: name one daily owner in the pilot hub. If that fails, return to planning on decision 13.
- **Local assembly is slow over GitHub.** Signal: a cold `compass hub assemble` of the pilot hub takes over 5 minutes. Response: use shallow partial clones or parallel fetches.
- **The check never runs where the stamper looks.** Signal: a `checks.txt` row logs `no check run on head` for a week. Response: change that repo's triggers with its maintainers, or remove the row.
- **CI and cloud agents miss branch-mode knowledge.** Signal: phase 03's fresh-clone scenario, or a phase 09 agent run, never runs branch setup. Response: return to planning for how branch-mode repos announce themselves.
- **Folder tests miss a GitHub behavior (decision 22).** Signal: a writer's first live run on a pilot repo fails, or does something its recorded-response tests don't show. Response: record the real response as a new fixture, fix the writer, and rerun on the pilot repo. Neither writer runs on a schedule, so nothing repeats the failure until a person reruns it.

## Unresolved Questions

Inputs with fixed deadlines:

1. **The pilot repos, with their maintainers' agreement.** Needed before phase 08.
2. **The stats repo.** Any repo the pilot team can share. Needed before phase 08.

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

### Session 2 — 2026-10-06 (re-plan)
**Trigger:** the user chose local-first operation with one CI gate, the repo bundle check, over the plan's hub CI and writer workflow. **Changed:** decisions 3, 7, 12 and 13–16; Design; acceptance criteria 3 and 7; Risks; phases 04–09. Phase 04 takes over the removal of hub CI, the apps and the writer workflow. Phase 05 takes over the decision-15 doc rows from the old phase 04. The old phases 06 (org and check job) and 07 (sync live) become phase 06 (writers run locally, personal account) and phase 07 (org move).
**Checked on 2026-10-06:**
- `actions/` doesn't exist yet.
- `internal/app` has one caller, `cmd/compass/main.go`.
- `templates/hub/.github/workflows/` holds `hub.yml` and `writer.yml`.
- `okf.yml` is on `main` of the three folder-mode scratch repos and on `okf/main` of the two branch-mode ones.
- `tuan-nng/okf-scratch-hub` doesn't exist.
- The compass Actions access level is `none`.
- The design doc and research report have 15 lines naming hub CI, the apps, `okf-write` or a nightly run (13 and 2).

### Session 3 — 2026-10-06 (user decisions)
**Trigger:** the user answered the open questions. **Decided:**
- The stamper pushes directly and names a protection refusal (decision 13).
- The sync merge row is proven at the branch pilot, where a second human can approve, not on the scratch repo.
- There is no org setting, and `repos.txt` may list any GitHub repo (decision 7).
- Setup clones the hub, asking for a folder that defaults to `~/src/<repo>`, and hub work happens there (decision 3).
- Knowledge stays in the code repos (design unchanged).
- A stats repo holds the report built from GitHub history, and the read-out (decision 20).
- Compass goes public before the pilot, and the bundle-check action stays (decision 21).

**Chosen without asking:** the sync job keeps its rule that the bundle check passed in CI on the knowledge PR head. The user's "sync runs the check itself" answered a no-CI premise that the user then withdrew. The old phase 07 (org move) becomes "publish compass".

**Checked on 2026-10-06:**
- `config.Org` has 5 callers, one each in `internal/hub`, `internal/okfinstall`, `internal/setup`, `internal/stamp` and `internal/syncjob`.
- `config.ParseRepos` takes the org and has 2 callers, in `internal/stamp` and `internal/syncjob`.
- 16 files name `OKF_ORG` or `--org`: 7 in `internal`, 5 in `templates` (2 of them are hub workflows that phase 04 deletes) and 4 in `test`. `assets.go` and `config.env` also carry it.
- `tuan-nng/okf` is public.
- No secret scanner is installed here.

### Session 4 — 2026-10-06
**Trigger:** `/af:plan validate` after the session 2–3 re-plan, with phase files edited but not committed. **Claims checked:** 41, by direct `git`, `gh`, `go` and `grep` runs; landed work re-checked with `go vet ./... && go test ./...` and `./test/setup.sh` (26 passes)
**Verified:** 36 | **Failed:** 5 | **Unverified:** 0
- Failed: phase 04 owns "the first push of compass code" — `origin/master` already equals local `4633439`, with `cmd/compass/main.go` on GitHub; fixed in phase 04
- Failed: phase 05's doc grep "matched 16 lines" — 14 after `9e74c4f`; fixed in phase 05
- Failed: phase 05's decision-13–16 grep misses "Runs in the hub on a schedule" (design:78), "The job runs on a schedule" (research:344) and the check-job lines (design:79,134); pattern widened to 32 lines and "check job" renamed to "the stamper"; fixed in phase 05
- Failed: phase 04's template grep misses `compass hub check --help` "as hub CI does" (`internal/hub/check.go:19`), the skill's "the check job" (`skill/okf/SKILL.md:160`) and the `<org>` wording (`templates/hub/repos.txt:4`); widened to `internal cmd`, 29 lines; and hub defaults now name `stamp` and `sync` as Design does; fixed in phase 04
- Failed: phase 06 stamps `okf-scratch-bundle-check`, which phase 04's scratch hub doesn't list (phase-04:19; `templates/hub/checks.txt:5`), and `compass sync` has no repo filter; fixed in phase 06 (hub row, one-repo temp hub, 422 status shared with "moved" at `internal/stamp/stamp.go:329-335`)
- Decided: phase 04 starts while phase 03 waits on the manual Cursor run — Dependencies
- Re-verified unchanged: caller counts (`config.Org` 5, `ParseRepos` 2, 16 `OKF_ORG`/`--org` files), `internal/app`'s one caller, no `actions/`, both hub workflows, five scratch `okf.yml` placements and the `bundle-check` job id, no `okf-scratch-hub`, Actions access `none`, `tuan-nng/okf` public, local `okf-tools` at `9dcdd60`, state-table rows (research:348-356), design section 9 item 5, phase 10's grep target (design:597), `actionlint templates/*.yml` exits 0, `af plans` 0 findings.

### Session 5 — 2026-10-07 (phase 04 cooked)
**Trigger:** `/af:cook` on phase 04. **Changed:**
- Decision 16 and Risks: the setup-go cache is off, because setup-go ignores a `go.sum` outside `GITHUB_WORKSPACE` (journal `2026-10-07-setup-go-cache-outside-workspace.md`).
- Decision 7: its accepted cost now also covers `stamp` and `sync` writing to any listed repo.
- Dependencies: phase 04 lists 01–02.
**Result:** phase 04 completed; evidence in its Result section.

### Session 6 — 2026-10-07 (scratch hub removed)
**Trigger:** the user decided that compass needs no hub repo of its own, since each team's hub is configured through `compass setup`. **Changed:** decision 4; phase 06 uses a temporary local hub folder made from `templates/hub/` instead of `tuan-nng/okf-scratch-hub`. **Checked:** `stamp.Run` reads only `repos.txt` and `checks.txt` from the hub folder (`internal/stamp/stamp.go:368-377`); `sync-live.sh` already planned its own temporary hub.

### Session 7 — 2026-10-07 (folder-only tests)
**Trigger:** `/af:plan`. The user asked that no test need an extra repo, and that tests run on folders. **Changed:**
- New decision 22.
- Decision 4: phase 05 deletes all six scratch repos.
- M3 and acceptance criteria 4 and 7.
- Phase 06 is rewritten: the stamper's refusal is told apart by re-reading the branch head, not by GitHub's message text, and both writers are tested on a template hub folder. `test/sync-live.sh` and the throwaway protected repo are dropped. Effort goes from 2d to 0.5d, and plan effort from 17d to 15.5d.
- Phase 07 checks an anonymous download of the action instead of a scratch run.
- Phase 08 takes the live stamp-commit checks that phase 06 had.
- Phase 09 states which sync rows are proven live.
- A new Risks entry.

**Checked on 2026-10-07:**
- The writers take `--api-url` (`internal/stamp/stamp.go:416`; `internal/github/github.go` `New`).
- `stamp_test.go` holds 22 tests and `sync_test.go` 25, with Row1–Row7 tests for every state-table row.
- `ref-not-fast-forward.json` and `ref-main.json` exist under `internal/stamp/testdata/fixtures/`.
- The ref update treats 409 and 422 as "moved" (`internal/stamp/stamp.go:332-338`).
- No file outside `plans/` names `okf-scratch`.
- The local scratch clones are under `/mnt/data/works/scratch`.
- Without credentials, `curl -sfL` of compass's codeload tarball exits 22 (HTTP 404) and of `tuan-nng/okf`'s exits 0.

**Decided by the user on 2026-10-07:** phase 05 deletes all six scratch repos and their local clones, together with `okf-tools` and `knowledge-hub`, after one `gh auth refresh -h github.com -s delete_repo`.

### Session 8 — 2026-10-07
**Trigger:** `/af:plan validate` after sessions 6–7 rewrote phases 05–09. **Claims checked:** 44, by direct `git`, `gh`, `go`, `curl` and `grep` runs, the built `compass` help, the pinned `okf`, and GitHub's REST docs; landed work re-checked with `go vet ./... && go test ./...` and `./test/run-all.sh` (7 suites pass)
**Verified:** 39 | **Failed:** 5 | **Unverified:** 0
- Failed: frontmatter effort 15.5d — the phase files sum to 17.5d; fixed in frontmatter
- Failed: phase 06's template-hub test names no row mode — `compass sync` skips folder-mode repos (`internal/syncjob/sync.go:687`); fixed in phase 06 (one `branch` row)
- Failed: phase 06 "the existing moved test still passes" — it registers one ref route returning the same head (`internal/stamp/stamp_test.go:378-384`), so the re-read would take the refusal path; fixed in phase 06 Verification (`ghfake` `Times` makes the sequence possible, `internal/github/ghfake/ghfake.go:68-69`)
- Failed: phase 03's open Verification runs actionlint on `templates/hub/.github/workflows/*.yml`, which phase 04 deleted; fixed in phase 03
- Failed: phase 04's `fetchBundle` finding had no owner — still present (`internal/hub/assemble.go:290-293`), and `test/bench-assemble.sh:18` maps URLs with `insteadOf`, as did `okf-tools/bin/assemble-hub.sh:90`; fixed in Dependencies
- Decided: the `fetchBundle` fix runs as `/af:fix` before phase 08, not inside phase 06 — it is a proven bug outside the writers
- Re-verified unchanged: all six scratch repos, `okf-tools` and `knowledge-hub` exist (private); the token lacks `delete_repo`; compass is private, `master`, Actions access `user`, codeload exits 22; `okf-tools` at `9dcdd60`; 22 stamper and 25 sync tests with Row1–Row7; the 409/422 "moved" path; `--api-url` on both writers; phase 05's grep counts (14, 24+8) and `research:472`; phase 09's `okf/ -> okf/main` line (`test/branch-setup.sh:142`); phase 10's grep target (design:597); "Update a reference" documents 200, 409, 422; `okf show` reports `trust_tier: human-reviewed`; no harness file tracked.

### Session 9 — 2026-10-07 (phases 05 and 06 cooked)
**Trigger:** `/af:cook`. **Result:**
- Phase 06 is completed; the evidence is in its Result section.
- Phase 05 is completed. The user deleted the eight repos on GitHub; the local clones were removed after the checks. Phase 05's Result has the details.
- The docs now describe decision 9's pointer line in place of an open gap.
- Phase 03 is completed. The user has no Cursor access, so the manual Cursor run became `./test/skill-eval.sh --agent claude --scenario read --runs 3` on the current skill, which passed 3 of 3. Cursor itself stays untested (Risks).
