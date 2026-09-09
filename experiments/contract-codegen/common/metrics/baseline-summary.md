# B0 baseline capture

## Repository baseline

- Base commit: `020581ad325735f73515c2e370cfc04f19b8f63e`.
- The canonical worktree had unrelated curriculum edits and the untracked spike plan, so all experiments are isolated in dedicated worktrees.
- Production routing/transport is unchanged.

## Selected real boundaries

- R1: `GET /api/v1/health-workspace`.
- R2 primary: `POST /api/v1/body-state/facts` because it combines a request body, optimistic revision and real 409 revision-conflict semantics.
- R2 supplemental path control: `PATCH /api/v1/body-state/facts/{fact_id}/review`.
- S1: public `StreamEvent` JSON/SSE family.
- I1: Go -> Python consultation runtime request + NDJSON stream.

The plan's R2 wording asks one sample to include path/query params, optimistic revision and 409 while naming `addFact`, `recordOutcome`, and `updateLifestyleCurrent` as candidates. No one of those three satisfies all criteria. This baseline keeps `addFact` as the scored R2 sample and uses `reviewFact` only to exercise generated path-parameter APIs.

## Current manual contract surface

See `manual-definition-count.tsv`. Representative handwritten duplication already includes:

- StreamEvent TS static contract: 334 LOC.
- StreamEvent TS runtime parser: 194 LOC.
- StreamEvent JSON Schema: 1037 LOC.
- StreamEvent Go DTO: 50 LOC.
- StreamEvent Python Pydantic model: 93 LOC.
- HealthWorkspace Go DTO: 77 LOC.
- HealthWorkspace Web DTO: 171 LOC.
- HealthWorkspace handwritten Web client: 206 LOC.
- Internal Go AI client/contract surface: 546 LOC.
- Python runtime HTTP contract surface: 332 LOC.

LOC is only a surface proxy; the decision metric is how many wire facts still require manual synchronized edits.

## Baseline verification

After installing each worktree's dependencies normally (reusing package caches/stores rather than symlinking another worktree's `node_modules`):

- Web typecheck: PASS, 13.07 s.
- Web production build: PASS, 6.71 s.
- Go `go test ./...`: PASS; warm run 0.79 s, first observed run 19.38 s.
- Python full pytest: PASS, 475 tests; 22.61 s outer elapsed, 16.18 s pytest-reported runtime.

Python full tests require the repository's `dev`, `ocr`, and `document-ocr` extras. A preliminary environment with only the dependency group named `dev` failed collection because those optional runtime imports were absent; that environment error is excluded from candidate scoring.

## Web bundle baseline

- raw: 2,676,615 bytes.
- gzip: 751,802 bytes.
- brotli: 637,955 bytes.

See `web-bundle-baseline.tsv` for per-file detail.

## Common OpenAPI fixture / governance

`spec/bodysense-spike.openapi.yaml` is OpenAPI 3.1 and passes Redocly 2.51.2 recommended lint with no findings. It is intentionally an experiment-only mirror of R1/R2 and is not connected to production handlers.

With oasdiff 1.31.0 and explicit `--fail-on ERR`:

| Mutation | Reported breaking | CI gate |
|---|---|---|
| M1 required response field rename | yes | blocked |
| M2 response field integer -> string | yes | blocked |
| M3 request enum value removal | yes | blocked |
| M4 optional request field -> required | yes | blocked |
| M5 nullable response field -> non-null | no | passed |
| M6 add optional response field | no | passed |

The M5 result is a concrete governance gap for this OpenAPI 3.1 `anyOf: [string, null]` encoding and must be covered by generated type/runtime tests or a stricter/custom compatibility rule if OpenAPI wins.

`oasdiff breaking` does not make the process fail for detected findings unless the selected CLI invocation includes `--fail-on ERR`/`WARN`; the reproducible governance script uses `--fail-on ERR` explicitly.

A preliminary harness mutated a fully dereferenced bundle, so component changes no longer affected the inline endpoint copies. Those false-negative results were discarded; the committed harness regenerates a ref-preserving bundle before each mutation run.

## Baseline runtime-trust observation

The current Web `request<T>() -> expectJson<T>()` path gives TypeScript a compile-time view but does not validate the network response against `HealthWorkspace` at runtime. Runtime trust is therefore a real comparison dimension rather than an already-solved baseline property.
