---
phase: 8
title: "Folder-mode pilot and metrics"
status: pending
priority: P1
effort: 2d + pilot window
dependencies: [6, 7]
---

# Phase 8: Folder-mode pilot and metrics

## Overview

Outcome: 1–2 real folder-mode repos follow the full loop in design sections 4.1–4.3. Their maintainers name them when this phase starts (plan decision 8), and the phase doesn't start without them. The pilot team provides the hub from the template (plan decision 3). The pilot window starts, and `compass pilot report` measures it.

Owns:

- **Onboarding each repo,** following design section 4.8's folder-mode steps:
  - an `okf/` bundle with `overview.md`;
  - the bundle-check workflow from the folder-mode template, calling the public compass action pinned by SHA, with its push trigger set to the repo's default branch (the template lists `main`);
  - the pointer line in `AGENTS.md`;
  - a `repos.txt` row;
  - a `checks.txt` row where a push-triggered check exists;
  - a ruleset bypass for the hub owner who runs `compass stamp`, if the maintainers accept it (plan decision 13).
- **The writer owner:** one named hub owner who runs `compass sync` and `compass stamp` each working day of the pilot window (plan decision 13).
- **The pilot guide,** the compass README. It covers getting `compass` onto a machine, `compass setup`, making a hub from the template, assembling and checking it locally, and running the writers.
- **The stats repo in setup (plan decision 20).** `compass setup` takes an optional stats repo and a clone folder for it, in the same way as the hub.
- **`compass pilot report`.** It reports the five metrics from design section 9, item 5, per repo, from GitHub pull requests and git history. It prints the report and writes it into the stats clone:
  - **Agents writing knowledge:** merged pull requests that change a concept with a `generated` stamp, out of all merged pull requests.
  - **Reviewers correcting knowledge:** pull requests where someone other than the author changed a concept, beyond adding `verified`.
  - **Stale concepts hit:** commits that change a concept whose previous `stale_after` had passed.
  - **Stamps lost to edits:** commits that remove a `verified` entry.
  - **Knowledge pull requests open a day after their code merged:** branch mode only. Folder-mode repos print `n/a`.
- **The pilot start date,** recorded in the stats repo and in this plan when the first repo goes live.

## Verification

- For each pilot repo, `gh run list -R <owner>/<repo> -w okf -L 1 --json conclusion -q '.[0].conclusion'` prints `success`.
- In a clone of the pilot hub, with the pilot repos in `repos.txt`, `compass hub assemble --ci . && compass hub check .` exits 0.
- The first real task with the skill gives a merged pull request that changes 1–3 concepts. After a reviewer's `verified` line merges, `okf show okf <id> | jq -r .concept.trust_tier` prints `human-reviewed`.
- Where a `checks.txt` row exists, the writer owner's next `compass stamp` run adds a `process:` entry to a covered concept. This is the stamper's first live run (plan decision 22), so on that repo:
  - `git log -1 --format=%ae` shows the writer owner's commit email;
  - `git show --stat HEAD` lists only covered concepts;
  - `git rev-parse HEAD~1` equals the commit the check job ran on;
  - a second `compass stamp` run makes no commit.

  If the push is refused, the log names the refusal, not "moved" (phase 06), and the maintainers decide on the bypass (plan Risks).
- `go test ./...` covers each metric against recorded GitHub responses with known counts.
- `compass pilot report --repos pilot.txt --since <start>` prints one line per metric per repo, and the same report appears as a new file in the stats clone.
