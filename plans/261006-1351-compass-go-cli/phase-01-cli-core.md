---
phase: 1
title: "compass CLI core"
status: pending
priority: P1
effort: 4d
dependencies: []
---

# Phase 1: compass CLI core

## Overview

Outcome: `compass` builds from a compass clone and replaces every `okf-tools` script except the stamper and the sync job. Each subcommand keeps the old script's flags, output and exit codes, so the moved suites pass unchanged except for the command they call (plan decisions 2, 6 and 7).

Owns:

- the module (`go.mod`, module `compass`), `Makefile` (`build`, `install`, `test`), `cmd/compass` and the internal packages for config, pins and the installer, the validator, the bundle check, hub assembly, branch setup and app creation;
- the shared config and GitHub client packages that phase 02 builds on. These land first, so phase 02 can start against them;
- `config.env`, `pins/okf.env`, `templates/`, `testdata/`, `test/` and the okf-skills reference copy with its license, all moved from `okf-tools` at `fa42d76`. The test data keeps its git history only in `okf-tools`; the move is a copy.

The validator port follows plan decision 8. The decision 8 answer the user gives at plan review comes first.

## Verification

- `go vet ./... && go test ./...` exits 0. The tests include the differential validator test over `testdata/fixture`, `testdata/proto` and the broken-bundle cases.
- `make install && compass version` prints the build's commit.
- `./test/run-all.sh` exits 0, with these suites driving `compass`: install, check-fixture, scale-counts, bundle-check, branch-setup (25 cases), assemble.
- `./test/bench-scale.sh` and `./test/bench-assemble.sh 50` meet the rollout plan's decision 9 targets.
- `grep -rn 'okf-tools' cmd internal test templates actions` exits 1.
