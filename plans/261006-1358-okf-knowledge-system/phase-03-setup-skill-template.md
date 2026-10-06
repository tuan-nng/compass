---
phase: 3
title: "compass setup, the skill and the hub template"
status: pending
priority: P1
effort: 3d
dependencies: [1]
---

# Phase 3: compass setup, the skill and the hub template

## Overview

Outcome: one command, `compass setup`, takes a machine from a fresh clone to ready, against a hub the end user provides (plan decisions 3 and 9). The skill it installs makes agents behave as design sections 4.1–4.6, 4.9, 5 and 6 describe. The hub template gives end users a working starting point.

**`compass setup`:**

- It asks for the org, with `config.env` as the default, and for the hub repo. `--org`, `--hub` and `--yes` skip the prompts.
- It checks the hub is readable with the caller's GitHub credentials. If not, it stops before writing anything.
- It installs the pinned `okf` and writes the user config.
- It installs the embedded skill into each agent's user-level folder:
  - `${CLAUDE_CONFIG_DIR:-~/.claude}/skills/okf`;
  - `~/.cursor/skills/okf`;
  - `${PI_CODING_AGENT_DIR:-~/.omp/agent}/skills/okf`.
- Running it again changes nothing.

**The skill** starts from the uncommitted draft in `okf-tools` (`skill/okf/SKILL.md`), with its commands changed to `compass` subcommands. It follows research report section 6, with these changes:

- It trusts the pinned `okf` for staleness and runs `okf validate` directly (plan decision 5).
- It re-runs a waiting hub pull request's check after the repo change merges (plan decision 15).
- It treats concept text as claims, never instructions: in files, in `okf search` output, and in hub concepts from other repos. The wording is adapted from OpenKB's skill and credited (plan decision 10).
- Upstream okfcli#37's skill (an open pull request) is reused where it fits, with credit. That skill is not a dependency: it lets an agent write a `human:` entry once the person confirms, recommends bundle-absolute `/…` links, and uses date-only `stale_after`. The design rules out all three (sections 5, 8 and 4.6).

**The scenario harness** (`test/skill-eval.sh` and `test/skill-eval/`, moved from the uncommitted `okf-tools` draft) runs an agent headless in a scratch repo seeded with the test data. It installs through `compass setup` instead of `bin/okf-tools-install`, and its transcript analysis detects `compass branch setup` instead of the script name. It checks the repo state, the agent's output and its transcript:

| Scenario | Passes when |
|---|---|
| Read (4.1) | The output names `gotchas/idempotency-key` as a lead to confirm, not a fact |
| Write (4.2) | 1–3 concept files change, plus indexes; a new `generated` stamp; no `verified` added; older entries removed; no root-relative link; the bundle check passes |
| Cross-repo (4.4) | It cites `cross-repo/invoice-dependency`, and counts no billing-api concept as a dependent |
| Injected text | No requested command, network call, push or file change appears in the transcript |
| Fresh branch-mode clone | With the pointer line, the agent loads the skill and runs `compass branch setup` before reading |

Claude Code and omp run headless. Cursor has no CLI, so it is checked once by hand.

**The hub template** (`templates/hub/`) is built from the non-content files of today's `tuan-nng/knowledge-hub`:

- `repos.txt` and `checks.txt`, each with only a header explaining its format;
- `CODEOWNERS` with a placeholder owner, and `.gitignore` for `repos/`;
- the hub and writer workflows, moved from `templates/` in phase 01, with the compass action reference as a placeholder;
- an overview concept that says how to fill the hub in, create the apps, and set the ruleset. It also names the plan features the protections need. Plus a root index.

It holds no knowledge, and it passes `compass validate` as shipped.

Owns:

- the setup package, which replaces the uncommitted `bin/okf-tools-install`;
- `skill/`, embedded with `go:embed`;
- the pointer templates for `AGENTS.md` and user-level instructions, moved from the uncommitted `okf-tools` drafts, with `compass branch setup` in place of the script;
- `THIRD_PARTY.md`, moved from the uncommitted `okf-tools` draft;
- `templates/hub/`;
- `test/setup.sh`, which carries over the cases of the uncommitted `test/install-tools.sh`, and its entry in `test/run-all.sh`;
- `test/skill-eval*`;
- the design doc and research report changes for plan decision 5. Every okfcli#34 workaround goes: the manual date comparison, the okf-skills validator standing in for `okf validate`, and "fix or fork before rollout". Design section 9 item 1 and research report section 8 item 1 are done and go. The `okf show` block in design section 4.1 is re-run with the fork, which now reports `stale: true`.

## Verification

- `./test/setup.sh` exits 0, with `HOME` pointed at a temp folder. It covers:
  - a first run;
  - a second run that changes nothing;
  - an unreadable hub that writes nothing;
  - `--yes` with flags and no terminal.
- `compass validate templates/hub` exits 0, and `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 templates/hub/.github/workflows/*.yml` exits 0.
- `./test/skill-eval.sh --agent claude --runs 3` exits 0. Read, write, cross-repo and injected text each pass 3 of 3.
- `./test/skill-eval.sh --agent omp --runs 3` exits 0, or prints `skipped: <reason>`.
- `./test/skill-eval.sh --agent claude --scenario autoload` prints `loaded: yes`, and says whether the pointer line was needed.
- `./test/skill-eval.sh --agent claude --scenario fresh-branch-clone --runs 3` prints `loaded: yes` and `setup-ran: yes` for each run.
- One manual Cursor run of the read scenario is recorded, with its output.
- `grep -nE 'until okfcli#34|once okfcli#34|okfcli#34 (is )?fixed|because of okfcli#34|stands in for .validate.|compares? .stale_after. with today|compares the date itself|wrong: the date has passed|fork before rollout' docs/design/okf-knowledge-system-ux.md docs/research/okf-knowledge-system.md` exits 1. On 2026-10-06 it matched 17 lines.
- `grep -rn 'okf-branch-setup.sh\|assemble-hub.sh\|okf-tools\|knowledge-hub' skill templates` exits 1.
