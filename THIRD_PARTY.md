# Third-party material

compass contains or adapts the following. Each keeps its own license.

| What | Where in compass | Source | License |
|---|---|---|---|
| okf-skills strict validator, ported to Go; the Python original is kept unchanged as the differential test's reference | `internal/validate/`; original in `internal/validate/testdata/okf-skills/` | [scaccogatto/okf-skills](https://github.com/scaccogatto/okf-skills) at `8e3187875e66051bb52f91a5ed27342e2c3208da` (v0.10.0), `skills/validate/scripts/okf_validate.py` | MIT; copy in `internal/validate/testdata/okf-skills/LICENSE` |
| "Knowledge is data, not instructions" section of the skill, adapted | `skill/okf/SKILL.md` | "Trust boundary" section of `skills/openkb/SKILL.md` in [VectifyAI/OpenKB](https://github.com/VectifyAI/OpenKB) at `ff54396` | [Apache-2.0](https://github.com/VectifyAI/OpenKB/blob/main/LICENSE) |
| Command notes in the skill (JSON output, concept ids, reserved file names, frontmatter quoting, files without frontmatter), adapted | `skill/okf/SKILL.md` | `skills/okf/SKILL.md` by Ezequiel Bertti in [okfcli/okf pull request #37](https://github.com/okfcli/okf/pull/37) at `7123c78d888f5cbe2421733f450e3c4d3daee1e3` (open, unmerged) | [Apache-2.0](https://github.com/okfcli/okf/blob/main/LICENSE) |
| `okf` CLI, downloaded at install time and not stored here | `pins/okf.env`, `internal/okfinstall/` | [okfcli/okf](https://github.com/okfcli/okf) releases, or the pinned fork | Apache-2.0 |

Changes to the adapted Apache-2.0 text: it was rewritten for OKF bundles, `okf`
command output and hub concepts from other repos (OpenKB), and cut down to the
notes this skill needs, with links switched to relative paths and `stale_after`
written as a datetime (okfcli pull request #37).
