---
phase: 10
title: "Pilot read-out"
status: pending
priority: P2
effort: 1d after the pilot window
dependencies: [9]
---

# Phase 10: Pilot read-out

## Overview

Outcome: a read-out in `compass` gives the five pilot metrics for each pilot repo, lists pilot incidents, and makes one decision: roll out wider, change the design, or stop. Each repo's window must be at least four weeks. The branch-mode window starts at phase 09, so it ends later than the folder-mode one.

This phase owns:

- **The read-out,** `docs/research/okf-knowledge-system/pilot-readout.md`. It gives each repo's mode, window start and end, and the script's output.
- **Updates to what remains, in both documents:** the "What remains" column of design section 7, design section 9, and research report section 8. They now list only what is still missing.
- **A decision on the fork,** if upstream has released a fix for okfcli#34 by then: retire the fork and move the pin, or keep it.
- **A decision on seeding from docs** (plan decision 16): run a one-off trial that drafts concepts from one pilot repo's existing prose docs, or don't. It rests on the write-rate metric and on pilot reports of agents finding nothing to read. If the answer is yes, the read-out names the repo, the tool (OpenKB, or another), and the review owner, and the trial gets its own plan.

## Verification

- `python3 bin/okf-pilot-report --repos pilot.txt --since <start>` exits 0. Its output appears in the read-out.
- The read-out's repo table lists at least one `folder` repo and one `branch` repo, each with a window of at least 28 days.
- `grep -c '^## Decision' docs/research/okf-knowledge-system/pilot-readout.md` prints `1`.
- `grep -c '^### Seeding from docs' docs/research/okf-knowledge-system/pilot-readout.md` prints `1`.
- `grep -n 'sync job, which is not built' docs/design/okf-knowledge-system-ux.md` finds nothing (exit code 1).
