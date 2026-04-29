# ctx

`ctx` is a deterministic context compiler runtime for Go CLI workflows.

It scans a repository, ranks the files and excerpts that matter for a task, and emits a reproducible context bundle under a token budget. The goal is benchmarkable context selection: same repo, same task, same budget, same output.

`ctx` is not a DSL. v0 is a local runtime with SQLite state, an estimate tokenizer, deterministic BM25/rules retrieval, and benchmark-first proof against simple baselines.

## Commands

```sh
ctx scan <path>
ctx compile "<task>" --repo <path> --budget <tokens> --format json --explain
ctx explain --last
ctx bench --repo <path> --cases <file> --baseline naive|repomix
```

## What It Does

- `scan` indexes a repository into local SQLite state.
- `compile` selects a deterministic context bundle for a task and token budget.
- `--format json` emits machine-readable output for downstream tools.
- `--explain` records the ranking and budgeting decisions for inspection.
- `explain --last` replays the most recent compile explanation from local state.
- `bench` runs fixed benchmark cases against `naive` or `repomix` baselines.

## Runtime Model

`ctx` is intentionally local and deterministic in v0:

- Local SQLite stores scan state and the most recent explanation.
- Token budgets use an estimate tokenizer, not provider billing tokenizers.
- Retrieval uses deterministic BM25 and rules.
- Benchmarks are the proof path for quality and regression tracking.

## Explicit Non-Goals For v0

`ctx` does not claim any of the following in v0:

- embeddings or vector search
- distributed state or remote indexing
- provider-side prompt caching
- a web UI or hosted control plane
- a custom DSL

## Development

Run the local checks:

```sh
go test ./...
go vet ./...
go build ./...
```

The CI workflow runs formatting validation, tests, vet, and build using the Go version declared in `go.mod`.
