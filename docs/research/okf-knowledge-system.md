# Knowledge graph for coding agents on OKF: research report

Date: 2026-09-29. Status: research only; nothing is installed or adopted.
How the system is used day to day: [interaction and UX design](../design/okf-knowledge-system-ux.md).

## Recommendation

Store knowledge as plain [OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md)
files: one bundle per repo, plus one hub repo for knowledge that spans repos.
Each repo's maintainers choose where its bundle lives. It can be an `okf/`
folder on the code branches, or separate `okf/…` branches of the same repo that
keep the code branches free of knowledge files (section 5). Agents read and
write through one small CLI and a short skill we write ourselves. Humans check
the work through ordinary git review.

- **CLI for agents and developers:** [okfcli](https://github.com/okfcli/okf)
  (binary name `okf`, v0.5.0). It is one static 5 MB binary with signed
  release archives, returns JSON by default, and was the only tool tested that
  met every speed target at 10,000 concepts.
- **CI gate:** the validator from [okf-skills](https://github.com/scaccogatto/okf-skills)
  (GitHub Action `scaccogatto/okf-skills@v1`). It was the most spec-faithful
  checker tested and the only one that accepts the spec's datetime form of
  `stale_after`.
- **Hub view:** built by copying each repo's bundle into `hub/repos/<name>/`.
  No tool follows symlinks, so a copy step is required. It took 2.9 s for 50
  repos (sketch: [evidence/assemble-hub.sh](okf-knowledge-system/evidence/assemble-hub.sh)).
- **Branch mode:** a setup script checks the paired knowledge branch out at
  `okf/` and installs two local git hooks. They switch it with the working
  branch and delete it with the working branch, unless that would lose work
  (sketch, tested on scratch repos:
  [evidence/okf-branch-setup.sh](okf-knowledge-system/evidence/okf-branch-setup.sh)).
- **Agent skill:** about one page, written by us, adapting the published skill
  text of okf-skills (MIT) and the okf gem (Apache-2.0). Section 6 has the outline.

Main risks:

- **Young tools.** Every OKF tool is at most four months old, and okfcli has
  2 contributors. The format is the durable part; the skill should depend on
  only six okfcli commands so the CLI can be swapped.
- **One adoption blocker.** okfcli rejects `stale_after` written as the spec
  requires (a datetime), filed upstream as [okfcli#34](https://github.com/okfcli/okf/issues/34).
  It must be fixed upstream or patched in a pinned fork before rollout.
- **Assembly step.** The hub's view of the repos is only as fresh as its last
  assembly, so cross-repo answers can lag the repos by one CI run.
- **Branch mode has more moving parts.** Knowledge changes in a second pull
  request next to the code one. Its merge and cleanup on the remote depend on a
  sync job in the hub, which is not built yet. Its local switching depends on
  hooks that repos with a hook manager (`core.hooksPath`) must add by hand.

## 1. What was tested and how

Every tool was installed into a scratch directory and run against the same
inputs. The full protocol is in [evidence/protocol.md](okf-knowledge-system/evidence/protocol.md).

- **Small test setup:** three repo bundles (`billing-api`, `web-app`,
  `shared-auth`) plus a hub. It has these planted cases:
  - a stale concept and a draft concept;
  - a deprecated concept;
  - human-reviewed and machine-confirmed concepts;
  - footnotes tied to `sources`;
  - frontmatter in repo `index.md` files;
  - every link form: in-bundle relative, root-relative `/…`, cross-repo
    `../../<repo>/…`, and full URL.
- **Scale set:** 50 bundles × 200 concepts (10,050 files, 40 MB). About 10% of
  concepts carry a cross-repo link, and one unique string exists for checking
  search results.
- **Two hub variants:** symlinks to each repo's `okf/`, and plain copies.
- **Timing:** 1 warmup run, then 5 runs with `/usr/bin/time`; medians are
  reported. The machine has 12 cores. For comparison, `grep -rl` found the
  unique string in 0.06 s.
- **Tests T1–T10:** install, validate, symlinks, link resolution, cross-repo
  backlinks, search, agent write path, move/rename, multi-bundle use without a
  hub, and scale.

Tools run by hand:

| Tool | Version | Language |
|---|---|---|
| okfcli (`okf`) | v0.5.0 | Go |
| okfctl | v0.4.0 | Go |
| okf-mcp | 0.9.1 | Node |
| Open Knowledge (`okn`) | v0.13.0 | Go |
| okf-skills | v0.10.0 (8e31878) | Python via uv |
| okf gem | 2.2.0 | Ruby |

Reviewed from documentation only: Google's reference agent, Basic Memory,
the llm-wiki omp extension, and about 30 smaller OKF projects
([evidence/desk.md](okf-knowledge-system/evidence/desk.md)).

## 2. Comparison

### Install and maturity

| Tool | Install | Telemetry | Stars / contributors | Last release |
|---|---|---|---|---|
| okfcli | release tarball or brew; 5 MB static | none seen | 25 / 2 | v0.5.0, 2026-09-03 |
| okfctl | install.sh, brew, go install; 15 MB + plugin | none | 8 / 2 | v0.4.0, 2026-08-19 |
| okf-mcp | npm, Node ≥ 22; 20 MB | none | 7 / 1 | v0.9.1, 2026-09-19 |
| okn | release binary; 23 MB | **on by default** | 59 / 4 | v0.13.0, 2026-08-31 |
| okf-skills | uv + Python ≥ 3.11 | none (installer has some) | 401 / n/a | v0.10.0, 2026-09-28 |
| okf gem | `gem install okf`, needs Ruby | none | 169 / n/a | 2.2.0 |

Notes:

- `go install` of okfcli or okfctl downloads a newer Go toolchain first (Go
  ≥ 1.27 and ≥ 1.26.6). Use the release binaries instead.
- okfcli, okf-mcp and the okf gem all install a binary called `okf`.
- okn's `--no-telemetry` flag did not persist its opt-out when `CI` was set
  in the environment. `DO_NOT_TRACK=1` stopped all network attempts
  ([evidence/okn.md](okf-knowledge-system/evidence/okn.md)).

### Test results

Legend: ✅ pass, 🟡 partial, ❌ fail. "copy" means it passed only on the copied hub.

| Test | okfcli | okfctl | okf-mcp | okn | okf-skills | okf gem |
|---|---|---|---|---|---|---|
| T1 install | ✅ | ✅ | ✅ | 🟡 telemetry | 🟡 needs uv | ✅ |
| T2 validate | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 |
| T3 symlinked hub | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| T4 link resolution | ✅ copy | 🟡 | ✅ copy | ✅ | ✅ | ✅ |
| T5 cross-repo backlinks | ✅ copy | ✅ copy | ✅ copy | ✅ | ✅ | ✅ copy |
| T6 search for agents | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 |
| T7 agent write path | 🟡 | 🟡 | 🟡 MCP only | 🟡 | 🟡 | 🟡 |
| T8 move rewrites links | ❌ | ✅ copy | ❌ | ❌ | ❌ | ❌ |
| T9 multi-bundle without hub | ❌ | ❌ | 🟡 strong | 🟡 search only | ❌ | 🟡 search only |
| T10 scale | ✅ | 🟡 | 🟡 | 🟡 | 🟡 | 🟡 |

### Speed at 10,000 concepts (medians)

Targets: search < 2 s, backlinks < 2 s, validate < 10 s.

| Tool | Validate | Search | Backlinks | Other |
|---|---|---|---|---|
| **okfcli** | **0.61 s** | **0.37 s** | **0.34 s** | index build 1.0 s |
| okfctl | 0.38 s | 0.43 s | 0.58 s¹ | `lint` 100 s (grows quadratically); `analyze` uses 2.15 GB |
| okf-mcp | 1.80 s | 2.91 s | 1.87 s | a warm MCP server answers in 14 ms after a 1.9 s start |
| okn | 1.33 s | 5.92 s | 1.55 s¹ | search uses 1.3 GB and keeps no index between runs |
| okf-skills | 8.42 s | 7.42 s² | 11.45 s² | re-reads the whole bundle on every call |
| okf gem | 4.08 s | 2.14 s | 4.42 s¹ | `search @all` over 50 registered bundles: 1.97 s |

¹ No backlinks command; timed as a graph export filtered for inbound edges.
² Only available through its MCP server; includes the 1.8 s server start.

Every result was correct on the copied hub: one hit for the unique string, and
backlinks matching an independent `grep`.

### How each tool handles the trust and freshness fields

| Tool | `verified` → trust level | `stale_after` as a spec datetime |
|---|---|---|
| okfcli | in `list` and `show`, not in search hits | **rejected as an error** ([#34](https://github.com/okfcli/okf/issues/34)) |
| okfctl | not shown anywhere | ignored |
| okf-mcp | in every search hit | rejected; it also refuses to update such concepts |
| okn | in `list --json` | flagged as stale in `list` and `audit` |
| okf-skills | only in the HTML graph viewer | accepted; staleness shown only in the HTML graph viewer, never by the validator |
| okf gem | in `lint` stats and `catalog` | rejected |

The spec (§5) requires every timestamp to be an ISO 8601 datetime with an
explicit UTC offset. Tools that accept only `YYYY-MM-DD` are behind the spec,
not ahead of it.

## 3. Findings that shape the design

1. **No tool follows symlinks.** Four tools report fewer concepts on a
   symlinked hub and still exit 0: okfcli, okfctl, okf-skills and the okf gem.
   This silent pass is the most dangerous failure seen. okn refuses symlinks
   outright; okf-mcp skips them. The hub must hold real copies.
2. **Links that start with `/` break inside the hub.** `/` means the bundle
   root. When a repo bundle is copied to `hub/repos/<name>/`, its root-relative
   links point at the hub root instead. Every tool agreed on this. Repo
   bundles must use relative links only.
3. **Frontmatter in a repo's root `index.md` becomes an error in the hub.**
   The spec allows frontmatter only in the bundle-root `index.md`. Once copied
   into the hub, each repo's `okf_version` line is a spec violation. All six
   tools reported it. The assembly step removes it.
4. **The spec has no way to link between bundles.** Only okf-mcp resolves
   links across bundles, using its own `okf://<bundle>/<id>` typed relations.
   GitHub, Obsidian and the other five tools do not understand them. okn and
   the okf gem can search several bundles at once but cannot follow links
   between them. Cross-repo edges therefore belong in one bundle that can see
   every repo: the assembled hub.
5. **Code identifiers can defeat search.** okf-mcp stores `` `Idempotency-Key` ``
   with the backtick attached, so a search for `idempotency` misses the API
   contract. okfcli matches plain substrings and found it. Coding knowledge is
   full of backticked names, so this should be checked for any future search
   tool.
6. **Plain file edits make the cleanest diffs.** Tool-driven writes added
   noise:
   - okfctl `node new` adds non-spec `created` and `modified` keys;
   - okf-mcp and okfctl rewrite YAML flow style (a one-tag change became a
     17-line diff);
   - the okf gem's library save deleted `.git`.

   Direct file writes, then `okf index` and `okf validate`, gave 1–3 file
   diffs. Adding a `verified` line was a one-line diff.
7. **No tool can tell who added a `verified: human:` stamp.** Every tool
   derives trust levels from the `verified` field, but an agent can write
   `human:alice` itself. The spec calls trust levels advisory. Git review is
   the only real check.
8. **Moving a file safely needs okfctl.** Only `okfctl node mv` rewrites
   inbound links. Inside a copied hub it also rewrote links from other repos.
   Side effects: it regenerated 18 index and log files and removed the repo's
   `okf_version`. Under the section 5 conventions, the only cross-repo links
   are in hub concepts. So a move is `okfctl node mv` in the repo, then fixing
   the hub links that the next assembly reports as broken. Moves should be
   rare, so this can stay a manual step.

## 4. Prototype of the recommended layout

The small test setup was re-run with the conventions from section 5:

- relative links only inside repo bundles;
- the web-app → billing-api dependency moved into a hub concept;
- the hub assembled by copying, with repo `index.md` frontmatter removed.

Results:

| Check | Result |
|---|---|
| okf-skills `--strict` on 3 repo bundles and the hub | all "conformant — no issues" |
| okfcli `validate` on the same | clean, except the known `stale_after` bug (#34) |
| okfcli `backlinks` to `repos/billing-api/contracts/invoice-api` in the hub | 3 correct inbound links, including the cross-repo edge from `cross-repo/invoice-dependency` |
| okfcli `list` in the hub | correct `status` and trust level for all 9 concepts |
| Copy 50 repos (10,000 files) into a fresh hub | 2.87 s; re-copy with no changes 2.63 s |
| okfcli on the assembled scale hub | validate 0.59 s; search 0.35 s with the one correct hit |

Branch mode was tested afterwards on scratch repos with git 2.53.0, using the
setup script ([evidence/branch-mode.md](okf-knowledge-system/evidence/branch-mode.md)).
The code branch stayed clean: `main` held only its code, and `git status` was
empty. `okf/` followed every branch checkout, including of a teammate's pushed
branch. okfcli read the `okf/` worktree normally. Deleting a code branch
deleted its knowledge branch, except when the knowledge was never pushed or
`okf/` held uncommitted work on it; then the hook kept it. Renaming a code
branch looks like a delete to the hook, so the knowledge branch must be
renamed first. A symlinked `okf/` pointing into a hub checkout was rejected:
okfcli read 0 concepts through it and still reported `"valid": true`.

## 5. Proposed conventions

### Layout

Each repo's bundle lives in one of two places, chosen per repo:

- **Folder mode:** an `okf/` folder on the code branches, changed in the same
  pull request as the code. This is the simpler mode and the default.
- **Branch mode:** separate `okf/…` branches of the same repo, for maintainers
  who want no knowledge files on their code branches. Knowledge changes in a
  paired pull request next to the code one. See "Branch mode" below.

Either way, developers and agents find the bundle at `<repo>/okf/`, so every
`okf` command and skill step is the same in both modes.

```
<each repo>/okf/                  # the bundle: a folder (folder mode) or a worktree (branch mode)
  index.md                        # root: frontmatter `okf_version: "0.2"`; generated by `okf index`
  overview.md                     # type: Overview — hand-written prose (index.md is regenerated)
  services/ modules/ contracts/ decisions/ gotchas/ runbooks/
  log.md                          # lifecycle events only (created, deprecated, verified)

knowledge-hub/                    # its own repo
  index.md
  cross-repo/                     # type: Cross-Repo Dependency — the only place cross-repo edges live
  decisions/                      # decisions that span repos
  glossary/
  repos/                          # generated by assembly, git-ignored, never edited by hand
  repos.txt                       # one line per repo: <name> <git URL> <folder|branch>
```

### Branch mode

Branches, all in the code repo:

| Branch | Holds | Pairs with |
|---|---|---|
| `okf/main` | The bundle at the branch root; an orphan branch that shares no history with the code | The default branch, whatever it is called |
| `okf/<b>` | The bundle as changed for the work on `<b>`; created from `origin/okf/main` on first checkout | Code branch `<b>` |

The trunk cannot be named plain `okf`: git then refuses to create
`okf/feat/x`. Code branch names must not start with `okf/`. The setup script
assumes the remote is called `origin`.

On each machine, [okf-branch-setup.sh](okf-knowledge-system/evidence/okf-branch-setup.sh)
does the local part:

- checks the knowledge branch out as a git worktree at `okf/`, and adds
  `/okf/` to `.git/info/exclude` so the code branches never see it. The
  leading slash keeps code folders such as `cmd/okf/` visible.
- installs two hooks in `.git/hooks/`, which git never commits:
  - `post-checkout` switches `okf/` to the paired knowledge branch on every
    branch checkout. It creates that branch from the remote copy if a
    teammate pushed one, else from `origin/okf/main`. A detached HEAD leaves
    `okf/` where it was.
  - `reference-transaction` deletes `okf/<b>` when `<b>` is deleted locally.
    It keeps the branch, with a message, when `okf/<b>` has commits no remote
    branch contains, or when `okf/` has uncommitted changes on it. It never
    deletes anything on the remote. git has no rename event, so
    `git branch -m` looks like a delete: rename `okf/<old>` to `okf/<new>`
    first.
- refuses to run if `core.hooksPath` is set (husky, lefthook). Those repos add
  the two hooks to their hook manager by hand.

Fresh clones in CI and cloud agents have no `okf/` worktree and no hooks. The
skill must create the worktree itself (section 6). Such agents also lack
developers' user-level instructions, so they learn that a repo uses branch
mode only from organisation-level agent instructions. That is an open gap
(section 8).

Review and merge:

- The agent pushes `<b>` first, then `okf/<b>`, and opens a knowledge pull
  request from `okf/<b>` into `okf/main`. The code pull request links to it.
  Reviewers add `verified` stamps there.
- `okf/main` is protected like a code branch: changes arrive only through pull
  requests. The one exception is the check job's `verified: process:` commits,
  as in folder mode.
- Knowledge pull requests merge with a merge commit, never a squash, so git
  ancestry shows whether everything on `okf/<b>` has reached `okf/main`.
- A workflow on `okf/main` (`.github/workflows/okf.yml`) runs the strict
  validator and index check on knowledge pull requests. okfcli and the
  okf-skills validator both ignore the `.github/` folder at the bundle root.
  [INFERENCE: that GitHub runs a workflow stored only on `okf/main` for pull
  requests into it was not tested.]

A sync job in the hub merges and cleans up on the remote. Local hooks cannot
see branches deleted on GitHub, and a workflow triggered by a merge would
have to live on the code branches. The job runs on a schedule for every
branch-mode repo in `repos.txt`. For each remote `okf/<b>` it looks at the
latest pull request from `<b>`, ignoring pull requests from forks:

| Code pull request from `<b>` | State of `okf/<b>` | Action |
|---|---|---|
| Open, or closed or missing while `<b>` is still on the remote | Any | Nothing |
| Merged | Knowledge pull request open, approved, checks green | Merge it with a merge commit |
| Merged | Knowledge pull request open, but not approved, failing, or conflicting | Comment on both pull requests; try again next run |
| Merged | Commits not on `okf/main`, and no open knowledge pull request | Comment on the code pull request asking for one |
| Merged, and `<b>` gone | Everything already on `okf/main` | Delete `okf/<b>` |
| Closed without merging, and `<b>` gone | Any | Close the knowledge pull request; delete `okf/<b>` |
| None, `<b>` not on the remote, and the last commit on `okf/<b>` older than 7 days | Any | Close the knowledge pull request; delete `okf/<b>` |

It asks GitHub whether the code pull request merged. Git history cannot tell,
because a squash merge leaves none of the branch's commits on the default
branch. The 7-day wait in the last row covers knowledge pushed before its code
branch. The job never merges knowledge nobody approved, so unreviewed
knowledge waits, where in folder mode it would merge with the code. The job
needs write access to every branch-mode repo.

### Concept types

| `type` | Lives in | Use for |
|---|---|---|
| `Overview` | repo | What the repo is; entry point for agents |
| `Service` / `Module` | repo | A deployable service or a significant code unit |
| `API Contract` | repo | A public interface; `resource` points at the OpenAPI/proto file |
| `Decision` | repo or hub | Architecture decisions and why |
| `Gotcha` | repo | Non-obvious behavior that caused or would cause a bug |
| `Runbook` | repo | Operational steps |
| `Cross-Repo Dependency` | hub | Repo A relies on concept B in repo C, and how |
| `Glossary Term` | hub | Shared vocabulary |

The spec does not register types. This list is our own convention and can grow.

### Links

- Inside a repo bundle, use relative links only (`../contracts/x.md`),
  never `/…`. Finding 2 is the reason.
- A repo bundle never links to another repo's files. It may include a full
  URL to the other repo on GitHub for human readers. Tools treat those as
  external links.
- Cross-repo edges are hub concepts that link to `/repos/<name>/…`. They
  resolve only in the assembled hub, which is where agents run cross-repo
  queries.

### Stamps

- `generated: { by: <agent>/<model>, at: <datetime> }` on every agent edit,
  for example `claude-code/opus-5` or `cursor/gpt-5.6`.
- `verified` entries with `human:<github-login>` are added only by that
  person, in their own commit during review.
- `stale_after` is required on `API Contract` and `Gotcha`. The default is 180
  days after `generated.at`, written as a UTC datetime. It can be enforced once
  okfcli#34 is fixed.
- Deprecate with `status: deprecated` instead of deleting, so inbound links
  keep resolving.

### Hub assembly and CI

- The hub CI clones each repo in `repos.txt`. It fetches only the `okf/`
  folder of the default branch in folder mode, and only the `okf/main` branch
  in branch mode. It copies each bundle into `repos/<name>/`, removes the root
  `index.md` frontmatter, then runs the strict validator and reports broken
  cross-repo links. Run it nightly, and on demand from repo pipelines.
- Knowledge on `okf/<b>` branches is not assembled. Cross-repo views contain
  only knowledge that has reached the default branch or `okf/main`.
- Each repo's CI runs the okf-skills validator with `--strict` on `okf/`. In
  branch mode, the workflow on `okf/main` runs the same checks.
- Developers who want cross-repo queries locally run the same assembly
  script. It fetches from each remote in `repos.txt`, so no local checkouts
  are needed; the prototype script still reads a workspace directory.

## 6. Agent skill outline

One `SKILL.md`, readable by Claude Code, Cursor and omp. It uses only these
commands: `okf search`, `okf show`, `okf list`, `okf backlinks`, `okf index`
and `okf validate`.

1. **When it applies:** any task in a repo that has `okf/`, or whose remote
   has an `okf/main` branch, or any question about how repos relate.
2. **Before starting work:**
   - in branch mode, make sure `okf/` exists and is on the paired branch:
     `okf/<b>` for code branch `<b>`, `okf/main` on the default branch. In a
     fresh clone, run the setup script first; it also hides `okf/` from the
     code branches, so a later `git add -A` cannot pick it up. Switch the
     branch if it does not match. The hooks are missing in CI, in cloud
     agents and in repos with a hook manager.
   - read `okf/index.md` and `overview.md`;
   - run `okf search okf --text <terms>`;
   - for cross-repo questions, run `okf search` and `okf backlinks` on the
     local assembled hub.
   - Treat anything draft, deprecated, past `stale_after`, or unverified as
     "confirm against the code before relying on it".
3. **While working:** if the code contradicts a concept, fix the concept in
   the same change. If code is not available, set `status: draft` and say why.
4. **Before finishing:** record new knowledge by editing files directly. Use
   one file per concept and set the `generated` stamp. Then run
   `okf index okf` and `okf validate okf`.
   - In branch mode, commit inside `okf/`, push `<b>` and then `okf/<b>`, and
     open a knowledge pull request into `okf/main` linked from the code pull
     request.
   - Cross-repo facts go to the hub repo as a separate change.
   - Agents never add a `verified` entry with a `human:` actor.
5. **What not to record:** anything the code already states plainly, copied
   code, or secrets. Knowledge should explain why, how things connect, and
   what is surprising.

## 7. Alternatives considered

| Option | Why not chosen now | When it would win |
|---|---|---|
| One central knowledge repo with no per-repo bundles | Knowledge could no longer change next to the code, in the same or a paired pull request. Branch mode already serves maintainers who want no knowledge files on their code branches. | If maintainers refuse even `okf/…` branches in their repo, or the assembly step proves costly or stale in practice; links then need no assembly |
| One shared `okf` branch in each repo, not paired with code branches | Knowledge for unmerged work would land on the trunk before its code, or wait until after the merge | If the local hooks prove unworkable |
| A git-ignored `okf/` folder, or a symlink into a hub checkout | An ignored folder is never shared. okfcli read 0 concepts through a symlink and still reported `"valid": true` ([evidence/branch-mode.md](okf-knowledge-system/evidence/branch-mode.md)). | Never |
| okf-mcp federation (`okf.project.yaml`, `okf://` typed relations) | Its cross-bundle links are not standard OKF; cold CLI search is 2.9 s; it writes only through MCP; it rejects datetime `stale_after` | If an MCP server becomes acceptable and links between bundles without a hub are needed |
| okf gem as the core CLI | Needs Ruby; backlinks 4.4 s; rejects datetime `stale_after` | If Ruby is available everywhere: it has the best lint (25 checks) and a ready-made skill |
| okfctl as the core CLI | No trust output; `lint` takes 100 s at 10,000 concepts | Keep it as an occasional tool for `node mv` |
| okn | Telemetry on by default; 5.9 s search at 10,000 concepts | If the telemetry defaults change and it gains a persistent index |
| Basic Memory | Not OKF: wikilinks by title, no `verified` field, SQLite index, AGPL-3.0 | If human verification and portability matter less than memory-style recall |
| llm-wiki (omp extension) | Works only in omp; no human verification workflow | Personal notes inside omp; it has an OKF v0.2 mode |
| Google reference agent | Reads only BigQuery; tied to Gemini and GCP | Never for code repos |
| MCP instead of CLI | Needs setup in every client, and the CLI already meets the speed targets | If an agent without a shell must use the knowledge |

**Trade-offs.** The recommendation depends most on okfcli staying maintained
and fixing #34. The first failure would be a spec-conformant `stale_after`
turning every validate run red. The mitigation is to pin a patched fork. The
date parsing sits in `internal/validate/v02.go:39` and needs only a small
change there to accept datetimes as well. The second
dependency is the hub assembly staying cheap. At 50 repos it takes 3 s, so
the likely failure is staleness between assembly runs, not cost. The third,
for branch-mode repos only, is the hub's sync job. If it stops, knowledge
pull requests stay unmerged after their code merges, and `okf/<b>` branches
pile up on the remote.

**Better approaches.** None tested beat the chosen shape. The okf gem's
`search @all` covers several bundles without a hub, but it cannot follow links
between them, and those links are the point of the hub. A central repo is
simpler, but it moves knowledge away from the code. It remains the fallback
for repos that refuse even branch mode. Switching costs little: move the
folders and run `okfctl node mv` to rewrite links. Moving a repo from folder
mode to branch mode should also be cheap: remove `okf/` from the code
branches, then copy it to the root of a new `okf/main` branch. Links inside a
bundle are relative, so they stay valid. [INFERENCE: not tested.]

## 8. Before adoption

1. Get okfcli#34 fixed upstream, or pin a patched fork. Consider also
   [okfcli#35](https://github.com/okfcli/okf/issues/35): `show` drops our own
   frontmatter keys.
2. Write the skill (section 6) and the assembly script for real. The prototype
   script is a sketch.
3. For branch mode: build the hub's sync job, check on GitHub that a workflow
   stored only on `okf/main` runs for pull requests into it, harden the setup
   script (tested only on scratch repos), and agree the sync job's write
   access with each repo's maintainers. Decide how CI and cloud agents learn
   that a repo uses branch mode; organisation-level agent instructions are
   the likely place.
4. Pilot with 2–3 repos, at least one in each mode, and measure how often
   agents write knowledge, how often reviewers correct it, and how often stale
   concepts are hit.

## Evidence

Full logs, with commands and real output, for each tool:
[okfcli](okf-knowledge-system/evidence/okfcli.md),
[okfctl](okf-knowledge-system/evidence/okfctl.md),
[okf-mcp](okf-knowledge-system/evidence/okf-mcp.md),
[okn](okf-knowledge-system/evidence/okn.md),
[okf-skills](okf-knowledge-system/evidence/okf-skills.md),
[okf gem](okf-knowledge-system/evidence/okf-gem.md),
[desk research and maturity data](okf-knowledge-system/evidence/desk.md),
[test protocol](okf-knowledge-system/evidence/protocol.md),
[branch mode](okf-knowledge-system/evidence/branch-mode.md).
The test data was generated under `/tmp/okf-research/`, and the protocol
describes it.
