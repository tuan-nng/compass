#!/usr/bin/env bash
# Generate the scale hub (research protocol, "Shared inputs"), in the hub's
# committed layout:
#   <dir>/index.md       the hub root index, with okf_version frontmatter
#   <dir>/repos/rNN/     50 repo folders x 200 concepts (10,000 .md files);
#   concept cIII lives in d(III mod 5)/; every tenth concept (10%) carries a
#   link ../../rMM/dX/cYYY.md into another repo's folder; the string
#   zebracorn-needle appears only in repos/r37/d3/c123.md. No index.md below
#   the root: run `okf index .` at <dir> for those. Output is deterministic.
#
# Usage: gen-scale.sh <dir> [repos] [concepts-per-repo]
set -euo pipefail
[ $# -ge 1 ] || { echo "usage: gen-scale.sh <dir> [repos] [concepts]" >&2; exit 2; }
dir=$1 repos=${2:-50} concepts=${3:-200}
[ ! -e "$dir/repos" ] || { echo "gen-scale.sh: $dir/repos already exists" >&2; exit 1; }
mkdir -p "$dir"
printf -- '---\nokf_version: "0.2"\n---\n# Scale hub\n' > "$dir/index.md"

python3 - "$dir" "$repos" "$concepts" <<'PY'
import os, sys

root, nrepos, nconcepts = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
types = ["Module", "Service", "Gotcha", "Decision", "Runbook"]

def rel(i):
    return f"d{i % 5}/c{i:03d}.md"

for r in range(nrepos):
    base = os.path.join(root, "repos", f"r{r:02d}")
    for d in range(5):
        os.makedirs(os.path.join(base, f"d{d}"), exist_ok=True)
    for i in range(nconcepts):
        j = (i + 32) % nconcepts
        lines = [
            "---",
            f"type: {types[i % len(types)]}",
            f"title: Concept r{r:02d} c{i:03d}",
            f"description: Synthetic concept {i} of repo r{r:02d}.",
            f"tags: [scale, r{r:02d}]",
            "status: stable",
            "generated: { by: process:gen-scale, at: 2026-09-29T00:00:00Z }",
            "---",
            f"# Concept r{r:02d} c{i:03d}",
            "",
            f"Synthetic body text for repo r{r:02d}, concept {i}.",
            "",
            f"Related: [c{j:03d}](../{rel(j)}).",
        ]
        if i % 10 == 7:
            other = (r + 1 + i) % nrepos
            if other == r:
                other = (r + 1) % nrepos
            k = (i * 7 + 3) % nconcepts
            lines.append(f"Depends on [r{other:02d} c{k:03d}](../../r{other:02d}/{rel(k)}).")
        if r == 37 and i == 123:
            lines.append("")
            lines.append("Search marker: zebracorn-needle.")
        with open(os.path.join(base, rel(i)), "w") as f:
            f.write("\n".join(lines) + "\n")
PY
echo "gen-scale.sh: wrote $repos repo folders x $concepts concepts under $dir/repos"
