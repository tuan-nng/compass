---
phase: 5
title: "Delete okf-tools, knowledge-hub and the scratch repos"
status: completed
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

## Result (2026-10-07)

Everything is done, and every check above passes. The user deleted the eight repos on GitHub, and the local clones were removed with the user's approval after the checks below.

- **Nothing left unmoved.** The local `okf-tools` clone is at `9dcdd60`, with 9 uncommitted entries. Its 138 tracked and untracked files fall into three groups:
  - 56 are identical in compass.
  - 21 are in compass with edits made during phases 01–04 (the skill, the pointers, the eval harness, the workflows, the suites).
  - 61 have no copy at the same path. The stamp and sync fixtures and the vendored validator are in compass with identical content. The rest were ported to Go (`bin/*`, `lib/okftools/*`, the Python tests), replaced (`bin/okf-tools-install` by `compass setup`, `test/install-tools.sh` by `test/setup.sh`), or dropped by decisions 7 and 14–16 (`config.env`, the hub and writer actions and workflows, `bin/okf-github-app`).
  - The `knowledge-hub` clone and the five clones under `/mnt/data/works/scratch` have no uncommitted or unpushed work.
- **Deleted on GitHub by the user:** `okf-tools`, `knowledge-hub` and all six `okf-scratch-*` repos, including `okf-scratch-hub`. The local clones `/mnt/data/works/okf-tools`, `/mnt/data/works/knowledge-hub` and `/mnt/data/works/scratch` are removed too. The `okf-tools` clone held the only copy of `783c201`..`9dcdd60`, all of which is in compass (above).
- **Docs.** Both docs now name compass subcommands and "your hub". They describe local assembly with `compass hub check`, hub owners running `compass stamp` and `compass sync`, and the bundle check as the only CI job. They also make these changes:
  - the hub CI row becomes a hub owner row;
  - research report section 8, item 2 drops the done `okf/main` check;
  - the OpenKB row cites plan decision 11;
  - the symlink row describes the built count check;
  - design section 4.4 and N4 cite the assembly bench, noting that its warm run re-cloned because of the `fetchBundle` cache bug;
  - the CI and cloud agent gap now describes decision 9's pointer line, tested only headless so far.
  - Design section 9 keeps five items, so item 5 is still the pilot metrics.
- **Review fixes.** An independent review found two major doc errors, both now fixed:
  - The bench numbers first cited were the `okf-tools` script's 5.0 s / 5.0 s. Compass's own bench is 3.7 s cold and 4.0 s warm (phase 01).
  - Research section 5 said a failed fetch fails local assembly. Locally, an unfetchable repo keeps its previous copy with a warning, and only `--ci` fails the run.

  Minor fixes:
  - The writers are described as "run live only as `--dry-run`", matching phase 04.
  - Research section 8, item 1 now lists a live stamp.
  - The Recommendation calls okf-skills the validator's source, not a CI action.
  - "Scratch repos" becomes "local test repos" for `compass branch setup`.
- Phase 10's check `grep -n 'sync job, which is not built' docs/design/okf-knowledge-system-ux.md` already exits 1, because the rewrite dropped that line.
- **Checks.**
  - The `okf-tools`/script grep, the `rollout plan decision` grep, and the hub CI/app/schedule/check-job grep all exit 1.
  - `gh search code 'okf-tools' --owner tuan-nng` finds nothing.
  - `gh repo view` exits 1 for `tuan-nng/okf-tools` and `tuan-nng/knowledge-hub`. `gh repo list` finds 0 `okf-scratch-*` repos, and the local clone `test -e` check exits 1.
  - Outside `plans/` and `evidence/`, `knowledge-hub` remains only as an example name: in the docs, and as the local bare remote that `test/skill-eval.sh` builds. Phase 03 left no compass user config or `~/src/knowledge-hub` clone on this machine.
