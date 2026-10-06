---
phase: 7
title: "Publish compass"
status: pending
priority: P1
effort: 0.5d
dependencies: [5]
---

# Phase 7: Publish compass

## Overview

Outcome: `tuan-nng/compass` is public (plan decision 21), so repos in any account can run `actions/bundle-check` pinned by SHA, and no org name or move is needed.

Publishing exposes the whole git history, so it goes ahead only after two checks pass:
- a secret scan of every commit. No scanner is installed here, so this phase picks one and records it;
- a read of the docs and plans for private repos or people.

Any finding stops the phase (plan Risks: publishing compass).

Owns:

- **The pre-publish checks and their recorded results** in this phase file.
- **The visibility change.**

## Verification

- The chosen secret scanner, run over every commit, reports no findings, and the command and tool version are recorded here.
- `gh repo view tuan-nng/compass --json visibility -q .visibility` prints `PUBLIC`.
- On `okf-scratch-bundle-check`, a new push run of `okf` concludes `success` with the action still pinned to the same SHA.
