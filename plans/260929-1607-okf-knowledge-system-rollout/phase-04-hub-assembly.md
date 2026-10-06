---
phase: 4
title: "Hub repo, assembly and hub CI"
status: pending
priority: P1
effort: 2.5d
dependencies: [1]
---

# Phase 4: Hub repo, assembly and hub CI

## Overview

Outcome: `<org>/knowledge-hub` exists, and its CI assembles every repo in `repos.txt` straight from the repo's remote. The same assembly script works on a developer machine with no checkouts of the repos. The script's interface, the hub action and the reader key's limits are in the plan's `## Design`.

This phase owns:

- **The assembly script in `okf-tools`,** `bin/assemble-hub.sh`, grown from the 15-line prototype (`docs/research/okf-knowledge-system/evidence/assemble-hub.sh`) and added to the installer (plan decision 17). What it adds:
  - it fetches each bundle from its remote: the `okf/` folder of the default branch in folder mode, or `okf/main` in branch mode;
  - it keeps a clone cache between runs;
  - it copies real files and refuses symlinks;
  - it rejects any URL outside `https://github.com/<org>/`;
  - it strips the root `index.md` frontmatter, as the prototype does;
  - it runs the per-repo checks.
- **The hub composite action in `okf-tools`.** It assembles the repos, runs the strict validator on the hub, and checks each repo's concept count. The count check compares the concepts `okf list` sees under `repos/<name>/` with the files copied; the count must be above zero and the two must match.
- **The reader GitHub App** (plan decision 6), installed only on the repos in `repos.txt`.
- **The `knowledge-hub` repo:**
  - root `index.md`, and the `cross-repo/`, `decisions/` and `glossary/` folders;
  - `repos.txt`;
  - a `.gitignore` entry for `repos/`;
  - CODEOWNERS covering `.github/`, `CODEOWNERS`, `repos.txt` and `checks.txt`;
  - a default-branch ruleset that blocks direct pushes and requires a code-owner approval;
  - a thin workflow that calls the hub action by SHA. It runs nightly, on manual dispatch, and on hub pull requests, including when a draft is marked ready for review. So marking a waiting draft ready, as design section 4.5 describes, re-runs its check.
- **Updates for plan decision 10** in both documents. Neither should promise a trigger from repo pipelines any more:
  - the design doc's "Hub CI" row in section 2, section 4.5 step 3, and the section 8 row on a hub pull request that links a concept not merged yet;
  - the research report's section 5, "Hub assembly and CI".
- **The design doc's section 8 row on symlinked hubs,** which still says the count check is not built.

## Verification

- `./test/assemble.sh` in `okf-tools` exits 0. It builds local bare remotes from the test data (billing-api in folder mode, web-app in branch mode, shared-auth in folder mode) and checks:
  - after assembly, `okf backlinks <hub> repos/billing-api/contracts/invoice-api` lists `cross-repo/invoice-dependency` among its results;
  - a hub concept with a broken link to a repo makes the strict validator exit non-zero;
  - an empty bundle, a symlink in a bundle, or a remote that cannot be reached each make the script exit non-zero and name the repo;
  - a `repos.txt` URL outside `https://github.com/<org>/`, or a `file://` URL, is rejected before anything is fetched;
  - in local mode, a remote that cannot be reached keeps the previous copy, prints a warning, and exits 0.
- `./test/bench-assemble.sh 50` prints a cold time under 300 s and a warm time under 60 s (plan decision 9).
- `gh workflow run hub -R <org>/knowledge-hub`, then `gh run watch`, ends with `success`.
- A draft hub pull request that links a concept missing from every repo shows a failed check (`gh pr checks <n>` exits non-zero).
- Marking that draft ready for review starts a new run: `gh run list -R <org>/knowledge-hub -w hub -e pull_request -L 1` shows it.
- A hub pull request that changes `.github/workflows/` shows `reviewDecision` = `REVIEW_REQUIRED` until a code owner approves (`gh pr view <n> --json reviewDecision`).
