---
title: "OKF knowledge system: build and pilot"
description: "Build every missing piece of the OKF knowledge system design and pilot it on real repos, folder mode first, then branch mode."
status: pending
priority: P1
effort: 20.5d build + 4-week pilot per mode
branch: main
tags: [okf, knowledge, agents, tooling]
created: 2026-09-29
---

# OKF knowledge system: build and pilot

## Overview

**Problem:** The design in `docs/design/okf-knowledge-system-ux.md` is complete on paper, but none of it runs. Its section 9 lists what is missing: a fix for okfcli#34, the agent skill, the real hub assembly and hub CI, the check job, the branch-mode sync job, a hardened setup script, and the pilot.
**Change:** Build those pieces in three new repos: a pinned okfcli fork, a versioned tooling repo, and the hub content repo. Then run a pilot, folder mode first and branch mode one milestone later.
**Why this way:** The tooling lives apart from the knowledge content so the hub can pin a tested tooling version. The rejected alternative put everything in one hub repo, which is simpler but mixes code releases with knowledge review. The main risk: the pilot depends on org permissions for three repos and two GitHub Apps that are not granted yet.

The work is grouped into five milestones. Each one leaves something usable behind:

| Milestone | What works when it closes |
|---|---|
| M1 Pinned toolchain | Any repo can run the CI check. The pinned `okf` handles the spec's datetime `stale_after`. |
| M2 Agent loop in one repo | An agent with the skill reads, trusts, and records knowledge in a folder-mode repo. |
| M3 Cross-repo view | The hub assembles every listed repo nightly and on demand, and fails on broken cross-repo links. |
| M4 Folder-mode pilot | 1–2 real folder-mode repos are live, the check job writes `process:` stamps, and pilot metrics are collected. |
| M5 Branch mode and read-out | A branch-mode repo is live with the sync job, and the pilot read-out decides whether to roll out further. |

This plan lives in `compass` because the design and research live here. Code changes land in the new repos, never in `compass`. The only `compass` edits are to the design doc and the research report, when a phase changes a claim they make.

## Decisions

Confirmed with the user this session:

1. **Code home.** The code goes in a separate tooling repo (`<org>/okf-tools`) and the knowledge in a separate hub content repo (`<org>/knowledge-hub`). The rejected alternative was one `knowledge-hub` repo for both. It has fewer repos but ties tooling changes to knowledge review.
2. **okfcli#34.** We ship a pinned, patched fork (`<org>/okf`) and send the same patch upstream as a pull request. We drop the fork once upstream releases a fix. The rejected alternative was date-only `stale_after`, which needs no fork but departs from the spec (design section 4.6).
3. **Mode order.** Folder mode pilots first (M4) and branch mode joins in M5. The rejected alternative piloted both modes at once, which would delay the first pilot until the sync job exists.

Decided in this plan:

4. **The skill targets the fork, not okfcli v0.5.0.** The fork lands in M1, before the skill (M2). So the skill trusts `okf show`'s `stale` field and runs `okf validate` directly. It drops the manual date comparison and the rule that the okf-skills validator stands in for `okf validate`. Phase 03 updates every place in the design doc that states those workarounds. Repo CI keeps the okf-skills strict validator, because the report chose it as the most spec-faithful check.
5. **The check job runs from the hub, for both modes.** It uses the GitHub API to find a named workflow job on the head commit of the repo's default branch. If that job passed, it stamps the concepts the check covers. The hub keeps the list of covered concepts, and hub CODEOWNERS review it, so a repo pull request cannot add stamps to itself. The rejected alternative ran the job in each repo's own CI after merge, as the design describes for folder mode. That needs a write credential and a workflow file in every repo, and branch-mode repos allow no such file on their code branches. The design doc is updated to match (phase 05).
6. **Two GitHub Apps.** A reader app can only read repo contents; hub pull request CI uses it to assemble repos. A writer app can push contents, manage pull requests and read Actions results; only the sync job and the check job use it. The writer's key lives in a GitHub environment that only the hub's default branch can use. The rejected alternative was one app. Then any branch pushed to the hub could run a workflow that mints a write token for every pilot repo.
7. **Languages.** The two scripts that run on developer machines stay in bash, because both already exist as sketches: the branch-mode setup script and the assembly script. The sync job, the stamper and the pilot metrics script use only the Python 3.11 standard library, so they need nothing beyond `python3`. The rejected alternative was Go binaries, which would need a second release pipeline for internal tools.
8. **The tests own their data.** Phase 01 builds the test data inside `okf-tools`, so nothing depends on `/tmp`. The research data under `/tmp/okf-research` was lost by 2026-10-06 (fixture, prototype, scale set and okfcli binary). So phase 01 rebuilds the small test set from the planted-facts table in `docs/research/okf-knowledge-system/evidence/protocol.md`, and the prototype from the report's conventions (research report sections 4 and 5). A generator in `okf-tools` recreates the scale set. With no reference copy left, it is checked against the counts the protocol states. Phase 01 then re-runs the design doc's commands on the rebuilt data and updates their printed output, so the doc's claim that its output is real stays true (confirmed with the user on 2026-10-06). Rejected alternatives: leaving the old output in place, which is cheaper but lets the doc drift from the tests; and committing the test data to `compass`, which breaks the rule that `compass` holds no code or test data.
9. **Quality targets,** confirmed by the user. Each is checked in the phase that owns it:
   - Pinned `okf` on the 10,000-concept hub: median validate under 10 s, search under 2 s, backlinks under 2 s. These are the report's targets.
   - Assembling 50 repos from local bare remotes: under 5 min from an empty cache, under 60 s when nothing changed.
   - An approved, green knowledge pull request merges within 2 sync runs after its code pull request merges. The sync runs every 30 minutes.
10. **No repo pipeline triggers the hub.** Hub CI runs nightly, on manual dispatch, and on hub pull requests. A hub pull request that waits on a repo change (design section 4.5) goes green when its author re-runs its check after that change merges. The skill tells agents to do this. The rejected alternative was a dispatch from every repo pipeline, as the design proposes. It needs a credential that can write to the hub in every pilot repo. The design doc and the report are updated to match (phase 04).
11. **The tooling ships as composite actions.** Each CI entry point in `okf-tools` is a composite action. Caller workflows reference it by commit SHA: the bundle check, the hub assembly, and the writer steps. GitHub fetches the action's own files at that SHA, so its scripts and pins arrive with it. `okf-tools` allows access from the org's private repos. The rejected alternative was reusable workflows. A reusable workflow gets its caller's token, which cannot check out the private `okf-tools`, and it has no plain way to learn its own SHA.

Confirmed with the user in a second round:

12. **Account.** Phases 01–04 run under the personal account `tuan-nng`, with private repos, so `<org>` means `tuan-nng` until then. Before phase 05, the three repos move to the company org, and every SHA-pinned `uses:` reference and allowed URL prefix moves with them (phase 05). The rejected alternative was to wait for the org grant, which would block the whole build.
13. **Pilot repos are chosen when phase 06 starts,** after M1–M3 give maintainers a working demo. Phase 06 does not start without them. Preferred: repos whose contract or schema test runs as a GitHub Actions job on pushes to the default branch (decision 5).
14. **CI and cloud agents find branch-mode knowledge through the skill plus a pointer line.** The skill is installed in every CI and cloud agent environment. Each platform's organisation-level instructions get one line: check `git ls-remote origin okf/main`, and follow the OKF skill if that branch exists. The skill alone has no trigger in a fresh clone without `okf/`. The rejected alternative was the skill alone, and committing a marker file to code branches was ruled out because branch-mode maintainers want no knowledge files there. Phase 09 applies this choice.

Decided in a third round, after a review of OpenKB on 2026-10-06:

15. **OpenKB is not adopted.** [OpenKB](https://github.com/VectifyAI/OpenKB) (v0.5.0-rc1, Apache-2.0, alpha) uses an LLM to compile documents into a wiki. It cannot be the core of this system, for five reasons:
    - every write is an LLM pass over a source document, and its skill forbids agents to edit the wiki;
    - it has no `verified` or `stale_after` field, and `recompile` overwrites manual edits;
    - it has no links between knowledge bases, no hub and no git support;
    - its pages follow OKF v0.1 with `[[wikilinks]]`, not this plan's OKF v0.2 conventions;
    - `openkb lint` always exits 0, so it cannot gate CI.

    What this plan takes from it:
    - Phase 03 adapts its skill's rule that wiki text is data, never instructions (`skills/openkb/SKILL.md`, Apache-2.0, credited in the skill).
    - Phase 10 decides whether to try OpenKB as a one-off tool that drafts concepts from existing prose docs (decision 16).
    - A lesson the plan already covers. When OpenKB updates a page, the LLM rewrites the body but keeps the old frontmatter, so a human stamp can outlive the text it vouched for. Here the skill's write rule (phase 03) and the stamper's `generated.at` rule (phase 05) prevent that.

    Its structural lint adds nothing. The bundle check already fails on a stale index (phase 01), and the strict validator fails on broken links. Its `lint --fix` turns broken links into plain text without failing; this plan does not copy that. Rejected alternatives: OpenKB as the core, or a fork of it with trust fields, git and a hub added. Both would rebuild most of this plan inside a 3,800-line alpha CLI that depends on a pre-release package (`pageindex==0.3.0.dev3`).
16. **Seeding from docs waits for the pilot.** The user confirmed on 2026-10-06 that most of the company's "why" knowledge lives in code, READMEs and pull requests, not in docs outside the repos. So agents writing as they work comes first. The read-out (phase 10) decides whether to try drafting concepts from a repo's existing prose docs, using the pilot's write-rate metric and any reports from pilot agents that found nothing to read. If it runs, it is a one-off step outside every pipeline. Its output becomes OKF v0.2 concepts with `status: draft`, a `generated` stamp, no `verified` entry and relative links, and goes through normal pull request review. The tool never touches a bundle after that. The rejected alternative was seeding before the pilot. That would mix two effects in the metrics: what agents write during their work, and what was imported.

Decided while validating on 2026-10-06, and confirmed with the user:

17. **One installer sets up every agent environment.** It puts the skill in each agent's user-level skill folder, installs the pinned `okf` through the install script, and installs the scripts the skill calls: the branch-mode setup script and the assembly script. The scripts keep the names the design doc uses, `okf-branch-setup.sh` and `assemble-hub.sh`. Developer machines, CI and cloud agent environments all run the same installer. Phase 03 builds it and copies the setup sketch into `okf-tools` unchanged, so the skill and its fresh-clone test do not wait for phase 07, which then hardens the script in place. Phase 04 adds the assembly script. The rejected alternative made phase 03 wait for phase 07. That ties M2 to branch-mode work, and still leaves CI and cloud agents without the `okf` binary and the scripts the skill calls.

## Design

| Item | States |
|---|---|
| Components | See the component list below. |
| Interfaces | See the interface list below. |
| Data | See the data list below. |
| Flow | Folder mode and branch mode, below. |
| Failure modes | See the failure list below. |
| Quality targets | Decision 9. Each target has a verification entry in the phase that owns it. |
| Trust boundaries | See the trust list below. |

**Components.** Each one owns one job and stays out of the others':

- **`<org>/okf` fork.** Owns reading `stale_after` as a datetime or a date, in both validation and the stale check. Adds nothing else: okfcli#35 and other features stay upstream's business. It publishes release binaries with checksums.
- **`<org>/okf-tools`.** Owns all code, tagged releases and test data:
  - the skill and its installer;
  - an install script that fetches the pinned `okf` and checks its checksum;
  - a vendored copy of the okf-skills validator, pinned to its v0.10.0 commit (MIT license);
  - the bundle-check action;
  - the assembly script and the hub action;
  - the branch-mode setup script;
  - the sync job, the stamper, and the writer action that runs them;
  - the pilot metrics script.

  It holds no knowledge content and no list of repos.
- **`<org>/knowledge-hub`.** Owns the list of repos (`repos.txt`), the list of which concepts each check covers (`checks.txt`), and the cross-repo concepts, decisions and glossary. It also owns thin workflow files that call the `okf-tools` actions, and the writer environment. It never holds code, and its assembled `repos/` folder is never committed.
- **Reader and writer GitHub Apps.** They own the credentials; see decision 6.
- **Pilot repos.** Each owns its bundle: `okf/` on its code branches, or its `okf/…` branches.

**Interfaces:**

- **Pinned `okf`.** The same commands and JSON output as okfcli v0.5.0. `stale_after` accepts an RFC 3339 datetime or a `YYYY-MM-DD` date. `okf show` reports `stale: true` once that point has passed. It is called by the skill, by the bundle check (`okf index`), and by the hub action.
- **Bundle-check action.** Input: the bundle path (`okf`, or `.` on `okf/main`). It fails on a strict-validator finding or an out-of-date index. Callers: each folder-mode repo's CI, and the workflow on each branch-mode `okf/main`.
- **Assembly script** (`assemble-hub.sh <hub-dir>`). Reads `repos.txt` lines of the form `<name> <git URL> <folder|branch>`. It accepts only `https://github.com/<org>/` URLs. It writes `repos/<name>/` and keeps a clone cache between runs. It exits non-zero and names the repo when:
  - fetching fails;
  - the bundle has no `index.md` or `overview.md`;
  - the bundle contains a symlink.

  In CI, any such failure fails the run. Locally, a repo that cannot be fetched keeps its previous copy and gets a warning.
- **Hub action.** Runs nightly, on manual dispatch, and on hub pull requests. It assembles the view, then runs the strict validator and the per-repo concept-count check.
- **Sync job** (`okf-sync`). Runs over every branch-mode repo in `repos.txt`, with a dry-run flag. It carries out the report's state table (research report section 5, "Branch mode"), with the stricter merge conditions under Trust boundaries. It logs one line per `okf/<b>` branch. It keeps no state: every run re-reads GitHub.
- **Stamper** (`okf-stamp`). Each `checks.txt` row gives: the repo, the workflow file, the job name, the `process:` actor, and concept ids. For each row, the stamper:
  - finds the latest successful run of that workflow job, triggered by a push to the default branch, on the branch's head commit;
  - stamps the concepts as they are at that commit, never at a later one;
  - sets `verified.at` to the job's completion time;
  - stamps only concepts whose `generated.at` is at or before that time, and that have no entry from this actor dated at or after `generated.at`. A concept without `generated` is stamped only when it has no entry from this actor;
  - pushes only as a fast-forward of the commit it read;
  - when nothing is due, makes no commit, and logs `no check run on head` if the job never ran there.

  In branch mode the check is on the code default branch, and the stamps go on `okf/main`, fast-forwarded from the `okf/main` commit it read.
- **Pilot metrics script.** Input: the pilot repos and a date window. Output: the five metrics from design section 9, item 6.

**Data:**

- **Concepts.** Owned by each repo, in `okf/` or on `okf/main`.
- **Hub control files:** `repos.txt`, `checks.txt`, `CODEOWNERS` and `.github/`. They belong to the hub, and every change needs a code-owner review (see Trust boundaries).
- **Assembled `repos/` folder.** Generated on each machine or CI run; git ignores it.
- **Clone cache.** Kept per machine under the user's cache directory.
- **Sync state.** None persisted. To avoid repeating comments, each comment carries a hidden marker that the next run looks for.
- **Pilot metrics.** Committed to `compass` at the read-out.
- **Migration.** Nothing to migrate; no bundles exist yet.

**Flow, folder mode:**

1. The agent edits `okf/` in the code pull request.
2. The bundle-check action runs on that pull request.
3. A reviewer approves and may add a `human:` stamp.
4. The pull request merges.
5. The next nightly hub run assembles the new knowledge. A hub pull request that waited on this merge goes green when its author re-runs its check (decision 10).
6. The writer workflow runs every 30 minutes. Its stamper commits `process:` stamps to the default branch through the writer app's ruleset bypass.

Every hop after the merge runs in the background; nothing blocks the developer.

**Flow, branch mode:**

1. The code pull request and the knowledge pull request are opened together. One of them links the other.
2. The workflow on `okf/main` runs the bundle check on the knowledge pull request.
3. A reviewer approves both.
4. The code pull request merges.
5. The next writer run's sync step merges the knowledge pull request with a merge commit.
6. The hub assembles `okf/main`: nightly, or when dispatched manually.
7. The same writer run's stamper step, which runs after the sync step, commits `process:` stamps to `okf/main`.

**Failure modes:**

- **The GitHub API is down or rate-limited.** The sync job and the stamper exit non-zero. The next scheduled run retries. Every action is safe to repeat: a pull request that is already merged is skipped, and a stamp that is already current is not rewritten.
- **Writer runs overlap,** for example a scheduled run and a manual one. The writer workflow uses a single concurrency group that queues runs and never cancels one. So the sync job and the stamper never run at the same time as another run's.
- **A sync action meets a conflict or failing checks.** The job comments once and retries on the next run (report state table). A stamp commit on `okf/main` can conflict with an open knowledge pull request that edits the same concept. It is resolved like any other conflict.
- **The stamper's push races another merge.** The fast-forward fails, and the stamper skips that repo. The next run reads the new head, and stamps only if the check passed on that commit.
- **The check never runs on the default-branch head,** for example because it runs only on pull requests, or path filters skip it. Nothing is stamped, and every run logs `no check run on head` for that row (see Risks).
- **A concept id in `checks.txt` is missing,** for example after a move. The stamper fails and names the row, and hub CODEOWNERS are notified.
- **A `verified` value is in a form the stamper does not handle.** The stamper fails on that concept and does not rewrite it.
- **A repo cannot be fetched or its bundle is empty.** The hub action fails and names the repo. This covers the silent false pass the report saw on symlinked hubs (research report section 3, finding 1).
- **The sync job stops.** Knowledge pull requests stay open. The pilot metrics script reports any still open more than a day after their code merged (design section 8).
- **Upstream okfcli ships a different fix.** We move back to upstream once it accepts datetimes. We write datetimes only, so our data survives either behavior.

**Trust boundaries:**

- **The writer key.** It can push to every pilot repo. It is usable only from the hub's default branch, through the writer environment (decision 6). That protects nothing if unreviewed code can reach the default branch. So:
  - a hub ruleset blocks direct pushes to the default branch and requires a code-owner approval;
  - CODEOWNERS covers `.github/`, `CODEOWNERS`, `repos.txt` and `checks.txt`, because these files decide what runs with the key and what it touches.
- **The reader key.** Hub pull request CI runs the pull request's own workflow files and `repos.txt` before review. So anyone with write access to the hub can read the reader key. Mitigations:
  - the reader app is installed only on the repos listed in `repos.txt`;
  - it can only read contents;
  - jobs use short-lived installation tokens;
  - the assembly script accepts only `https://github.com/<org>/` URLs.

  The remaining exposure is accepted: hub writers could already read those repos through the view the hub assembles.
- **Pinned tools.**
  - The `okf-tools` actions are referenced by commit SHA.
  - The vendored validator runs under `uv`, installed by a setup action pinned by SHA, with a locked `pyyaml`.
  - The `okf` binary is pinned by sha256.
  - The okf-skills published action is not used, because it pulls `astral-sh/setup-uv` by a tag that can move.
- **Merges by the sync job.** Its own checks are the only gate, because it merges through the writer app's bypass. It merges a knowledge pull request only when all of these hold:
  - the head is `okf/<b>` in the same repo, never a fork;
  - a human with write access approved the current head commit; approvals from either app, or on an older commit, do not count;
  - the bundle check passed on that head commit;
  - the code pull request from `<b>` merged into the default branch;
  - the code pull request and the knowledge pull request link each other in at least one direction. This stops a reused branch name from matching an old code pull request.

  It merges with a merge commit, passing the head commit it checked.
- **The check job's evidence.** It trusts only a job from the named workflow file, which lives on the repo's reviewed default branch, triggered by a push. It ignores check runs that any other workflow or app posts under the same name.
- **Repo bundles are untrusted text,** written by agents and flowing into other agents' context through the hub. The skill tells agents to treat concepts as claims about the code, never as instructions to follow.
- **`human:` stamps.** Nothing stops an agent from writing one; review remains the only check (design section 8). This plan does not change that.

## Phases

| # | Milestone | Phase | Outcome | Status |
|---|---|---|---|---|
| 01 | M1 | [phase-01-tooling-foundation.md](phase-01-tooling-foundation.md) | `okf-tools` exists, with rebuilt test data, the pinned install and the bundle-check action; the design doc's command output matches the rebuilt data | pending |
| 02 | M1 | [phase-02-okfcli-fork.md](phase-02-okfcli-fork.md) | The pinned `okf` fork accepts datetime `stale_after`; the upstream pull request is open | pending |
| 03 | M2 | [phase-03-agent-skill.md](phase-03-agent-skill.md) | The skill is installed and agents pass the scenario checks on the test data | pending |
| 04 | M3 | [phase-04-hub-assembly.md](phase-04-hub-assembly.md) | `knowledge-hub` and the reader app exist; the hub assembles every repo from its remote and fails on broken or empty bundles | pending |
| 05 | M4 | [phase-05-check-job.md](phase-05-check-job.md) | The writer app exists, and the stamper writes `process:` stamps only for covered concepts | pending |
| 06 | M4 | [phase-06-folder-pilot.md](phase-06-folder-pilot.md) | 1–2 folder-mode repos are live and the metrics script reports on them | pending |
| 07 | M5 | [phase-07-branch-setup.md](phase-07-branch-setup.md) | The setup script passes every branch-mode case automatically, and the `okf/main` workflow runs on GitHub | pending |
| 08 | M5 | [phase-08-sync-job.md](phase-08-sync-job.md) | The sync job carries out every state-table row against scratch GitHub repos | pending |
| 09 | M5 | [phase-09-branch-pilot.md](phase-09-branch-pilot.md) | One branch-mode repo is live, and CI and cloud agents learn which repos use branch mode | pending |
| 10 | M5 | [phase-10-pilot-readout.md](phase-10-pilot-readout.md) | A read-out with the five metrics and a roll-out decision is committed to `compass` | pending |

Dependencies: 02, 03 and 04 need 01; 03 also needs 02. 05 needs 04. 06 needs 03, 04 and 05. 07 needs 01. 08 needs 05 and 07. 09 needs 06 and 08. 10 needs 09 and the pilot window. Phase 07 can run in parallel with 02–06.

Scope: ten phases across four repos. That is more than the usual three phases, and it is justified. Each phase closes one item in design section 9, or one pilot step, and has its own exit check. Merging phases would hide which item is done.

## Non-Goals

- Enforcing who writes a `human:` stamp, for example by matching logins against pull request approvers (design section 4.3 names this as a later check).
- Removing a `process:` stamp when its check later fails. The stamp keeps its date, and `stale_after` still ages the concept.
- okfcli#35. The skill reads no custom frontmatter keys.
- Tooling that moves concepts. Moves stay the manual, human-led `okfctl node mv` step (design section 4.7).
- Git for Windows support for the setup script and hooks.
- Any dependency on OpenKB, or on a tool that calls an LLM, in the skill's commands, the CI actions or the hub jobs (decision 15).
- An MCP server or any agent interface other than the CLI and the skill.
- Rolling out beyond the pilot repos. The read-out decides that.

## Acceptance Criteria

1. The pinned `okf` validates the spec's datetime `stale_after` without error, and `okf show` reports `stale: true` once that datetime has passed.
2. In a folder-mode pilot repo, an agent with the skill runs through design sections 4.1–4.3. It reads with trust rules and records 1–3 knowledge files in the code pull request. The bundle check passes, and a reviewer's `verified` line makes `okf show` report `human-reviewed` after merge.
3. The hub assembles every repo in `repos.txt` from its remote. It fails on a broken cross-repo link, on an empty bundle, and on a symlink. `okf backlinks` on the hub returns the cross-repo dependents.
4. The stamper adds or refreshes `process:` stamps only on concepts listed in `checks.txt`, only after the named check passed, and makes no commit when stamps are current.
5. In a branch-mode repo, the setup script passes every automated case, the `okf/main` workflow runs on knowledge pull requests, and the sync job carries out each state-table row.
6. The read-out reports all five pilot metrics for at least one repo in each mode, over a window of at least four weeks, and states a roll-out decision.

## Verification

Each phase file holds its own exit commands. Plan-level checks, run from the `okf-tools` checkout:

- `./test/run-all.sh` exits 0. It runs the pinned-`okf` checks, the assembly tests, the setup-script cases, and the sync and stamper tests.
- `gh run list -R <org>/knowledge-hub -w hub -L 1 --json conclusion -q '.[0].conclusion'` prints `success`.
- `python3 bin/okf-pilot-report --repos pilot.txt --since <pilot start>` prints a value for each of the five metrics, for at least one repo in each mode.

## Risks

- **The company org grant arrives late.** Assumption: before phase 05, the user can move the three repos to the company org and install two GitHub Apps there, and pilot maintainers accept the writer app's bypass. Signal: phase 05 cannot transfer a repo or install an app. Response: adjust in place. M1–M3 stay usable under the personal account, and M4 waits for the grant.
- **Upstream disagrees with the fork.** Assumption: upstream accepts datetimes the same way. Signal: the upstream pull request is rejected, or a release parses them differently. Response: keep the fork pinned and adjust in place; return to planning only if upstream rejects datetimes outright.
- **Skill auto-loading.** Assumption: each agent loads a user-level skill when the task matches (design section 4.1 marks this untested). Signal: an agent in phase 03 skips the skill even though the repo's `AGENTS.md` line points at it. Response: adjust in place and make the pointer line the main trigger. Return to planning if Cursor cannot load user-level skills at all.
- **The `okf/main` workflow does not run on GitHub.** Signal: phase 07 opens a pull request into `okf/main` and no check starts. Response: return to planning for branch-mode CI, for example running the bundle check from the hub over open knowledge pull requests.
- **The ruleset bypass is refused.** Signal: a pilot maintainer declines to let the writer app push to the default branch or to `okf/main`. Response: adjust in place. That repo gets no `process:` stamps during the pilot, and the read-out reports it.
- **Assembly is slower through GitHub than locally.** Decision 9 measures against local bare remotes only. Signal: the hub workflow takes more than 10 minutes with the pilot repos. Response: adjust in place with shallow partial clones or by fetching repos in parallel.
- **The check never runs where the stamper looks.** Assumption: each covered check runs as a GitHub Actions job on pushes to the default branch. Signal: a `checks.txt` row logs `no check run on head` for a week, and stamps nothing. Response: adjust in place. Change that repo's workflow triggers with its maintainers, or remove the row.
- **CI and cloud agents miss branch-mode knowledge.** Assumption: the pointer line in each platform's instructions makes agents load the skill in a fresh clone that has no `okf/` (decision 14). Signal: phase 03's fresh-clone test with the line, or any phase 09 agent run, never runs the setup script. Response: return to planning for how branch-mode repos announce themselves.

## Unresolved Questions

None block phase 01. Two inputs have fixed deadlines:

1. **The company org name, and who approves app installs.** Needed before phase 05 (decision 12).
2. **The pilot repo names, with their maintainers' agreement.** Needed before phase 06 (decision 13).

## Validation Log

### Session 1 — 2026-09-29
**Trigger:** the plan carries `## Design` and phase files, so it got an adversarial pass, done by an independent read-only reviewer. **Claims checked:** 33
**Verified:** 24 | **Failed:** 2 | **Unverified:** 7 (GitHub and agent behavior; each is checked by a phase verification or a Risks entry)
- Failed: "`/tmp/okf-research/scale` is already gone". It exists: 50 repos, 79 MB. Fixed in decision 8 and phase 01.
- Failed: "`IsStale` is used by `show` and `list`". Only `runShow` calls it (okfcli v0.5.0 `cmd/okf/main.go:404`). Fixed in phase 02.
- Decided: tooling ships as composite actions, not reusable workflows. Added decision 11.
- Decided: hub control files sit under CODEOWNERS, with a code-owner ruleset on the hub. Changed trust boundaries and phase 04.
- Decided: the stamper stamps only the commit whose check passed, pushes only as a fast-forward, and finds the check by workflow file plus job name. Changed the stamper interface, failure modes and phase 05.
- Decided: the sync job requires an approval of the head commit, a check that passed on it, a code pull request based on the default branch, and a link between the two pull requests. Changed trust boundaries and phases 08 and 09.
- Decided: the uv requirement is dropped for the Python scripts, and the validator is vendored with pinned uv and pyyaml. Changed decision 7 and trust boundaries.
- Decided: Unresolved Question 4 now recommends a pointer line plus the skill, and phase 03 tests a fresh clone. Added a Risks entry.

### Session 2 — 2026-09-29
**Trigger:** the user answered the open questions. **Claims checked:** 0
- Decided: personal account for phases 01–04, and a move to the company org before phase 05. Added decision 12, changed Risks and phase 05.
- Decided: pilot repos are chosen when phase 06 starts. Added decision 13.
- Decided: the quality targets stand. Changed decision 9.
- Decided: skill plus an organisation-level pointer line for CI and cloud agents. Added decision 14, changed Risks and phases 03 and 09.

### Session 3 — 2026-10-06
**Trigger:** OpenKB (`/mnt/data/works/OpenKB`, HEAD `ff54396`) was reviewed as a possible base, and this plan was amended. **Claims checked:** 9
**Verified:** 8 | **Failed:** 1
- Failed: decision 8's premise that the test data and the scale set are still under `/tmp/okf-research`. The whole folder is gone. Decision 8 and phase 01 now rebuild the data from the protocol, which specifies the planted cases and the scale set's counts.
- Verified in OpenKB's source: writes go only through the LLM (`openkb/agent/tools.py`, `openkb/agent/compiler.py`); `recompile` overwrites edits (README); no `exit` call in the lint command (`openkb/cli.py`); lint describes its format as OKF v0.1 (`openkb/lint.py`); its skill's untrusted-data rule (`skills/openkb/SKILL.md`); there is no git use except for reading the user's name and email (`openkb/skill/marketplace.py`); the maturity data (175 commits since 2026-04-06; last commit 2026-07-22).
- Decided: OpenKB is not adopted (decision 15), and seeding from docs is a phase 10 decision (decision 16). Changed Non-Goals and phases 01, 03 and 10. Added a row to research report section 7.

### Session 4 — 2026-10-06
**Trigger:** the user answered five questions. **Claims checked:** 1 (the design doc has three command blocks taken from the lost data: two in section 4.1 and one in section 4.4)
- Decided: OpenKB stays out, and seeding is a read-out decision. Most "why" knowledge lives in code, READMEs and pull requests. Changed decision 16.
- Decided: phase 01 rebuilds the test data, then refreshes the design doc's command output from it. Changed decision 8 and phase 01 (effort 2.5d).
- Unchanged: the company org stays open until phase 05 (decision 12), and pilot repos are chosen after the M1–M3 demo (decision 13).

### Session 5 — 2026-10-06
**Trigger:** `/af:plan validate`; the plan has phase files and was last edited in another session. **Claims checked:** 44
**Verified:** 36 | **Failed:** 5 | **Unverified:** 3 (live GitHub behavior, each checked by a phase exit command: the `okf/main` workflow trigger, the rules API on `okf/main`, the environment refusal)
- Verified against live sources: okfcli#34 and #35 still open, v0.5.0 still latest; `IsStale` at `internal/concept/v02.go:163`, `validateV02` at `internal/validate/v02.go:30`, `runShow` at `cmd/okf/main.go:404` (v0.5.0); okf-skills `8e31878` is v0.10.0, MIT, and its `action.yml:36` uses `astral-sh/setup-uv@v5`; OpenKB facts in decision 15.
- Failed: phase 07 said the setup script adds three failures; the sketch already has them (`okf-branch-setup.sh:18-19,36-38,49-51`). Fixed in phase 07.
- Failed: phase 04 listed the Gist as promising a repo-pipeline trigger; it does not (`okf-knowledge-system-ux.md:19-21`). The rows that need changes are in section 8 (`:619`, `:624`). Fixed in phase 04.
- Failed: phase 03's grep missed two workaround lines (`okf-knowledge-system-ux.md:217`, `:587`). Fixed in phase 03.
- Failed: the planted-facts table shortens `stale_after` to a date (`protocol.md:25`); the fixture held a datetime (`okfcli.md:43`), which the section 4.1 output needs. Fixed in phase 01.
- Failed: no phase owned `test/bench-scale.sh` (`phase-02-okfcli-fork.md:37`). Fixed in phase 02.
- Decided with the user: one installer for the skill, `okf` and the scripts (decision 17). Changed Design and phases 03, 04, 06, 07 and 09.
- Decided with the user: phase 03 reads upstream okfcli#37, an open pull request adding an agent skill, for reuse, but does not depend on it. Changed phase 03.
- Unchanged: the org name and the pilot repos stay open until phases 05 and 06.
- Contract gaps fixed: phase 03 re-runs the 4.1 `okf show` block with the fork; phase 08 counts writer runs, not minutes, for decision 9, and lists the closed-unmerged row; phase 07 adds the design doc's section 2 Repo CI row; phase 05 adds section 4.8's folder-mode steps; phase 01's scale counts name `repos/`.
