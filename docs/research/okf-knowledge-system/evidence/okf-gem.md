# okf (serradura/okf, Ruby gem) — OKF v0.2 tool evaluation

- Tool: `okf` gem **2.2.0** (RubyGems), repo https://github.com/serradura/okf @ `cfb7ffa` (2026-09-06), docs https://okfgem.com
- Env: ruby 3.3.8, gem 3.6.7, 12 cores. Work dir `/tmp/okf-research/okf-gem/` (fixture copy `fx/`, T7 `t7/` `t7b/`, T8 `t8/`).
- All commands ran with `GEM_HOME=$W/gems HOME=$W/home OKF_HOME=$W/home/.okf`, binary `$W/gems/bin/okf` (`$W`=work dir).
- Sibling gems in the same repo (not installed): `okf-mcp` (read-only MCP tools: list_bundles, dirs, index, search, read_concept, catalog, log, validate, lint, graph, references, tags, types, stats), `okf-tui`, `okf-pro` (an opinionated profile with edit/commit/CI gates). Claude Code plugin in `plugin/` (skill + PostToolUse hook + `/okf:gem` command).

## T1 Install — PASS

```
$ /usr/bin/time -v env HOME=$W/home GEM_HOME=$W/gems gem install okf --no-document
Successfully installed rack-3.2.7
Successfully installed minifts-1.0.1
Successfully installed okf-2.2.0
3 gems installed
  Elapsed (wall clock) time: 0:01.48
```
- Runtime deps (gemspec): `rack >= 2.2`, `webrick >= 1.4` (already shipped with ruby 3.3, not downloaded), `minifts ~> 1.0` (the search engine library). **3 gems downloaded**: okf 489 KB, rack 121 KB, minifts 24 KB (`.gem` files); installed size okf 1.7 MB, rack 532 KB, minifts 84 KB (~2.5 MB total apparent incl. cache/specs). Pure Ruby, no native extensions, `required_ruby_version >= 2.4`.
- License: Apache-2.0 (gemspec + LICENSE.txt + NOTICE).
- One binary `okf`; extensions (okf-mcp, okf-tui) add verbs via an `okf/plugin.rb` seam, no second binary. Docker image `ghcr.io/serradura/okf` referenced by the shipped CI template (`resources/ci/github/okf.yml`), not tested.
- **Network/telemetry**: source grep of `lib/**/*.rb` for `Net::HTTP|open-uri|URI.open|TCPSocket|Socket.|require 'net…'|telemetry|analytics` → **no hits**. `strace -f -e trace=network` on `okf search` and `okf lint` → **0 syscalls**; `strace -e trace=connect okf render` → 0. Read verbs wrote nothing to `$HOME` (only `registry` writes, to `$OKF_HOME/registry.json` or project `.okf.json`).
  - Caveat: the HTML graph (`okf render`, `okf server`) loads **third-party assets in the browser**: cytoscape/marked/dompurify/mermaid/minisearch/dagre/cola/panzoom from `cdn.jsdelivr.net`, Poppins from `fonts.googleapis.com`, og image from `okfgem.com`. So the CLI is offline; the graph viewer is not.
- No daemon required. `okf server` (webrick, default bind 127.0.0.1:8808) is optional.

## T2 Validate — PARTIAL

`validate` = §11 conformance only (hard errors → exit 1); broken links / format issues are warnings. `lint` = curation checks (exit 0 unless `--fail-on warn|info`). Both take `--json`.

```
$ okf validate workspace/billing-api/okf
  concepts: 3   index.md: 1   log.md: 0
  ! warn  contracts/invoice-api.md: stale_after should be a YYYY-MM-DD date
  ! warn  gotchas/idempotency-key.md: stale_after should be a YYYY-MM-DD date
  ✓ conformant (2 warning(s))                                   exit=0
$ okf validate workspace/web-app/okf
  ! warn  services/web-app.md: cross-link target not found: `../../billing-api/contracts/invoice-api.md` (tolerated under §6.1)
  ! warn  services/web-app.md: cross-link target not found: `../../shared-auth/concepts/session-token.md` (tolerated under §6.1)
  ✓ conformant (2 warning(s))                                   exit=0
$ okf validate workspace/shared-auth/okf
  ✓ conformant — no issues                                      exit=0
$ okf validate workspace/hub          # symlinked
  concepts: 2   index.md: 1   log.md: 0
  ! warn  cross-repo/invoice-dependency.md: cross-link target not found: `/repos/web-app/services/web-app.md` (…)
  ! warn  … `/repos/billing-api/contracts/invoice-api.md` …   ! warn … `/repos/web-app/flows/checkout.md` …
  ✓ conformant (3 warning(s))                                   exit=0
$ okf validate hub-copy
  concepts: 9   index.md: 4   log.md: 0
  ✗ ERROR  repos/billing-api/index.md: nested index.md must not include frontmatter
  ✗ ERROR  repos/shared-auth/index.md: nested index.md must not include frontmatter
  ✗ ERROR  repos/web-app/index.md: nested index.md must not include frontmatter
  ! warn  repos/billing-api/contracts/invoice-api.md: stale_after should be a YYYY-MM-DD date
  ! warn  repos/billing-api/gotchas/idempotency-key.md: stale_after should be a YYYY-MM-DD date
  ! warn  repos/billing-api/gotchas/idempotency-key.md: cross-link target not found: `/contracts/invoice-api.md` (…)
  ✗ non-conformant (3 error(s))                                 exit=1
```

What it caught:
| Planted | Result |
|---|---|
| Nested index frontmatter in hub | **Caught as ERROR** (strict §8/§12) → hub-copy exit 1 |
| Broken links (root-relative in hub, `../../` standalone, `/repos/` in symlink hub) | Caught as warnings (validate) / `Backlog` info (lint `missing_concept`) |
| Stale concept `idempotency-key` (`stale_after: 2026-09-01T00:00:00Z`) | **Missed.** Datetime form is rejected (`stale_after should be a YYYY-MM-DD date`), then the `expired` check ignores it. With a copy changed to `stale_after: 2026-09-01`, plain `okf lint` reports `· info gotchas/idempotency-key.md: expired on 2026-09-01 (stale_after)` (CLI supplies today's date; `--today` overrides). `expired` is **info**, so `--fail-on warn` does not gate it. Separate opt-in `--stale-after 90d` flags on `generated.at` age: `! warn gotchas/idempotency-key.md: last updated 2026-06-01 12:00:00 UTC; older than cutoff …` |
| Footnote ↔ `sources[].id` join | Valid join → no finding. Tested on a copy: `[^nope]` without definition → `! warn services/billing-api.md: footnote [^nope] has no matching sources[].id` (`unattributed_claim`) and `· info … source main-go is never cited by a footnote` (`unused_source`). A footnote that has its own `[^x]:` definition is treated as an ordinary GFM footnote (not flagged). |
| Trust tiers | `lint` stats line: `trust: unverified 1, machine-confirmed 1, human-reviewed 1   status: stable 2, draft 1` (billing-api). The bare mapping `verified: {by: human:alice, …}` is accepted as a 1-element list. |
| Deprecated (`legacy-api-key`) | Counted in `status: deprecated 1`; also flagged `! warn … unreachable: no inbound links and not listed in any index.md` |

25 lint checks: orphan, not_in_index, disconnected_component, unlinked, missing_concept, broken_index_entry, stub, missing_title, missing_description, missing_generated, expired, stale, uncited_external, broken_source, unattributed_claim, unused_source, unprefixed_actor, incomplete_computation, broken_attestation_ref, legacy_timestamp, legacy_citations, duplicate_title, unused_reference_def, undefined_reference, self_link, log_order.

Verdict PARTIAL: strong checks, but spec-form datetime `stale_after` is rejected so the planted stale concept is not detected (same as other tools), and the hub layout itself is non-conformant.

## T3 Symlinks — FAIL

`workspace/hub` sees **2** concepts vs **9** in `hub-copy` (`okf files workspace/hub` lists only `cross-repo/`, `decisions/`; `okf search workspace/hub idempotency` → 0 of 2). Scale: `validate scale/hub` → `concepts: 0`. Cause (source): `Reader` uses `Dir.glob("**/*.md")` (Ruby `**` does not descend symlinked dirs) and additionally `File.realpath`-checks every file, putting any symlink escaping the root into the unparseable bucket — deliberate containment (`safe_read.rb`). So symlinked assembly is unsupported by design.

## T4 Link resolution — PASS (resolution is correct per view)

Evidence: validate/lint warnings above + `okf graph --json --minimal` edges.
| Link form | standalone repo | hub-copy | hub (symlink) |
|---|---|---|---|
| `../contracts/invoice-api.md` (billing-api.md) | resolves | resolves | not scanned |
| `/contracts/invoice-api.md` (idempotency-key, root-relative) | resolves (billing-api hub count `contracts/invoice-api ×2`) | **broken** (warn) | not scanned |
| `../../billing-api/…`, `../../shared-auth/…` (web-app) | **broken** (warn) | resolves (edge `repos/web-app/services/web-app → repos/billing-api/contracts/invoice-api`) | not scanned |
| URL `https://…` (web-app) | external, not an edge; lint `· info body has external link(s) but no sources` | same | — |
| `/repos/…` (hub invoice-dependency) | — | resolves (3 edges) | **broken** (3 warns) |

Root-relative `/x.md` is treated as bundle-root-relative (spec §6). No link rewriting or cross-bundle resolution.

## T5 Backlinks across repos — PASS (hub-copy only; no dedicated verb)

No `backlinks <id>` verb. Options: `okf graph <dir> --json --minimal` + filter, `okf graph --hubs` (inbound ranking), `okf catalog --json` (`links_in` count), or the server UI "Linked from" panel.
```
$ okf graph hub-copy --json --minimal | jq -r '.edges[]|select(.target=="repos/billing-api/contracts/invoice-api")|.source'
cross-repo/invoice-dependency
repos/billing-api/services/billing-api
repos/web-app/services/web-app
$ okf graph hub-copy --hubs
  repos/billing-api/contracts/invoice-api    ×3   repos 2, cross-repo 1
  …
```
Exactly the 3 expected; idempotency-key's broken root-relative link correctly excluded. Symlinked hub: 0 (T3). `graph`/`lint` refuse multi-bundle groups (`error: @backend names a group of 2 members; only okf search and okf server take a group`, exit 2).

## T6 Search / retrieval — PASS (with gaps)

```
$ okf search hub-copy idempotency
Search — hub-copy · idempotency (3 of 9 concepts)
  repos/billing-api/gotchas/idempotency-key  Invoice creation needs an idempotency key  ·  Gotcha  ·  title+id+description+body
  repos/billing-api/contracts/invoice-api    Invoice API v2  ·  API Contract  ·  body
  repos/billing-api/services/billing-api     Billing API  ·  Service  ·  body
$ okf search hub-copy idempotency --json   → {"bundle","query","count","matches":[{"id","title","type","dir","top_dir","tags","matched","score","snippet"}]}
```
- Machine-readable: every read verb has `--json`, `--pretty`, and `--fields/--except` projection (search/index/catalog/files).
- Trust/status/staleness: **not in search rows** (`--fields trust` → `error: unknown field(s): trust, status, stale_after (available: id, title, type, dir, top_dir, tags, matched, score, snippet)`), but search **filters** by `--trust`, `--status`, `--type`, `--dir`, `--tag`, `--in FIELDS` (`--trust unverified` → only idempotency-key). `okf catalog --json` gives per concept: `trust`, `status`, `stale_after` (raw), `generated_by/at`, `sources`, `links_in/out`. No "is expired" boolean outside lint.
- Engines: default scan (literal substring, `-e` regexp); `--engine index` = BM25+ via minifts (`--fuzzy`). Index engine returned only 2 hits (missed invoice-api, where the term is inside a backtick code span in a table) — documented behavior ("the index shatters identifiers/backticks").
- Progressive disclosure: `okf dirs` (one row per dir), `okf index <dir> --dir <branch> [--depth N] [--no-body]` (the §8 map: authored index body + computed listing + rollups), `graph --minimal`, `stats`, `types`, `tags`. **These are read-only views; there is no index.md generator.**
- Cross-bundle: `okf search @a @b …` / `@group` / `@all` (T9).

## T7 Agent write path — PARTIAL

The CLI has **no write verbs** (no new/stamp/verify/index-regenerate/log-append). The skill's model is: agent writes files with its own Write/Edit tools from `templates/` (concept, index, root-index, log, attested-computation), then runs `validate` + `lint` as the gate (automatically via a PostToolUse hook in the Claude Code plugin).

Performed per skill (in `t7/`, a git repo copy of billing-api):
1. Wrote `gotchas/retry-backoff.md` from the template: block-style `generated: {by: omp/claude-opus, at: 2026-09-29T10:00:00Z}`, `sources[]` + matching footnote, unknown key `x_owner: team-billing`.
2. `okf lint .` → `! warn gotchas/retry-backoff.md: unreachable: no inbound links and not listed in any index.md` (catches the missing index entry).
3. Added `verified: [{by: human:novpla, at: 2026-09-29T12:00:00Z}]`, hand-wrote `gotchas/index.md` and `log.md` (`## 2026-09-29` / `* **Creation**: …`).
4. `okf validate .` exit 0; `okf lint . --fail-on warn` → `✓ healthy — no issues` exit 0; `okf catalog . --json` → `{"trust":"human-reviewed","generated_by":"omp/claude-opus"}`.
5. `git diff --cached --stat`: `gotchas/index.md +4, gotchas/retry-backoff.md +26, log.md +4 — 3 files, 34 insertions`. Unknown keys untouched (the CLI never rewrites files).

Library write path (`OKF::Bundle::Folder.load(dir).save(overwrite: true)`) — **destructive, avoid**: in `t7/` it replaced the whole directory and **deleted `.git`**; in `t7b/` (git at parent) it deleted non-md file `okf/NOTES.txt`, left `.okf.okf.lock` in the parent, reformatted YAML (flow → block, re-indented lists) and rewrote datetimes to `2026-09-20 10:00:00.000000000 Z` (non-ISO-8601 `T` form): 4 files, +30/−14 for zero semantic change. Unknown key `x_owner` was preserved.

Verdict PARTIAL: the agent-edits-markdown + validate/lint-gate path works and yields small reviewable diffs, but stamping, index regeneration and log append are manual (skill-guided), and the only programmatic writer is unsafe for in-repo use.

## T8 Move/rename — FAIL (detection only)

No move/rename verb. In `t8/` (hub-copy copy) `mv repos/billing-api/contracts/invoice-api.md …/invoice-api-v2.md`:
```
$ okf lint . --only missing_concept,broken_index_entry
  Backlog
    · info  repos/billing-api/contracts/invoice-api.md: referenced by 3 link(s) across 3 concept(s) but does not exist
    · info  contracts/invoice-api.md: referenced by 1 link(s) across 1 concept(s) but does not exist
  ✓ 0 warn, 2 info
$ okf validate .   → 4 × "! warn … cross-link target not found: …invoice-api.md"
$ grep -rln "contracts/invoice-api.md" .   → 4 files still point at the old path
```
Nothing rewritten. Dead links after a move are only **info** in lint (`missing_concept` is treated as a "backlog" of to-be-written concepts), so `lint --fail-on warn` does not catch them; `validate` warns but exits 0. The refine playbook tells the agent to repoint links itself and prefer bundle-absolute links "so they survive moves".

## T9 Multi-bundle without hub — PARTIAL

Native **registry**: `okf registry init` creates a project-local `.okf.json` (nearest wins, relative paths — committable, e.g. in a hub repo); `set <dir> --as slug`, `group`, `link` (chain to another registry), `import`. Global registry at `$OKF_HOME` (default `~/.okf`).
```
$ okf registry init; okf registry set billing-api/okf --as billing-api …; okf registry group backend @billing-api @shared-auth
$ cat .okf.json → {"bundles":[{"slug":"billing-api","path":"billing-api/okf",…},…],"groups":[{"slug":"backend","members":[…]}],"links":[]}
$ okf search @all idempotency
Search — @billing-api @web-app @shared-auth @hub · idempotency (3 of 9 concepts)
  @billing-api  gotchas/idempotency-key  …
$ okf search @backend session   → 2 hits in @shared-auth
$ okf graph @backend --hubs     → error: @backend names a group of 2 members; only `okf search` and `okf server` take a group   (exit 2)
```
- Spans bundles: **search** (JSON rows carry `slug`) and **server** (multi-bundle hub UI; `[not run]` — long-lived web server, not needed for CLI verdict).
- Does not span: graph, backlinks, lint, validate, catalog, index.
- Cross-bundle links: none resolved. The authors' own convention (repo `AGENTS.md`, `.okf/index.md`): "A concept cannot link out of its bundle, so a reference across the line names the other in prose: `` `@okf-eco format/frontmatter` ``" — an unresolved prose ref; `../../repo/…` links show as broken in standalone view.
- The upstream repo itself runs this exact shape (several bundles + root `.okf.json`, `okf search @all`).

## T10 Scale — PARTIAL

`/usr/bin/time -f %e`, 1 warmup + 5 runs (`bench.sh`). 10,000 concepts / 10,051 md.
| Operation | min | median | target | Correct? |
|---|---|---|---|---|
| validate `scale/hub-copy` | 3.91 s | 4.08 s | <10 s ✓ | 50 ERRORs (nested index frontmatter), exit 1 |
| lint `scale/hub-copy` (all 25 checks) | 12.55 s | 12.89 s | — | — |
| search `zebracorn-needle` hub-copy (default scan) | 2.08 s | 2.14 s | <2 s ✗ (marginal) | 1 hit `repos/r37/d3/c123` ✓ |
| search, `--engine index` (BM25, rebuilds index per call) | 11.50 s | 11.95 s | <2 s ✗ | 1 hit ✓ |
| backlinks `repos/r37/d3/c123` (`graph --json --minimal \| jq`) | 4.36 s | 4.42 s | <2 s ✗ | `repos/r37/d1/c091`, `repos/r37/d4/c094` = grep ground truth ✓ |
| `index` map view `--no-body` (read-only; no generator) | 1.93 s | 1.96 s | — | index **generation** N-A (no verb) |
| validate `scale/hub` (symlink) | 0.12 s | 0.12 s | — | ✗ 0 concepts seen |
| search `scale/hub` (symlink) | 0.12 s | 0.12 s | — | ✗ 0 hits |
| `search @all` over 50 registered standalone bundles | 1.93 s | 1.97 s | <2 s ✓ | 1 hit `@r37 d3/c123` ✓ |
| validate loop over 50 standalone bundles (50 processes) | 9.84 s | 9.90 s | <10 s ✓ (marginal) | all exit 0 |
| validate single bundle r37 (200 concepts) | 0.19 s | 0.20 s | | |
| search single bundle r37 | 0.16 s | 0.17 s | | |
| backlinks c123 in standalone r37 | 0.19 s | 0.20 s | | same 2 sources ✓ |

No persistent index/cache: every call re-parses every file (Ruby startup ≈0.12 s). Per-repo operations are instant; whole-hub operations are 2–13 s.

## Agent Skill — workflow summary

Shipped in the gem (`lib/okf/skill/`, 27 files) and installable with `okf skill <dest>` → `<dest>/skills/okf/` (`--here` to install flat). Also in repo `skills/okf` and the Claude Code plugin (`plugin/`, adds `/okf:gem` + a PostToolUse hook on `Write|Edit|MultiEdit` that runs `validate` + `lint` and returns findings as context). A second skill `okf-principles` is about restructuring agent instruction files, not knowledge.

- **Triggers** (frontmatter `description`): "document this in OKF", "capture as a concept", migrate docs, "what do we know about X / where is X documented", "update the knowledge bundle", validate/lint, graph; or working in a repo that has a `.okf/` dir or a root `index.md` with `okf_version`. Verbs: menu, search, produce, migrate, maintain, refine, consume, curate, doctor, or any CLI verb.
- **Read path** (search/consume playbooks): run the CLI, don't probe or grep. Start with `okf dirs`, then `okf index --dir <branch>`, read `log.md`, use `okf search` (default literal; `--engine index` for themes, `--fuzzy` for typos), filter by `--type/--dir/--tag/--trust/--status`, then read only the winning files and follow links one hop at a time. Search across bundles with `@slug`/`@all`. No bundle in cwd → `okf registry list`. Anti-patterns: dumping `graph --json`, grep before map, retrying synonyms mechanically.
- **Write path** (produce/maintain): update bodies plus `generated.at/by` (actor in §7 form; a tool writes its own `<producer>/<version>`, never `human:<id>`), fix cross-links, re-enumerate every touched `index.md`, append a dated `log.md` entry (durable facts only), then a closeout gate (`validate` + `lint`). "Write-back reflex": if consuming reveals a gap or stale fact, switch to maintain. Trust is derived from `verified[].by` prefixes (`human:` ⇒ human-reviewed); never store a tier. `stale_after` = absolute `YYYY-MM-DD`; `lint` `expired` reports it; `--stale-after 90d` is a separate opt-in age check. Semantic staleness and contradictions are the agent's job, not the CLI's.
- **Portability**: the skill body is plain markdown with relative links and shell commands — agent-neutral. Frontmatter keys `allowed-tools`, `user-invocable`, `argument-hint` are Claude-specific but harmless elsewhere. The automatic validate/lint on edit is **Claude-plugin-only** (a Claude `hooks.json` using `${CLAUDE_PLUGIN_ROOT}`); on Cursor/omp the skill tells the agent to run the checks itself ("Without the plugin, the skill itself instructs the agent to run the same checks"). `okf skill .cursor` / `okf skill .omp` produce `<dir>/skills/okf` layouts [INFERENCE: pickup by Cursor/omp not tested]. Friction for our layout: the default bundle location and trigger is `.okf/`, not `okf/`. The skill says to detect an existing location, but a repo AGENTS.md pointer or a registry entry is needed for reliable triggering.

## Strengths
1. Offline, dependency-light CLI (3 small pure-Ruby gems, ~1.5 s install, 0 network syscalls, Apache-2.0) with `--json` + field projection on every read verb, and filters by trust/status/type/dir/tag.
2. Best-in-class curation checks: strict §11 validator plus 25 lint checks, including the footnote↔`sources[].id` join, derived trust tiers, `expired`, orphans/not-in-index, missing concepts, and actor-prefix nudges. A CI template and a Docker image are provided.
3. Mature agent skill (playbooks for search/produce/maintain/refine/curate, progressive-disclosure discipline, provenance rules, write-back reflex), plus native multi-bundle registry (`.okf.json`, groups, `search @all`) that matches the per-repo-bundle model. The upstream repo uses this model on itself.

## Blockers (for the hybrid layout)
1. **Hub assembly is broken by design**: symlinked repo bundles are invisible (Dir.glob + realpath containment). Copied bundles make the hub **non-conformant** (nested `index.md` with `okf_version` = ERROR, exit 1). Root-relative links break once a bundle is nested. Federation instead means the registry, which only spans `search`/`server`: no cross-bundle graph, backlinks, link validation or lint. The upstream convention is prose `@slug id` references.
2. **No write/move tooling**: no stamp/verify/new/index-regenerate/log-append/move verbs, and no link rewriting. After a move, dead links are only lint *info* (not gated by `--fail-on warn`). The library `Folder#save` rewrites the whole directory (deleted `.git` and non-md files, reformats YAML, writes non-ISO datetimes).
3. **Spec datetime `stale_after` rejected** (warn + not evaluated), so the planted stale concept is missed. Scale: no persistent index, so hub-wide search is 2.1 s, backlinks 4.4 s, lint 12.9 s, BM25 12 s per call. Graph viewer pulls CDN/Google Fonts in the browser.

## Surprises
- `expired` is severity *info* on purpose, and a moved/missing link target is *info* ("backlog"). A `--fail-on warn` CI gate is blind to both.
- Search rows lack trust/status; `catalog --json` has them.
- `search --engine index` silently misses terms inside backtick code spans (documented).

## Fit for hybrid layout
Good fit for **per-repo bundles + CLI + skill** (validate/lint in each repo's CI, `search @all` via a committed `.okf.json` in the hub repo). Poor fit for an **assembled hub bundle**. With okf, the hub should be its own small bundle holding cross-repo relations, with the repo bundles registered in it, not nested. Cross-repo edges then become unresolved prose/`@slug` references, and cross-repo backlinks need custom tooling (e.g. `graph --json` per bundle + a join script).
