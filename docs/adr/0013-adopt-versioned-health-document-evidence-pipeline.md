# ADR 0013: Adopt a versioned Health Document Evidence Pipeline with benchmark-gated OCR selection

- Status: Proposed
- Date: 2026-09-01
- Scope: health-report ingestion, OCR/PDF extraction, layout/table parsing, indicator extraction, evidence provenance, review, replay, deployment resource policy
- Related: ADR 0002, ADR 0004, ADR 0009, ADR 0011, ADR 0012

## Implementation status — 2026-09-02

The architectural direction is implemented behind a non-Champion qualification lane. `health-document-extraction-v20` (`hdex-config-f2495c95b6ed9de2`) passed the deterministic 40-document / 100-page synthetic authority and resource gates in a dedicated 2 CPU / 384 MiB `document-service`: 97% automatic evidence coverage, 100% exactness among auto-admitted indicators, zero critical false admissions, zero failed fixtures, zero OOM/memory events and zero swap.

Production remains on the frozen Tesseract baseline `hdex-config-14af808ef184bf8b`. This ADR remains **Proposed** because final acceptance still requires the deidentified, independently double-reviewed real-layout selection subset. Passing the synthetic mechanics gate is necessary but not sufficient for Champion promotion.

## Context

BodySense accepts health-report images and PDFs so users do not need to manually re-enter laboratory values, examination summaries or other report text. The current implementation is functional but is still an MVP mechanism:

```text
image
  -> Tesseract via pytesseract
  -> plain text
  -> regex indicator extractor

PDF
  -> PyMuPDF render every page at 300 DPI
  -> Tesseract via pytesseract
  -> plain text
  -> regex indicator extractor
```

Current evidence governance is already safer than the extraction mechanism itself. ADR 0011 separates OCR completion from evidence admissibility, and Assessment v4 only consumes report indicators carrying the exact supported `ocr-indicator-admissibility-v1` decision. However, the extraction mechanism that creates those candidates is not yet versioned as an immutable behavior contract.

The current path has six architectural weaknesses.

### 1. OCR technology selection was never completed as a BodySense benchmark

The archived Issue #5 design proposed PaddleOCR as the preferred Chinese OCR engine and Tesseract as a fallback. The executable implementation later converged on Tesseract, but there is no repository-owned benchmark proving that Tesseract is the best long-term choice for BodySense health documents.

A behavior-significant mechanism must not be made immutable before the project has demonstrated that the mechanism is the right baseline.

### 2. Born-digital PDFs are unnecessarily rasterized

The current PDF path renders every page to pixels and OCRs it, even when the PDF already contains a reliable text layer. That creates an avoidable lossy transformation:

```text
correct embedded text
  -> rasterize
  -> OCR
  -> potentially altered digits/units
```

Health-document ingestion must prefer a native text source when its quality is sufficient, and only OCR pages/regions that actually require OCR.

### 3. Plain OCR text loses document structure

Health reports are often tables. A flat text stream can lose the relationship between:

- indicator name;
- measured value;
- unit;
- reference range;
- explicit high/low flag;
- page/row/cell location.

For BodySense, a wrong row association can be as dangerous as a character recognition error.

### 4. Durable report evidence cannot explain how it was produced

Current `OCRResult` does not durably state the exact OCR engine/version, language configuration, PDF extraction/rendering path, model artifact, parser revision or indicator extractor revision that produced an indicator.

This is the document equivalent of the Posture mechanism problem resolved by ADR 0012.

### 5. Reprocessing currently looks like replacement rather than a first-class historical run

`user_uploads.ocr_result` is a convenient current projection, but a future engine/parser upgrade needs append-only extraction history so the system can answer:

```text
same frozen document
  -> old extraction configuration
  -> new extraction configuration
  -> what changed?
  -> did any numeric indicator regress?
```

### 6. BodySense has stricter correctness requirements than generic PDF-to-Markdown tools

A generic parser may tolerate cosmetic text mistakes. BodySense must heavily penalize errors such as:

```text
17.8 -> 77.8
1.25 -> 12.5
mg/L -> g/L
reference row A -> value from row B
```

The primary evidence path therefore needs deterministic, reviewable source grounding. VLM-based document parsing may be useful as a rescue or review assistant, but must not become an unqualified primary authority for health numbers.

## Research principles adopted from mature open-source systems

The target architecture intentionally borrows design principles rather than importing one monolithic project.

### Docling

Use the architectural pattern of a staged document pipeline and pluggable OCR backends. Docling supports multiple OCR engines, including RapidOCR and Tesseract, and separates document conversion/layout/table stages from OCR-engine choice.

### Paperless-ngx

Adopt two lifecycle rules:

1. always preserve the original uploaded document; and
2. do not OCR born-digital PDFs when sufficient embedded text is already available.

### OCRmyPDF

Adopt explicit interfaces between OCR engine, rasterizer and preprocessing. The PDF rasterizer is a behavior-significant mechanism and should not be an invisible implementation detail.

### PaddleOCR / RapidOCR

Treat PP-OCRv6 as the leading Chinese OCR model family and RapidOCR + ONNXRuntime as the leading lightweight CPU deployment candidate. This is a candidate decision, not yet a Champion decision.

### Unstructured / Marker / MinerU class of systems

Adopt strategy routing: native text, OCR, layout and table parsing should be selected according to the document/page quality rather than forcing every input through one path.

## Decision

### 1. Rename the architectural capability from “OCR” to “Health Document Evidence Pipeline”

OCR is one mechanism inside a larger document-ingestion system.

The target path is:

```text
Immutable UserUpload
  -> Document Classification
  -> Extraction Strategy Selection
  -> Native PDF Text OR OCR
  -> Document Evidence Model
  -> Layout / Table Reconstruction when needed
  -> Health Indicator Extraction
  -> Evidence Admissibility / Review
  -> Published Report Fact / Assessment Evidence
```

The public product may still use familiar copy such as “识别报告”, but internal architecture and contracts must distinguish document extraction from OCR.

### 2. Preserve the original upload as immutable source evidence

The original user document remains the source artifact. Extraction, rendering, OCR and indicator parsing create derived artifacts and must never overwrite the source.

Every extraction run binds to:

```text
upload_id
document_sha256
configuration_id
created_at
```

A reprocess creates a new extraction run.

### 3. Use native PDF text first; OCR is a fallback strategy

For every PDF page:

```text
native text extraction
  -> Text Quality Gate
     -> sufficient -> use native text
     -> empty/garbled/insufficient -> rasterize + OCR
```

The quality gate must be deterministic and versioned. It may consider, for example:

- non-whitespace character count;
- replacement/control-character ratio;
- printable character ratio;
- token/line density;
- suspicious repeated glyph patterns;
- whether expected table rows collapse into unusable text.

The first implementation must not use an LLM to decide whether the native text layer is trusted.

### 4. Introduce a pluggable `DocumentTextEngine` boundary

The application must not scatter direct imports of Tesseract/RapidOCR/PaddleOCR through domain logic.

Conceptual interface:

```text
DocumentTextEngine
  identify() -> immutable mechanism identity
  extract_image(image) -> OCR blocks + confidence + bbox

PdfTextExtractor
  identify() -> immutable mechanism identity
  extract_page(page) -> text blocks + source spans

PdfRasterizer
  identify() -> immutable mechanism identity
  rasterize(page, config) -> image artifact
```

Exact Python interface names may differ, but the dependency direction is mandatory:

```text
pipeline owns policy
engine adapter owns vendor API
```

### 5. Do not select the new OCR Champion by preference alone

The new Champion is benchmark-gated.

Initial candidates:

```text
Baseline:
  Tesseract + chi_sim+eng

Leading candidate:
  RapidOCR + ONNXRuntime + PP-OCRv6 small

Quality challenger:
  PP-OCRv6 medium
  via the most reproducible CPU runtime established by the benchmark
```

Optional complex-document/table candidates are evaluated separately and must not be forced into the default fast path.

A candidate cannot become Champion until it satisfies the benchmark gates defined in the active plan.

### 6. Separate text recognition from layout/table understanding

The default path must remain lightweight:

```text
native text OR OCR blocks
  -> deterministic row/column reconstruction
  -> health indicator parser
```

Only documents/pages that fail structural confidence may enter a more expensive table/layout path.

Candidate fallback technologies include Paddle table-recognition components / PP-StructureV3 subsystems. The architecture explicitly rejects “run the full document-understanding stack for every report” as the default policy.

### 7. Introduce a canonical Document Evidence Model

The extraction layer must preserve source structure rather than returning only one `raw_text` string.

Target concepts:

```text
DocumentExtractionRun
PageEvidence
TextBlockEvidence
TableEvidence
CellEvidence
SourceSpan
```

At minimum, each evidence block must be able to answer:

```text
which upload?
which page?
which extraction method?
which mechanism configuration?
where on the source page?
what exact extracted text?
```

For image/OCR evidence, source location should be a bounding box when the engine provides one. For native PDF text, it should retain page and text/block coordinates when available.

### 8. Indicator extraction must create source-grounded candidates

A health indicator candidate is not only:

```text
name + value + unit
```

It must retain an evidence reference to the source block/cell/span that produced it.

Target shape:

```text
indicator_id
canonical_indicator_key?   # optional until normalization is implemented
raw_name
value
unit
reference_range
explicit_report_flag?      # e.g. H/L/↑/↓ only when source explicitly states it
source_evidence_refs[]
parser_configuration_id
confidence
admissibility
```

Medical normal/abnormal interpretation remains outside the parser authority.

### 9. Mechanism provenance and evidence admissibility remain separate contracts

Mechanism provenance answers:

```text
how was this value produced?
```

Evidence admissibility answers:

```text
may this value enter health reasoning now?
```

A perfectly identified OCR engine can still read the wrong number. A user-confirmed value can become admissible even if initial machine confidence was not high. These concepts must never be collapsed into one confidence flag.

### 10. Human review becomes a first-class future publication path

`needs_review` is not a dead end.

The target lifecycle is:

```text
extracted candidate
  -> admissible automatically
  OR
  -> needs_review
       -> user/operator sees source highlight
       -> confirm / correct / reject
       -> reviewed report fact
```

A corrected value must record both the machine-extracted candidate and the review decision. Do not overwrite the historical extraction output.

### 11. Use append-only extraction runs with a current projection

The target persistence model is:

```text
user_uploads
  -> immutable source metadata + storage pointer

document_extraction_runs
  -> one row per processing attempt/configuration

document_evidence_blocks
  -> page/block/table/cell evidence

report_indicator_candidates
  -> parser outputs linked to evidence blocks

report_indicator_reviews
  -> human review decisions
```

The existing `user_uploads.ocr_result` may remain temporarily as a compatibility/current projection while the append-only model is introduced. Migration/backfill must never invent provenance for legacy rows.

### 12. Replay is defined on frozen source bytes + immutable configuration

A replay/counterfactual comparison is valid only when it binds:

```text
document_sha256
extraction_configuration_id
indicator_parser_configuration_id
admissibility_policy_revision
```

The system must be able to compare two extraction runs without mutating either one.

### 13. VLM document parsing is not the primary automatic health-number authority

VLM-based extraction may be evaluated for:

- complex layout rescue;
- shadow comparison;
- review assistance;
- explanation of a highlighted source region.

It must not directly create automatically admissible health numbers unless a future ADR defines an evidence contract and qualification gate strong enough for that authority.

### 14. Resource policy is part of technology selection

Current production is a small CPU host. A candidate that improves OCR quality but forces an unsafe persistent memory footprint is not automatically acceptable.

The benchmark therefore records:

- latency;
- peak RSS;
- model/artifact size;
- cold-start/provisioning behavior;
- steady-state AI-service health impact.

If the winning engine cannot safely coexist inside the current AI-service resource budget, execution topology may move to a bounded document worker/container while Go JobRuntime continues to own durable job truth. The domain contract must not depend on whether the engine is in-process or isolated.

## Champion selection gates

The exact numeric thresholds are maintained by the active benchmark plan, but the ADR requires these classes of gates:

```text
Critical Numeric Error Rate
Numeric Exact Match
Unit Exact Match
Reference Range Exact Match
Row Association Accuracy
Indicator Exact Match
Chinese text quality
Peak RSS / latency / image size
Failure determinism
Offline reproducibility
License review
```

The primary ranking objective is not generic OCR CER. Critical numeric/unit/row errors have higher severity than cosmetic text errors.

## Compatibility and migration policy

### Existing OCR rows

Existing `user_uploads.ocr_result` remains historical data. It is not retroactively assigned an engine configuration or source span that was never recorded.

ADR 0011 already makes legacy rows without current admissibility metadata unavailable to Assessment v4. The new document pipeline preserves the same fail-closed philosophy for mechanism identity.

### Current API

Existing upload/OCR APIs may be adapted behind compatibility projections. Public/client migration should not require the Web to understand every internal extraction stage immediately.

### JobRuntime

Go JobRuntime remains the durable owner of asynchronous processing lifecycle. This ADR does not create a second job-truth system in Python.

## Consequences

### Positive

- BodySense chooses OCR based on its own health-document evidence quality rather than general OCR reputation;
- native PDF text is no longer degraded by mandatory rasterization;
- document structure and source location remain reviewable;
- OCR/model/parser upgrades become replayable and auditable;
- health indicator values can be traced back to the exact source page/region;
- human review integrates cleanly with existing admissibility semantics;
- a future engine swap does not require another domain redesign.

### Trade-offs

- benchmark corpus and annotations require deliberate work before engine promotion;
- append-only extraction history increases schema and storage complexity;
- page/block/table evidence is larger than a single `raw_text` JSON field;
- lightweight and complex-layout paths require explicit routing logic;
- some medium-confidence values remain unavailable to Assessment until review.

## Rejected alternatives

### Declare Tesseract the permanent Champion now

Rejected because the project has not demonstrated that it is the best option for current Chinese health reports.

### Replace Tesseract immediately with PaddleOCR/RapidOCR

Rejected because selection without a BodySense benchmark would repeat the same governance mistake with a newer engine.

### Use PP-StructureV3/Docling/Marker as one monolithic parser for every report

Rejected because BodySense needs a small, deterministic default path and currently runs on constrained CPU/RAM. The projects are architectural references and optional fallback sources, not an automatic all-or-nothing dependency choice.

### OCR every PDF page

Rejected because born-digital text is often the higher-fidelity source.

### Parse only flat text and discard coordinates

Rejected because health-value review and row-association debugging require source grounding.

### Let an LLM/VLM extract final health values directly

Rejected for the primary automatic evidence path. Numeric health evidence requires deterministic source grounding and stronger qualification than generic document understanding.

### Overwrite `ocr_result` during reprocessing

Rejected because it destroys replay and provenance history.

## Implementation authority

This ADR is **Proposed**, not yet Accepted, because the OCR Champion remains benchmark-gated. The target architecture and benchmark methodology are described in:

- [`../architecture/health-document-evidence-pipeline.md`](../architecture/health-document-evidence-pipeline.md)
- [`../plan/active/2026-09-01-health-document-technology-selection-and-benchmark.md`](../plan/active/2026-09-01-health-document-technology-selection-and-benchmark.md)
- [`../feature_spec_health_document_ingestion.md`](../feature_spec_health_document_ingestion.md)

The current production source of truth remains Tesseract/PyMuPDF + ADR 0011 admissibility until this lane is implemented and promoted.

## Research references

- RapidOCR: https://github.com/RapidAI/RapidOCR
- RapidOCR install/backend documentation: https://github.com/RapidAI/RapidOCRDocs
- PaddleOCR: https://github.com/PaddlePaddle/PaddleOCR
- Docling OCR architecture: https://github.com/docling-project/docling/blob/main/docs/concepts/OCR.md
- Paperless-ngx usage/configuration: https://docs.paperless-ngx.com/usage/ and https://docs.paperless-ngx.com/configuration/
- OCRmyPDF plugin architecture: https://ocrmypdf.readthedocs.io/en/stable/plugins.html
