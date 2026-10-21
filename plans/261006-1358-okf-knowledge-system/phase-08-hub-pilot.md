---
phase: 8
title: "Hub-only pilot and metrics"
status: pending
priority: P1
effort: 2.5d + pilot window
dependencies: [6, 7]
---

# Phase 8: Hub-only pilot and metrics

## Overview

Outcome: 2–3 real repos, listed in one pilot hub, follow the full loop in design sections 4.1–4.3 and 4.9: an agent writes on hub branch `<id>/<b>`, a person approves the hub pull request, and `compass sync` merges it after the code pull request merges. Their maintainers name the repos when this phase starts (plan decision 8), and the phase doesn't start without them. The pilot team provides the hub from the template (plan decision 3). The agents that CI and cloud platforms run on these repos also find the hub. The first approved hub pull request after its code merged is sync's first live merge, and the first stamp is the stamper's first live run. The pilot window starts, and `compass pilot report` measures it.

This phase replaces the former folder-mode pilot (phase 08) and branch-mode pilot (phase 09), which plan decision 23 merged. It starts only after `261021-1007-hub-branch-knowledge` is completed (plan frontmatter `blocked_by`).

Owns:

- **The pilot hub,** made from `templates/hub/`, following `docs/install.md` sections 3–5:
  - `CODEOWNERS` naming the pilot's hub owners;
  - push access for every pilot developer whose agent writes knowledge;
  - a ruleset on the default branch that blocks direct pushes and requires pull requests with code-owner review, with a bypass only for the hub owner who runs `compass stamp`. If the hub is private on GitHub Free, the pilot follows the hub-only plan's Risks entry for a hub that can't have a ruleset.
- **Onboarding each repo:** one `repos.txt` row, and a `checks.txt` row where a push-triggered check exists. Nothing changes in the repo itself.
- **Developers and agents:** each pilot developer runs `compass setup` and adds the line in `templates/user-pointer.md` to their user-level agent instructions. Each CI and cloud agent environment the pilot repos use runs `compass setup --yes`, and the line goes into each platform's organisation-level instructions (plan decision 23).
- **The pilot's owners:**
  - agents run `compass sync` at the start of each session, as the skill says;
  - one named hub owner runs `compass stamp` each working day of the pilot window (plan decision 13);
  - one named daily reviewer approves hub pull requests (hub-only plan Risks).
- **The pilot guide,** the compass README and `docs/install.md`. Both were rewritten for the hub-only flow by phase 04 of `261021-1007-hub-branch-knowledge`, and the first version's steps were run end to end against local bare remotes on 2026-10-07. This phase adds the stats repo and `compass pilot report` to both, and fixes whatever the first pilot repos show is wrong.
- **The stats repo in setup (plan decision 20).** `compass setup` takes an optional stats repo and a clone folder for it, in the same way as the hub.
- **`compass pilot report`.** It reports the five metrics from design section 9, item 5, for every pilot repo, from GitHub pull requests in the code repos and the hub and from the hub's git history. It prints the report and writes it into the stats clone:
  - **Agents writing knowledge:** merged code pull requests whose linked hub pull request changes a concept under `repos/<id>/` with a `generated` stamp, out of all merged code pull requests.
  - **Reviewers correcting knowledge:** hub pull requests where someone other than the author changed a concept, beyond adding `verified`.
  - **Stale concepts hit:** hub commits that change a concept whose previous `stale_after` had passed.
  - **Stamps lost to edits:** hub commits that remove a `verified` entry.
  - **Hub pull requests open a day after their code merged:** for every repo, hub pull requests that were still open a day after their linked code pull request merged.
- **The pilot start date,** recorded in the stats repo and in this plan when the first repo goes live.

## Verification

- In a clone of the pilot hub, with the pilot repos in `repos.txt`, `compass hub check` exits 0.
- `gh api repos/<owner>/<hub>/rules/branches/<default>` lists a `pull_request` rule that requires code-owner review. Without one, the read-out names the gap (hub-only plan Risks).
- The first real task with the skill in each pilot repo gives a hub pull request that changes 1–3 concepts under `repos/<id>/` and names the code pull request. After a reviewer's `verified` line merges, `okf show . repos/<id>/<concept> | jq -r .concept.trust_tier` in an updated hub clone prints `human-reviewed`.
- On the first approved hub pull request after its code merged, which is sync's first live merge (plan decision 22):
  - the first `compass sync` run after the code pull request's `mergedAt` merges it (plan decision 12);
  - `gh pr view <n> -R <owner>/<hub> --json mergedBy -q .mergedBy.login` prints the login of the developer whose agent ran sync;
  - its merge commit has 2 parents;
  - the next `compass sync` run deletes the hub branch once the code branch is gone.
- Where a `checks.txt` row exists, the hub owner's next `compass stamp` run adds a `process:` entry to a covered concept. This is the stamper's first live run (plan decision 22), so on the hub's default branch:
  - `git log -1 --format=%ae` shows the hub owner's commit email;
  - `git show --stat HEAD` lists only covered concepts under `repos/<id>/`;
  - a second `compass stamp` run makes no commit.

  If the push is refused, the log names the refusal, not "moved" (phase 06), and the hub owners decide on the bypass (plan Risks).
- Three CI or cloud agent runs, on three different tasks, each show `compass hub branch` printing a worktree path in their log, and each cite a concept from `repos/<id>/`.
- `go test ./...` covers each metric against recorded GitHub responses with known counts.
- `compass pilot report --repos pilot.txt --since <start>` prints one line per metric per repo, with a number, not `n/a`, for the fifth metric in every repo, and the same report appears as a new file in the stats clone.
