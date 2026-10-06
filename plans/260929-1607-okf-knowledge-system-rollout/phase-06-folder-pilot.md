---
phase: 6
title: "Folder-mode pilot and metrics"
status: pending
priority: P1
effort: 1.5d + pilot window
dependencies: [3, 4, 5]
---

# Phase 6: Folder-mode pilot and metrics

## Overview

Outcome: 1–2 real folder-mode repos follow the full loop in design sections 4.1–4.3. Their maintainers name them when this phase starts (plan decision 13), and the phase does not start without them. The pilot window starts, and a script measures it.

This phase owns:

- **Onboarding each repo,** following the folder-mode steps in design section 4.8:
  - its `okf/` bundle with `overview.md`;
  - the bundle-check action in its CI;
  - the pointer line in `AGENTS.md`;
  - its `repos.txt` row;
  - a `checks.txt` row, where the repo has a check that runs on pushes to the default branch;
  - a ruleset bypass for the writer app, if the maintainers accept one (see the plan's Risks).
- **The pilot guide,** a README section in `okf-tools`. It says how a developer runs the installer (plan decision 17) and the assembly script.
- **The pilot metrics script in `okf-tools`.** It reports the five metrics from design section 9, item 6, per repo, from GitHub pull requests and git history:
  - **Agents writing knowledge:** merged pull requests that change a concept with a `generated` stamp, out of all merged pull requests.
  - **Reviewers correcting knowledge:** pull requests where someone other than the author changed a concept, apart from adding a `verified` line.
  - **Stale concepts hit:** commits that change a concept whose previous `stale_after` had already passed.
  - **Stamps lost to edits:** commits that remove a `verified` entry.
  - **Knowledge pull requests still open a day after their code merged:** branch mode only; the script prints `n/a` for folder-mode repos.
- **The pilot start date,** recorded in this plan's `## Phases` notes when the first repo goes live.

## Verification

- For each pilot repo, `gh run list -R <org>/<repo> -w okf -L 1 --json conclusion -q '.[0].conclusion'` prints `success`.
- `gh run list -R <org>/knowledge-hub -w hub -L 1 --json conclusion -q '.[0].conclusion'` prints `success`, with the pilot repos in `repos.txt`.
- The first real task done with the skill in a pilot repo gives a merged pull request that changes 1–3 concepts. After a reviewer's `verified` line merges, `okf show okf <id> | jq -r .trust_tier` prints `human-reviewed`.
- Where the repo has a `checks.txt` row, the next writer run adds a `process:` entry to a covered concept, and `okf show` reports its trust tier as `machine-confirmed` or `human-reviewed`.
- `python3 -m unittest discover test/report` in `okf-tools` exits 0. It checks each metric against recorded GitHub responses whose expected counts are known.
- `python3 bin/okf-pilot-report --repos pilot.txt --since <start>` prints one line per metric per repo, and at least one merged pull request that changes knowledge.
