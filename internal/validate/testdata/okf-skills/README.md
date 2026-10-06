# okf-skills validator (test-only reference)

- Source: https://github.com/scaccogatto/okf-skills, file
  `skills/validate/scripts/okf_validate.py`, unchanged.
- Commit: `8e3187875e66051bb52f91a5ed27342e2c3208da` (release v0.10.0).
- License: MIT, see `LICENSE` (copied from the same commit).
- `okf_validate.py.lock` pins its one dependency, `pyyaml`, by hash
  (`uv lock --script okf_validate.py`).

`compass validate` is the Go port of this script (plan decision 19). The copy
here only feeds `../../differential_test.go`, which runs it with
`uv run --locked --script okf_validate.py <bundle> [--json] --strict` and
requires the port to report the same findings.
