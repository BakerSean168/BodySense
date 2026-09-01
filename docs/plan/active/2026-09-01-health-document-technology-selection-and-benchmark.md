# Health Document Technology Selection & Benchmark — Active Plan

- Status: ACTIVE / DESIGN + BENCHMARK FIRST
- Date: 2026-09-01
- Owner: BodySense
- Governing ADR: [ADR 0013](../../adr/0013-adopt-versioned-health-document-evidence-pipeline.md) — Proposed until benchmark/promotion decision
- Target architecture: [Health Document Evidence Pipeline](../../architecture/health-document-evidence-pipeline.md)
- User-facing contract: [Health Document Ingestion Feature Spec](../../feature_spec_health_document_ingestion.md)
- Existing safety gate: [ADR 0011](../../adr/0011-adopt-ocr-report-indicator-evidence-admissibility.md)

## 1. Objective

Choose and implement the long-term BodySense health-document extraction stack using evidence from BodySense-specific documents rather than general OCR reputation.

The plan must answer four different questions independently:

```text
1. How should born-digital PDF text be extracted?
2. Which OCR engine/model should be the default image/scanned-page Champion?
3. When is a layout/table fallback necessary and which implementation is justified?
4. How should health indicators be parsed, grounded, reviewed, replayed and governed?
```

The plan explicitly avoids the shortcut:

```text
"newer OCR looks better" -> replace production
```

## 2. Current baseline

Production currently uses:

```text
Image:
  Tesseract
  pytesseract
  chi_sim+eng

PDF:
  PyMuPDF
  render every page at 300 DPI
  Tesseract

Indicator extraction:
  repository regex patterns

Evidence authority:
  ocr-indicator-admissibility-v1
  Assessment v4 / assessment-evidence-contract-v3
```

This baseline remains production truth until the benchmark lane selects and qualifies a replacement.

## 3. Candidate matrix

### 3.1 Required candidates

| Candidate       | Role               | Runtime                                     | Model          | Why it is included                     |
| --------------- | ------------------ | ------------------------------------------- | -------------- | -------------------------------------- |
| Tesseract       | baseline           | CLI/pytesseract                             | `chi_sim+eng`  | current production comparator          |
| RapidOCR        | leading challenger | ONNXRuntime CPU                             | PP-OCRv6 small | lightweight Chinese-oriented candidate |
| PP-OCRv6 medium | quality challenger | benchmark-selected reproducible CPU runtime | medium tier    | test quality/weight trade-off          |

### 3.2 Optional candidates

Only add when required candidates are already reproducible:

- RapidOCR PP-OCRv6 tiny, if production memory is the dominant constraint;
- OpenVINO backend, if ONNXRuntime CPU is clearly suboptimal on the deployment CPU class;
- Paddle table-recognition subsystem / PP-StructureV3 table path for complex-table cohort;
- a VLM/document model only as shadow/rescue, never primary automatic health-number authority in this lane.

### 3.3 Architecture references, not benchmark candidates by default

- Docling — backend/pipeline abstraction;
- Paperless-ngx — original-preserving and OCR-auto lifecycle;
- OCRmyPDF — OCR/rasterizer plugin boundary;
- Marker/Unstructured/MinerU class — strategy routing and selective OCR.

Do not benchmark monolithic frameworks merely because they are popular. Benchmark the mechanism choices BodySense could actually deploy.

## 4. Benchmark corpus

### 4.1 Corpus classes

Target initial corpus: **minimum 40 documents / 100 pages**, then expand if results are unstable.

Required cohorts:

| Cohort               |   Minimum | Description                                              |
| -------------------- | --------: | -------------------------------------------------------- |
| native_pdf_simple    |  10 pages | born-digital laboratory/exam PDFs with usable text layer |
| native_pdf_table     |  10 pages | born-digital reports with multi-column tables            |
| scanned_pdf_clear    |  15 pages | clean scanner-style PDFs                                 |
| scanned_pdf_degraded |  10 pages | skew/noise/compression/low contrast                      |
| phone_photo_clear    | 15 images | photographed reports with good lighting                  |
| phone_photo_degraded | 15 images | perspective, reflection, shadow, blur                    |
| chinese_lab_table    |  15 pages | Chinese blood/biochemistry/vitamin-type reports          |
| mixed_zh_en_units    |  10 pages | Chinese + English names/units/symbols                    |
| complex_table        |  10 pages | multi-column/merged-cell/irregular structure             |

One document may belong to multiple semantic cohorts, but benchmark summaries must report each cohort independently.

### 4.2 Data-source policy

Preferred order:

1. synthetic BodySense fixtures generated from repository-owned templates;
2. openly licensed/public sample reports with license metadata;
3. manually de-identified private evaluation samples stored outside Git.

Never commit identifiable user health documents.

Private corpus manifest entries may include:

```json
{
  "fixture_id": "private-lab-001",
  "sha256": "...",
  "cohorts": ["phone_photo_degraded", "chinese_lab_table"],
  "source": "private-deidentified",
  "license": null
}
```

No local filesystem path, patient name or raw text belongs in committed benchmark results.

## 5. Ground-truth annotation

### 5.1 Annotation unit

The primary ground truth is not the full OCR transcript. It is a set of health-critical fields grounded to source locations.

```json
{
  "document_id": "fixture-001",
  "page": 1,
  "indicators": [
    {
      "raw_name": "25-羟维生素D",
      "canonical_key": "vitamin_d_25_oh",
      "value": "17.8",
      "unit": "ng/mL",
      "reference_range": "30-100",
      "source_flag": "L",
      "bbox": [0.12, 0.34, 0.88, 0.39]
    }
  ]
}
```

### 5.2 Annotation rules

- preserve source spelling separately from canonical key;
- do not infer a missing unit/range;
- record explicit H/L/↑/↓ only when present;
- ambiguous source text is annotated as ambiguous instead of guessed;
- every numeric ground-truth item is manually double-checked;
- a subset (target >= 20%) is independently reviewed twice to estimate annotation error.

## 6. Benchmark outputs

Every candidate run must emit a machine-readable result with:

```text
candidate configuration identity
engine/runtime versions
model hashes
input fixture hashes
per-document metrics
per-cohort metrics
runtime metrics
failure reason codes
critical regression examples (redacted)
```

Results are deterministic artifacts under a benchmark results directory; `generated_at` must not participate in result identity/digest.

## 7. Accuracy metrics

### 7.1 Supporting OCR metrics

- Character Error Rate (CER);
- Word Error Rate (WER) when meaningful;
- Chinese character exact accuracy.

These metrics are useful but **not** the promotion objective.

### 7.2 Primary health-evidence metrics

#### Numeric Exact Match (NEM)

```text
correct normalized numeric strings / total expected numeric indicators
```

`17.8` vs `77.8` is an error even if surrounding text is perfect.

#### Unit Exact Match (UEM)

Exact normalized unit match after only approved deterministic normalization.

#### Reference Range Exact Match (RREM)

Both bounds/operator semantics must match.

#### Indicator Name Match (INM)

Canonical indicator key matches the ground truth.

#### Row Association Accuracy (RAA)

Value/unit/reference range were associated with the correct indicator row/cell.

#### Indicator Exact Match (IEM)

An indicator counts as exact only when required fields all match:

```text
name/key + value + unit + row association
```

Reference range/flag may be included as stricter sub-metrics depending on fixture annotation.

#### Critical Numeric Error Rate (CNER)

Critical errors include:

- digit substitution/insertion/deletion changing value;
- decimal-point shift;
- sign loss;
- exponent/scientific notation corruption;
- unit magnitude corruption;
- value attached to wrong indicator.

CNER is the most important fail-closed metric.

## 8. Runtime metrics

Measure at minimum:

```text
cold initialization seconds
warm page latency p50/p95
pages/minute
peak RSS
steady-state RSS delta
CPU time
model bytes
container image delta
failure rate
```

Run under:

1. developer benchmark hardware for iteration;
2. a production-shaped Linux CPU/memory limit before promotion.

Current Production is approximately 2 vCPU / 1.6 GiB host RAM with a constrained AI-service allocation, so memory measurements are promotion-relevant rather than cosmetic.

## 9. Initial promotion gates

The first benchmark iteration should use conservative gates. Exact values may be adjusted **before** seeing final candidate names/scores, not after, to avoid winner-fitting.

Proposed hard gates:

```text
CNER <= 0.5% overall
CNER = 0 on the manually designated safety-critical numeric subset
NEM >= 99.0%
UEM >= 99.0%
RAA >= 98.5%
IEM >= 97.5%
```

Cohort gates:

```text
no required cohort may regress > 2 percentage points on NEM vs current Tesseract baseline
complex-table cohort may use a separate fallback-path gate
```

Resource gate for in-process AI-service deployment:

```text
no OOM / swap-thrash in production-shaped validation
AI healthcheck remains stable
peak RSS remains within declared container budget with safety margin
```

If the accuracy winner violates the in-process resource gate, do **not** downgrade quality silently. Evaluate isolated document-extraction execution topology.

## 10. Ranking policy

A candidate is disqualified by any hard safety/resource/license gate.

Among qualified candidates, rank roughly by:

```text
1. CNER
2. NEM / RAA / IEM
3. degraded-photo + Chinese-table cohort quality
4. peak RSS
5. warm latency
6. operational complexity
```

Generic CER is secondary.

## 11. Native PDF strategy benchmark

This lane is independent from OCR-engine selection.

Compare:

```text
A. current all-pages rasterize + OCR
B. native text first + deterministic quality gate + selective OCR fallback
```

Required evidence:

- no loss of correct native numeric values;
- reduced OCR page count on born-digital PDFs;
- equal or better indicator exact match;
- lower latency/CPU for native PDFs;
- correct mixed PDF page routing.

The target architecture expects B to win, but the implementation must still be covered by regression fixtures.

## 12. Table/layout strategy benchmark

Do not start by introducing a heavyweight table model globally.

Phase sequence:

```text
1. OCR/native blocks + deterministic row reconstruction
2. measure RAA on complex_table cohort
3. only if RAA fails target:
     benchmark table fallback candidates
4. enable fallback only behind structural confidence routing
```

Possible table candidates:

- Paddle table-recognition components;
- PP-StructureV3 table subsystem.

Measure table-specific peak RSS separately.

## 13. Parser benchmark

The current regex parser is a baseline.

Target deterministic parser v1 should include:

```text
normalization
row reconstruction
versioned indicator lexicon
numeric parser
unit parser
range parser
source flag parser
source evidence refs
```

Compare:

```text
current regex baseline
vs
structured deterministic parser v1
```

Primary metric is end-to-end Indicator Exact Match, not number of extracted candidates.

False positives are more expensive than missing low-confidence candidates because ADR 0011 may still auto-admit a candidate if upstream confidence is high.

## 14. Required benchmark harness implementation

Proposed files:

```text
apps/ai-service/benchmarks/health_document/
  corpus_manifest.jsonl
  thresholds.json
  annotations/
  fixtures/synthetic/

apps/ai-service/src/document_pipeline/
  contracts.py
  strategy.py
  engines/

apps/ai-service/scripts/
  benchmark_health_document_candidates.py
  compare_health_document_runs.py
```

The benchmark script must support:

```text
--candidate tesseract
--candidate rapidocr-ppocrv6-small
--candidate ppocrv6-medium
--cohort ...
--fixture ...
--output ...
--production-shaped
```

Do not bake developer-specific absolute paths into committed artifacts.

## 15. Implementation phases

### Phase 0 — Freeze current baseline

- [ ] record current Tesseract/pytesseract/PyMuPDF versions;
- [ ] record current `chi_sim+eng` language configuration;
- [ ] convert current behavior into an explicit historical baseline configuration ID;
- [ ] add baseline regression fixtures before changing extraction behavior.

Exit gate: current production behavior reproducible in benchmark harness.

### Phase 1 — Build corpus + annotations

- [ ] create synthetic report generator/templates;
- [ ] add at least 40 documents / 100 pages across required cohorts;
- [ ] annotate health-critical indicator truth;
- [ ] add double-review subset;
- [ ] add privacy/license metadata.

Exit gate: corpus validator passes and no sensitive source is committed.

### Phase 2 — Benchmark current Tesseract

- [ ] run baseline accuracy;
- [ ] collect resource metrics;
- [ ] store result artifact;
- [ ] identify failure taxonomy.

Exit gate: reproducible baseline report.

### Phase 3 — Benchmark RapidOCR + PP-OCRv6 small

- [ ] pin RapidOCR/runtime versions;
- [ ] pin model artifacts/hashes;
- [ ] run same corpus;
- [ ] compare NEM/UEM/RAA/IEM/CNER;
- [ ] measure production-shaped resource use.

Exit gate: candidate result is reproducible and license review complete.

### Phase 4 — Benchmark PP-OCRv6 medium

- [ ] pin exact runtime/model identity;
- [ ] run same corpus;
- [ ] measure whether quality improvement justifies cost.

Exit gate: quality challenger comparison complete.

### Phase 5 — Select OCR Champion

Decision record must state:

```text
selected candidate
exact configuration identity
why alternatives lost
accuracy evidence
resource evidence
license evidence
rollback baseline
```

At this point ADR 0013 may move from Proposed to Accepted with exact Champion identity.

### Phase 6 — Native PDF first implementation

- [ ] add versioned PyMuPDF text extractor;
- [ ] implement deterministic Text Quality Gate;
- [ ] selective page OCR fallback;
- [ ] mixed-PDF regression tests;
- [ ] benchmark against old all-page OCR path.

### Phase 7 — Document Evidence Model

- [ ] define extraction-run and source-span contracts;
- [ ] add append-only schema migrations;
- [ ] retain current `ocr_result` only as compatibility projection;
- [ ] source blocks/cells carry page/bbox/method/mechanism identity.

### Phase 8 — Structured indicator parser v1

- [ ] versioned health indicator lexicon;
- [ ] block/table row reconstruction;
- [ ] source-grounded value/unit/range extraction;
- [ ] mandatory source refs;
- [ ] current regex kept as historical comparator.

### Phase 9 — Admissibility v2 / review integration if required

ADR 0011 v1 remains valid until evidence shows a reason to change.

Possible additions:

- source-grounding requirement;
- mechanism-provenance requirement;
- user-confirmed path from `needs_review` to reviewed evidence.

Any change to automatic evidence membership requires a new versioned admissibility policy and corresponding Assessment evidence-policy/config revision.

### Phase 10 — Human review UX

- [ ] show candidate + source page/region;
- [ ] confirm/correct/reject;
- [ ] append review history;
- [ ] never mutate machine extraction result;
- [ ] reviewed values expose reviewer provenance.

### Phase 11 — Replay / comparison

- [ ] same frozen document under old/new configurations;
- [ ] numeric/unit/row regression diff;
- [ ] privacy-bounded artifacts;
- [ ] qualification report.

### Phase 12 — Shadow / promotion

Because BodySense is pre-user/single-owner, promotion may be compressed after qualification, but no engine/config can become default merely because the code merged.

Suggested sequence:

```text
qualification corpus PASS
  -> Dev real sample smoke
  -> Staging
  -> frozen-document replay
  -> owner review of critical diffs
  -> new Champion
  -> patch/minor release
  -> Production
```

Old Tesseract configuration remains explicit rollback/replay identity until a later cleanup ADR retires it.

## 16. Production topology decision gate

After the OCR winner is known:

```text
if winner fits current ai-service memory/latency budget:
  keep document engine in ai-service
else:
  split bounded document-extraction runtime/container
```

Do not create a second durable job queue. Go JobRuntime continues to own job state and retry/idempotency.

## 17. Validation matrix

### Unit

- native PDF quality gate;
- engine identity and model hash validation;
- OCR block ordering;
- row reconstruction;
- numeric/unit/reference parser;
- candidate source refs;
- provenance completeness;
- admissibility integration.

### Contract

- Python extraction response schema;
- Go independent mechanism/config validation;
- unsupported configuration fails closed;
- compatibility projection does not lose review state.

### Integration

- image report -> extraction run -> candidates;
- born-digital PDF -> native text path without OCR;
- scanned PDF -> page OCR path;
- mixed PDF -> mixed strategy;
- needs-review candidate -> review -> reviewed fact;
- reprocessing creates new run, not overwrite.

### E2E

- upload report;
- processing status;
- show indicators;
- source highlight/page context;
- confirm/correct/reject;
- Assessment only sees admissible/reviewed evidence.

### Deployment

- exact engine/runtime/model identity in built image;
- no runtime mutable/latest model downloads;
- production-shaped peak RSS;
- health checks remain green;
- rollback to previous extraction Champion is possible.

## 18. Definition of done

Technology selection is done when:

- [ ] required corpus exists and passes privacy/license validation;
- [ ] Tesseract baseline measured;
- [ ] RapidOCR + PP-OCRv6 small measured;
- [ ] PP-OCRv6 medium measured;
- [ ] hard safety/resource gates frozen and evaluated;
- [ ] Champion selected with exact immutable identity;
- [ ] ADR 0013 updated to Accepted.

Architecture implementation is done when:

- [ ] native PDF text-first routing is current;
- [ ] immutable extraction-run provenance is durable;
- [ ] source-grounded evidence blocks/cells exist;
- [ ] indicator candidates carry source refs + parser identity;
- [ ] Go validates supported mechanism/config before publication;
- [ ] legacy rows are not assigned invented provenance;
- [ ] human review is append-only;
- [ ] replay comparison works on frozen documents;
- [ ] production-shaped validation passes;
- [ ] current architecture/feature docs updated in same PR;
- [ ] current Champion deployed to Staging and Production via exact-SHA Release lifecycle.

## 19. Explicit non-goals

Do not expand this lane into:

- general medical diagnosis from document text;
- cloud OCR provider integration;
- arbitrary Office document ingestion;
- full hospital interoperability / HL7/FHIR import;
- unbounded VLM document agents;
- a generalized data-labeling platform.

Those require separate product decisions.

## 20. Research references

- RapidOCR: https://github.com/RapidAI/RapidOCR
- RapidOCR docs: https://github.com/RapidAI/RapidOCRDocs
- PaddleOCR / PP-OCRv6: https://github.com/PaddlePaddle/PaddleOCR
- Docling OCR backends: https://github.com/docling-project/docling/blob/main/docs/concepts/OCR.md
- Paperless-ngx OCR modes/original lifecycle: https://docs.paperless-ngx.com/configuration/
- OCRmyPDF engine/rasterizer plugins: https://ocrmypdf.readthedocs.io/en/stable/plugins.html
