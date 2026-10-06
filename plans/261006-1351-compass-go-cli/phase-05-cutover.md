---
phase: 5
title: "Delete okf-tools and knowledge-hub, repoint the rollout plan"
status: pending
priority: P2
effort: 0.5d
dependencies: [4]
---

# Phase 5: Delete okf-tools and knowledge-hub, repoint the rollout plan

## Overview

Outcome: compass is the only code home, no hub repo belongs to the project, and every plan and doc says so.

Owns:

- **Deleting `tuan-nng/okf-tools`** (plan decision 4), after phase 04 is green. The local clone at `/mnt/data/works/okf-tools` is removed too, after confirming its last commit is `9dcdd60` or later and nothing in it is unmoved.
- **Deleting `tuan-nng/knowledge-hub`** (plan decision 4), and its local clone at `/mnt/data/works/knowledge-hub`.
- **The rollout plan** `260929-1607-okf-knowledge-system-rollout`:
  - its decisions 1, 7, 11 and 17 point to this plan;
  - its `<org>/knowledge-hub` component becomes "the hub the pilot team provides from the compass hub template";
  - its phase files and `plan.md` name compass subcommands instead of `okf-tools` scripts. 9 files mention `okf-tools`, and 4 mention `knowledge-hub`;
  - its `blocked_by: [261006-1351-compass-go-cli]` frontmatter, added when this plan was written, is removed.
- **The design doc and research report** where they name `okf-tools`, a script, or `knowledge-hub` as a repo the project owns. The design doc has 4 `okf-tools` and 9 `knowledge-hub` mentions; the report has 2 and 1.

## Verification

- `gh repo view tuan-nng/okf-tools` and `gh repo view tuan-nng/knowledge-hub` both exit non-zero.
- `grep -rn 'okf-tools\|knowledge-hub' docs plans/260929-1607-okf-knowledge-system-rollout` prints only lines that record the history of the move, or that use `knowledge-hub` as an example name for the end user's hub.
- `af plans` reports zero findings.
