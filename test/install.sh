#!/usr/bin/env bash
# `compass okf install` installs the pinned okf, and refuses a checksum
# mismatch without installing anything. Needs network access to GitHub releases.
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
want=$(sed -n 's/^OKF_VERSION=//p' "$TOOLS_ROOT/pins/okf.env")

compass okf install --dest "$work/good" >/dev/null 2>&1
check "installs the pinned okf" test -x "$work/good/okf"
got=$("$work/good/okf" --version 2>/dev/null | jq -r .version)
check "okf --version prints the pinned version $want" test "$got" = "$want"

sed -E 's/^(OKF_SHA256_[a-z]+_[a-z0-9]+=).*/\10000000000000000000000000000000000000000000000000000000000000000/' \
  "$TOOLS_ROOT/pins/okf.env" > "$work/bad.env"
check "a checksum mismatch exits non-zero" bash -c "! '$COMPASS' okf install --dest '$work/bad' --pins '$work/bad.env'"
check "a checksum mismatch installs nothing" test ! -e "$work/bad/okf"

finish
