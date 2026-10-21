---
phase: 4
title: "Skill, docs and pilot re-plan"
status: completed
priority: P1
effort: 1d
dependencies: [2, 3]
---

# Phase 4: Skill, docs and pilot re-plan

## Overview

Outcome: everything a person or agent reads describes one flow:
1. list the repo;
2. run `compass hub branch`;
3. write in the hub worktree;
4. open a linked hub pull request;
5. get it approved;
6. run `compass sync`, then `compass hub branch --prune`.

The old plan's pilot phases match this flow.

Owns:

- **`skill/okf/SKILL.md`.** One flow (plan Design, Components), with the trust rules unchanged:
  - reading uses the printed worktree, or the hub clone when on a default branch;
  - writing covers `repos/<id>/` and hub-level concepts;
  - `okf index .` and `compass hub check` run before each commit;
  - the hub pull request names the code pull request as `owner/repo#N` or its URL;
  - sync runs at session start and after the code merges, and a per-repo read failure is reported, not fatal;
  - code pull requests from forks get no knowledge merge;
  - the reviewer runs `compass hub check` before approving a hub pull request that will be merged by hand.
- **`templates/user-pointer.md`.** The single trigger line for agents.
- **`test/skill-eval.sh` and `test/skill-eval/analyze.py`.** The scenarios are rebuilt on a hub-only setup. The removed mode, assembly and bundle-check assertions give way to the new flow's: the agent ran `compass hub branch`, wrote under the printed path, and `compass hub check` passed.
- **`templates/hub/overview.md`, `README.md` and `docs/install.md`.**
  - Adding a repo is one step.
  - They state the access boundary (plan decision 3) and the hub write access and ruleset requirement (plan Trust boundaries).
  - Updating compass no longer mentions `COMPASS_SHA`.
- **`docs/design/okf-knowledge-system-ux.md`.** The journeys, the mechanisms, the failure table and "what does not exist yet" describe the hub-only flow.
- **`docs/research/okf-knowledge-system.md`.**
  - A dated note at the top and in section 5 points to this plan's design.
  - In section 7, the central repo becomes the choice, and per-repo bundles in both modes become a rejected row that says when they would win.
  - The rest stays as the record of the earlier recommendation (plan Non-Goals).
- **`plans/261006-1358-okf-knowledge-system/`.**
  - A decision records that this plan supersedes the modes, assembly, the bundle-check action and decision 16, and its Design points here.
  - Phases 08 and 09 merge into one hub-only pilot phase 08, and phase 09 becomes `cancelled`.
  - Pilot metric 5 applies to every repo.
  - Phase 10 depends on phase 08, in its frontmatter and in the plan's Dependencies.
  - Phase 10's checks drop the per-mode repo table.
  - The milestones M4 and M5, the phase table rows for 08–10, Acceptance 5, 7 and 8, and the Verification lines that run `compass hub assemble` or ask for a repo "in each mode" describe the hub-only pilot. Acceptance met by completed phases stays, marked as superseded by the new decision where it names a removed piece.

Contract: the pointer line changes. Files that name it: 1 in `templates`, 2 in `docs`, 1 in `test`.

## Verification

- `git grep -nIiE 'okf/main|(folder|branch)[ -]mode|assembl|bundle[ -]check|branch setup|folder\|branch|COMPASS_SHA' -- skill templates test README.md docs/install.md docs/design` exits 1.
- `grep -c '^| Per-repo bundles' docs/research/okf-knowledge-system.md` prints `1`.
- `./test/skill-eval.sh --agent claude --scenario read --runs 3` and `--scenario write --runs 3` each pass 3 of 3. They run the claude CLI headless on the user's account, as the old plan's phase 03 did.
- `af plans` prints `0 finding(s)`, and `af plans --json | jq -r '.data.plans[] | select(.name=="261006-1358-okf-knowledge-system/") | .phases[] | select(.phase==9) | .status'` prints `cancelled`.
- `make test` exits 0.

## Result (2026-10-21)

Everything above is built, and every check passes. Not committed.

- `skill/okf/SKILL.md` and `templates/user-pointer.md` describe one flow: `compass hub branch`, write and check in the printed worktree, push and open a hub pull request naming `owner/repo#N`, then `compass sync` and `--prune`. At session start the pointer runs `compass hub branch` before `compass sync`; plan Flow step 5 lists only the sync after the code merge, so the two orders do not conflict.
- `test/skill-eval.sh` runs read, write, cross-repo, injected and autoload on a hub-only setup. Code repos carry nothing, so the user pointer is passed through `--append-system-prompt`.
- README, install guide, template overview, design doc, research report and the old plan are rewritten as listed above; the old plan's phase 08 is now `phase-08-hub-pilot.md`, and phase 09 is `cancelled`.
- **Checks on the final binary.** `--scenario read --runs 3`: 3/3 passed. `--scenario write --runs 3`: 3/3 passed (`.cache/skill-eval/final2-*`). The phase grep exits 1, the research report has 1 `| Per-repo bundles` row, phase 9 prints `cancelled`, `make test` exits 0.
- **Review notes accepted, not changed.** Decision 11 compares findings by path and message, so a second broken link in a file that already has one is not new, and a count message flags any new concept in a folder that already mismatched. This is the trade-off decision 11 chose.
