---
phase: 5
title: "Delete okf-tools and knowledge-hub"
status: pending
priority: P2
effort: 0.5d
dependencies: [4]
---

# Phase 5: Delete okf-tools and knowledge-hub

## Overview

Outcome: compass is the only code home, the project owns no hub repo, and the docs say so (plan decisions 1, 3 and 4).

Owns:

- **Deleting `tuan-nng/okf-tools` and its local clone** at `/mnt/data/works/okf-tools`. First confirm that its last commit is `9dcdd60` or later, and that nothing in it was left unmoved.
- **Deleting `tuan-nng/knowledge-hub` and its local clone** at `/mnt/data/works/knowledge-hub`.
- **The design doc and research report.** Where they name `okf-tools`, a script, or `knowledge-hub` as a repo the project owns, they now name compass subcommands and "your hub". `knowledge-hub` may stay as an example name. The design doc's setup-script references point to `compass branch setup` and its git minimums: 2.28, or 2.42 for `--init`.

## Verification

- `gh repo view tuan-nng/okf-tools` and `gh repo view tuan-nng/knowledge-hub` both exit non-zero.
- `test -e /mnt/data/works/okf-tools || test -e /mnt/data/works/knowledge-hub` exits 1.
- `grep -rn 'okf-tools\|assemble-hub.sh\|okf-branch-setup.sh' docs` prints only lines that record the history of the move.
- `gh search code 'okf-tools' --owner tuan-nng` finds no workflow file.
