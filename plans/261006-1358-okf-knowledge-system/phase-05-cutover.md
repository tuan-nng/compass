---
phase: 5
title: "Delete okf-tools, knowledge-hub and the scratch repos"
status: pending
priority: P2
effort: 0.5d
dependencies: [4]
---

# Phase 5: Delete okf-tools, knowledge-hub and the scratch repos

## Overview

Outcome: compass is the only code home, the project owns no hub repo and no test repos on GitHub, and the docs say so and describe local-first operation (plan decisions 1, 3, 4, 13–16 and 22).

Owns:

- **Deleting `tuan-nng/okf-tools` and its local clone** at `/mnt/data/works/okf-tools`. First confirm that the local clone's last commit is `9dcdd60` or later (GitHub stops at `fa42d76`), and that nothing in it, committed or not, was left unmoved.
- **Deleting `tuan-nng/knowledge-hub` and its local clone** at `/mnt/data/works/knowledge-hub`.
- **Deleting the six scratch repos** (plan decision 22): `tuan-nng/okf-scratch-hub` (if not already deleted), `okf-scratch-billing-api`, `okf-scratch-shared-auth`, `okf-scratch-web-app`, `okf-scratch-bundle-check` and `okf-scratch-branch-mode`, plus the local clones under `/mnt/data/works/scratch`. Phase 04's Result keeps their run ids as the record. No later phase uses them.
- **The design doc and research report.** Where they name `okf-tools`, a script, `okf_validate.py`, or `knowledge-hub` as a repo the project owns, they now name compass subcommands and "your hub". `knowledge-hub` may stay as an example name. Links into `evidence/` stay, since those files are dated records. The design doc's setup-script references point to `compass branch setup` and its git minimums: 2.28, or 2.42 for `--init`. Also:
  - research report section 8, item 2 drops the `okf/main` workflow check, which is done;
  - the research report's OpenKB row cites plan decision 11 instead of the replaced rollout plan's decision 16.

- **The design doc and research report, for decisions 13–16.** They stop describing:
  - hub CI;
  - a scheduled check job and sync job in the hub;
  - the GitHub Apps and the writer key.

  Instead they describe local assembly and `compass hub check`, hub owners running `compass stamp` and `compass sync`, and the bundle check as the only CI job. The docs' "check job" becomes "the stamper" (`compass stamp`), the plan's term, because the check is the CI job that supplies evidence and the stamper is what commits. This covers:
  - the decision-15 places: the design doc's section 2 "Hub CI" row, section 4.5 step 3, the section 8 row on a hub pull request waiting for an unmerged concept, and research report section 5, "Hub assembly and CI";
  - the section 8 row on symlinked hubs, because the count check is now built;
  - the places that say the 50-repo fetch time was not measured (design section 4.4 and section 7, N4). They now cite the assembly bench.

## Verification

- `gh repo view tuan-nng/okf-tools` and `gh repo view tuan-nng/knowledge-hub` both exit non-zero.
- `test -e /mnt/data/works/okf-tools || test -e /mnt/data/works/knowledge-hub || test -e /mnt/data/works/scratch` exits 1.
- `gh repo list tuan-nng --limit 200 --json name -q '.[].name' | grep -c '^okf-scratch-'` prints `0`.
- `grep -rnE --exclude-dir=evidence '(^|[^/])(okf-tools|assemble-hub\.sh|okf-branch-setup\.sh|okf_validate\.py)' docs` exits 1. On 2026-10-06, after the okfcli#34 doc commit `9e74c4f`, it matched 14 lines. The `[^/]` skips link targets into `evidence/`.
- `grep -n 'rollout plan decision' docs/research/okf-knowledge-system.md` exits 1.
- `gh search code 'okf-tools' --owner tuan-nng` finds no workflow file.
- `grep -niE "hub CI|writer app|reader app|okf-write|GitHub App|nightly|on a schedule|sync job in the hub|hub's sync job|check job" docs/design/okf-knowledge-system-ux.md docs/research/okf-knowledge-system.md` exits 1. On 2026-10-06 it matched 32 lines (24 and 8). The schedule and check-job terms catch lines the narrower pattern missed, such as design section 2's "Runs in the hub on a schedule" and research report section 5's "The job runs on a schedule".
