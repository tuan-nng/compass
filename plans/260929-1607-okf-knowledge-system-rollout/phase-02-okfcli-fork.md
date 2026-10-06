---
phase: 2
title: "okfcli fork for datetime stale_after"
status: pending
priority: P1
effort: 1d
dependencies: [1]
---

# Phase 2: okfcli fork for datetime stale_after

## Overview

Outcome: `<org>/okf`, a fork of okfcli v0.5.0, accepts `stale_after` as an RFC 3339 datetime as well as a date, in both validation and the stale check. A datetime makes a concept stale from that instant on. A date keeps today's behavior: stale from that day on. Plan decision 2 covers the fork; the upstream fix is okfcli#34.

In okfcli v0.5.0, two functions parse `stale_after` as a date only:

- `concept.Frontmatter.IsStale`, which gives `okf show` its `stale` field;
- `validate.validateV02`, used by `okf validate`.

The patch changes both and nothing else.

Owns:

- **The fork repo.** It publishes release `v0.5.0-<org>.1`, with binaries for Linux and macOS on amd64 and arm64, and a checksum file.
- **The upstream pull request.** It carries the same patch and is linked from okfcli#34.
- **The pin.** The version and checksum pinned in `okf-tools` move to this release.
- **The benchmark,** `test/bench-scale.sh` in `okf-tools`. It assembles a hub from the phase 01 scale set and reports medians over 5 runs, after 1 warm-up, for `okf validate`, a search for `zebracorn-needle`, and backlinks for `repos/r37/d3/c123` (protocol, test T10).

## Verification

- `go test ./...` in the fork exits 0.
- On a copy of the `okf-tools` test data, where `billing-api/okf/gotchas/idempotency-key.md` has `stale_after: 2026-09-01T00:00:00Z`:
  - `okf validate <copy>/billing-api/okf` reports no error for `stale_after`, and reports the concept as stale;
  - `okf show <copy>/billing-api/okf gotchas/idempotency-key | jq .stale` prints `true`;
  - after changing it to a datetime one year ahead, the same command prints `false`.
- `./test/bench-scale.sh` in `okf-tools`, with the pinned fork, prints medians under the plan's decision 9 targets: validate under 10 s, search under 2 s, backlinks under 2 s. Search returns exactly one hit.
- `gh pr list -R okfcli/okf --author @me --state open` lists the upstream pull request.
