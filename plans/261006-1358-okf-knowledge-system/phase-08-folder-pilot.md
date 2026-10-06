---
phase: 8
title: "Folder-mode pilot and metrics"
status: pending
priority: P1
effort: 2d + pilot window
dependencies: [6]
---

# Phase 8: Folder-mode pilot and metrics

## Overview

Outcome: 1–2 real folder-mode repos follow the full loop in design sections 4.1–4.3. Their maintainers name them when this phase starts (plan decision 8), and the phase doesn't start without them. The pilot team provides the hub from the template (plan decision 3). The pilot window starts, and `compass pilot report` measures it.

Owns:

- **Onboarding each repo,** following design section 4.8's folder-mode steps:
  - an `okf/` bundle with `overview.md`;
  - the bundle-check action in CI, from the folder-mode workflow template, with its push trigger set to the repo's default branch (the template lists `main`);
  - the pointer line in `AGENTS.md`;
  - a `repos.txt` row;
  - a `checks.txt` row where a push-triggered check exists;
  - the writer app's ruleset bypass, if the maintainers accept it.
- **The pilot guide,** the compass README. It covers clone, `make install`, `compass setup`, and making a hub from the template.
- **`compass pilot report`.** It reports the five metrics from design section 9, item 6, per repo, from GitHub pull requests and git history:
  - **Agents writing knowledge:** merged pull requests that change a concept with a `generated` stamp, out of all merged pull requests.
  - **Reviewers correcting knowledge:** pull requests where someone other than the author changed a concept, beyond adding `verified`.
  - **Stale concepts hit:** commits that change a concept whose previous `stale_after` had passed.
  - **Stamps lost to edits:** commits that remove a `verified` entry.
  - **Knowledge pull requests open a day after their code merged:** branch mode only. Folder-mode repos print `n/a`.
- **The pilot start date,** recorded in this plan when the first repo goes live.

## Verification

- For each pilot repo, `gh run list -R <org>/<repo> -w okf -L 1 --json conclusion -q '.[0].conclusion'` prints `success`.
- The pilot hub's latest `hub` run prints `success`, with the pilot repos in `repos.txt`.
- The first real task with the skill gives a merged pull request that changes 1–3 concepts. After a reviewer's `verified` line merges, `okf show okf <id> | jq -r .concept.trust_tier` prints `human-reviewed`.
- Where a `checks.txt` row exists, the next writer run adds a `process:` entry to a covered concept.
- `go test ./...` covers each metric against recorded GitHub responses with known counts.
- `compass pilot report --repos pilot.txt --since <start>` prints one line per metric per repo.
