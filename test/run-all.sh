#!/usr/bin/env bash
# Run every compass end-to-end suite against one build of compass. Exits
# non-zero if any suite fails. Go tests run separately: go test ./...
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.." || exit 1

if [ -z "${COMPASS:-}" ]; then
  COMPASS="$PWD/.cache/bin/compass"
  go build -o "$COMPASS" ./cmd/compass || { echo "run-all: go build failed"; exit 1; }
fi
export COMPASS

suites=(
  test/install.sh
  test/check-fixture.sh
  test/scale-counts.sh
  test/bundle-check.sh
  test/branch-setup.sh
  test/assemble.sh
  test/setup.sh
)

failed=()
for s in "${suites[@]}"; do
  echo "### $s"
  if ! "$s"; then failed+=("$s"); fi
done

if [ ${#failed[@]} -gt 0 ]; then
  echo "run-all: failed: ${failed[*]}"
  exit 1
fi
echo "run-all: all ${#suites[@]} suites passed"
