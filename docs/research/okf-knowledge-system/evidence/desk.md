# OKF v0.2 tooling — desk research (2026-09-29)

Method: `gh api repos/O/R`, `gh api repos/O/R/releases/latest`, `gh api repos/O/R/commits?per_page=1`,
`gh api repos/O/R/contributors?per_page=100 --paginate | jq -s 'add|length'`. Values collected 2026-09-29.
Scratch downloads (SPEC copies, READMEs) were fetched via raw.githubusercontent.com and deleted after use.

## 1. Maturity table

| Repo | ★ | Forks | License | Created | Latest release | Last commit (default branch) | Open issues+PRs | Contributors | Lang |
|---|---|---|---|---|---|---|---|---|---|
| [cwest/okfctl](https://github.com/cwest/okfctl) | 8 | 3 | Apache-2.0 | 2026-07-21 | v0.4.0 (2026-08-19) | 2026-09-29 | 17 | 2 | Go |
| [okfcli/okf](https://github.com/okfcli/okf) (binary `okf`; tap [okfcli/homebrew-okf](https://github.com/okfcli/homebrew-okf)) | 25 | 4 | Apache-2.0 | 2026-06-18 | v0.5.0 (2026-09-03) | 2026-09-03 | 8 | 2 | Go |
| [mfdaves/okf-mcp](https://github.com/mfdaves/okf-mcp) | 7 | 1 | MIT | 2026-06-21 | v0.9.1 (2026-09-19) | 2026-09-19 | 0 | 1 | JavaScript |
| [openknowledge-sh/openknowledge](https://github.com/openknowledge-sh/openknowledge) | 59 | 7 | Apache-2.0 | 2026-06-15 | v0.13.0 (2026-08-31) | 2026-09-04 | 24 | 4 | Go |
| [GoogleCloudPlatform/open-knowledge-format](https://github.com/GoogleCloudPlatform/open-knowledge-format) | 640 | 47 | Apache-2.0 | 2026-08-11 | none (releases/latest → 404) | 2026-08-21 | 23 | 1 | HTML |
| [GoogleCloudPlatform/knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog) | 9315 | 791 | Apache-2.0 | 2026-05-04 | none (404) | 2026-09-21 | 200 | 13 | TypeScript |
| [basicmachines-co/basic-memory](https://github.com/basicmachines-co/basic-memory) | 4062 | 298 | AGPL-3.0 | 2024-12-02 | v0.23.2 (2026-08-25) | 2026-09-29 | 69 | 45 | Python |

Notes: okfcli repo found via `gh api users/okfcli/repos` after `gh search repos okfcli` only returned the tap + an unrelated 0★ repo. `open_issues_count` includes PRs (GitHub API semantics). Contributor count for knowledge-catalog is whole-repo, not OKF-only. pushed_at for okfcli/okf is 2026-09-21 but last default-branch commit is 2026-09-03.

## 2. Google reference agent

Source: [open-knowledge-format/README.md](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/README.md), [pyproject.toml](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/pyproject.toml), [src/reference_agent/cli.py](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/src/reference_agent/cli.py), [agent.py](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/src/reference_agent/agent.py). Identical copy under `knowledge-catalog/okf/src/reference_agent/` (frozen).

- Package `reference-agent` 0.1.0, `requires-python >=3.11`; README installs with `python3.13 -m venv` + `pip install -e .[dev]`.
- Deps: `google-adk>=2.0` (ADK), `google-cloud-bigquery>=3.20`, `pyyaml`, `pydantic>=2`, `markdownify`.
- Model: Gemini, `DEFAULT_MODEL = "gemini-flash-latest"`, `--model` override; auth via `GEMINI_API_KEY` or Vertex (`GOOGLE_GENAI_USE_VERTEXAI=true`, `GOOGLE_CLOUD_PROJECT`). BigQuery via `gcloud auth application-default login` + billing project.
- Consumes: **only BigQuery** (`_SOURCES = ("bq",)`, `enrich --source bq`) — dataset metadata pass, then an optional web pass crawling seed URLs (`--web-seed[-file]`, `--web-max-pages`, `--web-allowed-host`, `--no-web`).
- Produces: OKF bundle dir (concept docs per table/dataset/metric/join, `index.md`s) + `visualize` subcommand emitting a single-file `viz.html` graph viewer. Samples: ga4, stackoverflow, crypto_bitcoin (+ hand-made acme_retail).
- Connector: `connectors/gcp-knowledge-catalog.md` (publish bundles to Knowledge Catalog; PRs #2/#3).
- **Fit for code-repo knowledge: poor.** No source for git repos/code; hard GCP/Gemini coupling. Useful only as a reference for bundle/index/viewer code (`bundle/index.py`, `viewer/`).

## 3. Spec version

- Canonical home **moved**: `knowledge-catalog/okf/README.md` begins "OKF now lives in its own repository: GoogleCloudPlatform/open-knowledge-format … Stop using the copy under `okf/` … It is a frozen snapshot" ([source](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/README.md)). Import PR: [#1](https://github.com/GoogleCloudPlatform/open-knowledge-format/pull/1) (2026-08-11).
- SPEC diff: both files 1006 lines, line 3 `**Version 0.2**`; `diff` of [OKF main SPEC](https://raw.githubusercontent.com/GoogleCloudPlatform/open-knowledge-format/main/SPEC.md) vs [knowledge-catalog okf/SPEC](https://raw.githubusercontent.com/GoogleCloudPlatform/knowledge-catalog/main/okf/SPEC.md) → **0 differences** (so the snapshot already includes merged PR [#6](https://github.com/GoogleCloudPlatform/open-knowledge-format/pull/6) "Make every timestamp an ISO 8601 datetime with an explicit offset").
- Conformance (§11): every non-reserved `.md` has YAML frontmatter with non-empty `type`; `index.md`/`log.md` reserved. `verified` mapping-or-list; `human:<id>` convention for human verifiers (SPEC lines ~93, ~494–501).
- Branches ([api](https://api.github.com/repos/GoogleCloudPlatform/open-knowledge-format/branches)): `main`, `full-content` (2026-08-14 "Snapshot of the full repository contents", SPEC still 0.2, 66 diff lines = older text), `okf-iso-datetimes` (= main SPEC). No v0.3 branch. `grep -n '0\.3'` of SPEC: no hit.
- **No v0.3 draft exists**. Only proposals: knowledge-catalog issue [#312](https://github.com/GoogleCloudPlatform/knowledge-catalog/issues/312) "v0.3 proposal: a Skill concept type"; OKF issue [#24](https://github.com/GoogleCloudPlatform/open-knowledge-format/issues/24) notes #6 is breaking without a version bump (`okf_version: "0.2"` names two documents). Other open spec proposals: #16 typed `relationships`, #22 supersedes/contested_by, #28 `revised`, #13 `refuted`, #15 `imported`, #11 deletion semantics, #9 `[[wikilinks]]`, #8 JSON Schema, #29/#31 path-relativity inconsistency, #26 multi-part bundles/`okf_version` loss on index regen (PR #25). ~7 open community PRs, none merged since 2026-08-21 → spec maintenance slow.

## 4. Basic Memory (desk only)

Source: [README](https://github.com/basicmachines-co/basic-memory/blob/main/README.md) "The Markdown format", "CLI essentials"; [models/search.py](https://github.com/basicmachines-co/basic-memory/blob/main/src/basic_memory/models/search.py).

- Frontmatter: `title`, `type: note`, `permalink`, `tags` (README shows `type: note`; other values not documented in README → [not found]).
- Observations: `- [category] text #tag (context)`; Relations: `- relation_type [[Target]]`, quoted multi-word `- "pairs well with" [[X]]`; bare `[[X]]` indexes as `links_to`. **Links are wikilinks by title**, not relative markdown paths.
- OKF conformance: frontmatter has `type`, so files would pass OKF §11 structurally [INFERENCE]; but no `index.md`/`log.md` conventions, no `okf_version`, no trust (`verified`) family, wikilinks unresolvable by OKF consumers (OKF issue #9 still open).
- Install: `uv tool install basic-memory` (README), Homebrew mentioned for auto-updates (exact formula [not found] in README); `uvx basic-memory mcp` for clients.
- Index: local SQLite with FTS5 virtual table `search_index` (search.py lines 134–137), optional Postgres, hybrid FastEmbed vector search, optional reranking. `basic-memory doctor` = file↔DB consistency.
- Multi-project: yes (`basic-memory project add/list`, per-project cloud routing).
- Surfaces: MCP (primary) **and** CLI (`basic-memory`/`bm`, `--json`, `tool edit-note` exposes MCP tools on CLI); Obsidian compatible; paid cloud tier.
- Human verification features: none documented in README ([not found]; searched README for "verif"/"review").
- License AGPL-3.0 (consider for embedding).

## 5. llm-wiki omp extension

Located at `~/.omp/plugins/node_modules/@zosmaai/pi-llm-wiki/skills/llm-wiki/SKILL.md` (package `@zosmaai/pi-llm-wiki` v0.12.1, repo [zosmaai/pi-llm-wiki](https://github.com/zosmaai/pi-llm-wiki), 598★, pushed 2026-09-26). Not present under `/mnt/data/works/compass/.omp`; no `.llm-wiki/` in compass.

- Layout (SKILL.md l.13–43): `.llm-wiki/{config.json,templates/,raw/sources/SRC-*/,raw/trajectories/TRJ-*/,wiki/{sources,entities,concepts,syntheses,analyses,cases,skills},meta/{registry.json,backlinks.json,index.md,log.md,events.jsonl},outputs/}`. Personal vault `~/.llm-wiki/` + project vault merged in recall (l.123).
- `type` values (l.270): `entity | concept | source | synthesis | analysis | skill | case | trajectory`; plus `created`, `updated`, `sources`, and per-type `category`/`domain`/`trajectories`/`outcome`; company variant `confidence`.
- Links (l.51, 263–264): standard Markdown `[label](/folder/page.md)` preferred; legacy `[[wikilinks]]` still read; citations still shown as `[[sources/SRC-…]]` (l.284).
- meta/ is extension-owned/generated (`meta/index.md` catalog; don't hand-edit).
- **OKF mode exists**: `dist/extensions/llm-wiki/lib/vault-format.js` supports vault format `"legacy" | "okf-0.2"`; in okf mode requires `wiki/index.md` with `okf_version: "0.2"` and generates OKF `index.md`s and `wiki/log.md` (`lib/metadata.js` l.39–64). Design doc `docs/superpowers/specs/2026-08-02-okf-v0.2-interoperability-design.md` (goal: `.llm-wiki/wiki/` a conformant OKF v0.2 bundle; import review gate, export, migration; "Automatically committing wiki changes to git" is a non-goal).
- Human verification: no `verified`/human-review workflow in SKILL.md ([not found]); OKF interop doc lists provenance/verification visibility as a goal.
- Coupling: pi/omp extension (package.json keywords `pi`,`omp`; `omp.extensions`; peerDep `@earendil-works/pi-coding-agent >=0.78.1`); tools (`wiki_*`) are agent-side; design mentions an MCP surface sharing service ops. No standalone CLI observed.

## 6. Other OKF tools (>10★ or pushed ≥2026-07-01; filtered to relevant)

Queries: `gh search repos "open knowledge format" --limit 50`, `"okf bundle"`, `"okf knowledge"`, `"okf mcp"`, `okf --sort updated --limit 100`. `gh search code okf_version` worked but returned mostly unrelated repos (noise) — not listed. Star counts from search API, 2026-09-29.

| Repo | ★ | Pushed | What |
|---|---|---|---|
| [zosmaai/pi-llm-wiki](https://github.com/zosmaai/pi-llm-wiki) | 598 | 09-26 | pi/omp LLM wiki, OKF 0.2 mode (see §5) |
| [scaccogatto/okf-skills](https://github.com/scaccogatto/okf-skills) | 401 | 09-28 | Claude Code toolkit: author/maintain/validate/visualize OKF |
| [joshuaswarren/remnic](https://github.com/joshuaswarren/remnic) | 211 | 09-28 | agent memory w/ provenance (OKF-related per search) |
| [serradura/okf](https://github.com/serradura/okf) | 169 | 09-06 | OKF durable memory for agents: author/validate |
| [coleam00/cole-medin-knowledge-base](https://github.com/coleam00/cole-medin-knowledge-base) | 130 | 07-28 | OKF bundle + LLM wiki (content) |
| [jyjeanne/okf-rs](https://github.com/jyjeanne/okf-rs) | 104 | 08-21 | Rust toolkit: generate/validate/serve OKF |
| [Albertchamberlain/Awesome-OKF](https://github.com/Albertchamberlain/Awesome-OKF) | 98 | 09-15 | curated tool catalog |
| [OWOX/models](https://github.com/OWOX/models) | 89 | 09-25 | visual data-model editor in OKF |
| [0dust/OKFy](https://github.com/0dust/OKFy) | 73 | 08-17 | docs → OKF bundles |
| [sniperunder123/okf-knowledge](https://github.com/sniperunder123/okf-knowledge) | 66 | 09-17 | Claude Code `/okf` skill |
| [oak-invest/kiso](https://github.com/oak-invest/kiso) | 46 | 09-28 | OKF bundles → websites |
| [taikunudel/wiki-as-an-mcp](https://github.com/taikunudel/wiki-as-an-mcp) | 44 | 08-10 | wiki MCP on OKF |
| [longsizhuo/okf-frontmatter](https://github.com/longsizhuo/okf-frontmatter) | 43 | 06-30 | skill: maintain **repo docs** in OKF + lookup |
| [saschb2b/okf-studio](https://github.com/saschb2b/okf-studio) | 41 | 09-03 | desktop bundle reader |
| [stjbrown/agent-knowledge](https://github.com/stjbrown/agent-knowledge) | 40 | 08-01 | Agent Skills for OKF **project wikis** in repos |
| [jkroepke/okf-crossplane-v2](https://github.com/jkroepke/okf-crossplane-v2) | 37 | 09-25 | LLM-wiki content in OKF |
| [xSAVIKx/okf-skills](https://github.com/xSAVIKx/okf-skills) | 34 | 08-30 | agent skills |
| [aws-samples/sample-okf-llm-wiki](https://github.com/aws-samples/sample-okf-llm-wiki) | 31 | 09-29 | AWS data wiki → OKF |
| [DavidROliverBA/ai-xf-format](https://github.com/DavidROliverBA/ai-xf-format) | 30 | 09-27 | strict superset of OKF |
| [superops-team/okf](https://github.com/superops-team/okf) | 29 | 09-29 | project-level agent knowledge base |
| [W4G1/okf](https://github.com/W4G1/okf) | 26 | 09-04 | Rust impl + CLI |
| [gsemet/okf-schema](https://github.com/gsemet/okf-schema) | 22 | 09-18 | JSON Schema for OKF |
| [jeromeetienne/mnemo_wiki](https://github.com/jeromeetienne/mnemo_wiki) | 21 | 07-25 | LLM wiki in OKF |
| [Sudhakaran88/okf-conformance](https://github.com/Sudhakaran88/okf-conformance) | 16 | 08-12 | conformance checker |
| [MartinForReal/okf-enforcer](https://github.com/MartinForReal/okf-enforcer) | 15 | 08-14 | Obsidian OKF validator plugin |
| [nicholsn/lokf](https://github.com/nicholsn/lokf) | 14 | 09-29 | OKF markdown → queryable graph |
| [linyiru/awesome-okf](https://github.com/linyiru/awesome-okf) | 14 | 07-31 | awesome list |
| [evist0/okf-matt-skills](https://github.com/evist0/okf-matt-skills) | 13 | 07-08 | skills in OKF |
| [vishal-raaj-dnd/gemini-okf-compiler](https://github.com/vishal-raaj-dnd/gemini-okf-compiler) | 12 | 07-24 | PDF/DOCX → OKF |
| [vinodborole/okf-kit](https://github.com/vinodborole/okf-kit) | 11 | 07-15 | website → OKF, no LLM |
| [Arindam200/okfgen](https://github.com/Arindam200/okfgen) | 10 | 07-30 | LangChain generate/validate |
| [lars20070/code2okf](https://github.com/lars20070/code2okf) | 0 | 09-20 | **codebase → OKF** (code-repo relevant) |
| [opum-ai/lore-cli](https://github.com/opum-ai/lore-cli) | 0 | 09-29 | OKF-native repo-resident docs CLI |
| [btwld/wayfinder](https://github.com/btwld/wayfinder) | 0 | 09-28 | project knowledge: OKF validation, search, MCP |
| [travisjakel/okf-mcp](https://github.com/travisjakel/okf-mcp) | 6 | 09-22 | MCP consumer |
| [hdean-ssp/okf-mcp](https://github.com/hdean-ssp/okf-mcp) | 4 | 08-28 | MCP + CLI |
| [chris-page-gov/okf-explorer](https://github.com/chris-page-gov/okf-explorer) | 7 | 09-28 | registry/conformance for **federated** bundles (hub-relevant) |

Ecosystem is very young (all repos ≤4 months), many 0★ placeholders/spam in `okf` search; full raw list: search output in session artifact (not persisted).
