---
title: "Hub-only knowledge: one hub branch per code branch"
description: "All knowledge lives in the hub; repo X's branch A pairs with hub branch X/A, and compass sync merges its approved hub pull request after A's code pull request merges and a local consistency check passes."
status: completed
priority: P1
effort: 6.5d
branch: master
tags: [okf, knowledge, hub, sync, cutover]
created: 2026-10-21
blocks: [261006-1358-okf-knowledge-system]
---

# Hub-only knowledge: one hub branch per code branch

## Overview

**Problem:** Setting up compass is too much work. Each repo picks a mode, then gets an `okf/` bundle or an `okf/main` branch, a CI workflow with two placeholders, an agent pointer line, git hooks in branch mode, and a ruleset bypass for hub owners. Then every reader assembles the hub from all the repos before a cross-repo query.
**Change:** All knowledge moves into the hub repo, under `repos/<id>/`, next to the cross-repo concepts. When an agent works in repo X on branch A, it writes to hub branch `X/A` in a local worktree of the hub and opens a hub pull request. Once A's code pull request merges, `compass sync`, run by the agent, checks consistency locally and merges the approved hub pull request. Adding a repo becomes one line in `repos.txt`. Folder mode, branch mode, assembly, the per-repo bundle check and its action are all removed.
**Why this way:** The user chose hub-only storage, a human approval before every merge, a hub as the access boundary, and a merge check inside `compass sync` (Decisions 1–4). The strongest rejected alternative kept folder mode: knowledge is reviewed in the code pull request, but every repo keeps its per-repo setup. The main risk accepted is that knowledge is reviewed in a separate hub pull request that reviewers may ignore, so knowledge waits unmerged after its code ships.

This plan blocks phases 08–10 of `261006-1358-okf-knowledge-system`, the pilot. Those phases assume the two modes. Phase 04 here rewrites them for the hub-only design. The pilot has not started, so no real repo, hub or bundle has to migrate.

The research report already listed this design as the fallback. It names "One central knowledge repo with no per-repo bundles" in section 7, and its "Better approaches" note says moving bundles costs little, because links inside a bundle are relative.

## Decisions

Confirmed with the user on 2026-10-21:

1. **Hub-only storage, both modes removed.** Knowledge for repo `<id>` lives at `repos/<id>/` on the hub's default branch, committed like any other file. Hub-level concepts stay outside `repos/`. The links `/repos/<id>/…` in the hub's own concepts keep their meaning, so the hub's link format does not change. Rejected: keeping folder mode next to the hub, which keeps two modes in the skill, the docs and the tests, plus per-repo CI.
2. **A human approves every hub pull request before sync merges it.** The approval must come from a human with write access to the hub, on the pull request's current head, as sync requires today. Rejected: the agent merges without approval, which puts unreviewed agent text on the default branch and needs a bypass for every developer.
3. **The hub is the access boundary.** Anyone who can read the hub reads every listed repo's knowledge. A team that needs separation runs one hub per access group. No code enforces this; the hub template and the install guide say it. Rejected: per-repo access, which cannot work while knowledge sits in one shared repo.
4. **The consistency check runs locally, inside `compass sync`.** Before each merge, sync builds the merge of the hub's default head and the pull request head, then checks the result. Rejected: a hub CI workflow on hub pull requests. Assembly is gone, so it would need no GitHub App, it would check GitHub's merge ref, and a ruleset could require it. But it brings Actions back to the hub, against the local-first rule (old plan decision 15). A hub pull request merged by hand therefore goes unchecked, which the skill and the hub template cover by asking the reviewer to run `compass hub check` before approving, as today.

Decided in planning:

5. **Pairing is by name and by link.** Hub branch `<id>/<b>` pairs with branch `<b>` of the repo that `repos.txt` names `<id>`. The hub pull request must also name the code pull request, as `owner/repo#N` or its URL. A code pull request that names the hub pull request the same way also counts. A short `#N` no longer counts, because the two pull requests live in different repos. Today's link matcher resolves a reference against the target's repo, so each direction is checked against the other pull request's repo. The link keeps today's protection against a reused branch name, where a new hub branch would pair with an old merged code pull request. Rejected: the name alone, which merges stale knowledge when a branch name is reused.
6. **compass never writes to a code repo.** Sync only reads from code repos: pull request state, branch existence and pull request bodies. The stamper only reads the default branch, its head, and workflow runs and jobs there. Every comment goes on the hub pull request. Where sync used to comment on the code pull request because a knowledge pull request was missing, it now reports the branch in its own output and does nothing else. Rejected: commenting on code pull requests, which needs write access to every repo and brings back the per-repo agreement this plan removes.
7. **The writers find the hub from the clone and read control files from its fetched default branch.** `compass sync` and `compass stamp` find the hub repo by parsing the clone's `origin` URL, so `--hub DIR` and the recorded clone behave the same. They read `repos.txt` and `checks.txt` from the fetched default branch, never from the working tree, so a stale, dirty or switched checkout cannot change which repos they read or which paths they allow. `compass hub branch` reads `repos.txt` the same way. Rejected: reading `OKF_HUB` from the user config, which a `--hub` override would contradict.
8. **One command opens the hub branch: `compass hub branch`.** Run it in a clone of a listed repo. It prints the path of a hub worktree on `<id>/<b>`, creating it on the first run. The skill uses that path for all reading and writing during the task. `compass hub branch --prune` removes this machine's worktrees whose hub branch is gone from the remote and that hold no uncommitted or unpushed work; the skill runs it after `compass sync`. Rejected: pruning on every run, which hides a destructive step inside a command that prints a path; and `compass sync` removing worktrees, which ties the writer to one machine's layout.
9. **`compass hub check` takes over the index check and refuses links.** It now also fails when any `index.md` differs from what `okf index` would write, and when any symlink exists anywhere in the hub. It never modifies the folder it checks. It drops the "run `compass hub assemble` first" precondition, so a new hub with no `repos/` passes. Rejected: keeping `compass bundle check` for the index, which is a second command for a check that only makes sense hub-wide now.
10. **The stamper commits to the hub's default branch.** It keeps the `checks.txt` format and its gate: the named job passed on the code repo's default-branch head, which it only reads. It reads and writes each concept at `repos/<id>/<concept>.md`, chains every stamp of a run into commits on the hub, and moves the hub's default branch with one fast-forward. Its runner still needs a ruleset bypass, now on the hub alone. Rejected: one fast-forward per repo, as today, which races itself once every repo writes to the same branch.
11. **Sync refuses only findings the branch adds.** It checks the default head and the merge result, and blocks the merge only on findings the merge result has that the default head lacks. Rejected: requiring a clean merge result, which lets one broken default branch block every sync merge until someone hand-merges a fix.
12. **Origin matching reuses setup's URL handling.** `internal/setup`'s `githubRepo` and `withoutUserinfo`, and the origin read in its `existingClone` that tries the configured URL before the `insteadOf`-rewritten one, move to `internal/config`, which already parses GitHub URLs. Setup, `compass hub branch` and the hub-identity helper (decision 7) all call them, so the https, `ssh://`, `git@github.com:` and token-carrying forms, and the raw-then-rewritten order, behave the same in every command. Rejected: a second matcher in `internal/hub`.

## Design

**Components:**

- **`compass hub check`** (`internal/hub`). Runs the strict validator over the hub, the per-repo concept count, and decision 9's index and symlink checks. It owns no git or GitHub work.
- **`compass hub branch`** (`internal/hub`, new). Maps the current clone to a `repos.txt` row and opens or reuses the hub worktree for the current branch (decision 8). It runs git only: no GitHub API and no push.
- **`compass sync`** (`internal/syncjob`, rewritten). Runs the hub state table below. It reads code repos, writes only to the hub, and builds and checks the merge result from the local hub clone.
- **`compass stamp`** (`internal/stamp`, retargeted per decisions 7 and 10).
- **`internal/config`.**
  - **One `repos.txt` parser** for two columns, `<id> <https URL>`. A line with any other number of fields fails with its line number.
  - **Id rules:** today's character rule, and an id must be a valid single git ref component, so no leading `.` and no `.lock` suffix.
  - **Duplicates:** ids that differ only in case, and two rows naming the same owner/repo, are both refused.
  - **Removed:** the `Mode` field, and the assembly parser in `internal/hub`.
  - **Moved in:** it holds the hub-identity helper (decision 7) and the URL helpers (decision 12).
- **The skill and the pointer line** (`skill/okf/SKILL.md`, `templates/user-pointer.md`). They describe one flow: `compass hub branch`, read and write in that path, commit, push, open a hub pull request that names the code pull request, and run `compass sync` when a session starts and when the developer says the code pull request merged. Code repos carry nothing, so the user-level or organisation-level pointer line is the only trigger.
- **The hub template** (`templates/hub/`). `repos/` is no longer ignored, and `repos.txt` has two columns. `overview.md` describes the hub-only flow, the access boundary, and the write access and ruleset in Trust boundaries.
- **Removed:**
  - `compass bundle check`, `compass hub assemble` with its clone cache, and `compass branch setup`;
  - `internal/bundlecheck` and `internal/branchsetup`;
  - `actions/bundle-check/`;
  - the templates `folder-workflow.yml`, `okf-main-workflow.yml`, `hooks/` and `AGENTS-pointer.md`;
  - the test suites `bundle-check.sh`, `branch-setup.sh`, `assemble.sh` and `bench-assemble.sh`.

**Interfaces:**

- **`repos.txt`:** `<id> <https://github.com/<owner>/<repo>[.git]>`, with `#` comments, under the id rules above.
- **`compass hub branch [--hub DIR] [--prune]`:**
  - **Where it runs:** inside a clone of a code repo. `--hub` defaults to the recorded clone.
  - **Matching:** it matches the clone's `origin` against `repos.txt` from the hub's fetched default branch (decisions 7 and 12), owner/repo case-insensitively.
  - **Default branch:** the code repo's default branch comes from `origin/HEAD`, falling back to `git ls-remote --symref origin HEAD`.
  - **Output:** on success, one line on stdout with the absolute worktree path, exit 0.
  - **Branches:** an existing local `<id>/<b>` is reused. Otherwise a remote `<id>/<b>` is tracked, or a new branch starts from the hub's remote default head.
  - **Worktree folders:** they sit in a folder next to the hub clone, one per hub branch.
  - **Errors (exit 1, each naming its cause):**
    - origin not listed, which prints the URL;
    - detached HEAD;
    - the current branch is the repo's default branch, which names the hub clone to read from and says to write through an ordinary hub branch;
    - a hub ref that conflicts with `<id>/<b>`, such as a kept `<id>/feat` blocking `<id>/feat/x`, or an id equal to an existing hub branch name; it names the conflicting ref;
    - no hub clone recorded or given.
  - **`--prune`:** it prints each worktree it removes and each one it keeps, with the reason.
- **`compass sync`:** flags unchanged: `--hub`, `--dry-run`, `--token-env`, `--api-url`, `--ignore-login`, `--now`. It scans hub branches `<id>/*` for each `repos.txt` id. For each one, it finds the latest code pull request from `<b>` in repo `<id>`, the open hub pull request from `<id>/<b>`, and the latest closed one, then applies the first matching row:

  | Code pull request for `<b>` in repo `<id>` | Hub side | Action |
  |---|---|---|
  | any | no open hub pull request, and the latest closed one was merged by any method | delete the hub branch once `<b>` is gone from the remote |
  | none, `<b>` on the remote | any | nothing |
  | none, `<b>` gone, hub branch's last commit ≤ 7 days old | any | nothing |
  | none, `<b>` gone, > 7 days | any | comment on and close the hub pull request if one is open; delete the hub branch |
  | open | any | nothing |
  | closed unmerged, `<b>` on the remote | any | nothing |
  | closed unmerged, `<b>` gone | any | comment on and close the hub pull request; delete the hub branch |
  | merged into a base other than the default branch | any | nothing |
  | merged | hub pull request open | try to merge (below) |
  | merged | no open hub pull request, the latest one closed unmerged | report "hub pull request rejected" in sync's output; delete the hub branch once `<b>` is gone |
  | merged | no hub pull request ever, branch has commits not on the hub default | report "needs a hub pull request" in sync's output |
  | merged | no hub pull request ever, nothing left to merge, `<b>` on the remote | nothing |
  | merged | no hub pull request ever, nothing left to merge, `<b>` gone | delete the hub branch |

  Try to merge: merge only when every condition in Trust boundaries holds. Otherwise post one comment per blocker on the hub pull request and retry on the next run. Exit 0 when every repo synced, 1 when any repo failed, 2 on config errors, as today.
- **`compass hub check [<hub-dir>]`:** decision 9. Exit 0 or 1, naming each finding.
- **`compass stamp`:** flags and `checks.txt` unchanged; decisions 7 and 10.

**Data:**

- **Concepts** belong to the hub: `repos/<id>/` for each repo, hub-level concepts elsewhere. `okf index .` at the hub root also writes `repos/index.md`, which lists the repo folders (checked with the pinned `okf` on 2026-10-21). A bundle that lives outside the hub moves in by copying its contents into `repos/<id>/`, removing the frontmatter from its root `index.md`, and running `okf index .` at the hub root. No real bundle needs this today.
- **Hub branches** `<id>/<b>` are created by a developer's agent and deleted by sync.
- **Worktrees** are local to each machine and removed by `compass hub branch --prune`.
- **Control files:** `repos.txt`, `checks.txt` and `CODEOWNERS` stay under code-owner review.
- **Sync state** is not persisted; comment markers keep comments from repeating. The user config is unchanged.

**Flow:**

1. In repo X on branch A, the agent runs `compass hub branch`, which is local and synchronous. It reads and writes concepts under `repos/X/` and hub-level paths in the printed worktree.
2. The agent runs `okf index .` and `compass hub check` in the worktree, commits, pushes `X/A`, and opens a hub pull request into the default branch that names A's code pull request.
3. A human with hub write access reviews and approves it, at any time before or after the code pull request merges.
4. The code pull request merges in X.
5. At its next session, or when told the code merged, an agent runs `compass sync`, then `compass hub branch --prune`. For each branch that is ready, sync:
   - reads X's pull request state;
   - fetches the hub;
   - applies the path and file-mode rules;
   - builds the merge result without running the user's git hooks or filters, and checks it against the default head (decision 11);
   - re-reads the default head;
   - merges through the API with the head SHA it checked.
6. A hub owner's `compass stamp` stamps covered concepts on the hub's default branch.

**Failure modes:**

- **GitHub is down, rate-limited, or a code repo can't be read with the caller's token.** That repo's branches fail with the reason, the others continue, and sync exits 1. Every action is safe to repeat. A developer who can't read every listed repo sees exit 1 at every session start; the skill treats a per-repo read failure as a report, not a stop.
- **Two sync runs race.** The merge passes the checked head SHA, so the later one gets 405 or 409 and retries next run. A comment may be posted twice if both runs read before either writes, as today.
- **The default branch moves between the check and the merge.** Sync re-reads the default head right before merging and skips the branch this run if it moved. A move inside the remaining window merges an unchecked result. The next `compass hub check` anyone runs reports it (Risks).
- **The default branch is already broken.** Decision 11 keeps sync merging branches that add no new findings. The fix itself is a hub-level pull request that a reviewer merges by hand after running `compass hub check`.
- **The merge conflicts**, most often on generated `index.md` files when two branches add concepts to one folder. Sync comments "conflict". The agent merges the default branch into `X/A`, re-runs `okf index .`, and pushes. Approval is tied to the head, so the new head needs approval again (Risks).
- **The check fails on the merge result.** Sync comments with the new findings and doesn't merge.
- **A branch breaks the path or file-mode rules** (Trust boundaries). Sync comments with the paths and doesn't merge, and never runs the check on that branch.
- **Nobody runs sync.** Approved hub pull requests stay open after their code merges. The pilot measures this as before.
- **A repo is removed from `repos.txt`.** Its hub branches are no longer scanned, and its open hub pull requests and `repos/<id>/` stay until someone removes them by hand.
- **A code pull request comes from a fork.** It never pairs, as today. Its hub branch closes as abandoned once the 7 days pass. The skill says so.
- **A worktree holds uncommitted or unpushed work when its branch is gone.** `--prune` keeps it and prints its path.

**Quality targets:**

- **`compass hub check`** on the 10,000-concept scale hub, including the index and symlink checks: median under 10 s, old plan decision 12's validate target. On 2026-10-21, an ad hoc run on the bench's scale hub took 1.25 s without the index check, and copying plus `okf index` on a copy took 1.3 s. Checked by `test/bench-scale.sh`, which phase 01 extends to time `hub check`.
- **`compass hub branch`** on the scale hub: first run under 5 s, reuse under 1 s. Checked by `test/bench-scale.sh`.
- **`compass sync` reads** at most 5 GitHub requests for a hub branch whose code pull request is open. A sweep over 50 such branches therefore stays near 250 requests, well inside the 5,000-per-hour authenticated limit. Checked by counting the fake's calls in the sync tests.

**Trust boundaries:**

- **Code repos are read-only to compass** (decision 6). Sync and stamp tests assert that the fake GitHub records no write under any code repo's path.
- **Hub write access.** Every developer whose agent writes knowledge needs push access to the hub, for `<id>/<b>` branches. Hub write therefore no longer marks a hub owner. The approval rule (decision 2) and the control-file review hold against people only where a ruleset on the hub's default branch requires pull requests, code-owner review for the control files, and no direct pushes. Rulesets need a public hub or a paid plan (old plan Risks, GitHub Free). The template and the install guide state this as a setup requirement.
- **Writer tokens.** Writes use the caller's token through `--token-env`. Sync needs write access to the hub only. A merge of an approved pull request satisfies the ruleset without a bypass. The stamper still needs a bypass on the hub's default branch.
- **Sync merges a hub pull request only when every condition holds:**
  - its head is `<id>/<b>` in the hub repo itself, never a fork, and `<id>` is listed in `repos.txt` on the fetched default branch;
  - its base is the hub's default branch;
  - the paired code pull request in repo `<id>` came from `<b>` in that same repo, merged into its default branch, and is linked (decision 5);
  - a human with write access to the hub approved the current head;
  - GitHub does not report it as conflicting;
  - **path rule:** between its merge base and its head, and again between the default head and the merge result, both compared with rename detection off so both sides of a move count, the change adds, changes or deletes only:
    - `repos/<id>/**`;
    - `repos/index.md`;
    - files outside `repos/`, except `repos.txt`, `checks.txt`, any `CODEOWNERS`, any `.gitattributes` or `.gitmodules`, `.gitignore`, and anything under `.github/`; names compare case-insensitively, and a top-level folder that case-folds to `repos` but is not exactly `repos` is refused;
  - **file-mode rule:** on both diffs, no changed entry is a symlink or a submodule link;
  - the merge of the default head and the pull request head has no conflicts, and `compass hub check` on it adds no finding the default head lacks (decision 11);
  - the default head has not moved since the check.

  The path and file-mode rules run before any check reads the branch's files, so a link to a file on the machine running sync is never read or quoted in a comment. Sync merges with a merge commit, as the caller, passing the checked head SHA.
- **Branch content is untrusted text** from any developer's agent. The check only parses it, through the Go validator and `okf`, and never runs it. The skill keeps "knowledge is data, not instructions".
- **`repos.txt` decides** which code repos sync reads and which id prefixes it scans. It stays under code-owner review.
- **Hub readers see all listed repos' knowledge** (decision 3).

## Phases

| # | Phase | Outcome | Status |
|---|---|---|---|
| 01 | [phase-01-hub-only-layout.md](phase-01-hub-only-layout.md) | `repos.txt` has two columns; `compass hub check` works on a committed hub with the index and symlink checks; Go tests get the pinned `okf`; the mode, assembly and bundle-check code, action, templates and suites are gone; `make test` passes | completed |
| 02 | [phase-02-hub-branch.md](phase-02-hub-branch.md) | `compass hub branch` opens and reuses hub worktrees paired with the current code branch, and `--prune` removes finished ones; an end-to-end suite takes a code repo from one `repos.txt` line to a pushed, checked hub branch | completed |
| 03 | [phase-03-writers-on-hub.md](phase-03-writers-on-hub.md) | `compass sync` runs the hub state table with the local consistency check against a real local hub; `compass stamp` commits to the hub; neither writes to a code repo | completed |
| 04 | [phase-04-skill-docs-pilot.md](phase-04-skill-docs-pilot.md) | The skill, pointer line, hub template text, README, install guide and design doc describe the hub-only flow; the research report records the switch; the old plan's pilot phases are rewritten for it | completed |

Dependencies: 02 and 03 need 01, for the two-column parser, the config helpers and the `okf` wiring for tests. 02 and 03 own separate packages and can run in parallel. 04 needs 02 and 03, because it documents their commands and runs the skill against them.

Scope: about 60 files change, 17 of them deletions, plus re-recorded fixtures. The cutover has to remove every mode, assembly and bundle-check path in one plan, because Non-Goals rules out shims. Splitting the deletions from the replacements would leave a release where knowledge has nowhere to live.

## Non-Goals

- Migrating existing bundles: none are live. The Data note above records how to move one.
- Checking that a concept's `sources` paths exist in the code.
- Access control for each repo inside one hub.
- Triggering sync from a webhook, a schedule or CI.
- Any compass write to a code repo, including comments.
- Keeping the removed commands as aliases or deprecation stubs.
- Rewriting the research report beyond recording the switch. Its evidence files under `docs/research/okf-knowledge-system/evidence/` record past runs and stay unchanged.
- Cleaning up hub branches or `repos/<id>/` for a repo removed from `repos.txt`.
- Running the pilot. Phase 04 only re-plans it.

## Acceptance Criteria

1. **Adding a repo takes one `repos.txt` line.** In an end-to-end suite on local bare remotes, a code repo with no compass file, branch or hook goes from that one line to a pushed hub branch that passes `compass hub check`. On the first branch for a new id, the hub branch changes `repos/<id>/`, `repos/index.md` and the root `index.md`. Sync's Go tests then merge such a branch against a local hub clone and recorded GitHub responses.
2. **`compass hub branch`** behaves as Interfaces states. That includes:
   - a branch name with slashes;
   - each URL form that setup accepts, including through an `insteadOf` rewrite;
   - every refusal;
   - reuse;
   - `--prune` removing a clean, pushed worktree while keeping one with work in it.
3. **`compass sync`** carries out every row of the state table, and refuses to merge for each failed condition in Trust boundaries. That includes:
   - a rename out of another repo's folder;
   - a symlink;
   - a branch that is valid alone but breaks a link once merged.

   It still merges a clean branch while the default branch carries an older finding.
4. **Neither writer makes a write call** under a code repo's API path in any test.
5. **`compass stamp`** stamps `repos/<id>/<concept>.md` on the hub's default branch with one fast-forward per run, and makes no commit when stamps are current.
6. **The removed pieces are gone.** A fresh build's `compass --help` lists none of `bundle check`, `hub assemble` or `branch setup`. `actions/bundle-check/`, `internal/bundlecheck/`, `internal/branchsetup/` and the four removed templates no longer exist.
7. **The docs, the skill and the template** describe one flow and name no removed command or mode. The research report records the switch in section 7.
8. **The old plan's pilot is rewritten.** Phases 08–10 of `261006-1358-okf-knowledge-system` describe a hub-only pilot, and `af plans` reports zero findings for both plans.

## Verification

Run from the compass root:

- `make test` exits 0.
- `make build && bin/compass --help | grep -cE 'bundle check|hub assemble|branch setup'` prints `0`.
- `LC_ALL=C ls actions/bundle-check internal/bundlecheck internal/branchsetup templates/folder-workflow.yml templates/okf-main-workflow.yml templates/hooks templates/AGENTS-pointer.md 2>&1 | grep -c 'No such file'` prints `7`.
- `git grep -nIiE 'okf/main|(folder|branch)[ -]mode|assembl|bundle[ -]check|branch setup|folder\|branch' -- internal cmd skill templates test testdata README.md docs/install.md docs/design` exits 1.
- `go test ./internal/syncjob/ ./internal/stamp/ ./internal/hub/ ./internal/config/` exits 0.
- `./test/bench-scale.sh` exits 0, with the `hub check` median under 10 s and both `hub branch` timings within target.
- `af plans` prints `0 finding(s)`.

## Risks

- **Reviewers ignore hub pull requests** (decision 2). Signal: in the pilot, hub pull requests stay unapproved a day after their code merged. Response: name a daily reviewer in the pilot hub. If that fails, return to planning on decision 2.
- **Index conflicts force re-approval too often.** Signal: more than one in five pilot merges needs a second approval after a conflict. Response: return to planning. One candidate is accepting an approval on an earlier head when the newer commits only merge the default branch and regenerate `index.md` files.
- **The window between check and merge lets a broken link land** (Failure modes). Signal: `compass hub check` on the default branch fails after a sync merge. Response: fix with a follow-up hub pull request; decision 11 keeps other merges going meanwhile. If it happens more than once in the pilot, return to planning on decision 4's rejected hub-CI alternative.
- **Agents don't run sync.** Signal: pilot metric 5, hub pull requests still open a day after their code merged. Response: the skill makes sync the first command of every session in a listed repo. If that fails, name a daily owner, as the old plan did.
- **Pointer-only discovery fails.** Code repos carry nothing, so an agent that never reads the user-level pointer line finds no knowledge. Signal: the phase 04 skill-eval read scenario fails, or the pilot's first CI or cloud agent run cites no concept. Response: put the pointer in organisation-level instructions. If a platform has none, return to planning on an optional `AGENTS.md` line.
- **The pilot hub can't have a ruleset** (Trust boundaries, hub write access). Signal: the pilot team's hub is private on GitHub Free. Response: make the pilot hub public if its knowledge allows, or use a paid plan. Otherwise the pilot runs with approval by convention, and the read-out names the gap.
- **Hub size outgrows a single clone.** Signal: `compass hub branch` misses its 5 s target on a real hub. Response: return to planning on sparse worktrees of `repos/<id>/` plus hub-level paths, which needs the validator to accept a partial tree.

## Unresolved Questions

None.

## Validation Log

### Session 1 — 2026-10-21
**Trigger:** adversarial pass required by the `## Design` section; one `af-review` red-team agent, plus direct `okf` and `af plans` runs. **Claims checked:** 21
**Verified:** 8 | **Failed:** 13 | **Unverified:** 0
- Failed: the path rule refuses `repos/index.md`, which `okf index .` writes when a first repo folder appears (confirmed with the pinned `okf`); fixed in Trust boundaries and Acceptance 1
- Failed: renames, deletions, symlinks, submodule links, `.gitattributes` and `.gitmodules` pass the path rule; `internal/hub/check.go` has no symlink check (check.go:69-84), and the validator follows symlinks (validate.go:114-133); fixed in decision 9 and Trust boundaries
- Failed: no base-branch condition; today's code filters on it (sync.go:256-268); fixed in Trust boundaries
- Failed: sync and stamp read control files from the working tree (sync.go:677, stamp.go:389); fixed in decision 7
- Failed: hub write is needed by every developer, so approval depends on a ruleset (templates/hub/overview.md:85-87); fixed in Trust boundaries and Risks
- Failed: `X/feat` and `X/feat/x` can't coexist as git refs; ids allowed by `repoNameRe` (config.go:183) can be invalid ref components; fixed in the Components id rules and the Interfaces refusals
- Failed: a hub pull request closed or hand-merged with squash leaves its branch stuck (sync.go:375 `onTrunk`); fixed in the state table
- Failed: a broken default branch blocks every merge; fixed in decision 11
- Failed: phase 01's grep covers phase 04's files and `test/check-fixture.sh:87`; the pattern misses hyphenated forms (cmd/compass/main.go:41); fixed in phase 01, phase 04 and Verification
- Failed: `bin/compass --help` reads a stale binary, since `make test` builds `.cache/bin/compass` (run-all.sh:7-9); fixed in Verification and phase 01
- Failed: Go tests would need `okf`, which only the suites install (check.go:96-99, Makefile `test`); fixed in phase 01
- Failed: no phase owned Acceptance 1's end-to-end suite, and phase 10 depends on phase 09 (phase-10-pilot-readout.md:7); fixed in phases 02 and 04
- Failed: a second URL matcher beside setup's `githubRepo` and `withoutUserinfo` (setup.go:367-382); fixed in decision 12
- Decided: auto-pruning dropped for an explicit `--prune` (decision 8); `hub branch` timings moved to `bench-scale.sh`; the research report records the switch instead of being rewritten

### Session 2 — 2026-10-21
**Trigger:** `/af:plan validate`; direct source reads, the pinned `okf` on a scale hub and a template hub, and `af plans --json`. **Claims checked:** 48
**Verified:** 35 | **Failed:** 13 | **Unverified:** 0
- Failed: the raw-then-rewritten origin order lives in `existingClone` (setup.go:341-357), not in the two helpers; fixed in decision 12 and phase 01
- Failed: phase 01's grep covers `cmd`, while phase 03 owned the "branch-mode" sync summary (main.go:41); fixed in phases 01 and 03
- Failed: `test/scale-counts.sh` asserts the generator's `repos/rNN/okf/` layout (scale-counts.sh:10-12) and no phase owned it; fixed in phase 01
- Failed: `hub assembl` misses "assembly" and "assembled" (templates/hub/CODEOWNERS:1, test/gen-scale.sh:6, test/lib.sh:53); widened in Verification and phases 01 and 04
- Failed: bare `go test` lines would miss `okf`, which only `$OKF` or PATH supply (config.go:99-104); fixed in phase 01
- Failed: `hub check` only prints (check.go:31-91), but sync diffs findings; fixed in phase 01
- Failed: `ghfake.WriteHub` and `TemplateHub` write plain files (ghfake.go:211-225), not a git hub with an origin; fixed in phase 03
- Failed: `syncjob.Run`'s caller is `run` in cli.go:106; fixed in phase 03
- Failed: three-column sites in `internal/config` and `internal/hub` are 2 files each (config_test.go:23, assemble_test.go:41); fixed in phase 01
- Failed: old plan Acceptance 5, 7, 8, milestones M4–M5 and two Verification lines name modes or assembly (old plan.md:28-29, 276-290); fixed in phase 04
- Failed: `af plans` text shows no per-phase status; fixed in phase 04
- Failed: the stamper also reads the repo, its head ref and contents (stamp.go:143, 205), and fast-forwards per repo, not per row (stamp.go:335); fixed in decisions 6 and 10; scope is ~60 files, 17 deleted; fixed in Phases
- Re-measured: `hub check` 1.25 s on the scale hub; `okf index` writes `repos/index.md`, and a first repo changes exactly `repos/<id>/`, `repos/index.md` and the root `index.md`
