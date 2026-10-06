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

- **Deleting `tuan-nng/okf-tools` and its local clone** at `/mnt/data/works/okf-tools`. First confirm that the local clone's last commit is `9dcdd60` or later (GitHub stops at `fa42d76`), and that nothing in it, committed or not, was left unmoved.
- **Deleting `tuan-nng/knowledge-hub` and its local clone** at `/mnt/data/works/knowledge-hub`.
- **The design doc and research report.** Where they name `okf-tools`, a script, `okf_validate.py`, or `knowledge-hub` as a repo the project owns, they now name compass subcommands and "your hub". `knowledge-hub` may stay as an example name. Links into `evidence/` stay, since those files are dated records. The design doc's setup-script references point to `compass branch setup` and its git minimums: 2.28, or 2.42 for `--init`. Also:
  - research report section 8, item 2 drops the `okf/main` workflow check, which is done;
  - the research report's OpenKB row cites plan decision 11 instead of the replaced rollout plan's decision 16.

## Verification

- `gh repo view tuan-nng/okf-tools` and `gh repo view tuan-nng/knowledge-hub` both exit non-zero.
- `test -e /mnt/data/works/okf-tools || test -e /mnt/data/works/knowledge-hub` exits 1.
- `grep -rnE --exclude-dir=evidence '(^|[^/])(okf-tools|assemble-hub\.sh|okf-branch-setup\.sh|okf_validate\.py)' docs` exits 1. On 2026-10-06 it matched 16 lines. The `[^/]` skips link targets into `evidence/`.
- `grep -n 'rollout plan decision' docs/research/okf-knowledge-system.md` exits 1.
- `gh search code 'okf-tools' --owner tuan-nng` finds no workflow file.
