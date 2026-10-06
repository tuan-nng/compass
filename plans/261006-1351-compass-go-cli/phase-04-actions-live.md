---
phase: 4
title: "Compass actions live on GitHub"
status: pending
priority: P1
effort: 1d
dependencies: [1, 2, 3]
---

# Phase 4: Compass actions live on GitHub

## Overview

Outcome: compass is pushed to the private `tuan-nng/compass` (plan decision 1). The hub and all five scratch repos run their checks from compass actions pinned by commit SHA, and each action builds `compass` from its own checkout (decision 5).

Owns:

- `actions/bundle-check`, `actions/hub` and `actions/writer`, moved from `okf-tools` and changed to build `compass` with `setup-go` pinned by SHA and its build cache on;
- compass's Actions access setting;
- the workflow files in `tuan-nng/knowledge-hub`;
- the workflow files in the scratch repos `okf-scratch-bundle-check`, `okf-scratch-branch-mode`, `okf-scratch-billing-api`, `okf-scratch-shared-auth` and `okf-scratch-web-app`.

Before the first push, `git ls-files` must list only what compass means to publish (see Risks in `plan.md`).

This phase also finishes the live part of the rollout plan's phase 04 that never ran: the reader GitHub App, the hub run, and the draft-PR check. The user clicks the app creation and installation pages.

## Verification

- `gh api repos/tuan-nng/compass/actions/permissions/access -q .access_level` prints `user`.
- On `okf-scratch-bundle-check`, a new push run concludes `success`. A PR that adds a concept without `okf index` concludes `failure` with "index files are out of date".
- On `okf-scratch-branch-mode`, the `okf` workflow on `okf/main` concludes `success`.
- `gh workflow run hub -R tuan-nng/knowledge-hub`, then `gh run list -R tuan-nng/knowledge-hub -w hub -L 1 --json conclusion -q '.[0].conclusion'` prints `success`.
- The "build compass" step in that run takes at most 60 s.
- `gh search code 'okf-tools' --owner tuan-nng` returns no workflow file.
