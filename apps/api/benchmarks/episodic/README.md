# Episodic memory benchmark and recall harness

Offline measurement harness for #515 (S2 shadow generation) and #517 (S4
keyword recall). It supports two deliverables:

1. a deterministic 12-month synthetic longitudinal corpus used to measure
   per-user note count, retained characters and rollover under candidate
   retention caps, plus prune-pass latency with and without a generation stub;
2. an annotated Chinese query fixture and a keyword-only recall@8 evaluation
   over the real `episodic_notes` schema on PostgreSQL 18.

Results and interpretation: [episodic-s2-benchmark-recall-eval.md](../../../docs/architecture/episodic-s2-benchmark-recall-eval.md).
Approved memory design: [episodic-memory-hybrid-plan.md](../../../docs/architecture/episodic-memory-hybrid-plan.md).

## Boundaries

- Every user, conversation, message and note is generated from a fixed seed.
  No real user data is read or written; nobody is contacted.
- There is no API, server wire-up, scheduler or runtime retrieval path. The
  harness is a standalone `main` package and executes SQL directly.
- Runtime retention constants are not imported or changed. The retention caps
  measured here are local values in `corpus.go`; the S2-A provisional
  48 notes / 16,000 characters is test-only and is not approved policy.
- No model or remote provider is called. The "extraction stub" is a
  deterministic text builder, so measured latency excludes inference.

## Run

```bash
# 1. Disposable PostgreSQL 18 with the repository migrations applied, e.g.
docker run -d --name bodysense-episodic-bench-pg \
  -e POSTGRES_PASSWORD=benchlocal -e POSTGRES_DB=bodysense_bench \
  -p 55432:5432 pgvector/pgvector:pg18
# then apply apps/api/migrations/000001..000006 and:
#   CREATE EXTENSION pg_trgm;

# 2. Regenerate the committed query fixture (no database needed)
cd apps/api
go run ./benchmarks/episodic -write-fixtures

# 3. Measure storage, rollover, prune latency and recall
BODYSENSE_EPISODIC_BENCH_DSN='postgres://postgres:benchlocal@127.0.0.1:55432/bodysense_bench?sslmode=disable' \
  GIT_COMMIT=$(git rev-parse HEAD) \
  go run ./benchmarks/episodic
```

The run writes `results/benchmark.json` and prints one line per cap/mode and
per recall slice. The harness deletes and re-seeds its own synthetic users on
every cap/mode, so repeated runs are deterministic apart from latency.

## Committed data

- `fixtures/queries.json` — 320 annotated queries (100 original term, 110
  paraphrase, 110 temporal reference), each naming the exact note id it must
  recall and why. Every expected note is retained at the test-only cap, so the
  same fixture is valid at every measured cap.
- `results/benchmark.json` — aggregate result of the committed run.
