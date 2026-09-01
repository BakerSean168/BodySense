# Health Document Evidence Pipeline — Target Architecture

> Status: Target architecture / not yet current production source-of-truth.
> Decision gate: ADR 0013 (Proposed).
> Current production remains Tesseract + PyMuPDF + regex indicator extraction + ADR 0011 admissibility until the benchmark/promotion lane completes.

## 1. Purpose

BodySense needs to ingest user-provided health documents such as:

- laboratory reports;
- physical-examination reports;
- outpatient/inpatient summaries;
- imaging **text reports**;
- scanned paper records;
- report screenshots/photos;
- born-digital PDFs.

The capability exists so users can contribute longitudinal health evidence without manually retyping every value. It is not a diagnosis mechanism.

The target architecture treats document ingestion as an evidence-producing system with explicit provenance, review and replay boundaries.

## 2. Authority boundaries

```text
Health Document Pipeline
  owns: perception / extraction / source grounding

Indicator Parser
  owns: syntactic structuring of source-explicit values

Evidence Admissibility
  owns: whether a candidate may participate in health reasoning

Assessment
  owns: evidence-grounded body-state synthesis

Diagnosis
  owns: diagnosis candidate reasoning / decision authority
```

Forbidden shortcuts:

```text
OCR text -> diagnosis
VLM document summary -> automatically admissible indicator
parser inferred "abnormal" -> diagnosis fact
completed job -> evidence authority
```

## 3. High-level topology

```text
Browser
  -> Go Upload API
  -> immutable object storage + user_uploads
  -> Go JobRuntime: health_document.extract
       -> Python Document Pipeline
            -> strategy selection
            -> native PDF / OCR / layout / table
            -> Document Evidence Model
            -> indicator parser
            -> mechanism provenance
       -> Go contract validation
       -> append-only extraction run persistence
       -> admissibility evaluation
  -> Web review projection
  -> Assessment evidence catalog
```

Go remains the durable job/business authority. Python owns document-perception execution and returns typed, versioned derived artifacts.

## 4. Source artifact contract

Every processing run starts from immutable source identity:

```text
upload_id
document_sha256
mime_type
byte_size
storage_backend
storage_key
created_at
```

The user-facing delete/privacy lifecycle can remove the underlying artifact according to product policy, but while a document exists, no extraction process mutates the original bytes.

Derived files such as rendered pages are temporary/cache artifacts unless explicitly retained for review. They are never substitutes for source identity.

## 5. Strategy selector

### 5.1 Images

```text
JPEG / PNG / WebP
  -> image validation
  -> optional deterministic preprocessing
  -> OCR engine
```

### 5.2 PDF

```text
PDF
  -> native text extraction per page
  -> deterministic Text Quality Gate
       ├─ sufficient
       │    -> native_pdf_text
       └─ insufficient / empty / garbled
            -> rasterize exact page
            -> OCR
```

Mixed PDFs are processed page-by-page; a document is not globally forced into one mode.

### 5.3 Complex table fallback

After text/OCR blocks are available:

```text
row reconstruction confidence good
  -> deterministic health-row parser

row reconstruction confidence poor
  AND document looks tabular
  -> table/layout fallback
```

The fallback must remain bounded to the required pages/regions.

## 6. Text Quality Gate

The gate must be deterministic, versioned and replayable. Example inputs:

```text
printable_ratio
replacement_character_ratio
control_character_ratio
non_whitespace_character_count
line_count
average_line_length
suspicious_repetition_ratio
native_block_count
```

Target output:

```json
{
  "decision": "native_text" | "ocr_required",
  "policy_revision": "document-text-quality-v1",
  "reason_codes": ["sufficient_printable_text"]
}
```

No LLM participates in this decision in the first contract.

## 7. OCR engine boundary

Conceptual adapter:

```python
class DocumentTextEngine(Protocol):
    def identity(self) -> TextEngineIdentity: ...
    def extract_image(self, image: ImageArtifact) -> OCRPageResult: ...
```

`TextEngineIdentity` must cover every behavior-significant mechanism property, for example:

```text
engine family
runtime/backend
runtime version
model family
model artifact/version
model SHA256
language set
orientation/classification model identity
preprocessing revision
numeric normalization policy revision
```

The current benchmark candidates are:

```text
Tesseract + chi_sim+eng                    # baseline
RapidOCR + ONNXRuntime + PP-OCRv6 small   # leading candidate
PP-OCRv6 medium                            # quality challenger
```

The adapter contract allows future replacement without changing evidence-domain semantics.

## 8. PDF mechanism boundary

PDF behavior is independently versioned.

### `PdfTextExtractor`

Identity includes at minimum:

```text
engine
engine version
text extraction options
quality-policy revision
```

### `PdfRasterizer`

Identity includes at minimum:

```text
engine
engine version
DPI
colorspace
alpha/background policy
page box policy
rotation/orientation handling
```

This prevents “same OCR config” from hiding behavior changes in the rasterizer.

## 9. Canonical Document Evidence Model

The pipeline produces structure-preserving evidence instead of only concatenated text.

### 9.1 `DocumentExtractionRun`

```text
id
upload_id
document_sha256
configuration_id
status
started_at
completed_at
mechanism_provenance
quality_summary
error?
```

### 9.2 `PageEvidence`

```text
page_index
width / height
extraction_method:
  native_pdf_text | image_ocr | pdf_page_ocr
mechanism_ref
quality_decision
```

### 9.3 `TextBlockEvidence`

```text
block_id
page_index
text
bbox?                  # normalized or document coordinates
confidence?            # engine-reported, descriptive only
reading_order
source_method
```

### 9.4 `TableEvidence`

```text
table_id
page_index
bbox
rows[]
cells[]
structure_confidence
mechanism_ref
```

### 9.5 `SourceSpan`

A stable reference used by downstream candidates:

```text
document:<upload_id>:run:<run_id>:page:<n>:block:<id>
```

or, for tables:

```text
document:<upload_id>:run:<run_id>:page:<n>:table:<t>:cell:<r>:<c>
```

The exact durable syntax is finalized during implementation, but it must be deterministic inside one frozen run.

## 10. Indicator parser architecture

The parser consumes evidence blocks/cells, not an unstructured global string whenever structured evidence exists.

Pipeline:

```text
Unicode / whitespace normalization
  -> row reconstruction
  -> indicator-name lexicon match
  -> value parser
  -> unit parser
  -> reference-range parser
  -> explicit source flag parser (H/L/↑/↓)
  -> candidate validation
```

### 10.1 Canonical indicator names

Synonyms should converge through a versioned lexicon, for example:

```text
25-羟维生素D
25(OH)D
25-OH-D
Vitamin D
维生素D
```

The lexicon maps source names to a canonical key; it does not interpret clinical meaning.

### 10.2 Numeric parsing

The parser must preserve the raw source string in addition to the normalized value:

```text
raw_value = "17.8"
normalized_value = "17.8"
```

Potentially ambiguous transformations must produce `needs_review` rather than silently normalizing.

### 10.3 Explicit abnormal flags

Only source-explicit flags are parsed:

```text
H / L / ↑ / ↓
```

The parser may record:

```text
source_flag = high
```

but it must not independently conclude medical abnormality from a numeric comparison. Clinical interpretation belongs downstream.

## 11. Indicator candidate contract

Target candidate:

```json
{
  "raw_name": "25-羟维生素D",
  "canonical_key": "vitamin_d_25_oh",
  "raw_value": "17.8",
  "value": "17.8",
  "unit": "ng/mL",
  "reference_range": "30-100",
  "source_flag": "low",
  "source_evidence_refs": ["document:...:cell:2:3"],
  "parser_configuration_id": "report-parser-...",
  "confidence": "high",
  "evidence_admissibility": {
    "status": "admissible",
    "policy_revision": "ocr-indicator-admissibility-v1",
    "reason_codes": ["..."]
  }
}
```

`source_evidence_refs` are mandatory for current-pipeline automatic admission.

## 12. Mechanism provenance

A new extraction result must state exact subordinate identities.

Example:

```json
{
  "document_pipeline_configuration_id": "doc-extract-config-...",
  "text_source": {
    "method": "pdf_page_ocr",
    "ocr_engine": "rapidocr",
    "backend": "onnxruntime",
    "runtime_version": "...",
    "model_family": "PP-OCRv6-small",
    "det_model_sha256": "...",
    "rec_model_sha256": "...",
    "language_profile": "zh-en-v1"
  },
  "pdf": {
    "text_extractor": "pymupdf",
    "text_extractor_version": "...",
    "rasterizer": "pymupdf",
    "rasterizer_version": "...",
    "dpi": 300
  },
  "indicator_parser": {
    "configuration_id": "report-parser-..."
  }
}
```

Go independently validates supported configuration IDs before publishing a durable current projection.

## 13. Persistence target

Proposed relational model:

```text
user_uploads
  id
  user_id
  storage identity
  source metadata
  current_document_extraction_run_id?   # projection only

health_document_extraction_runs
  id
  upload_id
  document_sha256
  configuration_id
  status
  mechanism_provenance jsonb
  quality_summary jsonb
  timestamps

health_document_evidence_blocks
  id
  run_id
  page_index
  kind                 # text_block / table / cell
  source_method
  bbox jsonb
  text
  confidence
  ordinal

report_indicator_candidates
  id
  run_id
  parser_configuration_id
  raw_name
  canonical_key
  raw_value
  normalized_value
  unit
  reference_range
  source_flag
  source_evidence_refs jsonb
  confidence
  evidence_admissibility jsonb

report_indicator_reviews
  id
  candidate_id
  reviewer_type        # user / operator
  decision             # confirm / correct / reject
  corrected fields jsonb
  created_at
```

Schema names are target names and may be adjusted during migration design. Invariants are more important than exact naming.

## 14. Current projection and backward compatibility

During migration:

```text
append-only new tables
  -> authoritative history

user_uploads.ocr_result
  -> compatibility/current projection
```

New code should write the append-only run first and derive the compatibility projection. Do not make JSONB the only truth after the new model is accepted.

Legacy rows:

```text
no invented provenance
no fake source spans
no automatic promotion
```

They remain historical data and can be explicitly reprocessed from the original source if the user still retains the upload.

## 15. Human review flow

```text
needs_review candidate
  -> UI displays value + source region/page
  -> user can confirm / correct / reject
  -> append ReportIndicatorReview
  -> derive reviewed report fact/admissibility state
```

Review UI must display enough source context to avoid asking the user to trust the machine transcription blindly.

A review decision never mutates the historical machine candidate.

## 16. Replay and regression

Replay input:

```text
source document SHA256
frozen extraction configuration
frozen parser configuration
frozen admissibility policy
```

Comparison output should include:

```text
text block additions/removals
numeric value changes
unit changes
reference-range changes
row-association changes
candidate additions/removals
admissibility changes
critical regression flags
```

Raw private document text should not be dumped into generic CI artifacts. Private corpus runs should emit bounded hashes/metrics and redacted diffs.

## 17. Benchmark architecture

The benchmark harness is a first-class repository component, not a one-off notebook.

Conceptual layout:

```text
apps/ai-service/
  benchmarks/health_document/
    corpus_manifest.jsonl
    annotations/
    fixtures/synthetic/
    thresholds.json
  scripts/
    benchmark_health_document_candidates.py
    compare_health_document_runs.py
```

Sensitive/private source files are stored outside Git. Repository manifests reference opaque fixture IDs/hashes, not user paths or PHI.

## 18. Benchmark metrics

### OCR/text metrics

```text
CER / WER                 # supporting metrics, not primary ranking
Chinese character accuracy
```

### Health-critical metrics

```text
Numeric Exact Match
Unit Exact Match
Reference Range Exact Match
Indicator Name Match
Row Association Accuracy
Indicator Exact Match
Critical Numeric Error Rate
```

### Runtime metrics

```text
cold start
warm latency/page
throughput
peak RSS
CPU utilization
model bytes
container/image delta
failure rate
```

A candidate with better CER but worse critical numeric error rate does not win.

## 19. Candidate deployment policy

### Leading fast path

```text
RapidOCR + ONNXRuntime + PP-OCRv6 small
```

This is the current leading candidate because it combines a Chinese-oriented PP-OCR model family with a CPU-friendly runtime/backend abstraction. It is not yet the Champion.

### Baseline

```text
Tesseract + chi_sim+eng
```

Keep as benchmark baseline, historical comparator and emergency fallback candidate until the new Champion is proven.

### Quality challenger

```text
PP-OCRv6 medium
```

Benchmark whether quality gains justify memory/latency cost.

### Complex-table fallback

Evaluate only on complex-table cohort. Possible candidates:

```text
Paddle table-recognition components
PP-StructureV3 table subsystem
```

Do not add this cost to the default fast path without evidence.

## 20. Production resource boundary

Current production is CPU constrained. The document pipeline must be benchmarked under a production-shaped resource limit.

Promotion options:

```text
A. keep execution in existing ai-service
   when incremental RSS/latency is safe

B. isolate a document-extraction container/worker
   when the winning engine is bursty/heavy
```

In both cases:

```text
Go JobRuntime owns job truth
same immutable extraction configuration
same durable evidence contract
```

Topology is replaceable; evidence semantics are not.

## 21. Security and privacy

- originals are private user data;
- no benchmark corpus may contain identifiable real user health data in Git;
- raw OCR text is not emitted to generic logs;
- source bounding boxes are treated as document-derived private data;
- external cloud OCR is out of the default design; a future remote provider requires a separate privacy/security decision;
- deletion/retention must follow existing user-upload/privacy-erasure contracts.

## 22. Observability

Safe operational events should include:

```text
upload_id? hashed/bounded internal id
document mime category
page count
strategy counts: native/OCR/table
configuration IDs
latency
peak/worker resource class
candidate count
admissible / needs_review / rejected counts
failure reason code
```

Do not log raw report text, indicator values or source images by default.

## 23. Failure semantics

```text
source unreadable
  -> extraction failed

native text low quality + OCR unavailable
  -> extraction failed, not silent empty success

OCR verified but no text found
  -> completed_no_text

parser finds no indicators
  -> valid extraction; zero candidates

candidate ambiguous
  -> needs_review

unknown mechanism/provenance
  -> cannot auto-admit
```

## 24. Non-goals for first implementation

- generalized medical document understanding of arbitrary hospital forms;
- automatic diagnosis from reports;
- full clinical terminology ontology;
- VLM-first document extraction;
- cloud OCR dependency;
- universal table reconstruction for every PDF;
- backfilling invented provenance onto old OCR rows.

## 25. External architecture references

- Docling OCR backends / staged document architecture: https://github.com/docling-project/docling
- RapidOCR / PP-OCR backends: https://github.com/RapidAI/RapidOCR
- PaddleOCR / PP-OCRv6 / table pipelines: https://github.com/PaddlePaddle/PaddleOCR
- Paperless-ngx original-document and OCR-mode lifecycle: https://docs.paperless-ngx.com/
- OCRmyPDF engine/rasterizer plugin boundary: https://ocrmypdf.readthedocs.io/en/stable/plugins.html
