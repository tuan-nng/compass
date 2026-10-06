#!/usr/bin/env bash
# `compass bundle check` passes a clean bundle and fails on a stale index or a
# strict-validator finding, in folder mode (okf) and on an okf/main root (.).
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

OKF=$(okf_bin) || exit 1
export OKF
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

new_repo() {
  git init -q -b main "$1"
  git -C "$1" config user.email t@example.com
  git -C "$1" config user.name test
}
commit_all() { git -C "$1" add -A && git -C "$1" commit -qm "$2"; }

# Folder mode: bundle at okf/.
r="$work/folder"
new_repo "$r"
cp -R "$TESTDATA/proto/billing-api/okf" "$r/okf"
commit_all "$r" base
check "folder mode: clean bundle passes" bash -c "cd '$r' && '$COMPASS' bundle check okf"

cat > "$r/okf/gotchas/retry-budget.md" <<'EOF'
---
type: Gotcha
title: Retry budget
description: Invoice retries stop after three attempts.
tags: [billing, retries]
generated: { by: claude-code/opus-5, at: 2026-09-29T14:00:00Z }
stale_after: 2027-03-28T14:00:00Z
---
Retries stop after three attempts.
EOF
commit_all "$r" "add concept without okf index"
check "folder mode: concept added without okf index fails" bash -c "cd '$r' && ! '$COMPASS' bundle check okf"
git -C "$r" checkout -q -- okf
"$OKF" index "$r/okf" >/dev/null && commit_all "$r" index
check "folder mode: passes after okf index" bash -c "cd '$r' && '$COMPASS' bundle check okf"

printf -- '---\ntitle: No type\n---\nBody.\n' > "$r/okf/gotchas/no-type.md"
"$OKF" index "$r/okf" >/dev/null 2>&1
commit_all "$r" "concept without type"
check "folder mode: strict-validator finding fails" bash -c "cd '$r' && ! '$COMPASS' bundle check okf"

# okf/main: bundle at the branch root.
m="$work/okf-main"
new_repo "$m"
cp -R "$TESTDATA/proto/shared-auth/okf/." "$m/"
mkdir -p "$m/.github/workflows" && echo "name: okf" > "$m/.github/workflows/okf.yml"
commit_all "$m" base
check "okf/main root: clean bundle passes with ." bash -c "cd '$m' && '$COMPASS' bundle check ."

finish
