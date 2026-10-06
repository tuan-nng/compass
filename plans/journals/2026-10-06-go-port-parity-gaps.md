# Go port of the OKF tooling: where parity broke and how it was caught

## What happened

Phases 01–02 of `plans/261006-1358-okf-knowledge-system` ported the okf-tools bash and Python tools into the `compass` Go binary. Every ported test suite passed, but an independent review that compared the Go code line by line with the Python originals still found a trust-boundary blocker.

- `internal/config.ParseChecks` declared the `process:<name>` actor pattern but never applied it. Python's `config.parse_checks` rejects any other actor. No Python or Go test had a bad-actor row, so both suites passed. The effect: a `checks.txt` row with `human:alice`, or a quoted actor carrying `, at: … }`, would have made the writer app commit a forged or malformed `verified` entry.
- Smaller gaps of the same kind:
  - `fmt.Sscan` reads `#010` as octal 8, where Python's `int()` reads 10;
  - Go's `\d` is ASCII-only, while Python's matches any Unicode digit;
  - `"head_sha": null` was treated like a missing key;
  - control files split lines only on `\n`, while Python's `splitlines()` also splits on a lone CR, form feed and U+2028.

## Decision

- Port tests are not enough evidence of parity for trust-boundary code. For each guard in the original (each `raise`, each `fullmatch`), confirm the port has a matching guard and a test row that trips it.
- Expected values for parsing edge cases come from running the Python function, not from reasoning about it (`TestLinksReadsReferencesAsPythonDoes`).
- The validator uses yaml.v3 with an emulation of pyyaml's type rules, not a pyyaml port. A differential test against the Python reference is the guard. The only difference it allows is the parser's wording inside "frontmatter is not valid YAML".
- `compass setup` downloads and checksums the pinned `okf` on every run, as the old installer did. It rewrites the binary only when the bytes differ, so a second run changes nothing. Trusting an installed binary whose `--version` matched was rejected, because a copy installed with the unchecked `--okf-bin` flag would have been kept forever.
- `compass setup` checks the hub with `git ls-remote` over https before writing anything. A machine whose git uses ssh and has no https credentials fails that check until it runs `gh auth setup-git`. The error message says so.

## Verification

- With the actor check removed, `TestChecksRows` and `TestChecksRowActorMustBeProcess` fail. With it restored, they pass.
- `go test ./...` and `./test/run-all.sh` (7 suites) pass.
- A live `compass setup` against GitHub installed okf on the first run. The second run reported every item current. A symlinked skill folder was replaced with a real one.

## Next steps

Phase 04 needs explicit authorization before it pushes compass and creates the scratch hub and apps.

Historical work record — not durable authority.
