---
phase: 4
title: "Compass live on GitHub"
status: pending
priority: P1
effort: 1.5d
dependencies: [1, 2, 3]
---

# Phase 4: Compass live on GitHub

## Overview

Outcome: the compass code is on the private `tuan-nng/compass` (which already holds the docs and plans on `master`), and its Actions access, `none` today, opens to the account's private repos (plan decision 1).

A scratch hub, `tuan-nng/okf-scratch-hub`, is made the way an end user would make one (plan decision 4):

1. copy `templates/hub/`;
2. list `okf-scratch-billing-api`, `okf-scratch-shared-auth` and `okf-scratch-web-app` in `repos.txt`;
3. copy in the prototype's cross-repo concepts from `testdata/proto`, so there are links to check;
4. run `compass app create --role reader`. The user clicks the app creation and installation pages.

That hub, and all five scratch repos, then run their checks from compass actions pinned by commit SHA.

Owns:

- **`actions/bundle-check`, `actions/hub` and `actions/writer`.** Each sets up Go with a SHA-pinned `setup-go`, with its cache on, then builds and runs `compass` (plan decision 16).
- **The first push of compass code.** Before it, `git ls-files` must list only what compass means to publish (see the plan's Risks).
- **`tuan-nng/okf-scratch-hub`** and the reader app.
- **The workflow files in the five scratch repos:** `okf-scratch-bundle-check`, `okf-scratch-branch-mode`, `okf-scratch-billing-api`, `okf-scratch-shared-auth` and `okf-scratch-web-app`.
- **Design doc and research report changes for plan decision 15.** Neither promises a hub trigger from repo pipelines any more:
  - the design doc's section 2 "Hub CI" row;
  - section 4.5, step 3;
  - the section 8 row on a hub pull request waiting for an unmerged concept;
  - research report section 5, "Hub assembly and CI".

  Also the design section 8 row on symlinked hubs, because the count check is now built, and the places that say the 50-repo fetch time was not measured (design section 4.4 and section 7, N4), which now cite the bench and this phase's hub run.

## Verification

- `gh api repos/tuan-nng/compass/actions/permissions/access -q .access_level` prints `user`.
- On `okf-scratch-bundle-check`:
  - a new push run concludes `success`;
  - a PR that adds a concept without `okf index` concludes `failure` with "index files are out of date".
- On `okf-scratch-branch-mode`, the `okf` workflow on a PR into `okf/main` concludes `success`.
- `gh workflow run hub -R tuan-nng/okf-scratch-hub`, then `gh run list -R tuan-nng/okf-scratch-hub -w hub -L 1 --json conclusion -q '.[0].conclusion'` prints `success`. The "build compass" step takes at most 60 s.
- A draft hub PR that links a concept missing from every repo makes `gh pr checks <n> -R tuan-nng/okf-scratch-hub` exit non-zero.
- Marking that draft ready with `gh pr ready <n>` adds one run: `gh run list -R tuan-nng/okf-scratch-hub -w hub -e pull_request -c <head-sha> --json databaseId -q length` goes up by 1.
- `compass setup --org tuan-nng --hub tuan-nng/okf-scratch-hub --yes` exits 0. `okf --version | jq -r .version` then prints `0.5.0-tuan-nng.1`.
