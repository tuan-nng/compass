---
phase: 2
title: "Stamper and sync job in Go"
status: pending
priority: P1
effort: 2d
dependencies: [1]
---

# Phase 2: Stamper and sync job in Go

## Overview

Outcome: `compass stamp` and `compass sync` make the same decisions as the Python jobs in `okf-tools` commits `783c201` and `b10aaf1`. They are tested against the same recorded-API JSON fixtures, and the fake transport fails any test that makes a request it has no recording for.

The behavior to keep comes from the rollout plan's phases 05 and 08:

- The stamper stamps a concept only from a successful push run of the named workflow and job on the head commit.
- The stamper edits only the `verified` field, keeps the rest of the file byte for byte, and refuses forms it doesn't support.
- The stamper pushes nothing when the branch moved after it read it.
- The sync job trusts `bundle-check` only from the `github-actions` app.
- Dry-run refuses every write call. The token reaches the jobs only through the environment variable named by `--token-env`.

Owns:

- the stamper, sync and front-matter packages;
- `actions/writer` and `templates/hub-writer-workflow.yml`, moved and changed to build `compass`;
- the recorded fixtures, moved from `okf-tools/test/stamp` and `test/sync`.

It can start once phase 01 has landed the module, config and GitHub client packages.

## Verification

- `go test ./...` exits 0. It covers every case named in the Python suites: 27 stamper tests and 25 sync tests at `9dcdd60`.
- `compass stamp --help` and `compass sync --help` list `--hub`, `--dry-run`, `--token-env` and `--api-url`; `sync` also lists `--ignore-login` and `--now`.
- `actionlint templates/hub-writer-workflow.yml` exits 0.
