# How the OKF knowledge system works: interactions and UX

Date: 2026-09-29. Status: design. It builds on the
[research report](../research/okf-knowledge-system.md). The tooling it
describes is built into compass as one binary, `compass`, and tested; the
pilot on real repos has not started.

## Gist

The system adds no new app or server. People and agents use tools they already
have: markdown files in each repo, one CLI (`okf`), one agent skill, pull
request review, and CI. It runs as a loop:

1. **Read.** Before a task, the agent reads what the repo knows.
2. **Work and write.** The agent records what it learned next to the code. In
   folder mode that is the same pull request as the code. In branch mode, for
   maintainers who want no knowledge files on their code branches, it is a
   paired knowledge pull request (section 4.9).
3. **Review.** A human checks that knowledge in the normal diff, or in the
   linked knowledge pull request in branch mode, and can mark it verified.
4. **Assemble.** `compass hub assemble` copies every repo's knowledge into a
   local view of your hub, so cross-repo questions can be answered, and
   `compass hub check` reports broken cross-repo links. Agents run both before
   a cross-repo question or a hub pull request; reviewers run them before
   approving a hub pull request. The hub has no CI.

The main UX risks:

- Search results do not show trust or staleness, so the agent needs a second call.
- The hub view is only as fresh as its last local assembly, and each assembly
  fetches every repo. Nothing reports a broken cross-repo link until someone
  assembles.
- Nothing stops an agent from writing a fake "human verified" stamp. Only review catches it.
- Branch mode depends on two local git hooks and on a hub owner running
  `compass sync`. An agent without the hooks must pair `okf/` with its branch
  itself, and until someone runs `compass sync`, knowledge pull requests stay
  open after their code merges.
  `git pull` does not update `okf/`, so the agent pulls it before reading.
- Upstream okfcli rejects the datetime `stale_after` this design uses
  (okfcli#34). Compass pins a patched fork, `tuan-nng/okf` release
  `v0.5.0-tuan-nng.1`, which accepts it in `okf validate` and reports
  `stale: true` in `okf show` once it passes. `compass setup` installs it. The
  same patch is open upstream as okfcli#39; we drop the fork once upstream
  releases a fix.

Command output in this document is real. It comes from okfcli v0.5.0, or from
the pinned fork `v0.5.0-tuan-nng.1` for `okf show` in section 4.1, run on
the test data in compass: `testdata/fixture/`, rebuilt from the research
protocol's planted facts, and, for hub queries, `testdata/proto/`, the same
data with the report's conventions, assembled into a hub.
`test/check-fixture.sh` checks both sets. Branch-mode output comes from
scratch repos ([evidence](../research/okf-knowledge-system/evidence/branch-mode.md)).
The skill, `compass hub assemble`, `compass hub check`, `compass branch setup`,
the stamper (`compass stamp`) and `compass sync` are built and tested. The
stamper and `compass sync` have written nothing live yet: their tests use
recorded GitHub responses, and their live runs so far were `--dry-run`.
Section 9 lists what does not exist yet.

## 1. The original problem

The research set out to give coding agents a knowledge system that works
across 10–50+ repos. The research contract was only partly retrievable, so
this list is rebuilt from the report and its test protocol. It has five needs:

| # | Need | What goes wrong today |
|---|---|---|
| N1 | Agents start each task knowing why the code is the way it is | Every session starts blank. The agent re-derives knowledge or repeats a known mistake, such as retrying an invoice call without an idempotency key. |
| N2 | Agents can update that knowledge as a normal part of their work | What agents learn stays in chat logs and is lost |
| N3 | Humans can check agent-written knowledge cheaply | Agent notes are either trusted blindly or ignored |
| N4 | Knowledge covers how repos depend on each other | A change in one repo breaks another, and no single repo records the link |
| N5 | Knowledge stays current and portable across agents and tools | Notes rot, and a tool-specific store locks the team in |

Section 7 maps each need to the part of the system that meets it.

## 2. Who and what is involved

People and machines:

| Actor | Role |
|---|---|
| Coding agent | Claude Code, Cursor or omp, working in one repo with the skill loaded |
| Developer | Works next to the agent and asks it questions |
| Reviewer | Reviews pull requests. Is the only one who adds `verified: human:<login>` stamps. |
| Repo CI | The only CI job. On every pull request, compass's `actions/bundle-check` builds `compass` and runs `compass bundle check`, which checks that the repo's bundle follows the format and that its index files are current. In branch mode this is a workflow stored on `okf/main` that runs on knowledge pull requests; GitHub runs it for pull requests into `okf/main` (tested on a scratch repo, 2026-10-06). |
| Hub owner | A person who owns your hub. Runs `compass stamp` and `compass sync` on their own machine, with their own GitHub token, on demand; daily during the pilot. Needs a ruleset bypass on each protected repo, because the stamper pushes straight to the default branch or `okf/main`. |
| `compass sync` | Branch mode only. Run by a hub owner. Once a code pull request merges, it merges the paired knowledge pull request if a human approved it and its checks pass. It deletes knowledge branches whose code branch is gone from the remote. |
| The stamper (`compass stamp`) | Run by a hub owner. For each row of your hub's `checks.txt`, it asks the GitHub API whether a named CI job, such as a contract test against the OpenAPI file, passed on the head of the repo's default branch. If it did, it commits a fresh `verified: process:<actor>` entry on each concept the row covers. It is the only writer of `process:` stamps. In branch mode it commits to `okf/main`. When branch protection refuses its push, it says so and exits non-zero. |

Each repo stores its bundle in one of two modes, chosen by its maintainers:

- **Folder mode:** `okf/` is a folder on the code branches.
- **Branch mode:** the bundle lives on its own branches of the same repo:
  `okf/main` pairs with the default branch and `okf/<b>` with code branch
  `<b>`. `okf/` is a local git worktree of the paired branch, hidden from the
  code branches (section 4.9).

In both modes the bundle is at `<repo>/okf/`, so every `okf` command below
works unchanged.

Artifacts:

| Artifact | Where | Written by |
|---|---|---|
| Repo bundle | `<repo>/okf/`: one markdown file per concept | Agents and developers, in code pull requests (folder mode) or paired knowledge pull requests (branch mode) |
| Your hub | A repo your team makes from compass's `templates/hub/`, such as `knowledge-hub/`: `repos.txt`, `checks.txt`, cross-repo dependencies, shared decisions, glossary. It holds no workflows and no credentials. | Agents and developers, in hub pull requests |
| Assembled hub view | `<your hub>/repos/<name>/`: copies of every repo bundle, ignored by git | `compass hub assemble` only, run locally |
| Skill | One `SKILL.md` that every agent loads; `compass setup` installs it | The platform team |
| Branch-mode setup | `compass branch setup`: creates the `okf/` worktree and installs two hooks in `.git/hooks/`, which git never commits. Needs git 2.28 or newer, or 2.42 for `--init`. | The platform team; each developer runs it once per clone |

```mermaid
flowchart LR
  subgraph RepoA["billing-api repo"]
    CA[code] --- KA[okf/ bundle]
  end
  subgraph RepoB["web-app repo"]
    CB[code] --- KB[okf/ bundle]
  end
  subgraph Hub["your hub repo"]
    X[cross-repo/ decisions/ glossary/]
    V["repos/ (assembled copies)"]
  end
  Agent((Coding agent)) -- "okf search / show / backlinks" --> KA
  Agent -- "edits files, same or paired PR" --> KA
  Agent -- "cross-repo questions" --> V
  Agent -- "separate hub PR" --> X
  Reviewer((Reviewer)) -- "reviews diff, adds verified" --> KA
  KA -- "assembly copies, strips index frontmatter" --> V
  KB -- "assembly copies" --> V
  X -- "links to repos/NAME/…" --> V
```

## 3. What each actor touches

| Actor | Uses | Never does |
|---|---|---|
| Coding agent | Six commands: `okf search`, `show`, `list`, `backlinks`, `index`, `validate`. Plain file edits. `compass hub assemble` and `compass hub check` for cross-repo work. In branch mode, `git -C okf` to check the paired branch, pull, commit, and push. | Adds a `verified` stamp, `human:` or `process:`; edits the hub's `repos/`; links from one repo bundle into another repo's files; pushes to `okf/main` directly |
| Developer | Reads the markdown on GitHub or in an editor. Asks the agent. Runs `compass hub assemble` locally for cross-repo work. In branch-mode repos, runs `compass branch setup` once per clone. | Needs to learn the CLI to benefit |
| Reviewer | The pull request diff, usually 1–3 knowledge files; in branch mode, the knowledge pull request linked from the code one. Adds one `verified` line in their own commit. | Approves a `verified: human:` line they did not write |
| Repo CI | `compass bundle check`: `compass validate`, the strict validator ported from okf-skills; then `okf index` on the bundle and a check that the committed index files did not change. The branch-mode workflow runs the same check on `.`, the bundle root. | Changes any files |
| Hub owner | `compass stamp` and `compass sync`, with their own GitHub token | Runs them from CI; stores a write key |
| `compass sync` | The GitHub API for pull request states; merges approved knowledge pull requests with a merge commit; comments when one is missing, unapproved or failing; deletes `okf/<b>` branches | Merges a knowledge pull request before its code pull request merges, or without a human approval; opens knowledge pull requests itself; touches code branches or deletes `okf/main` |
| The stamper | The result of the named CI job; one `verified` line per covered concept, in its own commit | Stamps a concept its row in `checks.txt` does not list |

**The UX choice: the interface is the file.** Tools that rewrite frontmatter
made noisy diffs; one tag change became a 17-line diff (report, finding 6).
Plain file edits kept every change to 1–3 files. So reviewers read knowledge
changes the way they read code.

## 4. Journeys

### 4.1 An agent starts a task in one repo

The task: "Add retries to the invoice client in billing-api."

1. The skill applies because the repo has an `okf/` folder, or, in branch
   mode, because its remote has an `okf/main` branch. Agents load a skill
   only when its description matches the task, so each repo's `AGENTS.md`
   (or `CLAUDE.md`) also has one line pointing at `okf/index.md` and the
   skill (section 4.8). A branch-mode repo cannot carry that line on its code
   branches, so it goes in each developer's user-level agent instructions
   instead (section 4.9). [INFERENCE: skill auto-loading was not tested.]
2. The agent reads `okf/index.md`, then `okf/overview.md`. The test data has no
   `overview.md`; section 4.8 adds one to every repo.
3. It searches for the task's key words:

   ```
   $ okf search okf --text idempotency
   "count": 3,
   "results": [
     {"id": "contracts/invoice-api",   "title": "Invoice API v2", "type": "API Contract"},
     {"id": "gotchas/idempotency-key", "title": "Invoice creation needs an idempotency key", "type": "Gotcha"},
     {"id": "services/billing-api",    "title": "Billing API", "type": "Service"}]
   ```

4. Search hits show only the id, title and type. One `okf list okf` call adds
   `status` and `trust_tier` for every concept in the bundle, but not
   staleness. So the agent opens each hit it will rely on:

   ```
   $ okf show okf gotchas/idempotency-key
   "description": "POST /v2/invoices double-charges on retry without Idempotency-Key.",
   "generated":   {"at": "2026-06-01T12:00:00Z", "by": "cursor/gpt-5.6"},
   "stale": true,
   "stale_after": "2026-09-01T00:00:00Z",
   "status": "draft",
   "trust_tier": "unverified"
   ```

5. The concept is a draft, unverified and stale. The agent treats it as a
   lead to check, not a fact (section 6). It reads the handler code and
   confirms that retries without the key double-charge.
6. The agent writes the retry code, and it sends the key.

This single read could prevent a double-charge bug. The agent still checked
the claim, because the concept's trust level told it to.

### 4.2 The agent records what it learned

The agent updates the knowledge next to the code: on the same branch in
folder mode, or on the paired knowledge branch `okf/<b>` in branch mode. The
steps are the same in both; section 4.9 adds the branch-mode commit and push.

1. It edits `okf/gotchas/idempotency-key.md`. It sets `status: stable`, adds
   `sources` pointing at the handler, and adds a new `generated` stamp and a
   new `stale_after`. If the concept had `verified` entries, it removes them,
   because a stamp covers only the text that was checked (section 5):

   ```yaml
   status: stable
   generated: { by: claude-code/opus-5, at: 2026-09-29T14:00:00Z }
   stale_after: 2027-03-28T14:00:00Z
   sources:
     - id: create-invoice
       resource: https://github.com/acme/billing-api/blob/main/internal/invoice/create.go
   ```

2. It changes the root-relative link `/contracts/invoice-api.md` to
   `../contracts/invoice-api.md`. Links starting with `/` break inside the hub
   (section 4.4).
3. If it learned something new, for example a retry budget, it writes a new
   file, `okf/gotchas/retry-budget.md`, with one concept in it.
4. It runs `okf index okf` and `okf validate okf`. Both pass on the edit
   above. Once the repo's index files exist (section 4.8), the research test
   of this path produced a 1–3 file diff (okfcli evidence, test T7).
5. It does not add `verified`. The pull request description lists the
   knowledge files it changed, so the reviewer can find them. In branch mode
   the code pull request links the knowledge pull request instead.

The index files are generated, so two pull requests often conflict in them.
In a test, conflicts appeared when both added concepts that sort next to each
other, when both created a folder, and when both edited concepts with adjacent
index rows. When a merge or rebase conflicts in an `index.md`, the agent takes
either side and re-runs `okf index okf`; that gave a correct index every time
([evidence](../research/okf-knowledge-system/evidence/index-files.md)).
In `log.md` it keeps both entries. Neither validator notices an out-of-date
index, so repo CI checks it (section 3).

What the agent must not record: facts the code already states plainly, copied
code, or secrets. Knowledge explains why, how things connect, and what
surprises people.

### 4.3 The reviewer checks and verifies

In folder mode:

```mermaid
sequenceDiagram
  participant A as Agent
  participant PR as Pull request
  participant CI as Repo CI
  participant R as Reviewer
  A->>PR: code + okf/ changes (generated stamp, no verified)
  PR->>CI: strict validator, index check
  CI-->>PR: pass / findings
  R->>PR: reads knowledge diff next to code diff
  alt claim is right
    R->>PR: own commit adds verified by human alice
  else claim is wrong or unsure
    R->>PR: request change, or leave unverified
  end
  PR->>PR: merge — knowledge and code land together
```

- The reviewer reads the knowledge change in the same diff as the code it
  describes. They do not need a second tool.
- Verifying adds one line. In the research test it produced a one-line diff,
  and the regenerated index files did not change.
- After merge, `okf list` and `okf show` report `trust_tier: human-reviewed`.
  Future agents treat the concept as reliable.
- A reviewer may merge the code and leave the knowledge unverified. Unverified
  knowledge is still useful. Agents just check it first.
- The proof of who added a `human:` stamp is the pull request, not the commit.
  A squash merge folds the reviewer's commit into one commit authored by the
  pull request author. So a later check should match the stamp's login
  against the reviewers who approved that pull request, not against commit
  authors.

In branch mode the knowledge arrives in its own pull request, from `okf/<b>`
into `okf/main`, linked from the code pull request. The reviewer reads both,
adds any `verified` line on `okf/<b>`, and approves the knowledge pull request.
Nobody merges it before the code. Once the code pull request merges, the next
`compass sync` run by a hub owner merges the knowledge pull request, but only
if a human approved it and its checks pass (section 4.9). Unlike folder mode,
knowledge nobody reviewed does not ride along with the code; it waits. The
knowledge pull request is the proof of who added a `human:` stamp.

### 4.4 An agent answers a cross-repo question

The task, in billing-api: "Change the response shape of POST /v2/invoices."
The agent first asks who depends on that contract.

1. A single repo cannot answer this, so the agent uses the assembled hub. The
   developer has your hub cloned, here at `~/src/knowledge-hub` (`compass setup`
   records the clone), and the agent refreshes its view:

   ```
   $ compass hub assemble ~/src/knowledge-hub
   ```

   It reads `repos.txt` and fetches only the bundle of each listed
   repo from its git remote: the `okf/` folder of the default branch in folder
   mode, or the `okf/main` branch in branch mode. It copies each one into
   `repos/<name>/` and removes frontmatter from the copied `index.md` files.
   No CI runs it; every developer and agent runs it locally. The developer
   needs read access to every listed repo, but no local checkout of any of
   them. It keeps a clone cache between runs. In a bench, it assembled 50
   repos (10,100 files) from local bare remotes in 3.7 s cold and 4.0 s warm
   (`test/bench-assemble.sh 50`). The warm number measured cold clones: a
   known cache bug (`internal/hub/assemble.go` compares the `insteadOf`-
   rewritten `git remote get-url origin` with the `repos.txt` URL) made the
   warm run re-clone, and the fix is scheduled before the folder-mode pilot.
   Fetch time from GitHub itself is not measured until the pilot.
2. It asks for backlinks, meaning every concept that links to this one:

   ```
   $ okf backlinks ~/src/knowledge-hub repos/billing-api/contracts/invoice-api
   "backlinks": [
     "cross-repo/invoice-dependency",
     "repos/billing-api/gotchas/idempotency-key",
     "repos/billing-api/services/billing-api"],
   "count": 3
   ```

3. Only `cross-repo/…` and `repos/<other repo>/…` entries are dependents. The
   other two are billing-api's own concepts. The one dependent here is
   `cross-repo/invoice-dependency`, a hub concept verified by `human:alice`.
   It says: "Breaking changes to the contract need a web-app release first."
4. The agent stops before the breaking change. It tells the developer that
   web-app must ship first, citing the concept. It can then offer an additive
   change instead.

At 10,000 concepts, search took 0.37 s and backlinks 0.34 s (medians). So the
agent can run these checks by default.

### 4.5 Recording a cross-repo fact

Say the agent finds, while working in web-app, that checkout now also calls
shared-auth's session endpoint.

1. The web-app pull request records only web-app facts in `web-app/okf/`. It
   may cite shared-auth by GitHub URL, but it does not link to shared-auth's
   files.
2. The agent opens a separate hub pull request as a draft. It adds
   `cross-repo/web-app-session-dependency.md` with type `Cross-Repo Dependency`,
   linking `/repos/web-app/…` and `/repos/shared-auth/…`.
3. Before marking it ready, the agent runs `compass hub assemble` and then
   `compass hub check`, which runs the strict validator and fails on any
   broken link. Until the web-app knowledge reaches the bundle the hub
   assembles, a link to a concept it adds is broken, so the check fails. In
   folder mode that happens when the web-app pull request merges; in branch
   mode, when `compass sync` merges its knowledge pull request. After that
   change merges, the agent or developer runs both commands again; once they
   pass, the hub pull request is marked ready for review. The reviewer runs
   them again before approving.

Cross-repo links live only in the hub, because the OKF spec has no way to link
between bundles (report, finding 4). This rule keeps every repo bundle valid
when read on its own.

### 4.6 Knowledge goes stale or turns out wrong

| Trigger | What the agent does |
|---|---|
| The code contradicts a concept | Fixes the concept in the same change (section 4.2) |
| The concept is past `stale_after` | Checks the claim against the code, then updates `generated` and `stale_after` and removes any `verified` entries, so a reviewer verifies again. If it cannot check the claim, it sets `status: draft` and says why. |
| The thing described was removed | Sets `status: deprecated`. It does not delete the file, so inbound links still resolve. |
| The agent cannot reach the code, such as another repo's internals | Sets `status: draft` and gives the reason in the body |

`stale_after` is required for `API Contract` and `Gotcha`. The default is 180
days. `okf show` reports `stale: true` for these concepts once the date
passes. Stale concepts come up as agents meet them during tasks, not in a
separate cleanup job.

This design keeps the spec's datetime form of `stale_after`. Upstream okfcli
accepts only a date-only value (okfcli#34); the pinned fork accepts both.

### 4.7 Renaming or moving a concept

Moves should be rare. They are a manual, human-led step:

1. Before moving, run `okf backlinks` on a freshly assembled hub for the old
   path. Note the hub concepts in the result (`cross-repo/…`, `decisions/…`).
2. In the repo, run `okfctl node mv <old> <new> --bundle okf --dry-run`,
   then run it for real. It rewrites the inbound links inside that repo. It
   also appends to `log.md` and regenerates the index files, replacing the
   root `index.md` prose. Run `okf index okf` afterwards so the index files
   match what okfcli writes.
3. After the repo's knowledge change merges (the code pull request in folder
   mode, the knowledge pull request in branch mode), the same person opens a
   hub pull request fixing the links from step 1. `compass hub assemble` and
   `compass hub check`, run before that pull request is opened and again
   before it is approved, fail on any link that was missed. Nothing reports a
   missed link elsewhere until someone assembles.

### 4.8 Adding a repo

First, the repo's maintainers choose a mode: folder mode, or branch mode if
they want no knowledge files on their code branches.

Folder mode:

1. Create `okf/index.md` with `okf_version: "0.2"`, plus `okf/overview.md`
   (type `Overview`). Put all prose in `overview.md`.
2. Run `okf index okf` and commit the result. The first run replaces the
   prose in the root `index.md` and adds an `index.md` to each folder. Doing
   it here keeps that one-time noise out of the first agent pull request.
3. Add the bundle check to the repo's CI: copy compass's
   `templates/folder-workflow.yml` into `.github/workflows/`. It runs
   `actions/bundle-check` (section 3).
4. Add one line to the repo's `AGENTS.md` (or `CLAUDE.md`): read
   `okf/index.md` and follow the OKF skill before starting work.
5. Add `<name> <GitHub URL> folder` to your hub's `repos.txt`.

Branch mode:

1. Agree with the maintainers on the `okf/…` branches, and on write access
   for the hub owners who run `compass sync` and `compass stamp`.
2. In a clone, run `compass branch setup --init` (git 2.42 or newer). It
   creates the orphan branch `okf/main` with a root `index.md` and checks it
   out at `okf/`. Add `overview.md`, run `okf index okf`, commit inside
   `okf/`, and publish with `git -C okf push -u origin okf/main`.
3. Protect `okf/main` so changes arrive only through pull requests, with a
   bypass for the hub owners, whose `compass sync` merges and `compass stamp`
   commits. Add compass's `templates/okf-main-workflow.yml` as
   `.github/workflows/okf.yml` on `okf/main`; it runs the same check as repo
   CI on `.` (section 3). The validators and `okf index` ignore `.github/`.
4. The repo's `AGENTS.md` cannot carry the pointer line, because it lives on
   the code branches. Put it in each developer's user-level agent
   instructions: in a repo whose remote has an `okf/main` branch, run
   `compass branch setup` if `okf/` is missing, then follow the OKF skill.
5. Add `<name> <GitHub URL> branch` to your hub's `repos.txt`.
6. Every developer runs `compass branch setup` once in each clone. It needs
   git 2.28 or newer.

Either way, agents then fill in knowledge as they work. The okf-skills
`backfill` command can draft concepts from git history as `status: draft`,
but it was not tested.

### 4.9 Working in a branch-mode repo

The report's section 5 ("Branch mode") defines the branches, hooks and
`compass sync` rules. This is how they feel in use. Output is from the
scratch-repo test
([evidence](../research/okf-knowledge-system/evidence/branch-mode.md)).

1. Once per clone, the developer runs `compass branch setup`. `okf/` is now a
   worktree of `okf/main`:

   ```
   $ compass branch setup
   okf/ -> okf/main
   ```

2. Starting work on a branch switches `okf/` with it. The code branch never
   shows `okf/`; `git status` stays empty. A detached HEAD, as during a
   bisect, leaves `okf/` where it was.

   ```
   $ git switch -q -c feat/retry
   okf/ -> okf/feat/retry
   ```

3. Before reading, the agent checks that `okf/` exists and is paired:
   `git -C okf branch --show-current`. The hooks do not run in CI, in cloud
   agents, or in repos whose hook manager (`core.hooksPath`) made
   `compass branch setup` refuse. A fresh clone there has no `okf/` at all, so
   the agent runs `compass branch setup` first. Otherwise it switches `okf/`
   itself, creating the branch from `origin/okf/feat/retry` if a teammate
   pushed it, else from `origin/okf/main`.

   Then it updates `okf/`. `git pull` on the code branch does not touch it,
   so without this an agent on `main` reads `okf/main` as this clone last
   saw it, missing knowledge pull requests merged since. The pull applies
   only when `okf/` tracks a remote branch: `okf/main`, or a knowledge branch
   someone pushed. A new, unpushed `okf/<b>` has nothing to pull, and
   `git pull` would fail on it for lack of an upstream.

   ```
   $ git -C okf status --short --branch
   ## okf/main...origin/okf/main [behind 2]
   $ git -C okf rev-parse -q --verify @{upstream} >/dev/null && git -C okf pull -q --ff-only
   ```
4. The agent reads and writes exactly as in sections 4.1 and 4.2. Then it
   commits inside `okf/`, pushes the code branch and then the knowledge
   branch, and opens the knowledge pull request from `okf/feat/retry` into
   `okf/main`. The code pull request links to it.

   ```
   $ git -C okf add -A && git -C okf commit -qm 'okf: retry budget'
   $ git push -q -u origin feat/retry && git -C okf push -q -u origin okf/feat/retry
   ```

   It pushes the code branch first. `compass sync` treats a knowledge branch
   with no code branch on the remote as abandoned, after a 7-day wait.
5. A teammate who checks out `feat/retry` gets its knowledge too. The hook
   tracks the pushed branch:

   ```
   $ git switch -q feat/retry
   okf/ -> okf/feat/retry
   $ ls okf; git -C okf rev-parse --abbrev-ref @{upstream}
   index.md
   retry-budget.md
   origin/okf/feat/retry
   ```

6. The reviewer reviews both pull requests (section 4.3). When the code pull
   request merges, the next `compass sync` run merges the approved knowledge
   pull request. When GitHub deletes `feat/retry`, the next `compass sync` run
   deletes `okf/feat/retry`. The report's section 5 has the full rules.
7. When the developer deletes the local branch, the hook deletes its local
   knowledge branch if nothing would be lost. It never touches the remote.

   ```
   $ git switch -q main; git branch -D feat/retry
   okf/ -> okf/main
   Deleted branch okf/feat/retry (was 56dcb3a).
   Deleted branch feat/retry (was ad1266d).
   ```

   It keeps the knowledge branch, with a message, in two cases:
   `Kept okf/feat/x: it has commits that are not on the remote.` and
   `Kept okf/feat/y: okf/ has uncommitted changes on it.`

```mermaid
sequenceDiagram
  participant A as Agent
  participant C as Code PR
  participant K as Knowledge PR
  participant S as compass sync (hub owner)
  A->>C: push feat/retry
  A->>K: push okf/feat/retry, open PR, link it from C
  Note over C,K: reviewer reviews both, adds verified on okf/feat/retry, approves K
  C->>C: merged, branch deleted
  S->>C: code PR merged?
  S->>K: approved and checks green? merge into okf/main
  S->>K: delete okf/feat/retry once feat/retry is gone
```

Four more things to know:

- Uncommitted edits in `okf/` follow a branch switch to the next knowledge
  branch, as git does for code. Commit knowledge before switching.
- git has no rename event, so the hook sees `git branch -m` as a delete. To
  rename a branch, rename the knowledge branch first:
  `git branch -m okf/feat/old okf/feat/new`, then the code branch. `okf/`
  follows both renames.
- The hub and other repos see only `okf/main`. Knowledge on `okf/feat/retry`
  is visible only on that branch until `compass sync` merges it.
- `git gc` reports every branch it packs as deleted. The hook ignores those
  reports, so `okf/` stays paired and no knowledge branch is lost.

## 5. Life of a concept

The spec defines `status` as one of `draft`, `stable` or `deprecated`. Trust
is separate: tools derive a `trust_tier` from the `verified` entries. The tiers
are `unverified`, `machine-confirmed` (a `process:` verifier, written by the
stamper) and `human-reviewed`. In the diagram, `Stable_verified` covers both
verified tiers.

```mermaid
stateDiagram-v2
  [*] --> Draft: agent writes, cannot confirm
  [*] --> Stable_unverified: agent writes, confirmed against code
  Draft --> Stable_unverified: confirmed later
  Stable_unverified --> Stable_verified: reviewer or stamper adds stamp
  Stable_verified --> Stale: stale_after passes
  Stable_unverified --> Stale: stale_after passes
  Stale --> Stable_unverified: agent re-checks, new generated and stale_after, verified removed
  Stale --> Draft: agent cannot re-check
  Stable_verified --> Stable_unverified: agent changes claim, verified removed
  Draft --> Deprecated: thing removed
  Stable_unverified --> Deprecated: thing removed
  Stable_verified --> Deprecated: thing removed
  Stale --> Deprecated: thing removed
  Deprecated --> [*]
```

A `verified` entry covers only the text that was checked. The tools do not
track this: in a test, a concept whose `verified.at` was older than its
`generated.at` still showed `trust_tier: human-reviewed`. So the skill adds two
rules:

- Whenever an agent writes a new `generated` stamp, it removes the concept's
  `verified` entries and says so in the pull request. The reviewer can then
  verify again, and the stamper re-confirms after merge. Agents may remove a
  stamp; they never add one, `human:` or `process:`.
- When reading, the agent counts a `verified` entry only if its `at` is at or
  after `generated.at`. This catches edits that skipped the first rule.

## 6. How the agent decides what to trust

The skill turns the fields on a concept into an action. Check the rows in
order; the first match wins. "Stale" means `stale_after` is in the past;
`okf show` reports it as `stale: true`. A `verified` entry
counts only if its `at` is at or after `generated.at` (section 5).

| Concept | Agent behavior |
|---|---|
| `deprecated` | Do not follow it. Look for what replaced it. |
| `draft`, or stale | Treat it as a lead only. Confirm it, then fix or refresh it (section 4.6). |
| `human-reviewed` | Rely on it. Cite it when it drives a decision. |
| `machine-confirmed` | Rely on it for what the machine check covers, such as a contract matching its OpenAPI file |
| `unverified` | Use it as a strong lead. Confirm against the code before a risky change. |

Two rules for backlinks:

- Empty backlinks can mean no links or a mistyped ID. okfcli returns
  `count 0` in both cases, so check the ID with `okf list`.
- In the hub, only `cross-repo/…` and `repos/<other repo>/…` entries are
  dependents. Entries from the concept's own repo are not.

## 7. How this solves the original problem

| Need | Mechanism | Evidence | What remains |
|---|---|---|---|
| N1: agents start informed | Skill step "before starting work", plus a pointer line in `AGENTS.md` (in branch mode, in user-level agent instructions); search, then list or show; trust rules in section 6 | Search 0.37 s and backlinks 0.34 s at 10,000 concepts; correct hits on the test data | Search hits lack trust and status, so the agent needs a second call; plain substring matching with no ranking |
| N2: agents update knowledge in normal work | Plain file edits in the same pull request, or in a paired knowledge pull request in branch mode, then `okf index` and `okf validate` | 1–3 file diffs; unknown frontmatter keys kept on disk; in branch mode, `okf/` followed every branch checkout on scratch repos | `okf index` overwrites hand-written `index.md` prose, so prose goes in `overview.md`; branch mode needs a hub owner to run `compass sync`, which has merged nothing live yet (tested on recorded GitHub responses, run live only as `--dry-run`) |
| N3: cheap human checks | Knowledge sits in the code diff, or in a knowledge pull request linked from it; one-line `verified` stamp; stamps removed when the claim changes; CI validation | A one-line verify diff; strict validator clean on the prototype | Nothing enforces who writes a `human:` stamp. Review is the only check; a later check could match the stamp's login against the pull request's approving reviewers (section 4.3). In branch mode the reviewer must open a second pull request, and knowledge waits until someone approves it. |
| N4: cross-repo knowledge | Hub `Cross-Repo Dependency` concepts plus a local assembled view of all repos, fetched from their remotes by `compass hub assemble` and checked by `compass hub check` | 3 correct backlinks on the prototype hub, including the cross-repo edge; `test/bench-assemble.sh 50` assembled 50 repos (10,100 files) from local bare remotes in 3.7 s cold and 4.0 s warm. The warm run re-cloned because of a known cache bug (`insteadOf` rewrites in the cache's origin check), so it measured cold clones; the fix is scheduled before the folder-mode pilot. | The view is only as fresh as its last local assembly; fetch time from GitHub for 50 repos is unmeasured until the pilot |
| N5: current and portable | `stale_after`, `deprecated`, and agents fixing contradictions as they go; plain OKF files; skill depends on six commands | Every tool read the same files; a CLI swap only touches the skill | Staleness detection relies on the pinned fork until upstream okfcli fixes okfcli#34 |

## 8. What users see when something breaks

| Failure | What the user or agent sees | Handling |
|---|---|---|
| Hub built with symlinks instead of copies | Validation passes but loads 0 repo concepts: a false pass | `compass hub assemble` only copies, and fails, naming the repo, when a bundle holds a symlink. `compass hub check` checks that each repo's concept count is above zero and matches the files copied. |
| Repo bundle uses a `/…` link | Passes in the repo, reported broken in the hub | `compass hub check` reports it. The skill forbids `/…` links in repo bundles. The re-run test data shows exactly this finding for `idempotency-key`. |
| `stale_after` in the spec's datetime form, with upstream okfcli (okfcli#34) | `okf validate` errors; `okf show` says `stale: false` for a stale concept | Compass pins a patched fork, `v0.5.0-tuan-nng.1`, and `compass setup` installs it. The fork accepts datetimes and reports `stale: true`. |
| Agent writes `verified: human:…` itself | Concept shows as `human-reviewed` | Caught only in review. The skill forbids it. |
| Agent changes a reviewed concept but keeps its `verified` entry | Tools still show `human-reviewed` | The skill ignores a stamp older than `generated.at` (section 5). The reviewer sees the kept stamp in the diff. |
| Hub pull request links a concept that is not merged yet | `compass hub check`, run locally, fails on the broken link | Open the hub pull request as a draft. After the repo change merges, run `compass hub assemble` and `compass hub check` again, and mark it ready once they pass (section 4.5) |
| Search term inside backticks, such as `` `Idempotency-Key` `` | okfcli finds it; okf-mcp did not | Re-test any replacement search tool against this case |
| Moved file breaks links from the hub | `compass hub check` fails at the next local assembly; nothing reports it before then | Section 4.7 |
| Two pull requests change the same folder | Merge conflict in a generated `index.md` or in `log.md` | Take either side of `index.md` and re-run `okf index okf`; keep both `log.md` entries (section 4.2) |
| Pull request adds or edits a concept without re-running `okf index` | Repo CI fails the index check | Run `okf index okf` and commit |
| Branch mode: hooks missing (CI, cloud agent, or a hook manager set `core.hooksPath`) | A fresh clone has no `okf/`; an older one keeps `okf/` on the knowledge branch of an earlier code branch, and knowledge commits land there | The skill runs `compass branch setup` if `okf/` is missing and checks the pairing before reading or writing (section 4.9). For hook managers, add the two hooks to the manager by hand (`compass branch setup --no-hooks`, then `--print-hook`). |
| Branch mode: knowledge merged since the last pull | `okf/` lacks it; `git -C okf status --short --branch` shows `behind` | The skill pulls `okf/` before reading whenever it tracks a remote branch (section 4.9) |
| Branch mode: CI or cloud agent does not know the repo uses branch mode | The agent works without the repo's knowledge | Each CI and cloud agent environment runs `compass setup`, and the platform's organisation-level agent instructions carry the pointer line from `templates/user-pointer.md`: if `git ls-remote origin okf/main` prints a line, run `compass branch setup` and follow the skill. Tested only in a headless scenario so far (section 9). |
| Branch mode: code pull request merged, but its knowledge pull request is unapproved, failing or conflicting | The knowledge pull request stays open; `compass sync` comments on both pull requests | Approve it, or resolve as in section 4.2 (`okf index okf` for `index.md` conflicts); the next `compass sync` run merges it |
| Branch mode: code branch deleted locally while `okf/` holds uncommitted work on its knowledge branch | `Kept okf/feat/y: okf/ has uncommitted changes on it.` | Commit or discard the work, switch `okf/` to `okf/main`, then delete the knowledge branch by hand |
| Branch mode: code branch deleted locally before its knowledge branch was pushed | `Kept okf/feat/x: it has commits that are not on the remote.` | Push it and open a knowledge pull request, or delete it by hand with `git branch -D okf/feat/x` |
| Branch mode: code branch renamed with `git branch -m` | The hook treats it as a delete. A pushed knowledge branch is deleted locally and `okf/` moves to `okf/main`; the next checkout pairs with a new, empty `okf/<new>`. An unpushed one is kept, with the usual message. | Rename the knowledge branch first (section 4.9). After the fact: `git branch -m okf/<old> okf/<new>` if it was kept, or `git branch okf/<new> origin/okf/<old>` if it was pushed; then `git -C okf switch okf/<new>`. |
| Branch mode: nobody runs `compass sync` | Knowledge pull requests stay open after their code merges; `okf/<b>` branches pile up on the remote | The hub owners run it, daily during the pilot. Nothing alerts on this; the pilot measures it (section 9). |

## 9. What does not exist yet

These are needed before the journeys above work end to end. Section 8 of the
report lists the same open work in shorter form.

1. The skill in use. It is built (`skill/okf/SKILL.md` in compass, installed
   by `compass setup`) from the section 6 outline in the report and sections
   4.1–4.6, 4.9, 5 and 6 of this document, but no agent has used it on a real
   repo. If the skill ever reads custom frontmatter keys, also okfcli#35:
   `okf show` drops them.
2. Assembly at real scale. `compass hub assemble` and `compass hub check` are
   built and have assembled a hub made from `templates/hub/` from scratch
   repos on GitHub. Fetch time from GitHub for 50 repos is unmeasured, and
   the clone-cache fix (section 4.4) is still to land.
3. A live stamp. The stamper (`compass stamp`) is built and tested on
   recorded GitHub responses, and has run live only as `--dry-run`. The test
   data has one `process:ci-contract-test` stamp, but no stamper run wrote it.
4. For branch mode:
   - a live `compass sync` merge; it is built and tested on recorded GitHub
     responses, and has run live only as `--dry-run`;
   - `compass branch setup` on real repos. It passes every evidence case in
     compass's `test/branch-setup.sh`, but only on local test repos, and the
     `okf/main` workflow has run only on a scratch GitHub repo;
   - CI and cloud agents finding branch-mode knowledge through the pointer
     line (section 8). In a headless scenario on a fresh clone, the agent
     loaded the skill and ran `compass branch setup` in 3 of 3 runs; no real
     CI or cloud agent has run it yet.
5. A pilot with 2–3 repos, at least one in each mode, measuring:
   - how often agents write knowledge;
   - how often reviewers correct it;
   - how often agents run into stale concepts;
   - how often each concept loses its `verified` stamp to an edit and has to
     be re-verified;
   - in branch mode, how often a knowledge pull request is still open a day
     after its code merged.
