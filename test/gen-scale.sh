#!/usr/bin/env bash
# Generate the scale test set (research protocol, "Shared inputs"):
#   <dir>/repos/rNN/okf/  50 bundles x 200 concepts, plus one root index.md each
#   (10,050 .md files); concept cIII lives in d(III mod 5)/; every tenth concept
#   (10%) carries a cross-repo link ../../rMM/dX/cYYY.md that resolves only in
#   an assembled hub; the string zebracorn-needle appears only in
#   r37/okf/d3/c123.md. Output is deterministic.
#
# Usage: gen-scale.sh <dir> [repos] [concepts-per-repo]
set -euo pipefail
[ $# -ge 1 ] || { echo "usage: gen-scale.sh <dir> [repos] [concepts]" >&2; exit 2; }
dir=$1 repos=${2:-50} concepts=${3:-200}
[ ! -e "$dir/repos" ] || { echo "gen-scale.sh: $dir/repos already exists" >&2; exit 1; }

python3 - "$dir" "$repos" "$concepts" <<'PY'
import os, sys

root, nrepos, nconcepts = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
types = ["Module", "Service", "Gotcha", "Decision", "Runbook"]

def rel(i):
    return f"d{i % 5}/c{i:03d}.md"

for r in range(nrepos):
    base = os.path.join(root, "repos", f"r{r:02d}", "okf")
    for d in range(5):
        os.makedirs(os.path.join(base, f"d{d}"), exist_ok=True)
    with open(os.path.join(base, "index.md"), "w") as f:
        f.write(f'---\nokf_version: "0.2"\n---\n# Repo r{r:02d}\n\n')
        f.write("".join(f"- [d{d}](d{d}/)\n" for d in range(5)))
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
echo "gen-scale.sh: wrote $repos bundles x $concepts concepts under $dir/repos"
