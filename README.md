# compass

compass is one command-line tool, `compass`, for keeping what coding agents
learn about a codebase next to the code. Each repo keeps its knowledge as an
[OKF v0.2](https://github.com/GoogleCloudPlatform/open-knowledge-format/blob/main/SPEC.md)
bundle, a folder of markdown files with frontmatter. Agents read the bundle
before a task and add to it in the same pull request as the code. A hub repo
lists the repos and puts their bundles together, so agents can answer
questions that span repos.

There is no server and no hub CI. Everything runs on people's machines with
their own GitHub credentials. The only CI job is a bundle check in each repo.

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
repos, CI and cloud agents, and removal.

## Commands

| Command | What it does |
|---|---|
| `compass setup` | Connects this machine to your hub: installs `okf`, records the hub clone, installs the skill |
| `compass okf install` | Installs the pinned `okf` binary, refusing a checksum mismatch |
| `compass validate` | Runs the strict OKF validator on a bundle |
| `compass bundle check` | Checks a bundle the way repo CI does |
| `compass hub assemble` | Fetches every bundle in the hub's `repos.txt` into `repos/` |
| `compass hub check` | Checks an assembled hub, including cross-repo links |
| `compass branch setup` | Sets up a branch-mode repo's `okf/` in a clone |
| `compass stamp` | Writes `process:` stamps for the checks in `checks.txt` |
| `compass sync` | Merges approved branch-mode knowledge pull requests after their code merges |
| `compass version` | Prints the commit the build came from |

Run `compass <command> --help` for each command's flags.

## Two ways a repo keeps its knowledge

- **Folder mode:** the bundle is `okf/` on the default branch. It is reviewed
  in the code pull request.
- **Branch mode:** for maintainers who want no knowledge files on their code
  branches. The bundle lives on an `okf/main` branch. Each code branch gets
  its own knowledge branch, reviewed in a separate pull request.

The [design doc](docs/design/okf-knowledge-system-ux.md) walks through both
modes as people and agents use them. The
[research report](docs/research/okf-knowledge-system.md) explains why the
system works this way.

## What is in this repo

| Path | Contents |
|---|---|
| `cmd/compass/`, `internal/` | The Go source |
| `skill/okf/SKILL.md` | The agent skill, built into the binary |
| `pins/okf.env` | The pinned `okf` release and its checksums, built into the binary |
| `actions/bundle-check/` | The GitHub Action that repo CI runs |
| `templates/hub/` | The template for a new hub repo |
| `templates/*.yml`, `templates/*-pointer.md`, `templates/hooks/` | Repo workflows, agent instruction lines, and hook-manager snippets |
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
