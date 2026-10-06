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

Outcome: a read-out in compass gives the five pilot metrics for each pilot repo, lists pilot incidents, and makes one decision: roll out wider, change the design, or stop. Each repo's window is at least four weeks. The branch-mode window starts at phase 09, so it ends later.

Owns:

- **The read-out,** `docs/research/okf-knowledge-system/pilot-readout.md`. It gives each repo's mode, its window, and the `compass pilot report` output.
- **What remains, in both documents:** design section 7's "What remains" column, design section 9, and research report section 8 list only what is still missing.
- **A decision on the fork,** if upstream has released a fix for okfcli#34: retire it and move the pin, or keep it.
- **A decision on seeding from docs** (plan decision 11): run a one-off drafting trial on one pilot repo's prose docs, or don't. If yes, the read-out names the repo, the tool and the review owner, and the trial gets its own plan.

## Verification

- `compass pilot report --repos pilot.txt --since <start>` exits 0, and its output appears in the read-out.
- The read-out's repo table lists at least one `folder` and one `branch` repo, each with a window of at least 28 days.
- `grep -c '^## Decision' docs/research/okf-knowledge-system/pilot-readout.md` prints `1`.
- `grep -c '^### Seeding from docs' docs/research/okf-knowledge-system/pilot-readout.md` prints `1`.
- `grep -n 'sync job, which is not built' docs/design/okf-knowledge-system-ux.md` exits 1.
