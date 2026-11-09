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
  measured here are local values in `corpus.go`; the 48 notes / 16,000
  characters tier is a historical comparison baseline, not the approved
  policy. The approved default is 96 notes / 32,000 characters (product
  owner, 2026-11-05).
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

# 4. Score the S4 real-time recall fixture (no database needed)
go run ./benchmarks/episodic -s4-eval
```

The run writes `results/benchmark.json` and prints one line per cap/mode and
per recall slice. The harness deletes and re-seeds its own synthetic users on
every cap/mode, so repeated runs are deterministic apart from latency.

## S4 real-time recall fixture

`fixtures/s4_multiweek.json` is a six-week, two-user synthetic conversation
set with 28 notes and 19 later queries (three deliberately paraphrase-only so
the metric is not trivially 1.0). It exercises the shipping
`internal/episodicrecall` scorer — same tokenizer, top-8 cap, score>=1
threshold and 100ms budget — through an in-memory candidate store.

- `recall_at_k` / `hit_rate_at_k` are computed over queries with an expected
  note; the fixture gate is >= 0.80 (ADR Stage 2).
- `ghost_hits` counts recalled notes whose source range is covered by an S6-A
  deletion tombstone and must be 0.
- `naive_control_ghost_hits` reruns without the tombstone exclusion; it must be
  > 0, proving the fixture contains a genuine deleted-note ghost risk rather
  than passing by construction.
- Latency is in-process scoring only; the PostgreSQL round trip is measured by
  the runtime path and is bounded by the same 100ms hard timeout.

`go test ./benchmarks/episodic -run TestS4SyntheticMultiweekRecall -v` runs the
same fixture and asserts the gates.

## Committed data

- `fixtures/queries.json` — 320 annotated queries (100 original term, 110
  paraphrase, 110 temporal reference), each naming the exact note id it must
  recall and why. Every expected note is retained at the 48/16,000 baseline
  cap, so the same fixture is valid at every measured cap.
- `results/benchmark.json` — aggregate result of the committed run.
