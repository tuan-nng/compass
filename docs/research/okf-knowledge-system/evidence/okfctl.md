# okfctl v0.4.0 — OKF v0.2 tooling evaluation

- Tool: okfctl (https://github.com/cwest/okfctl, https://okfctl.dev), tag **v0.4.0** (latest tag on 2026-09-29; `git ls-remote --tags` lists v0.1.0 … v0.4.0).
- Work dir: `/tmp/okf-research/okfctl/` (`bin/`, `home/` = `$HOME`, `gopath/`, `gocache/`, `src/` = source clone at v0.4.0 for code inspection only).
- Env for all runs: `HOME=/tmp/okf-research/okfctl/home PATH=/tmp/okf-research/okfctl/bin:$PATH`.
- All mutating tests ran on `cp -a` copies (`fx/` = copy of `fixture/`, plus `t7/ t8/ t9/ t10/`). Fixture checksum before/after: `f9a6b205…` = `f9a6b205…` (unchanged).
- Timing method (hyperfine not installed): `bench.sh` = 1 warmup + 5 runs of `/usr/bin/time -f '%e %M'` (10 ms resolution), reporting min/median wall seconds and max RSS. Stdout of the warmup run is kept in `bench-out/` for correctness checks.

## T1 Install — PASS (with caveats)

```
$ GOBIN=$PWD/bin GOPATH=$PWD/gopath GOCACHE=$PWD/gocache HOME=$PWD/home go install github.com/cwest/okfctl@v0.4.0
go: github.com/cwest/okfctl@v0.4.0 requires go >= 1.26.6; switching to go1.26.8
go: downloading go1.26.8 (linux/amd64)
go: downloading github.com/spf13/cobra v1.10.2 … goldmark-meta v1.1.0, yaml.v3 v3.0.1, pflag v1.0.9, yaml.v2 v2.3.0, goldmark v1.8.4
Elapsed (wall clock) time: 1:17.43
$ go install github.com/cwest/okfctl/cmd/okfctl-search@v0.4.0     # plugin is a 2nd main package in the same module
real 0m2.828s
$ ls -la bin
-rwxrwxr-x 15182564 okfctl
-rwxrwxr-x  5298483 okfctl-search
$ okfctl version
okfctl v0.4.0 (commit none, built unknown)
```

- Wall time 77 s, dominated by Go auto-downloading toolchain go1.26.8 (local go is 1.26.0; go.mod requires ≥1.26.6; 235 MB toolchain in module cache). `okfctl-search` then built in 2.8 s from cache.
- Size: okfctl 15.2 MB, okfctl-search 5.3 MB (static Go binaries). Direct deps: cobra, pflag, goldmark, goldmark-meta, yaml.v2/v3. License **Apache-2.0** (LICENSE, plugin.json; source headers say "Copyright 2026 Google LLC").
- Runtime deps: none required; optional `git` binary (validate drift check runs `git log` per node that carries `modified`; `connect` shells out to `git clone`/`git pull --ff-only`).
- Network/telemetry at runtime: `strace -f -e trace=network` over `validate`, `search`, `lint`, `analyze`, `index build`, `node new`, `okfctl-search --semantic` → **0** socket/connect syscalls each. Source grep: no telemetry/analytics code; `net/http` only in `serve` and the separate `okfctl-api` plugin (loopback-only). Only files written to `$HOME`: `~/.config/okfctl/config.json` (registry) — the `~/.config/go/telemetry/local/*` counters come from the Go toolchain during `go install` (local mode, not okfctl).
- Official install paths are `brew install cwest/tap/okfctl` or a curl one-liner (not used, per protocol). `go install` works but needs a ≥1.26.6 toolchain.
- Plugin discovery: `okfctl plugin list` → `okfctl-search	/tmp/okf-research/okfctl/bin/okfctl-search`.

## T2 Validate — PARTIAL

```
$ for b in workspace/billing-api/okf workspace/web-app/okf workspace/shared-auth/okf workspace/hub hub-copy; do okfctl validate $b; echo exit=$?; done
===== workspace/billing-api/okf   OK: bundle conforms to the OKF spec floor   exit=0 (strict exit=0)
===== workspace/web-app/okf       OK: …  exit=0 (strict 0)
===== workspace/shared-auth/okf   OK: …  exit=0 (strict 0)
===== workspace/hub               OK: …  exit=0 (strict 0)    ← symlinked repos never walked
===== hub-copy
FAIL repos/billing-api/index.md: index files contain no frontmatter (§8); frontmatter is permitted only on the bundle-root index and only for okf_version (§12)
FAIL repos/shared-auth/index.md: …
FAIL repos/web-app/index.md: …
okfctl: 3 conformance finding(s)
exit=1 (strict exit=1)
```

`validate` = spec floor only (type present, parseable frontmatter, reserved-file structure) + git `modified` drift. Everything else is split across `lint` (CI gate with `--strict`), `analyze` (report, always exit 0) and `eval transparency`:

```
$ okfctl lint <b>; okfctl lint --strict <b>
billing-api/okf: OK: no lint findings (exit 0 / strict 0)
web-app/okf:     OK: no lint findings (exit 0 / strict 0)      ← the 2 hub-relative links are NOT reported here
shared-auth/okf: orphan: concepts/legacy-api-key.md has no inbound links … (exit 0 / strict 1)
workspace/hub:   orphan: decisions/adr-001-okf-layout.md … (strict 1)
hub-copy:        orphan: decisions/adr-001-okf-layout.md …
                 broken-link: repos/billing-api/gotchas/idempotency-key.md links to /contracts/invoice-api.md which resolves to no node; did you mean repos/billing-api/contracts/invoice-api.md?
                 orphan: repos/shared-auth/concepts/legacy-api-key.md …   (exit 0 / strict 1)
$ okfctl analyze workspace/web-app/okf
## Coverage gaps — dangling forward-links (research opportunities)
  - services/web-app.md → missing: ../../billing-api/contracts/invoice-api.md
  - services/web-app.md → missing: ../../shared-auth/concepts/session-token.md
$ okfctl analyze hub-copy
## Freshness — stale / undated nodes (revalidate)
  - repos/shared-auth/concepts/legacy-api-key.md (age: 627d, basis: 2025-01-10)
$ okfctl eval transparency workspace/billing-api/okf
grade-missing: contracts/invoice-api.md carries no epistemic or authority grade (provenance is invisible)   (×3, all nodes)
```

Planted checks:

| Planted item | Caught? |
|---|---|
| stale concept (`stale_after: 2026-09-01` on idempotency-key) | **No.** `analyze` freshness is age-based (`--stale-days 180` over verified/generated/modified/created); idempotency-key (generated 2026-06-01, 120 d) not flagged. `Node.IsStale()` exists in `internal/okf/provenance.go` but has no caller outside tests. |
| broken links | Partly. `lint` reports a broken link only when a same-basename node exists elsewhere (hub-copy idempotency-key). Standalone web-app hub-relative links appear only in `analyze` as "dangling forward-links" (advisory). Symlinked hub: `analyze` lists the 3 `/repos/...` links from cross-repo as dangling. |
| nested index frontmatter (hub) | **Yes** — hard FAIL, exit 1 in hub-copy (and scale/hub-copy: 50 FAILs). Not seen in symlinked hub (not walked). |
| footnote ↔ `sources[].id` join | **No.** Negative test: renamed `[^main-go]` → `[^nope]` in a copy; `validate` OK, `lint` no footnote finding, `eval transparency` only grade-missing. |
| bonus: `status: verified` (off-enum) | `lint`: `status-lifecycle: … not in the §5.4 lifecycle enum (draft|stable|deprecated)`. |

Also: `bundle info` reports `okf_version: 0.2` even when the bundle-root `index.md` declares `okf_version: "0.1"` — okfctl only reads a non-spec `.okf` sidecar and falls back to its build version.

## T3 Symlinks — FAIL

```
$ okfctl node list --bundle workspace/hub           $ okfctl node list --bundle hub-copy
cross-repo/invoice-dependency.md  Cross-Repo Dep.    cross-repo/invoice-dependency.md …
decisions/adr-001-okf-layout.md   Decision           decisions/adr-001-okf-layout.md …
(2 nodes)                                            repos/billing-api/contracts/invoice-api.md … (9 nodes)
$ okfctl bundle info workspace/hub → nodes: 2 reserved: 1     hub-copy → nodes: 9 reserved: 4
$ okfctl node list --bundle workspace/hub/repos/billing-api    → (empty, exit 0)
$ okfctl node list --bundle workspace/hub/repos/billing-api/   → 3 nodes (trailing slash makes Lstat follow)
```
Cause (source): `internal/okf/bundle.go` `Load` uses `filepath.WalkDir`, which never follows symlinks; no flag to change it. Symlinked hub sees 2/9 concepts.

## T4 Link resolution — PARTIAL (resolver correct; symlink view blind)

From `graph export` edges / `analyze` dangling lists:

| Link form | standalone repo | symlinked hub | hub-copy |
|---|---|---|---|
| relative `../contracts/invoice-api.md` (billing-api service) | resolves | not walked | resolves |
| root-relative `/contracts/invoice-api.md` (idempotency-key) | resolves (edge `gotchas/idempotency-key.md -> contracts/invoice-api.md`) | not walked | **broken** (lint broken-link + analyze dangling) — matches expectation |
| hub-relative `../../billing-api/…`, `../../shared-auth/…` (web-app) | **broken** (analyze dangling) — expected | not walked | resolves (edges to `repos/billing-api/contracts/invoice-api.md`, `repos/shared-auth/concepts/session-token.md`) |
| URL `https://github.com/…` | external, ignored (no edge, no finding) | same | same |
| hub `/repos/billing-api/…`, `/repos/web-app/…` (cross-repo) | n/a | **broken** (targets invisible) | resolves (3 edges) |
| `/cross-repo/invoice-dependency.md` (ADR) | n/a | resolves | resolves |

## T5 Backlinks across repos — PASS on hub-copy / FAIL on symlinked hub

No dedicated backlinks command; `search --neighbors` is undirected; true inbound = `graph export` JSON filtered on `to`.
```
$ okfctl search --neighbors repos/billing-api/contracts/invoice-api.md hub-copy
cross-repo/invoice-dependency.md         Cross-Repo Dependency depth=1
repos/billing-api/services/billing-api.md Service      depth=1
repos/web-app/services/web-app.md        Service      depth=1
$ okfctl graph export hub-copy | python3 -c '…edges where to==repos/billing-api/contracts/invoice-api.md'
cross-repo/invoice-dependency.md
repos/billing-api/services/billing-api.md
repos/web-app/services/web-app.md
$ okfctl search --neighbors repos/billing-api/contracts/invoice-api.md workspace/hub
okfctl: node not found: repos/billing-api/contracts/invoice-api.md    (exit 1)
```
Matches the expected 3 inbound (idempotency-key's link correctly absent). Caveat: `--neighbors` mixes in/out edges (here coincidentally equal since the service is linked both ways).

## T6 Search / retrieval for agents — PARTIAL

```
$ okfctl search idempotency hub-copy
repos/billing-api/contracts/invoice-api.md API Contract [body]
repos/billing-api/gotchas/idempotency-key.md Gotcha       [body,title]
repos/billing-api/services/billing-api.md Service      [body]
$ okfctl search --json idempotency hub-copy
[ {"path": "repos/billing-api/contracts/invoice-api.md", "title": "Invoice API v2", "type": "API Contract", "neighborhood": "repos", "matched_on": ["body"]}, … ]
$ okfctl search idempotency workspace/hub     → (no output, exit 0)
$ okfctl node show repos/billing-api/gotchas/idempotency-key --bundle hub-copy
path: repos/billing-api/gotchas/idempotency-key.md
type: Gotcha
<body only — status/generated/stale_after not shown>
```
- JSON: yes (`search`, `lint`, `analyze`, `graph export`, `eval`). Fields: path/title/type/neighborhood/matched_on only — **no status, trust tier (generated/verified), staleness, or description**; `node show` prints type + body only (agent must `cat` the file for provenance). Filters: `--field any|title|tag|type|body`; no status/type filter in core search (the plugin has `--type/--tag/--path` filters).
- Progressive disclosure: `index build` writes per-directory `index.md` with `* [Title](file.md) - description` entries (spec §8 shape) — see T7.
- Semantic plugin (brief, `--embedder hash` default, "hash-test-embedder" dim 64): `okfctl-search index build sem-hub` → `indexed 9 node(s) … -> sem-hub/.okfctl/index.db` (writes a `.okfctl/` dir inside the bundle; later core commands print `note: skipped 1 vendored/derived directory (.okfctl)`). `okfctl-search --semantic "retry double charge"` ranked session-token (0.218) first and idempotency-key at 0.0000 — hash embedder is a toy. `okfctl search --semantic …` → `okfctl: unknown flag: --semantic` (docs say the plugin is reachable via `okfctl search` dispatch, but the core builtin `search` shadows it). `okfctl lint --semantic` works but two findings print without a node path (`no semantically close node (best neighbor 0.09, below 0.20) — dead concept, or missing context?`). model2vec embedder needs a locally downloaded model — [not run]: requires network download.

## T7 Agent write path — PARTIAL

Run in a git-initialised copy of billing-api (`t7/billing-api`).
```
$ okfctl node new gotchas/rate-limit --type Gotcha --title "Invoice API rate limit" --bundle okf
Created okf/gotchas/rate-limit.md
$ git status --short
 M okf/index.md
?? okf/contracts/index.md  ?? okf/gotchas/index.md  ?? okf/services/index.md
?? okf/gotchas/rate-limit.md  ?? okf/log.md
$ cat okf/gotchas/rate-limit.md
---
type: Gotcha
title: Invoice API rate limit
created: 2026-09-29T11:35:30Z
modified: 2026-09-29T11:35:30Z
---
# Invoice API rate limit
$ git diff okf/index.md
 okf_version: "0.2"                      (root frontmatter kept)
-# Billing API knowledge
+# Knowledge Base
-* [Services](services/) - Deployable services in this repo      (hand-written descriptions lost)
+## Subdirectories
+* [Contracts](contracts/)
```
- `node new` has only `--type/--title` — no flag for `description`, `generated`, `verified`, tags. Stamps non-spec `created`/`modified`.
- Adding provenance: `node edit` is `$EDITOR`-driven, but scriptable via `OKFCTL_EDITOR=<script>` (non-interactive) → re-validates, bumps `modified`, appends log, rebuilds index:
```
$ EXTRA=… OKFCTL_EDITOR=add-prov.py okfctl node edit gotchas/rate-limit --bundle okf
gotchas/rate-limit.md edited; bundle valid
+description: POST /v2/invoices is limited to 100 req/min per tenant.
+generated: {by: claude-code/opus-5, at: '2026-09-29T10:00:00Z'}
+x_owner: team-billing
+x_nested: {a: 1, b: [x, y]}
-modified: 2026-09-29T11:35:30Z
+modified: 2026-09-29T11:35:42Z
```
- Unknown keys (`x_owner`, `x_nested`, `x_reviewed_note`) **preserved**, key order preserved. But `node edit` on an existing fixture node **re-serialises the YAML**, adding noise to untouched lines:
```
-generated: { by: claude-code/opus-5, at: 2026-09-20T10:00:00Z }
-verified: { by: human:alice, at: 2026-09-21T09:00:00Z }
+generated: {by: claude-code/opus-5, at: '2026-09-20T10:00:00Z'}
+verified: {by: 'human:alice', at: '2026-09-21T09:00:00Z'}
+modified: 2026-09-29T11:35:49Z
```
- Direct file edit (how an agent would add `verified: { by: human:novpla, … }`) + `okfctl index build okf` + `okfctl log append --message "human:novpla verified gotchas/rate-limit.md" okf` → minimal 2-file diff (1 frontmatter line + 1 log line); `index check` exit 0.
- log.md format: `- 2026-09-29 — edited gotchas/rate-limit.md` flat bullet list, newest first. Spec §9 wants `## YYYY-MM-DD` date headings; `validate` doesn't flag this.
- `node new` / first `index build` created one `index.md` per directory (3 new files in a 3-dir bundle, 552 in scale hub). First-touch diff is noisy; later diffs are small.
- **Data-loss bug:** `bundle init` help says "It refuses to overwrite an existing bundle", but it overwrites. `okfctl bundle init x/okf` on a copy of web-app/okf → exit 0, `index.md` replaced by the stub (okf_version frontmatter and entries lost); `.okf` + `log.md` added. Source `internal/okf/reserved.go Scaffold` calls `os.WriteFile` with no existence check.
- Git drift: `validate` compares `modified` to `git log` per node → see T10 cost.

## T8 Move/rename — PASS in hub-copy (the highlight); FAIL across symlinked/standalone repos

hub-copy copy (git-initialised, `t8/hub-copy`):
```
$ okfctl node mv repos/billing-api/contracts/invoice-api repos/billing-api/contracts/invoice-api-v2 --bundle . --dry-run
move repos/billing-api/contracts/invoice-api.md -> repos/billing-api/contracts/invoice-api-v2.md
  rewrite cross-repo/invoice-dependency.md: /repos/billing-api/contracts/invoice-api.md -> /repos/billing-api/contracts/invoice-api-v2.md
  rewrite repos/billing-api/services/billing-api.md: ../contracts/invoice-api.md -> ../contracts/invoice-api-v2.md
  rewrite repos/web-app/services/web-app.md: ../../billing-api/contracts/invoice-api.md -> ../../billing-api/contracts/invoice-api-v2.md
$ okfctl node mv … (real)
Moved … (3 inbound link(s) rewritten)
$ git add -A && git diff --cached -M --stat
 .../contracts/{invoice-api.md => invoice-api-v2.md} | 0      ← moved file byte-identical
 cross-repo/invoice-dependency.md | 2 +-
 repos/billing-api/services/billing-api.md | 2 +-
 repos/web-app/services/web-app.md | 2 +-
 + index.md/log.md churn: 18 files changed, 72 insertions(+), 23 deletions(-) (11 new index.md, 4 rewritten, new log.md)
```
- All 3 inbound forms rewritten, each preserving its own style (root-relative stays root-relative, relative stays relative). Broken `/contracts/invoice-api.md` in idempotency-key and the external URL correctly left alone.
- Side effect: index regeneration strips `okf_version` frontmatter and hand-written text from `repos/*/index.md` (`-okf_version: "0.2"`, `+# Billing-api`). Syncing hub-copy changes back into repos would drop each repo's version declaration.
- Scale (10k-node copy): `node mv repos/r37/d3/c123 …-renamed` → `2 inbound link(s) rewritten`, 1.37 s; grep confirms exactly c091 and c094 updated.

Copy of the symlinked workspace (`t8/ws`, symlinks intact):
```
$ okfctl node mv repos/billing-api/contracts/invoice-api repos/billing-api/contracts/invoice-api-v2 --bundle hub
okfctl: node not found: repos/billing-api/contracts/invoice-api.md   (exit 1)
$ okfctl node mv cross-repo/invoice-dependency cross-repo/web-app-invoice-dependency --bundle hub --dry-run
  rewrite decisions/adr-001-okf-layout.md: /cross-repo/invoice-dependency.md -> …     (hub-local only)
$ okfctl node mv contracts/invoice-api contracts/invoice-api-v2 --bundle billing-api/okf
Moved … (2 inbound link(s) rewritten)
$ grep -rn invoice-api web-app/okf hub/cross-repo
web-app/okf/services/web-app.md:13: …(../../billing-api/contracts/invoice-api.md)      ← now dangling
hub/cross-repo/invoice-dependency.md:15: …(/repos/billing-api/contracts/invoice-api.md) ← now dangling
```

## T9 Multi-bundle without hub — FAIL (no native federation)

- Every command takes exactly one bundle dir: `okfctl search idempotency billing-api/okf web-app/okf` → `okfctl: accepts at most 2 arg(s), received 3`. No workspace/project file or multi-root.
- `registry` = named git URLs in `~/.config/okfctl/config.json`; `connect` = `git clone` / `git pull --ff-only` into a directory. Test with 3 local bare repos:
```
$ okfctl registry add billing-api file:///…/remotes/billing-api.git   (×3)
$ okfctl registry list
billing-api	file:///tmp/okf-research/okfctl/t9/remotes/billing-api.git …
$ okfctl connect billing-api hub-fed/repos/billing-api   (×3)
Connected file:///…/billing-api.git -> hub-fed/repos/billing-api
$ okfctl node list --bundle hub-fed
repos/billing-api/okf/contracts/invoice-api.md API Contract     ← whole repo cloned; extra /okf/ segment
note: skipped 3 vendored/derived directories (repos/billing-api/.git, …)
$ okfctl search --neighbors repos/billing-api/okf/contracts/invoice-api.md hub-fed
repos/billing-api/okf/services/billing-api.md Service depth=1      ← web-app & cross-repo links broken by the /okf/ segment
$ okfctl connect billing-api hub-fed/repos/billing-api
Updated hub-fed/repos/billing-api from … (fast-forward re-sync works)
```
- `connect` can assemble a hub-copy-style checkout (real dirs, not symlinks, so okfctl sees them) but has no subdirectory/sparse option. Hub-relative links only resolve if each repo's bundle sits at the repo root or the hub layout is `repos/<name>/okf/…` (and the links are written to match). Cross-bundle links can only be ordinary relative/root-relative paths inside one assembled tree; there is no bundle-qualified link form.

## T10 Scale — PARTIAL (read paths fast; lint/analyze don't scale; symlink hub unusable)

Method: `bench.sh` (1 warmup + 5 × `/usr/bin/time -f '%e %M'`), min / median wall seconds.

| Operation | Target | scale/hub-copy (10,000 nodes) | scale/hub (symlinks) |
|---|---|---|---|
| `validate` | < 10 s | **0.37 / 0.38 s**, exit 1 (50 FAILs: nested `repos/rNN/index.md` frontmatter), 69 MB | 0.00 / 0.00 s, "OK" but 0 repo nodes walked |
| `search zebracorn-needle` | < 2 s | **0.41 / 0.43 s**; 1 hit `repos/r37/d3/c123.md [body]` ✔ | 0.00 s, **0 hits** ✘ |
| `search --json zebracorn-needle` | < 2 s | 0.39 / 0.40 s, 1 hit ✔ | — |
| backlinks: `search --neighbors repos/r37/d3/c123.md` (undirected) | < 2 s | **0.37 / 0.38 s**, 5 neighbors (c040, c091, c094, c149, c177) | exit 1 `node not found` |
| backlinks exact: `graph export` + python filter on `to` | < 2 s | **0.58 / 0.58 s** → `['repos/r37/d1/c091.md', 'repos/r37/d4/c094.md']` (= grep ground truth) | — |
| `graph export` (31,016 edges, 998 cross-repo) | — | 0.47 / 0.48 s, 104 MB | — |
| `index build` (cp -a copy) | — | first build on fresh copy 0.98 s (552 index.md written); rebuild 0.97 / 0.97 s; afterwards `validate` OK (nested frontmatter stripped) | — |
| `index check` | — | 0.37 / 0.38 s (exit 1: out of date) | — |
| `node list` | — | 0.42 / 0.52 s | — |
| `lint` | — | **98.30 / 100.06 s** (468 findings) | — |
| `analyze --json` | — | 7.42 / 8.18 s, **2.15 GB RSS**, 19,090,215 lines of JSON | — |
| `node mv` (one run) | — | 1.37 s, 2 links rewritten | — |
| `okfctl-search index build` (hash, one run) | — | 1.48 s, 392 MB RSS, 47 MB `.okfctl/index.db` | — |
| `okfctl-search --semantic zebracorn-needle` | — | 0.42 / 0.44 s; needle **not** in top 3 (hash embedder); with `--lexical-gate` → only `repos/r37/d3/c123.md` | — |

Scaling of `lint` (subsets of hub-copy, single runs): 1,000 nodes 1.15 s → 2,000 nodes 4.08 s → 4,000 nodes 16.03 s → 10,000 nodes ~100 s. **Quadratic.** `analyze` RSS: 36 MB → 90 MB → 311 MB → 2.15 GB.

Git drift cost (bundles live in git repos): a 10k-node hub-copy copy, every node given `modified:`, git-committed → `validate` **20.62 s** (one `git log` subprocess per node; strace on 200-node bundle: 199 git execs, 0.38 s). Without `modified` keys no git calls happen (0.01 s).

Loop over the 50 standalone bundles (`loop50.sh`, sequential):
- validate ×50: 0.70 / 0.71 s total, all OK.
- search zebracorn-needle ×50: 0.77 / 0.79 s total; single hit `r37/okf: d3/c123.md [body]` ✔.
- `lint` on standalone r37 (200 nodes): 0.09 s, 28 findings — cross-repo links get a misleading suggestion: `broken-link: d9/c059.md links to ../../r45/d5/c175.md … did you mean d5/c175.md?` (same basename, wrong repo).

## Frontmatter / markers

- Adds non-spec keys `created` (on `node new`) and `modified` (on `node new`/`edit`, and `node refresh` rewrites it from git). `validate` drift-checks `modified` against git.
- `bundle init` writes a non-spec `.okf` sidecar (`okf_version: 0.2`); `bundle info` reads the version only from it and ignores `okf_version` in index.md frontmatter, which is where the spec puts it.
- Semantic plugin writes `.okfctl/index.db` inside the bundle (skipped by later walks).
- `eval transparency` expects okfctl-dialect `epistemic`/authority grade keys (non-spec); every fixture node gets `grade-missing`.
- Unknown keys preserved on `node edit`/`mv`/`refresh`; `mv` is byte-preserving; `edit` re-serialises flow mappings and quotes timestamps.

## Other observations

- `node rm --dry-run` reports orphans only (`orphaned: repos/billing-api/services/billing-api.md`); by design it doesn't rewrite or list the inbound links it will break.
- `node promote` on hub-copy (the remedy for the nested-index FAIL) turned the 3 `okf_version`-only indexes into concept files `repos/<r>/<r>.md` with no `type` → validate now `FAIL … missing or empty required field: type`. `index build` is the correct remedy.
- `okfctl serve` (local web graph) and the separate `okfctl-api` loopback server exist — [not run]: outside scope.
- 4 bundled agent skills in `skills/` (okf-authoring, okf-curation-health, okf-migrate-plan, okf-semantic-search; 1,099 lines total), packaged via plugin.json. They shell out to the CLI (no MCP).

## Strengths
1. `node mv` link rewriting: dry-run plan, rewrites relative, root-relative and hub-relative (`../../<repo>/…`) inbound links across all repos in an assembled (copied) hub, byte-preserves the moved file, and runs in 1.4 s at 10k nodes.
2. Offline, telemetry-free, single static binary; JSON output on search/lint/analyze/graph/eval; core read paths (validate, search, neighbors, graph export, index build) run in 0.4–1.0 s at 10k nodes.
3. Solid curation toolchain for agents: `index build/check` (spec-shaped progressive-disclosure indexes, CI gate), `log append`, `lint --strict`, `analyze` dangling-link/orphan/freshness reports, `eval transparency`, 4 ready-made agent skills, unknown frontmatter keys preserved.

## Blockers
1. Symlinks are never followed (`filepath.WalkDir`) → the symlinked hub sees 2/9 concepts; search, backlinks and mv are blind across repos. The hub must be a real copy/checkout, and there is no native multi-bundle federation (registry/connect only clone whole repos; no subdir option).
2. Doesn't scale for curation: `lint` is quadratic (~100 s at 10k nodes); `analyze` uses 2.15 GB RSS and prints 19M lines of JSON; validate git drift = 1 `git log` per node (20.6 s at 10k when `modified` is present).
3. Trust/lifecycle semantics are weak and some behaviour contradicts the docs: `stale_after` is ignored by every CLI command; the footnote↔`sources[].id` join isn't checked; search/show don't surface status, trust or staleness; `bundle init` silently overwrites an existing bundle's index.md/log.md although the docs say it refuses; log.md format doesn't follow spec §9; `okf_version` is read only from the non-spec `.okf` file.

## Surprises
- `okfctl search --semantic` fails (`unknown flag`) although the docs say the plugin is reachable through `okfctl search` dispatch; call `okfctl-search` directly.
- `index build` in a hub strips each repo's `okf_version` frontmatter and replaces hand-written index text and titles ("# Knowledge Base", "# Billing-api").
- `node promote` turns a spec warning into a hard failure on hub-style nested indexes.
- `go install` pulls a newer Go toolchain (235 MB) if the local Go is older than 1.26.6.

## Fit for the hybrid layout
Partial fit. Use okfctl per repo (`<repo>/okf/`: validate, index, log, lint, search) and on a **materialised** hub-copy (built with `cp`/`rsync`/`connect`, not symlinks) for cross-repo search, backlinks and `node mv`. After a move, sync the hub edits back to each repo while guarding against index regeneration dropping `okf_version`. Before relying on it you would need wrapper scripts or upstream fixes for: symlink following (or a documented copy step), a `stale_after`-aware staleness check, a footnote/source check, a faster lint at 10k nodes, and `bundle init` safety. `connect` is not a federation layer; it is git clone/pull.
