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

Outcome: `compass` builds from a compass clone and replaces every `okf-tools` script except the writer jobs. Each subcommand keeps the old script's flags, output and exit codes. So the moved suites pass unchanged except for the command they call (plan decisions 2, 17 and 18).

Owns:

- **The Go module and its build:** `go.mod` (module `compass`), the `Makefile` targets `build`, `install` and `test`, and `cmd/compass`.
- **Internal packages for:**
  - config and the org name (plan decision 7);
  - pins and the `okf` installer;
  - the validator (plan decision 19);
  - the bundle check;
  - hub assembly and the hub check;
  - branch setup, with the same flags; the hook snippets in `templates/hooks/` call `compass branch setup --print-hook` instead of the script;
  - app creation.
- **The shared config and GitHub client packages** that phase 02 builds on. These land first.
- **Moved from the `okf-tools` working tree at `9dcdd60`:** `config.env`, `pins/okf.env`, `templates/` (without the two uncommitted pointer templates), `testdata/`, the committed bash `test/` suites and benches, and the okf-skills reference copy with its license and uv lock. The Python stamp and sync tests stay in `okf-tools` as phase 02's reference. Comments that cite the old plans' decision numbers are updated to this plan's numbers.
- **`test/run-all.sh`,** reduced to the six suites below. The three other suites it runs today change owner: `stamp-unit.sh` and `sync-unit.sh` become Go tests in phase 02, and `install-tools.sh` becomes `test/setup.sh` in phase 03.

The move is a copy. The old git history stays in `okf-tools` until phase 05 deletes it. The uncommitted files (`skill/`, `test/skill-eval*`, `test/install-tools.sh`, `bin/okf-tools-install`, `templates/AGENTS-pointer.md`, `templates/user-pointer.md`, `THIRD_PARTY.md`) exist only in the local `okf-tools` working tree; phase 03 moves or replaces them.

## Verification

- `go vet ./... && go test ./...` exits 0. This includes the differential validator test over `testdata/fixture`, `testdata/proto` and the broken-bundle cases.
- `make install && compass version` prints the build's commit.
- `./test/run-all.sh` exits 0, with each suite driving `compass`:
  - install;
  - check-fixture (46 checks);
  - scale-counts;
  - bundle-check;
  - branch-setup (25 cases);
  - assemble.
- `./test/bench-scale.sh` and `./test/bench-assemble.sh 50` meet plan decision 12's targets.
- `grep -rn 'okf-tools' cmd internal test templates` exits 1.
