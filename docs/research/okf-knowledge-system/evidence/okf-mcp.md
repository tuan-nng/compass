# okf-mcp 0.9.1 (`@mfdaves/okf-mcp`) — OKF v0.2 tool evaluation

- Repo: https://github.com/mfdaves/okf-mcp (MIT, 7 stars, 1 fork, 1 contributor, created 2026-06-21, last push 2026-09-19).
- Evaluated: CLI (`okf` / `okf-mcp`, same entrypoint `bin/okf-mcp.js`); stdio MCP driven with a tiny Python JSON-RPC client (`/tmp/okf-research/okf-mcp/mcp_drive.py`, `mcp_warm.py`).
- Aliases below: `O=/tmp/okf-research/okf-mcp/node_modules/.bin/okf`, `F=/tmp/okf-research/fixture`, `S=/tmp/okf-research/scale`, `HOME=/tmp/okf-research/okf-mcp/home` for every run.
- Timing method (hyperfine not installed): `bench.sh` = 1 warmup + 5 runs of `/usr/bin/time -o … -f '%e %M'`, reporting min/median wall seconds and max RSS.

## Verdict table

| Test | Verdict | One-liner |
|---|---|---|
| T1 Install | PASS | 1 cmd, 1.6 s, 11 pkgs / 20 MB, Node ≥22, no runtime network (strace: 0 socket/connect), no telemetry code |
| T2 Validate | PARTIAL | Catches nested-index frontmatter, broken links, outside-root links; **flags spec-format `stale_after` as invalid → stale never detected**; no footnote/source join check; dir links `services/` reported as broken |
| T3 Symlinks | FAIL | Walker skips symlinked dirs: symlink hub = 2 concepts vs 9 in hub-copy; scale hub = 0 concepts |
| T4 Link resolution | PASS (hub-copy) / FAIL (symlink hub) | All planted forms behave as expected in hub-copy and standalone; URL body links silently dropped |
| T5 Backlinks | PASS (hub-copy) / FAIL (symlink hub) | Exactly the 3 expected inbound edges |
| T6 Search | PARTIAL | JSON, trust tier/status/freshness per hit, rich filters; **backtick-wrapped identifiers not matched by plain terms**; no index.md generation |
| T7 Write path | PARTIAL | MCP `okf_apply_changes` works (generated stamp, atomic batch, optional git commit); no CLI write; frontmatter reformatted + comments lost; can't update the stale concept; no index/log maintenance; agent can write `verified: human:*` |
| T8 Move/rename | FAIL (N-A by design) | Move intentionally unsupported; manual move → validate (`--strict-links`) reports all 3 now-broken inbound links, rewrites none |
| T9 Federation | PARTIAL (strong) | `okf.project.yaml` / repeated `--bundle id=path` federate search + backlinks; cross-bundle links only via non-standard `relations:` with `okf://bundle/id`; `okf://` in Markdown body links ignored; yaml rejects absolute/`..`/symlink paths |
| T10 Scale | PARTIAL | Cold CLI: validate 1.8 s, backlinks 1.87 s, search **2.9 s** (> 2 s target); warm MCP: search 14 ms, neighbors 2 ms; symlink hub unusable |

---

## T1 Install — PASS

```
$ npm view @mfdaves/okf-mcp dist-tags license   → latest 0.9.1, MIT; deps: js-yaml ^5.2.3, commonmark ^0.31.2, minisearch 7.2.0, @modelcontextprotocol/server 2.0.0
$ /usr/bin/time -v npm install --prefix /tmp/okf-research/okf-mcp @mfdaves/okf-mcp@0.9.1
added 11 packages in 1s
Elapsed (wall clock) time: 0:01.57      Maximum resident set size: 148652 KB
$ du -sh node_modules node_modules/@mfdaves/okf-mcp  → 20M total, 688K for the package itself
  (largest: zod 8.1M, @modelcontextprotocol 7.6M, js-yaml 1.6M)
$ ls node_modules/.bin → okf, okf-mcp (both → @mfdaves/okf-mcp/bin/okf-mcp.js), commonmark, js-yaml
$ $O --version → 0.9.1          ($O --version cold start: median 0.25 s)
```
- Runtime: Node ≥22 (engines). Pure JS, ~13.7k lines in `src/`.
- Network: source grep shows fetches only in `src/remote.js` (`api.github.com`, used only by `--remote-bundle` / `load_remote_bundle`). `strace -f -e trace=connect,socket $O --root $F/hub-copy validate` → **0 lines**. No telemetry/analytics strings in `src/`. HOME contained only the npm cache after all runs (no config/state written). `unshare -rn` offline check: [not run] — `unshare` not permitted (uid_map EPERM).
- Writes state only when asked: proposals under `.okf-proposals/` (authoring mode), `.okf-producer.json` (producers).

## T2 Validate — PARTIAL

Commands: `$O --root <bundle> validate` (JSON with `conformant`, `validForProject`, `diagnostics`, plus duplicated `errors`/`warnings`/`advisories`/`projectDiagnostics` arrays — verbose).

| Bundle | exit | conformant / validForProject | Diagnostics (trimmed) |
|---|---|---|---|
| billing-api/okf | 0 | true / true | warn `invalid_stale_after` ×2 (contracts/invoice-api, gotchas/idempotency-key); warn `broken_link` index.md → `services/`, `contracts/`, `gotchas/` |
| web-app/okf | **1** | true / **false** | warn `link_outside_root` services/web-app.md → `../../billing-api/...`, `../../shared-auth/...` (these invalidate the project); warn `broken_link` index.md → `services/` |
| shared-auth/okf | 0 | true / true | warn `broken_link` index.md → `concepts/` |
| workspace/hub (symlinks) | 0 | true / true | warn `broken_link` ×3 for `/repos/...` links (symlinked repos invisible, see T3); index.md dir links |
| hub-copy | **1** | **false** / false | **error `reserved_index_frontmatter_not_allowed`** repos/{billing-api,shared-auth,web-app}/index.md; warn `broken_link` repos/billing-api/gotchas/idempotency-key.md → `/contracts/invoice-api.md`; `invalid_stale_after` ×2; dir-link warnings |

Planted-fact checks:
- **Stale concept: NOT caught.** `stale_after: 2026-09-01T00:00:00Z` (the exact form in SPEC §5.5 example and §5 "every timestamp is an ISO 8601 datetime with UTC offset") is rejected as `invalid_stale_after` — `src/v02.js validIsoDate()` accepts only `^\d{4}-\d{2}-\d{2}$`. Signals then show `freshness: "invalid"`, and `search --freshness stale` returns 0. Proof it works only with date-only (non-spec) values: copy `t2s/` with `stale_after: 2026-09-01` → `search "" --freshness stale` → `[{"c":"gotchas/idempotency-key","f":"stale","s":"2026-09-01"}]`; `--as-of 2026-08-01` → 0. Even then, `validate` never reports staleness; it's only a search filter/signal.
- Broken links: caught (root-relative link broken inside hub; hub-relative links flagged `link_outside_root` standalone). Advisory by default; `--strict-links` / `strictLinks: true` makes them project-invalid. Note: bare directory links in index.md (`[Services](services/)`, the SPEC §8 example form) are reported broken when the dir has no index.md → noisy.
- Nested index frontmatter: caught as a **conformance error** (exit 1) in hub-copy — matches strict reading of §8.
- Footnote ↔ `sources[].id` join: **not checked** (no footnote handling in `src/`; grep "footnote" → 0 hits).
- `verified` mapping → trust tiers correct: `--inspect` on hub-copy → `byTrustTier: {human-reviewed: 3, unverified: 5, machine-confirmed: 1}`; status: `{stable: 7, draft: 1, deprecated: 1}`.
- Quirk: `--inspect` output lists `"errors": []` even when `validate` reports conformance errors.

## T3 Symlinks — FAIL

```
$ $O --root $F/workspace/hub --inspect | jq '{documents,concepts}'   → documents 3, concepts 2 (hub only)
$ $O --root $F/hub-copy --inspect                                    → documents 13, concepts 9
$ $O --root $S/hub --inspect                                         → {"documents":1,"concepts":0}
$ $O --root $S/hub search zebracorn-needle                           → total 0
```
Cause: `src/indexer.js walkMarkdown()` uses `readdirSync(withFileTypes)` and only recurses `entry.isDirectory()`; symlinks are neither followed nor reported. Additionally `okf.project.yaml` bundle roots **reject** any path that traverses a symlink (`project_path_symlink`, `src/project.js`), and authoring refuses to write through symlinks. Symlink-assembled hubs are a non-starter; use copies or federation (T9).

## T4 Link resolution — PASS in hub-copy / FAIL in symlink hub

`$O --root <view> graph json --include-external | jq '.edges[]|…'` and `validate`:

| Link form | Standalone | hub (symlink) | hub-copy |
|---|---|---|---|
| idempotency-key → `/contracts/invoice-api.md` (root-relative) | resolves (edge, broken:false) | concept invisible | **broken** (warn `broken_link`) ✔ expected |
| web-app → `../../billing-api/contracts/invoice-api.md` | `link_outside_root` (project-invalid, exit 1) ✔ | invisible | resolves → `okf://hub-copy/repos/billing-api/contracts/invoice-api` ✔ |
| web-app → `../../shared-auth/concepts/session-token.md` | `link_outside_root` ✔ | invisible | resolves ✔ |
| web-app → `https://github.com/acme/billing-api/.../invoice-api.md` (URL) | silently dropped: not in `concept.links`, not a graph edge, not validated | — | same |
| hub `/repos/<repo>/...` links | n/a | **broken** (3 warnings) ✘ | resolve ✔ |
| `resource:` / `sources[].resource` URLs | external leaves (`resource`, `source` edges) | same | same |

## T5 Backlinks across repos — PASS (hub-copy) / FAIL (symlink hub)

```
$ $O --root $F/hub-copy neighbors repos/billing-api/contracts/invoice-api | jq '[.inbound[].edge.source]'
okf://hub-copy/cross-repo/invoice-dependency       (href /repos/billing-api/contracts/invoice-api.md)
okf://hub-copy/repos/billing-api/services/billing-api  (href ../contracts/invoice-api.md)
okf://hub-copy/repos/web-app/services/web-app      (href ../../billing-api/contracts/invoice-api.md)
$ $O --root $F/workspace/hub neighbors repos/billing-api/contracts/invoice-api
Unknown OKF concept URI or ID: repos/billing-api/contracts/invoice-api     (exit 1)
```
Exactly the 3 expected; idempotency-key's link correctly absent (broken in hub). Each neighbor carries full `signals` (status, trustTier, freshness, generated). `paths <from> <to>` also works (T9).

## T6 Search / retrieval — PARTIAL

```
$ $O --root $F/hub-copy search idempotency
{"total":2,... results: [
  {"uri":"okf://hub-copy/repos/billing-api/gotchas/idempotency-key","type":"Gotcha",
   "signals":{"status":"draft","freshness":"invalid","trustTier":"unverified","generated":{"by":"cursor/gpt-5.6",...}}, "score":33.8,"snippet":"…"},
  {"uri":"okf://hub-copy/repos/billing-api/services/billing-api","signals":{"trustTier":"human-reviewed",...},"score":3.47}]}
$ $O --root $F/workspace/hub search idempotency   → {"total":0}
```
- Machine-readable: CLI always emits JSON (full detail incl. `signals`); MCP `search_concepts` defaults to compact `{uri,title,type,description}` with `detail:"full"` opt-in.
- Filters (CLI): `--status draft` → idempotency-key; `--trust-tier human-reviewed` → billing-api, invoice-dependency, session-token; `search "" --status deprecated` → legacy-api-key; `--freshness stale` → 0 (due to T2 date bug); `--freshness invalid` → the two spec-formatted concepts. MCP adds types/tags/pathPrefix/linkedTo/linkedFrom/relationType/generatedBy/verifiedBy.
- **Recall bug for code identifiers:** `contracts/invoice-api` contains ``Requires `Idempotency-Key` header`` but is missed by `idempotency`, `Idempotency-Key`, and `idempotency key`. Searching ``'`idempotency'`` finds it. Cause: MiniSearch default tokenizer splits on `\p{Z}\p{P}`; backtick is `\p{Sk}`, so the indexed token is `` `idempotency``. Raw body is indexed (`src/search-index.js`). Prefix/fuzzy/stemming deliberately disabled. For coding knowledge full of backticked identifiers this is a serious recall gap.
- Progressive disclosure: no `index.md` generation or synthesis command (`generate` only runs filesystem/json-spec generator plugins that create concepts). `graph_summary`/`--inspect` give counts by type/tag/status/trust — a usable orientation step for agents.
- `provenance okf://billing-api/services/billing-api` → trust signals + source traversal (externals only with `--include-external`); never fetches.

## T7 Agent write path — PARTIAL

No CLI write command for concepts (CLI writes only via `generate` plugins and `producer run --write --actor`). Concept writes go through MCP: `--authoring` (proposal → accept; not exercised) or `--write --actor <actor>` exposing `okf_validate_changes` / `okf_apply_changes`. Driven over stdio against copy `t7/okf` (git repo; base commit added unknown keys `x_owner: payments   # comment` and flow-map `x_meta: { tier: 1, oncall: [alice, bob] }` to services/billing-api.md):

```
$ python3 mcp_drive.py t7/c1.json -- $O --root t7/okf --write --actor claude-code/opus-5 mcp
# call okf_validate_changes isError=None          (dry-run preview, no write)
# call okf_apply_changes isError=None  create Gotcha "Invoice retries need backoff" (metadata {status: draft, x_owner})
  "status":"applied","generated":{"by":"claude-code/opus-5","at":"2026-09-29T11:35:00.324Z"},
  "path":"gotcha/invoice-retries-need-backoff.md", "durability":{"state":"filesystem_published_uncommitted"}
# call okf_apply_changes isError=None  update services/billing-api: tags.add [payments], metadata.verified {by: human:carol, at: …}
# call okf_apply_changes isError=True  update gotchas/idempotency-key metadata {verified, status: stable}
  "status":"rejected" … {"path":"gotchas/idempotency-key.md","code":"invalid_stale_after","severity":"error"}
# call okf_apply_changes isError=True  metadata.generated override
  "changes[].metadata.generated is server-managed; use its dedicated field instead."
```
Resulting diff (`git diff`, services/billing-api.md):
```
-tags: [billing, service]
+tags:
+  - billing
+  - service
+  - payments
 status: stable
-x_owner: payments   # custom unknown key with comment
-x_meta: { tier: 1, oncall: [alice, bob] }
-generated: { by: claude-code/opus-5, at: 2026-09-20T10:00:00Z }
-verified: { by: human:alice, at: 2026-09-21T09:00:00Z }
+x_owner: payments
+x_meta:
+  tier: 1
+  oncall:
+    - alice
+    - bob
+generated:
+  by: claude-code/opus-5
+  at: 2026-09-29T11:35:00.336Z
+verified:
+  by: human:carol
+  at: 2026-09-29T09:00:00Z
```
Body (incl. `[^main-go]` footnote) and `sources` untouched.
- `generated` stamp: yes, server-managed, whole batch shares one `at` (millisecond precision); every update restamps `generated.by` to the server actor.
- Unknown keys: values preserved, **YAML comments dropped and flow style rewritten to block style** → a 1-tag change produced a 17-line frontmatter diff. Reviewable, but noisy.
- `verified: human:<id>`: settable via `metadata` by any MCP caller — the agent can stamp human verification itself (no separate human-only path; rely on diff review). `verified` replaced wholesale (not appended as list).
- **Cannot touch the planted stale concept**: authoring is strict and the spec-conformant `stale_after` datetime is an *error* on write, so an agent can't verify/update it without rewriting `stale_after` to a non-spec date-only value.
- Default path derivation put the new Gotcha in a new `gotcha/` dir despite existing `gotchas/` (1 same-type concept isn't a "dominant convention"); pass `path` explicitly.
- index.md not regenerated (`git status`: index.md unchanged, new concept unlisted); log.md not appended. Neither is supported.
- `--git-commit` (copy `t7b`, clean repo): `okf_apply_changes` with `message` → `git log`: `0946bb7 docs(okf): add refresh-token concept — okf/concepts/refresh-token.md | 14 +`, worktree clean afterwards. One commit per batch, never pushes.

## T8 Move/rename — FAIL (unsupported by design)

README: "Paths and URIs are immutable during update: moving a concept changes its portable identity and remains a separate, intentionally unsupported operation." No CLI/MCP move. Manual `mv repos/billing-api/contracts/invoice-api.md …/invoice-api-v2.md` in a hub-copy copy (`t8/hub`), then `$O --root t8/hub validate --strict-links` → exit 1 with `broken_link` from cross-repo/invoice-dependency.md, repos/billing-api/services/billing-api.md, repos/web-app/services/web-app.md (all 3 inbound). Detection yes, rewrite no.

## T9 Multi-bundle without hub — PARTIAL (best-in-class federation, non-standard links)

`/tmp/okf-research/okf-mcp/t9/okf.project.yaml` (okf-mcp extension, not OKF):
```yaml
project: okf-eval-t9
strictLinks: false
bundles:
  - { id: billing-api, root: workspace/billing-api/okf }
  - { id: web-app,     root: workspace/web-app/okf }
  - { id: shared-auth, root: workspace/shared-auth/okf }
  - { id: hub,         root: workspace/hub }
```
Constraint: roots must be **relative, inside the yaml's directory, and not traverse symlinks**, so the bundles were copied into `t9/workspace/` (`cp -a`). An absolute-path yaml (`t10/okf.project.abs.yaml`) → `invalid_project_path … must be a relative path inside the project root`, 0 concepts loaded, exit 1. The CLI flag form `--bundle id=/abs/path` (repeatable) does accept absolute paths — that's the workable federation for repos outside one tree. Without `id=`, every per-repo bundle defaults to id `okf` → `duplicate_bundle_id … later bundle was ignored` while `--inspect` still exits 0.

Baseline (unmodified copies):
```
$ $O --project okf.project.yaml --inspect → concepts 9, byBundle {billing-api:3, web-app:2, shared-auth:2, hub:2}
$ $O --project okf.project.yaml validate   → exit 1 (web-app link_outside_root ×2 invalidate project; hub /repos/... broken ×3)
$ … search invoices → okf://billing-api/contracts/invoice-api, okf://web-app/services/web-app, okf://hub/cross-repo/invoice-dependency, okf://billing-api/services/billing-api, okf://billing-api/gotchas/idempotency-key
$ … search auth     → okf://shared-auth/concepts/legacy-api-key, okf://shared-auth/concepts/session-token, okf://web-app/services/web-app
$ … neighbors okf://billing-api/contracts/invoice-api → inbound only from billing-api (no cross-bundle edges yet)
$ … concept services/web-app → okf://web-app/services/web-app   (bare id resolves when unique)
$ $O --bundle billing-api=… --bundle web-app=$F/workspace/web-app/okf --bundle shared-auth=… search invoices → spans 3 bundles
```
With cross-bundle links added to copies (web-app.md: `relations: [{type: depends_on, target: okf://billing-api/contracts/invoice-api}, {type: consumes, target: okf://shared-auth/concepts/session-token}]` + body link `[…](okf://billing-api/contracts/invoice-api)`; hub invoice-dependency.md: `relations` depends_on/related_to incl. one broken `okf://web-app/flows/does-not-exist`, + body links `okf://…/invoice-api` and `okf://…/nope`):
```
$ … validate → error broken_relation hub cross-repo/invoice-dependency.md target okf://web-app/flows/does-not-exist (invalidatesProject)
$ … neighbors okf://billing-api/contracts/invoice-api
  okf://billing-api/gotchas/idempotency-key  markdown_link
  okf://billing-api/services/billing-api     markdown_link
  okf://web-app/services/web-app             relation depends_on
  okf://hub/cross-repo/invoice-dependency    relation depends_on
$ … paths okf://hub/cross-repo/invoice-dependency okf://shared-auth/concepts/session-token
  [[invoice-dependency, okf://web-app/services/web-app, okf://shared-auth/concepts/session-token]]
$ … concept okf://hub/cross-repo/invoice-dependency | jq .links → only the 3 /repos/... links; okf:// body links absent
```
- Typed `relations` + `okf://<bundle>/<id>` targets: resolve across bundles, feed backlinks/paths, broken targets are project errors. **Non-standard** (okf-mcp extension; plain OKF readers see an unknown key).
- `okf://` in Markdown body links: **silently ignored** (not links, not edges, broken ones not reported).
- Relative `../../<repo>/…` links never resolve across federated bundles (`link_outside_root`).
- `graph mermaid` works across bundles (unlabeled edges, no per-bundle grouping).

## T10 Scale — PARTIAL

Correctness (hub-copy, 10,051 docs):
```
$ $O --root $S/hub-copy --inspect → {"documents":10051,"concepts":10000,"reserved":51,"edges":31016,"brokenLinks":0}
$ $O --root $S/hub-copy validate  → exit 1; conformant false: reserved_index_frontmatter_not_allowed ×50, reserved_index_missing_link ×51 (heading-only index.md files)
$ $O --root $S/hub-copy search zebracorn-needle → {"total":1,"r":["repos/r37/d3/c123"]}   ✔ single correct hit
$ $O --root $S/hub-copy neighbors repos/r37/d3/c123 → in: r37/d1/c091, r37/d4/c094  (matches rg ground truth)
$ $O --root $S/hub …           → documents 1, concepts 0; search needle → 0  ✘ (symlinks)
50-bundle project (copies in t10/proj, relative roots): concepts 10000; validate codes {link_outside_root:1016, reserved_index_missing_link:50};
  search needle → okf://r37/d3/c123 ✔; neighbors okf://r37/d3/c123 → okf://r37/d1/c091, okf://r37/d4/c094 ✔
```
Timings (bench.sh: 1 warmup + 5 runs, `/usr/bin/time`):
```
hub-copy validate                         min=1.78s median=1.80s maxRSS=170MB
hub-copy search zebracorn-needle          min=2.80s median=2.91s maxRSS=227MB
hub-copy neighbors repos/r37/d3/c123      min=1.86s median=1.87s maxRSS=170MB
hub-copy --inspect                        min=1.87s median=1.89s maxRSS=173MB
hub(symlink) validate [sees 0 concepts]   min=0.26s median=0.27s
hub(symlink) search needle [0 hits]       min=0.27s median=0.28s
project(50) validate                      min=1.90s median=1.95s maxRSS=170MB
project(50) search zebracorn-needle       min=2.92s median=2.93s maxRSS=225MB
project(50) neighbors okf://r37/d3/c123   min=1.81s median=1.83s maxRSS=167MB
--bundle x50 search zebracorn-needle      min=2.88s median=2.93s maxRSS=224MB
--bundle x50 validate                     min=1.87s median=1.88s maxRSS=168MB
loop 50x --root rNN/okf validate          min=18.00s median=18.38s
loop 50x --root rNN/okf search needle     min=19.70s median=20.29s
single bundle r37 search needle           min=0.37s median=0.37s
single bundle r37 validate                min=0.34s median=0.35s
okf --version (node startup floor)        min=0.24s median=0.25s
```
Warm stdio MCP server on `$S/hub-copy` (`mcp_warm.py`, 1 first + 5 warm calls):
```
startup+initialize: 1.86s
search_concepts zebracorn-needle: first=0.997s (builds MiniSearch index) warm min=0.013s median=0.014s
get_neighbors:                     first=0.008s warm min=0.001s median=0.002s
validate_bundle:                   first=0.004s warm min=0.001s median=0.002s
```
- Targets: validate < 10 s ✔ (1.8 s); backlinks < 2 s ✔ (1.87 s cold, borderline); search < 2 s ✘ cold (2.9 s: full parse + index build each process) / ✔ warm MCP (14 ms).
- Index generation time: N-A — no index.md generation command.
- No file watcher: a long-lived MCP server must be restarted after external edits (own writes reindex).

## Strengths
1. Real federation: `okf.project.yaml` or repeated `--bundle id=path` gives one graph/search across 50 bundles with `okf://bundle/id` locators; typed `relations` resolve cross-bundle and feed backlinks/paths; broken relation targets are errors.
2. Agent-grade read surface: every CLI command emits JSON with normalized trust tier / status / freshness / generated signals; rich filters; graph tools (neighbors, paths, provenance, mermaid/dot/json); separate `conformant` vs `validForProject` with correct §8 nested-index and §11 checks.
3. Safe, one-command, offline install (npm, 20 MB, MIT, no telemetry, 0 sockets observed); careful write path (atomic validated batches, revision checks, server-stamped `generated`, optional one-commit-per-batch with clean-tree gate); warm MCP queries in ms at 10k concepts.

## Blockers (for the hybrid layout)
1. **Spec drift on `stale_after`**: the SPEC's own datetime form is flagged invalid → staleness never computed, and authoring refuses to update any concept carrying it (can't verify the stale gotcha). Must fix upstream or use non-spec date-only values.
2. **Symlinked hub unsupported** (walker ignores symlinks; project roots reject them); cross-repo links in the hub view require copies, and in federation require non-standard `relations`/`okf://` (body `okf://` links ignored, `../../repo` links rejected).
3. **Agent ergonomics gaps**: no CLI write (MCP required for authoring — conflicts with "CLI + skill, no MCP requirement"); no index.md/log.md maintenance; no move/rename; frontmatter rewrite loses comments/flow style (noisy diffs); backticked identifiers not searchable by plain term; cold CLI search 2.9 s at 10k.

## Surprises
- `--inspect` / CLI exit 0 when a duplicate bundle id silently drops a bundle (default id = dirname `okf` for every `<repo>/okf`).
- Directory links in index.md (`[X](dir/)`, the §8 example form) warn as broken when the dir has no index.md.
- Heading-only `index.md` is a conformance *error* (`reserved_index_missing_link`).
- Any caller with `--write` can stamp `verified: {by: human:…}` via `metadata`.
- Single maintainer, repo 3 months old, frequent minor releases (0.3 → 0.9.1 since June 2026).

## Fit for the hybrid layout
Moderate. Viable as the **read/query layer** if the hub is built by federation (`--bundle id=path` per repo + hub bundle) instead of symlinks, and cross-repo edges are written as `relations:` with `okf://<repo>/<id>` (accepting a non-standard extension), or if hubs are assembled by copying. Not viable as-is for the **write/curation** half of the design: stale detection is broken on spec-formatted data, writes need MCP, and index/log/move maintenance is missing — those would need a companion CLI or upstream fixes.

## Artifacts
`/tmp/okf-research/okf-mcp/`: `bench.sh`, `mcp_drive.py`, `mcp_warm.py`, `t7/` (git repo + `c1.json` calls + `c1.out` receipts), `t7b/` (git-commit test), `t8/`, `t9/okf.project.yaml` (+ modified copies), `t10/okf.project.abs.yaml`, `t10/proj/okf.project.yaml`, `t2s/` (date-only stale test).
