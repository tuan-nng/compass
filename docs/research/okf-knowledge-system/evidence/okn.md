# Open Knowledge CLI (`okn` / `openknowledge`) v0.13.0 — hands-on evaluation

- Repo: https://github.com/openknowledge-sh/openknowledge (Apache-2.0, Go, 59★, 7 forks, created 2026-06-15, last commit on `main` 2026-09-04, 24 open issues+PRs — `gh api repos/openknowledge-sh/openknowledge`, run 2026-09-29)
- Docs: https://openknowledge.sh/wiki/ (the docs site is itself an OKF bundle exported by the tool)
- Work dir: `/tmp/okf-research/okn/` (`bin/`, `home/` = `$HOME` for all runs, `home-scale/` = `$HOME` for the 50-bundle registry, `work/` = mutable copies)
- Every command below ran with `HOME=/tmp/okf-research/okn/home` (or `home-scale`) and `--no-telemetry` before the subcommand, except the telemetry probes in T1, which were explicitly sandboxed.
- Environment note: the harness shell exports **`CI=true`**, which changes telemetry behavior (see T1). `hyperfine` is not installed; timing used `/usr/bin/time` (see T10).

## Verdict summary

| Test | Verdict | One-liner |
|---|---|---|
| T1 Install | PASS (telemetry caveat) | Checksummed `gh release download`, 8.4 MB tgz → one 22.8 MB static Go binary, no runtime deps. Telemetry is **on by default**, and `--no-telemetry` only persists when `CI` is unset. |
| T2 Validate | PARTIAL | Catches nested-index frontmatter (error), link escapes, root-relative breakage in the hub, footnote↔`sources[].id` mismatch, bad `verified` actor/datetime, and bad log dates. Stale concepts are not a validate finding (they show up in `list`/`audit`). Directory links (`services/`) are false-positive warnings. Broken links are only warnings (exit 0). |
| T3 Symlinks | FAIL | Refuses a symlinked hub outright (`symbolic links are not supported inside knowledge bundles`, exit 2/1). |
| T4 Link resolution | PASS | Resolves relative, root-relative (bundle root) and `../../repo/...` forms exactly as expected per view. URL links are treated as external. |
| T5 Backlinks | PASS | `export graph` gives exactly the 3 expected inbound links in hub-copy. Search context also returns backlinks. There is no dedicated `backlinks` command. |
| T6 Search | PARTIAL | Good JSON with content-addressed locators and backlink expansion. Search output has **no trust/status/stale fields** (`list --json` and `audit` do). No index.md generation. |
| T7 Agent write path | PARTIAL | There is no "create concept" command. Agents write files by hand, and okn validates them (unknown keys untouched, `verified` format checked, trust tier derived). The tool-assisted write path (`claims`) needs a claim profile and an RDF-style ontology. `setup skill` installs a Claude skill. |
| T8 Move/rename | FAIL | No move command. Manual moves are detected only as `link-target` warnings (exit 0). |
| T9 Multi-bundle | PARTIAL | The user-level registry (`connect`) plus `search --all` (RRF fusion) spans bundles. Backlinks and links do **not** cross bundles, and there is no cross-bundle link syntax. |
| T10 Scale | PARTIAL | Validate 1.33 s ✅, backlinks via graph export 1.55 s ✅, hub search **5.92 s ❌** (1.3 GB RSS). Symlinked hub is unusable. |

---

## T1 Install

```
$ gh release view -R openknowledge-sh/openknowledge
tag: v0.13.0  published: 2026-08-31T20:28:16Z
asset: checksums.txt, install, openknowledge_{darwin,linux,windows}_{amd64,arm64}.tar.gz
$ /usr/bin/time -f '%e s' gh release download v0.13.0 -R openknowledge-sh/openknowledge \
    -p 'openknowledge_linux_amd64.tar.gz' -p checksums.txt -D bin
1.10 s
$ sha256sum -c checksums.txt --ignore-missing
openknowledge_linux_amd64.tar.gz: OK
$ tar xzf openknowledge_linux_amd64.tar.gz; ls -la; file openknowledge
-rwxr-xr-x 22769826 openknowledge          # 22.8 MB
-rw-r--r--  8409335 openknowledge_linux_amd64.tar.gz
LICENSE (Apache 2.0), README.md, THIRD_PARTY_NOTICES.md, third_party/licenses/{mermaid,mangle-go,goRDFlib,fsnotify,gofrs-flock,natefinch-atomic,golang-x-sys,GoogleCloudPlatform-knowledge-catalog}
openknowledge: ELF 64-bit LSB executable, x86-64, statically linked, Go BuildID=..., stripped
$ ln -s openknowledge okn; okn --no-telemetry version
0.13.0
```

- Install: one command, one static binary, no runtime deps, no daemon. The docs recommend `curl -fsSL https://openknowledge.sh/install | bash`, which was **not used** here.
- Commands `check` and `upgrade` **do not exist** in v0.13.0: `okn check` → `unknown command: check` and `okn upgrade` → `unknown command: upgrade`. `okn --version` is also unknown (exit 2); use `okn version`.
- Offline: the core commands (validate/list/search/get/export/query/audit) made no network syscalls under strace when telemetry was off.

### Telemetry (A1)

```
$ okn telemetry --help
Telemetry is enabled by default after a first-run disclosure. It sends only allowlisted command, outcome,
duration, version, platform, and random installation identifiers. ... Use --no-telemetry before a command
to disable telemetry persistently.

$ okn telemetry status            # fresh HOME
Telemetry:      enabled
Configuration: default

$ okn --no-telemetry telemetry show-payload
{ "schema_version": "1", "events": [ { "event_name": "cli_command_completed", "event_id": "random-event-id",
  "occurred_at": "2026-08-07T12:00:00Z", "surface": "cli", "installation_id": "random-installation-id",
  "app_version": "0.9.0", "os": "linux", "arch": "arm64", "command": "validate", "outcome": "success",
  "duration_bucket": "100ms-1s" } ] }          # static example payload (note stale app_version 0.9.0)
```

- Endpoint (from `strings`): `https://openknowledge.sh/api/telemetry`. The docs say the relay forwards to PostHog: https://openknowledge.sh/wiki/features/telemetry.html.
- Storage: `$XDG_CONFIG_HOME|~/.config/openknowledge/telemetry.json`, mode 0600. Env overrides: `DO_NOT_TRACK=1` and `OPENKNOWLEDGE_TELEMETRY=off` (process-level, not persisted). The docs also say "Installer preflight and continuous integration do not send telemetry."
- **Finding 1 (on by default):** with `CI` unset, a fresh `HOME`, and a plain `okn list <bundle>` run inside `bwrap --unshare-net`, the tool printed the disclosure to stderr and attempted DNS for the send on the **first** command. `strace -e connect` showed 8 `connect(... port 53 ...)` calls. Network was namespaced away, so nothing left the machine.
  ```
  Open Knowledge sends anonymous usage and sanitized error telemetry by default.
  It sends command names, outcomes, timing buckets, version, OS, architecture, and a random installation ID. ...
  ```
  With `DO_NOT_TRACK=1`: 0 `connect` calls.
- **Finding 2 (`--no-telemetry` persistence depends on `CI`):** in the harness shell (`CI=true`), `okn --no-telemetry version` and `okn --no-telemetry list …` **did not write** `telemetry.json`, and afterwards `okn telemetry status` still printed `enabled / Configuration: default`. strace showed the config file was never opened. This contradicts the help text ("disable telemetry persistently"). With `CI` unset (`env -u CI`), `okn --no-telemetry list …` did write `{"enabled": false}` and removed the `installationId`, so status became `disabled / Configuration: saved`. So the opt-out persists only outside CI. Inside CI, nothing is sent regardless (0 `connect` calls with `CI=true` and no flag).
- **Mitigation used for this evaluation:** ran `okn telemetry disable` explicitly, which wrote `home/.config/openknowledge/telemetry.json` = `{"schemaVersion":"1","enabled":false,"disclosedAt":…}` and printed `Telemetry is disabled. The random installation ID was deleted.` Kept `--no-telemetry` on every later command as well. **Recommendation:** set `DO_NOT_TRACK=1` in the environment and run `okn telemetry disable` once at install time. Do not rely on `--no-telemetry` alone.

## T2 Validate

`okn --no-telemetry validate <path>` (profile `bundle`, spec 0.2). Exit codes: 0 = pass (with or without warnings), 1 = errors, 2 = usage/setup error. `--json`, `--quiet`, and `--rule <id>=off|warn|error` are available; `.openknowledge.toml` `[validation.rules]` can also set severities.

| Target | Exit | Result |
|---|---|---|
| billing-api/okf | 0 | 3× warning `index.md: link target does not exist: services/` (also `contracts/`, `gotchas/`) |
| web-app/okf | 0 | `services/web-app.md:13` / `:14 link target escapes bundle root: ../../billing-api/...`, `../../shared-auth/...`, plus a `services/` dir warning |
| shared-auth/okf | 0 | `concepts/` dir warning only |
| workspace/hub (symlinks) | **2** | `symbolic links are not supported inside knowledge bundles: repos/billing-api` |
| hub-copy | **1** | 3× **error** `repos/*/index.md:1: index.md frontmatter may only declare okf_publish and okf_targets metadata` (rule `index-frontmatter`), plus `repos/billing-api/gotchas/idempotency-key.md:11: link target does not exist: /contracts/invoice-api.md` and dir-link warnings |
| hub-copy `--rule index-frontmatter=warn` | 0 | passes |

Planted items:
- Stale `idempotency-key` (stale_after 2026-09-01): **not** a validate issue. It is surfaced by `okn list` (`[unverified, draft, stale]`), by `list --json` (`okf02.stale: true`), and by `okn audit` (`HIGH stale … evidence: gotchas/idempotency-key.md:stale_after=2026-09-01T00:00:00Z`).
- Broken links: root-relative breakage in the hub ✅. Escaping `../../` links standalone ✅ (as warnings).
- Nested index frontmatter ✅ as an error. This is stricter than needed for the hub pattern and can be downgraded with `--rule index-frontmatter=warn`.
- Footnote/source id join: ✅. In a work copy, `[^nope]` with no matching `sources[].id` gives `okf-0.2-metadata warning runbooks/retry-storm.md footnote "nope" should match a sources[].id`. The valid `[^main-go]` in billing-api raised nothing.
- **False positive:** directory links such as `[Services](services/)`, which are the spec §8 example form (`* [Subdirectory](subdir/)` in the embedded `okn spec 0.2`), are flagged `link-target … does not exist`.
- **Broken links are warnings by default**, so `validate` exits 0. Use `--rule link-target=error` or the toml setting to gate CI.

## T3 Symlinks — FAIL

`validate`, `list`, and `search` on `workspace/hub` all abort:
```
$ okn --no-telemetry list  .../workspace/hub     → symbolic links are not supported inside knowledge bundles: repos/billing-api   exit=2
$ okn --no-telemetry search .../workspace/hub idempotency --matches   → same message, exit=1
```
The `.openknowledge.toml` docs confirm this is deliberate (a "real file system boundary"). A symlinked hub needs materialization (copy, git subtree/submodule, or `export tar`) before okn can read it.

## T4 Link resolution (from `okn export graph`, edges with resolved `targetId`)

| Link form | billing-api standalone | web-app standalone | hub-copy |
|---|---|---|---|
| `../contracts/invoice-api.md` (relative) | resolves | – | resolves |
| `/contracts/invoice-api.md` (root-relative, idempotency-key) | **resolves** | – | **broken** (warning) |
| `../../billing-api/contracts/invoice-api.md` (web-app) | – | **broken** ("escapes bundle root") | **resolves** → `repos/billing-api/contracts/invoice-api` |
| `../../shared-auth/concepts/session-token.md` | – | broken | resolves |
| `https://github.com/.../invoice-api.md` (URL) | – | external, no edge | external, no edge |
| hub `/repos/billing-api/...`, `/repos/web-app/...` | – | – | resolves |

Root-relative `/x` means the bundle root, as in the spec. Verdict PASS: matches the planted expectations exactly.

## T5 Backlinks across repos (hub-copy) — PASS

```
$ okn --no-telemetry export graph .../hub-copy | python3 (filter targetId == repos/billing-api/contracts/invoice-api)
cross-repo/invoice-dependency -> /repos/billing-api/contracts/invoice-api.md
repos/billing-api/services/billing-api -> ../contracts/invoice-api.md
repos/web-app/services/web-app -> ../../billing-api/contracts/invoice-api.md
```
These are the 3 expected links. The broken idempotency-key link is correctly absent. Graph JSON keys: `schemaVersion, root, specVersion, type, nodes, edges, issues`; each edge has `source/target/sourceId/targetId/label/href/line`. `okn search` also adds backlinks as context (`Relation: backlink`). `okn get --info` does **not** list backlinks. There is no `backlinks` subcommand. SPARQL/RDF (`okn query sparql`, `export rdf`) projects sources and claims, not Markdown links, and refuses bundles with validation errors (`semantic facts are invalid; fix validation issues before RDF projection` on hub-copy).

## T6 Search / retrieval — PARTIAL

```
$ okn --no-telemetry search .../hub-copy idempotency --matches
Matches: 5   Validation issues: 11
1. repos/billing-api/gotchas/idempotency-key.md:10-13  direct   900.88  Type: Gotcha
2. repos/billing-api/services/billing-api.md:16-22     direct   405.40
3. repos/billing-api/contracts/invoice-api.md:12-20    direct    87.65
4. cross-repo/invoice-dependency.md:13-20             backlink  39.44
5. repos/web-app/services/web-app.md:10-16            backlink  39.44
```
- `--format json` is machine-readable. Result fields: `path, id, locator (okf+sha256://<revision>/<path>#<section-sha>), contentSha256, kind, type, title, description, heading, lineStart/End, estimatedTokens, snippet, score, lexicalScore, vectorScore, rerankScore, matches[], relation`. Context mode (the default) packs original Markdown under a `--budget` token limit (default 2400) and returns `route: [bm25, vector, rerank, link_expansion]`. The vector is a local hashed-feature vector with no embedding service.
- **Trust tier, status, and staleness are NOT in search output.** They are available from `okn list --json`:
  ```
  "okf02": {"trustTier": "unverified", "status": "draft", "stale": true, "staleAfter": "2026-09-01T00:00:00Z",
            "generated": {"by": "cursor/gpt-5.6", "at": "2026-06-01T12:00:00Z"}}
  ```
  Text `list` shows `[machine-confirmed, stable]` for invoice-api, `[human-reviewed, stable]` for billing-api, and `[unverified, draft, stale]` for idempotency-key.
- `--filter type=…/tag=…` is supported.
- Progressive disclosure: `get` prints the root `index.md` or a declared `okf_bundle_entry_*`. **No index.md generation or regeneration command exists.** Indexes are hand- or agent-maintained.
- `mcp` (stdio, read-only) exists; not tested, since it is out of scope.
- `view` smoke test: started as a managed service on `127.0.0.1:18731`, got `GET /hub-copy/ → 302`, then killed it. No server left running.

## T7 Agent write path — PARTIAL

Work copy: `cp -a fixture/workspace/billing-api work/`, `git init`.
1. Hand-authored (agent-style) `okf/runbooks/retry-storm.md` with unknown keys `x-team-owner`, `custom_key: {nested: [1,2]}`, `generated: {by: claude-code/opus-5, …}`, and `sources`. okn never rewrites concept files, so unknown keys are trivially preserved.
2. `verified: { by: alice, at: yesterday }` →
   ```
   okf-0.2-metadata warning verified[0].by should identify an actor as <producer>/<version>, human:<id>, or process:<id>
   okf-0.2-metadata warning verified[0].at should be an ISO 8601 datetime with an explicit offset
   ```
   After fixing to `verified: { by: "human:alice", at: 2026-09-29T11:00:00Z }`, `list --json` gives `"trustTier": "human-reviewed"`.
3. `log.md` heading `## Sept 29` → `log-date error log.md 7 log date heading must use YYYY-MM-DD` (exit 1). `## 2026-09-29` passes.
4. index.md: appended by hand. There is no regenerate command.
5. `okn setup skill --scope project --project okf --harness claude </dev/null` validated the bundle, registered it (`access write`), and wrote `.openknowledge/integration.toml` plus `.claude/skills/openknowledge/SKILL.md` (40 lines, managed block). The skill tells agents to use `registry list`, `list/get/search`, and `validate`, and to route factual changes through `okn claims propose/apply`. Harnesses: codex/claude/opencode.
6. Typed-claims write path: `okn claims propose` failed with `claim proposal requires structured evidence`. Retrying with snake_case evidence failed with `json: unknown field "source_ref"` (the docs use snake_case, but the JSON input needs `sourceRef`). After adding evidence it wrote a digest-bound proposal JSON. `claims apply` then failed with `openknowledge_claim_profile must be "1"…`, and after adding that, `claim id must be an absolute IRI or use a declared namespace`. The path needs `claim_ontology` namespaces and predicates, which is RDF-grade authoring overhead. It has a human-approval model (`--approved-by <identity>` on verify/reject/supersede).
7. Diff size: `git diff --stat` after steps 1–4 showed only the files touched by hand (`okf/index.md | 1 +`, plus new `log.md` and the runbook). okn itself made no edits to concept files.

## T8 Move/rename — FAIL

No `mv`/`rename`/`refactor` command. Manual move on a hub-copy work copy (`invoice-api.md → invoice-api-v2.md`):
```
link-target warning cross-repo/invoice-dependency.md 14 link target does not exist: /repos/billing-api/contracts/invoice-api.md
link-target warning repos/billing-api/services/billing-api.md 18 link target does not exist: ../contracts/invoice-api.md
link-target warning repos/web-app/services/web-app.md 13 link target does not exist: ../../billing-api/contracts/invoice-api.md
grep hits for new name: 0 (nothing rewritten)
```
okn detects the breakage but does not repair it, and only warns (exit 0) by default.

## T9 Multi-bundle without hub — PARTIAL

```
$ okn --no-telemetry connect .../billing-api/okf --as billing-api   (also web-app, shared-auth)
OK Connected knowledge bundle  key billing-api  access read  status warnings
$ okn --no-telemetry registry list      → config /tmp/okf-research/okn/home/.config/openknowledge/registry.json
$ okn --no-telemetry search --all "session token" --matches
1. shared-auth / Session token   shared-auth:concepts/session-token.md:10-12  direct
2. web-app / Dependencies        web-app:services/web-app.md:10-16           direct
4. web-app / Checkout flow       outgoing-link
```
- Federation is a **per-user registry** (`~/.config/openknowledge/registry.json`), not a repo-committed workspace file. `connect` also accepts git URLs (`--git-ref`, `--git-subdir`), tarball URLs, and manifest URLs, cloned into a cache.
- `search --all` fuses per-bundle ranks with RRF (k=60) and reports each bundle's status. JSON results are `{knowledgeBase, rank, fusionScore, result{…}}`.
- **No cross-bundle link resolution or backlinks.** `../../billing-api/...` stays an "escapes bundle root" warning, and link expansion is per-bundle. No cross-bundle link syntax was found in the docs or binary strings.
- **Config and frontmatter:** none required for multi-bundle. `.openknowledge.toml` is optional (validation severities, viewer, publish, maintenance). The optional `okf_bundle_name/okf_bundle_title/okf_bundle_entry_*` keys in the root index.md frontmatter are okn extensions. `connect --as` defaults to `okf_bundle_name`, then the folder name. Claims need `openknowledge_claim_profile` plus `claim_ontology`.

## T10 Scale

Method: `hyperfine` is not installed, so I used `/tmp/okf-research/okn/bench.sh`: 1 untimed warmup, then 5 runs of `/usr/bin/time -f '%e %M %x'`, reporting min/median wall seconds and max RSS. Machine: linux x64. Scale fixture: `hub-copy` = 10,051 md files (10,000 concepts, 51 indexes).

| Operation | min s | median s | max RSS | Target | Result |
|---|---|---|---|---|---|
| `validate --quiet scale/hub-copy` (exit 1: 50× index-frontmatter errors, 0 link warnings) | 1.30 | 1.33 | 167 MB | <10 s | ✅ |
| `validate --quiet --rule index-frontmatter=warn scale/hub-copy` (exit 0) | 1.29 | 1.33 | 168 MB | <10 s | ✅ |
| `validate scale/hub` (symlinks) → exit 2 immediately | 0.00 | 0.00 | 14 MB | – | ❌ unsupported |
| `search hub-copy zebracorn-needle --matches --format json --no-expand` | 5.86 | 5.92 | **1.31 GB** | <2 s | ❌ |
| `search hub-copy zebracorn-needle` (context mode, default) | 5.86 | 5.92 | 1.37 GB | <2 s | ❌ |
| backlinks = `export graph --out g.json hub-copy` (14 MB JSON, 10,051 nodes / 30,839 edges; filter is extra) | 1.52 | 1.55 | 237 MB | <2 s | ✅ |
| `list --json hub-copy` | 1.41 | 1.43 | 203 MB | – | – |
| loop: `validate --quiet` × 50 standalone bundles | 1.81 | 1.87 | – | <10 s | ✅ |
| loop: `search` needle × 50 standalone bundles | 6.71 | 6.79 | – | <2 s | ❌ |
| `search --all` needle (50 registry entries, RRF) | 5.98 | 5.99 | 57 MB | <2 s | ❌ |
| single bundle r37: validate / search / export graph | 0.03 / 0.13 / 0.03 | | | | ✅ |
| index generation | – | – | – | – | N-A (no command) |

Correctness:
- hub-copy needle with `--no-expand`: exactly 1 hit, `repos/r37/d3/c123.md direct 351.72 ['body']`. With expansion (default): 6 results (1 direct, 3 outgoing-link, 2 backlink).
- `search --all`: `kbs 50 results 1 → r37 d3/c123.md`.
- The 50-bundle loop gave 1 hit in total (r37 only).
- Backlinks to `repos/r37/d3/c123` = `['repos/r37/d1/c091', 'repos/r37/d4/c094']`, which equals the ground truth from an independent regex scan of all 10,051 files.
- Cross-repo `../../rNN/...` links: 0 warnings in hub-copy. Standalone r37 has 17 `link target escapes bundle root` warnings.

Observations: there is no persistent index. Every search re-parses and re-indexes the whole corpus, and `connect` / `registry refresh` build no cache for local bundles (`connection "r37" is local and cannot be refreshed`). Search cost is superlinear: 0.13 s for 200 docs versus 5.9 s and 1.3 GB for 10k docs.

---

## Strengths
1. Single static offline binary with a checksummed release. Validation is precise and spec-aware, with per-rule severities, JSON output, the OKF 0.2 provenance/trust checks (`verified` actor/datetime, footnote↔`sources[].id`), and trust tiers derived as unverified/machine-confirmed/human-reviewed.
2. Link semantics are correct for all planted forms. Graph export gives exact cross-repo backlinks in a materialized hub in 1.5 s at 10k docs, and validate takes 1.3 s at 10k docs.
3. Agent-oriented retrieval: token-budgeted context packing, content-addressed citations, and backlink/outgoing expansion. There is also `setup skill` for Claude/Codex/OpenCode, a registry plus `search --all` federation, `audit` (stale, missing owner/source), and an optional read-only MCP.

## Blockers
1. **Symlinked hub is rejected**, so the hybrid layout needs a materialized hub-copy (copy, subtree, or tar). There is also no cross-bundle link or backlink resolution in registry federation.
2. **Search is too slow at scale**: 5.9 s and 1.3 GB RSS for a 10k-doc hub, and 6 s for `search --all` over 50 bundles, with no persistent index. Search output also omits trust tier, status, and staleness, so the agent must call `list --json` too.
3. **Telemetry on by default**, and `--no-telemetry` does not persist when `CI` is set (contrary to its help text). There are also no write-path helpers: no concept create, index regenerate, log append, or move/rename with link rewrite. `claims` requires RDF ontology authoring.

## Surprises
- `check` and `upgrade` do not exist in v0.13.0. `--version` fails; use `version`.
- Spec-sanctioned directory links (`[x](subdir/)`) are reported as broken links.
- Nested `index.md` with `okf_version` is an **error** (`index-frontmatter`), so every assembled hub fails until the rule is downgraded or repo index frontmatter is stripped.
- The claims JSON input uses camelCase (`sourceRef`), while the documented YAML is snake_case (`source_ref`).
- `telemetry show-payload` prints a static example (`app_version 0.9.0`, `arch arm64`), not this machine's payload.
- The harness blocked `gh api repos/.../releases` ("merging or releasing through the API requires explicit user approval"). Release data came from `gh release view` instead.

## Fit for the hybrid layout
Good as a **validator and graph/backlink engine run on a materialized hub** (a CI job that copies repo bundles into `hub/repos/<name>/`, runs `validate --rule index-frontmatter=warn --rule link-target=error` and `export graph`). Good for per-repo validation in each repo's CI (0.03 s per bundle). Weak as the agent's primary interactive search over the whole hub at 50 repos: ~6 s per query with no index. Per-repo search (0.13 s) is fine. There is no agent write tooling for concepts, index, log, or moves. Symlink assembly and registry federation cannot give cross-repo backlinks. Pin the telemetry opt-out via `DO_NOT_TRACK=1` plus `okn telemetry disable`.
