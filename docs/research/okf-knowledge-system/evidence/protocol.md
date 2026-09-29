# OKF tool evaluation protocol

Goal: evaluate OKF v0.2 tooling for an agent-curated knowledge system spanning
10–50+ code repositories. Layout under test ("hybrid"): each repo owns an OKF
bundle at `<repo>/okf/`; a central **hub** bundle holds cross-repo relations and
assembles the repo bundles under `hub/repos/<name>/`.

## Shared inputs (READ-ONLY — never modify)

- `/tmp/okf-research/fixture/workspace/`
  - `billing-api/okf/`, `web-app/okf/`, `shared-auth/okf/` — standalone repo bundles
  - `hub/` — hub bundle; `hub/repos/<name>` are **relative symlinks** to `../../<name>/okf`
- `/tmp/okf-research/fixture/hub-copy/` — same hub with repo bundles copied (no symlinks)
- `/tmp/okf-research/scale/repos/rNN/okf/` — 50 bundles x 200 concepts (10,050 .md files incl. index)
- `/tmp/okf-research/scale/hub/` (symlinks) and `/tmp/okf-research/scale/hub-copy/` (copies)
  - needle: the string `zebracorn-needle` exists only in `repos/r37/okf/d3/c123.md`
  - ~10% of concepts carry a cross-repo link `../../rNN/dX/cYYY.md` that resolves only inside the hub

Copy anything you need to mutate into your own work dir with `cp -a` (keeps relative symlinks).

## Planted facts in the fixture (expected results)

| Item | Expected |
|---|---|
| `billing-api/okf/gotchas/idempotency-key.md` | `status: draft`, `stale_after: 2026-09-01` → **stale** (today 2026-09-29), unverified |
| same file, link `/contracts/invoice-api.md` | root-relative: resolves standalone; **broken inside hub** (hub root differs) |
| `billing-api/okf/contracts/invoice-api.md` | machine-confirmed (`process:` verifier) |
| `billing-api/okf/services/billing-api.md` | human-reviewed; footnote `[^main-go]` keyed to `sources[].id` |
| `web-app/okf/services/web-app.md` | has 1 URL cross-repo link (external), 2 hub-relative `../../<repo>/...` links: **broken standalone, valid in hub** |
| `shared-auth/okf/concepts/legacy-api-key.md` | `status: deprecated` |
| `hub/cross-repo/invoice-dependency.md` | links `/repos/billing-api/...`, `/repos/web-app/...` (valid in hub) |
| `hub/repos/*/index.md` | carry `okf_version` frontmatter; spec §8/§12 permits frontmatter only in the **bundle-root** index.md → a strict validator may flag these inside the hub |
| Inbound links to `billing-api` `contracts/invoice-api` (hub view) | from `repos/billing-api/services/billing-api`, `repos/web-app/services/web-app`, `cross-repo/invoice-dependency` (idempotency-key's link is broken in hub) |

## Test matrix (run every applicable test; record exact commands + trimmed real output)

- **T1 Install**: command, wall time, binary/package size, runtime deps, network calls/telemetry at runtime, license.
- **T2 Validate**: each standalone repo bundle, `workspace/hub` (symlink) and `hub-copy`. Errors vs warnings; exit codes. Did it catch: stale concept, broken links, nested index frontmatter, footnote/source id join?
- **T3 Symlinks**: does the symlinked hub see the same concepts as `hub-copy`?
- **T4 Link resolution**: which of the planted link forms resolve, per view.
- **T5 Backlinks across repos**: inbound links to `repos/billing-api/contracts/invoice-api` in hub view.
- **T6 Search/retrieval for agents**: `idempotency` across hub; is output JSON/machine-readable; does it surface trust tier, status, staleness? Progressive disclosure (index generation)?
- **T7 Agent write path**: create a concept via the tool (if supported) with `generated` stamp; add `verified: human:<id>`; regenerate index.md; append log.md. Are unknown frontmatter keys preserved? Does the diff stay small and reviewable?
- **T8 Move/rename**: move a concept; are inbound links (including from other repos in the hub) rewritten?
- **T9 Multi-bundle without hub**: native federation (project/workspace/registry) across the 3 standalone bundles — does search/backlinks span bundles, how are cross-bundle links expressed?
- **T10 Scale** (use `hyperfine --warmup 1 --runs 5` where possible): on `scale/hub-copy` and `scale/hub`: full validate; search `zebracorn-needle` (correct single hit?); backlinks for `repos/r37/d3/c123`; index generation time (on a `cp -a` copy). Also: loop over 50 standalone bundles if the tool cannot do the hub. Targets: search < 2 s, backlinks < 2 s, validate < 10 s.

## Output

Write the full log to `/tmp/okf-research/results/<tool>.md`: one section per test with commands, trimmed real output, and a PASS/FAIL/PARTIAL/N-A verdict. End with: strengths, blockers, surprises, and fit for the hybrid layout. Never fabricate output; mark anything not observed as `[not run]` with the reason.
