# Index files: first run, freshness check and merge conflicts

Date: 2026-09-29. git 2.53.0, okfcli v0.5.0, okf-skills validator v0.10.0.
Scratch repo only: a copy of the billing-api bundle from the test data
(`/tmp/okf-research/fixture/workspace/billing-api/okf`), with the design's
conventions applied: the `/contracts/…` link made relative and an
`overview.md` added. A second gotcha, `invoice-rounding.md`, sits next to
`idempotency-key.md` in the gotchas index so that case 6 has adjacent rows.

## Question

`okf index` writes the `index.md` files, so they change in most knowledge pull
requests. Does repo CI catch a stale index, and can an agent resolve index
conflicts without editing tables by hand?

## Run

Command lines are shown with `$`, output indented. `okf` is on `PATH`, and `$V`
is the okf-skills `okf_validate.py`. `same-as-rebuild` deletes every folder
`index.md` in a scratch copy of `okf/`, re-runs `okf index` there and diffs:

```sh
#!/bin/sh
# Rebuild every index.md of okf/ in a scratch copy and compare.
r=$(mktemp -d); cp -r okf "$r/okf"
find "$r/okf" -name index.md ! -path "$r/okf/index.md" -delete
okf index "$r/okf" >/dev/null && diff -r okf "$r/okf" && echo "same as a clean rebuild"
rm -rf "$r"
```

```
## 1. First okf index run on a repo (section 4.8)
$ okf index okf >/dev/null; git status --porcelain
   M okf/index.md
  ?? okf/contracts/index.md
  ?? okf/gotchas/index.md
  ?? okf/services/index.md
$ sed -n '1,8p' okf/index.md
  ---
  okf_version: "0.2"
  ---

  # Index

  Bundle root.
$ git add -A && git commit -qm 'okf: index'; okf index okf >/dev/null; git status --porcelain -- okf | wc -l
  0

## 2. A human verified stamp does not change the index (section 4.3)
$ sed -i 's/^stale_after: 2026-09-01T00:00:00Z$/&\nverified: { by: human:alice, at: 2026-09-29T15:00:00Z }/' okf/gotchas/idempotency-key.md
$ okf index okf >/dev/null; git diff --stat
   okf/gotchas/idempotency-key.md | 1 +
   1 file changed, 1 insertion(+)
$ git checkout -q -- okf

## 3. An out-of-date index: only the index check fails (section 3)
$ printf -- '---\ntype: Gotcha\ntitle: Retry budget\ndescription: Retry budget.\ntags: [billing]\ngenerated: { by: claude-code/opus-5, at: 2026-09-29T14:00:00Z }\nstale_after: 2027-03-28T14:00:00Z\n---\nBody.\n' > okf/gotchas/retry-budget.md
$ python3 $V okf --strict | tail -1; okf validate okf | grep -o '"rule": "[^"]*"' | sort | uniq -c
    ✓ conformant (2 warning(s))
        1 "rule": "okf/frontmatter/tags-recommended"
        4 "rule": "okf/lifecycle/stale-after-invalid"
$ okf index okf >/dev/null; test -z "$(git status --porcelain -- okf)"; echo index-check-exit=$?
  index-check-exit=1
$ git checkout -q -- okf && git clean -qfd okf

## 4. Conflict: both add concepts that sort next to each other (section 4.2)
$ git switch -q -c a base; printf -- '---\ntype: Gotcha\ntitle: Job alpha\ndescription: Job alpha.\ntags: [billing]\ngenerated: { by: claude-code/opus-5, at: 2026-09-29T14:00:00Z }\nstale_after: 2027-03-28T14:00:00Z\n---\nBody.\n' > okf/gotchas/job-alpha.md; okf index okf >/dev/null; git add -A; git commit -qm a
$ git switch -q -c b base; printf -- '---\ntype: Gotcha\ntitle: Job beta\ndescription: Job beta.\ntags: [billing]\ngenerated: { by: claude-code/opus-5, at: 2026-09-29T14:00:00Z }\nstale_after: 2027-03-28T14:00:00Z\n---\nBody.\n' > okf/gotchas/job-beta.md; okf index okf >/dev/null; git add -A; git commit -qm b
$ git switch -q a; git merge -q b -m merge; git checkout --ours -- $(git diff --name-only --diff-filter=U)
  Auto-merging okf/gotchas/index.md
  CONFLICT (content): Merge conflict in okf/gotchas/index.md
  Automatic merge failed; fix conflicts and then commit the result.
$ okf index okf >/dev/null; same-as-rebuild; git merge --abort
  same as a clean rebuild
$ git switch -q a; git merge -q b -m merge; git checkout --theirs -- $(git diff --name-only --diff-filter=U)
  Auto-merging okf/gotchas/index.md
  CONFLICT (content): Merge conflict in okf/gotchas/index.md
  Automatic merge failed; fix conflicts and then commit the result.
$ okf index okf >/dev/null; same-as-rebuild; git merge --abort
  same as a clean rebuild
$ git switch -q main; git branch -qD a b

## 5. Conflict: both create the same new folder (section 4.2)
$ git switch -q -c a base; mkdir -p okf/runbooks; printf -- '---\ntype: Gotcha\ntitle: Runbook one\ndescription: Runbook one.\ntags: [billing]\ngenerated: { by: claude-code/opus-5, at: 2026-09-29T14:00:00Z }\nstale_after: 2027-03-28T14:00:00Z\n---\nBody.\n' > okf/runbooks/r1.md; okf index okf >/dev/null; git add -A; git commit -qm a
$ git switch -q -c b base; mkdir -p okf/runbooks; printf -- '---\ntype: Gotcha\ntitle: Runbook two\ndescription: Runbook two.\ntags: [billing]\ngenerated: { by: claude-code/opus-5, at: 2026-09-29T14:00:00Z }\nstale_after: 2027-03-28T14:00:00Z\n---\nBody.\n' > okf/runbooks/r2.md; okf index okf >/dev/null; git add -A; git commit -qm b
$ git switch -q a; git merge -q b -m merge; git checkout --ours -- $(git diff --name-only --diff-filter=U)
  Auto-merging okf/runbooks/index.md
  CONFLICT (add/add): Merge conflict in okf/runbooks/index.md
  Automatic merge failed; fix conflicts and then commit the result.
$ okf index okf >/dev/null; same-as-rebuild; git merge --abort
  same as a clean rebuild
$ git switch -q a; git merge -q b -m merge; git checkout --theirs -- $(git diff --name-only --diff-filter=U)
  Auto-merging okf/runbooks/index.md
  CONFLICT (add/add): Merge conflict in okf/runbooks/index.md
  Automatic merge failed; fix conflicts and then commit the result.
$ okf index okf >/dev/null; same-as-rebuild; git merge --abort
  same as a clean rebuild
$ git switch -q main; git branch -qD a b

## 6. Conflict: both edit concepts with adjacent index rows (section 4.2)
$ git switch -q -c a base; sed -i 's/^description: POST/description: Retried POST/' okf/gotchas/idempotency-key.md; okf index okf >/dev/null; git add -A; git commit -qm a
$ git switch -q -c b base; sed -i 's/^description: Totals/description: Invoice totals/' okf/gotchas/invoice-rounding.md; okf index okf >/dev/null; git add -A; git commit -qm b
$ git switch -q a; git merge -q b -m merge; git checkout --ours -- $(git diff --name-only --diff-filter=U)
  Auto-merging okf/gotchas/index.md
  CONFLICT (content): Merge conflict in okf/gotchas/index.md
  Automatic merge failed; fix conflicts and then commit the result.
$ okf index okf >/dev/null; same-as-rebuild; git merge --abort
  same as a clean rebuild
$ git switch -q a; git merge -q b -m merge; git checkout --theirs -- $(git diff --name-only --diff-filter=U)
  Auto-merging okf/gotchas/index.md
  CONFLICT (content): Merge conflict in okf/gotchas/index.md
  Automatic merge failed; fix conflicts and then commit the result.
$ okf index okf >/dev/null; same-as-rebuild; git merge --abort
  same as a clean rebuild
$ git switch -q main; git branch -qD a b
```

## Results

| Case | Result |
|---|---|
| First `okf index` on a repo | Replaces the root `index.md` prose and adds an `index.md` to each folder; a second run changes nothing |
| Adding a `verified` line | One-line diff; the regenerated index files do not change |
| Concept added without re-running `okf index` | Both validators miss it (the okfcli findings are only the known okfcli#34 errors and a lint warning); `okf index okf` plus `test -z "$(git status --porcelain -- okf)"` exits 1 |
| Two branches add concepts that sort next to each other | Conflict in the folder `index.md` |
| Two branches create the same new folder | add/add conflict in the new folder's `index.md` |
| Two branches edit concepts with adjacent index rows | Conflict in the folder `index.md` |
| Resolution | Taking either side of the conflicted `index.md` and re-running `okf index okf` matched a clean rebuild in all six merges |
