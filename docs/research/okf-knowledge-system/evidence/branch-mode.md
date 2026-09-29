# Branch mode: knowledge on its own branches of the code repo

Date: 2026-09-29. git 2.53.0, okfcli v0.5.0, okf-skills validator v0.10.0.
Scratch repos only: bare `origin.git` and `origin2.git` plus clones for alice,
bob, carol, dave and `m`. Script under test:
[okf-branch-setup.sh](okf-branch-setup.sh). The hub's sync job was not built,
so nothing here covers the remote side.

## Question

Some maintainers do not want an `okf/` folder on their code branches. Can the
bundle live on separate branches of the same repo, follow the working branch,
and be cleaned up when that branch is deleted, without losing work?

## Design tested

- `okf/main` is an orphan branch whose root is the bundle root. It pairs with
  the repo's default branch, whatever that is called.
- Code branch `<b>` pairs with knowledge branch `okf/<b>`. The trunk cannot be
  named plain `okf`: git refuses `okf/feat/x` while a branch `okf` exists
  (`cannot lock ref 'refs/heads/okf/feat/x': 'refs/heads/okf' exists`).
- `<repo>/okf/` is a git worktree of the paired knowledge branch, hidden from
  the code branches by `/okf/` in `.git/info/exclude`. The leading slash
  limits it to the top level, so code folders such as `cmd/okf/` stay visible.
- Two local hooks in `.git/hooks/`, which are never committed:
  - `post-checkout` switches `okf/` on every branch checkout.
  - `reference-transaction` deletes `okf/<b>` when `<b>` is deleted, unless
    `okf/<b>` has commits that no remote-tracking branch contains, or `okf/`
    has uncommitted changes while on `okf/<b>`.

Earlier versions of the hooks hit four traps, all fixed in the script:

- The hooks directory is shared by every worktree, so a switch inside `okf/`
  ran `post-checkout` again and created branches such as `okf/okf/feat/retry`.
  The hook now exits unless the current worktree contains `okf/`.
- Deleting a code branch while `okf/` still had its knowledge branch checked
  out left `okf/` on an unborn branch (`No commits yet on okf/feat/c`). The
  hook now switches `okf/` to `okf/main` first.
- The hook kept a branch only when that switch failed. A new untracked file
  does not make `git switch` fail, so the draft moved to `okf/main` and the
  branch was deleted. The hook now checks `git status --porcelain` first.
- The hook force-deleted knowledge branches whose commits were never pushed.
  It now keeps them.

A symlinked `okf/` pointing into a hub checkout was also tried and rejected:
`okf list okf` returned 0 concepts and `okf validate okf` returned
`"valid": true` with no findings, the same silent pass as report finding 1.

## Run

Command lines are shown with `$`, output indented. `okf-branch-setup.sh` and
`okf` are on `PATH`. The seed commit has `main.go` and `cmd/okf/okf.go`. carol
cloned the repo before alice pushed `okf/main`.

```
## 1. First setup of a repo (alice)
$ okf-branch-setup.sh
  This repo has no okf/main branch yet. Re-run with --init to create it.
$ okf-branch-setup.sh --init
  Created okf/main. Publish it with: git -C okf push -u origin okf/main
  okf/ -> okf/main
$ git -C okf push -q -u origin okf/main
$ cat .git/info/exclude | grep okf; git status --porcelain; git ls-tree -r --name-only main
  /okf/
  cmd/okf/okf.go
  main.go
$ okf validate okf | grep -E '"valid"|"errors"'
    "errors": 0,
    "valid": true,

## 2. A code folder named okf is not hidden
$ echo 'package okf // new' > cmd/okf/new.go; git status --porcelain; rm cmd/okf/new.go
  ?? cmd/okf/new.go

## 3. Knowledge follows the code branch
$ git switch -q -c feat/retry
  okf/ -> okf/feat/retry
$ printf -- '---\ntype: Gotcha\ntitle: Retry budget\n---\nThree retries.\n' > okf/retry-budget.md
$ git -C okf add -A && git -C okf commit -qm 'okf: retry budget'
$ git push -q -u origin feat/retry && git -C okf push -q -u origin okf/feat/retry
$ okf list okf | grep -c '"id"'
  1
$ git switch -q main; ls okf
  okf/ -> okf/main
  index.md
$ git status --porcelain

## 4. A teammate (bob) sets up and checks out the same branch
$ okf-branch-setup.sh
  okf/ -> okf/main
$ git switch -q feat/retry; ls okf; git -C okf rev-parse --abbrev-ref @{upstream}
  okf/ -> okf/feat/retry
  index.md
  retry-budget.md
  origin/okf/feat/retry

## 5. Deleting a code branch deletes its pushed knowledge branch
$ git switch -q main; git branch -D feat/retry
  okf/ -> okf/main
  Deleted branch okf/feat/retry (was 56dcb3a).
  Deleted branch feat/retry (was ad1266d).
$ git branch --format='%(refname:short)'
  main
  okf/main
$ git ls-remote --heads origin | sed 's/.*refs.heads.//'
  feat/retry
  main
  okf/feat/retry
  okf/main

## 6. Knowledge commits that were never pushed are kept
$ git switch -q -c feat/x; echo one > okf/one.md; git -C okf add -A; git -C okf commit -qm one; git switch -q main
  okf/ -> okf/feat/x
  okf/ -> okf/main
$ git branch -D feat/x
  Kept okf/feat/x: it has commits that are not on the remote.
  Deleted branch feat/x (was ad1266d).
$ git branch --format='%(refname:short)'
  main
  okf/feat/x
  okf/main
$ git branch -D okf/feat/x
  Deleted branch okf/feat/x (was 1001666).

## 7. Uncommitted work in okf/ on the knowledge branch is kept
$ git switch -q -c feat/y; git switch -q main; git -C okf switch -q okf/feat/y; echo draft > okf/draft.md
  okf/ -> okf/feat/y
  okf/ -> okf/main
$ git branch -D feat/y
  Kept okf/feat/y: okf/ has uncommitted changes on it.
  Deleted branch feat/y (was ad1266d).
$ git -C okf branch --show-current; git branch --format='%(refname:short)' | grep feat/y
  okf/feat/y
  okf/feat/y
$ rm okf/draft.md; git -C okf switch -q okf/main; git branch -D okf/feat/y
  Deleted branch okf/feat/y (was 58fd858).

## 8. Uncommitted knowledge edits follow a code-branch switch, like code edits
$ git switch -q -c feat/z; echo draft > okf/draft.md; git switch -q main; git -C okf status --short --branch
  okf/ -> okf/feat/z
  okf/ -> okf/main
  ## okf/main...origin/okf/main
  ?? draft.md
$ rm okf/draft.md; git branch -D feat/z
  Deleted branch okf/feat/z (was 58fd858).
  Deleted branch feat/z (was ad1266d).

## 9. Renaming a code branch
$ git switch -q -c feat/old; git push -q -u origin feat/old; git -C okf push -q -u origin okf/feat/old
  okf/ -> okf/feat/old
$ git branch -m feat/old feat/new
  Deleted branch okf/feat/old (was 58fd858).
$ git branch --show-current; git -C okf branch --show-current
  feat/new
  okf/main
$ git switch -q main; git switch -q feat/new; git -C okf branch --show-current
  okf/ -> okf/main
  okf/ -> okf/feat/new
  okf/feat/new

## 9b. Renaming: rename the knowledge branch first
$ git branch -m okf/feat/new okf/feat/newer; git branch -m feat/new feat/newer
$ git branch --show-current; git -C okf branch --show-current
  feat/newer
  okf/feat/newer

## 10. Detached HEAD leaves okf/ where it was
$ git switch -q main; git switch -q --detach HEAD; git -C okf branch --show-current; git switch -q main
  okf/ -> okf/main
  okf/main
  okf/ -> okf/main

## 11. Second code worktree without okf/
$ git worktree add -q ../bob-wt -b feat/wt; ls ../bob-wt
  cmd
  main.go
$ git branch --format='%(refname:short)' | grep -c okf/feat/wt
  0

## 12. --init in a clone made before okf/main was pushed (carol)
$ okf-branch-setup.sh --init; git -C okf log --oneline | wc -l; git -C okf rev-parse --abbrev-ref @{upstream}
  okf/ -> okf/main
  1
  origin/okf/main

## 13. Default branch named master
$ okf-branch-setup.sh --init | tail -1
  okf/ -> okf/main
$ git -C okf push -q -u origin okf/main; git switch -q -c feat/m; git switch -q master; git branch --format='%(refname:short)'
  okf/ -> okf/feat/m
  okf/ -> okf/main
  feat/m
  master
  okf/feat/m
  okf/main
$ git branch -D feat/m; git branch --format='%(refname:short)'
  Deleted branch okf/feat/m (was 22fd6c2).
  Deleted branch feat/m (was b6583de).
  master
  okf/main

## 14. Repo that uses a hook manager
$ git config core.hooksPath .husky; okf-branch-setup.sh; echo exit=$?
  core.hooksPath is set, so git ignores .git/hooks. Add the two hooks from this script to that hook manager by hand.
  exit=1
```

## CI files on the knowledge branch

A workflow file at `.github/workflows/okf.yml` on `okf/main` sits inside the
bundle root. None of the three checks the design runs saw it: `okf index .`
wrote 5 index files and listed only `svc/`, `okf validate .` reported only the
known `stale_after` errors (okfcli#34) from the copied test concepts, and the
okf-skills validator with `--strict` printed `✓ conformant — no issues`.
Whether GitHub runs that workflow for pull requests into `okf/main` was not
tested.

## Results

| Case | Result |
|---|---|
| Code branches stay free of knowledge files | Pass: `main` holds only its code; `git status` stays empty |
| A code folder named `okf` (`cmd/okf/`) | Still tracked: a new file there shows as `??` |
| `okf/` follows branch checkouts | Pass, both for new branches and for a teammate's pushed branch (tracks `origin/okf/feat/retry`) |
| okfcli on the `okf/` worktree | Pass: it is a real folder, so `list` and `validate` read it |
| Local delete, knowledge pushed | Knowledge branch deleted locally; remote branches untouched, left to the sync job |
| Local delete, knowledge commits never pushed | Kept, with a message |
| Local delete, uncommitted work in `okf/` on that branch | Kept, with a message |
| Uncommitted knowledge edits at switch time | Carried to the next knowledge branch, as git does for code |
| Renaming a code branch with `git branch -m` | Seen as a delete. A pushed knowledge branch is deleted locally and `okf/` moves to `okf/main`; the next checkout pairs with a fresh `okf/feat/new`. Renaming the knowledge branch first keeps the pairing. |
| Detached HEAD | `okf/` stays where it was |
| Extra code worktrees | Hook stays quiet and creates no branches |
| `--init` in a clone older than the pushed `okf/main` | The script fetches first and uses the pushed trunk; no second trunk |
| Default branch named `master` | Pairs with `okf/main`; feature branches pair and clean up normally |
| `core.hooksPath` set (husky, lefthook) | Setup refuses; hooks must be added to that manager by hand |

Limits:

- Minimum git: 2.42 for `worktree add --orphan` (used only by `--init`), 2.28
  for the `reference-transaction` hook.
- The remote must be called `origin`.
- git runs the `reference-transaction` hook for every ref update, including
  each commit, fetch and rebase step. The hook exits after one comparison for
  anything but a committed deletion. On Linux that cost is negligible.
  [INFERENCE: on Git for Windows each shell start is slower; not measured.]
