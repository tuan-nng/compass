#!/usr/bin/env bash
# Check the rebuilt test data against the planted-facts table of the research
# protocol (compass: docs/research/okf-knowledge-system/evidence/protocol.md),
# and the prototype set against the report's conventions (report sections 4, 5).
#
# Staleness is judged from stale_after and today's UTC date, never from
# `okf show`'s stale field, so this passes with upstream v0.5.0 and the fork.
set -uo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

O=$(okf_bin) || { echo "cannot install pinned okf" >&2; exit 1; }
F="$TESTDATA/fixture"
P="$TESTDATA/proto"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# validate_json <bundle>: okf validate output; exit code ignored.
validate_json() { "$O" validate "$1" 2>/dev/null || true; }
# count_findings <json> <rule> <concept-id>
count_findings() { jq --arg r "$2" --arg c "$3" '[.findings[] | select(.rule == $r and .concept_id == $c)] | length' <<<"$1"; }
# field <bundle> <id> <jq-path>: one field of `okf show`.
field() { "$O" show "$1" "$2" | jq -r ".concept$3 // empty"; }
# tier <bundle> <id>: trust tier from `okf list`.
tier() { "$O" list "$1" | jq -r --arg id "$2" '.concepts[] | select(.id == $id) | .trust_tier'; }
eq() { [ "$1" = "$2" ]; }

copy_hub "$F" "$work/hub-copy"
HUB="$work/hub-copy"
today=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "== fixture: planted facts"
idem="$F/billing-api/okf/gotchas/idempotency-key.md"
sa=$(field "$F/billing-api/okf" gotchas/idempotency-key .stale_after)
check "idempotency-key stale_after is the spec datetime 2026-09-01T00:00:00Z" eq "$sa" "2026-09-01T00:00:00Z"
check "idempotency-key is stale on $today (stale_after < now)" test "$sa" \< "$today"
check "idempotency-key is a draft" eq "$(field "$F/billing-api/okf" gotchas/idempotency-key .status)" draft
check "idempotency-key is unverified" eq "$(tier "$F/billing-api/okf" gotchas/idempotency-key)" unverified
check "idempotency-key links root-relative /contracts/invoice-api.md" grep -q '](/contracts/invoice-api.md)' "$idem"

billing=$(validate_json "$F/billing-api/okf")
hub=$(validate_json "$HUB")
check "root-relative link resolves standalone" eq "$(count_findings "$billing" okf/links/broken gotchas/idempotency-key)" 0
check "root-relative link is broken inside the hub" eq "$(count_findings "$hub" okf/links/broken repos/billing-api/gotchas/idempotency-key)" 1

check "invoice-api is machine-confirmed (process: verifier)" eq "$(tier "$F/billing-api/okf" contracts/invoice-api)" machine-confirmed
check "invoice-api verifier is a process: actor" eq "$(field "$F/billing-api/okf" contracts/invoice-api '.verified[0].by' | cut -d: -f1)" process

svc="$F/billing-api/okf/services/billing-api.md"
check "billing-api service is human-reviewed" eq "$(tier "$F/billing-api/okf" services/billing-api)" human-reviewed
check "billing-api footnote [^main-go] is defined" grep -q '^\[\^main-go\]:' "$svc"
check "billing-api sources[].id main-go matches the footnote" eq "$(field "$F/billing-api/okf" services/billing-api '.sources[0].id')" main-go
check "footnote join raises no finding" eq "$(jq '[.findings[] | select(.rule | startswith("okf/sources/"))] | length' <<<"$billing")" 0

web="$F/web-app/okf/services/web-app.md"
webj=$(validate_json "$F/web-app/okf")
check "web-app has exactly 1 URL link" eq "$(grep -oE '\]\(https?://[^)]+\)' "$web" | wc -l)" 1
check "web-app has exactly 2 hub-relative ../../ links" eq "$(grep -oE '\]\(\.\./\.\./[^)]+\)' "$web" | wc -l)" 2
check "web-app ../../ links are broken standalone" eq "$(count_findings "$webj" okf/links/broken services/web-app)" 2
check "web-app ../../ links are valid in the hub" eq "$(count_findings "$hub" okf/links/broken repos/web-app/services/web-app)" 0

check "legacy-api-key is deprecated" eq "$(field "$F/shared-auth/okf" concepts/legacy-api-key .status)" deprecated

dep="$F/hub/cross-repo/invoice-dependency.md"
check "invoice-dependency links /repos/billing-api/" grep -q '](/repos/billing-api/' "$dep"
check "invoice-dependency links /repos/web-app/" grep -q '](/repos/web-app/' "$dep"
check "invoice-dependency links are valid in the hub" eq "$(count_findings "$hub" okf/links/broken cross-repo/invoice-dependency)" 0

for r in billing-api web-app shared-auth; do
  check "hub repos/$r/index.md carries okf_version" grep -q '^okf_version:' "$HUB/repos/$r/index.md"
  check "okf flags repos/$r/index frontmatter in the hub" eq "$(count_findings "$hub" okf/reserved/index-frontmatter "repos/$r/index")" 1
done
check "strict validator flags the 3 nested index frontmatters" eq "$(strict_validate "$HUB" 2>/dev/null | grep -c 'index.md should contain no frontmatter')" 3

want='["cross-repo/invoice-dependency","repos/billing-api/services/billing-api","repos/web-app/services/web-app"]'
got=$("$O" backlinks "$HUB" repos/billing-api/contracts/invoice-api | jq -c '.backlinks | sort')
check "hub backlinks to invoice-api are exactly the 3 planted links" eq "$got" "$want"
check "hub view lists 9 concepts" eq "$("$O" list "$HUB" | jq .count)" 9

echo "== proto: report conventions"
for b in billing-api/okf web-app/okf shared-auth/okf; do
  check "proto $b passes the strict validator" strict_validate "$P/$b"
  check "proto $b has no root-relative links" bash -c "! grep -rqE '\]\(/' '$P/$b'"
  check "proto $b has an overview.md" test -f "$P/$b/overview.md"
done
copy_hub "$P" "$work/proto-hub" strip
PH="$work/proto-hub"
check "proto hub assembled with stripped indexes passes the strict validator" strict_validate "$PH"
check "proto web-app has no links into other repos' files" bash -c "! grep -qE '\]\(\.\./\.\./' '$P/web-app/okf/services/web-app.md'"
want='["cross-repo/invoice-dependency","repos/billing-api/gotchas/idempotency-key","repos/billing-api/services/billing-api"]'
got=$("$O" backlinks "$PH" repos/billing-api/contracts/invoice-api | jq -c '.backlinks | sort')
check "proto hub backlinks include the cross-repo edge and the fixed relative link" eq "$got" "$want"
check "proto idempotency-key keeps the stale datetime" eq "$(field "$P/billing-api/okf" gotchas/idempotency-key .stale_after)" "2026-09-01T00:00:00Z"
for b in billing-api/okf web-app/okf shared-auth/okf; do
  rm -rf "$work/idx" && cp -R "$P/$b" "$work/idx"
  "$O" index "$work/idx" >/dev/null 2>&1
  check "proto $b index files are current" diff -r "$P/$b" "$work/idx"
done

finish
