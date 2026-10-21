# Path rule must also check the merge result

## What happened

`compass sync` (`internal/syncjob/sync.go`) guards each hub merge with a path
rule: a hub branch `<id>/<b>` may change only `repos/<id>/**`,
`repos/index.md`, and hub-level files other than the control files. The plan
(`plans/261021-1007-hub-branch-knowledge/plan.md`, Trust boundaries) worded it
as "between its merge base and its head", and the first build checked exactly
that diff.

An independent review found two ways past it, with no change to the branch
diff itself:

- **Several merge bases.** A branch built as a merge of two commits already on
  the default branch gets two merge bases. Git's single `merge-base` picks one;
  the diff from it shows nothing forbidden, but the real merge starts from a
  virtual base and can revert a control-file or other-repo change.
- **Rename carry.** If the default branch moved `repos/web-app/z.md` to
  `repos/billing-api/z.md` after the branch started, the branch's allowed edit
  to `repos/web-app/z.md` lands, through rename detection, in billing-api's
  folder.

The same review found that `Repos/billing-api/x.md` passed as "outside
`repos/`", while case-insensitive filesystems (macOS, Windows) put it inside
billing-api's folder.

## Decision

Run the path and file-mode rules on two diffs, both with `--no-renames`: merge
base to head, and default head to the merge tree from `git merge-tree
--write-tree`. The second reads only tree metadata, so the rule that no branch
file is read before the rules pass still holds. Compare `repos` and the root
control-file names case-insensitively, and refuse a top-level folder that
case-folds to `repos` but is not `repos`. The plan's Trust boundaries now say
this.

## Verification

`TestRenameOnMainCarryingBranchEditIntoAnotherFolderIsRefused`, and new
`TestPathRule`/`TestPathRuleRefusals` cases for `Repos/`, `REPOS/`,
`Checks.TXT` and `.GITHUB/`. Each failed with its fix removed (checked with
`go test -overlay`). `make test` exits 0. There is no criss-cross test; the
merge-result diff covers it by construction.

## Next steps

Any new merge guard in sync should be checked against what the merge
produces, not only against what the branch changed.

Historical work record — not durable authority.
