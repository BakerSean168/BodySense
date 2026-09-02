# Health Document Benchmark

This directory contains the **committed benchmark contract**, not private user reports or bulk run output.

Current lane: ADR 0013 / Health Document Evidence Pipeline technology selection.

## Committed vs local-only data

Committed:

- candidate identity JSON;
- corpus generation spec;
- benchmark thresholds;
- privacy-bounded aggregate result summaries;
- scripts/contracts/tests.

Gitignored under `apps/ai-service/data/benchmarks/health_document/`:

- generated synthetic PDF/image assets;
- exact JSONL corpus manifest containing generated asset hashes;
- full per-fixture candidate run artifacts;
- future deidentified benchmark source material.

Never commit identifiable health documents.

## Current candidates

### Production baseline

```text
tesseract-production-v0.10.2
hdex-config-14af808ef184bf8b
Tesseract 5.5.0
chi_sim + eng
PyMuPDF 1.28.0
300 DPI all-page PDF rasterization
current regex indicator parser
```

### Leading Challenger

```text
rapidocr-ppocrv6-small-v1
hdex-config-381a5312d0eddc5c
RapidOCR 3.9.2
ONNXRuntime 1.29.0
PP-OCRv6 small det + rec
PP-OCRv4 mobile orientation classifier
same 300 DPI all-page PDF rasterization
same current regex indicator parser
```

The Challenger is benchmark-only. It is not the production OCR Champion.

## Generate deterministic synthetic mechanics corpus

```bash
cd apps/ai-service
uv run --extra ocr python scripts/generate_health_document_synthetic_corpus.py --force
uv run --extra ocr python scripts/validate_health_document_corpus.py
```

Expected mechanical corpus:

```text
40 documents
100 pages
9 required cohorts
contains_real_user_data=false
CORPUS_VALIDATION=PASS
SELECTION_READY=false
```

`SELECTION_READY=false` is intentional until a deidentified, human-reviewed and double-reviewed real-layout subset exists.

## Build benchmark image without changing Production dependencies

Default Production-shaped dependency set remains:

```text
--extra ocr --extra pose
```

Benchmark image:

```bash
docker build \
  --build-arg 'UV_SYNC_EXTRAS=--extra ocr --extra pose --extra ocr-benchmark' \
  -t bodysense-ai-healthdoc-benchmark:phase3 \
  apps/ai-service
```

## Production-shaped run

Use the current Production AI resource envelope:

```text
CPU: 2
memory: 384 MiB
swap envelope: 768 MiB total memory+swap
```

Run candidate inside the benchmark image with the synthetic corpus mounted read-only. Full run artifacts remain gitignored.

## Aggregate summaries

Only privacy-bounded aggregate summaries belong under `results/`:

```bash
uv run python scripts/summarize_health_document_run.py \
  --run data/benchmarks/health_document/runs/<run>.json \
  --corpus-root data/benchmarks/health_document/synthetic-v1 \
  --output benchmarks/health_document/results/<candidate>.summary.json
```

These summaries contain metrics and failure counts only; no raw OCR text, images or user identifiers.

## Champion gate

Synthetic results are **mechanics evidence**, not sufficient promotion evidence.

Before ADR 0013 can become Accepted and a new OCR Champion can be promoted, add a privacy-reviewed deidentified real-layout subset with a double-reviewed ground-truth cohort and run the exact same immutable candidates/corpus contract.
