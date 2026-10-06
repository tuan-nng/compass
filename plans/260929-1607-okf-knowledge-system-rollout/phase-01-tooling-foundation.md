---
phase: 1
title: "Tooling foundation"
status: pending
priority: P1
effort: 2.5d
dependencies: []
---

# Phase 1: Tooling foundation

## Overview

Outcome: `<org>/okf-tools` exists and tags its first release. Every later phase builds on four parts of it, listed below. The design doc's printed command output is refreshed from the rebuilt test data.

- **Test data** (plan decision 8):
  - the small test set: three repo bundles plus the hub, rebuilt by hand from the planted-facts table in `docs/research/okf-knowledge-system/evidence/protocol.md`. Every row of that table must hold. The table shortens one value: `gotchas/idempotency-key` carries `stale_after: 2026-09-01T00:00:00Z`, the spec's datetime form the fixture used (okfcli evidence, test T2). The design doc's section 4.1 output and phase 02's tests depend on that form. Phases 03 and 04 rely on its named concepts, such as `gotchas/idempotency-key` and `cross-repo/invoice-dependency`;
  - the same set rewritten to the report's conventions (research report sections 4 and 5);
  - a generator for the scale set: 50 bundles × 200 concepts under `<dir>/repos/rNN/okf/`, 10,050 files including indexes, about 10% of concepts with a cross-repo link, and the unique search string `zebracorn-needle` in `r37/okf/d3/c123.md` only (protocol, "Shared inputs"). The hub built from these bundles is not part of those counts.

  The research data under `/tmp/okf-research` is gone (plan decision 8), so there is nothing to copy and no reference copy to compare against.
- **A pinned install.** One script installs the `okf` binary into a given folder and refuses a checksum mismatch. It starts on upstream okfcli v0.5.0; phase 02 moves it to the fork.
- **The bundle-check composite action** (plan decision 11). It:
  - runs the vendored okf-skills validator (commit `8e31878`, release v0.10.0) in strict mode, under a SHA-pinned `uv` with a locked `pyyaml`;
  - runs `okf index` on the bundle;
  - fails if `okf index` changed any file (research report section 5, "Hub assembly and CI").
- **One test entry point,** `test/run-all.sh`, which later phases extend.

This phase also owns **refreshing the design doc's command output** (plan decision 8). The doc says its output is real, but that output came from the lost data. Re-run, with the pinned `okf` on the rebuilt sets, the three blocks that came from it: `okf search` and `okf show` in section 4.1, and `okf backlinks` in section 4.4. Replace any line that differs, keeping the doc's trimming and notes. Then repoint two paragraphs from `/tmp/okf-research` to the test data in `okf-tools`: the one at the end of the doc's Gist that says where its output comes from, and the last paragraph of the research report's Evidence section. The branch-mode output in section 4.9 came from scratch repos, not from this data, so it stays as it is.

## Verification

- `./test/check-fixture.sh` exits 0. It checks each row of the protocol's planted-facts table against the small set with the pinned `okf` and the strict validator: the stale draft, the root-relative link that breaks in the hub, the machine-confirmed and human-reviewed concepts, the deprecated concept, the cross-repo links, and the three inbound links to `contracts/invoice-api` in the hub view. It judges staleness from `stale_after` and the run date, not from `okf show`'s `stale` field, so it still passes after phase 02 moves the pin.
- `./test/gen-scale.sh "$d"` builds the set, and:
  - `find "$d/repos" -name '*.md' | wc -l` prints `10050`;
  - `grep -rl zebracorn-needle "$d/repos"` prints only the path ending in `r37/okf/d3/c123.md`;
  - the share of concepts with a cross-repo link is between 8% and 12%.
- `./bin/okf-install --dest "$b" && "$b/okf" --version` prints the pinned version. The same run with the pinned checksum edited exits non-zero and installs nothing.
- A scratch private repo calls the bundle-check action from a private `okf-tools`, referenced by SHA. It must be private, because public repos would hide an access problem until the pilot.
  - On a clean bundle, `gh run view <id> --json conclusion -q .conclusion` prints `success`.
  - On a pull request that adds a concept without running `okf index`, it prints `failure`.
- The `compass` commit for this phase records the full output of each of the three re-run commands. Every id, title, type and field value in the matching design doc block appears in that output.
- `grep -n '/tmp/okf-research' docs/design/okf-knowledge-system-ux.md docs/research/okf-knowledge-system.md` in `compass` finds nothing (exit code 1).
- `./test/run-all.sh` exits 0.
