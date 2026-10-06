---
phase: 1
title: "compass CLI core"
status: completed
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

## Result (2026-10-06)

Done; nothing is committed yet. Every check above passed:

- `go vet ./... && go test ./...` exits 0. The differential validator test runs the Python reference through uv over the test data and 84 broken bundles. The only difference it allows is the YAML parser's wording inside "frontmatter is not valid YAML" messages.
- `compass version` prints `compass c57b3e4…-dirty`, the commit `make install` built from.
- `./test/run-all.sh` reports all 7 suites passed: the six above plus phase 03's `test/setup.sh`. check-fixture runs 46 checks and branch-setup 25 cases.
- `bench-scale.sh`: validate 0.330 s, search 0.307 s, backlinks 0.306 s. `bench-assemble.sh 50`: 3.7 s cold, 4.0 s warm, for 10,100 files.
- The `okf-tools` grep exits 1.

What changed on the way, all inside the non-goal (no decision changes):

- **Validator YAML:** yaml.v3 with an emulation of pyyaml's type rules, not a pyyaml port, to keep plan decision 2's dependency limit.
- **Message prefixes:** `okf install:`, `bundle check:`, `hub check:`, `hub assemble:` and `app create:` replace the script names.
- **`app create`:** a failed `gh` call in the browser callback now answers 500 and exits 1; the script hung. The manifest JSON is compact with sorted keys.
- **`hub assemble`** keeps its own `repos.txt` parser, so its messages match the script's.
- **Config:** `OKF_ORG` comes from the environment, then the user config `compass setup` writes (`$XDG_CONFIG_HOME/compass/config`), then the built-in `config.env`. The suites point `XDG_CONFIG_HOME` at an empty folder so a developer's own config can't leak in.
- **Review fixes:** an independent review compared the port with the scripts. Two config-parsing differences were fixed and pinned by `internal/config/config_test.go`:
  - an unreadable user config is now an error instead of a silent fall-back;
  - control files now split lines exactly as Python did (a lone CR, form feed and U+2028 end a line), so line numbers in messages match.
