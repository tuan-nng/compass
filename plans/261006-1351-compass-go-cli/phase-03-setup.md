---
phase: 3
title: "compass setup"
status: pending
priority: P1
effort: 1.5d
dependencies: [1]
---

# Phase 3: compass setup

## Overview

Outcome: one command, `compass setup`, takes a machine from a fresh clone to ready, against a hub the end user provides (plan decision 3).

- It asks for the org, with the `config.env` value as the default, and for the hub repo. Flags `--org`, `--hub` and `--yes` skip the prompts, for CI and cloud agents.
- It checks the hub is readable with the caller's GitHub credentials. If it isn't, setup stops before writing anything.
- It installs the pinned `okf` and writes the user config.
- It installs the embedded skill into each agent's user-level folder:
  - `${CLAUDE_CONFIG_DIR:-~/.claude}/skills/okf`;
  - `~/.cursor/skills/okf`;
  - `${PI_CODING_AGENT_DIR:-~/.omp/agent}/skills/okf`.
- Running it again changes nothing.

The skill and pointer texts come from the uncommitted phase 03 work in `okf-tools`: `skill/okf/SKILL.md`, `templates/AGENTS-pointer.md` and `templates/user-pointer.md`. The skill-eval harness and scenarios, with `THIRD_PARTY.md`, come too. The skill's commands change from the scripts to `compass` subcommands.

The hub template is the end user's starting point for their own hub. It is built from the non-content files of today's `tuan-nng/knowledge-hub`:

- `repos.txt` and `checks.txt`, each holding only a header that explains its format;
- `CODEOWNERS` with a placeholder owner, and `.gitignore`;
- the hub and writer workflows, with the compass action reference as a placeholder;
- an overview concept that says how to fill the hub in, and a root index.

It holds no knowledge, and it passes `compass validate` as it ships.

Owns:

- the setup package and the `setup` subcommand;
- `skill/`, embedded with `go:embed`;
- the pointer templates and `THIRD_PARTY.md`;
- `test/setup.sh`, which replaces `install-tools.sh`;
- `test/skill-eval*`;
- `templates/hub/`.

The rollout plan's phase 03 still owns the skill's content and its scenario pass rates. This phase owns only installing it.

## Verification

- `./test/setup.sh` exits 0, with `HOME` pointed at a temp folder. It covers a first run, a second run with no changes, an unreadable hub that writes nothing, and `--yes` with flags and no terminal.
- `compass validate templates/hub` exits 0.
- `grep -rn 'okf-branch-setup.sh\|assemble-hub.sh\|okf-tools\|knowledge-hub' skill templates` exits 1.
