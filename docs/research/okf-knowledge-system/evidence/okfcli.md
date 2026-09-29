# okfcli/okf — OKF v0.2 evaluation

- Tool: `github.com/okfcli/okf` **v0.5.0** (module resolved by `@latest`; `go version -m bin/okf` → `mod github.com/okfcli/okf v0.5.0`, dep `gopkg.in/yaml.v3 v3.0.1`). `okf version` prints `"version": "dev"` because `go install` does not inject ldflags.
- Binary: `/tmp/okf-research/okfcli/bin/okf` (always invoked by absolute path, written below as `$O`). `HOME`, `GOPATH`, `GOMODCACHE`, `GOCACHE` were all pointed inside `/tmp/okf-research/okfcli/`.
- Fixtures: `F=/tmp/okf-research/fixture`, `S=/tmp/okf-research/scale` (read-only; every mutation happened on `cp -a` copies under the work dir).
- Authority: `$O --help`, `$O schema` (JSON describing every command, its flags and exit codes) and source under `gopath/pkg/mod/github.com/okfcli/okf@v0.5.0`.
- Timing method (hyperfine is not installed): a Python `time.perf_counter` harness (`t10/bench.py`) does 1 warmup plus 5 timed runs and reports min and median. For the index timing, `rm -rf && cp -a` runs as untimed setup before each run. Peak RSS comes from `/usr/bin/time -f '%e s %M KB'`.

## T1 Install — PASS (with caveats)

```
GOBIN=/tmp/okf-research/okfcli/bin go install github.com/okfcli/okf/cmd/okf@latest
go: downloading github.com/okfcli/okf v0.5.0
go: github.com/okfcli/okf@v0.5.0 requires go >= 1.27.0; switching to go1.27.1
go: downloading go1.27.1 (linux/amd64)
go: downloading gopkg.in/yaml.v3 v3.0.1
Elapsed (wall clock) 0:13.01
```
- Wall time was 13.0 s, and that includes auto-downloading the go1.27.1 toolchain. **The module requires go ≥ 1.27**, while the host has go 1.26. With `GOTOOLCHAIN=local` the install would fail. Afterwards the module cache plus toolchain took 321 MB and the build cache 45 MB.
- The binary is 5.0 MB, a single static ELF (`statically linked`, not stripped). Its only runtime dependency is libc-free Go; the one Go dependency is yaml.v3.
- License: Apache-2.0 (LICENSE file).
- Network and telemetry: the source has no `net`/`http` imports. `strace -f -e trace=network okf validate $F/hub-copy` recorded **0 network syscalls** (only the `+++ exited` lines). No config is written to $HOME and no daemon runs. The tool is offline.
- `okf init t1/newb` created `index.md` (with `okf_version: "0.2"` frontmatter) and a `.gitignore`, and reported `tables/ datasets/ playbooks/` as created. That layout suits a data catalog, not code repos, and empty directories are not tracked by git. A fresh bundle validates as `True 0 0`.
- Exit codes follow a documented contract: 0 ok, 1 validation, 2 I/O, 3 internal, 4 usage. Every command writes JSON to stdout and `error[kind]: …` to stderr.

## T2 Validate — PARTIAL

| Bundle | Exit | Errors | Warnings | Findings |
|---|---|---|---|---|
| billing-api/okf | 1 | 2 | 0 | `okf/lifecycle/stale-after-invalid` ×2 (invoice-api, idempotency-key) |
| web-app/okf | 0 | 0 | 2 | `okf/links/broken` ×2 (`../../billing-api/…`, `../../shared-auth/…`) |
| shared-auth/okf | 0 | 0 | 0 | — |
| workspace/hub (symlinks) | 0 | 0 | 3 | `okf/links/broken` ×3 on cross-repo/invoice-dependency (`/repos/...` not found; symlinks were not followed, see T3) |
| hub-copy | 1 | 5 | 1 | stale-after-invalid ×2; `okf/reserved/index-frontmatter` ×3 (repos/*/index); `links/broken` idempotency-key → `/contracts/invoice-api.md` |

Sample output:
```
{"concept_id": "gotchas/idempotency-key", "message": "frontmatter: 'stale_after' must be an absolute YYYY-MM-DD date, got \"2026-09-01T00:00:00Z\" (OKF §5.5)", "rule": "okf/lifecycle/stale-after-invalid", "severity": "ERROR"}
{"concept_id": "repos/billing-api/index", "message": "index.md files must not contain frontmatter; only the bundle-root index.md may carry 'okf_version' (OKF §8)", "rule": "okf/reserved/index-frontmatter", "severity": "ERROR"}
```

What it caught:
- **Stale concept: missed on the fixture.** The fixture uses the spec's own form, `stale_after: 2026-09-01T00:00:00Z`. SPEC §5.5 says "An absolute instant" and its examples are all datetimes. The tool hard-codes `time.Parse("2006-01-02")` (internal/validate/v02.go:39) and raises an ERROR instead. This is a **tool/spec divergence (bug)**. `show` reports `"stale": false` for that concept.
  - On a copy with the value rewritten as `stale_after: 2026-09-01`, it emits `okf/lifecycle/stale WARN "concept is stale: today >= stale_after (2026-09-01)"`, and `show` gives `stale: true`.
- **Broken links: yes**, and the result depends on the view (see T4). URL links are not checked. Broken links are WARN, not error.
- **Nested index frontmatter: yes**, as an ERROR in hub-copy (3×).
- **Footnote ↔ sources[].id join: yes.**
  - The unmodified billing-api file (`[^main-go]` ↔ `sources[].id: main-go`) raises no finding.
  - In a copy with the definition renamed to `[^mian-go]`, three WARNs fire: `okf/sources/footnote-undefined`, `footnote-unreferenced` and `footnote-unmatched`.
- SARIF: `validate --format sarif` produces `2.1.0 https://json.schemastore.org/sarif-2.1.0.json`, driver `okf`, one rule entry per rule. Results carry `artifactLocation.uri` but **no `region` (line numbers)**. `--exit-zero` exits 0.
- **`lint` bug:** `okf lint $F/workspace/billing-api/okf` returned `"errors": 2, "findings": [], "valid": false` with exit 0. The error counts leak while the findings are suppressed, which is inconsistent. On hub-copy it returned `"errors": 5`, only 1 WARN finding, and exit 0.

## T3 Symlinks — FAIL

```
$O list $F/workspace/hub   -> count 2 (cross-repo/invoice-dependency, decisions/adr-001-okf-layout)
$O list $F/hub-copy        -> count 9 (+ 7 repos/* concepts)
$O graph $F/workspace/hub  -> node_count 2, edge_count 1
```
The loader uses `filepath.WalkDir` (internal/bundle/bundle.go:70), which does not follow symlinked directories. `repos/<name>` symlinks are therefore **silently skipped**: no warning, and validate passes (exit 0). The same happens at scale: `list $S/hub` gives count 0 and validate gives `valid True errors 0`, a false green. The hub must be assembled with **real copies**, or something like a bind mount or git submodule. Plain symlinks do not work.

## T4 Link resolution — PASS (on copies)

| Link form | Standalone | hub-copy | hub (symlink) |
|---|---|---|---|
| billing `/contracts/invoice-api.md` (root-relative) | resolves | **broken** (WARN) | n/a (not loaded) |
| billing `../contracts/invoice-api.md` (relative) | resolves | resolves | n/a |
| web-app `../../billing-api/contracts/invoice-api.md`, `../../shared-auth/…` | **broken** (WARN ×2) | resolves (graph edges `repos/web-app/services/web-app → repos/billing-api/contracts/invoice-api`, `→ repos/shared-auth/concepts/session-token`) | n/a |
| web-app URL link (external) | not checked, not an edge | same | same |
| hub `/repos/billing-api/…`, `/repos/web-app/…` | — | resolves | **broken** (targets not loaded) |

Every result matches the planted expectations when the hub is a copy. `/`-prefixed links resolve against the root of whichever bundle is being validated, so a root-relative link inside a repo bundle breaks once that bundle is nested in the hub.

## T5 Backlinks across repos — PASS (hub-copy) / FAIL (symlink hub)

```
$O backlinks $F/hub-copy repos/billing-api/contracts/invoice-api
"backlinks": ["cross-repo/invoice-dependency", "repos/billing-api/services/billing-api", "repos/web-app/services/web-app"], "count": 3
$O backlinks $F/workspace/hub repos/billing-api/contracts/invoice-api   -> count 0, exit 0
$O backlinks $F/workspace/billing-api/okf contracts/invoice-api         -> ["gotchas/idempotency-key","services/billing-api"]
```
The hub-copy result matches the expected set exactly, and idempotency-key is correctly excluded because its link is broken in the hub. Caveat: an unknown ID (`…invoice-api.md` or a leading `/`) returns `count 0, exit 0` rather than an error. An agent cannot tell "no backlinks" apart from "wrong ID".

## T6 Search / retrieval — PARTIAL

```
$O search $F/hub-copy --text idempotency
"count": 3, "filters": {"Tag":"","Type":"","Text":"idempotency"},
"results": [{"id":"repos/billing-api/contracts/invoice-api","title":"Invoice API v2","type":"API Contract"},
            {"id":"repos/billing-api/gotchas/idempotency-key", ...,"type":"Gotcha"},
            {"id":"repos/billing-api/services/billing-api", ...,"type":"Service"}]
$O search $F/hub-copy --tag billing              -> 5 hits
$O search $F/hub-copy --type gotcha --tag retries -> 1 (AND-combined, case-insensitive)
$O search --text idempotency $F/hub-copy          -> error[usage]: unknown search flag: --text (exit 4; flags must follow the path)
$O search $F/workspace/hub --text idempotency     -> 0 (symlinks)
```
- Output is JSON and machine-readable.
- **Search hits carry only id, title and type**, with no status, trust_tier, stale, snippet or score. The agent needs a second call:
  - `list` adds `status` and `trust_tier` (for example `"status": "draft", "trust_tier": "unverified"`, `"trust_tier": "machine-confirmed"` for the `process:` verifier, `"human-reviewed"` for billing-api) but not `stale`.
  - `show <id>` returns everything: status, trust_tier, stale, stale_after, generated, verified[], sources[], body, all nested under `"concept"`.
- There is no ranking; results are sorted by ID and matching is plain substring.
- Staleness is wrong for spec-form datetimes: fixture `stale: false`, as described in T2.
- Progressive disclosure: `okf index` writes an `index.md` for each directory, containing a Title/Type/Description table, plus a root listing of subdirectories (see T7). The index does not include status, trust or stale.

## T7 Agent write path — PARTIAL

okfcli has no create/edit/move command. The agent path is: write the file directly, then `validate`, `index`, and `show` to confirm. This was run on a git copy `t7/` of billing-api/okf.

1. The agent wrote `okf/gotchas/retry-budget.md` with `generated: {by: claude-code/opus-5, at: …}`, an unknown key `x_owner: team-billing`, `sources[].id: retry-go` and footnote `[^retry-go]`.
2. `$O validate okf` returned 3 errors, all `stale-after-invalid`, including the new file because it used the spec's datetime form. The footnote join raised no finding.
3. `$O index okf` wrote 4 `index.md` files (contracts, gotchas, services, root). `git diff --stat`: `okf/index.md | 13 +++++++++----`, and 3 new sub-index files appeared.
   - **Root `okf_version: "0.2"` was preserved.**
   - **The hand-written root index was replaced.** The heading `# Billing API knowledge` and the per-directory descriptions (`- Deployable services in this repo`, …) became the generic `# Index / Bundle root. / ## Subdirectories`. Curated prose in `index.md` does not survive regeneration.
4. After commit, adding `verified: { by: human:novpla, at: 2026-09-29T12:00:00Z }` and rerunning `index` produced this diff: **a 1-line `+verified` in the concept and no `index.md` changes**, so index is idempotent. `show` then returned `trust_tier: human-reviewed, verified: [{by: human:novpla, …}]`.
5. The agent appended `log.md` by hand (`# Log / ## 2026-09-29 / - …`). It validates. A heading `## Sept 30` triggers `okf/reserved/log-date-heading`.
6. Unknown keys: `x_owner` is preserved on disk, because the tool never rewrites concept files. However, **`show` drops it**: the output keys are body, description, generated, id, path, resource, sources, stale, stale_after, status, tags, title, trust_tier, type, verified. Agents cannot read extension keys through the CLI.

Diffs are small and reviewable because every write is a plain file write. The only noise is the first index regeneration.

## T8 Move/rename — FAIL (no support; detection only)

With no `mv` command, the test used plain `mv` on a hub-copy copy: `repos/billing-api/contracts/invoice-api.md → invoice-api-v2.md`.
```
$O validate .  (okf/links/broken)
cross-repo/invoice-dependency | -> /repos/billing-api/contracts/invoice-api.md
repos/billing-api/gotchas/idempotency-key | -> /contracts/invoice-api.md
repos/billing-api/services/billing-api | -> ../contracts/invoice-api.md
repos/web-app/services/web-app | -> ../../billing-api/contracts/invoice-api.md
$O backlinks . repos/billing-api/contracts/invoice-api-v2 -> 0
```
Inbound links are not rewritten. Validate lists every dangling inbound link, including the cross-repo ones in the hub view, so an agent could fix them by hand. Standalone validation of the other repos would miss the cross-repo breakage.

## T9 Multi-bundle without hub — FAIL (none native)

- Every command takes exactly one `<bundle>`. `$O search $W/billing-api/okf $W/web-app/okf --text idempotency` fails with `error[usage]: unknown search flag: …/web-app/okf` (exit 4). The code and README have no workspace, registry or federation concept (README: "no schema registry, no central authority").
- Pointing it at the workspace root (`$O list $W`) treats that root as one big bundle. It returns 9 concepts with IDs like `billing-api/okf/...` and `hub/...`. Validate on the root gives 6 errors and 7 warnings: web-app `../../billing-api/contracts/...` is broken because of the extra `okf/` segment, and each repo's `index.md` gets flagged for index-frontmatter. This does not work as federation.
- A shell loop over bundles works (`for b in …; search`) but has no cross-bundle backlinks and no link resolution.
- **Assembling a hub from copied bundles is the only way to span repos.** Cross-bundle links must be written in the hub-relative form (`../../<repo>/…` from inside a repo bundle, or `/repos/<repo>/…` from the hub), and those are broken when validated standalone.

## T10 Scale — PASS on hub-copy, FAIL on symlink hub

Timing: 1 warmup plus 5 runs, perf_counter, min and median (`t10/bench.py`).

| Operation | hub-copy (10,051 .md) | hub (symlinks) | Target |
|---|---|---|---|
| validate | min 0.592 s / med 0.606 s (exit 1); peak RSS 67,744 KB | 0.002 s: loads 0 concepts, `valid True` (false green) | < 10 s ✔ |
| search `zebracorn-needle` | min 0.354 / med 0.374 s → **1 hit `repos/r37/d3/c123`** ✔ | 0.002 s → 0 hits ✘ | < 2 s ✔ |
| backlinks `repos/r37/d3/c123` | min 0.335 / med 0.341 s → 2 (`r37/d1/c091`, `r37/d4/c094`; matches grep) | 0 ✘ | < 2 s ✔ |
| index (on a fresh `cp -a` copy; cp untimed) | min 0.984 / med 0.998 s | — | — |
| loop validate 50 standalone bundles | min 0.854 / med 0.856 s | | |
| loop search needle 50 standalone | min 0.613 / med 0.619 s | | |

Validate on hub-copy found 50 errors, all `okf/reserved/index-frontmatter` (the nested `repos/rNN/index.md` files). It found 0 broken links, so the ~10% of cross-repo `../../rNN/...` links all resolve in the copy. There is no persistent index or cache: every command re-walks and re-parses the whole tree, and it stays below 1 s at 10k files.

## Strengths
1. Single 5 MB static offline binary. It makes 0 network syscalls, has no daemon or config, is Apache-2.0, and has a stable JSON contract, a `schema` command for agents, and documented exit codes.
2. Validator coverage of v0.2 is strong: stable rule IDs (`okf/<family>/<check>`), trust tiers (unverified / machine-confirmed / human-reviewed), the footnote ↔ sources.id join, nested index frontmatter, log.md headings, SARIF 2.1.0 output and `--exit-zero` for CI.
3. It is fast at 10k concepts (validate ≈0.6 s, search and backlinks ≈0.35 s), and hub-view backlinks and link resolution are exactly correct on a copied hub.

## Blockers
1. **Symlinked hub is invisible**: WalkDir does not follow symlinks, and validate returns a false green. Copies, submodules or bind mounts are required.
2. **`stale_after` rejects the spec's datetime form** (SPEC §5.5 is an instant; the tool accepts YYYY-MM-DD only). As a result staleness is not detected on spec-conformant files. The same applies to nested `index.md` `okf_version` inside the hub (3 errors per hub view, which matches the planted strict-validator expectation).
3. Read-only with no federation: there are no create, mv or rewrite commands, only one bundle per command, and `index` overwrites curated root `index.md` prose. Search hits omit trust, status and staleness, and `show` drops extension keys.

## Surprises
- `lint` reports non-zero `errors` and `valid: false` with empty findings and exit 0.
- Flags must come after the bundle path.
- An unknown ID in `backlinks` returns exit 0 with count 0.
- `okf version` prints "dev".
- The module needs go ≥ 1.27 and silently downloads the toolchain.
- `init` scaffolds data-catalog directories (tables, datasets, playbooks).

## Fit for the hybrid layout
It works as a per-repo validator/CI gate and as the query engine over a **copied/assembled** hub. It does not work over the symlink hub layout as given. The agent write path is "plain file write + `okf validate` + `okf index`", which gives small git-reviewable diffs. Before adoption: (a) assemble the hub with copies or a sync step, (b) use a date-only `stale_after` or patch the tool, (c) drop frontmatter from repo-bundle `index.md` or accept 1 error per repo in the hub view, (d) keep curated prose out of `index.md`.
