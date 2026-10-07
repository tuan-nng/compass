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
- With no credentials, the action's code downloads: `curl -sfL -o /dev/null https://codeload.github.com/tuan-nng/compass/tar.gz/$(git rev-parse origin/master)` exits 0. That is the fetch a runner in another account makes. On 2026-10-07, while compass was private, it exited 22 (HTTP 404), and the same command against the public `tuan-nng/okf` exited 0. The first workflow run on the action happens on a pilot repo (phase 08's first verification check), not on a scratch repo (plan decision 22).
