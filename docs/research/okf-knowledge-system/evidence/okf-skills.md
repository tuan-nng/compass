# okf-skills (scaccogatto/okf-skills): evaluation log

- Repo: https://github.com/scaccogatto/okf-skills, `git clone --depth 1` on 2026-09-29
- Commit: `8e3187875e66051bb52f91a5ed27342e2c3208da` (2026-09-28, "chore(release): 0.10.0 …")
- Version: CHANGELOG `[0.10.0] - 2026-09-28`; `.claude-plugin/plugin.json` `"version": "0.10.0"`. MIT (the vendored SPEC is Apache-2.0).
- Work dir: `/tmp/okf-research/okf-skills/` (`repo/` is the clone, `work/` holds mutated copies, `env.sh` sets HOME, UV_CACHE_DIR, UV_TOOL_DIR, UV_PYTHON_INSTALL_DIR and XDG dirs inside the work dir)
- Runtime: uv 0.11.8, CPython 3.13.0. uv picked that interpreter. The symlink behaviour was rechecked on 3.12.13 and 3.14.4.

## What ships (scripts and CLI usage)

| Path | Kind | CLI |
|---|---|---|
| `skills/validate/scripts/okf_validate.py` | Conformance checker (PEP 723, needs `pyyaml`) | `uv run okf_validate.py <bundle> [--strict \| --max-warnings N] [--migrate] [--json]`. Exit 0 = ok, 1 = errors or too many warnings, 2 = not a directory |
| `skills/okf/scripts/okf_init.py` | Scaffolder | `uv run okf_init.py <target> [--title T] [--force]`. Refuses a directory that already has `.md` files |
| `skills/visualize/scripts/okf_visualize.py` | Writes one self-contained `viz.html` graph | `uv run okf_visualize.py <bundle> [-o out] [-t title] [-l link] [--layout …] [--max-nodes N]` |
| `skills/backfill/scripts/okf_backfill_events.py` | Deterministic git and Claude-session event extractor for LLM backfill | `uv run okf_backfill_events.py <repo> [--out events.jsonl] [--no-sessions] …`, `--show <sha>`, `--check-coverage events.jsonl .okf` |
| `servers/okf_mcp.py` | Read-only stdio MCP server (needs `mcp>=2`, `pyyaml`) with `search_concepts(query, limit)`, `read_concept(id)`, `get_neighbors(id)` | `uv run okf_mcp.py [<bundle>]`. Bundle falls back to `$OKF_BUNDLE`, then `./.okf` |
| `hooks/okf-stop-check.sh` + `hooks/hooks.json` | Claude Code `Stop` hook, dormant by default | Receives stdin JSON from Claude Code |
| `action.yml` | Composite GitHub Action wrapping the validator | `uses: scaccogatto/okf-skills@v1` with `bundle`, `strict`, `max-warnings`; the JSON report is the `report` output |
| `agents/event-analyzer.md`, `agents/bundle-weaver.md` | Claude Code subagents used by backfill (haiku / sonnet) | Plugin install only |

Neither the tool nor the MCP server has a CLI for search, backlinks or index generation. Search and backlinks exist only as MCP tools. index.md is written by the agent, by hand.

Default bundle location is **`.okf/`**, not `okf/`. The skill says "unless the project already uses another location". Three places hardcode `.okf`: the Stop hook, the plugin `.mcp.json` (no argument, so it falls back to `./.okf`) and `templates/CLAUDE-okf.md`.

---

## T1 Install: PARTIAL (no single binary; uv script)

Commands:
```
$ source env.sh && time uv run $V --help        # first run, cold cache
Installed 1 package in 2ms
usage: okf_validate.py [-h] [--strict] [--max-warnings N] [--migrate] [--json] bundle
real 0m0.536s
$ time uv run $V --help                          # warm
real 0m0.157s
$ time uv run $R/servers/okf_mcp.py --help </dev/null   # MCP deps, cold
 Downloaded pydantic-core
 Downloaded cryptography
Installed 29 packages in 4ms
real 0m2.498s
```
- Size: clone 9.8 MB (mostly `benchmark/` and `docs/`). Runtime surface: `skills/` 204 KB, `servers/` 8 KB, `hooks/` 8 KB. uv cache is 3.5 MB after the validator alone (pyyaml) and 58 MB once the MCP stack is added (29 packages: mcp, pydantic, cryptography, …).
- Runtime deps: `uv` (or `python3` + `pip install pyyaml`, the fallback the SKILL.md files document) and Python ≥3.11. The hook needs `bash`, `awk`, `git`.
- Network and telemetry: the first `uv run` of each script pulls wheels from PyPI. After that, `UV_OFFLINE=1 uv run $V …` gives exit 0 and the MCP search still works offline (checked). A grep over `skills/ servers/ hooks/` for urllib, requests, socket, telemetry or analytics found nothing, so the tool itself has no telemetry. `viz.html` loads cytoscape, marked and dompurify from `cdn.jsdelivr.net` when opened in a browser. The skills.sh installer (`npx skills`) **does** send anonymous telemetry by default: its README "Telemetry" section says so, and `DISABLE_TELEMETRY` / `DO_NOT_TRACK` turn it off.
- License: MIT.
- Install channels, from the README (none run against real config):
  - Claude Code plugin: `/plugin marketplace add scaccogatto/okf-skills` then `/plugin install okf@scaccogatto`. Gives skills, both subagents, the Stop hook and the MCP server (`mcp__plugin_okf_bundle__*`, rooted at `./.okf`).
  - `npx skills add scaccogatto/okf-skills` (vercel-labs `skills` CLI): installs the four skill directories `okf`, `validate`, `visualize`, `backfill`, each with its `scripts/`, `templates/` and `reference/SPEC.md`. Per the CLI README, project scope goes to `.claude/skills/` for Claude Code, `.agents/skills/` for Cursor, Codex and Pi, and `-g` goes to `~/.claude/skills/` or `~/.cursor/skills/`. The default is a symlink to a canonical copy; `--copy` copies. It installs **no** hook, no MCP config and no subagents, so backfill does not work from this channel (README says so).
  - GitHub Action, or `uv run servers/okf_mcp.py <bundle>` for any MCP host.
- Verdict: one command per channel with no daemon (the MCP server is stdio and per-session). Offline after the first dependency fetch. It is "one command" only if uv is already present; the scripts need Python plus PyPI, not a static binary.

### Stop hook (what it does)
`hooks/hooks.json` registers `bash ${CLAUDE_PLUGIN_ROOT}/hooks/okf-stop-check.sh` on `Stop` with a 15 s timeout. It is dormant unless **all** of these hold: no `stop_hook_active` loop, `OKF_HOOK != off`, `./.okf/index.md` exists with `upkeep: enforced` in its frontmatter, the working tree is a git repo with modified *tracked* files, and nothing under `.okf/` changed (untracked files count). Then it prints `{"decision":"block","reason":"…update the matching concept (body + generated); add a dated log.md entry only for a lifecycle event…"}`. Exercised in `work/hook`:
```
[code changed, .okf untouched]          -> {"decision":"block",...}
[{"stop_hook_active": true}]            -> no output (allowed)
[OKF_HOOK=off]                          -> no output
[untracked .okf/new.md]                 -> no output (counts as upkeep)
[bundle moved to okf/ instead of .okf/] -> no output (hook never sees it)
```
It is a nudge, not a correctness check. Touching any file under `.okf/` satisfies it. It is Claude Code-only, and it never fires for our `<repo>/okf/` layout without patching the path.

---

## T2 Validate: PARTIAL

```
$ for b in billing-api/okf web-app/okf shared-auth/okf hub hub-copy; do uv run $V $b; done
billing-api/okf   concepts 3  ✓ conformant — no issues                       default=0 strict=0
web-app/okf       concepts 2  2 warn: cross-link target not found `../../billing-api/...`, `../../shared-auth/...`   default=0 strict=1
shared-auth/okf   concepts 2  ✓ no issues                                     default=0 strict=0
hub (symlinks)    concepts 2  3 warn: `/repos/web-app/services/web-app.md`, `/repos/billing-api/contracts/invoice-api.md`, `/repos/web-app/flows/checkout.md` not found   default=0 strict=1
hub-copy          concepts 9  4 warn: repos/{billing-api,shared-auth,web-app}/index.md "§8 index.md should contain no frontmatter";
                              repos/billing-api/gotchas/idempotency-key.md: cross-link target not found `/contracts/invoice-api.md`   default=0 strict=1
$ uv run $V /nonexistent   -> "error: /nonexistent is not a directory"  exit=2
```
JSON (`--json`): `{bundle, conformant, passed, counts{concepts,indexes,logs}, errors[], warnings[] (strings "path: message"), migrated[]}`. Warnings are unstructured strings with no code or severity field.

Negative cases on a copy (`work/t2`), all caught:
```
✗ ERROR  README.md: §11.1 no parseable YAML frontmatter block (...)
✗ ERROR  gotchas/bad.md: §11.2 missing or empty required `type` field
! warn   gotchas/bad.md: §7 `verified[0].by` `Human:dana` is a near-miss of `human:`/`process:` ...
! warn   gotchas/bad.md: §5.5 `stale_after` `next week` is not an ISO 8601 datetime (a bare date is tolerated)
! warn   services/billing-api.md: §5.1 footnote `[^main-gox]` matches no `sources[].id`
✗ non-conformant (2 error(s))   exit=1
```
Checklist:
- **Stale concept**: NOT flagged. The validator only checks that `stale_after` is well-formed. `idempotency-key.md` (`stale_after: 2026-09-01T00:00:00Z`, today 2026-09-29) passes clean. Spec-form RFC 3339 datetimes **are accepted**, unlike several other tools. Staleness is only computed in the visualizer (browser JS) and left to the agent in consume mode.
- **Broken links**: yes, as warnings. Both root-relative and relative links are resolved; URLs and non-`.md` targets are skipped.
- **Nested index frontmatter**: yes, the §8 warning appears on the 3 `hub-copy/repos/*/index.md`. Root index extra keys other than `okf_version`/`upkeep` get a §12 warning.
- **Footnote ↔ sources id join**: yes. The fixture's `[^main-go]` joins silently, and a mismatch warns.
- Also caught: actor near-misses, `usage_count` without `usage_window`, malformed `generated`/`verified`, legacy v0.1 constructs, Attested Computation `runtime`/path checks.
- Not caught: index.md completeness. A concept missing from its directory's index.md passes `--strict` (seen in T7: `gotchas/legacy.md` was not in `gotchas/index.md` and got 0 warnings).
- Exit codes: warnings never fail without `--strict`/`--max-warnings`. Errors give 1, bad path gives 2.

## T3 Symlinks: FAIL

`workspace/hub` (symlinked repos) reports `concepts: 2` versus `concepts: 9` for `hub-copy`. Same result on Python 3.12.13 and 3.14.4 (`uv run --python 3.12 $V $F/hub` gives `concepts: 2`), because `Path.rglob` does not descend into symlinked directories. On the scale hub it is worse: `scale/hub` gives `{'concepts': 0, 'indexes': 1}` with **0 warnings, exit 0**. That is a silent false pass.

MCP server on `workspace/hub`: `get_neighbors repos/billing-api/contracts/invoice-api` returns `ERROR no such concept`. The traversal guard `concept_path()` resolves the path, and a symlink target outside the bundle root is refused. `search_concepts idempotency` returns `[]`. Visualizer: `rendered 2 concepts, 1 links` (hub) versus `9 concepts, 12 links` (hub-copy).

Workaround: assemble the hub with copies (`cp -a`, `cp -rL`, git submodule/subtree). The symlink form cannot be used.

## T4 Link resolution: PASS (per view, copy hub)

Validator warnings and MCP `get_neighbors` agree:

| Link | Standalone | hub (symlinks) | hub-copy |
|---|---|---|---|
| idempotency-key `/contracts/invoice-api.md` (root-relative) | resolves (billing-api: no warning; MCP incoming of `contracts/invoice-api` lists `gotchas/idempotency-key`) | file not seen | **broken**: warned; MCP outgoing of idempotency-key is `[]` |
| web-app `../../billing-api/contracts/invoice-api.md`, `../../shared-auth/concepts/session-token.md` | **broken**: 2 warnings; MCP outgoing only `flows/checkout` | file not seen | **valid**: no warning; MCP outgoing includes both |
| web-app URL link `https://github.com/acme/billing-api/blob/main/okf/...` | ignored as external (not a neighbour) | – | ignored |
| web-app `../flows/checkout.md` | valid | – | valid |
| hub `/repos/billing-api/...`, `/repos/web-app/...` | n/a | **broken** (3 warnings, targets not traversed) | **valid**: no warning; MCP outgoing = invoice-api, checkout, web-app |
| billing-api service `../gotchas/idempotency-key.md` | valid | – | valid |

All planted expectations match in the copy view. Root-relative links resolve against whatever directory is passed as the bundle, which is spec-correct.

## T5 Backlinks across repos: PASS (hub-copy only, MCP)

```
$ uv run mcpq.py fixture/hub-copy get_neighbors '{"concept_id":"repos/billing-api/contracts/invoice-api"}'
incoming: cross-repo/invoice-dependency, repos/billing-api/services/billing-api, repos/web-app/services/web-app
outgoing: repos/billing-api/services/billing-api
```
This matches the expected set exactly: idempotency-key is correctly absent because its link is broken in the hub. `sources[].resource` values pointing at bundle paths also count as edges. The visualizer's "Cited by" panel showed the same three (screenshot check). On the symlinked hub the result is `ERROR no such concept`. The only backlinks surfaces are MCP and viz.html; there is no CLI.

## T6 Search / retrieval for agents: PARTIAL

MCP `search_concepts` (case-insensitive substring over id, title, description and tags, then body; metadata hits rank first):
```
hub-copy, query "idempotency"  (10 ms in-server)
 repos/billing-api/gotchas/idempotency-key  Gotcha  status "draft"  stale_after "2026-09-01 00:00:00+00:00"
 repos/billing-api/contracts/invoice-api    API Contract  status ""  stale_after "2026-12-31 00:00:00+00:00"
 repos/billing-api/services/billing-api     Service  status "stable"  stale_after ""
hub (symlinks): result []
```
- Machine-readable: yes, MCP structured JSON cards `{id,type,title,description,status,stale_after}`. There is no CLI search. The skills tell the agent to use Read/Grep plus `index.md`.
- Trust tier: **not** in the MCP cards (no `generated`/`verified`). Staleness is a raw `stale_after` string (PyYAML's `str(datetime)` form, e.g. `2026-09-01 00:00:00+00:00`), not a computed `stale: true`. The agent must compare dates itself, and `read_concept` returns the raw frontmatter. The visualizer computes both badges. Checked in a headless browser on `hubcopy-viz.html`: idempotency-key shows `["unverified","stale since 2026-09-01 00:00:00+00:00"]`, billing-api service `["human-reviewed"]`, invoice-api `["machine-confirmed"]`. That view is for humans, not agents.
- Progressive disclosure: by convention only. The SKILL says to read root `index.md` first and follow links. There is **no index generator**: `okf_init` writes the first index, backfill says "write index.md per directory by hand", and the validator does not check index completeness.

## T7 Agent write path: PARTIAL (manual edits + validator; no write tooling)

No create or stamp command exists. The agent edits files directly (`allowed-tools: Read Write Edit Grep Glob Bash`). Simulated on a git copy (`work/t7/billing-api`) following the SKILL "maintain" steps:
1. Wrote `okf/gotchas/retry-budget.md` with `generated: { by: claude-code/opus-5, at: 2026-09-29T12:00:00Z }`, `status: draft`, `stale_after`, an unknown key `x-owner: team-billing   # comment` and `sources` plus footnote. Hand-wrote `gotchas/index.md`. `uv run $V okf --strict`: ✓ no issues, exit 0.
2. Human review: added `verified: { by: human:alice, at: 2026-09-29T15:00:00Z }`, set `status: stable`, and appended a `log.md` "Verification" entry (per the SKILL a verification pass is a lifecycle event; routine edits get no log entry). `--strict`: ✓, exit 0.
3. `git diff --stat`: `gotchas/index.md +4, retry-budget.md +19, log.md +4`: 3 files, 27 insertions. Small and reviewable.
- Unknown keys are preserved, because nothing rewrites frontmatter except `--migrate`, which is textual. Tested on a v0.1 concept: `x-owner: team-billing   # keep me` and its comment survived; `timestamp` became `generated: {by: process:okf-migrate, at: …}` and `# Citations` became `sources` (diff: 7 lines).
- `okf_init.py work/init/.okf --title Demo` creates `index.md` (`okf_version: '0.2'`), `log.md` (`## 2026-09-29 * **Creation**…`) and `getting-started.md` (`generated.by: process:okf_init`), which passes `--strict`. On an existing bundle it prints `refusing: … already contains .md files`, exit 1.
- The `verified: human:<id>` stamp is plain text. Nothing stops an agent from writing `human:alice` itself. The only guards are git diff review and the Stop hook's "did .okf change" nudge.

## T8 Move/rename: FAIL (detection only)

`mv repos/billing-api/contracts/invoice-api.md …/invoice-api-v2.md` in a hub-copy copy. No move tool exists, and no link is rewritten. The validator reports the 4 broken inbound links:
```
! warn cross-repo/invoice-dependency.md: cross-link target not found: `/repos/billing-api/contracts/invoice-api.md`
! warn repos/billing-api/gotchas/idempotency-key.md: … `/contracts/invoice-api.md`
! warn repos/billing-api/services/billing-api.md: … `../contracts/invoice-api.md`
! warn repos/web-app/services/web-app.md: … `../../billing-api/contracts/invoice-api.md`
✓ conformant (7 warning(s))   exit 0 (1 under --strict)
```
The external URL link in web-app goes stale undetected. Cross-repo breakage is only visible when validating the assembled hub copy.

## T9 Multi-bundle without hub: FAIL (no federation)

Every script and the MCP server take exactly one bundle root. There is no workspace, registry or project config. The only multi-bundle idea in the repo is "render a subdirectory". The stop-hook concept lists as a known limit that "a bundle not at `./.okf/` (monorepo subdirectory…) is never seen". Pointing the tools at the workspace root, as a fake super-bundle:
```
$ uv run $V fixture/workspace  -> concepts 9; 11 warnings: 4 "§8 index.md should contain no frontmatter",
    billing-api `/contracts/invoice-api.md` broken, hub `/repos/...` broken x3, web-app `../../billing-api/...` broken x2
$ mcp search "invoice" -> hits across billing-api/okf/*, web-app/okf/*, hub/* (search spans bundles)
$ mcp get_neighbors billing-api/okf/contracts/invoice-api -> incoming: only billing-api/okf/services/billing-api
```
Search spans the bundles this way, but root-relative links break and the hub-relative `../../<repo>/…` form does not resolve (the extra `okf/` path segment), so cross-bundle backlinks are lost. Cross-bundle links can only be written as URLs (ignored) or as relative paths valid in one assembled tree. Multiple MCP servers (one per bundle) would need manual host config; the plugin ships one server rooted at `./.okf`.

## T10 Scale: PARTIAL (correct on hub-copy; MCP backlinks > 2 s; symlink hub false-passes)

Method: `bench.sh` = 1 warmup + 5 runs of `/usr/bin/time -f %e`. All MCP numbers include spawning `uv run okf_mcp.py`, because each client invocation starts a fresh server.

| Operation | min | median | Target | Correct? |
|---|---|---|---|---|
| validate `scale/hub-copy` (10,000 concepts, 51 index) | 8.30 s | 8.42 s | <10 s ✓ | 0 errors, 50 warnings (all nested-index §8); no broken-link warnings, so cross-repo `../../rNN/…` resolve |
| validate `scale/hub` (symlinks) | 0.06 s | 0.06 s | – | **counts 0 concepts, exit 0: false pass** |
| validate loop over 50 standalone `repos/r*/okf` | 11.57 s | 11.76 s | – | per-bundle (~0.23 s each incl. uv start) |
| MCP spawn + `search_concepts zebracorn-needle` hub-copy | 7.35 s | 7.42 s | <2 s ✗ | single hit `repos/r37/d3/c123` ✓ (in-server call 5.7 s) |
| MCP spawn + search, `scale/hub` (symlinks) | 1.56 s | 1.59 s | – | 0 hits ✗ (no concepts visible) |
| MCP spawn + `get_neighbors repos/r37/d3/c123` hub-copy | 11.42 s | 11.45 s | <2 s ✗ | ✓ incoming = {r37/d1/c091, r37/d4/c094}, equal to an independent ground-truth scan; outgoing 3 (in-server call 9.6–9.8 s) |
| MCP spawn + get_neighbors `d3/c123` standalone r37 (200 concepts) | 1.82 s | 1.85 s | – | ✓ |
| MCP spawn baseline (search in r00, 200 concepts) | 1.79 s | 1.85 s | – | spawn ≈ 1.6–1.8 s is uv + mcp import |
| `grep -rl zebracorn-needle scale/hub-copy` (what the SKILL tells Claude to use: Grep) | 0.06 s | 0.06 s | – | ✓ |
| `grep -rlR zebracorn-needle scale/hub` (following symlinks) | 0.06 s | 0.07 s | – | ✓ |
| Index generation | – | – | – | [not run]: the tool has no index generator |
| visualize hub-copy (10k nodes, 30,693 links, 23 MB html) | 8.77 s | 8.85 s | – | auto-switches to concentric layout and warns "hairball" |

The MCP server re-reads and re-parses the whole bundle on every call, with no cache (the source comment says as much: "fine up to a few thousand concepts"). At 10k concepts that gives 5.7 s per search and 9.7 s per backlinks call even when warm.

---

## Agent workflow in the SKILL.md files

- **`okf`** (auto-triggers on "document this in OKF", "update the knowledge bundle", "capture this as a concept", or "any work in a repo that has an OKF bundle"; `user-invocable`, args `[produce|maintain|consume] [path]`). "Always read the canonical spec before non-trivial work" (the vendored `reference/SPEC.md`, 300+ lines).
  - **Read (consume)**: read the bundle-root `index.md` first, then follow links only into relevant concepts. Treat `status: draft|deprecated`, a past `stale_after`, or no `verified` entry as "check before relying on this". Broken links mean not-yet-written knowledge. For numbers covered by an Attested Computation, run its computation. "If you learn something durable while working, switch to maintain and write it back."
  - **Write (produce)**: new bundle via `uv run ${CLAUDE_SKILL_DIR}/scripts/okf_init.py`. One concept per file. Set `type` plus recommended fields, record `generated` and the `sources` actually read, cross-link (prefer root-relative `/x/y.md`), refresh `index.md` per directory, log a Creation entry, then validate.
  - **Write (maintain)**: find affected concepts by `resource`, path or topic. Update the body and `generated.at` with your own actor in `generated.by`. Removed assets get `status: deprecated` plus a log entry instead of deletion. Update index.md files. Log only lifecycle events (created, deprecated, superseded, regenerated, verified). Then validate.
  - **Stamps**: `generated: {by: <producer>/<version>|human:<id>|process:<id>, at: RFC3339}`. `verified: [{by, at}]`: "use `human:` whenever a person authored or signed off". The template says to omit `verified` "until someone confirms it". Nothing enforces who writes `verified`.
  - **Stale**: `stale_after` is an absolute datetime, not a TTL. On read it is only a caution flag. The skill has no refresh workflow for stale concepts, and no tool lists stale concepts for agents (only viz.html shows them).
  - **Validation**: "Never eyeball conformance": run `/okf:validate <dir> --strict` or `uv run ${CLAUDE_SKILL_DIR}/../validate/scripts/okf_validate.py <dir> --strict` before declaring done.
- **`validate`**: runs the checker with `$ARGUMENTS`, defaulting to `.okf`; explains ERROR versus warn. The "fix on sight" warning is the `Human:` near-miss. `--migrate` rewrites in place ("say what it will touch before running it").
- **`visualize`**: runs the renderer. The derived trust tier and staleness badges are "advisory signals… say which tier a concept is in rather than treating any of them as a gate".
- **`backfill`**: LLM map/reduce over git and Claude-session history. It dispatches the `okf:event-analyzer` subagents through "Claude Code's Workflow `agentType`" and a single `okf:bundle-weaver` writer, with a "host-agnostic fallback: spawn generic subagents". Reconstructed concepts are `status: draft` with `generated.by: okf-backfill/<ver>`, never `human:`. A deterministic `--check-coverage` confirms every live event is mapped. [not run]: LLM-driven and requires Claude subagents; also outside our "agent-curated, no extraction" scope.
- **Soft mode**: `templates/CLAUDE-okf.md`, pasted into CLAUDE.md, gives the before-task read and after-change write rules. **Enforced mode**: the Stop hook, described above.

### Portability
- **Cursor**: `npx skills add` installs the skills into `.agents/skills/`, and the frontmatter (`name`, `description`, `allowed-tools`) is within the Agent Skills spec. However, every command uses `${CLAUDE_SKILL_DIR}` and `$ARGUMENTS`, which Cursor does not expand [INFERENCE: Claude-specific substitution]. The agent has to work out the script path, and `okf` → `../validate/…` assumes sibling install. The skills CLI compatibility table lists Hooks: No for Cursor, so there is no enforced mode. MCP must be hand-registered in `.cursor/mcp.json`. No subagents, so no backfill. Result: the read, write and validate workflow ports, with path friction.
- **omp**: skills are discovered from `.claude/skills`, `.agents/skills` and Claude marketplace plugin roots (omp docs: `claude`, `agents` and `claude-plugins` skill providers). omp appends `[Skill directory: <baseDir>]` and tells the model to resolve relative paths against it. `${CLAUDE_SKILL_DIR}` is not listed as expanded, but the agent can substitute it. omp hooks are TS factories (`.omp/hooks/pre|post`), so the bash `Stop` hook does not carry over [INFERENCE: not tested]. omp discovers MCP from `.claude/`, `.cursor/` and similar files; whether it loads a plugin's `.mcp.json` was [not verified]. `$ARGUMENTS` is supported in omp slash commands.

## Strengths
1. **The strictest and most spec-faithful validator seen.** It separates §11 errors from warnings, accepts RFC 3339 `stale_after`, checks the footnote↔`sources[].id` join, flags `Human:` actor near-misses and nested-index frontmatter, emits `--json`, has a CI Action, and `--migrate` for v0.1 preserves unknown keys and comments.
2. **Clear, Claude-native agent protocol.** Produce, maintain and consume modes with explicit read triggers (index-first, weigh draft/stale/unverified), write rules (`generated` stamp on every edit, `human:` for sign-off, lifecycle-only log), "validate before done", plus an opt-in Stop-hook backstop. Diffs stay small because agents edit plain files.
3. **Correct link semantics and cross-repo backlinks on an assembled copy hub.** T4/T5 matched every planted expectation. Install is zero-daemon and offline once the cache is warm, with no telemetry in the tool itself.

## Blockers (for the hybrid 10–50 repo layout)
1. **No symlinked hub, no federation.** A symlinked hub validates as 0 concepts with exit 0 (a silent false pass), and the MCP server refuses symlinked concepts. There is no multi-bundle workspace, so the hub must be a copied tree. Each tool takes one bundle root, and the hook, the MCP default and the templates all hardcode `.okf` rather than `okf/`.
2. **No agent-facing retrieval CLI, and MCP does not scale.** Search and backlinks exist only through MCP, which re-parses everything per call: 7.4 s search and 11.5 s backlinks at 10k concepts (targets 2 s). There is no staleness or trust-tier field in agent output; that is computed only in the browser viz.
3. **Write-side tooling is minimal.** No index generator (and no index completeness check), no concept create/stamp/verify command, no move/rename with link rewrite (T8 detects only), and no stale-concept report. Everything relies on the agent hand-editing correctly plus the validator.

## Surprises
- `rglob` skipping symlinks makes `scale/hub` "conformant — no issues" at 0 concepts. The skills never mention symlinks.
- The Stop hook ignores untracked non-`.okf` files, so a brand-new source file with no concept does not trigger it. Any touch under `.okf/` silences it.
- `stale_after` comes back from MCP as `"2026-09-01 00:00:00+00:00"` (PyYAML datetime `str`), not the ISO string written in the file.
- The repo documents itself in `.okf/` (with `upkeep: enforced`) and ships pre-registered benchmarks on trust metadata.

## Fit for the hybrid layout
Good as the **conformance and CI gate** (validator + Action) for each `<repo>/okf/` bundle and for a *copied* hub. It is also a good **skill text to borrow** for the Claude agent workflow. It is weak as the retrieval and navigation layer for 50 repos: no symlink or federation support, MCP-only search and backlinks too slow at 10k concepts, and no index, move or stale tooling. It would need pairing with a faster CLI (or plain grep) plus a hub-assembly step that copies rather than symlinks, and `.okf`→`okf` path overrides (`OKF_BUNDLE`, hook patch).

Artifacts: `work/scale-hub-copy.json`, `work/scale-neighbors.txt`, `work/hubcopy-viz.html`, `mcpq.py` (throwaway MCP client), `bench.sh`.
