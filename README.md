# go-elasticsearch-mutant

Mutation testing tool for Elasticsearch Query DSL built with the [go-elasticsearch](https://github.com/elastic/go-elasticsearch) v8 Typed API.

Your test suite sends queries to a real Elasticsearch instance. This tool injects small mutations into your query-building code and checks whether the tests catch them — surfacing gaps in test coverage that unit tests alone cannot find.

## Requirements

- Go 1.21+
- [go-elasticsearch v8](https://github.com/elastic/go-elasticsearch) Typed API
- A running Elasticsearch instance accessible during test execution

## Installation

```bash
go install github.com/kurakura967/go-elasticsearch-mutant/cmd/esmutant@latest
```

Or build from source:

```bash
git clone https://github.com/kurakura967/go-elasticsearch-mutant.git
cd go-elasticsearch-mutant
go build -o ~/bin/esmutant ./cmd/esmutant
```

## Usage

Run from the root of your module (where `go.mod` lives):

```bash
esmutant run ./... --threshold 80
```

When your production code and integration tests live in separate packages, use `--test` to specify where to run the tests:

```bash
esmutant run ./internal/repository/... \
  --test ./testing/integration/... \
  --workers 1
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | `.` | Project root directory (where `go.mod` lives) |
| `--test` | | *(same as pattern)* | Package pattern for running tests; use when tests are in a different package from the mutated code |
| `--workers` | `-w` | `4` | Number of parallel workers |
| `--timeout` | `-t` | `30` | Per-test timeout in seconds |
| `--threshold` | | `80.0` | Minimum mutation score (0–100); exits with code 1 if below |
| `--output` | `-o` | `console` | Output format: `console` or `json` |
| `--verbose` | `-v` | `false` | Show `go test` output for survived/errored mutants |

### Example output

```
Analyzing ./...
  Found 9 mutation target(s)

Generating mutants...
  example/search.go:41 BuildActiveUsersQuery    BoolQuery.Must → nil    [RemoveClause]
  ...
  13 mutant(s) total

Running 13 mutant(s) (4 worker(s)) ...

Mutation Score: 9/11 (81.8%)
  Killed: 9  Survived: 2  Timeouts: 0  Errors: 0  |  Skipped: 2 (not counted in score)

KILLED (9):
  example/search.go:41 BuildActiveUsersQuery    BoolQuery.Must → nil    [RemoveClause]
  ...

SURVIVED (2):
  example/search.go:98 BuildArticlesQuery    BoolQuery.Must → Should    [MustToShould]

SKIPPED (2):
  example/search.go:48 BuildActiveUsersQuery    BoolQuery.Filter → Must    [FilterToMust]
    reason: BoolQuery already has a Must field; renaming Filter would create a duplicate struct key
```

## Supported Mutation Operators

The tool detects struct field assignments from the `github.com/elastic/go-elasticsearch/v8/typedapi/types` package and applies the following operators:

### RemoveClause

Sets `BoolQuery.Must` or `BoolQuery.Should` to `nil`, testing that required clauses are enforced by tests.

```go
// Original
Bool: &types.BoolQuery{
    Must: []types.Query{{Match: ...}},
}

// Mutant
Bool: &types.BoolQuery{
    Must: nil,
}
```

### MustToShould

Renames `BoolQuery.Must` to `Should`, testing that tests distinguish between required and optional matching.

```go
// Original          // Mutant
Must: []types.Query  →  Should: []types.Query
```

### FilterToMust

Renames `BoolQuery.Filter` to `Must`, testing whether tests detect the difference between a non-scoring filter and a scoring must clause.

> **Note:** Automatically skipped when the same `BoolQuery` already contains a `Must` field (renaming would create a duplicate struct key, which is a compile error in Go).

```go
// Original          // Mutant
Filter: []types.Query  →  Must: []types.Query
```

### RangeBoundary

Changes inclusive range boundaries to exclusive ones, testing that tests verify exact boundary conditions.

Applies to: `NumberRangeQuery`, `DateRangeQuery`, `TermRangeQuery`, `UntypedRangeQuery`

```go
// Original          // Mutant
Gte: &min      →    Gt: &min
Lte: &max      →    Lt: &max
```

### RemoveMustNot

Sets `BoolQuery.MustNot` to `nil`, testing that exclusion conditions are enforced by tests.

```go
// Original
MustNot: []types.Query{{Term: ...}}

// Mutant
MustNot: nil
```

## How It Works

1. **Analyze** — Loads your Go packages and finds all Typed API struct field assignments that are mutation targets.
2. **Generate** — Applies each operator to each target site, producing a set of mutated source files (using `go/ast` rewriting).
3. **Run** — Executes `go test` with each mutant via `go test -overlay`, leaving original files untouched.
4. **Report** — Shows which mutants were killed (tests caught the change), survived (tests did not catch it), or skipped.

The mutation score is `killed / (killed + survived + timeouts + errors) × 100`. Skipped mutations are excluded from the score.

## Notes

- Mutations are applied via [`go test -overlay`](https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program), so original source files are never modified.
- If your tests share Elasticsearch indices across parallel runs (e.g., via `TestMain`), use `--workers 1` to avoid race conditions.
