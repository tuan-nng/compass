# The bundle-check action can't use setup-go's cache

## What happened

Phase 04 of `plans/261006-1358-okf-knowledge-system` ran `actions/bundle-check` live for the first time. Plan decision 16 said the action would set up Go "with its build cache on". The setup-go v6.5.0 source shows that the cache can't key on compass's `go.sum`, even with `cache-dependency-path` pointed at it. This came from reading the source; no live run tried `cache: true`.

- `glob.hashFiles` skips any file outside `GITHUB_WORKSPACE` and logs "Ignore '${file}' since it is not under GITHUB_WORKSPACE.";
- `cache-restore.ts` then throws "Some specified paths were not resolved, unable to cache dependencies.".

A composite action's own checkout sits under the runner's `_actions` folder, outside the caller's workspace. So setup-go can never key a cache on the action's `go.sum`.

## Decision

- `actions/bundle-check` sets `cache: false`. The uncached `go build` takes 19.3 s and 18.9 s on the live runs, and the whole action takes 30–35 s. Both are under the 60 s target in plan decision 12.
- If the build ever goes over 60 s, cache `GOCACHE` with `actions/cache`, keyed on a hash of the action's Go sources computed in a run step. This is recorded in the plan's Risks.
- The action finds its own checkout through `$GITHUB_ACTION_PATH` in a run step, not `github.action_path` in `with:`, which names the host path inside container jobs (actions/runner#2185).

## Verification

- Five scratch repos pin the action to compass `be5b9cf`. All five push runs conclude `success`.
- A PR that adds a concept without `okf index` fails with "index files are out of date".

## Next steps

Phase 05 deletes `okf-tools` and `tuan-nng/knowledge-hub` and rewrites the docs.
