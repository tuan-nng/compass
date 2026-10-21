# compass

compass is one command-line tool, `compass`, for keeping what coding agents
learn about a codebase where every agent can find it. All knowledge lives in
one hub repo, as
[OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md)
concepts: markdown files with frontmatter. Each listed repo's knowledge sits
under `repos/<id>/`, next to the concepts that span repos, so agents can
answer questions that cross repos. The code repos carry nothing.

When an agent works on branch `A` of repo `X`, it writes knowledge on hub
branch `X/A` and opens a hub pull request that names the code pull request.
A person approves it, and once the code pull request merges, `compass sync`
checks the result and merges the hub pull request. Adding a repo is one line
in the hub's `repos.txt`.

There is no server and no CI. Everything runs on people's machines with
their own GitHub credentials.

**Status:** compass is built and tested. The pilot on real repos has not
started. The two commands that write to GitHub, `compass stamp` and
`compass sync`, have run live only with `--dry-run`. The
[design doc](docs/design/okf-knowledge-system-ux.md#9-what-does-not-exist-yet)
lists what is not done yet.

## Install

You need Go 1.23 or newer, git, and Linux or macOS on amd64 or arm64.

```sh
git clone https://github.com/tuan-nng/compass
cd compass
make install
compass setup
```

`make install` puts `compass` in `$(go env GOBIN)`, or `$(go env GOPATH)/bin`
if `GOBIN` is unset. `compass setup` asks for your hub repo. It then installs
the pinned `okf` CLI, clones the hub, and gives the OKF skill to your agents.

[docs/install.md](docs/install.md) covers prerequisites, making a hub, adding
repos, protecting the hub, CI and cloud agents, and removal.

## Commands

| Command | What it does |
|---|---|
| `compass setup` | Connects this machine to your hub: installs `okf`, records the hub clone, installs the skill |
| `compass okf install` | Installs the pinned `okf` binary, refusing a checksum mismatch |
| `compass validate` | Runs the strict OKF validator on a folder of concepts |
| `compass hub branch` | Prints the path of a hub worktree on `<id>/<branch>` for the current code branch, creating it on the first run; `--prune` removes finished ones |
| `compass hub check` | Checks a hub: strict validator, cross-repo links, per-repo concept counts, current `index.md` files, no symlinks |
| `compass stamp` | Writes `process:` stamps for the checks in `checks.txt` on the hub's default branch |
| `compass sync` | Merges approved hub pull requests after their code pull requests merge, and cleans up hub branches |
| `compass version` | Prints the commit the build came from |

Run `compass <command> --help` for each command's flags.

## One hub, one access group

Anyone who can read the hub reads every listed repo's knowledge. A team that
needs to keep some repos' knowledge apart runs one hub per access group.

Every developer whose agent writes knowledge needs push access to the hub,
for its `<id>/<branch>` branches. So the review before each merge holds only
with a ruleset on the hub's default branch that blocks direct pushes and
requires pull requests, with code-owner review of the control files.
Rulesets need a public hub or a paid GitHub plan. Only the person who runs
`compass stamp` needs a bypass, on the hub alone.

The [design doc](docs/design/okf-knowledge-system-ux.md) walks through how
people and agents use the system. The
[research report](docs/research/okf-knowledge-system.md) explains why it
works this way, and the
[hub-only plan](plans/261021-1007-hub-branch-knowledge/plan.md) records the
move from per-repo bundles to the hub.

## What is in this repo

| Path | Contents |
|---|---|
| `cmd/compass/`, `internal/` | The Go source |
| `skill/okf/SKILL.md` | The agent skill, built into the binary |
| `pins/okf.env` | The pinned `okf` release and its checksums, built into the binary |
| `templates/hub/` | The template for a new hub repo |
| `templates/user-pointer.md` | The one agent instruction line that sends agents to the hub and the skill |
| `testdata/`, `test/` | Test data and end-to-end test suites |
| `docs/` | Install guide, design doc, research report |
| `plans/` | Build plans and their records |

## Development

```sh
make build   # bin/compass
make test    # go vet, go test, then every suite in test/
```

The suites need `jq`, and `test/install.sh` downloads the pinned `okf` from
GitHub. The Go differential test of the validator runs only when `uv` is
installed, and is skipped otherwise.

## Third-party material

compass ports or adapts code and text from other projects. Each keeps its own
license; [THIRD_PARTY.md](THIRD_PARTY.md) lists them.
