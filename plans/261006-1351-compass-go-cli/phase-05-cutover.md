---
phase: 5
title: "Delete okf-tools and repoint the rollout plan"
status: pending
priority: P2
effort: 0.5d
dependencies: [4]
---

# Phase 5: Delete okf-tools and repoint the rollout plan

## Overview

Outcome: compass is the only code home, and every plan and doc says so.

Owns:

- **Deleting `tuan-nng/okf-tools`** (plan decision 4), after phase 04 is green. The local clone at `/mnt/data/works/okf-tools` is removed too, after confirming its last commit is `9dcdd60` or later and nothing in it is unmoved.
- **The rollout plan** `260929-1607-okf-knowledge-system-rollout`:
  - its decisions 1, 7, 11 and 17 point to this plan;
  - its phase files and `plan.md` name compass subcommands instead of `okf-tools` scripts, in the 9 files that mention `okf-tools`;
  - its `blocked_by: [261006-1351-compass-go-cli]` frontmatter, added when this plan was written, is removed.
- **The design doc and research report** where they name `okf-tools` or a script: 4 mentions in the design doc, 2 in the report.

## Verification

- `gh repo view tuan-nng/okf-tools` exits non-zero.
- `grep -rn 'okf-tools' docs plans/260929-1607-okf-knowledge-system-rollout` prints only lines that record the history of the move.
- `af plans` reports zero findings.
